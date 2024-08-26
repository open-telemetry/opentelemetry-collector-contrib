// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkareceiver

import (
    "testing"

    "github.com/stretchr/testify/assert"
    "go.opentelemetry.io/collector/pdata/plog"
    "go.opentelemetry.io/collector/pdata/pmetric"
    "go.opentelemetry.io/collector/pdata/ptrace"
)

func TestNewOTLPJSONTracesUnmarshaler(t *testing.T) {
    t.Parallel()
    um := newOTLPJSONTracesUnmarshaler()
    assert.Equal(t, otlpJSONEncoding, um.Encoding())
}

func TestOTLPJSONTracesUnmarshalerReturnType(t *testing.T) {
    t.Parallel()
    um := newOTLPJSONTracesUnmarshaler()
    json := `{"resourceSpans":[{"resource":{},"scopeSpans":[{"scope":{},"spans":[]}]}]}`
    unmarshaledTraces, err := um.Unmarshal([]byte(json))
    assert.NoError(t, err)
    var expectedType ptrace.Traces
    assert.IsType(t, expectedType, unmarshaledTraces)
}

func TestNewOTLPJSONMetricsUnmarshaler(t *testing.T) {
    t.Parallel()
    um := newOTLPJSONMetricsUnmarshaler()
    assert.Equal(t, otlpJSONEncoding, um.Encoding())
}

func TestOTLPJSONMetricsUnmarshalerReturnType(t *testing.T) {
    t.Parallel()
    um := newOTLPJSONMetricsUnmarshaler()
    json := `{"resourceMetrics":[{"resource":{},"scopeMetrics":[{"scope":{},"metrics":[]}]}]}`
    unmarshaledMetrics, err := um.Unmarshal([]byte(json))
    assert.NoError(t, err)
    var expectedType pmetric.Metrics
    assert.IsType(t, expectedType, unmarshaledMetrics)
}

func TestNewOTLPJSONLogsUnmarshaler(t *testing.T) {
    t.Parallel()
    um := newOTLPJSONLogsUnmarshaler()
    assert.Equal(t, otlpJSONEncoding, um.Encoding())
}

func TestOTLPJSONLogsUnmarshalerReturnType(t *testing.T) {
    t.Parallel()
    um := newOTLPJSONLogsUnmarshaler()
    json := `{"resourceLogs":[{"resource":{},"scopeLogs":[{"scope":{},"logRecords":[]}]}]}`
    unmarshaledLogs, err := um.Unmarshal([]byte(json))
    assert.NoError(t, err)
    var expectedType plog.Logs
    assert.IsType(t, expectedType, unmarshaledLogs)
}