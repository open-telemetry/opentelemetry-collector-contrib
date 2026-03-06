// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkaexporter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/exporter/xexporter"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/kafka/configkafka"
)

// applyConfigOption is used to modify values of the
// the default exporter config to make it easier to
// use the return in a test table set up
func applyConfigOption(option func(conf *Config)) *Config {
	conf := createDefaultConfig().(*Config)
	option(conf)
	return conf
}

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
	assert.Equal(t, configkafka.NewDefaultClientConfig(), cfg.ClientConfig)
	assert.Empty(t, cfg.Topic)
}

func TestCreateMetricExporter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		conf *Config
	}{
		{
			name: "valid config (no validating broker)",
			conf: applyConfigOption(func(conf *Config) {
				conf.Metadata.Full = false
				conf.Brokers = []string{"invalid:9092"}
				conf.ProtocolVersion = "2.0.0"
			}),
		},
		{
			name: "default_encoding",
			conf: applyConfigOption(func(conf *Config) {
				// Disabling broker check to ensure encoding work
				conf.Metadata.Full = false
				conf.Encoding = "otlp_proto"
			}),
		},
		{
			name: "with include metadata keys and partitioner",
			conf: applyConfigOption(func(conf *Config) {
				// Disabling broker check
				conf.Metadata.Full = false
				conf.IncludeMetadataKeys = []string{"k1", "k2"}
				conf.QueueBatchConfig.GetOrInsertDefault().Batch = configoptional.Some(exporterhelper.BatchConfig{
					Sizer: exporterhelper.RequestSizerTypeBytes,
				})
			}),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			f := NewFactory()
			exporter, err := f.CreateMetrics(
				t.Context(),
				exportertest.NewNopSettings(metadata.Type),
				tc.conf,
			)
			require.NoError(t, err)
			assert.NotNil(t, exporter, "Must return valid exporter")
			err = exporter.Start(t.Context(), componenttest.NewNopHost())
			assert.NoError(t, err, "Must not error")
			assert.NotNil(t, exporter, "Must return valid exporter when no error is returned")
			assert.NoError(t, exporter.Shutdown(t.Context()))
		})
	}
}

func TestCreateLogExporter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		conf *Config
	}{
		{
			name: "valid config (no validating broker)",
			conf: applyConfigOption(func(conf *Config) {
				conf.Metadata.Full = false
				conf.Brokers = []string{"invalid:9092"}
				conf.ProtocolVersion = "2.0.0"
			}),
		},
		{
			name: "default_encoding",
			conf: applyConfigOption(func(conf *Config) {
				// Disabling broker check to ensure encoding work
				conf.Metadata.Full = false
				conf.Encoding = "otlp_proto"
			}),
		},
		{
			name: "with include metadata keys and partitioner",
			conf: applyConfigOption(func(conf *Config) {
				// Disabling broker check
				conf.Metadata.Full = false
				conf.IncludeMetadataKeys = []string{"k1", "k2"}
				conf.QueueBatchConfig.GetOrInsertDefault().Batch = configoptional.Some(exporterhelper.BatchConfig{
					Sizer: exporterhelper.RequestSizerTypeBytes,
				})
			}),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			f := NewFactory()
			exporter, err := f.CreateLogs(
				t.Context(),
				exportertest.NewNopSettings(metadata.Type),
				tc.conf,
			)
			require.NoError(t, err)
			assert.NotNil(t, exporter, "Must return valid exporter")
			err = exporter.Start(t.Context(), componenttest.NewNopHost())
			assert.NoError(t, err, "Must not error")
			assert.NotNil(t, exporter, "Must return valid exporter when no error is returned")
			assert.NoError(t, exporter.Shutdown(t.Context()))
		})
	}
}

func TestCreateTraceExporter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		conf *Config
	}{
		{
			name: "valid config (no validating brokers)",
			conf: applyConfigOption(func(conf *Config) {
				conf.Metadata.Full = false
				conf.Brokers = []string{"invalid:9092"}
				conf.ProtocolVersion = "2.0.0"
			}),
		},
		{
			name: "default_encoding",
			conf: applyConfigOption(func(conf *Config) {
				// Disabling broker check to ensure encoding work
				conf.Metadata.Full = false
				conf.Encoding = "otlp_proto"
			}),
		},
		{
			name: "with include metadata keys and partitioner",
			conf: applyConfigOption(func(conf *Config) {
				// Disabling broker check
				conf.Metadata.Full = false
				conf.IncludeMetadataKeys = []string{"k1", "k2"}
				conf.QueueBatchConfig.GetOrInsertDefault().Batch = configoptional.Some(exporterhelper.BatchConfig{
					Sizer: exporterhelper.RequestSizerTypeBytes,
				})
			}),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			f := NewFactory()
			exporter, err := f.CreateTraces(
				t.Context(),
				exportertest.NewNopSettings(metadata.Type),
				tc.conf,
			)
			require.NoError(t, err)
			assert.NotNil(t, exporter, "Must return valid exporter")
			err = exporter.Start(t.Context(), componenttest.NewNopHost())
			assert.NoError(t, err, "Must not error")
			assert.NotNil(t, exporter, "Must return valid exporter when no error is returned")
			assert.NoError(t, exporter.Shutdown(t.Context()))
		})
	}
}

