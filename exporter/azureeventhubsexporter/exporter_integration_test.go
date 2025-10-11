// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureeventhubsexporter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pipeline"
	"go.uber.org/zap"
)

// TestCreateEventHubsClient tests the client creation logic for different signal types
func TestCreateEventHubsClient(t *testing.T) {
	tests := []struct {
		name        string
		signal      pipeline.Signal
		config      *Config
		expectError bool
		errorMsg    string
	}{
		{
			name:   "connection_string for traces",
			signal: pipeline.SignalTraces,
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
				FormatType:    "json",
				MaxEventSize:  1048576,
				BatchSize:     100,
				BackOffConfig: configretry.NewDefaultBackOffConfig(),
			},
			expectError: false,
		},
		{
			name:   "connection_string for logs",
			signal: pipeline.SignalLogs,
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
				FormatType:    "json",
				MaxEventSize:  1048576,
				BatchSize:     100,
				BackOffConfig: configretry.NewDefaultBackOffConfig(),
			},
			expectError: false,
		},
		{
			name:   "connection_string for metrics",
			signal: pipeline.SignalMetrics,
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
				FormatType:    "json",
				MaxEventSize:  1048576,
				BatchSize:     100,
				BackOffConfig: configretry.NewDefaultBackOffConfig(),
			},
			expectError: false,
		},
		{
			name:   "invalid connection string",
			signal: pipeline.SignalTraces,
			config: &Config{
				Auth: Authentication{
					Type:             ConnectionString,
					ConnectionString: "invalid-connection-string",
				},
				EventHub: TelemetryConfig{
					Traces:  "traces",
					Metrics: "metrics",
					Logs:    "logs",
				},
				FormatType:    "json",
				MaxEventSize:  1048576,
				BatchSize:     100,
				BackOffConfig: configretry.NewDefaultBackOffConfig(),
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			set := component.TelemetrySettings{
				Logger: zap.NewNop(),
			}
			exp, err := newExporter(tt.config, set, tt.signal)
			require.NoError(t, err)

			client, err := exp.createEventHubsClient()
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, client)
				// Clean up
				if client != nil {
					_ = client.Close(context.Background())
				}
			}
		})
	}
}

// TestPushTracesWithMarshalError tests error handling when marshalling fails
func TestPushTracesWithMarshalError(t *testing.T) {
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
		FormatType:    "invalid", // This will cause createMarshaller to fail
		MaxEventSize:  1048576,
		BatchSize:     100,
		BackOffConfig: configretry.NewDefaultBackOffConfig(),
	}

	set := component.TelemetrySettings{
		Logger: zap.NewNop(),
	}

	// This should fail during newExporter due to invalid format
	_, err := newExporter(config, set, pipeline.SignalTraces)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown format type")
}

// TestGeneratePartitionKeyEdgeCases tests edge cases in partition key generation
func TestGeneratePartitionKeyEdgeCases(t *testing.T) {
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
		FormatType:    "json",
		MaxEventSize:  1048576,
		BatchSize:     100,
		BackOffConfig: configretry.NewDefaultBackOffConfig(),
	}

	tests := []struct {
		name          string
		partitionKey  PartitionKeyConfig
		setupData     func() ptrace.ResourceSpansSlice
		expectedEmpty bool
	}{
		{
			name: "trace_id with empty spans",
			partitionKey: PartitionKeyConfig{
				Source: "trace_id",
			},
			setupData: func() ptrace.ResourceSpansSlice {
				rs := ptrace.NewResourceSpansSlice()
				rs.AppendEmpty()
				return rs
			},
			expectedEmpty: true,
		},
		{
			name: "span_id with empty spans",
			partitionKey: PartitionKeyConfig{
				Source: "span_id",
			},
			setupData: func() ptrace.ResourceSpansSlice {
				rs := ptrace.NewResourceSpansSlice()
				rs.AppendEmpty()
				return rs
			},
			expectedEmpty: true,
		},
		{
			name: "resource_attribute with non-string value",
			partitionKey: PartitionKeyConfig{
				Source: "resource_attribute",
				Value:  "int.value",
			},
			setupData: func() ptrace.ResourceSpansSlice {
				rs := ptrace.NewResourceSpansSlice()
				r := rs.AppendEmpty()
				r.Resource().Attributes().PutInt("int.value", 123)
				return rs
			},
			expectedEmpty: false, // AsString() will convert it
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config.PartitionKey = tt.partitionKey
			set := component.TelemetrySettings{
				Logger: zap.NewNop(),
			}
			exp, err := newExporter(config, set, pipeline.SignalTraces)
			require.NoError(t, err)

			data := tt.setupData()
			key := exp.generatePartitionKey(data)

			if tt.expectedEmpty {
				assert.Empty(t, key)
			} else {
				assert.NotEmpty(t, key)
			}
		})
	}
}

