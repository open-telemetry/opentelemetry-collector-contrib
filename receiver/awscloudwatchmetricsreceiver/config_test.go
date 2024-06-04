// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscloudwatchmetricsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscloudwatchmetricsreceiver"

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestConfigValidation(t *testing.T) {
	cases := []struct {
		name        string
		config      Config
		expectedErr error
	}{
		{
			name:        "No metric key configured",
			config:      Config{},
			expectedErr: errNoMetricsConfigured,
		},
		{
			name: "valid autodiscover config",
			config: Config{
				Region:       "us-west-2",
				Profile:      "my_profile",
				PollInterval: defaultPollInterval,
				Metrics: &MetricsConfig{
					AutoDiscover: &AutoDiscoverConfig{
						Namespace:      "AWS/EC2",
						Limit:          20,
						AwsAggregation: "Average",
						Period:         time.Second * 60 * 5,
					},
				},
			},
			expectedErr: nil,
		},
		{
			name: "valid group config",
			config: Config{
				Region:       "us-west-2",
				Profile:      "my_profile",
				PollInterval: defaultPollInterval,
				Metrics: &MetricsConfig{
					Group: []GroupConfig{
						{
							Namespace: "AWS/EC2",
							Period:    time.Second * 60 * 5,
							MetricName: []NamedConfig{
								{
									MetricName:     "CPUUtilization",
									AwsAggregation: "Average",
									Dimensions: []MetricDimensionsConfig{
										{
											Name:  "InstanceId",
											Value: "i-1234567890abcdef0",
										},
									},
								},
							},
						},
					},
				},
			},
			expectedErr: nil,
		},
		{
			name: "No metric_name configured in group config",
			config: Config{
				Region:       "eu-west-2",
				PollInterval: time.Minute * 5,
				Metrics: &MetricsConfig{
					Group: []GroupConfig{
						{
							Namespace: "AWS/EC2",
							Period:    time.Second * 60 * 5,
							MetricName: []NamedConfig{
								{
									AwsAggregation: "Sum",
								},
							},
						},
					},
				},
			},
			expectedErr: errNoMetricNameConfigured,
		},
		{
			name: "No namespace configured",
			config: Config{
				Region:       "eu-west-2",
				PollInterval: defaultPollInterval,
				Metrics: &MetricsConfig{
					Group: []GroupConfig{
						{
							Period: time.Second * 60 * 5,
							MetricName: []NamedConfig{
								{
									MetricName:     "CPUUtilization",
									AwsAggregation: "Average",
									Dimensions: []MetricDimensionsConfig{
										{
											Name:  "InstanceId",
											Value: "i-1234567890abcdef0",
										},
									},
								},
							},
						},
					},
				},
			},
			expectedErr: errNoNamespaceConfigured,
		},
		{
			name: "No region configured",
			config: Config{
				Metrics: &MetricsConfig{
					Group: []GroupConfig{
						{
							Namespace: "AWS/EC2",
							Period:    time.Second * 60 * 5,
							MetricName: []NamedConfig{
								{
									MetricName:     "CPUUtilization",
									AwsAggregation: "Average",
									Dimensions: []MetricDimensionsConfig{
										{
											Name:  "InstanceId",
											Value: "i-1234567890abcdef0",
										},
									},
								},
							},
						},
					},
				},
			},
			expectedErr: errNoRegion,
		},
		{
			name: "Poll interval less than 1 second",
			config: Config{
				Region:       "us-west-2",
				Profile:      "my_profile",
				PollInterval: time.Millisecond * 500,
				Metrics: &MetricsConfig{
					Group: []GroupConfig{
						{
							Namespace: "AWS/EC2",
							Period:    time.Second * 60 * 5,
							MetricName: []NamedConfig{
								{
									MetricName:     "CPUUtilization",
									AwsAggregation: "Average",
									Dimensions: []MetricDimensionsConfig{
										{
											Name:  "InstanceId",
											Value: "i-1234567890abcdef0",
										},
									},
								},
							},
						},
					},
				},
			},
			expectedErr: errInvalidPollInterval,
		},
		{
			name: "Auto discover parameter has limit of 0",
			config: Config{
				Region:       "us-west-2",
				Profile:      "my_profile",
				PollInterval: defaultPollInterval,
				Metrics: &MetricsConfig{
					AutoDiscover: &AutoDiscoverConfig{
						Namespace:      "AWS/EC2",
						Limit:          -1,
						AwsAggregation: "Average",
						Period:         time.Second * 60 * 5,
						DefaultDimensions: []MetricDimensionsConfig{
							{
								Name:  "InstanceId",
								Value: "i-1234567890abcdef0",
							},
						},
					},
				},
			},
			expectedErr: errInvalidAutodiscoverLimit,
		},
		{
			name: "Both group and auto discover parameters are defined",
			config: Config{
				Region:       "us-west-2",
				Profile:      "my_profile",
				PollInterval: defaultPollInterval,
				Metrics: &MetricsConfig{
					Group: []GroupConfig{
						{
							Namespace: "AWS/EC2",
							Period:    time.Second * 60 * 5,
							MetricName: []NamedConfig{
								{
									MetricName:     "CPUUtilization",
									AwsAggregation: "Average",
									Dimensions: []MetricDimensionsConfig{
										{
											Name:  "InstanceId",
											Value: "i-1234567890abcdef0",
										},
									},
								},
							},
						},
					},
					AutoDiscover: &AutoDiscoverConfig{
						Namespace:         "AWS/EC2",
						Limit:             100,
						AwsAggregation:    "Average",
						Period:            time.Second * 60 * 5,
						DefaultDimensions: []MetricDimensionsConfig{},
					},
				},
			},
			expectedErr: errAutodiscoverAndNamedConfigured,
		},
		{
			name: "Name parameter in dimension is empty",
			config: Config{
				Region:       "us-west-2",
				Profile:      "my_profile",
				PollInterval: defaultPollInterval,
				Metrics: &MetricsConfig{
					Group: []GroupConfig{
						{
							Namespace: "AWS/EC2",
							Period:    time.Second * 60 * 5,
							MetricName: []NamedConfig{
								{
									MetricName:     "CPUUtilization",
									AwsAggregation: "Average",
									Dimensions: []MetricDimensionsConfig{
										{
											Name:  "",
											Value: "i-1234567890abcdef0",
										},
									},
								},
							},
						},
					},
				},
			},
			expectedErr: errEmptyDimensions,
		},
		{
			name: "Value parameter in dimension is empty",
			config: Config{
				Region:       "us-west-2",
				Profile:      "my_profile",
				PollInterval: defaultPollInterval,
				Metrics: &MetricsConfig{
					Group: []GroupConfig{
						{
							Namespace: "AWS/EC2",
							Period:    time.Second * 60 * 5,
							MetricName: []NamedConfig{
								{
									MetricName:     "CPUUtilization",
									AwsAggregation: "Average",
									Dimensions: []MetricDimensionsConfig{
										{
											Name:  "InstanceId",
											Value: "",
										},
									},
								},
							},
						},
					},
				},
			},
			expectedErr: errEmptyDimensions,
		},
		{
			name: "Name parameter in dimension starts with a colon",
			config: Config{
				Region:       "us-west-2",
				Profile:      "my_profile",
				PollInterval: defaultPollInterval,
				Metrics: &MetricsConfig{
					Group: []GroupConfig{
						{
							Namespace: "AWS/EC2",
							Period:    time.Second * 60 * 5,
							MetricName: []NamedConfig{
								{
									MetricName:     "CPUUtilization",
									AwsAggregation: "Average",
									Dimensions: []MetricDimensionsConfig{
										{
											Name:  ":InvalidName",
											Value: "i-1234567890abcdef0",
										},
									},
								},
							},
						},
					},
				},
			},
			expectedErr: errDimensionColonPrefix,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.config.Validate()
			if tc.expectedErr != nil {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.expectedErr.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}
