// Copyright  The OpenTelemetry Authors
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

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config"
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
							Limit: 10000,
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

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(config.NewComponentIDWithName(typeStr, "").String())
	require.NoError(t, err)
	require.NoError(t, config.UnmarshalReceiver(sub, cfg))

	expected := factory.CreateDefaultConfig().(*Config)
	expected.Region = "us-west-1"
	expected.Logs.PollInterval = time.Minute

	require.Equal(t, expected, cfg)
}
