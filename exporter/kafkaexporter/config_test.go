// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkaexporter

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/kafka/configkafka"
)

// megabyte is 1 MB in bytes (decimal, 10^6). Used for test config values matching Kafka's max.message.bytes.
const megabyte = 1_000_000

// testdata/config.yaml sets producer.max_message_bytes to this value (10 MB) for the "kafka" test case.
const testConfigYAMLMaxMessageBytes = 10 * megabyte

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id       component.ID
		expected component.Config
	}{
		{
			id: component.NewIDWithName(metadata.Type, ""),
			expected: &Config{
				TimeoutSettings: exporterhelper.TimeoutConfig{
					Timeout: 10 * time.Second,
				},
				BackOffConfig: func() configretry.BackOffConfig {
					config := configretry.NewDefaultBackOffConfig()
					config.InitialInterval = 10 * time.Second
					config.MaxInterval = 60 * time.Second
					config.MaxElapsedTime = 10 * time.Minute
					return config
				}(),
				QueueBatchConfig: configoptional.Some(func() exporterhelper.QueueBatchConfig {
					queue := exporterhelper.NewDefaultQueueConfig()
					queue.NumConsumers = 2
					queue.QueueSize = 10
					// Unmarshal auto-populates a default batch when sending_queue::batch is not set
					// (from producer.max_message_bytes - kafkaOverheadBytes). Expected config must match.
					queue.Batch = configoptional.Some(exporterhelper.BatchConfig{
						Sizer:        exporterhelper.RequestSizerTypeBytes,
						MaxSize:      int64(testConfigYAMLMaxMessageBytes - kafkaOverheadBytes),
						FlushTimeout: 200 * time.Millisecond,
						MinSize:      0,
					})
					return queue
				}()),
				ClientConfig: func() configkafka.ClientConfig {
					config := configkafka.NewDefaultClientConfig()
					config.Brokers = []string{"foo:123", "bar:456"}
					return config
				}(),
				Producer: func() configkafka.ProducerConfig {
					config := configkafka.NewDefaultProducerConfig()
					config.MaxMessageBytes = testConfigYAMLMaxMessageBytes
					config.RequiredAcks = configkafka.WaitForAll
					return config
				}(),
				Logs: SignalConfig{
					Topic:    "spans",
					Encoding: "otlp_proto",
				},
				Metrics: SignalConfig{
					Topic:    "spans",
					Encoding: "otlp_proto",
				},
				Traces: SignalConfig{
					Topic:    "spans",
					Encoding: "otlp_proto",
				},
				Profiles: SignalConfig{
					Topic:    "spans",
					Encoding: "otlp_proto",
				},
				Topic:                                "spans",
				PartitionTracesByID:                  true,
				PartitionMetricsByResourceAttributes: true,
				PartitionLogsByResourceAttributes:    true,
				PartitionLogsByTraceID:               false,
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "legacy_topic"),
			expected: &Config{
				TimeoutSettings:  exporterhelper.NewDefaultTimeoutConfig(),
				BackOffConfig:    configretry.NewDefaultBackOffConfig(),
				QueueBatchConfig: configoptional.Some(func() exporterhelper.QueueBatchConfig {
					q := exporterhelper.NewDefaultQueueConfig()
					// Unmarshal auto-populates a default batch when sending_queue::batch is not set
					// (from producer.max_message_bytes - kafkaOverheadBytes). Expected config must match.
					q.Batch = configoptional.Some(exporterhelper.BatchConfig{
						Sizer:        exporterhelper.RequestSizerTypeBytes,
						MaxSize:      int64(1000000 - kafkaOverheadBytes),
						FlushTimeout: 200 * time.Millisecond,
						MinSize:      0,
					})
					return q
				}()),
				ClientConfig: configkafka.NewDefaultClientConfig(),
				Producer:     configkafka.NewDefaultProducerConfig(),
				Logs: SignalConfig{
					Topic:                "legacy_topic",
					Encoding:             "otlp_proto",
					TopicFromMetadataKey: "metadata_key",
				},
				Metrics: SignalConfig{
					Topic:    "metrics_topic",
					Encoding: "otlp_proto",
				},
				Traces: SignalConfig{
					Topic:    "legacy_topic",
					Encoding: "otlp_proto",
				},
				Profiles: SignalConfig{
					Topic:    "legacy_topic",
					Encoding: "otlp_proto",
				},
				Topic: "legacy_topic",
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "legacy_encoding"),
			expected: &Config{
				TimeoutSettings:  exporterhelper.NewDefaultTimeoutConfig(),
				BackOffConfig:    configretry.NewDefaultBackOffConfig(),
				QueueBatchConfig: configoptional.Some(func() exporterhelper.QueueBatchConfig {
					q := exporterhelper.NewDefaultQueueConfig()
					// Unmarshal auto-populates a default batch when sending_queue::batch is not set
					// (from producer.max_message_bytes - kafkaOverheadBytes). Expected config must match.
					q.Batch = configoptional.Some(exporterhelper.BatchConfig{
						Sizer:        exporterhelper.RequestSizerTypeBytes,
						MaxSize:      int64(1000000 - kafkaOverheadBytes),
						FlushTimeout: 200 * time.Millisecond,
						MinSize:      0,
					})
					return q
				}()),
				ClientConfig: configkafka.NewDefaultClientConfig(),
				Producer:     configkafka.NewDefaultProducerConfig(),
				Logs: SignalConfig{
					Topic:    "otlp_logs",
					Encoding: "legacy_encoding",
				},
				Metrics: SignalConfig{
					Topic:    "otlp_metrics",
					Encoding: "metrics_encoding",
				},
				Traces: SignalConfig{
					Topic:    "otlp_spans",
					Encoding: "legacy_encoding",
				},
				Profiles: SignalConfig{
					Topic:    "otlp_profiles",
					Encoding: "legacy_encoding",
				},
				Encoding: "legacy_encoding",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			cfg := createDefaultConfig().(*Config)

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			assert.NoError(t, xconfmap.Validate(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestLoadConfigFailed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		id            component.ID
		expectedError error
		configFile    string
	}{
		{
			id:            component.NewIDWithName(metadata.Type, ""),
			expectedError: errLogsPartitionExclusive,
			configFile:    "config-partitioning-failed.yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			cm, err := confmaptest.LoadConf(filepath.Join("testdata", tt.configFile))
			require.NoError(t, err)

			cfg := createDefaultConfig().(*Config)

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			assert.ErrorIs(t, xconfmap.Validate(cfg), tt.expectedError)
		})
	}
}

func TestConfig_DefaultSendingQueueBatchWhenNotSet(t *testing.T) {
	t.Parallel()

	// GIVEN: config with producer.max_message_bytes set and sending_queue::batch not set
	cfg := createDefaultConfig().(*Config)
	conf := confmap.NewFromStringMap(map[string]any{
		"producer": map[string]any{
			"max_message_bytes": megabyte,
		},
	})

	// WHEN: config is unmarshaled
	require.NoError(t, cfg.Unmarshal(conf))

	// THEN: userSetSendingQueueBatch is false and default batch is applied (bytes sizer,
	// max_size = producer.max_message_bytes - kafkaOverheadBytes, default flush_timeout and min_size)
	assert.False(t, cfg.userSetSendingQueueBatch)
	require.True(t, cfg.QueueBatchConfig.HasValue())
	queueBatchConfig := cfg.QueueBatchConfig.GetOrInsertDefault()
	require.True(t, queueBatchConfig.Batch.HasValue())
	batch := queueBatchConfig.Batch.Get()
	assert.Equal(t, exporterhelper.RequestSizerTypeBytes, batch.Sizer)
	assert.Equal(t, int64(megabyte - kafkaOverheadBytes), batch.MaxSize)
	assert.Equal(t, 200*time.Millisecond, batch.FlushTimeout)
	assert.Equal(t, int64(0), batch.MinSize)
}

func TestConfig_UserSetSendingQueueBatchPreserved(t *testing.T) {
	t.Parallel()

	// GIVEN: config with producer.max_message_bytes and sending_queue::batch explicitly set
	cfg := createDefaultConfig().(*Config)
	conf := confmap.NewFromStringMap(map[string]any{
		"producer": map[string]any{
			"max_message_bytes": 2000000,
		},
		"sending_queue": map[string]any{
			"batch": map[string]any{
				"sizer":         "bytes",
				"max_size":      5000,
				"min_size":      100,
				"flush_timeout": "100ms",
			},
		},
	})

	// WHEN: config is unmarshaled
	require.NoError(t, cfg.Unmarshal(conf))

	// THEN: userSetSendingQueueBatch is true and user batch values are preserved
	assert.True(t, cfg.userSetSendingQueueBatch)
	require.True(t, cfg.QueueBatchConfig.HasValue())
	queueBatchConfig := cfg.QueueBatchConfig.GetOrInsertDefault()
	require.True(t, queueBatchConfig.Batch.HasValue())
	batch := queueBatchConfig.Batch.Get()
	assert.Equal(t, exporterhelper.RequestSizerTypeBytes, batch.Sizer)
	assert.Equal(t, int64(5000), batch.MaxSize)
	assert.Equal(t, int64(100), batch.MinSize)
	assert.Equal(t, 100*time.Millisecond, batch.FlushTimeout)
}