func TestCreateProfileExporter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		conf *Config
	}{
		{
			name: "valid config (no validating broker)",
			conf: applyConfigOption(func(conf *Config) {
				conf.Metadata.Full = false
				conf.Brokers = []string{"invalid:9092"}
				conf.ProtocolVersion = "2.0.0"
			}),
		},
		{
			name: "default_encoding",
			conf: applyConfigOption(func(conf *Config) {
				// Disabling broker check to ensure encoding work
				conf.Metadata.Full = false
				conf.Encoding = "otlp_proto"
			}),
		},
		{
			name: "with include metadata keys and partitioner",
			conf: applyConfigOption(func(conf *Config) {
				// Disabling broker check
				conf.Metadata.Full = false
				conf.IncludeMetadataKeys = []string{"k1", "k2"}
				conf.QueueBatchConfig.GetOrInsertDefault().Batch = configoptional.Some(exporterhelper.BatchConfig{
					Sizer: exporterhelper.RequestSizerTypeBytes,
				})
			}),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			f := NewFactory().(xexporter.Factory)
			exporter, err := f.CreateProfiles(
				t.Context(),
				exportertest.NewNopSettings(metadata.Type),
				tc.conf,
			)
			require.NoError(t, err)
			assert.NotNil(t, exporter, "Must return valid exporter")
			err = exporter.Start(t.Context(), componenttest.NewNopHost())
			assert.NoError(t, err, "Must not error")
			assert.NotNil(t, exporter, "Must return valid exporter when no error is returned")
			assert.NoError(t, exporter.Shutdown(t.Context()))
		})
	}
}

func TestPartitionerKeysSet(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(cfg *Config) (metadataKeys []string, topicFromMetadataKey string)
		expected []string
	}{
		{
			name: "includes configured metadata keys",
			setup: func(cfg *Config) ([]string, string) {
				cfg.IncludeMetadataKeys = []string{"tenant_id", "request_id"}
				cfg.Logs.TopicFromMetadataKey = "kafka_topic_logs"
				cfg.Metrics.TopicFromMetadataKey = "kafka_topic_metrics"
				cfg.Traces.TopicFromMetadataKey = "kafka_topic_traces"
				cfg.Profiles.TopicFromMetadataKey = "kafka_topic_profiles"
				return cfg.IncludeMetadataKeys, cfg.Traces.TopicFromMetadataKey
			},
			expected: []string{"tenant_id", "request_id", "kafka_topic_traces"},
		},
		{
			name: "de-duplicates and drops empty keys",
			setup: func(cfg *Config) ([]string, string) {
				cfg.IncludeMetadataKeys = []string{"", "tenant_id", "tenant_id"}
				return cfg.IncludeMetadataKeys, "tenant_id"
			},
			expected: []string{"tenant_id"},
		},
		{
			name: "does not include other signal topic metadata keys",
			setup: func(cfg *Config) ([]string, string) {
				cfg.Logs.TopicFromMetadataKey = "logs_topic"
				cfg.Metrics.TopicFromMetadataKey = "metrics_topic"
				cfg.Traces.TopicFromMetadataKey = "traces_topic"
				cfg.Profiles.TopicFromMetadataKey = "profiles_topic"
				return cfg.IncludeMetadataKeys, cfg.Traces.TopicFromMetadataKey
			},
			expected: []string{"traces_topic"},
		},
		{
			name: "adds topic_from_metadata_key when absent from include_metadata_keys",
			setup: func(cfg *Config) ([]string, string) {
				cfg.IncludeMetadataKeys = []string{"x-tenant-id"}
				cfg.Traces.TopicFromMetadataKey = "kafka_topic"
				return cfg.IncludeMetadataKeys, cfg.Traces.TopicFromMetadataKey
			},
			expected: []string{"x-tenant-id", "kafka_topic"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := createDefaultConfig().(*Config)
			metadataKeys, topicFromMetadataKey := tc.setup(cfg)
			assert.Equal(t, tc.expected, partitionerKeysSet(metadataKeys, topicFromMetadataKey))
		})
	}
}
