// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureeventhubsexporter

import (
	"context"
	"errors"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azeventhubs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pipeline"
	"go.uber.org/zap"
)

// Helper function to create a mock exporter with a mock client
func newMockExporter(t *testing.T, config *Config, signal pipeline.Signal) (*azureEventHubsExporter, *mockEventHubProducerClient) {
	set := component.TelemetrySettings{
		Logger: zap.NewNop(),
	}

	exp, err := newExporter(config, set, signal)
	require.NoError(t, err)

	mockClient := &mockEventHubProducerClient{}
	exp.client = mockClient

	return exp, mockClient
}

func TestStartWithMockClient(t *testing.T) {
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

	// Start should create the client
	err = exp.start(context.Background(), componenttest.NewNopHost())
	assert.NoError(t, err)
	assert.NotNil(t, exp.client)

	// Cleanup
	if exp.client != nil {
		_ = exp.client.Close(context.Background())
	}
}

func TestShutdownWithMockClient(t *testing.T) {
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

	exp, mockClient := newMockExporter(t, config, pipeline.SignalTraces)

	// Expect Close to be called
	mockClient.On("Close", mock.Anything).Return(nil)

	err := exp.shutdown(context.Background())
	assert.NoError(t, err)

	mockClient.AssertExpectations(t)
}

func TestShutdownWithMockClientError(t *testing.T) {
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

	exp, mockClient := newMockExporter(t, config, pipeline.SignalTraces)

	// Expect Close to return an error
	mockClient.On("Close", mock.Anything).Return(errors.New("close error"))

	err := exp.shutdown(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "close error")

	mockClient.AssertExpectations(t)
}

func TestPushTracesSuccess(t *testing.T) {
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
			Source: "static",
			Value:  "test-key",
		},
	}

	exp, mockClient := newMockExporter(t, config, pipeline.SignalTraces)

	// Create test trace data
	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()
	span := ss.Spans().AppendEmpty()
	span.SetName("test-span")
	span.SetTraceID(pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}))

	// Create mock batch
	mockBatch := &mockEventDataBatch{maxBytes: 1048576}
	mockBatch.On("AddEventData", mock.Anything, mock.Anything).Return(nil)

	// Mock the batch creation and sending
	mockClient.On("NewEventDataBatch", mock.Anything, mock.Anything).Return(mockBatch, nil)
	mockClient.On("SendEventDataBatch", mock.Anything, mockBatch, mock.Anything).Return(nil)

	err := exp.pushTraces(context.Background(), traces)
	assert.NoError(t, err)

	mockClient.AssertExpectations(t)
	mockBatch.AssertExpectations(t)
}

func TestPushLogsSuccess(t *testing.T) {
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

	exp, mockClient := newMockExporter(t, config, pipeline.SignalLogs)

	// Create test log data
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()
	logRecord := sl.LogRecords().AppendEmpty()
	logRecord.Body().SetStr("test log message")

	// Create mock batch
	mockBatch := &mockEventDataBatch{maxBytes: 1048576}
	mockBatch.On("AddEventData", mock.Anything, mock.Anything).Return(nil)

	// Mock the batch creation and sending
	mockClient.On("NewEventDataBatch", mock.Anything, mock.Anything).Return(mockBatch, nil)
	mockClient.On("SendEventDataBatch", mock.Anything, mockBatch, mock.Anything).Return(nil)

	err := exp.pushLogs(context.Background(), logs)
	assert.NoError(t, err)

	mockClient.AssertExpectations(t)
	mockBatch.AssertExpectations(t)
}

func TestPushMetricsSuccess(t *testing.T) {
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

	exp, mockClient := newMockExporter(t, config, pipeline.SignalMetrics)

	// Create test metric data
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	metric := sm.Metrics().AppendEmpty()
	metric.SetName("test-metric")

	// Create mock batch
	mockBatch := &mockEventDataBatch{maxBytes: 1048576}
	mockBatch.On("AddEventData", mock.Anything, mock.Anything).Return(nil)

	// Mock the batch creation and sending
	mockClient.On("NewEventDataBatch", mock.Anything, mock.Anything).Return(mockBatch, nil)
	mockClient.On("SendEventDataBatch", mock.Anything, mockBatch, mock.Anything).Return(nil)

	err := exp.pushMetrics(context.Background(), metrics)
	assert.NoError(t, err)

	mockClient.AssertExpectations(t)
	mockBatch.AssertExpectations(t)
}

