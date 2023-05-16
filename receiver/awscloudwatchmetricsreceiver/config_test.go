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

package awscloudwatchmetricsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscloudwatchmetricsreceiver"

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestValidate(t *testing.T) {
	cases := []struct {
		name        string
		config      Config
		expectedErr error
	}{
		{
			name: "Valid config",
			config: Config{
				Region:       "eu-west-1",
				PollInterval: time.Minute * 5,
				Metrics: &MetricsConfig{
					Names: []*NamedConfig{{
						Namespace:      "AWS/EC2",
						MetricName:     "CPUUtilizaition",
						Period:         time.Second * 60,
						AwsAggregation: "Sum",
						Dimensions: []MetricDimensionsConfig{{
							Name:  "Test",
							Value: "Test",
						}},
					}},
				},
			},
		},
		{
			name: "No metrics defined",
			config: Config{
				Region:       "eu-west-1",
				PollInterval: time.Minute * 5,
			},
			expectedErr: errNoMetricsConfigured,
		},
		{
			name: "No named config defined",
			config: Config{
				Region:       "eu-west-1",
				PollInterval: time.Minute * 5,
				Metrics:      &MetricsConfig{},
			},
			expectedErr: errNoMetricsConfigured,
		},
		{
			name: "No namespace defined",
			config: Config{
				Region:       "eu-west-1",
				PollInterval: time.Minute * 5,
				Metrics: &MetricsConfig{
					Names: []*NamedConfig{{
						Namespace: "",
					}},
				},
			},
			expectedErr: errNoNamespaceConfigured,
		},
		{
			name: "No metric name defined",
			config: Config{
				Region:       "eu-west-1",
				PollInterval: time.Minute * 5,
				Metrics: &MetricsConfig{
					Names: []*NamedConfig{{
						Namespace:      "AWS/EC2",
						MetricName:     "",
						AwsAggregation: "Sum",
					}},
				},
			},
			expectedErr: errNoMetricsConfigured,
		},
		{
			name: "Bad AWS Aggregation",
			config: Config{
				Region:       "eu-west-1",
				PollInterval: time.Minute * 5,
				Metrics: &MetricsConfig{
					Names: []*NamedConfig{{
						Namespace:      "AWS/EC2",
						MetricName:     "CPUUtilizaition",
						Period:         time.Second * 60,
						AwsAggregation: "test",
					}},
				},
			},
			expectedErr: errInvalidAwsAggregation,
		},
		{
			name: "P99 AWS Aggregation",
			config: Config{
				Region:       "eu-west-1",
				PollInterval: time.Minute * 5,
				Metrics: &MetricsConfig{
					Names: []*NamedConfig{{
						Namespace:      "AWS/EC2",
						MetricName:     "CPUUtilizaition",
						Period:         time.Second * 60,
						AwsAggregation: "p99",
					}},
				},
			},
		},
		{
			name: "TS99 AWS Aggregation",
			config: Config{
				Region:       "eu-west-1",
				PollInterval: time.Minute * 5,
				Metrics: &MetricsConfig{
					Names: []*NamedConfig{{
						Namespace:      "AWS/EC2",
						MetricName:     "CPUUtilizaition",
						Period:         time.Second * 60,
						AwsAggregation: "TS99",
					}},
				},
			},
		},
		{
			name: "Multiple Metrics",
			config: Config{
				Region:       "eu-west-1",
				PollInterval: time.Minute * 5,
				Metrics: &MetricsConfig{
					Names: []*NamedConfig{{
						Namespace:      "AWS/EC2",
						MetricName:     "CPUUtilizaition",
						Period:         time.Second * 60,
						AwsAggregation: "TS99",
					},
						{
							Namespace:      "AWS/EC2",
							MetricName:     "CPUUtilizaition",
							Period:         time.Second * 60,
							AwsAggregation: "TS99"},
					},
				},
			},
		},
		{
			name: "Invalid region",
			config: Config{
				Region: "",
			},
			expectedErr: errNoRegion,
		},
		{
			name: "Invalid IMDS endpoint url",
			config: Config{
				Region:       "eu-west-1",
				IMDSEndpoint: "testsijdasidj",
			},
			expectedErr: errors.New("unable to parse URI for imds_endpoint"),
		},
		{
			name: "Polling Interval < 1s",
			config: Config{
				Region:       "eu-west-1",
				PollInterval: time.Millisecond * 500,
			},
			expectedErr: errInvalidPollInterval,
		},
		{
			name: "Invalid dimensions name and value",
			config: Config{
				Region:       "eu-west-1",
				PollInterval: time.Minute * 5,
				Metrics: &MetricsConfig{
					Names: []*NamedConfig{{
						Namespace:      "AWS/EC2",
						MetricName:     "CPUUtilizaition",
						Period:         time.Second * 60,
						AwsAggregation: "Sum",
						Dimensions: []MetricDimensionsConfig{{
							Name:  "",
							Value: "",
						}},
					}},
				},
			},
			expectedErr: errEmptyDimensions,
		},
		{
			name: "Dimension name begins with colon",
			config: Config{
				Region:       "eu-west-1",
				PollInterval: time.Minute * 5,
				Metrics: &MetricsConfig{
					Names: []*NamedConfig{{
						Namespace:      "AWS/EC2",
						MetricName:     "CPUUtilizaition",
						Period:         time.Second * 60,
						AwsAggregation: "Sum",
						Dimensions: []MetricDimensionsConfig{{
							Name:  ":",
							Value: "Test",
						}},
					}},
				},
			},
			expectedErr: errDimensionColonPrefix,
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
