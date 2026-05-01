// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "").String())
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))
	require.NotNil(t, cfg)
}

func TestConfigValidatePayloadCompression(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	require.NoError(t, cfg.Validate())

	cfg.QueueSettings.QueueConfig = configoptional.Some(exporterhelper.NewDefaultQueueConfig())
	cfg.QueueSettings.PayloadCompression = QueuePayloadCompressionSnappy
	require.NoError(t, cfg.Validate())
	require.Nil(t, cfg.QueueSettings.QueueConfig.Get().StorageID)

	cfg.QueueSettings.PayloadCompression = QueuePayloadCompressionZstd
	require.NoError(t, cfg.Validate())

	cfg.QueueSettings.PayloadCompression = QueuePayloadCompression("invalid")
	require.Error(t, cfg.Validate())
}

func TestConfigValidateCompressInMemory(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.QueueSettings.CompressInMemory = true
	require.ErrorContains(t, cfg.Validate(), "sending_queue.compress_in_memory requires sending_queue.enabled=true")

	cfg.QueueSettings.QueueConfig = configoptional.Some(exporterhelper.NewDefaultQueueConfig())
	require.ErrorContains(t, cfg.Validate(), "sending_queue.compress_in_memory requires sending_queue.payload_compression")

	cfg.QueueSettings.PayloadCompression = QueuePayloadCompressionNone
	require.ErrorContains(t, cfg.Validate(), "sending_queue.compress_in_memory requires sending_queue.payload_compression")

	cfg.QueueSettings.PayloadCompression = QueuePayloadCompressionSnappy
	require.NoError(t, cfg.Validate())

	cfg.QueueSettings.PayloadCompression = QueuePayloadCompressionZstd
	require.NoError(t, cfg.Validate())

	queueCfg := exporterhelper.NewDefaultQueueConfig()
	queueCfg.WaitForResult = true
	cfg.QueueSettings.QueueConfig = configoptional.Some(queueCfg)
	cfg.QueueSettings.PayloadCompression = QueuePayloadCompressionSnappy
	require.NoError(t, cfg.Validate())
}

func TestConfigValidateLogBatcher(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	require.NoError(t, cfg.Validate())

	cfg.LogBatcher.Enabled = true
	cfg.LogBatcher.MaxRecords = 0
	require.ErrorContains(t, cfg.Validate(), "log_batcher.max_records")

	cfg.LogBatcher.MaxRecords = 10
	cfg.LogBatcher.MaxBytes = 0
	require.ErrorContains(t, cfg.Validate(), "log_batcher.max_bytes")

	cfg.LogBatcher.MaxBytes = 1024
	cfg.LogBatcher.FlushInterval = 0
	require.ErrorContains(t, cfg.Validate(), "log_batcher.flush_interval")

	cfg.LogBatcher.FlushInterval = time.Second
	cfg.LogBatcher.PayloadCompression = QueuePayloadCompression("brotli")
	require.ErrorContains(t, cfg.Validate(), "log_batcher.payload_compression")

	cfg.LogBatcher.PayloadCompression = QueuePayloadCompressionSnappy
	require.NoError(t, cfg.Validate())

	cfg.LogBatcher.PayloadCompression = QueuePayloadCompressionZstd
	require.NoError(t, cfg.Validate())
}

func TestConfigValidateMetricBatcher(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	require.NoError(t, cfg.Validate())

	cfg.MetricBatcher.Enabled = true
	cfg.MetricBatcher.MaxDataPoints = 0
	require.ErrorContains(t, cfg.Validate(), "metric_batcher.max_datapoints")

	cfg.MetricBatcher.MaxDataPoints = 10
	cfg.MetricBatcher.MaxBytes = 0
	require.ErrorContains(t, cfg.Validate(), "metric_batcher.max_bytes")

	cfg.MetricBatcher.MaxBytes = 1024
	cfg.MetricBatcher.FlushInterval = 0
	require.ErrorContains(t, cfg.Validate(), "metric_batcher.flush_interval")

	cfg.MetricBatcher.FlushInterval = time.Second
	cfg.MetricBatcher.MaxRetryBufferMultiplier = 0
	require.ErrorContains(t, cfg.Validate(), "metric_batcher.max_retry_buffer_multiplier")

	cfg.MetricBatcher.MaxRetryBufferMultiplier = 5
	require.NoError(t, cfg.Validate())
}

