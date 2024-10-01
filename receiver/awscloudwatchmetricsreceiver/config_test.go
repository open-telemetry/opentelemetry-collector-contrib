// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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
							Name:  "InstanceId",
							Value: " i-1234567890abcdef0",
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
						AwsAggregation: "Last",
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
				IMDSEndpoint: "xyz",
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
							Name:  ":BucketName",
							Value: "open-telemetry",
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
