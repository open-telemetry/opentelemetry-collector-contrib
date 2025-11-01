// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureeventhubsexporter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pipeline"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azureeventhubsexporter/internal/metadata"
)

func TestNewExporter(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		signal      pipeline.Signal
		expectError bool
	}{
		{
			name: "valid json exporter for traces",
			config: &Config{
				Auth: Authentication{
					Type:             ConnectionString,
					ConnectionString: "Endpoint=sb://test.servicebus.windows.net/;SharedAccessKeyName=RootManageSharedAccessKey;SharedAccessKey=testkey",
				},
				EventHub: TelemetryConfig{
					Traces:  "traces",
					Metrics: "metrics",
					Logs:    "logs",
				},
				FormatType: formatTypeJSON,
				PartitionKey: PartitionKeyConfig{
					Source: "random",
				},
				MaxEventSize:  1024 * 1024,
				BatchSize:     100,
				BackOffConfig: configretry.NewDefaultBackOffConfig(),
			},
			signal:      pipeline.SignalTraces,
			expectError: false,
		},
		{
			name: "valid proto exporter for logs",
			config: &Config{
				Auth: Authentication{
					Type:             ConnectionString,
					ConnectionString: "Endpoint=sb://test.servicebus.windows.net/;SharedAccessKeyName=RootManageSharedAccessKey;SharedAccessKey=testkey",
				},
				EventHub: TelemetryConfig{
					Traces:  "traces",
					Metrics: "metrics",
					Logs:    "logs",
				},
				FormatType: formatTypeProto,
				PartitionKey: PartitionKeyConfig{
					Source: "random",
				},
				MaxEventSize:  1024 * 1024,
				BatchSize:     100,
				BackOffConfig: configretry.NewDefaultBackOffConfig(),
			},
			signal:      pipeline.SignalLogs,
			expectError: false,
		},
		{
			name: "invalid format type",
			config: &Config{
				Auth: Authentication{
					Type:             ConnectionString,
					ConnectionString: "Endpoint=sb://test.servicebus.windows.net/;SharedAccessKeyName=RootManageSharedAccessKey;SharedAccessKey=testkey",
				},
				EventHub: TelemetryConfig{
					Traces:  "traces",
					Metrics: "metrics",
					Logs:    "logs",
				},
				FormatType: "xml",
				PartitionKey: PartitionKeyConfig{
					Source: "random",
				},
				MaxEventSize:  1024 * 1024,
				BatchSize:     100,
				BackOffConfig: configretry.NewDefaultBackOffConfig(),
			},
			signal:      pipeline.SignalMetrics,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			set := component.TelemetrySettings{
				Logger: zap.NewNop(),
			}
			exp, err := newExporter(tt.config, set, tt.signal)
			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, exp)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, exp)
				assert.Equal(t, tt.signal, exp.signal)
			}
		})
	}
}

