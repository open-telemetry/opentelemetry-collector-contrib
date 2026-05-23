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
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/kafka/configkafka"
)

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
					return queue
				}()),
				ClientConfig: func() configkafka.ClientConfig {
					config := configkafka.NewDefaultClientConfig()
					config.Brokers = []string{"foo:123", "bar:456"}
					return config
				}(),
				Producer: func() configkafka.ProducerConfig {
					config := configkafka.NewDefaultProducerConfig()
					config.MaxMessageBytes = 10000000
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
				PartitionTracesByID:                  true,
				PartitionMetricsByResourceAttributes: true,
				PartitionLogsByResourceAttributes:    true,
				PartitionLogsByTraceID:               false,
				RecordPartitioner: (RecordPartitionerConfig{
					StickyKey: &StickyKeyPartitionerConfig{
						Hasher: "sarama_compat",
					},
				}),
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "round_robin_partitioner"),
			expected: &Config{
				TimeoutSettings:  exporterhelper.NewDefaultTimeoutConfig(),
				BackOffConfig:    configretry.NewDefaultBackOffConfig(),
				QueueBatchConfig: configoptional.Some(exporterhelper.NewDefaultQueueConfig()),
				ClientConfig:     configkafka.NewDefaultClientConfig(),
				Producer:         configkafka.NewDefaultProducerConfig(),
				Logs:             SignalConfig{Topic: defaultLogsTopic, Encoding: defaultLogsEncoding},
				Metrics:          SignalConfig{Topic: defaultMetricsTopic, Encoding: defaultMetricsEncoding},
				Traces:           SignalConfig{Topic: defaultTracesTopic, Encoding: defaultTracesEncoding},
				Profiles:         SignalConfig{Topic: defaultProfilesTopic, Encoding: defaultProfilesEncoding},
				RecordPartitioner: (RecordPartitionerConfig{
					RoundRobin: &struct{}{},
				}),
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "least_backup_partitioner"),
			expected: &Config{
				TimeoutSettings:  exporterhelper.NewDefaultTimeoutConfig(),
				BackOffConfig:    configretry.NewDefaultBackOffConfig(),
				QueueBatchConfig: configoptional.Some(exporterhelper.NewDefaultQueueConfig()),
				ClientConfig:     configkafka.NewDefaultClientConfig(),
				Producer:         configkafka.NewDefaultProducerConfig(),
				Logs:             SignalConfig{Topic: defaultLogsTopic, Encoding: defaultLogsEncoding},
				Metrics:          SignalConfig{Topic: defaultMetricsTopic, Encoding: defaultMetricsEncoding},
				Traces:           SignalConfig{Topic: defaultTracesTopic, Encoding: defaultTracesEncoding},
				Profiles:         SignalConfig{Topic: defaultProfilesTopic, Encoding: defaultProfilesEncoding},
				RecordPartitioner: (RecordPartitionerConfig{
					LeastBackup: &struct{}{},
				}),
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "sticky_key_partitioner"),
			expected: &Config{
				TimeoutSettings:  exporterhelper.NewDefaultTimeoutConfig(),
				BackOffConfig:    configretry.NewDefaultBackOffConfig(),
				QueueBatchConfig: configoptional.Some(exporterhelper.NewDefaultQueueConfig()),
				ClientConfig:     configkafka.NewDefaultClientConfig(),
				Producer:         configkafka.NewDefaultProducerConfig(),
				Logs:             SignalConfig{Topic: defaultLogsTopic, Encoding: defaultLogsEncoding},
				Metrics:          SignalConfig{Topic: defaultMetricsTopic, Encoding: defaultMetricsEncoding},
				Traces:           SignalConfig{Topic: defaultTracesTopic, Encoding: defaultTracesEncoding},
				Profiles:         SignalConfig{Topic: defaultProfilesTopic, Encoding: defaultProfilesEncoding},
				RecordPartitioner: (RecordPartitionerConfig{
					StickyKey: &StickyKeyPartitionerConfig{
						Hasher: "sarama_compat",
					},
				}),
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "sticky_key_partitioner_murmur2"),
			expected: &Config{
				TimeoutSettings:  exporterhelper.NewDefaultTimeoutConfig(),
				BackOffConfig:    configretry.NewDefaultBackOffConfig(),
				QueueBatchConfig: configoptional.Some(exporterhelper.NewDefaultQueueConfig()),
				ClientConfig:     configkafka.NewDefaultClientConfig(),
				Producer:         configkafka.NewDefaultProducerConfig(),
				Logs:             SignalConfig{Topic: defaultLogsTopic, Encoding: defaultLogsEncoding},
				Metrics:          SignalConfig{Topic: defaultMetricsTopic, Encoding: defaultMetricsEncoding},
				Traces:           SignalConfig{Topic: defaultTracesTopic, Encoding: defaultTracesEncoding},
				Profiles:         SignalConfig{Topic: defaultProfilesTopic, Encoding: defaultProfilesEncoding},
				RecordPartitioner: (RecordPartitionerConfig{
					StickyKey: &StickyKeyPartitionerConfig{
						Hasher: "murmur2",
					},
				}),
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "per_signal_topic"),
			expected: &Config{
				TimeoutSettings:  exporterhelper.NewDefaultTimeoutConfig(),
				BackOffConfig:    configretry.NewDefaultBackOffConfig(),
				QueueBatchConfig: configoptional.Some(exporterhelper.NewDefaultQueueConfig()),
				ClientConfig:     configkafka.NewDefaultClientConfig(),
				Producer:         configkafka.NewDefaultProducerConfig(),
				Logs: SignalConfig{
					Topic:                "per_signal_topic",
					Encoding:             "otlp_proto",
					TopicFromMetadataKey: "metadata_key",
				},
				Metrics: SignalConfig{
					Topic:    "metrics_topic",
					Encoding: "otlp_proto",
				},
				Traces: SignalConfig{
					Topic:    "per_signal_topic",
					Encoding: "otlp_proto",
				},
				Profiles: SignalConfig{
					Topic:    "per_signal_topic",
					Encoding: "otlp_proto",
				},
				IncludeMetadataKeys: []string{
					"metadata_key",
				},
				RecordPartitioner: (RecordPartitionerConfig{
					StickyKey: &StickyKeyPartitionerConfig{
						Hasher: "sarama_compat",
					},
				}),
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "per_signal_encoding"),
			expected: &Config{
				TimeoutSettings:  exporterhelper.NewDefaultTimeoutConfig(),
				BackOffConfig:    configretry.NewDefaultBackOffConfig(),
				QueueBatchConfig: configoptional.Some(exporterhelper.NewDefaultQueueConfig()),
				ClientConfig:     configkafka.NewDefaultClientConfig(),
				Producer:         configkafka.NewDefaultProducerConfig(),
				Logs: SignalConfig{
					Topic:    "otlp_logs",
					Encoding: "per_signal_encoding",
				},
				Metrics: SignalConfig{
					Topic:    "otlp_metrics",
					Encoding: "metrics_encoding",
				},
				Traces: SignalConfig{
					Topic:    "otlp_spans",
					Encoding: "per_signal_encoding",
				},
				Profiles: SignalConfig{
					Topic:    "otlp_profiles",
					Encoding: "per_signal_encoding",
				},
				RecordPartitioner: (RecordPartitionerConfig{
					StickyKey: &StickyKeyPartitionerConfig{
						Hasher: "sarama_compat",
					},
				}),
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "metadata_batch_valid"),
			expected: &Config{
				TimeoutSettings: exporterhelper.NewDefaultTimeoutConfig(),
				BackOffConfig:   configretry.NewDefaultBackOffConfig(),
				QueueBatchConfig: configoptional.Some(func() exporterhelper.QueueBatchConfig {
					queue := exporterhelper.NewDefaultQueueConfig()
					queue.Batch = configoptional.Some(func() exporterhelper.BatchConfig {
						batch := exporterhelper.BatchConfig{
							Sizer: exporterhelper.RequestSizerTypeBytes,
						}
						batch.FlushTimeout = 200 * time.Millisecond
						batch.MinSize = 8192
						batch.Partition.MetadataKeys = []string{"metadata_key", "another_key", "kafka_topic"}
						return batch
					}())
					return queue
				}()),
				ClientConfig: configkafka.NewDefaultClientConfig(),
				Producer:     configkafka.NewDefaultProducerConfig(),
				Logs: SignalConfig{
					Topic:                "otlp_logs",
					TopicFromMetadataKey: "kafka_topic",
					Encoding:             "otlp_proto",
				},
				Metrics: SignalConfig{
					Topic:                "otlp_metrics",
					TopicFromMetadataKey: "kafka_topic",
					Encoding:             "otlp_proto",
				},
				Traces: SignalConfig{
					Topic:                "otlp_spans",
					TopicFromMetadataKey: "kafka_topic",
					Encoding:             "otlp_proto",
				},
				Profiles: SignalConfig{
					Topic:                "otlp_profiles",
					TopicFromMetadataKey: "kafka_topic",
					Encoding:             "otlp_proto",
				},
				IncludeMetadataKeys: []string{"metadata_key"},
				RecordPartitioner: (RecordPartitionerConfig{
					StickyKey: &StickyKeyPartitionerConfig{
						Hasher: "sarama_compat",
					},
				}),
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
		errorContains string
		configFile    string
	}{
		{
			id:            component.NewIDWithName(metadata.Type, ""),
			errorContains: errLogsPartitionExclusive.Error(),
			configFile:    "config-partitioning-failed.yaml",
		},
		{
			id:            component.NewIDWithName(metadata.Type, ""),
			errorContains: `logs::topic_from_metadata_key: topic_from_metadata_key must be present in sending_queue::batch::partition::metadata_keys`,
			configFile:    "config-topic-from-metadata-failed.yaml",
		},
		{
			id:            component.NewIDWithName(metadata.Type, "missing_batch_partitioner_keys"),
			errorContains: errBatchPartitionMetadataKeysRequired.Error(),
			configFile:    "config-batch-partition-validation-failed.yaml",
		},
		{
			id:            component.NewIDWithName(metadata.Type, "not_superset_batch_partitioner_keys"),
			errorContains: `sending_queue::batch::partition::metadata_keys must include all include_metadata_keys values: missing "required_key" from sending_queue::batch::partition::metadata_keys=[metadata_key]`,
			configFile:    "config-batch-partition-validation-failed.yaml",
		},
		{
			id:            component.NewIDWithName(metadata.Type, "multiple_partitioner"),
			errorContains: errRecordPartitionerMultipleSet.Error(),
			configFile:    "config-partitioning-failed.yaml",
		},
		{
			id:            component.NewIDWithName(metadata.Type, "invalid_sticky_key_hasher"),
			errorContains: `sticky_key: unknown hasher "invalid_hasher", valid values are "sarama_compat", "murmur2"`,
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

			err = xconfmap.Validate(cfg)
			assert.ErrorContains(t, err, tt.errorContains)
		})
	}
}
