// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cassandraexporter

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

func TestSpanSchemaQueriesByTraceID(t *testing.T) {
	cfg := withDefaultConfig()
	table := parseCreateSpanTableSQL(cfg)
	require.Contains(t, table, `PRIMARY KEY ((trace_id), span_id)`)
	require.Contains(t, table, `list<frozen<span_event>>`)
	require.Contains(t, table, `list<frozen<span_link>>`)
	require.Contains(t, table, `duration bigint`)
	require.Contains(t, parseCreateEventsTypeSQL(cfg), "span_event")
	require.Contains(t, parseCreateLinksTypeSQL(cfg), "span_link")
	require.Contains(t, parseInsertSpanSQL(cfg), "events")
	require.Contains(t, parseInsertSpanSQL(cfg), "links")
	require.Contains(t, parseCreateDatabaseSQL(cfg), `"otel"`)
}

func TestQuoteIdentEscapesQuotes(t *testing.T) {
	require.Equal(t, `"otel"`, quoteIdent("otel"))
	require.Equal(t, `"ot""el"`, quoteIdent(`ot"el`))
}

func TestPushTraceDataBatchesByTraceAndReturnsInsertError(t *testing.T) {
	cfg := withDefaultConfig()
	fake := &fakeExecutor{}
	exp := &tracesExporter{cfg: cfg, logger: zap.NewNop(), exec: fake}

	err := exp.pushTraceData(context.Background(), testTraces())
	require.NoError(t, err)
	require.Len(t, fake.batches, 2)
	require.Len(t, fake.batches[0], 2)
	require.Len(t, fake.batches[1], 1)

	events, ok := fake.batches[0][0][12].([]spanEvent)
	require.True(t, ok)
	require.Equal(t, "exception", events[0].Name)
	require.Equal(t, `"boom"`, events[0].Attributes["exception.message"])

	links, ok := fake.batches[0][0][13].([]spanLink)
	require.True(t, ok)
	require.Len(t, links, 1)
	require.NotEmpty(t, links[0].TraceID)
	require.Equal(t, "query", fake.batches[0][0][5])
	duration, ok := fake.batches[0][0][9].(int64)
	require.True(t, ok)
	require.Equal(t, int64(time.Second), duration)

	fake.err = errors.New("unavailable")
	err = exp.pushTraceData(context.Background(), testTraces())
	require.ErrorContains(t, err, "unavailable")
}

func TestPushTraceDataChunksLargePartitions(t *testing.T) {
	cfg := withDefaultConfig()
	fake := &fakeExecutor{}
	exp := &tracesExporter{cfg: cfg, logger: zap.NewNop(), exec: fake}

	traces := ptrace.NewTraces()
	spans := traces.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans()
	traceID := pcommon.TraceID{1}
	for i := 0; i < maxStatementsPerBatch+1; i++ {
		span := spans.AppendEmpty()
		span.SetName("span")
		span.SetTraceID(traceID)
		span.SetSpanID(pcommon.SpanID{byte(i + 1)})
		span.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Unix(0, 0).UTC()))
		span.SetEndTimestamp(pcommon.NewTimestampFromTime(time.Unix(0, 1).UTC()))
	}

	require.NoError(t, exp.pushTraceData(context.Background(), traces))
	require.Len(t, fake.batches, 2)
	require.Len(t, fake.batches[0], maxStatementsPerBatch)
	require.Len(t, fake.batches[1], 1)
}

func TestPushTraceDataRequiresStart(t *testing.T) {
	exp := &tracesExporter{cfg: withDefaultConfig(), logger: zap.NewNop()}
	err := exp.pushTraceData(context.Background(), testTraces())
	require.ErrorContains(t, err, "not started")
}

func testTraces() ptrace.Traces {
	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("service.name", "api")
	scope := rs.ScopeSpans().AppendEmpty().Spans()

	start := time.Unix(100, 0).UTC()
	first := scope.AppendEmpty()
	first.SetName("query")
	first.SetTraceID(pcommon.TraceID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	first.SetSpanID(pcommon.SpanID{1, 2, 3, 4, 5, 6, 7, 8})
	first.SetParentSpanID(pcommon.SpanID{8, 7, 6, 5, 4, 3, 2, 1})
	first.SetStartTimestamp(pcommon.NewTimestampFromTime(start))
	first.SetEndTimestamp(pcommon.NewTimestampFromTime(start.Add(time.Second)))
	first.Status().SetCode(ptrace.StatusCodeError)
	first.Status().SetMessage("failed")
	first.Attributes().PutStr("db.system", "cassandra")
	event := first.Events().AppendEmpty()
	event.SetName("exception")
	event.SetTimestamp(pcommon.NewTimestampFromTime(start.Add(time.Millisecond)))
	event.Attributes().PutStr("exception.message", "boom")
	link := first.Links().AppendEmpty()
	link.SetTraceID(pcommon.TraceID{9})
	link.SetSpanID(pcommon.SpanID{9})

	second := scope.AppendEmpty()
	second.SetName("query-child")
	second.SetTraceID(first.TraceID())
	second.SetSpanID(pcommon.SpanID{2})
	second.SetStartTimestamp(first.StartTimestamp())
	second.SetEndTimestamp(first.EndTimestamp())

	other := scope.AppendEmpty()
	other.SetName("other")
	other.SetTraceID(pcommon.TraceID{2})
	other.SetSpanID(pcommon.SpanID{3})
	other.SetStartTimestamp(first.StartTimestamp())
	other.SetEndTimestamp(first.EndTimestamp())
	return traces
}