func TestGeneratePartitionKey(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		setupData   func() ptrace.ResourceSpansSlice
		expectEmpty bool
		expectedKey string
	}{
		{
			name: "static partition key",
			config: &Config{
				Auth: Authentication{
					Type:             ConnectionString,
					ConnectionString: "Endpoint=sb://test.servicebus.windows.net/;SharedAccessKeyName=RootManageSharedAccessKey;SharedAccessKey=test",
				},
				EventHub: TelemetryConfig{
					Traces:  "traces",
					Metrics: "metrics",
					Logs:    "logs",
				},
				FormatType:   "json",
				MaxEventSize: 1048576,
				BatchSize:    100,
				PartitionKey: PartitionKeyConfig{
					Source: "static",
					Value:  "my-static-key",
				},
				BackOffConfig: configretry.NewDefaultBackOffConfig(),
			},
			setupData:   func() ptrace.ResourceSpansSlice { return ptrace.NewResourceSpansSlice() },
			expectEmpty: false,
			expectedKey: "my-static-key",
		},
		{
			name: "resource attribute partition key - found",
			config: &Config{
				Auth: Authentication{
					Type:             ConnectionString,
					ConnectionString: "Endpoint=sb://test.servicebus.windows.net/;SharedAccessKeyName=RootManageSharedAccessKey;SharedAccessKey=test",
				},
				EventHub: TelemetryConfig{
					Traces:  "traces",
					Metrics: "metrics",
					Logs:    "logs",
				},
				FormatType:   "json",
				MaxEventSize: 1048576,
				BatchSize:    100,
				PartitionKey: PartitionKeyConfig{
					Source: "resource_attribute",
					Value:  "service.name",
				},
				BackOffConfig: configretry.NewDefaultBackOffConfig(),
			},
			setupData: func() ptrace.ResourceSpansSlice {
				rs := ptrace.NewResourceSpansSlice()
				r := rs.AppendEmpty()
				r.Resource().Attributes().PutStr("service.name", "test-service")
				return rs
			},
			expectEmpty: false,
			expectedKey: "test-service",
		},
		{
			name: "resource attribute partition key - not found",
			config: &Config{
				Auth: Authentication{
					Type:             ConnectionString,
					ConnectionString: "Endpoint=sb://test.servicebus.windows.net/;SharedAccessKeyName=RootManageSharedAccessKey;SharedAccessKey=test",
				},
				EventHub: TelemetryConfig{
					Traces:  "traces",
					Metrics: "metrics",
					Logs:    "logs",
				},
				FormatType:   "json",
				MaxEventSize: 1048576,
				BatchSize:    100,
				PartitionKey: PartitionKeyConfig{
					Source: "resource_attribute",
					Value:  "missing.attribute",
				},
				BackOffConfig: configretry.NewDefaultBackOffConfig(),
			},
			setupData: func() ptrace.ResourceSpansSlice {
				rs := ptrace.NewResourceSpansSlice()
				r := rs.AppendEmpty()
				r.Resource().Attributes().PutStr("service.name", "test-service")
				return rs
			},
			expectEmpty: true,
		},
		{
			name: "trace_id partition key",
			config: &Config{
				Auth: Authentication{
					Type:             ConnectionString,
					ConnectionString: "Endpoint=sb://test.servicebus.windows.net/;SharedAccessKeyName=RootManageSharedAccessKey;SharedAccessKey=test",
				},
				EventHub: TelemetryConfig{
					Traces:  "traces",
					Metrics: "metrics",
					Logs:    "logs",
				},
				FormatType:   "json",
				MaxEventSize: 1048576,
				BatchSize:    100,
				PartitionKey: PartitionKeyConfig{
					Source: "trace_id",
				},
				BackOffConfig: configretry.NewDefaultBackOffConfig(),
			},
			setupData: func() ptrace.ResourceSpansSlice {
				rs := ptrace.NewResourceSpansSlice()
				r := rs.AppendEmpty()
				ss := r.ScopeSpans().AppendEmpty()
				span := ss.Spans().AppendEmpty()
				traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
				span.SetTraceID(traceID)
				return rs
			},
			expectEmpty: false,
			expectedKey: "0102030405060708090a0b0c0d0e0f10",
		},
		{
			name: "span_id partition key",
			config: &Config{
				Auth: Authentication{
					Type:             ConnectionString,
					ConnectionString: "Endpoint=sb://test.servicebus.windows.net/;SharedAccessKeyName=RootManageSharedAccessKey;SharedAccessKey=test",
				},
				EventHub: TelemetryConfig{
					Traces:  "traces",
					Metrics: "metrics",
					Logs:    "logs",
				},
				FormatType:   "json",
				MaxEventSize: 1048576,
				BatchSize:    100,
				PartitionKey: PartitionKeyConfig{
					Source: "span_id",
				},
				BackOffConfig: configretry.NewDefaultBackOffConfig(),
			},
			setupData: func() ptrace.ResourceSpansSlice {
				rs := ptrace.NewResourceSpansSlice()
				r := rs.AppendEmpty()
				ss := r.ScopeSpans().AppendEmpty()
				span := ss.Spans().AppendEmpty()
				spanID := pcommon.SpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})
				span.SetSpanID(spanID)
				return rs
			},
			expectEmpty: false,
			expectedKey: "0102030405060708",
		},
		{
			name: "random partition key",
			config: &Config{
				Auth: Authentication{
					Type:             ConnectionString,
					ConnectionString: "Endpoint=sb://test.servicebus.windows.net/;SharedAccessKeyName=RootManageSharedAccessKey;SharedAccessKey=test",
				},
				EventHub: TelemetryConfig{
					Traces:  "traces",
					Metrics: "metrics",
					Logs:    "logs",
				},
				FormatType:   "json",
				MaxEventSize: 1048576,
				BatchSize:    100,
				PartitionKey: PartitionKeyConfig{
					Source: "random",
				},
				BackOffConfig: configretry.NewDefaultBackOffConfig(),
			},
			setupData:   func() ptrace.ResourceSpansSlice { return ptrace.NewResourceSpansSlice() },
			expectEmpty: false,
		},
		{
			name: "empty source - default to empty",
			config: &Config{
				Auth: Authentication{
					Type:             ConnectionString,
					ConnectionString: "Endpoint=sb://test.servicebus.windows.net/;SharedAccessKeyName=RootManageSharedAccessKey;SharedAccessKey=test",
				},
				EventHub: TelemetryConfig{
					Traces:  "traces",
					Metrics: "metrics",
					Logs:    "logs",
				},
				FormatType:   "json",
				MaxEventSize: 1048576,
				BatchSize:    100,
				PartitionKey: PartitionKeyConfig{
					Source: "",
				},
				BackOffConfig: configretry.NewDefaultBackOffConfig(),
			},
			setupData:   func() ptrace.ResourceSpansSlice { return ptrace.NewResourceSpansSlice() },
			expectEmpty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			set := component.TelemetrySettings{
				Logger: zap.NewNop(),
			}
			exp, err := newExporter(tt.config, set, pipeline.SignalTraces)
			assert.NoError(t, err)

			data := tt.setupData()
			key := exp.generatePartitionKey(data)

			if tt.expectEmpty {
				assert.Empty(t, key)
			} else {
				assert.NotEmpty(t, key)
				if tt.expectedKey != "" {
					assert.Equal(t, tt.expectedKey, key)
				}
			}
		})
	}
}

