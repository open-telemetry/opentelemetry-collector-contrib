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

	cfg.MetricBatcher.PayloadCompression = QueuePayloadCompression("brotli")
	require.ErrorContains(t, cfg.Validate(), "metric_batcher.payload_compression")

	cfg.MetricBatcher.PayloadCompression = QueuePayloadCompressionSnappy
	require.NoError(t, cfg.Validate())

	cfg.MetricBatcher.PayloadCompression = QueuePayloadCompressionZstd
	require.NoError(t, cfg.Validate())
}

func TestCentralQueueDefaultDisabled(t *testing.T) {
	cfg := createDefaultConfig().(*Config)

	require.False(t, cfg.CentralQueue.Enabled)
	require.Zero(t, cfg.CentralQueue.MaxCompressedBytes)
	require.Equal(t, QueuePayloadCompressionZstd, cfg.CentralQueue.PayloadCompression)
	require.Equal(t, 16<<20, cfg.CentralQueue.MaxUncompressedBatchBytes)
	require.Equal(t, int64(512<<20), cfg.CentralQueue.MaxInflightUncompressedBytes)
	require.Equal(t, int64(256<<10), cfg.CentralQueue.TargetCompressedBytes)
	require.Equal(t, 250*time.Millisecond, cfg.CentralQueue.MaxBatchDelay)
	require.Equal(t, 64, cfg.CentralQueue.LaneCount)
	require.NoError(t, cfg.Validate())
}

func TestCentralQueueValidation(t *testing.T) {
	tests := []struct {
		name        string
		mutate      func(*CentralQueueConfig)
		expectedErr string
	}{
		{
			name:        "missing compressed budget",
			mutate:      func(c *CentralQueueConfig) { c.MaxCompressedBytes = 0 },
			expectedErr: "central_queue.max_compressed_bytes",
		},
		{
			name:        "none compression",
			mutate:      func(c *CentralQueueConfig) { c.PayloadCompression = QueuePayloadCompressionNone },
			expectedErr: "central_queue.payload_compression",
		},
		{
			name:        "empty compression",
			mutate:      func(c *CentralQueueConfig) { c.PayloadCompression = "" },
			expectedErr: "central_queue.payload_compression",
		},
		{
			name:        "invalid compression",
			mutate:      func(c *CentralQueueConfig) { c.PayloadCompression = QueuePayloadCompression("brotli") },
			expectedErr: "central_queue.payload_compression",
		},
		{
			name:        "missing batch budget",
			mutate:      func(c *CentralQueueConfig) { c.MaxUncompressedBatchBytes = 0 },
			expectedErr: "central_queue.max_uncompressed_batch_bytes",
		},
		{
			name:        "missing inflight budget",
			mutate:      func(c *CentralQueueConfig) { c.MaxInflightUncompressedBytes = 0 },
			expectedErr: "central_queue.max_inflight_uncompressed_bytes",
		},
		{
			name: "batch budget exceeds inflight budget",
			mutate: func(c *CentralQueueConfig) {
				c.MaxUncompressedBatchBytes = 2048
				c.MaxInflightUncompressedBytes = 1024
			},
			expectedErr: "central_queue.max_uncompressed_batch_bytes",
		},
		{
			name:        "missing target compressed bytes",
			mutate:      func(c *CentralQueueConfig) { c.TargetCompressedBytes = 0 },
			expectedErr: "central_queue.target_compressed_bytes",
		},
		{
			name:        "missing max batch delay",
			mutate:      func(c *CentralQueueConfig) { c.MaxBatchDelay = 0 },
			expectedErr: "central_queue.max_batch_delay",
		},
		{
			name:        "missing lane count",
			mutate:      func(c *CentralQueueConfig) { c.LaneCount = 0 },
			expectedErr: "central_queue.lane_count",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := createDefaultConfig().(*Config)
			cfg.CentralQueue.Enabled = true
			cfg.CentralQueue.MaxCompressedBytes = 1024
			tt.mutate(&cfg.CentralQueue)

			require.ErrorContains(t, cfg.Validate(), tt.expectedErr)
		})
	}

	cfg := createDefaultConfig().(*Config)
	cfg.CentralQueue.Enabled = true
	cfg.CentralQueue.MaxCompressedBytes = 1024
	cfg.CentralQueue.PayloadCompression = QueuePayloadCompressionSnappy
	require.NoError(t, cfg.Validate())

	cfg.CentralQueue.PayloadCompression = QueuePayloadCompressionZstd
	require.NoError(t, cfg.Validate())
}