func TestSendEventBatchCreationError(t *testing.T) {
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

	exp, mockClient := newMockExporter(t, config, pipeline.SignalTraces)

	// Create test trace data
	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()
	span := ss.Spans().AppendEmpty()
	span.SetName("test-span")

	// Mock batch creation to return an error
	mockClient.On("NewEventDataBatch", mock.Anything, mock.Anything).Return(nil, errors.New("batch creation error"))

	err := exp.pushTraces(context.Background(), traces)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create event batch")

	mockClient.AssertExpectations(t)
}

func TestSendEventSendError(t *testing.T) {
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

	exp, mockClient := newMockExporter(t, config, pipeline.SignalTraces)

	// Create test trace data
	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()
	span := ss.Spans().AppendEmpty()
	span.SetName("test-span")

	// Create mock batch
	mockBatch := &mockEventDataBatch{maxBytes: 1048576}
	mockBatch.On("AddEventData", mock.Anything, mock.Anything).Return(nil)

	// Mock batch creation success but send failure
	mockClient.On("NewEventDataBatch", mock.Anything, mock.Anything).Return(mockBatch, nil)
	mockClient.On("SendEventDataBatch", mock.Anything, mockBatch, mock.Anything).Return(errors.New("send error"))

	err := exp.pushTraces(context.Background(), traces)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to send event batch")

	mockClient.AssertExpectations(t)
	mockBatch.AssertExpectations(t)
}

func TestSendEventWithPartitionKey(t *testing.T) {
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
			Source: "static",
			Value:  "my-partition",
		},
	}

	exp, mockClient := newMockExporter(t, config, pipeline.SignalTraces)

	// Create test trace data
	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()
	span := ss.Spans().AppendEmpty()
	span.SetName("test-span")

	// Create mock batch
	mockBatch := &mockEventDataBatch{maxBytes: 1048576}
	mockBatch.On("AddEventData", mock.Anything, mock.Anything).Return(nil)

	// Verify that NewEventDataBatch is called with partition key options
	mockClient.On("NewEventDataBatch", mock.Anything, mock.MatchedBy(func(opts *azeventhubs.EventDataBatchOptions) bool {
		return opts != nil && opts.PartitionKey != nil && *opts.PartitionKey == "my-partition"
	})).Return(mockBatch, nil)
	mockClient.On("SendEventDataBatch", mock.Anything, mockBatch, mock.Anything).Return(nil)

	err := exp.pushTraces(context.Background(), traces)
	assert.NoError(t, err)

	mockClient.AssertExpectations(t)
	mockBatch.AssertExpectations(t)
}

func TestSendEventWithoutPartitionKey(t *testing.T) {
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
			Source: "", // Empty source means no partition key
		},
	}

	exp, mockClient := newMockExporter(t, config, pipeline.SignalTraces)

	// Create test trace data
	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()
	span := ss.Spans().AppendEmpty()
	span.SetName("test-span")

	// Create mock batch
	mockBatch := &mockEventDataBatch{maxBytes: 1048576}
	mockBatch.On("AddEventData", mock.Anything, mock.Anything).Return(nil)

	// Verify that NewEventDataBatch is called with nil options
	mockClient.On("NewEventDataBatch", mock.Anything, (*azeventhubs.EventDataBatchOptions)(nil)).Return(mockBatch, nil)
	mockClient.On("SendEventDataBatch", mock.Anything, mockBatch, mock.Anything).Return(nil)

	err := exp.pushTraces(context.Background(), traces)
	assert.NoError(t, err)

	mockClient.AssertExpectations(t)
	mockBatch.AssertExpectations(t)
}

func TestSendEventExceedsMaxSize(t *testing.T) {
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
		MaxEventSize:  10, // Very small size to trigger error
		BatchSize:     100,
		BackOffConfig: configretry.NewDefaultBackOffConfig(),
		PartitionKey: PartitionKeyConfig{
			Source: "random",
		},
	}

	exp, mockClient := newMockExporter(t, config, pipeline.SignalTraces)

	// Create test trace data
	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()
	span := ss.Spans().AppendEmpty()
	span.SetName("test-span-with-long-name")

	// The error should happen before calling the client
	err := exp.pushTraces(context.Background(), traces)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "event size")
	assert.Contains(t, err.Error(), "exceeds maximum allowed size")

	// Client methods should not be called
	mockClient.AssertNotCalled(t, "NewEventDataBatch")
	mockClient.AssertNotCalled(t, "SendEventDataBatch")
}