func TestGeneratePartitionKeyFromLogs(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		setupData   func() plog.ResourceLogsSlice
		expectEmpty bool
		expectedKey string
	}{
		{
			name: "static partition key",
			config: &Config{
				Auth: Authentication{
					Type:             ConnectionString,
					ConnectionString: "Endpoint=sb://test.servicebus.windows.net/;SharedAccessKeyName=RootManageSharedAccessKey;SharedAccessKey=test",
				},
				EventHub: TelemetryConfig{
					Traces:  "traces",
					Metrics: "metrics",
					Logs:    "logs",
				},
				FormatType:   "json",
				MaxEventSize: 1048576,
				BatchSize:    100,
				PartitionKey: PartitionKeyConfig{
					Source: "static",
					Value:  "log-partition",
				},
				BackOffConfig: configretry.NewDefaultBackOffConfig(),
			},
			setupData:   func() plog.ResourceLogsSlice { return plog.NewResourceLogsSlice() },
			expectEmpty: false,
			expectedKey: "log-partition",
		},
		{
			name: "resource attribute partition key",
			config: &Config{
				Auth: Authentication{
					Type:             ConnectionString,
					ConnectionString: "Endpoint=sb://test.servicebus.windows.net/;SharedAccessKeyName=RootManageSharedAccessKey;SharedAccessKey=test",
				},
				EventHub: TelemetryConfig{
					Traces:  "traces",
					Metrics: "metrics",
					Logs:    "logs",
				},
				FormatType:   "json",
				MaxEventSize: 1048576,
				BatchSize:    100,
				PartitionKey: PartitionKeyConfig{
					Source: "resource_attribute",
					Value:  "host.name",
				},
				BackOffConfig: configretry.NewDefaultBackOffConfig(),
			},
			setupData: func() plog.ResourceLogsSlice {
				rl := plog.NewResourceLogsSlice()
				r := rl.AppendEmpty()
				r.Resource().Attributes().PutStr("host.name", "test-host")
				return rl
			},
			expectEmpty: false,
			expectedKey: "test-host",
		},
		{
			name: "trace_id partition key from log",
			config: &Config{
				Auth: Authentication{
					Type:             ConnectionString,
					ConnectionString: "Endpoint=sb://test.servicebus.windows.net/;SharedAccessKeyName=RootManageSharedAccessKey;SharedAccessKey=test",
				},
				EventHub: TelemetryConfig{
					Traces:  "traces",
					Metrics: "metrics",
					Logs:    "logs",
				},
				FormatType:   "json",
				MaxEventSize: 1048576,
				BatchSize:    100,
				PartitionKey: PartitionKeyConfig{
					Source: "trace_id",
				},
				BackOffConfig: configretry.NewDefaultBackOffConfig(),
			},
			setupData: func() plog.ResourceLogsSlice {
				rl := plog.NewResourceLogsSlice()
				r := rl.AppendEmpty()
				sl := r.ScopeLogs().AppendEmpty()
				log := sl.LogRecords().AppendEmpty()
				traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
				log.SetTraceID(traceID)
				return rl
			},
			expectEmpty: false,
			expectedKey: "0102030405060708090a0b0c0d0e0f10",
		},
		{
			name: "random partition key",
			config: &Config{
				Auth: Authentication{
					Type:             ConnectionString,
					ConnectionString: "Endpoint=sb://test.servicebus.windows.net/;SharedAccessKeyName=RootManageSharedAccessKey;SharedAccessKey=test",
				},
				EventHub: TelemetryConfig{
					Traces:  "traces",
					Metrics: "metrics",
					Logs:    "logs",
				},
				FormatType:   "json",
				MaxEventSize: 1048576,
				BatchSize:    100,
				PartitionKey: PartitionKeyConfig{
					Source: "random",
				},
				BackOffConfig: configretry.NewDefaultBackOffConfig(),
			},
			setupData:   func() plog.ResourceLogsSlice { return plog.NewResourceLogsSlice() },
			expectEmpty: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			set := component.TelemetrySettings{
				Logger: zap.NewNop(),
			}
			exp, err := newExporter(tt.config, set, pipeline.SignalLogs)
			assert.NoError(t, err)

			data := tt.setupData()
			key := exp.generatePartitionKeyFromLogs(data)

			if tt.expectEmpty {
				assert.Empty(t, key)
			} else {
				assert.NotEmpty(t, key)
				if tt.expectedKey != "" {
					assert.Equal(t, tt.expectedKey, key)
				}
			}
		})
	}
}