func TestCentralQueueRejectsChildOTLPQueue(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	conf := confmap.NewFromStringMap(map[string]any{
		"central_queue": map[string]any{
			"enabled":                         true,
			"max_compressed_bytes":            1024,
			"payload_compression":             "zstd",
			"max_uncompressed_batch_bytes":    1024,
			"max_inflight_uncompressed_bytes": 2048,
		},
		"protocol": map[string]any{
			"otlp": map[string]any{
				"endpoint": "localhost:4317",
				"tls": map[string]any{
					"insecure": true,
				},
				"sending_queue": map[string]any{
					"enabled":    true,
					"queue_size": 1000,
				},
			},
		},
		"resolver": map[string]any{
			"static": map[string]any{
				"hostnames": []string{"localhost:4317"},
			},
		},
	})

	require.NoError(t, conf.Unmarshal(cfg))
	require.ErrorContains(t, cfg.Validate(), "central_queue.enabled=true is incompatible with protocol.otlp.sending_queue")
}

func TestCentralQueueAllowsChildOTLPQueueRemovalAcrossUnmarshal(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	withChildQueue := confmap.NewFromStringMap(map[string]any{
		"protocol": map[string]any{
			"otlp": map[string]any{
				"endpoint": "localhost:4317",
				"tls": map[string]any{
					"insecure": true,
				},
				"sending_queue": map[string]any{
					"enabled":    true,
					"queue_size": 1000,
				},
			},
		},
		"resolver": map[string]any{
			"static": map[string]any{
				"hostnames": []string{"localhost:4317"},
			},
		},
	})
	withoutChildQueue := confmap.NewFromStringMap(map[string]any{
		"central_queue": map[string]any{
			"enabled":                         true,
			"max_compressed_bytes":            1024,
			"payload_compression":             "zstd",
			"max_uncompressed_batch_bytes":    1024,
			"max_inflight_uncompressed_bytes": 2048,
		},
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
	})

	require.NoError(t, withChildQueue.Unmarshal(cfg))
	require.True(t, cfg.protocolOTLPSendingQueueConfigured)

	require.NoError(t, withoutChildQueue.Unmarshal(cfg))
	require.False(t, cfg.protocolOTLPSendingQueueConfigured)
	require.NoError(t, cfg.Validate())
}

func TestCentralQueueRejectsIndependentBuffering(t *testing.T) {
	tests := []struct {
		name        string
		mutate      func(*Config)
		expectedErr string
	}{
		{
			name: "top level sending queue",
			mutate: func(cfg *Config) {
				cfg.QueueSettings.QueueConfig = configoptional.Some(exporterhelper.NewDefaultQueueConfig())
			},
			expectedErr: "central_queue.enabled=true is incompatible with sending_queue.enabled=true",
		},
		{
			name: "log batcher",
			mutate: func(cfg *Config) {
				cfg.LogBatcher.Enabled = true
			},
			expectedErr: "central_queue.enabled=true is incompatible with log_batcher.enabled=true",
		},
		{
			name: "metric batcher",
			mutate: func(cfg *Config) {
				cfg.MetricBatcher.Enabled = true
			},
			expectedErr: "central_queue.enabled=true is incompatible with metric_batcher.enabled=true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := createDefaultConfig().(*Config)
			cfg.CentralQueue.Enabled = true
			cfg.CentralQueue.MaxCompressedBytes = 1024
			tt.mutate(cfg)

			require.ErrorContains(t, cfg.Validate(), tt.expectedErr)
		})
	}
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

