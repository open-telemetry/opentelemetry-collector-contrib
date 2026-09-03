// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkareceiver

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/kafka/configkafka"
)

func loadConfig(t *testing.T, name string) (*Config, error) {
	t.Helper()
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	cfg := NewFactory().CreateDefaultConfig()
	sub, err := cm.Sub(name)
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))
	return cfg.(*Config), confmap.Validate(cfg)
}

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		expected    *Config
		expectedErr string
	}{
		{
			name: "kafka/logs",
			expected: func() *Config {
				cfg := NewFactory().CreateDefaultConfig().(*Config)
				cfg.ClientConfig.Brokers = []string{"coffee:123", "foobar:456"}
				cfg.ClientConfig.Metadata.Retry.Max = 10
				cfg.ClientConfig.Metadata.Retry.Backoff = 5 * time.Second
				cfg.ClientConfig.Authentication.SASL = &configkafka.SASLConfig{
					Mechanism: "PLAIN",
					Username:  "user",
					Password:  "password",
				}
				cfg.ClientConfig.TLS = &configtls.ClientConfig{
					Config: configtls.Config{
						CAFile:   "ca.pem",
						CertFile: "cert.pem",
						KeyFile:  "key.pem",
					},
				}
				cfg.ConsumerConfig.InitialOffset = configkafka.EarliestOffset
				cfg.ConsumerConfig.SessionTimeout = 45 * time.Second
				cfg.ConsumerConfig.HeartbeatInterval = 15 * time.Second
				cfg.Logs = TopicEncodingConfig{
					Topics:   []string{"logs"},
					Encoding: "direct",
				}
				cfg.ErrorBackOff = configretry.BackOffConfig{
					Enabled:         true,
					InitialInterval: 1 * time.Second,
					MaxInterval:     10 * time.Second,
					MaxElapsedTime:  1 * time.Minute,
					Multiplier:      1.5,
				}
				return cfg
			}(),
		},
		{
			name: "kafka/rebalance_strategies",
			expected: func() *Config {
				cfg := NewFactory().CreateDefaultConfig().(*Config)
				cfg.ConsumerConfig.GroupRebalanceStrategies = []configkafka.GroupRebalanceStrategy{
					configkafka.CooperativeStickyBalanceStrategy,
					"my_balancer",
				}
				return cfg
			}(),
		},
		{
			name: "kafka/message_marking",
			expected: func() *Config {
				cfg := NewFactory().CreateDefaultConfig().(*Config)
				cfg.MessageMarking = MessageMarking{
					After:            true,
					OnError:          true,
					OnPermanentError: false,
				}
				return cfg
			}(),
		},
		{
			name:     "kafka/message_marking_not_specified",
			expected: NewFactory().CreateDefaultConfig().(*Config),
		},
		{
			name: "kafka/message_marking_on_permanent_error_inherited",
			expected: func() *Config {
				cfg := NewFactory().CreateDefaultConfig().(*Config)
				cfg.MessageMarking = MessageMarking{
					After:            true,
					OnError:          true,
					OnPermanentError: true,
				}
				return cfg
			}(),
		},
		{
			name: "kafka/regex_topic_with_exclusion",
			expected: func() *Config {
				cfg := NewFactory().CreateDefaultConfig().(*Config)
				cfg.Logs = TopicEncodingConfig{
					Topics:        []string{"^logs-.*"},
					ExcludeTopics: []string{"^logs-(test|dev)$"},
					Encoding:      "otlp_proto",
				}
				cfg.Metrics = TopicEncodingConfig{
					Topics:        []string{"^metrics-.*"},
					ExcludeTopics: []string{"^metrics-internal-.*$"},
					Encoding:      "otlp_proto",
				}
				cfg.Traces = TopicEncodingConfig{
					Topics:        []string{"^traces-.*"},
					ExcludeTopics: []string{"^traces-debug-.*$"},
					Encoding:      "otlp_proto",
				}
				return cfg
			}(),
		},
		{
			name: "kafka/conn_idle_timeout",
			expected: func() *Config {
				cfg := NewFactory().CreateDefaultConfig().(*Config)
				cfg.ClientConfig.ConnIdleTimeout = 5 * time.Minute
				return cfg
			}(),
		},
		{
			name: "kafka/valid_exclude_topics_logs",
			expected: func() *Config {
				cfg := NewFactory().CreateDefaultConfig().(*Config)
				cfg.Logs = TopicEncodingConfig{
					Topics:        []string{"^logs-.*"},
					ExcludeTopics: []string{"^logs-test$"},
					Encoding:      "otlp_proto",
				}
				return cfg
			}(),
		},
		{
			name: "kafka/valid_logs_without_exclude",
			expected: func() *Config {
				cfg := NewFactory().CreateDefaultConfig().(*Config)
				cfg.Logs = TopicEncodingConfig{
					Topics:   []string{"logs"},
					Encoding: "otlp_proto",
				}
				return cfg
			}(),
		},
		{
			name: "kafka/partition_processing",
			expected: func() *Config {
				cfg := NewFactory().CreateDefaultConfig().(*Config)
				cfg.PartitionProcessing = PartitionProcessing{
					Independent:        true,
					MaxBufferedBatches: 2,
				}
				return cfg
			}(),
		},
		{
			name:        "kafka/invalid_exclude_topics_logs_non_regex",
			expectedErr: "logs.exclude_topics is configured but none of the configured logs.topics use regex pattern (must start with '^')",
		},
		{
			name:        "kafka/invalid_exclude_topics_metrics_non_regex",
			expectedErr: "metrics.exclude_topics is configured but none of the configured metrics.topics use regex pattern (must start with '^')",
		},
		{
			name:        "kafka/invalid_exclude_topics_traces_non_regex",
			expectedErr: "traces.exclude_topics is configured but none of the configured traces.topics use regex pattern (must start with '^')",
		},
		{
			name:        "kafka/invalid_exclude_topics_profiles_non_regex",
			expectedErr: "profiles.exclude_topics is configured but none of the configured profiles.topics use regex pattern (must start with '^')",
		},
		{
			name:        "kafka/invalid_exclude_topics_regex",
			expectedErr: "logs.exclude_topic contains invalid regex pattern",
		},
		{
			name:        "kafka/invalid_exclude_topics_logs_empty",
			expectedErr: "logs.exclude_topics contains empty string",
		},
		{
			name:        "kafka/invalid_exclude_topics_metrics_empty",
			expectedErr: "metrics.exclude_topics contains empty string",
		},
		{
			name:        "kafka/invalid_exclude_topics_traces_empty",
			expectedErr: "traces.exclude_topics contains empty string",
		},
		{
			name:        "kafka/invalid_exclude_topics_profiles_empty",
			expectedErr: "profiles.exclude_topics contains empty string",
		},
		{
			name:        "kafka/invalid_partition_processing_zero_buffered_batches",
			expectedErr: "partition_processing.max_buffered_batches must be greater than zero",
		},
		{
			name:        "kafka/invalid_partition_processing_manual_commit",
			expectedErr: "partition_processing.independent requires autocommit.enable",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := loadConfig(t, tc.name)
			if tc.expectedErr != "" {
				require.ErrorContains(t, err, tc.expectedErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.expected, cfg)
		})
	}
}