func TestGeneratePartitionKeyFromMetrics(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		setupData   func() pmetric.ResourceMetricsSlice
		expectEmpty bool
		expectedKey string
	}{
		{
			name: "static partition key",
			config: &Config{
				Auth: Authentication{
					Type:             ConnectionString,
					ConnectionString: "Endpoint=sb://test.servicebus.windows.net/;SharedAccessKeyName=RootManageSharedAccessKey;SharedAccessKey=test",
				},
				EventHub: TelemetryConfig{
					Traces:  "traces",
					Metrics: "metrics",
					Logs:    "logs",
				},
				FormatType:   "json",
				MaxEventSize: 1048576,
				BatchSize:    100,
				PartitionKey: PartitionKeyConfig{
					Source: "static",
					Value:  "metric-partition",
				},
				BackOffConfig: configretry.NewDefaultBackOffConfig(),
			},
			setupData:   func() pmetric.ResourceMetricsSlice { return pmetric.NewResourceMetricsSlice() },
			expectEmpty: false,
			expectedKey: "metric-partition",
		},
		{
			name: "resource attribute partition key",
			config: &Config{
				Auth: Authentication{
					Type:             ConnectionString,
					ConnectionString: "Endpoint=sb://test.servicebus.windows.net/;SharedAccessKeyName=RootManageSharedAccessKey;SharedAccessKey=test",
				},
				EventHub: TelemetryConfig{
					Traces:  "traces",
					Metrics: "metrics",
					Logs:    "logs",
				},
				FormatType:   "json",
				MaxEventSize: 1048576,
				BatchSize:    100,
				PartitionKey: PartitionKeyConfig{
					Source: "resource_attribute",
					Value:  "environment",
				},
				BackOffConfig: configretry.NewDefaultBackOffConfig(),
			},
			setupData: func() pmetric.ResourceMetricsSlice {
				rm := pmetric.NewResourceMetricsSlice()
				r := rm.AppendEmpty()
				r.Resource().Attributes().PutStr("environment", "production")
				return rm
			},
			expectEmpty: false,
			expectedKey: "production",
		},
		{
			name: "random partition key",
			config: &Config{
				Auth: Authentication{
					Type:             ConnectionString,
					ConnectionString: "Endpoint=sb://test.servicebus.windows.net/;SharedAccessKeyName=RootManageSharedAccessKey;SharedAccessKey=test",
				},
				EventHub: TelemetryConfig{
					Traces:  "traces",
					Metrics: "metrics",
					Logs:    "logs",
				},
				FormatType:   "json",
				MaxEventSize: 1048576,
				BatchSize:    100,
				PartitionKey: PartitionKeyConfig{
					Source: "random",
				},
				BackOffConfig: configretry.NewDefaultBackOffConfig(),
			},
			setupData:   func() pmetric.ResourceMetricsSlice { return pmetric.NewResourceMetricsSlice() },
			expectEmpty: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			set := component.TelemetrySettings{
				Logger: zap.NewNop(),
			}
			exp, err := newExporter(tt.config, set, pipeline.SignalMetrics)
			assert.NoError(t, err)

			data := tt.setupData()
			key := exp.generatePartitionKeyFromMetrics(data)

			if tt.expectEmpty {
				assert.Empty(t, key)
			} else {
				assert.NotEmpty(t, key)
				if tt.expectedKey != "" {
					assert.Equal(t, tt.expectedKey, key)
				}
			}
		})
	}
}

