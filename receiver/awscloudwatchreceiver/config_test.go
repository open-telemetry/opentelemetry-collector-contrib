// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package awscloudwatchreceiver

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
)

func TestValidate(t *testing.T) {
	cases := []struct {
		name        string
		config      Config
		expectedErr error
	}{
		{
			name: "Valid Config",
			config: Config{
				Region: "us-west-2",
				Logs: &LogsConfig{
					MaxEventsPerRequest: defaultEventLimit,
					PollInterval:        defaultPollInterval,
					Groups: GroupConfig{
						AutodiscoverConfig: nil,
					},
				},
			},
		},
		{
			name: "Invalid No Region",
			config: Config{
				Region: "",
			},
			expectedErr: errNoRegion,
		},
		{
			name: "Nil Logs",
			config: Config{
				Region: "us-west-2",
				Logs:   nil,
			},
			expectedErr: errNoLogsConfigured,
		},
		{
			name: "Invalid Event Limit",
			config: Config{
				Region: "us-west-2",
				Logs: &LogsConfig{
					MaxEventsPerRequest: -1,
					PollInterval:        defaultPollInterval,
				},
			},
			expectedErr: errInvalidEventLimit,
		},
		{
			name: "Invalid Poll Interval",
			config: Config{
				Region: "us-west-2",
				Logs: &LogsConfig{
					MaxEventsPerRequest: defaultEventLimit,
					PollInterval:        100 * time.Millisecond,
					Groups: GroupConfig{
						AutodiscoverConfig: nil,
					},
				},
			},
			expectedErr: errInvalidPollInterval,
		},
		{
			name: "Invalid Log Group Limit",
			config: Config{
				Region: "us-east-1",
				Logs: &LogsConfig{
					MaxEventsPerRequest: defaultEventLimit,
					PollInterval:        defaultPollInterval,
					Groups: GroupConfig{
						AutodiscoverConfig: &AutodiscoverConfig{
							Limit: -10000,
						},
					}},
			},
			expectedErr: errInvalidAutodiscoverLimit,
		},
		{
			name: "Invalid IMDS Endpoint",
			config: Config{
				Region:       "us-east-1",
				IMDSEndpoint: "xyz",
				Logs: &LogsConfig{
					MaxEventsPerRequest: defaultEventLimit,
					PollInterval:        defaultPollInterval,
					Groups: GroupConfig{
						AutodiscoverConfig: nil,
					},
				},
			},
			expectedErr: errors.New("unable to parse URI for imds_endpoint"),
		},
		{
			name: "Both Logs Autodiscover and Named Set",
			config: Config{
				Region: "us-east-1",
				Logs: &LogsConfig{
					MaxEventsPerRequest: defaultEventLimit,
					PollInterval:        defaultPollInterval,
					Groups: GroupConfig{
						AutodiscoverConfig: &AutodiscoverConfig{
							Limit: defaultEventLimit,
						},
						NamedConfigs: map[string]StreamConfig{
							"some-log-group": {
								Names: []*string{aws.String("some-lg-name")},
							},
						},
					},
				},
			},
			expectedErr: errAutodiscoverAndNamedConfigured,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.config.Validate()
			if tc.expectedErr != nil {
				require.ErrorContains(t, err, tc.expectedErr.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestLoadConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	cases := []struct {
		name           string
		expectedConfig component.Config
	}{
		{
			name: "default",
			expectedConfig: &Config{
				Region: "us-west-1",
				Logs: &LogsConfig{
					PollInterval:        time.Minute,
					MaxEventsPerRequest: defaultEventLimit,
					Groups: GroupConfig{
						AutodiscoverConfig: &AutodiscoverConfig{
							Limit: defaultLogGroupLimit,
						},
					},
				},
			},
		},
		{
			name: "prefix-log-group-autodiscover",
			expectedConfig: &Config{
				Region: "us-west-1",
				Logs: &LogsConfig{
					PollInterval:        time.Minute,
					MaxEventsPerRequest: defaultEventLimit,
					Groups: GroupConfig{
						AutodiscoverConfig: &AutodiscoverConfig{
							Limit:  100,
							Prefix: "/aws/eks/",
						},
					},
				},
			},
		},
		{
			name: "autodiscover-filter-streams",
			expectedConfig: &Config{
				Region: "us-west-1",
				Logs: &LogsConfig{
					PollInterval:        time.Minute,
					MaxEventsPerRequest: defaultEventLimit,
					Groups: GroupConfig{
						AutodiscoverConfig: &AutodiscoverConfig{
							Limit: 100,
							Streams: StreamConfig{
								Prefixes: []*string{aws.String("kube-api-controller")},
							},
						},
					},
				},
			},
		},
		{
			name: "autodiscover-filter-streams",
			expectedConfig: &Config{
				Region: "us-west-1",
				Logs: &LogsConfig{
					PollInterval:        time.Minute,
					MaxEventsPerRequest: defaultEventLimit,
					Groups: GroupConfig{
						AutodiscoverConfig: &AutodiscoverConfig{
							Limit: 100,
							Streams: StreamConfig{
								Prefixes: []*string{aws.String("kube-api-controller")},
							},
						},
					},
				},
			},
		},
		{
			name: "named-prefix",
			expectedConfig: &Config{
				Profile: "my-profile",
				Region:  "us-west-1",
				Logs: &LogsConfig{
					PollInterval:        5 * time.Minute,
					MaxEventsPerRequest: defaultEventLimit,
					Groups: GroupConfig{
						NamedConfigs: map[string]StreamConfig{
							"/aws/eks/dev-0/cluster": {},
						},
					},
				},
			},
		},
		{
			name: "named-prefix-with-streams",
			expectedConfig: &Config{
				Profile: "my-profile",
				Region:  "us-west-1",
				Logs: &LogsConfig{
					PollInterval:        5 * time.Minute,
					MaxEventsPerRequest: defaultEventLimit,
					Groups: GroupConfig{
						NamedConfigs: map[string]StreamConfig{
							"/aws/eks/dev-0/cluster": {
								Names: []*string{aws.String("kube-apiserver-ea9c831555adca1815ae04b87661klasdj")},
							},
						},
					},
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			loaded, err := cm.Sub(component.NewIDWithName(typeStr, tc.name).String())
			require.NoError(t, err)
			require.NoError(t, component.UnmarshalConfig(loaded, cfg))
			require.Equal(t, cfg, tc.expectedConfig)
			require.NoError(t, component.ValidateConfig(cfg))
		})
	}
}