// TestGeneratePartitionKeyFromLogsEdgeCases tests edge cases for logs
func TestGeneratePartitionKeyFromLogsEdgeCases(t *testing.T) {
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
		FormatType:    "json",
		MaxEventSize:  1048576,
		BatchSize:     100,
		BackOffConfig: configretry.NewDefaultBackOffConfig(),
	}

	tests := []struct {
		name          string
		partitionKey  PartitionKeyConfig
		setupData     func() plog.ResourceLogsSlice
		expectedEmpty bool
	}{
		{
			name: "trace_id with empty logs",
			partitionKey: PartitionKeyConfig{
				Source: "trace_id",
			},
			setupData: func() plog.ResourceLogsSlice {
				rl := plog.NewResourceLogsSlice()
				rl.AppendEmpty()
				return rl
			},
			expectedEmpty: true,
		},
		{
			name: "trace_id with valid log",
			partitionKey: PartitionKeyConfig{
				Source: "trace_id",
			},
			setupData: func() plog.ResourceLogsSlice {
				rl := plog.NewResourceLogsSlice()
				r := rl.AppendEmpty()
				sl := r.ScopeLogs().AppendEmpty()
				lr := sl.LogRecords().AppendEmpty()
				traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
				lr.SetTraceID(traceID)
				return rl
			},
			expectedEmpty: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config.PartitionKey = tt.partitionKey
			set := component.TelemetrySettings{
				Logger: zap.NewNop(),
			}
			exp, err := newExporter(config, set, pipeline.SignalLogs)
			require.NoError(t, err)

			data := tt.setupData()
			key := exp.generatePartitionKeyFromLogs(data)

			if tt.expectedEmpty {
				assert.Empty(t, key)
			} else {
				assert.NotEmpty(t, key)
			}
		})
	}
}

// TestGeneratePartitionKeyFromMetricsEdgeCases tests edge cases for metrics
func TestGeneratePartitionKeyFromMetricsEdgeCases(t *testing.T) {
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
		FormatType:    "json",
		MaxEventSize:  1048576,
		BatchSize:     100,
		BackOffConfig: configretry.NewDefaultBackOffConfig(),
	}

	tests := []struct {
		name          string
		partitionKey  PartitionKeyConfig
		setupData     func() pmetric.ResourceMetricsSlice
		expectedEmpty bool
	}{
		{
			name: "resource_attribute with empty metrics",
			partitionKey: PartitionKeyConfig{
				Source: "resource_attribute",
				Value:  "missing",
			},
			setupData: func() pmetric.ResourceMetricsSlice {
				rm := pmetric.NewResourceMetricsSlice()
				rm.AppendEmpty()
				return rm
			},
			expectedEmpty: true,
		},
		{
			name: "random with empty metrics",
			partitionKey: PartitionKeyConfig{
				Source: "random",
			},
			setupData: func() pmetric.ResourceMetricsSlice {
				return pmetric.NewResourceMetricsSlice()
			},
			expectedEmpty: false, // random always returns something
		},
		{
			name: "unknown source",
			partitionKey: PartitionKeyConfig{
				Source: "",
			},
			setupData: func() pmetric.ResourceMetricsSlice {
				return pmetric.NewResourceMetricsSlice()
			},
			expectedEmpty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config.PartitionKey = tt.partitionKey
			set := component.TelemetrySettings{
				Logger: zap.NewNop(),
			}
			exp, err := newExporter(config, set, pipeline.SignalMetrics)
			require.NoError(t, err)

			data := tt.setupData()
			key := exp.generatePartitionKeyFromMetrics(data)

			if tt.expectedEmpty {
				assert.Empty(t, key)
			} else {
				assert.NotEmpty(t, key)
			}
		})
	}
}

