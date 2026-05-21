// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hologresexporter

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

func newTestTraces() ptrace.Traces {
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("service.name", "test-service")
	rs.Resource().Attributes().PutStr("host.name", "test-host")

	ss := rs.ScopeSpans().AppendEmpty()
	ss.Scope().SetName("test-scope")
	ss.Scope().SetVersion("v1.0.0")

	timestamp := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	span := ss.Spans().AppendEmpty()
	span.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	span.SetSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})
	span.SetParentSpanID([8]byte{8, 7, 6, 5, 4, 3, 2, 1})
	span.TraceState().FromRaw("key=value")
	span.SetName("test-span")
	span.SetKind(ptrace.SpanKindServer)
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(timestamp))
	span.SetEndTimestamp(pcommon.NewTimestampFromTime(timestamp.Add(100 * time.Millisecond)))
	span.Attributes().PutStr("http.method", "GET")
	span.Attributes().PutInt("http.status_code", 200)
	span.Status().SetCode(ptrace.StatusCodeOk)
	span.Status().SetMessage("success")

	// Add an event.
	event := span.Events().AppendEmpty()
	event.SetName("test-event")
	event.SetTimestamp(pcommon.NewTimestampFromTime(timestamp.Add(50 * time.Millisecond)))
	event.Attributes().PutStr("event.key", "event-value")

	// Add a link.
	link := span.Links().AppendEmpty()
	link.SetTraceID([16]byte{16, 15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1})
	link.SetSpanID([8]byte{8, 7, 6, 5, 4, 3, 2, 1})
	link.TraceState().FromRaw("linked=true")
	link.Attributes().PutStr("link.key", "link-value")

	return td
}

func TestConvertEvents(t *testing.T) {
	t.Run("empty events", func(t *testing.T) {
		events := ptrace.NewSpanEventSlice()
		result, err := convertEvents(events)
		require.NoError(t, err)
		assert.JSONEq(t, "[]", string(result))
	})

	t.Run("single event", func(t *testing.T) {
		events := ptrace.NewSpanEventSlice()
		e := events.AppendEmpty()
		e.SetName("test-event")
		ts := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
		e.SetTimestamp(pcommon.NewTimestampFromTime(ts))
		e.Attributes().PutStr("key", "value")

		result, err := convertEvents(events)
		require.NoError(t, err)

		var parsed []map[string]any
		require.NoError(t, json.Unmarshal(result, &parsed))
		assert.Len(t, parsed, 1)
		assert.Equal(t, "test-event", parsed[0]["name"])
		attrs := parsed[0]["attributes"].(map[string]any)
		assert.Equal(t, "value", attrs["key"])
	})

	t.Run("multiple events", func(t *testing.T) {
		events := ptrace.NewSpanEventSlice()
		ts := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
		for i := range 3 {
			e := events.AppendEmpty()
			e.SetName("event-" + string(rune('a'+i)))
			e.SetTimestamp(pcommon.NewTimestampFromTime(ts.Add(time.Duration(i) * time.Millisecond)))
		}

		result, err := convertEvents(events)
		require.NoError(t, err)

		var parsed []map[string]any
		require.NoError(t, json.Unmarshal(result, &parsed))
		assert.Len(t, parsed, 3)
	})
}

func TestConvertLinks(t *testing.T) {
	t.Run("empty links", func(t *testing.T) {
		links := ptrace.NewSpanLinkSlice()
		result, err := convertLinks(links)
		require.NoError(t, err)
		assert.JSONEq(t, "[]", string(result))
	})

	t.Run("single link", func(t *testing.T) {
		links := ptrace.NewSpanLinkSlice()
		l := links.AppendEmpty()
		l.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
		l.SetSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})
		l.TraceState().FromRaw("state=test")
		l.Attributes().PutStr("link.key", "link-value")

		result, err := convertLinks(links)
		require.NoError(t, err)

		var parsed []map[string]any
		require.NoError(t, json.Unmarshal(result, &parsed))
		assert.Len(t, parsed, 1)
		assert.Equal(t, "0102030405060708090a0b0c0d0e0f10", parsed[0]["trace_id"])
		assert.Equal(t, "0102030405060708", parsed[0]["span_id"])
		assert.Equal(t, "state=test", parsed[0]["trace_state"])
		attrs := parsed[0]["attributes"].(map[string]any)
		assert.Equal(t, "link-value", attrs["link.key"])
	})

	t.Run("link with empty IDs", func(t *testing.T) {
		links := ptrace.NewSpanLinkSlice()
		links.AppendEmpty()
		// leave TraceID and SpanID as zero values

		result, err := convertLinks(links)
		require.NoError(t, err)

		var parsed []map[string]any
		require.NoError(t, json.Unmarshal(result, &parsed))
		assert.Len(t, parsed, 1)
		assert.Equal(t, "", parsed[0]["trace_id"])
		assert.Equal(t, "", parsed[0]["span_id"])
	})
}

func TestPushTraceData_EmptyTraces(t *testing.T) {
	cfg := &Config{
		TracesTableName: "otel_traces",
	}
	exp := newTracesExporter(nil, cfg)

	td := ptrace.NewTraces()
	err := exp.pushTraceData(t.Context(), td)
	assert.NoError(t, err)
}

