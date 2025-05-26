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
			id: component.NewIDWithName(metadata.Type, ""),
			expected: &Config{
				ClientConfig: func() configkafka.ClientConfig {
					config := configkafka.NewDefaultClientConfig()
					config.Brokers = []string{"foo:123", "bar:456"}
					config.ResolveCanonicalBootstrapServersOnly = true
					config.ClientID = "the_client_id"
					return config
				}(),
				ConsumerConfig: func() configkafka.ConsumerConfig {
					config := configkafka.NewDefaultConsumerConfig()
					config.GroupID = "the_group_id"
					return config
				}(),
				Logs: TopicEncodingConfig{
					Topic:    "spans",
					Encoding: "otlp_proto",
				},
				Metrics: TopicEncodingConfig{
					Topic:    "spans",
					Encoding: "otlp_proto",
				},
				Traces: TopicEncodingConfig{
					Topic:    "spans",
					Encoding: "otlp_proto",
				},
				Topic: "spans",
				ErrorBackOff: configretry.BackOffConfig{
					Enabled: false,
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "legacy_topic"),
			expected: &Config{
				ClientConfig:   configkafka.NewDefaultClientConfig(),
				ConsumerConfig: configkafka.NewDefaultConsumerConfig(),
				Logs: TopicEncodingConfig{
					Topic:    "legacy_topic",
					Encoding: "otlp_proto",
				},
				Metrics: TopicEncodingConfig{
					Topic:    "metrics_topic",
					Encoding: "otlp_proto",
				},
				Traces: TopicEncodingConfig{
					Topic:    "legacy_topic",
					Encoding: "otlp_proto",
				},
				Topic: "legacy_topic",
				ErrorBackOff: configretry.BackOffConfig{
					Enabled: false,
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "legacy_encoding"),
			expected: &Config{
				ClientConfig:   configkafka.NewDefaultClientConfig(),
				ConsumerConfig: configkafka.NewDefaultConsumerConfig(),
				Logs: TopicEncodingConfig{
					Topic:    "otlp_logs",
					Encoding: "legacy_encoding",
				},
				Metrics: TopicEncodingConfig{
					Topic:    "otlp_metrics",
					Encoding: "metrics_encoding",
				},
				Traces: TopicEncodingConfig{
					Topic:    "otlp_spans",
					Encoding: "legacy_encoding",
				},
				Encoding: "legacy_encoding",
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
					Topic:    "logs",
					Encoding: "direct",
				},
				Metrics: TopicEncodingConfig{
					Topic:    "otlp_metrics",
					Encoding: "otlp_proto",
				},
				Traces: TopicEncodingConfig{
					Topic:    "otlp_spans",
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
					Topic:    "otlp_logs",
					Encoding: "otlp_proto",
				},
				Metrics: TopicEncodingConfig{
					Topic:    "otlp_metrics",
					Encoding: "otlp_proto",
				},
				Traces: TopicEncodingConfig{
					Topic:    "otlp_spans",
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