// TestShutdownWithoutClient tests shutdown when client is nil
func TestShutdownWithoutClient(t *testing.T) {
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
		FormatType:    "json",
		MaxEventSize:  1048576,
		BatchSize:     100,
		BackOffConfig: configretry.NewDefaultBackOffConfig(),
		PartitionKey: PartitionKeyConfig{
			Source: "random",
		},
	}

	set := component.TelemetrySettings{
		Logger: zap.NewNop(),
	}

	exp, err := newExporter(config, set, pipeline.SignalTraces)
	require.NoError(t, err)
	require.Nil(t, exp.client)

	// Shutdown should not fail even if client is nil
	err = exp.shutdown(context.Background())
	assert.NoError(t, err)
}

// TestNewExporterWithInvalidFormat tests creation with invalid format
func TestNewExporterWithInvalidFormat(t *testing.T) {
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
		FormatType:    "xml", // Invalid format
		MaxEventSize:  1048576,
		BatchSize:     100,
		BackOffConfig: configretry.NewDefaultBackOffConfig(),
	}

	set := component.TelemetrySettings{
		Logger: zap.NewNop(),
	}

	// Should fail validation first
	_, err := newExporter(config, set, pipeline.SignalTraces)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown format type")
}

// TestGeneratePartitionKeyFromLogsAllCases tests all remaining cases
func TestGeneratePartitionKeyFromLogsAllCases(t *testing.T) {
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
		FormatType:    "json",
		MaxEventSize:  1048576,
		BatchSize:     100,
		BackOffConfig: configretry.NewDefaultBackOffConfig(),
	}

	tests := []struct {
		name          string
		partitionKey  PartitionKeyConfig
		setupData     func() plog.ResourceLogsSlice
		expectedEmpty bool
	}{
		{
			name: "trace_id with no scope logs",
			partitionKey: PartitionKeyConfig{
				Source: "trace_id",
			},
			setupData: func() plog.ResourceLogsSlice {
				rl := plog.NewResourceLogsSlice()
				rl.AppendEmpty()
				// No ScopeLogs added
				return rl
			},
			expectedEmpty: true,
		},
		{
			name: "trace_id with no log records",
			partitionKey: PartitionKeyConfig{
				Source: "trace_id",
			},
			setupData: func() plog.ResourceLogsSlice {
				rl := plog.NewResourceLogsSlice()
				r := rl.AppendEmpty()
				r.ScopeLogs().AppendEmpty()
				// No LogRecords added
				return rl
			},
			expectedEmpty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config.PartitionKey = tt.partitionKey
			set := component.TelemetrySettings{
				Logger: zap.NewNop(),
			}
			exp, err := newExporter(config, set, pipeline.SignalLogs)
			require.NoError(t, err)

			data := tt.setupData()
			key := exp.generatePartitionKeyFromLogs(data)

			if tt.expectedEmpty {
				assert.Empty(t, key)
			} else {
				assert.NotEmpty(t, key)
			}
		})
	}
}

// TestNewExporterWithProtoFormat tests creating exporter with proto format
func TestNewExporterWithProtoFormat(t *testing.T) {
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
		FormatType:    "proto",
		MaxEventSize:  1048576,
		BatchSize:     100,
		BackOffConfig: configretry.NewDefaultBackOffConfig(),
		PartitionKey: PartitionKeyConfig{
			Source: "random",
		},
	}

	set := component.TelemetrySettings{
		Logger: zap.NewNop(),
	}

	exp, err := newExporter(config, set, pipeline.SignalTraces)
	assert.NoError(t, err)
	assert.NotNil(t, exp)
	assert.NotNil(t, exp.marshaller)
}
