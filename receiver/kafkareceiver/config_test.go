// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkareceiver

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"

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
		expectedErr error
	}{
		{
			id: component.NewIDWithName(metadata.Type, "legacy_topic"),
			expected: &Config{
				ClientConfig: func() configkafka.ClientConfig {
					config := configkafka.NewDefaultClientConfig()
					return config
				}(),
				ConsumerConfig: func() configkafka.ConsumerConfig {
					config := configkafka.NewDefaultConsumerConfig()
					return config
				}(),
				Logs: TopicEncodingConfig{
					// if deprecated topic is set and topics is not set
					// give precedence to topic
					topicAlias: "legacy_logs",
					Topics:     []string{"legacy_logs"},
					Encoding:   "otlp_proto",
				},
				Metrics: TopicEncodingConfig{
					topicAlias: "legacy_metric",
					Topics:     []string{"legacy_metric"},
					Encoding:   "otlp_proto",
				},
				Traces: TopicEncodingConfig{
					topicAlias: "legacy_spans",
					Topics:     []string{"legacy_spans"},
					Encoding:   "otlp_proto",
				},
				Profiles: TopicEncodingConfig{
					topicAlias: "legacy_profile",
					Topics:     []string{"legacy_profile"},
					Encoding:   "otlp_proto",
				},
				ErrorBackOff: configretry.BackOffConfig{
					Enabled: false,
				},
			},
		},
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
					Topics:   []string{"logs"}, // topics is given precedence if it is set
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
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			assert.NoError(t, xconfmap.Validate(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		expectedErr string
	}{
		{
			name: "valid config with regex and exclude_topic",
			config: &Config{
				Logs: TopicEncodingConfig{
					Topics:        []string{"^logs-.*"},
					ExcludeTopics: []string{"^logs-test$"},
					Encoding:      "otlp_proto",
				},
			},
			expectedErr: "",
		},
		{
			name: "invalid config with non-regex topic and exclude_topic for logs",
			config: &Config{
				Logs: TopicEncodingConfig{
					Topics:        []string{"logs"},
					ExcludeTopics: []string{"^logs-test$"},
					Encoding:      "otlp_proto",
				},
			},
			expectedErr: "logs.exclude_topics is configured but none of the configured logs.topics use regex pattern (must start with '^')",
		},
		{
			name: "invalid config with non-regex topic and exclude_topic for metrics",
			config: &Config{
				Metrics: TopicEncodingConfig{
					Topics:        []string{"metrics"},
					ExcludeTopics: []string{"^metrics-test$"},
					Encoding:      "otlp_proto",
				},
			},
			expectedErr: "metrics.exclude_topics is configured but none of the configured metrics.topics use regex pattern (must start with '^')",
		},
		{
			name: "invalid config with non-regex topic and exclude_topic for traces",
			config: &Config{
				Traces: TopicEncodingConfig{
					Topics:        []string{"traces"},
					ExcludeTopics: []string{"^traces-test$"},
					Encoding:      "otlp_proto",
				},
			},
			expectedErr: "traces.exclude_topics is configured but none of the configured traces.topics use regex pattern (must start with '^')",
		},
		{
			name: "invalid config when both topic and topics are set",
			config: &Config{
				Logs: TopicEncodingConfig{
					Topic:    "legacy_log",
					Topics:   []string{"logs"},
					Encoding: "otlp_proto",
				},
			},
			expectedErr: "both logs.topic and logs.topics cannot be set",
		},
		{
			name: "invalid config when both exclude_topic and exclude_topics are set",
			config: &Config{
				Logs: TopicEncodingConfig{
					ExcludeTopic:  "^logs-[invalid(regex",
					ExcludeTopics: []string{"^logs-[invalid(regex"},
					Encoding:      "otlp_proto",
				},
			},
			expectedErr: "both logs.exclude_topic and logs.exclude_topics cannot be set",
		},
		{
			name: "invalid config with non-regex topic and exclude_topic for profiles",
			config: &Config{
				Profiles: TopicEncodingConfig{
					Topics:        []string{"profiles"},
					ExcludeTopics: []string{"^profiles-test$"},
					Encoding:      "otlp_proto",
				},
			},
			expectedErr: "profiles.exclude_topics is configured but none of the configured profiles.topics use regex pattern (must start with '^')",
		},
		{
			name: "valid config without exclude_topic",
			config: &Config{
				Logs: TopicEncodingConfig{
					Topics:   []string{"logs"},
					Encoding: "otlp_proto",
				},
			},
			expectedErr: "",
		},
		{
			name: "invalid config with invalid regex in exclude_topic",
			config: &Config{
				Logs: TopicEncodingConfig{
					Topics:        []string{"^logs-.*"},
					ExcludeTopics: []string{"^logs-[invalid(regex"},
					Encoding:      "otlp_proto",
				},
			},
			expectedErr: "logs.exclude_topic contains invalid regex pattern",
		},
		{
			name: "invalid config with empty string in exclude_topics for logs",
			config: &Config{
				Logs: TopicEncodingConfig{
					Topics:        []string{"^logs-.*"},
					ExcludeTopics: []string{""},
					Encoding:      "otlp_proto",
				},
			},
			expectedErr: "logs.exclude_topics contains empty string",
		},
		{
			name: "invalid config with empty string in exclude_topics for metrics",
			config: &Config{
				Metrics: TopicEncodingConfig{
					Topics:        []string{"^metrics-.*"},
					ExcludeTopics: []string{"", "^metrics-test$"},
					Encoding:      "otlp_proto",
				},
			},
			expectedErr: "metrics.exclude_topics contains empty string",
		},
		{
			name: "invalid config with empty string in exclude_topics for traces",
			config: &Config{
				Traces: TopicEncodingConfig{
					Topics:        []string{"^traces-.*"},
					ExcludeTopics: []string{"^traces-test$", ""},
					Encoding:      "otlp_proto",
				},
			},
			expectedErr: "traces.exclude_topics contains empty string",
		},
		{
			name: "invalid config with empty string in exclude_topics for profiles",
			config: &Config{
				Profiles: TopicEncodingConfig{
					Topics:        []string{"^profiles-.*"},
					ExcludeTopics: []string{""},
					Encoding:      "otlp_proto",
				},
			},
			expectedErr: "profiles.exclude_topics contains empty string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectedErr == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.expectedErr)
			}
		})
	}
}
