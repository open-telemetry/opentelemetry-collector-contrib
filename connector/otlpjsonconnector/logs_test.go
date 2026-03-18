// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otlpjsonconnector

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/connector/connectortest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/otlpjsonconnector/internal/metadata"
)

func TestNewLogsConnector(t *testing.T) {
	set := connectortest.NewNopSettings(metadata.Type)
	cfg := createDefaultConfig()
	sink := &consumertest.LogsSink{}

	c := newLogsConnector(set, cfg, sink)

	require.NotNil(t, c)
	assert.NotNil(t, c.logger)
	assert.NotNil(t, c.logsConsumer)
}

func TestLogsConnectorCapabilities(t *testing.T) {
	c := &connectorLogs{}
	caps := c.Capabilities()
	assert.Equal(t, consumer.Capabilities{MutatesData: false}, caps)
}

func TestLogsConnectorConsumeLogsWithValidLogPayload(t *testing.T) {
	set := connectortest.NewNopSettings(metadata.Type)
	cfg := createDefaultConfig()
	sink := &consumertest.LogsSink{}
	c := newLogsConnector(set, cfg, sink)

	// Create a valid OTLP JSON log payload embedded in a log record
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()
	lr := sl.LogRecords().AppendEmpty()
	lr.Body().SetStr(`{"resourceLogs":[{"resource":{},"scopeLogs":[{"scope":{},"logRecords":[{"body":{"stringValue":"test log"}}]}]}]}`)

	err := c.ConsumeLogs(t.Context(), logs)
	require.NoError(t, err)

	// The valid log payload should be forwarded to the sink
	assert.Equal(t, 1, sink.LogRecordCount())
}

func TestLogsConnectorConsumeLogsWithTracePayload(t *testing.T) {
	set := connectortest.NewNopSettings(metadata.Type)
	cfg := createDefaultConfig()
	sink := &consumertest.LogsSink{}
	c := newLogsConnector(set, cfg, sink)

	// Create a trace payload in a log body - should be skipped
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()
	lr := sl.LogRecords().AppendEmpty()
	lr.Body().SetStr(`{"resourceSpans":[{"resource":{},"scopeSpans":[]}]}`)

	err := c.ConsumeLogs(t.Context(), logs)
	require.NoError(t, err)

	// Trace payloads should be ignored by the logs connector
	assert.Equal(t, 0, sink.LogRecordCount())
}

func TestLogsConnectorConsumeLogsWithMetricPayload(t *testing.T) {
	set := connectortest.NewNopSettings(metadata.Type)
	cfg := createDefaultConfig()
	sink := &consumertest.LogsSink{}
	c := newLogsConnector(set, cfg, sink)

	// Create a metric payload in a log body - should be skipped
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()
	lr := sl.LogRecords().AppendEmpty()
	lr.Body().SetStr(`{"resourceMetrics":[{"resource":{},"scopeMetrics":[]}]}`)

	err := c.ConsumeLogs(t.Context(), logs)
	require.NoError(t, err)

	// Metric payloads should be ignored by the logs connector
	assert.Equal(t, 0, sink.LogRecordCount())
}

func TestLogsConnectorConsumeLogsWithInvalidPayload(t *testing.T) {
	set := connectortest.NewNopSettings(metadata.Type)
	cfg := createDefaultConfig()
	sink := &consumertest.LogsSink{}
	c := newLogsConnector(set, cfg, sink)

	// Invalid JSON payload
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()
	lr := sl.LogRecords().AppendEmpty()
	lr.Body().SetStr(`this is not valid otlp json`)

	err := c.ConsumeLogs(t.Context(), logs)
	require.NoError(t, err)

	// Invalid payloads should be dropped
	assert.Equal(t, 0, sink.LogRecordCount())
}

func TestLogsConnectorConsumeLogsWithMalformedLogJSON(t *testing.T) {
	set := connectortest.NewNopSettings(metadata.Type)
	cfg := createDefaultConfig()
	sink := &consumertest.LogsSink{}
	c := newLogsConnector(set, cfg, sink)

	// Regex matches but JSON is malformed
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()
	lr := sl.LogRecords().AppendEmpty()
	lr.Body().SetStr(`{"resourceLogs": [invalid json`)

	err := c.ConsumeLogs(t.Context(), logs)
	require.NoError(t, err)

	// Malformed JSON should not cause panic but be dropped
	assert.Equal(t, 0, sink.LogRecordCount())
}

func TestLogsConnectorConsumeLogsWithEmptyLogs(t *testing.T) {
	set := connectortest.NewNopSettings(metadata.Type)
	cfg := createDefaultConfig()
	sink := &consumertest.LogsSink{}
	c := newLogsConnector(set, cfg, sink)

	// Empty logs input
	logs := plog.NewLogs()
	err := c.ConsumeLogs(t.Context(), logs)
	require.NoError(t, err)
	assert.Equal(t, 0, sink.LogRecordCount())
}