func TestGenerateRandomPartitionKey(t *testing.T) {
	set := component.TelemetrySettings{
		Logger: zap.NewNop(),
	}
	config := &Config{
		Auth: Authentication{
			Type:             ConnectionString,
			ConnectionString: "Endpoint=sb://test.servicebus.windows.net/;SharedAccessKeyName=RootManageSharedAccessKey;SharedAccessKey=test",
		},
		EventHub: TelemetryConfig{
			Traces:  "traces",
			Metrics: "metrics",
			Logs:    "logs",
		},
		FormatType:   "json",
		MaxEventSize: 1048576,
		BatchSize:    100,
		PartitionKey: PartitionKeyConfig{
			Source: "random",
		},
		BackOffConfig: configretry.NewDefaultBackOffConfig(),
	}
	exp, err := newExporter(config, set, pipeline.SignalTraces)
	assert.NoError(t, err)

	// Generate multiple keys and ensure they're different (very high probability)
	keys := make(map[string]bool)
	for i := 0; i < 10; i++ {
		key := exp.generateRandomPartitionKey()
		assert.NotEmpty(t, key)
		keys[key] = true
	}

	// With random generation, we should have multiple unique keys
	assert.Greater(t, len(keys), 1, "Random partition keys should be unique")
}

func TestCreateMarshallerInvalidFormat(t *testing.T) {
	config := &Config{
		FormatType: "invalid",
	}

	_, err := createMarshaller(config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported format type")
}

func TestCreateMarshallerJSON(t *testing.T) {
	config := &Config{
		FormatType: "json",
	}

	m, err := createMarshaller(config)
	assert.NoError(t, err)
	assert.NotNil(t, m)
}

func TestCreateMarshallerProto(t *testing.T) {
	config := &Config{
		FormatType: "proto",
	}

	m, err := createMarshaller(config)
	assert.NoError(t, err)
	assert.NotNil(t, m)
}

func TestNewExporterWithInvalidConfig(t *testing.T) {
	set := component.TelemetrySettings{
		Logger: zap.NewNop(),
	}

	tests := []struct {
		name        string
		config      *Config
		expectedErr string
	}{
		{
			name: "missing namespace for non-connection-string auth",
			config: &Config{
				Auth: Authentication{
					Type: SystemManagedIdentity,
				},
				EventHub: TelemetryConfig{
					Traces:  "traces",
					Metrics: "metrics",
					Logs:    "logs",
				},
				FormatType:   "json",
				MaxEventSize: 1048576,
				BatchSize:    100,
			},
			expectedErr: "namespace cannot be empty",
		},
		{
			name: "invalid format type",
			config: &Config{
				Auth: Authentication{
					Type:             ConnectionString,
					ConnectionString: "Endpoint=sb://test.servicebus.windows.net/;SharedAccessKeyName=test;SharedAccessKey=test",
				},
				EventHub: TelemetryConfig{
					Traces:  "traces",
					Metrics: "metrics",
					Logs:    "logs",
				},
				FormatType:   "xml",
				MaxEventSize: 1048576,
				BatchSize:    100,
			},
			expectedErr: "unknown format type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := newExporter(tt.config, set, pipeline.SignalTraces)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedErr)
		})
	}
}

