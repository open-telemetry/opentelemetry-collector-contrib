// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscloudwatchreceiver

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscloudwatchreceiver/internal/metadata"
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
				Logs: LogsConfig{
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
				Logs: LogsConfig{
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
				Logs: LogsConfig{
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
				Logs: LogsConfig{
					MaxEventsPerRequest: defaultEventLimit,
					PollInterval:        defaultPollInterval,
					Groups: GroupConfig{
						AutodiscoverConfig: &AutodiscoverConfig{
							Limit: -10000,
						},
					},
				},
			},
			expectedErr: errInvalidAutodiscoverLimit,
		},
		{
			name: "Invalid Log Group Prefix And Pattern",
			config: Config{
				Region: "us-east-1",
				Logs: LogsConfig{
					MaxEventsPerRequest: defaultEventLimit,
					PollInterval:        defaultPollInterval,
					Groups: GroupConfig{
						AutodiscoverConfig: &AutodiscoverConfig{
							Limit:   defaultLogGroupLimit,
							Prefix:  "/aws/eks",
							Pattern: "eks",
						},
					},
				},
			},
			expectedErr: errPrefixAndPatternConfigured,
		},
		{
			name: "Invalid IMDS Endpoint",
			config: Config{
				Region:       "us-east-1",
				IMDSEndpoint: "xyz",
				Logs: LogsConfig{
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
				Logs: LogsConfig{
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

func TestLoadLogsConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	// defaultMetrics is the metrics config that results from createDefaultConfig() when no metrics
	// section is present in the YAML. Period, Delay, and CollectionInterval are set to their defaults.
	defaultMetrics := createDefaultConfig().(*Config).Metrics

	cases := []struct {
		name           string
		expectedConfig component.Config
	}{
		{
			name: "default",
			expectedConfig: &Config{
				Region:  "us-west-1",
				Metrics: defaultMetrics,
				Logs: LogsConfig{
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
				Region:  "us-west-1",
				Metrics: defaultMetrics,
				Logs: LogsConfig{
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
			name: "name-pattern-log-group-autodiscover",
			expectedConfig: &Config{
				Region:  "us-west-1",
				Metrics: defaultMetrics,
				Logs: LogsConfig{
					PollInterval:        time.Minute,
					MaxEventsPerRequest: defaultEventLimit,
					Groups: GroupConfig{
						AutodiscoverConfig: &AutodiscoverConfig{
							Limit:   100,
							Pattern: "eks",
						},
					},
				},
			},
		},
		{
			name: "autodiscover-filter-streams",
			expectedConfig: &Config{
				Region:  "us-west-1",
				Metrics: defaultMetrics,
				Logs: LogsConfig{
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
				Metrics: defaultMetrics,
				Logs: LogsConfig{
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
				Metrics: defaultMetrics,
				Logs: LogsConfig{
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

			loaded, err := cm.Sub(component.NewIDWithName(metadata.Type, tc.name).String())
			require.NoError(t, err)
			require.NoError(t, loaded.Unmarshal(cfg))
			require.Equal(t, tc.expectedConfig, cfg)
			require.NoError(t, xconfmap.Validate(cfg))
		})
	}
}

func TestLoadMetricsConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	defaultLogs := func() LogsConfig {
		return createDefaultConfig().(*Config).Logs
	}

	cases := []struct {
		name           string
		expectedConfig component.Config
	}{
		{
			name: "metrics-explicit",
			expectedConfig: &Config{
				Region: "us-east-1",
				Logs:   defaultLogs(),
				Metrics: MetricsConfig{
					ControllerConfig: scraperhelper.ControllerConfig{CollectionInterval: time.Minute, InitialDelay: time.Second},
					Period:           60 * time.Second,
					Delay:            defaultMetricsDelay,
					Queries: []MetricQuery{
						{
							Namespace:  "AWS/EC2",
							MetricName: "CPUUtilization",
							Dimensions: map[string]string{"InstanceId": "i-1234567890abcdef0"},
						},
						{
							Namespace:  "AWS/EC2",
							MetricName: "NetworkIn",
						},
					},
				},
			},
		},
		{
			name: "metrics-discovery",
			expectedConfig: &Config{
				Region: "us-east-1",
				Logs:   defaultLogs(),
				Metrics: MetricsConfig{
					ControllerConfig: scraperhelper.ControllerConfig{CollectionInterval: 5 * time.Minute, InitialDelay: time.Second},
					Period:           300 * time.Second,
					Delay:            defaultMetricsDelay,
					Discovery: &MetricsDiscoveryConfig{
						Filters: configoptional.Some(MetricsDiscoveryFilters{Namespace: "AWS/EC2"}),
						Limit:   200,
					},
				},
			},
		},
		{
			name: "metrics-explicit-gauge",
			expectedConfig: &Config{
				Region: "us-east-1",
				Logs:   defaultLogs(),
				Metrics: MetricsConfig{
					ControllerConfig: scraperhelper.ControllerConfig{CollectionInterval: time.Minute, InitialDelay: time.Second},
					Period:           60 * time.Second,
					Delay:            defaultMetricsDelay,
					Queries: []MetricQuery{
						{
							Namespace:  "AWS/EC2",
							MetricName: "CPUUtilization",
							Stats:      []string{"Average", "p99"},
						},
					},
				},
			},
		},
		{
			name: "metrics-discovery-gauge",
			expectedConfig: &Config{
				Region: "us-east-1",
				Logs:   defaultLogs(),
				Metrics: MetricsConfig{
					ControllerConfig: scraperhelper.ControllerConfig{CollectionInterval: 5 * time.Minute, InitialDelay: time.Second},
					Period:           300 * time.Second,
					Delay:            defaultMetricsDelay,
					Discovery: &MetricsDiscoveryConfig{
						Filters: configoptional.Some(MetricsDiscoveryFilters{Namespace: "AWS/EC2"}),
						Limit:   100,
						Stats:   []string{"Sum", "Average"},
					},
				},
			},
		},
		{
			name: "metrics-discovery-no-namespace",
			expectedConfig: &Config{
				Region: "us-west-2",
				Logs:   defaultLogs(),
				Metrics: MetricsConfig{
					ControllerConfig: scraperhelper.ControllerConfig{CollectionInterval: 2 * time.Minute, InitialDelay: time.Second},
					Period:           120 * time.Second,
					Delay:            defaultMetricsDelay,
					Discovery: &MetricsDiscoveryConfig{
						Limit: 50,
					},
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			loaded, err := cm.Sub(component.NewIDWithName(metadata.Type, tc.name).String())
			require.NoError(t, err)
			require.NoError(t, loaded.Unmarshal(cfg))
			require.Equal(t, tc.expectedConfig, cfg)
			require.NoError(t, xconfmap.Validate(cfg))
		})
	}
}

func TestValidateMetricsConfig(t *testing.T) {
	defaultLogs := createDefaultConfig().(*Config).Logs

	validBase := func() Config {
		return Config{
			Region: "us-east-1",
			Logs:   defaultLogs,
		}
	}

	withMetrics := func(m MetricsConfig) Config {
		c := validBase()
		c.Metrics = m
		return c
	}

	cases := []struct {
		name        string
		config      Config
		expectedErr error
	}{
		{
			name:   "no metrics config is valid",
			config: validBase(),
		},
		{
			name: "explicit metrics valid",
			config: withMetrics(MetricsConfig{
				Period: 60 * time.Second,
				Queries: []MetricQuery{
					{Namespace: "AWS/EC2", MetricName: "CPUUtilization"},
				},
			}),
		},
		{
			name: "discovery valid",
			config: withMetrics(MetricsConfig{
				Period:    300 * time.Second,
				Discovery: &MetricsDiscoveryConfig{Limit: 100},
			}),
		},
		{
			name: "metrics and discovery mutually exclusive",
			config: withMetrics(MetricsConfig{
				Queries:   []MetricQuery{{Namespace: "AWS/EC2", MetricName: "CPUUtilization"}},
				Discovery: &MetricsDiscoveryConfig{Limit: 10},
			}),
			expectedErr: errMetricsAndDiscoveryConfigured,
		},
		{
			name: "discovery limit zero",
			config: withMetrics(MetricsConfig{
				Discovery: &MetricsDiscoveryConfig{Limit: 0},
			}),
			expectedErr: errInvalidDiscoveryLimit,
		},
		{
			name: "collection interval too short",
			config: withMetrics(MetricsConfig{
				ControllerConfig: scraperhelper.ControllerConfig{CollectionInterval: 500 * time.Millisecond},
				Queries:          []MetricQuery{{Namespace: "AWS/EC2", MetricName: "CPUUtilization"}},
			}),
			expectedErr: errInvalidMetricsCollectionInterval,
		},
		{
			name: "period too short",
			config: withMetrics(MetricsConfig{
				Period:  500 * time.Millisecond,
				Queries: []MetricQuery{{Namespace: "AWS/EC2", MetricName: "CPUUtilization"}},
			}),
			expectedErr: errInvalidMetricsPeriod,
		},
		{
			name: "delay too short",
			config: withMetrics(MetricsConfig{
				Delay:   500 * time.Millisecond,
				Queries: []MetricQuery{{Namespace: "AWS/EC2", MetricName: "CPUUtilization"}},
			}),
			expectedErr: errInvalidMetricsDelay,
		},
		{
			name: "delay too short in discovery",
			config: withMetrics(MetricsConfig{
				Delay:     500 * time.Millisecond,
				Discovery: &MetricsDiscoveryConfig{Limit: 100},
			}),
			expectedErr: errInvalidMetricsDelay,
		},
		{
			name: "collection_interval less than period",
			config: withMetrics(MetricsConfig{
				ControllerConfig: scraperhelper.ControllerConfig{CollectionInterval: 1 * time.Minute},
				Period:           5 * time.Minute,
				Queries:          []MetricQuery{{Namespace: "AWS/EC2", MetricName: "CPUUtilization"}},
			}),
			expectedErr: errCollectionIntervalLessThanPeriod,
		},
		{
			name: "metric missing namespace",
			config: withMetrics(MetricsConfig{
				Queries: []MetricQuery{{MetricName: "CPUUtilization"}},
			}),
			expectedErr: errMetricMissingNamespace,
		},
		{
			name: "metric missing name",
			config: withMetrics(MetricsConfig{
				Queries: []MetricQuery{{Namespace: "AWS/EC2"}},
			}),
			expectedErr: errMetricMissingName,
		},
		{
			name: "period zero uses default (valid)",
			config: withMetrics(MetricsConfig{
				Period:  0,
				Queries: []MetricQuery{{Namespace: "AWS/EC2", MetricName: "CPUUtilization"}},
			}),
		},
		{
			name: "collection interval zero uses default (valid)",
			config: withMetrics(MetricsConfig{
				ControllerConfig: scraperhelper.ControllerConfig{CollectionInterval: 0},
				Queries:          []MetricQuery{{Namespace: "AWS/EC2", MetricName: "CPUUtilization"}},
			}),
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