func TestConfigValidateEndpointHealth(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	require.False(t, cfg.EndpointHealth.Enabled)
	require.Equal(t, defaultEndpointHealthQuarantineDuration, cfg.EndpointHealth.QuarantineDuration)
	require.True(t, cfg.EndpointHealth.RerouteOnFailure)
	require.Equal(t, 1, cfg.EndpointHealth.MaxRerouteAttempts)
	require.NoError(t, cfg.Validate())

	cfg.EndpointHealth.QuarantineDuration = -time.Second
	require.NoError(t, cfg.Validate())
	cfg.EndpointHealth.QuarantineDuration = defaultEndpointHealthQuarantineDuration

	cfg.EndpointHealth.Enabled = true
	require.NoError(t, cfg.Validate())

	cfg.EndpointHealth.QuarantineDuration = 0
	require.ErrorContains(t, cfg.Validate(), "endpoint_health.quarantine_duration")

	cfg.EndpointHealth.QuarantineDuration = -time.Second
	require.ErrorContains(t, cfg.Validate(), "endpoint_health.quarantine_duration")

	cfg.EndpointHealth.QuarantineDuration = time.Second
	cfg.EndpointHealth.MaxRerouteAttempts = -1
	require.ErrorContains(t, cfg.Validate(), "endpoint_health.max_reroute_attempts")

	cfg.EndpointHealth.MaxRerouteAttempts = 0
	cfg.EndpointHealth.RerouteOnFailure = false
	require.NoError(t, cfg.Validate())
}

func TestLoadConfigWithQueueCompression(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	conf := confmap.NewFromStringMap(map[string]any{
		"protocol": map[string]any{
			"otlp": map[string]any{
				"endpoint": "localhost:4317",
				"tls": map[string]any{
					"insecure": true,
				},
			},
		},
		"resolver": map[string]any{
			"static": map[string]any{
				"hostnames": []string{"localhost:4317"},
			},
		},
		"sending_queue": map[string]any{
			"enabled":             true,
			"queue_size":          1000,
			"num_consumers":       2,
			"payload_compression": "zstd",
			"compress_in_memory":  true,
		},
	})

	require.NoError(t, conf.Unmarshal(cfg))
	require.True(t, cfg.QueueSettings.QueueConfig.HasValue())
	require.Equal(t, int64(1000), cfg.QueueSettings.QueueConfig.Get().QueueSize)
	require.Equal(t, 2, cfg.QueueSettings.QueueConfig.Get().NumConsumers)
	require.Equal(t, QueuePayloadCompressionZstd, cfg.QueueSettings.PayloadCompression)
	require.True(t, cfg.QueueSettings.CompressInMemory)
	require.Nil(t, cfg.QueueSettings.QueueConfig.Get().StorageID)
}

func TestLoadConfigWithQueueCompressionAndExplicitNullStorage(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	conf := confmap.NewFromStringMap(map[string]any{
		"protocol": map[string]any{
			"otlp": map[string]any{
				"endpoint": "localhost:4317",
				"tls": map[string]any{
					"insecure": true,
				},
			},
		},
		"resolver": map[string]any{
			"static": map[string]any{
				"hostnames": []string{"localhost:4317"},
			},
		},
		"sending_queue": map[string]any{
			"enabled":             true,
			"queue_size":          1000,
			"num_consumers":       2,
			"payload_compression": "zstd",
			"compress_in_memory":  true,
			"storage":             nil,
		},
	})

	require.NoError(t, conf.Unmarshal(cfg))
	require.True(t, cfg.QueueSettings.QueueConfig.HasValue())
	require.Nil(t, cfg.QueueSettings.QueueConfig.Get().StorageID)
	require.Equal(t, QueuePayloadCompressionZstd, cfg.QueueSettings.PayloadCompression)
	require.True(t, cfg.QueueSettings.CompressInMemory)
}

func TestLoadConfigWithLegacyQueueFieldsKeepsQueueDisabled(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	conf := confmap.NewFromStringMap(map[string]any{
		"protocol": map[string]any{
			"otlp": map[string]any{
				"endpoint": "localhost:4317",
				"tls": map[string]any{
					"insecure": true,
				},
			},
		},
		"resolver": map[string]any{
			"static": map[string]any{
				"hostnames": []string{"localhost:4317"},
			},
		},
		"sending_queue": map[string]any{
			"queue_size":    1000,
			"num_consumers": 2,
			"storage":       "file_storage/queue",
		},
	})

	require.NoError(t, conf.Unmarshal(cfg))
	require.False(t, cfg.QueueSettings.QueueConfig.HasValue())
	require.Empty(t, cfg.QueueSettings.PayloadCompression)
	require.False(t, cfg.QueueSettings.CompressInMemory)
	require.NoError(t, cfg.Validate())
}

func TestLoadConfigWithDisabledQueueIgnoresCompressionFields(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	conf := confmap.NewFromStringMap(map[string]any{
		"protocol": map[string]any{
			"otlp": map[string]any{
				"endpoint": "localhost:4317",
				"tls": map[string]any{
					"insecure": true,
				},
			},
		},
		"resolver": map[string]any{
			"static": map[string]any{
				"hostnames": []string{"localhost:4317"},
			},
		},
		"sending_queue": map[string]any{
			"enabled":             false,
			"queue_size":          1000,
			"payload_compression": "zstd",
			"compress_in_memory":  true,
		},
	})

	require.NoError(t, conf.Unmarshal(cfg))
	require.False(t, cfg.QueueSettings.QueueConfig.HasValue())
	require.Empty(t, cfg.QueueSettings.PayloadCompression)
	require.False(t, cfg.QueueSettings.CompressInMemory)
	require.NoError(t, cfg.Validate())
}