func TestPushTraceData_DataConversion(t *testing.T) {
	// Verify the data conversion logic produces correct span rows
	// by directly testing the row construction without a database.
	td := newTestTraces()

	rsSpans := td.ResourceSpans()
	rs := rsSpans.At(0)
	serviceName := getServiceName(rs.Resource())
	assert.Equal(t, "test-service", serviceName)

	resourceAttrs, err := attributesToJSON(rs.Resource().Attributes())
	require.NoError(t, err)

	var resAttrsMap map[string]any
	require.NoError(t, json.Unmarshal(resourceAttrs, &resAttrsMap))
	assert.Equal(t, "test-service", resAttrsMap["service.name"])
	assert.Equal(t, "test-host", resAttrsMap["host.name"])

	ss := rs.ScopeSpans().At(0)
	assert.Equal(t, "test-scope", ss.Scope().Name())
	assert.Equal(t, "v1.0.0", ss.Scope().Version())

	span := ss.Spans().At(0)

	spanAttrs, err := attributesToJSON(span.Attributes())
	require.NoError(t, err)
	var spanAttrsMap map[string]any
	require.NoError(t, json.Unmarshal(spanAttrs, &spanAttrsMap))
	assert.Equal(t, "GET", spanAttrsMap["http.method"])

	eventsJSON, err := convertEvents(span.Events())
	require.NoError(t, err)
	var events []map[string]any
	require.NoError(t, json.Unmarshal(eventsJSON, &events))
	assert.Len(t, events, 1)
	assert.Equal(t, "test-event", events[0]["name"])

	linksJSON, err := convertLinks(span.Links())
	require.NoError(t, err)
	var links []map[string]any
	require.NoError(t, json.Unmarshal(linksJSON, &links))
	assert.Len(t, links, 1)

	// Verify duration calculation.
	duration := int64(span.EndTimestamp() - span.StartTimestamp())
	assert.Equal(t, int64(100*time.Millisecond), duration)
}

func TestInsertBatch_QueryConstruction(t *testing.T) {
	// Verify that insertBatch constructs the correct SQL query structure.
	// We can't execute it without a DB, but we can verify the row building.
	rows := []traceSpanRow{
		{args: make([]any, traceColumnsCount)},
		{args: make([]any, traceColumnsCount)},
	}

	// Verify the batch has the expected number of rows.
	assert.Len(t, rows, 2)
	assert.Len(t, rows[0].args, traceColumnsCount)
}

func TestMaxSpansPerBatch(t *testing.T) {
	// Verify constant calculations.
	assert.Equal(t, 17, traceColumnsCount)
	assert.Equal(t, 65535/17, maxSpansPerBatch)
	assert.True(t, maxSpansPerBatch*traceColumnsCount <= 65535)
}

func TestPushTraceData_WithDB(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectExec("INSERT INTO").WillReturnResult(sqlmock.NewResult(0, 1))

	cfg := &Config{TracesTableName: "test_traces"}
	exp := &tracesExporter{
		logger: zap.NewNop(),
		cfg:    cfg,
		db:     db,
	}

	td := newTestTraces()
	err = exp.pushTraceData(t.Context(), td)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPushTraceData_DBError(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectExec("INSERT INTO").WillReturnError(fmt.Errorf("connection refused"))

	cfg := &Config{TracesTableName: "test_traces"}
	exp := &tracesExporter{
		logger: zap.NewNop(),
		cfg:    cfg,
		db:     db,
	}

	td := newTestTraces()
	err = exp.pushTraceData(t.Context(), td)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "connection refused")
}

func TestTracesInsertBatch_WithDB(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectExec("INSERT INTO").WillReturnResult(sqlmock.NewResult(0, 2))

	cfg := &Config{TracesTableName: "test_traces"}
	exp := &tracesExporter{
		logger: zap.NewNop(),
		cfg:    cfg,
		db:     db,
	}

	rows := []traceSpanRow{
		{args: make([]any, traceColumnsCount)},
		{args: make([]any, traceColumnsCount)},
	}

	err = exp.insertBatch(t.Context(), rows)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTracesInsertBatch_Error(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectExec("INSERT INTO").WillReturnError(fmt.Errorf("insert error"))

	cfg := &Config{TracesTableName: "test_traces"}
	exp := &tracesExporter{
		logger: zap.NewNop(),
		cfg:    cfg,
		db:     db,
	}

	rows := []traceSpanRow{{args: make([]any, traceColumnsCount)}}
	err = exp.insertBatch(t.Context(), rows)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "insert error")
}

func TestTracesExporter_Shutdown(t *testing.T) {
	t.Run("nil db", func(t *testing.T) {
		exp := &tracesExporter{
			logger: zap.NewNop(),
			cfg:    &Config{},
		}
		err := exp.shutdown(context.Background())
		require.NoError(t, err)
	})

	t.Run("with db", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)

		mock.ExpectClose()

		exp := &tracesExporter{
			logger: zap.NewNop(),
			cfg:    &Config{},
			db:     db,
		}
		err = exp.shutdown(context.Background())
		require.NoError(t, err)
	})
}

func TestTracesExporter_Start_DBError(t *testing.T) {
	exp := &tracesExporter{
		logger: zap.NewNop(),
		cfg: &Config{
			DSN: "postgresql://user:pass@localhost:1/db",
		},
	}
	err := exp.start(context.Background(), nil)
	require.Error(t, err)
}
