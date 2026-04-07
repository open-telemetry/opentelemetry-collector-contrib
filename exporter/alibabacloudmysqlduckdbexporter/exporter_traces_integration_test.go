// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package alibabacloudmysqlduckdbexporter

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap/zaptest"
)

func TestTracesExporterIntegration(t *testing.T) {
	dsn := getMySQLDSN(t)
	cfg := newTestConfig(dsn)

	exporter := newTracesExporter(zaptest.NewLogger(t), cfg)
	require.NoError(t, exporter.start(t.Context(), nil))
	t.Cleanup(func() { _ = exporter.shutdown(t.Context()) })

	db := queryDB(t, dsn)
	truncateTable(t, db, cfg.Database, cfg.TracesTableName)

	traces := simpleTraces(10)
	require.NoError(t, exporter.pushTraceData(t.Context(), traces))

	// Verify count
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM `otel_test`.`otel_traces`").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 10, count)

	// Verify first row
	var (
		ts           time.Time
		traceID      string
		spanID       string
		parentSpanID string
		traceState   string
		spanName     string
		spanKind     string
		serviceName  string
		resAttrRaw   []byte
		scopeName    string
		scopeVersion string
		spanAttrRaw  []byte
		duration     uint64
		statusCode   string
		statusMsg    string
		eventsRaw    []byte
		linksRaw     []byte
	)
	err = db.QueryRow(`SELECT timestamp, trace_id, span_id, parent_span_id, trace_state,
		span_name, span_kind, service_name, resource_attributes, scope_name, scope_version,
		span_attributes, duration, status_code, status_message, events, links
		FROM `+"`otel_test`.`otel_traces`"+` ORDER BY id LIMIT 1`).Scan(
		&ts, &traceID, &spanID, &parentSpanID, &traceState,
		&spanName, &spanKind, &serviceName, &resAttrRaw, &scopeName, &scopeVersion,
		&spanAttrRaw, &duration, &statusCode, &statusMsg, &eventsRaw, &linksRaw,
	)
	require.NoError(t, err)

	assert.Equal(t, "01020300000000000000000000000000", traceID)
	assert.Equal(t, "0102030000000000", spanID)
	assert.Equal(t, "0102040000000000", parentSpanID)
	assert.Equal(t, "trace state", traceState)
	assert.Equal(t, "call db", spanName)
	assert.Equal(t, "Internal", spanKind)
	assert.Equal(t, "test-service", serviceName)
	assert.Equal(t, uint64(60000000000), duration)
	assert.Equal(t, "Error", statusCode)
	assert.Equal(t, "something wrong", statusMsg)
	assert.Equal(t, "io.opentelemetry.contrib.mysql", scopeName)
	assert.Equal(t, "1.0.0", scopeVersion)

	// Verify resource attributes
	var resAttrs map[string]string
	require.NoError(t, json.Unmarshal(resAttrRaw, &resAttrs))
	assert.Equal(t, "test-service", resAttrs["service.name"])

	// Verify span attributes
	var spanAttrs map[string]string
	require.NoError(t, json.Unmarshal(spanAttrRaw, &spanAttrs))
	assert.Equal(t, "v", spanAttrs["service.name"])

	// Verify events
	var events []eventJSON
	require.NoError(t, json.Unmarshal(eventsRaw, &events))
	assert.Len(t, events, 1)
	assert.Equal(t, "event1", events[0].Name)

	// Verify links
	var links []linkJSON
	require.NoError(t, json.Unmarshal(linksRaw, &links))
	assert.Len(t, links, 1)
	assert.Equal(t, "trace state", links[0].TraceState)

	// Verify TTL event was created
	var eventCount int
	err = db.QueryRow(
		"SELECT COUNT(*) FROM information_schema.EVENTS WHERE EVENT_SCHEMA = ? AND EVENT_NAME = ?",
		cfg.Database, "ttl_otel_traces",
	).Scan(&eventCount)
	require.NoError(t, err)
	assert.Equal(t, 1, eventCount, "TTL event ttl_otel_traces should exist")
}

func simpleTraces(count int) ptrace.Traces {
	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("service.name", "test-service")
	ss := rs.ScopeSpans().AppendEmpty()
	ss.Scope().SetName("io.opentelemetry.contrib.mysql")
	ss.Scope().SetVersion("1.0.0")

	for i := range count {
		span := ss.Spans().AppendEmpty()
		span.SetTraceID([16]byte{1, 2, 3, byte(i)})
		span.SetSpanID([8]byte{1, 2, 3, byte(i)})
		span.SetParentSpanID([8]byte{1, 2, 4, byte(i)})
		span.TraceState().FromRaw("trace state")
		span.SetName("call db")
		span.SetKind(ptrace.SpanKindInternal)
		span.SetStartTimestamp(pcommon.NewTimestampFromTime(telemetryTimestamp))
		span.SetEndTimestamp(pcommon.NewTimestampFromTime(telemetryTimestamp.Add(60 * time.Second)))
		span.Status().SetCode(ptrace.StatusCodeError)
		span.Status().SetMessage("something wrong")
		span.Attributes().PutStr("service.name", "v")

		event := span.Events().AppendEmpty()
		event.SetName("event1")
		event.SetTimestamp(pcommon.NewTimestampFromTime(telemetryTimestamp))
		event.Attributes().PutStr("key", "value")

		link := span.Links().AppendEmpty()
		link.SetTraceID([16]byte{1, 2, 3, byte(i)})
		link.SetSpanID([8]byte{1, 2, 3, byte(i)})
		link.TraceState().FromRaw("trace state")
		link.Attributes().PutStr("key", "value")
	}
	return traces
}
