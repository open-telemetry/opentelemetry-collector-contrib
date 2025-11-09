// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hydrolixexporter

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func TestTracesExporter_ConvertToHydrolixSpans(t *testing.T) {
	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("service.name", "test-service")

	ss := rs.ScopeSpans().AppendEmpty()
	span := ss.Spans().AppendEmpty()

	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	spanID := pcommon.SpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})
	parentSpanID := pcommon.SpanID([8]byte{0, 0, 0, 0, 0, 0, 0, 1})

	span.SetTraceID(traceID)
	span.SetSpanID(spanID)
	span.SetParentSpanID(parentSpanID)
	span.SetName("test-span")
	span.SetKind(ptrace.SpanKindServer)
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	span.SetEndTimestamp(pcommon.NewTimestampFromTime(time.Now().Add(100 * time.Millisecond)))
	span.Status().SetCode(ptrace.StatusCodeOk)
	span.Status().SetMessage("success")

	span.Attributes().PutStr("http.request.method", "GET")
	span.Attributes().PutStr("http.route", "/api/test")
	span.Attributes().PutStr("http.response.status_code", "200")

	cfg := &Config{
		ClientConfig: confighttp.ClientConfig{
			Endpoint: "http://localhost:8080",
		},
		HDXTable:     "traces_table",
		HDXTransform: "traces_transform",
		HDXUsername:  "user",
		HDXPassword:  "pass",
	}

	exporter := newTracesExporter(cfg, exportertest.NewNopSettings(component.MustNewType("hydrolix")))
	result := exporter.convertToHydrolixSpans(traces)

	require.Len(t, result, 1)
	assert.Equal(t, "test-span", result[0].Name)
	assert.Equal(t, "test-service", result[0].ServiceName)
	assert.Equal(t, traceID.String(), result[0].TraceID)
	assert.Equal(t, spanID.String(), result[0].SpanID)
	assert.Equal(t, parentSpanID.String(), result[0].ParentSpanID)
	assert.Equal(t, "Server", result[0].SpanKindString)
	assert.Equal(t, "Ok", result[0].StatusCodeString)
	assert.Equal(t, "success", result[0].StatusMessage)
	assert.Equal(t, "/api/test", result[0].HTTPRoute)
	assert.Equal(t, "GET", result[0].HTTPMethod)
	assert.Equal(t, "200", result[0].HTTPStatusCode)
	assert.Greater(t, result[0].Duration, uint64(0))
}

func TestTracesExporter_ConvertEvents(t *testing.T) {
	events := ptrace.NewSpanEventSlice()

	event1 := events.AppendEmpty()
	event1.SetName("event1")
	event1.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	event1.Attributes().PutStr("key1", "value1")

	event2 := events.AppendEmpty()
	event2.SetName("event2")
	event2.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	event2.Attributes().PutInt("key2", 42)

	result := convertEvents(events)

	require.Len(t, result, 2)
	assert.Equal(t, "event1", result[0].Name)
	assert.Equal(t, "event2", result[1].Name)
	assert.Greater(t, result[0].Timestamp, uint64(0))
	assert.Greater(t, result[1].Timestamp, uint64(0))
}

func TestTracesExporter_ConvertEmptyEvents(t *testing.T) {
	events := ptrace.NewSpanEventSlice()
	result := convertEvents(events)
	assert.Nil(t, result)
}

func TestTracesExporter_GetSpanKindString(t *testing.T) {
	tests := []struct {
		kind     ptrace.SpanKind
		expected string
	}{
		{ptrace.SpanKindUnspecified, "Unspecified"},
		{ptrace.SpanKindInternal, "Internal"},
		{ptrace.SpanKindServer, "Server"},
		{ptrace.SpanKindClient, "Client"},
		{ptrace.SpanKindProducer, "Producer"},
		{ptrace.SpanKindConsumer, "Consumer"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := getSpanKindString(tt.kind)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTracesExporter_GetStatusCodeString(t *testing.T) {
	tests := []struct {
		code     ptrace.StatusCode
		expected string
	}{
		{ptrace.StatusCodeUnset, "Unset"},
		{ptrace.StatusCodeOk, "Ok"},
		{ptrace.StatusCodeError, "Error"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := getStatusCodeString(tt.code)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTracesExporter_PushTraces(t *testing.T) {
	// Create a test server
	var receivedData []HydrolixSpan
	var receivedHeaders http.Header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header.Clone()
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		err = json.Unmarshal(body, &receivedData)
		require.NoError(t, err)

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create test traces
	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("service.name", "test-service")

	ss := rs.ScopeSpans().AppendEmpty()
	span := ss.Spans().AppendEmpty()

	span.SetTraceID(pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}))
	span.SetSpanID(pcommon.SpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8}))
	span.SetName("test-span")
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	span.SetEndTimestamp(pcommon.NewTimestampFromTime(time.Now().Add(time.Second)))

	// Create exporter
	cfg := &Config{
		ClientConfig: confighttp.ClientConfig{
			Endpoint: server.URL,
			Timeout:  5 * time.Second,
		},
		HDXTable:     "traces_table",
		HDXTransform: "traces_transform",
		HDXUsername:  "testuser",
		HDXPassword:  "testpass",
	}

	exporter := newTracesExporter(cfg, exportertest.NewNopSettings(component.MustNewType("hydrolix")))

	// Push traces
	err := exporter.pushTraces(context.Background(), traces)
	require.NoError(t, err)

	// Verify received data
	require.Len(t, receivedData, 1)
	assert.Equal(t, "test-span", receivedData[0].Name)
	assert.Equal(t, "test-service", receivedData[0].ServiceName)

	// Verify headers
	assert.Equal(t, "application/json", receivedHeaders.Get("Content-Type"))
	assert.Equal(t, "traces_table", receivedHeaders.Get("x-hdx-table"))
	assert.Equal(t, "traces_transform", receivedHeaders.Get("x-hdx-transform"))
	assert.NotEmpty(t, receivedHeaders.Get("Authorization"))
	assert.Contains(t, receivedHeaders.Get("Authorization"), "Basic")
}

func TestTracesExporter_PushTracesError(t *testing.T) {
	// Create a test server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()
	span := ss.Spans().AppendEmpty()
	span.SetName("test-span")

	cfg := &Config{
		ClientConfig: confighttp.ClientConfig{
			Endpoint: server.URL,
			Timeout:  5 * time.Second,
		},
		HDXTable:     "table",
		HDXTransform: "transform",
		HDXUsername:  "user",
		HDXPassword:  "pass",
	}

	exporter := newTracesExporter(cfg, exportertest.NewNopSettings(component.MustNewType("hydrolix")))
	err := exporter.pushTraces(context.Background(), traces)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected status code")
}

func TestTracesExporter_SpanDurationCalculation(t *testing.T) {
	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()
	span := ss.Spans().AppendEmpty()

	startTime := time.Now()
	endTime := startTime.Add(500 * time.Millisecond)

	span.SetName("duration-test")
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(startTime))
	span.SetEndTimestamp(pcommon.NewTimestampFromTime(endTime))

	cfg := &Config{
		ClientConfig: confighttp.ClientConfig{
			Endpoint: "http://localhost:8080",
		},
	}

	exporter := newTracesExporter(cfg, exportertest.NewNopSettings(component.MustNewType("hydrolix")))
	result := exporter.convertToHydrolixSpans(traces)

	require.Len(t, result, 1)
	expectedDuration := uint64(endTime.Sub(startTime))
	assert.Equal(t, expectedDuration, result[0].Duration)
}
