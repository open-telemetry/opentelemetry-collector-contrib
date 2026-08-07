// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkareceiver

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/kafka/configkafka"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkareceiver/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id          component.ID
		expected    component.Config
		expectedErr string
	}{
		{
			id: component.NewIDWithName(metadata.Type, "logs"),
			expected: &Config{
				ClientConfig: func() configkafka.ClientConfig {
					config := configkafka.NewDefaultClientConfig()
					config.Brokers = []string{"coffee:123", "foobar:456"}
					config.Metadata.Retry.Max = 10
					config.Metadata.Retry.Backoff = 5 * time.Second
					config.Authentication.SASL = &configkafka.SASLConfig{
						Mechanism: "PLAIN",
						Username:  "user",
						Password:  "password",
					}
					config.TLS = &configtls.ClientConfig{
						Config: configtls.Config{
							CAFile:   "ca.pem",
							CertFile: "cert.pem",
							KeyFile:  "key.pem",
						},
					}
					return config
				}(),
				ConsumerConfig: func() configkafka.ConsumerConfig {
					config := configkafka.NewDefaultConsumerConfig()
					config.InitialOffset = configkafka.EarliestOffset
					config.SessionTimeout = 45 * time.Second
					config.HeartbeatInterval = 15 * time.Second
					return config
				}(),
				Logs: TopicEncodingConfig{
					Topics:   []string{"logs"},
					Encoding: "direct",
				},
				Metrics: TopicEncodingConfig{
					Topics:   []string{"otlp_metrics"},
					Encoding: "otlp_proto",
				},
				Traces: TopicEncodingConfig{
					Topics:   []string{"otlp_spans"},
					Encoding: "otlp_proto",
				},
				Profiles: TopicEncodingConfig{
					Topics:   []string{"otlp_profiles"},
					Encoding: "otlp_proto",
				},
				ErrorBackOff: configretry.BackOffConfig{
					Enabled:         true,
					InitialInterval: 1 * time.Second,
					MaxInterval:     10 * time.Second,
					MaxElapsedTime:  1 * time.Minute,
					Multiplier:      1.5,
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "rebalance_strategy"),
			expected: &Config{
				ClientConfig: configkafka.NewDefaultClientConfig(),
				ConsumerConfig: func() configkafka.ConsumerConfig {
					config := configkafka.NewDefaultConsumerConfig()
					config.GroupRebalanceStrategy = "sticky"
					config.GroupInstanceID = "test-instance"
					return config
				}(),
				Logs: TopicEncodingConfig{
					Topics:   []string{"otlp_logs"},
					Encoding: "otlp_proto",
				},
				Metrics: TopicEncodingConfig{
					Topics:   []string{"otlp_metrics"},
					Encoding: "otlp_proto",
				},
				Traces: TopicEncodingConfig{
					Topics:   []string{"otlp_spans"},
					Encoding: "otlp_proto",
				},
				Profiles: TopicEncodingConfig{
					Topics:   []string{"otlp_profiles"},
					Encoding: "otlp_proto",
				},
				ErrorBackOff: configretry.BackOffConfig{
					Enabled: false,
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "rebalance_strategies"),
			expected: &Config{
				ClientConfig: configkafka.NewDefaultClientConfig(),
				ConsumerConfig: func() configkafka.ConsumerConfig {
					config := configkafka.NewDefaultConsumerConfig()
					config.GroupRebalanceStrategies = []configkafka.GroupRebalanceStrategy{
						configkafka.CooperativeStickyBalanceStrategy,
						"my_balancer",
					}
					return config
				}(),
				Logs: TopicEncodingConfig{
					Topics:   []string{"otlp_logs"},
					Encoding: "otlp_proto",
				},
				Metrics: TopicEncodingConfig{
					Topics:   []string{"otlp_metrics"},
					Encoding: "otlp_proto",
				},
				Traces: TopicEncodingConfig{
					Topics:   []string{"otlp_spans"},
					Encoding: "otlp_proto",
				},
				Profiles: TopicEncodingConfig{
					Topics:   []string{"otlp_profiles"},
					Encoding: "otlp_proto",
				},
				ErrorBackOff: configretry.BackOffConfig{
					Enabled: false,
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "message_marking"),
			expected: &Config{
				ClientConfig:   configkafka.NewDefaultClientConfig(),
				ConsumerConfig: configkafka.NewDefaultConsumerConfig(),
				Logs: TopicEncodingConfig{
					Topics:   []string{"otlp_logs"},
					Encoding: "otlp_proto",
				},
				Metrics: TopicEncodingConfig{
					Topics:   []string{"otlp_metrics"},
					Encoding: "otlp_proto",
				},
				Traces: TopicEncodingConfig{
					Topics:   []string{"otlp_spans"},
					Encoding: "otlp_proto",
				},
				Profiles: TopicEncodingConfig{
					Topics:   []string{"otlp_profiles"},
					Encoding: "otlp_proto",
				},
				MessageMarking: MessageMarking{
					After:            true,
					OnError:          true,
					OnPermanentError: false,
				},
				ErrorBackOff: configretry.BackOffConfig{
					Enabled: false,
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "message_marking_not_specified"),
			expected: &Config{
				ClientConfig:   configkafka.NewDefaultClientConfig(),
				ConsumerConfig: configkafka.NewDefaultConsumerConfig(),
				Logs: TopicEncodingConfig{
					Topics:   []string{"otlp_logs"},
					Encoding: "otlp_proto",
				},
				Metrics: TopicEncodingConfig{
					Topics:   []string{"otlp_metrics"},
					Encoding: "otlp_proto",
				},
				Traces: TopicEncodingConfig{
					Topics:   []string{"otlp_spans"},
					Encoding: "otlp_proto",
				},
				Profiles: TopicEncodingConfig{
					Topics:   []string{"otlp_profiles"},
					Encoding: "otlp_proto",
				},
				MessageMarking: MessageMarking{
					After:            false,
					OnError:          false,
					OnPermanentError: false,
				},
				ErrorBackOff: configretry.BackOffConfig{
					Enabled: false,
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "message_marking_on_permanent_error_inherited"),
			expected: &Config{
				ClientConfig:   configkafka.NewDefaultClientConfig(),
				ConsumerConfig: configkafka.NewDefaultConsumerConfig(),
				Logs: TopicEncodingConfig{
					Topics:   []string{"otlp_logs"},
					Encoding: "otlp_proto",
				},
				Metrics: TopicEncodingConfig{
					Topics:   []string{"otlp_metrics"},
					Encoding: "otlp_proto",
				},
				Traces: TopicEncodingConfig{
					Topics:   []string{"otlp_spans"},
					Encoding: "otlp_proto",
				},
				Profiles: TopicEncodingConfig{
					Topics:   []string{"otlp_profiles"},
					Encoding: "otlp_proto",
				},
				MessageMarking: MessageMarking{
					After:            true,
					OnError:          true,
					OnPermanentError: true,
				},
				ErrorBackOff: configretry.BackOffConfig{
					Enabled: false,
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "regex_topic_with_exclusion"),
			expected: &Config{
				ClientConfig:   configkafka.NewDefaultClientConfig(),
				ConsumerConfig: configkafka.NewDefaultConsumerConfig(),
				Logs: TopicEncodingConfig{
					Topics:        []string{"^logs-.*"},
					ExcludeTopics: []string{"^logs-(test|dev)$"},
					Encoding:      "otlp_proto",
				},
				Metrics: TopicEncodingConfig{
					Topics:        []string{"^metrics-.*"},
					ExcludeTopics: []string{"^metrics-internal-.*$"},
					Encoding:      "otlp_proto",
				},
				Traces: TopicEncodingConfig{
					Topics:        []string{"^traces-.*"},
					ExcludeTopics: []string{"^traces-debug-.*$"},
					Encoding:      "otlp_proto",
				},
				Profiles: TopicEncodingConfig{
					Topics:   []string{"otlp_profiles"},
					Encoding: "otlp_proto",
				},
				ErrorBackOff: configretry.BackOffConfig{
					Enabled: false,
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "conn_idle_timeout"),
			expected: &Config{
				ClientConfig: func() configkafka.ClientConfig {
					config := configkafka.NewDefaultClientConfig()
					config.ConnIdleTimeout = 5 * time.Minute
					return config
				}(),
				ConsumerConfig: configkafka.NewDefaultConsumerConfig(),
				Logs: TopicEncodingConfig{
					Topics:   []string{"otlp_logs"},
					Encoding: "otlp_proto",
				},
				Metrics: TopicEncodingConfig{
					Topics:   []string{"otlp_metrics"},
					Encoding: "otlp_proto",
				},
				Traces: TopicEncodingConfig{
					Topics:   []string{"otlp_spans"},
					Encoding: "otlp_proto",
				},
				Profiles: TopicEncodingConfig{
					Topics:   []string{"otlp_profiles"},
					Encoding: "otlp_proto",
				},
				ErrorBackOff: configretry.BackOffConfig{
					Enabled: false,
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "partition_processing"),
			expected: &Config{
				ClientConfig:   configkafka.NewDefaultClientConfig(),
				ConsumerConfig: configkafka.NewDefaultConsumerConfig(),
				Logs: TopicEncodingConfig{
					Topics:   []string{"otlp_logs"},
					Encoding: "otlp_proto",
				},
				Metrics: TopicEncodingConfig{
					Topics:   []string{"otlp_metrics"},
					Encoding: "otlp_proto",
				},
				Traces: TopicEncodingConfig{
					Topics:   []string{"otlp_spans"},
					Encoding: "otlp_proto",
				},
				Profiles: TopicEncodingConfig{
					Topics:   []string{"otlp_profiles"},
					Encoding: "otlp_proto",
				},
				PartitionProcessing: PartitionProcessing{
					Independent:        true,
					MaxBufferedBatches: 2,
				},
				ErrorBackOff: configretry.BackOffConfig{
					Enabled: false,
				},
			},
		},
		{
			id:          component.NewIDWithName(metadata.Type, "invalid_exclude_topics_logs_non_regex"),
			expectedErr: "logs.exclude_topics is configured but none of the configured logs.topics use regex pattern (must start with '^')",
		},
		{
			id:          component.NewIDWithName(metadata.Type, "invalid_exclude_topics_metrics_non_regex"),
			expectedErr: "metrics.exclude_topics is configured but none of the configured metrics.topics use regex pattern (must start with '^')",
		},
		{
			id:          component.NewIDWithName(metadata.Type, "invalid_exclude_topics_traces_non_regex"),
			expectedErr: "traces.exclude_topics is configured but none of the configured traces.topics use regex pattern (must start with '^')",
		},
		{
			id:          component.NewIDWithName(metadata.Type, "invalid_exclude_topics_profiles_non_regex"),
			expectedErr: "profiles.exclude_topics is configured but none of the configured profiles.topics use regex pattern (must start with '^')",
		},
		{
			id:          component.NewIDWithName(metadata.Type, "invalid_exclude_topics_regex"),
			expectedErr: "logs.exclude_topic contains invalid regex pattern",
		},
		{
			id:          component.NewIDWithName(metadata.Type, "invalid_exclude_topics_logs_empty"),
			expectedErr: "logs.exclude_topics contains empty string",
		},
		{
			id:          component.NewIDWithName(metadata.Type, "invalid_exclude_topics_metrics_empty"),
			expectedErr: "metrics.exclude_topics contains empty string",
		},
		{
			id:          component.NewIDWithName(metadata.Type, "invalid_exclude_topics_traces_empty"),
			expectedErr: "traces.exclude_topics contains empty string",
		},
		{
			id:          component.NewIDWithName(metadata.Type, "invalid_exclude_topics_profiles_empty"),
			expectedErr: "profiles.exclude_topics contains empty string",
		},
		{
			id:          component.NewIDWithName(metadata.Type, "invalid_partition_processing_zero_buffered_batches"),
			expectedErr: "partition_processing.max_buffered_batches must be greater than zero",
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			err = confmap.Validate(cfg)
			if tt.expectedErr != "" {
				require.ErrorContains(t, err, tt.expectedErr)
				return
			}

			require.NoError(t, err)
			expected := tt.expected.(*Config)
			if expected.PartitionProcessing.MaxBufferedBatches == 0 {
				expected.PartitionProcessing = PartitionProcessing{
					MaxBufferedBatches: 1,
				}
			}
			require.Equal(t, tt.expected, cfg)
		})
	}
}