func TestConfigValidateEndpointHealthActiveProbe(t *testing.T) {
	validProbe := EndpointHealthActiveProbeConfig{
		Enabled:        true,
		Type:           EndpointHealthActiveProbeTypeTCPConnect,
		Interval:       5 * time.Second,
		Timeout:        250 * time.Millisecond,
		Jitter:         "20%",
		MaxConcurrency: 4,
		Fall:           2,
		Rise:           2,
	}

	cfg := createDefaultConfig().(*Config)
	require.False(t, cfg.EndpointHealth.ActiveProbe.Enabled)
	cfg.EndpointHealth.ActiveProbe = validProbe
	require.ErrorContains(t, cfg.Validate(), "endpoint_health.active_probe requires endpoint_health.enabled=true")

	cfg.EndpointHealth.Enabled = true
	require.NoError(t, cfg.Validate())

	tests := []struct {
		name        string
		mutate      func(*EndpointHealthActiveProbeConfig)
		expectedErr string
	}{
		{
			name:        "unknown type",
			mutate:      func(c *EndpointHealthActiveProbeConfig) { c.Type = "grpc_health" },
			expectedErr: "endpoint_health.active_probe.type",
		},
		{
			name:        "zero interval",
			mutate:      func(c *EndpointHealthActiveProbeConfig) { c.Interval = 0 },
			expectedErr: "endpoint_health.active_probe.interval",
		},
		{
			name:        "zero timeout",
			mutate:      func(c *EndpointHealthActiveProbeConfig) { c.Timeout = 0 },
			expectedErr: "endpoint_health.active_probe.timeout",
		},
		{
			name:        "timeout not shorter than interval",
			mutate:      func(c *EndpointHealthActiveProbeConfig) { c.Timeout = c.Interval },
			expectedErr: "endpoint_health.active_probe.timeout must be shorter than endpoint_health.active_probe.interval",
		},
		{
			name:        "invalid jitter",
			mutate:      func(c *EndpointHealthActiveProbeConfig) { c.Jitter = "fast" },
			expectedErr: "endpoint_health.active_probe.jitter",
		},
		{
			name:        "jitter above one hundred percent",
			mutate:      func(c *EndpointHealthActiveProbeConfig) { c.Jitter = "101%" },
			expectedErr: "endpoint_health.active_probe.jitter",
		},
		{
			name:        "nan jitter",
			mutate:      func(c *EndpointHealthActiveProbeConfig) { c.Jitter = "NaN%" },
			expectedErr: "endpoint_health.active_probe.jitter",
		},
		{
			name:        "infinite jitter",
			mutate:      func(c *EndpointHealthActiveProbeConfig) { c.Jitter = "Inf%" },
			expectedErr: "endpoint_health.active_probe.jitter",
		},
		{
			name:        "zero max concurrency",
			mutate:      func(c *EndpointHealthActiveProbeConfig) { c.MaxConcurrency = 0 },
			expectedErr: "endpoint_health.active_probe.max_concurrency",
		},
		{
			name:        "zero fall",
			mutate:      func(c *EndpointHealthActiveProbeConfig) { c.Fall = 0 },
			expectedErr: "endpoint_health.active_probe.fall",
		},
		{
			name:        "zero rise",
			mutate:      func(c *EndpointHealthActiveProbeConfig) { c.Rise = 0 },
			expectedErr: "endpoint_health.active_probe.rise",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := createDefaultConfig().(*Config)
			cfg.EndpointHealth.Enabled = true
			cfg.EndpointHealth.ActiveProbe = validProbe
			tt.mutate(&cfg.EndpointHealth.ActiveProbe)

			require.ErrorContains(t, cfg.Validate(), tt.expectedErr)
		})
	}
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
			"active_probe": map[string]any{
				"enabled":         true,
				"type":            "tcp_connect",
				"interval":        "5s",
				"timeout":         "250ms",
				"jitter":          "20%",
				"max_concurrency": 4,
				"fall":            2,
				"rise":            2,
			},
		},
	})

	require.NoError(t, conf.Unmarshal(cfg))
	require.True(t, cfg.EndpointHealth.Enabled)
	require.Equal(t, 10*time.Second, cfg.EndpointHealth.QuarantineDuration)
	require.True(t, cfg.EndpointHealth.RerouteOnFailure)
	require.Equal(t, 1, cfg.EndpointHealth.MaxRerouteAttempts)
	require.True(t, cfg.EndpointHealth.ActiveProbe.Enabled)
	require.Equal(t, EndpointHealthActiveProbeTypeTCPConnect, cfg.EndpointHealth.ActiveProbe.Type)
	require.Equal(t, 5*time.Second, cfg.EndpointHealth.ActiveProbe.Interval)
	require.Equal(t, 250*time.Millisecond, cfg.EndpointHealth.ActiveProbe.Timeout)
	require.Equal(t, "20%", cfg.EndpointHealth.ActiveProbe.Jitter)
	require.Equal(t, 4, cfg.EndpointHealth.ActiveProbe.MaxConcurrency)
	require.Equal(t, 2, cfg.EndpointHealth.ActiveProbe.Fall)
	require.Equal(t, 2, cfg.EndpointHealth.ActiveProbe.Rise)
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