func TestGeneratePartitionKeyEmptyData(t *testing.T) {
	set := component.TelemetrySettings{
		Logger: zap.NewNop(),
	}
	config := &Config{
		Auth: Authentication{
			Type:             ConnectionString,
			ConnectionString: "Endpoint=sb://test.servicebus.windows.net/;SharedAccessKeyName=test;SharedAccessKey=test",
		},
		EventHub: TelemetryConfig{
			Traces:  "traces",
			Metrics: "metrics",
			Logs:    "logs",
		},
		FormatType:   "json",
		MaxEventSize: 1048576,
		BatchSize:    100,
		PartitionKey: PartitionKeyConfig{
			Source: "trace_id",
		},
		BackOffConfig: configretry.NewDefaultBackOffConfig(),
	}
	exp, err := newExporter(config, set, pipeline.SignalTraces)
	assert.NoError(t, err)

	// Test with empty resource spans
	emptySpans := ptrace.NewResourceSpansSlice()
	key := exp.generatePartitionKey(emptySpans)
	assert.Empty(t, key)
}

func TestGeneratePartitionKeyFromLogsEmptyData(t *testing.T) {
	set := component.TelemetrySettings{
		Logger: zap.NewNop(),
	}
	config := &Config{
		Auth: Authentication{
			Type:             ConnectionString,
			ConnectionString: "Endpoint=sb://test.servicebus.windows.net/;SharedAccessKeyName=test;SharedAccessKey=test",
		},
		EventHub: TelemetryConfig{
			Traces:  "traces",
			Metrics: "metrics",
			Logs:    "logs",
		},
		FormatType:   "json",
		MaxEventSize: 1048576,
		BatchSize:    100,
		PartitionKey: PartitionKeyConfig{
			Source: "trace_id",
		},
		BackOffConfig: configretry.NewDefaultBackOffConfig(),
	}
	exp, err := newExporter(config, set, pipeline.SignalLogs)
	assert.NoError(t, err)

	// Test with empty resource logs
	emptyLogs := plog.NewResourceLogsSlice()
	key := exp.generatePartitionKeyFromLogs(emptyLogs)
	assert.Empty(t, key)
}

func TestGeneratePartitionKeyFromMetricsEmptyData(t *testing.T) {
	set := component.TelemetrySettings{
		Logger: zap.NewNop(),
	}
	config := &Config{
		Auth: Authentication{
			Type:             ConnectionString,
			ConnectionString: "Endpoint=sb://test.servicebus.windows.net/;SharedAccessKeyName=test;SharedAccessKey=test",
		},
		EventHub: TelemetryConfig{
			Traces:  "traces",
			Metrics: "metrics",
			Logs:    "logs",
		},
		FormatType:   "json",
		MaxEventSize: 1048576,
		BatchSize:    100,
		PartitionKey: PartitionKeyConfig{
			Source: "resource_attribute",
			Value:  "test",
		},
		BackOffConfig: configretry.NewDefaultBackOffConfig(),
	}
	exp, err := newExporter(config, set, pipeline.SignalMetrics)
	assert.NoError(t, err)

	// Test with empty resource metrics
	emptyMetrics := pmetric.NewResourceMetricsSlice()
	key := exp.generatePartitionKeyFromMetrics(emptyMetrics)
	assert.Empty(t, key)
}

func TestGeneratePartitionKeyUnknownSource(t *testing.T) {
	set := component.TelemetrySettings{
		Logger: zap.NewNop(),
	}
	config := &Config{
		Auth: Authentication{
			Type:             ConnectionString,
			ConnectionString: "Endpoint=sb://test.servicebus.windows.net/;SharedAccessKeyName=test;SharedAccessKey=test",
		},
		EventHub: TelemetryConfig{
			Traces:  "traces",
			Metrics: "metrics",
			Logs:    "logs",
		},
		FormatType:   "json",
		MaxEventSize: 1048576,
		BatchSize:    100,
		PartitionKey: PartitionKeyConfig{
			Source: "", // empty/unknown source
		},
		BackOffConfig: configretry.NewDefaultBackOffConfig(),
	}
	exp, err := newExporter(config, set, pipeline.SignalTraces)
	assert.NoError(t, err)

	spans := ptrace.NewResourceSpansSlice()
	key := exp.generatePartitionKey(spans)
	assert.Empty(t, key)
}