func TestLoadConfigWithLogBatcher(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	conf := confmap.NewFromStringMap(map[string]any{
		"protocol": map[string]any{
			"otlp": map[string]any{
				"endpoint": "localhost:4317",
				"tls": map[string]any{
					"insecure": true,
				},
			},
		},
		"resolver": map[string]any{
			"static": map[string]any{
				"hostnames": []string{"localhost:4317"},
			},
		},
		"log_batcher": map[string]any{
			"enabled":             true,
			"max_records":         1024,
			"max_bytes":           2097152,
			"flush_interval":      "250ms",
			"payload_compression": "zstd",
		},
	})

	require.NoError(t, conf.Unmarshal(cfg))
	require.True(t, cfg.LogBatcher.Enabled)
	require.Equal(t, 1024, cfg.LogBatcher.MaxRecords)
	require.Equal(t, 2097152, cfg.LogBatcher.MaxBytes)
	require.Equal(t, 250*time.Millisecond, cfg.LogBatcher.FlushInterval)
	require.Equal(t, QueuePayloadCompressionZstd, cfg.LogBatcher.PayloadCompression)
}

func TestLoadConfigWithLogRouting(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	conf := confmap.NewFromStringMap(map[string]any{
		"protocol": map[string]any{
			"otlp": map[string]any{
				"endpoint": "localhost:4317",
				"tls": map[string]any{
					"insecure": true,
				},
			},
		},
		"resolver": map[string]any{
			"static": map[string]any{
				"hostnames": []string{"localhost:4317"},
			},
		},
		"log_routing": map[string]any{
			"ignore_trace_id": true,
		},
	})

	require.NoError(t, conf.Unmarshal(cfg))
	require.True(t, cfg.LogRouting.IgnoreTraceID)
}

func TestLoadConfigWithMetricBatcher(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	conf := confmap.NewFromStringMap(map[string]any{
		"protocol": map[string]any{
			"otlp": map[string]any{
				"endpoint": "localhost:4317",
				"tls": map[string]any{
					"insecure": true,
				},
			},
		},
		"resolver": map[string]any{
			"static": map[string]any{
				"hostnames": []string{"localhost:4317"},
			},
		},
		"metric_batcher": map[string]any{
			"enabled":                     true,
			"max_datapoints":              4096,
			"max_bytes":                   2097152,
			"flush_interval":              "300ms",
			"max_retry_buffer_multiplier": 12,
		},
	})

	require.NoError(t, conf.Unmarshal(cfg))
	require.True(t, cfg.MetricBatcher.Enabled)
	require.Equal(t, 4096, cfg.MetricBatcher.MaxDataPoints)
	require.Equal(t, 2097152, cfg.MetricBatcher.MaxBytes)
	require.Equal(t, 300*time.Millisecond, cfg.MetricBatcher.FlushInterval)
	require.Equal(t, 12, cfg.MetricBatcher.MaxRetryBufferMultiplier)
}

func TestLoadConfigWithEndpointHealth(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	conf := confmap.NewFromStringMap(map[string]any{
		"protocol": map[string]any{
			"otlp": map[string]any{
				"endpoint": "localhost:4317",
				"tls":      map[string]any{"insecure": true},
			},
		},
		"resolver": map[string]any{
			"dns": map[string]any{
				"hostname": "collector-backend-headless",
				"interval": "1s",
				"timeout":  "1s",
			},
		},
		"endpoint_health": map[string]any{
			"enabled":              true,
			"quarantine_duration":  "10s",
			"reroute_on_failure":   true,
			"max_reroute_attempts": 1,
		},
	})

	require.NoError(t, conf.Unmarshal(cfg))
	require.True(t, cfg.EndpointHealth.Enabled)
	require.Equal(t, 10*time.Second, cfg.EndpointHealth.QuarantineDuration)
	require.True(t, cfg.EndpointHealth.RerouteOnFailure)
	require.Equal(t, 1, cfg.EndpointHealth.MaxRerouteAttempts)
	require.NoError(t, cfg.Validate())
}

func TestLoadConfigWithNullNestedOTLPQueueDisablesChildQueue(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	conf := confmap.NewFromStringMap(map[string]any{
		"protocol": map[string]any{
			"otlp": map[string]any{
				"endpoint": "localhost:4317",
				"tls": map[string]any{
					"insecure": true,
				},
				"sending_queue": nil,
			},
		},
		"resolver": map[string]any{
			"static": map[string]any{
				"hostnames": []string{"localhost:4317"},
			},
		},
	})

	require.NoError(t, conf.Unmarshal(cfg))
	require.False(t, cfg.Protocol.OTLP.QueueConfig.HasValue())
}