func TestGeneratePartitionKeyResourceAttributeNotFound(t *testing.T) {
	set := component.TelemetrySettings{
		Logger: zap.NewNop(),
	}
	config := &Config{
		Auth: Authentication{
			Type:             ConnectionString,
			ConnectionString: "Endpoint=sb://test.servicebus.windows.net/;SharedAccessKeyName=test;SharedAccessKey=test",
		},
		EventHub: TelemetryConfig{
			Traces:  "traces",
			Metrics: "metrics",
			Logs:    "logs",
		},
		FormatType:   "json",
		MaxEventSize: 1048576,
		BatchSize:    100,
		PartitionKey: PartitionKeyConfig{
			Source: "resource_attribute",
			Value:  "nonexistent.attribute",
		},
		BackOffConfig: configretry.NewDefaultBackOffConfig(),
	}
	exp, err := newExporter(config, set, pipeline.SignalTraces)
	assert.NoError(t, err)

	spans := ptrace.NewResourceSpansSlice()
	rs := spans.AppendEmpty()
	rs.Resource().Attributes().PutStr("different.attribute", "value")

	key := exp.generatePartitionKey(spans)
	assert.Empty(t, key)
}

func TestCreateLogsExporterSuccess(t *testing.T) {
	config := &Config{
		Auth: Authentication{
			Type:             ConnectionString,
			ConnectionString: "Endpoint=sb://test.servicebus.windows.net/;SharedAccessKeyName=test;SharedAccessKey=test",
		},
		EventHub: TelemetryConfig{
			Traces:  "traces",
			Metrics: "metrics",
			Logs:    "logs",
		},
		FormatType:   "json",
		MaxEventSize: 1048576,
		BatchSize:    100,
		BackOffConfig: configretry.NewDefaultBackOffConfig(),
	}

	exp, err := createLogsExporter(
		context.Background(),
		exportertest.NewNopSettings(metadata.Type),
		config,
	)
	assert.NoError(t, err)
	assert.NotNil(t, exp)
}

func TestCreateMetricsExporterSuccess(t *testing.T) {
	config := &Config{
		Auth: Authentication{
			Type:             ConnectionString,
			ConnectionString: "Endpoint=sb://test.servicebus.windows.net/;SharedAccessKeyName=test;SharedAccessKey=test",
		},
		EventHub: TelemetryConfig{
			Traces:  "traces",
			Metrics: "metrics",
			Logs:    "logs",
		},
		FormatType:   "json",
		MaxEventSize: 1048576,
		BatchSize:    100,
		BackOffConfig: configretry.NewDefaultBackOffConfig(),
	}

	exp, err := createMetricsExporter(
		context.Background(),
		exportertest.NewNopSettings(metadata.Type),
		config,
	)
	assert.NoError(t, err)
	assert.NotNil(t, exp)
}

func TestCreateLogsExporterWithInvalidConfig(t *testing.T) {
	config := &Config{
		Auth: Authentication{
			Type: ServicePrincipal,
			// Missing required fields
		},
		FormatType:   "json",
		MaxEventSize: 1048576,
		BatchSize:    100,
	}

	_, err := createLogsExporter(
		context.Background(),
		exportertest.NewNopSettings(metadata.Type),
		config,
	)
	assert.Error(t, err)
}

func TestCreateMetricsExporterWithInvalidConfig(t *testing.T) {
	config := &Config{
		Auth: Authentication{
			Type: ServicePrincipal,
			// Missing required fields
		},
		FormatType:   "json",
		MaxEventSize: 1048576,
		BatchSize:    100,
	}

	_, err := createMetricsExporter(
		context.Background(),
		exportertest.NewNopSettings(metadata.Type),
		config,
	)
	assert.Error(t, err)
}
