// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package spanpruningprocessor

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/spanpruningprocessor/internal/metadata"
)

func TestNewTraces(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	tp, err := factory.CreateTraces(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)
	require.NotNil(t, tp)
}

func TestLeafSpanPruning_BasicAggregation(t *testing.T) {
	// Test: 3 identical leaf spans should be aggregated into 1 summary span
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.MinSpansToAggregate = 2

	tp, err := factory.CreateTraces(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)

	td := createTestTraceWithLeafSpans(t, 3, "SELECT", map[string]string{"db.operation": "select"})
	originalSpanCount := countSpans(td)
	assert.Equal(t, 4, originalSpanCount) // 1 parent + 3 leaf spans

	err = tp.ConsumeTraces(t.Context(), td)
	require.NoError(t, err)

	// After processing: should have 1 parent + 1 summary span
	finalSpanCount := countSpans(td)
	assert.Equal(t, 2, finalSpanCount)

	// Verify summary span exists with aggregation attributes
	summarySpan := findSpanByNameSuffix(td, "_aggregated")
	require.NotNil(t, summarySpan, "summary span should exist")

	// Check aggregation attributes
	attrs := summarySpan.Attributes()
	spanCount, exists := attrs.Get("aggregation.span_count")
	assert.True(t, exists, "aggregation.span_count should exist")
	assert.Equal(t, int64(3), spanCount.Int())
}

func TestLeafSpanPruning_BelowThreshold(t *testing.T) {
	// Test: 1 leaf span with min_spans_to_aggregate=2 should not be aggregated
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.MinSpansToAggregate = 2

	tp, err := factory.CreateTraces(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)

	td := createTestTraceWithLeafSpans(t, 1, "SELECT", map[string]string{"db.operation": "select"})
	originalSpanCount := countSpans(td)
	assert.Equal(t, 2, originalSpanCount) // 1 parent + 1 leaf span

	err = tp.ConsumeTraces(t.Context(), td)
	require.NoError(t, err)

	// Should remain unchanged
	finalSpanCount := countSpans(td)
	assert.Equal(t, 2, finalSpanCount)
}

func TestLeafSpanPruning_MixedLeafAndNonLeaf(t *testing.T) {
	// Test: only aggregate leaf spans, not spans with children
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.MinSpansToAggregate = 2

	tp, err := factory.CreateTraces(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)

	// Create trace: root -> intermediate -> 3 leaf spans
	td := createTestTraceWithIntermediateSpan(t)
	originalSpanCount := countSpans(td)
	assert.Equal(t, 5, originalSpanCount) // 1 root + 1 intermediate + 3 leaf spans

	err = tp.ConsumeTraces(t.Context(), td)
	require.NoError(t, err)

	// After: 1 root + 1 intermediate + 1 summary
	finalSpanCount := countSpans(td)
	assert.Equal(t, 3, finalSpanCount)
}

func TestLeafSpanPruning_DifferentGroups(t *testing.T) {
	// Test: spans with different attributes should stay in separate groups
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.MinSpansToAggregate = 2
	cfg.GroupByAttributes = []string{"db.operation"}

	tp, err := factory.CreateTraces(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)

	// Create trace with mixed operations: 3 SELECT + 2 INSERT
	td := createTestTraceWithMixedOperations(t)
	originalSpanCount := countSpans(td)
	assert.Equal(t, 6, originalSpanCount) // 1 parent + 3 SELECT + 2 INSERT

	err = tp.ConsumeTraces(t.Context(), td)
	require.NoError(t, err)

	// After: 1 parent + 1 SELECT summary + 1 INSERT summary
	finalSpanCount := countSpans(td)
	assert.Equal(t, 3, finalSpanCount)
}

func TestLeafSpanPruning_EmptyTrace(t *testing.T) {
	// Test: empty trace should be handled gracefully
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)

	tp, err := factory.CreateTraces(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)

	td := ptrace.NewTraces()

	err = tp.ConsumeTraces(t.Context(), td)
	require.NoError(t, err)
	assert.Equal(t, 0, countSpans(td))
}

func TestLeafSpanPruning_SingleSpanTrace(t *testing.T) {
	// Test: single span trace (root only) should not be modified
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.MinSpansToAggregate = 2

	tp, err := factory.CreateTraces(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)

	td := createSingleSpanTrace(t)
	originalSpanCount := countSpans(td)
	assert.Equal(t, 1, originalSpanCount)

	err = tp.ConsumeTraces(t.Context(), td)
	require.NoError(t, err)

	// Should remain unchanged
	finalSpanCount := countSpans(td)
	assert.Equal(t, 1, finalSpanCount)
}

func TestLeafSpanPruning_StatusAggregation(t *testing.T) {
	// Test: spans with different status codes should be in separate groups
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.MinSpansToAggregate = 2

	tp, err := factory.CreateTraces(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)

	// Create trace with 4 OK spans and 2 Error spans (same name)
	td := createTestTraceWithMixedStatusSpans(t)
	originalSpanCount := countSpans(td)
	assert.Equal(t, 7, originalSpanCount) // 1 parent + 4 OK + 2 Error

	err = tp.ConsumeTraces(t.Context(), td)
	require.NoError(t, err)

	// After: 1 parent + 1 OK summary (4 spans) + 1 Error summary (2 spans)
	finalSpanCount := countSpans(td)
	assert.Equal(t, 3, finalSpanCount)

	// Verify we have both an OK summary and an Error summary
	okSummary := findSpanByNameAndStatus(td, "_aggregated", ptrace.StatusCodeOk)
	require.NotNil(t, okSummary)
	okCount, _ := okSummary.Attributes().Get("aggregation.span_count")
	assert.Equal(t, int64(4), okCount.Int())

	errorSummary := findSpanByNameAndStatus(td, "_aggregated", ptrace.StatusCodeError)
	require.NotNil(t, errorSummary)
	errorCount, _ := errorSummary.Attributes().Get("aggregation.span_count")
	assert.Equal(t, int64(2), errorCount.Int())
}

func TestLeafSpanPruning_StatusBelowThreshold(t *testing.T) {
	// Test: 1 OK span + 1 Error span should not aggregate (each group below threshold)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.MinSpansToAggregate = 2

	tp, err := factory.CreateTraces(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)

	td := createTestTraceWithErrorSpan(t) // Creates 1 OK + 1 Error span
	originalSpanCount := countSpans(td)
	assert.Equal(t, 3, originalSpanCount) // 1 parent + 1 OK + 1 Error

	err = tp.ConsumeTraces(t.Context(), td)
	require.NoError(t, err)

	// Should remain unchanged - neither group meets threshold
	finalSpanCount := countSpans(td)
	assert.Equal(t, 3, finalSpanCount)
}

func TestLeafSpanPruning_DurationStats(t *testing.T) {
	// Test: verify duration statistics are calculated correctly
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.MinSpansToAggregate = 2

	tp, err := factory.CreateTraces(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)

	// Create spans with known durations: 100ns, 200ns, 300ns
	td := createTestTraceWithKnownDurations(t, []int64{100, 200, 300})

	err = tp.ConsumeTraces(t.Context(), td)
	require.NoError(t, err)

	summarySpan := findSpanByNameSuffix(td, "_aggregated")
	require.NotNil(t, summarySpan)

	attrs := summarySpan.Attributes()

	minDuration, _ := attrs.Get("aggregation.duration_min_ns")
	assert.Equal(t, int64(100), minDuration.Int())

	maxDuration, _ := attrs.Get("aggregation.duration_max_ns")
	assert.Equal(t, int64(300), maxDuration.Int())

	avgDuration, _ := attrs.Get("aggregation.duration_avg_ns")
	assert.Equal(t, int64(200), avgDuration.Int()) // (100+200+300)/3 = 200

	totalDuration, _ := attrs.Get("aggregation.duration_total_ns")
	assert.Equal(t, int64(600), totalDuration.Int())
}

// Helper functions

func createTestTraceWithLeafSpans(t *testing.T, numLeafSpans int, spanName string, attrs map[string]string) ptrace.Traces {
	t.Helper()
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()

	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	parentSpanID := pcommon.SpanID([8]byte{1, 0, 0, 0, 0, 0, 0, 0})

	// Create parent span
	parentSpan := ss.Spans().AppendEmpty()
	parentSpan.SetTraceID(traceID)
	parentSpan.SetSpanID(parentSpanID)
	parentSpan.SetName("parent")

	// Create leaf spans
	for i := 0; i < numLeafSpans; i++ {
		span := ss.Spans().AppendEmpty()
		span.SetTraceID(traceID)
		span.SetSpanID(pcommon.SpanID([8]byte{2, byte(i), 0, 0, 0, 0, 0, 0}))
		span.SetParentSpanID(parentSpanID)
		span.SetName(spanName)
		span.SetStartTimestamp(pcommon.Timestamp(1000000000 + int64(i)*100))
		span.SetEndTimestamp(pcommon.Timestamp(1000000100 + int64(i)*100))
		for k, v := range attrs {
			span.Attributes().PutStr(k, v)
		}
	}

	return td
}

func createTestTraceWithIntermediateSpan(t *testing.T) ptrace.Traces {
	t.Helper()
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()

	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	rootSpanID := pcommon.SpanID([8]byte{1, 0, 0, 0, 0, 0, 0, 0})
	intermediateSpanID := pcommon.SpanID([8]byte{2, 0, 0, 0, 0, 0, 0, 0})

	// Root span
	rootSpan := ss.Spans().AppendEmpty()
	rootSpan.SetTraceID(traceID)
	rootSpan.SetSpanID(rootSpanID)
	rootSpan.SetName("root")

	// Intermediate span (child of root, parent of leaves)
	intermediateSpan := ss.Spans().AppendEmpty()
	intermediateSpan.SetTraceID(traceID)
	intermediateSpan.SetSpanID(intermediateSpanID)
	intermediateSpan.SetParentSpanID(rootSpanID)
	intermediateSpan.SetName("intermediate")

	// 3 leaf spans (children of intermediate)
	for i := 0; i < 3; i++ {
		span := ss.Spans().AppendEmpty()
		span.SetTraceID(traceID)
		span.SetSpanID(pcommon.SpanID([8]byte{3, byte(i), 0, 0, 0, 0, 0, 0}))
		span.SetParentSpanID(intermediateSpanID)
		span.SetName("SELECT")
		span.SetStartTimestamp(pcommon.Timestamp(1000000000))
		span.SetEndTimestamp(pcommon.Timestamp(1000000100))
	}

	return td
}

func createTestTraceWithMixedOperations(t *testing.T) ptrace.Traces {
	t.Helper()
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()

	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	parentSpanID := pcommon.SpanID([8]byte{1, 0, 0, 0, 0, 0, 0, 0})

	// Parent span
	parentSpan := ss.Spans().AppendEmpty()
	parentSpan.SetTraceID(traceID)
	parentSpan.SetSpanID(parentSpanID)
	parentSpan.SetName("parent")

	// 3 SELECT spans
	for i := 0; i < 3; i++ {
		span := ss.Spans().AppendEmpty()
		span.SetTraceID(traceID)
		span.SetSpanID(pcommon.SpanID([8]byte{2, byte(i), 0, 0, 0, 0, 0, 0}))
		span.SetParentSpanID(parentSpanID)
		span.SetName("db_query")
		span.Attributes().PutStr("db.operation", "select")
		span.SetStartTimestamp(pcommon.Timestamp(1000000000))
		span.SetEndTimestamp(pcommon.Timestamp(1000000100))
	}

	// 2 INSERT spans
	for i := 0; i < 2; i++ {
		span := ss.Spans().AppendEmpty()
		span.SetTraceID(traceID)
		span.SetSpanID(pcommon.SpanID([8]byte{3, byte(i), 0, 0, 0, 0, 0, 0}))
		span.SetParentSpanID(parentSpanID)
		span.SetName("db_query")
		span.Attributes().PutStr("db.operation", "insert")
		span.SetStartTimestamp(pcommon.Timestamp(1000000000))
		span.SetEndTimestamp(pcommon.Timestamp(1000000100))
	}

	return td
}

func createSingleSpanTrace(t *testing.T) ptrace.Traces {
	t.Helper()
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()

	span := ss.Spans().AppendEmpty()
	span.SetTraceID(pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}))
	span.SetSpanID(pcommon.SpanID([8]byte{1, 0, 0, 0, 0, 0, 0, 0}))
	span.SetName("root")

	return td
}

func createTestTraceWithErrorSpan(t *testing.T) ptrace.Traces {
	t.Helper()
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()

	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	parentSpanID := pcommon.SpanID([8]byte{1, 0, 0, 0, 0, 0, 0, 0})

	// Parent span
	parentSpan := ss.Spans().AppendEmpty()
	parentSpan.SetTraceID(traceID)
	parentSpan.SetSpanID(parentSpanID)
	parentSpan.SetName("parent")

	// Leaf span with OK status
	span1 := ss.Spans().AppendEmpty()
	span1.SetTraceID(traceID)
	span1.SetSpanID(pcommon.SpanID([8]byte{2, 0, 0, 0, 0, 0, 0, 0}))
	span1.SetParentSpanID(parentSpanID)
	span1.SetName("SELECT")
	span1.Status().SetCode(ptrace.StatusCodeOk)
	span1.SetStartTimestamp(pcommon.Timestamp(1000000000))
	span1.SetEndTimestamp(pcommon.Timestamp(1000000100))

	// Leaf span with Error status
	span2 := ss.Spans().AppendEmpty()
	span2.SetTraceID(traceID)
	span2.SetSpanID(pcommon.SpanID([8]byte{2, 1, 0, 0, 0, 0, 0, 0}))
	span2.SetParentSpanID(parentSpanID)
	span2.SetName("SELECT")
	span2.Status().SetCode(ptrace.StatusCodeError)
	span2.Status().SetMessage("query failed")
	span2.SetStartTimestamp(pcommon.Timestamp(1000000000))
	span2.SetEndTimestamp(pcommon.Timestamp(1000000100))

	return td
}

func createTestTraceWithKnownDurations(t *testing.T, durationsNs []int64) ptrace.Traces {
	t.Helper()
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()

	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	parentSpanID := pcommon.SpanID([8]byte{1, 0, 0, 0, 0, 0, 0, 0})

	// Parent span
	parentSpan := ss.Spans().AppendEmpty()
	parentSpan.SetTraceID(traceID)
	parentSpan.SetSpanID(parentSpanID)
	parentSpan.SetName("parent")

	// Leaf spans with specific durations
	baseTime := int64(1000000000)
	for i, duration := range durationsNs {
		span := ss.Spans().AppendEmpty()
		span.SetTraceID(traceID)
		span.SetSpanID(pcommon.SpanID([8]byte{2, byte(i), 0, 0, 0, 0, 0, 0}))
		span.SetParentSpanID(parentSpanID)
		span.SetName("SELECT")
		span.SetStartTimestamp(pcommon.Timestamp(baseTime))
		span.SetEndTimestamp(pcommon.Timestamp(baseTime + duration))
	}

	return td
}

func countSpans(td ptrace.Traces) int {
	count := 0
	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		ilss := rss.At(i).ScopeSpans()
		for j := 0; j < ilss.Len(); j++ {
			count += ilss.At(j).Spans().Len()
		}
	}
	return count
}

func findSpanByNameSuffix(td ptrace.Traces, suffix string) ptrace.Span {
	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		ilss := rss.At(i).ScopeSpans()
		for j := 0; j < ilss.Len(); j++ {
			spans := ilss.At(j).Spans()
			for k := 0; k < spans.Len(); k++ {
				span := spans.At(k)
				if len(span.Name()) >= len(suffix) && span.Name()[len(span.Name())-len(suffix):] == suffix {
					return span
				}
			}
		}
	}
	return ptrace.Span{}
}

func findSpanByNameAndStatus(td ptrace.Traces, nameSuffix string, statusCode ptrace.StatusCode) ptrace.Span {
	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		ilss := rss.At(i).ScopeSpans()
		for j := 0; j < ilss.Len(); j++ {
			spans := ilss.At(j).Spans()
			for k := 0; k < spans.Len(); k++ {
				span := spans.At(k)
				if strings.HasSuffix(span.Name(), nameSuffix) && span.Status().Code() == statusCode {
					return span
				}
			}
		}
	}
	return ptrace.Span{}
}

func createTestTraceWithMixedStatusSpans(t *testing.T) ptrace.Traces {
	t.Helper()
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()

	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	parentSpanID := pcommon.SpanID([8]byte{1, 0, 0, 0, 0, 0, 0, 0})

	// Parent span
	parentSpan := ss.Spans().AppendEmpty()
	parentSpan.SetTraceID(traceID)
	parentSpan.SetSpanID(parentSpanID)
	parentSpan.SetName("parent")

	// 4 leaf spans with OK status
	for i := 0; i < 4; i++ {
		span := ss.Spans().AppendEmpty()
		span.SetTraceID(traceID)
		span.SetSpanID(pcommon.SpanID([8]byte{2, byte(i), 0, 0, 0, 0, 0, 0}))
		span.SetParentSpanID(parentSpanID)
		span.SetName("SELECT")
		span.Status().SetCode(ptrace.StatusCodeOk)
		span.SetStartTimestamp(pcommon.Timestamp(1000000000))
		span.SetEndTimestamp(pcommon.Timestamp(1000000100))
	}

	// 2 leaf spans with Error status
	for i := 0; i < 2; i++ {
		span := ss.Spans().AppendEmpty()
		span.SetTraceID(traceID)
		span.SetSpanID(pcommon.SpanID([8]byte{3, byte(i), 0, 0, 0, 0, 0, 0}))
		span.SetParentSpanID(parentSpanID)
		span.SetName("SELECT")
		span.Status().SetCode(ptrace.StatusCodeError)
		span.Status().SetMessage("query failed")
		span.SetStartTimestamp(pcommon.Timestamp(1000000000))
		span.SetEndTimestamp(pcommon.Timestamp(1000000100))
	}

	return td
}

// Glob pattern matching tests

func TestLeafSpanPruning_GlobPatternWildcard(t *testing.T) {
	// Test: "db.*" pattern matches db.operation, db.name, db.statement
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.MinSpansToAggregate = 2
	cfg.GroupByAttributes = []string{"db.*"}

	tp, err := factory.CreateTraces(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)

	// Create trace with spans having multiple db.* attributes
	td := createTestTraceWithMultipleDbAttrs(t)
	originalSpanCount := countSpans(td)
	assert.Equal(t, 4, originalSpanCount) // 1 parent + 3 leaf spans

	err = tp.ConsumeTraces(t.Context(), td)
	require.NoError(t, err)

	// All 3 leaf spans have same db.* values, should aggregate to 1
	finalSpanCount := countSpans(td)
	assert.Equal(t, 2, finalSpanCount) // 1 parent + 1 summary

	summarySpan := findSpanByNameSuffix(td, "_aggregated")
	require.NotNil(t, summarySpan)

	attrs := summarySpan.Attributes()
	spanCount, _ := attrs.Get("aggregation.span_count")
	assert.Equal(t, int64(3), spanCount.Int())
}

func TestLeafSpanPruning_GlobPatternSeparatesGroups(t *testing.T) {
	// Test: spans with different db.* values should be in separate groups
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.MinSpansToAggregate = 2
	cfg.GroupByAttributes = []string{"db.*"}

	tp, err := factory.CreateTraces(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)

	// Create trace with spans having different db.operation values
	td := createTestTraceWithDifferentDbOperations(t)
	originalSpanCount := countSpans(td)
	assert.Equal(t, 5, originalSpanCount) // 1 parent + 2 select + 2 insert

	err = tp.ConsumeTraces(t.Context(), td)
	require.NoError(t, err)

	// 2 select spans -> 1 summary, 2 insert spans -> 1 summary
	finalSpanCount := countSpans(td)
	assert.Equal(t, 3, finalSpanCount) // 1 parent + 2 summaries
}

func TestLeafSpanPruning_GlobPatternMultiplePatterns(t *testing.T) {
	// Test: multiple glob patterns ["db.*", "http.*"]
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.MinSpansToAggregate = 2
	cfg.GroupByAttributes = []string{"db.*", "http.*"}

	tp, err := factory.CreateTraces(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)

	td := createTestTraceWithDbAndHttpAttrs(t)
	originalSpanCount := countSpans(td)
	assert.Equal(t, 4, originalSpanCount) // 1 parent + 3 leaf spans

	err = tp.ConsumeTraces(t.Context(), td)
	require.NoError(t, err)

	// All spans have same db.* and http.* values, should aggregate
	finalSpanCount := countSpans(td)
	assert.Equal(t, 2, finalSpanCount)
}

func TestLeafSpanPruning_GlobPatternExactMatch(t *testing.T) {
	// Test: pattern without wildcard "db.operation" matches exactly
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.MinSpansToAggregate = 2
	cfg.GroupByAttributes = []string{"db.operation"}

	tp, err := factory.CreateTraces(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)

	td := createTestTraceWithMultipleDbAttrs(t)

	err = tp.ConsumeTraces(t.Context(), td)
	require.NoError(t, err)

	// Should still group by db.operation exactly
	finalSpanCount := countSpans(td)
	assert.Equal(t, 2, finalSpanCount)
}

func TestLeafSpanPruning_InvalidGlobPattern(t *testing.T) {
	// Test: invalid glob pattern should return error during creation
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.GroupByAttributes = []string{"[invalid"}

	_, err := factory.CreateTraces(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid glob pattern")
}

// Helper functions for glob pattern tests

func createTestTraceWithMultipleDbAttrs(t *testing.T) ptrace.Traces {
	t.Helper()
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()

	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	parentSpanID := pcommon.SpanID([8]byte{1, 0, 0, 0, 0, 0, 0, 0})

	// Parent span
	parentSpan := ss.Spans().AppendEmpty()
	parentSpan.SetTraceID(traceID)
	parentSpan.SetSpanID(parentSpanID)
	parentSpan.SetName("parent")

	// 3 leaf spans with identical db.* attributes
	for i := 0; i < 3; i++ {
		span := ss.Spans().AppendEmpty()
		span.SetTraceID(traceID)
		span.SetSpanID(pcommon.SpanID([8]byte{2, byte(i), 0, 0, 0, 0, 0, 0}))
		span.SetParentSpanID(parentSpanID)
		span.SetName("db_query")
		span.Attributes().PutStr("db.operation", "select")
		span.Attributes().PutStr("db.name", "users")
		span.Attributes().PutStr("db.system", "postgresql")
		span.SetStartTimestamp(pcommon.Timestamp(1000000000))
		span.SetEndTimestamp(pcommon.Timestamp(1000000100))
	}

	return td
}

func createTestTraceWithDifferentDbOperations(t *testing.T) ptrace.Traces {
	t.Helper()
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()

	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	parentSpanID := pcommon.SpanID([8]byte{1, 0, 0, 0, 0, 0, 0, 0})

	// Parent span
	parentSpan := ss.Spans().AppendEmpty()
	parentSpan.SetTraceID(traceID)
	parentSpan.SetSpanID(parentSpanID)
	parentSpan.SetName("parent")

	// 2 SELECT spans
	for i := 0; i < 2; i++ {
		span := ss.Spans().AppendEmpty()
		span.SetTraceID(traceID)
		span.SetSpanID(pcommon.SpanID([8]byte{2, byte(i), 0, 0, 0, 0, 0, 0}))
		span.SetParentSpanID(parentSpanID)
		span.SetName("db_query")
		span.Attributes().PutStr("db.operation", "select")
		span.Attributes().PutStr("db.name", "users")
		span.SetStartTimestamp(pcommon.Timestamp(1000000000))
		span.SetEndTimestamp(pcommon.Timestamp(1000000100))
	}

	// 2 INSERT spans
	for i := 0; i < 2; i++ {
		span := ss.Spans().AppendEmpty()
		span.SetTraceID(traceID)
		span.SetSpanID(pcommon.SpanID([8]byte{3, byte(i), 0, 0, 0, 0, 0, 0}))
		span.SetParentSpanID(parentSpanID)
		span.SetName("db_query")
		span.Attributes().PutStr("db.operation", "insert")
		span.Attributes().PutStr("db.name", "users")
		span.SetStartTimestamp(pcommon.Timestamp(1000000000))
		span.SetEndTimestamp(pcommon.Timestamp(1000000100))
	}

	return td
}

func createTestTraceWithDbAndHttpAttrs(t *testing.T) ptrace.Traces {
	t.Helper()
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()

	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	parentSpanID := pcommon.SpanID([8]byte{1, 0, 0, 0, 0, 0, 0, 0})

	// Parent span
	parentSpan := ss.Spans().AppendEmpty()
	parentSpan.SetTraceID(traceID)
	parentSpan.SetSpanID(parentSpanID)
	parentSpan.SetName("parent")

	// 3 leaf spans with both db.* and http.* attributes
	for i := 0; i < 3; i++ {
		span := ss.Spans().AppendEmpty()
		span.SetTraceID(traceID)
		span.SetSpanID(pcommon.SpanID([8]byte{2, byte(i), 0, 0, 0, 0, 0, 0}))
		span.SetParentSpanID(parentSpanID)
		span.SetName("api_call")
		span.Attributes().PutStr("db.operation", "select")
		span.Attributes().PutStr("db.name", "users")
		span.Attributes().PutStr("http.method", "GET")
		span.Attributes().PutStr("http.route", "/api/users")
		span.SetStartTimestamp(pcommon.Timestamp(1000000000))
		span.SetEndTimestamp(pcommon.Timestamp(1000000100))
	}

	return td
}

func TestLeafSpanPruningProcessorWithHistogram(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                          string
		buckets                       []time.Duration
		expectedBuckets               []float64
		spanDurations                 []time.Duration
		expectedBucketCounts          []int64
		minSpansToAggregate           int
		expectedSpanCount             int
		shouldHaveHistogramAttributes bool
	}{
		{
			name: "aggregate 3 spans with histogram",
			buckets: []time.Duration{
				10 * time.Millisecond,
				50 * time.Millisecond,
				100 * time.Millisecond,
			},
			expectedBuckets: []float64{0.01, 0.05, 0.1},
			spanDurations: []time.Duration{
				5 * time.Millisecond,   // Should fall in bucket 0 (<=10ms)
				15 * time.Millisecond,  // Should fall in bucket 1 (<=50ms)
				25 * time.Millisecond,  // Should fall in bucket 1 (<=50ms)
				75 * time.Millisecond,  // Should fall in bucket 2 (<=100ms)
				150 * time.Millisecond, // Should fall in bucket 3 (+Inf)
			},
			expectedBucketCounts:          []int64{1, 3, 4, 5},
			minSpansToAggregate:           2,
			expectedSpanCount:             2, // 1 original + 1 summary
			shouldHaveHistogramAttributes: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a simple trace
			td := ptrace.NewTraces()
			rs := td.ResourceSpans().AppendEmpty()
			rs.Resource().Attributes().PutStr("service.name", "test-service")

			// Create spans with specified durations
			ils := rs.ScopeSpans().AppendEmpty()
			spans := ils.Spans()

			traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
			parentSpanID := pcommon.SpanID([8]byte{1, 0, 0, 0, 0, 0, 0, 0})

			// Create parent span
			parentSpan := spans.AppendEmpty()
			parentSpan.SetName("parent")
			parentSpan.SetTraceID(traceID)
			parentSpan.SetSpanID(parentSpanID)

			// Create leaf spans with varying durations
			for i, duration := range tt.spanDurations {
				span := spans.AppendEmpty()
				span.SetName("test-span")
				span.SetTraceID(traceID)
				span.SetSpanID(pcommon.SpanID([8]byte{2, byte(i), 0, 0, 0, 0, 0, 0}))
				span.SetParentSpanID(parentSpanID)
				span.SetKind(ptrace.SpanKindInternal)

				startTime := pcommon.Timestamp(1000000000)
				endTime := pcommon.Timestamp(1000000000 + int64(duration))
				span.SetStartTimestamp(startTime)
				span.SetEndTimestamp(endTime)

				// Add attributes to ensure they don't interfere with grouping
				span.Attributes().PutStr("db.operation", "SELECT")
			}

			// Create processor with custom buckets
			cfg := &Config{
				GroupByAttributes:           []string{"db.operation"},
				MinSpansToAggregate:         tt.minSpansToAggregate,
				SummarySpanNameSuffix:       "_aggregated",
				AggregationAttributePrefix:  "aggregation.",
				AggregationHistogramBuckets: tt.buckets,
			}

			ctx := context.Background()
			set := processortest.NewNopSettings(metadata.Type)

			p, err := newSpanPruningProcessor(set, cfg)
			assert.NoError(t, err)

			resultTd, err := p.processTraces(ctx, td)
			assert.NoError(t, err)

			// Get the spans
			spanCount := 0
			rss := resultTd.ResourceSpans()
			for i := 0; i < rss.Len(); i++ {
				rs := rss.At(i)
				ilss := rs.ScopeSpans()
				for j := 0; j < ilss.Len(); j++ {
					ils := ilss.At(j)
					spans := ils.Spans()
					spanCount += spans.Len()
				}
			}

			// Verify we have the expected number of spans
			assert.Equal(t, tt.expectedSpanCount, spanCount)

			// If we have a summary span, check its histogram attributes
			var summarySpan ptrace.Span
			foundSummary := false
			rss = resultTd.ResourceSpans()
			for i := 0; i < rss.Len() && !foundSummary; i++ {
				rs := rss.At(i)
				ilss := rs.ScopeSpans()
				for j := 0; j < ilss.Len() && !foundSummary; j++ {
					ils := ilss.At(j)
					spans := ils.Spans()
					for k := 0; k < spans.Len(); k++ {
						span := spans.At(k)
						if strings.Contains(span.Name(), "_aggregated") {
							summarySpan = span
							foundSummary = true
							break
						}
					}
				}
			}

			// If we are expecting histogram attributes, verify them
			if tt.shouldHaveHistogramAttributes {
				// Check if bucket bounds are present and correct
				bounds, exists := summarySpan.Attributes().Get("aggregation.histogram_bucket_bounds_s")
				assert.True(t, exists)
				assert.Equal(t, len(tt.expectedBuckets), bounds.Slice().Len())

				counts, exists := summarySpan.Attributes().Get("aggregation.histogram_bucket_counts")
				assert.True(t, exists)
				assert.Equal(t, len(tt.expectedBucketCounts), counts.Slice().Len())
			}
		})
	}
}

// TestLeafSpanPruning_RecursiveParentAggregation tests that parent spans are aggregated
// when all their children are aggregated
func TestLeafSpanPruning_RecursiveParentAggregation(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.MinSpansToAggregate = 2
	cfg.GroupByAttributes = []string{"db.op"}

	tp, err := factory.CreateTraces(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)

	// Create the complex trace from the plan example
	td := createTestTraceWithRecursiveAggregation(t)

	// Before: 1 root + 3 OK handlers + 3 OK SELECTs + 2 Error handlers + 2 Error SELECTs + 1 OK handler + 1 INSERT + 1 worker + 1 SELECT = 15 spans
	originalSpanCount := countSpans(td)
	assert.Equal(t, 15, originalSpanCount)

	err = tp.ConsumeTraces(t.Context(), td)
	require.NoError(t, err)

	// After: 1 root + 1 OK handler_aggregated + 1 OK SELECT_aggregated + 1 Error handler_aggregated + 1 Error SELECT_aggregated + 1 OK handler + 1 INSERT + 1 worker + 1 SELECT = 9 spans
	finalSpanCount := countSpans(td)
	assert.Equal(t, 9, finalSpanCount)

	// Verify aggregated spans exist
	handlerOKAgg, found := findSpanByName(td, "handler_aggregated", "Ok")
	require.True(t, found, "OK handler_aggregated should exist")

	handlerErrorAgg, found := findSpanByName(td, "handler_aggregated", "Error")
	require.True(t, found, "Error handler_aggregated should exist")

	selectOKAgg, found := findSpanByName(td, "SELECT_aggregated", "Ok")
	require.True(t, found, "OK SELECT_aggregated should exist")

	selectErrorAgg, found := findSpanByName(td, "SELECT_aggregated", "Error")
	require.True(t, found, "Error SELECT_aggregated should exist")

	// Verify span counts
	handlerOKCount, _ := handlerOKAgg.Attributes().Get("aggregation.span_count")
	assert.Equal(t, int64(3), handlerOKCount.Int())

	handlerErrorCount, _ := handlerErrorAgg.Attributes().Get("aggregation.span_count")
	assert.Equal(t, int64(2), handlerErrorCount.Int())

	selectOKCount, _ := selectOKAgg.Attributes().Get("aggregation.span_count")
	assert.Equal(t, int64(3), selectOKCount.Int())

	selectErrorCount, _ := selectErrorAgg.Attributes().Get("aggregation.span_count")
	assert.Equal(t, int64(2), selectErrorCount.Int())

	// Verify parent-child relationships
	// SELECT_aggregated (OK) should be child of handler_aggregated (OK)
	assert.Equal(t, handlerOKAgg.SpanID(), selectOKAgg.ParentSpanID())

	// SELECT_aggregated (Error) should be child of handler_aggregated (Error)
	assert.Equal(t, handlerErrorAgg.SpanID(), selectErrorAgg.ParentSpanID())

	// Verify non-aggregated spans still exist
	_, foundInsert := findSpanByExactName(td, "INSERT")
	require.True(t, foundInsert, "INSERT span should still exist")

	_, foundWorker := findSpanByExactName(td, "worker")
	require.True(t, foundWorker, "worker span should still exist")
}

// TestLeafSpanPruning_ParentNotAggregatedIfChildrenMixed tests that parents are not
// aggregated if some children are aggregated but others are not
func TestLeafSpanPruning_ParentNotAggregatedIfChildrenMixed(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.MinSpansToAggregate = 2

	tp, err := factory.CreateTraces(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)

	td := createTestTraceWithMixedChildren(t)

	// Before: 1 root + 2 handlers + 3 SELECTs + 1 INSERT = 7 spans
	originalSpanCount := countSpans(td)
	assert.Equal(t, 7, originalSpanCount)

	err = tp.ConsumeTraces(t.Context(), td)
	require.NoError(t, err)

	// After: 1 root + 2 handlers + 1 SELECT_aggregated + 1 INSERT = 5 spans
	// Handlers should NOT be aggregated because one has a non-aggregated child (INSERT)
	finalSpanCount := countSpans(td)
	assert.Equal(t, 5, finalSpanCount)

	// Verify handler_aggregated does NOT exist
	_, found := findSpanByExactName(td, "handler_aggregated")
	assert.False(t, found, "handler_aggregated should NOT exist")

	// Verify SELECT_aggregated exists
	selectAgg, found := findSpanByExactName(td, "SELECT_aggregated")
	require.True(t, found, "SELECT_aggregated should exist")

	selectCount, _ := selectAgg.Attributes().Get("aggregation.span_count")
	assert.Equal(t, int64(3), selectCount.Int())

	// Verify original handler spans still exist
	handlers := findAllSpansByExactName(td, "handler")
	assert.Equal(t, 2, len(handlers), "both handler spans should still exist")
}

// TestLeafSpanPruning_RootSpansNotAggregated tests that root spans (with no parent)
// are never aggregated
func TestLeafSpanPruning_RootSpansNotAggregated(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.MinSpansToAggregate = 2

	tp, err := factory.CreateTraces(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)

	td := createTestTraceWithMultipleRoots(t)

	// Before: 3 root spans + 6 leaf spans (2 per root) = 9 spans
	originalSpanCount := countSpans(td)
	assert.Equal(t, 9, originalSpanCount)

	err = tp.ConsumeTraces(t.Context(), td)
	require.NoError(t, err)

	// After: 3 root spans + 1 SELECT_aggregated = 4 spans
	// Root spans should NOT be aggregated even though all children are aggregated
	finalSpanCount := countSpans(td)
	assert.Equal(t, 4, finalSpanCount)

	// Verify all root spans still exist
	roots := findAllSpansByExactName(td, "root")
	assert.Equal(t, 3, len(roots), "all root spans should still exist")

	// Verify SELECT_aggregated exists
	selectAgg, found := findSpanByExactName(td, "SELECT_aggregated")
	require.True(t, found, "SELECT_aggregated should exist")

	selectCount, _ := selectAgg.Attributes().Get("aggregation.span_count")
	assert.Equal(t, int64(6), selectCount.Int())
}

// TestLeafSpanPruning_ThreeLevelAggregation tests aggregation across three levels
func TestLeafSpanPruning_ThreeLevelAggregation(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.MinSpansToAggregate = 2
	cfg.MaxParentDepth = -1 // unlimited for this test

	tp, err := factory.CreateTraces(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)

	td := createTestTraceWithThreeLevels(t)

	// Before: 1 root + 2 middleware + 4 handlers (2 per middleware) + 8 SELECTs (2 per handler) = 15 spans
	originalSpanCount := countSpans(td)
	assert.Equal(t, 15, originalSpanCount)

	err = tp.ConsumeTraces(t.Context(), td)
	require.NoError(t, err)

	// After: 1 root + 1 middleware_aggregated + 1 handler_aggregated + 1 SELECT_aggregated = 4 spans
	finalSpanCount := countSpans(td)
	assert.Equal(t, 4, finalSpanCount)

	// Verify all aggregated spans exist
	middlewareAgg, found := findSpanByExactName(td, "middleware_aggregated")
	require.True(t, found, "middleware_aggregated should exist")

	handlerAgg, found := findSpanByExactName(td, "handler_aggregated")
	require.True(t, found, "handler_aggregated should exist")

	selectAgg, found := findSpanByExactName(td, "SELECT_aggregated")
	require.True(t, found, "SELECT_aggregated should exist")

	// Verify parent-child relationships
	// handler_aggregated should be child of middleware_aggregated
	assert.Equal(t, middlewareAgg.SpanID(), handlerAgg.ParentSpanID())

	// SELECT_aggregated should be child of handler_aggregated
	assert.Equal(t, handlerAgg.SpanID(), selectAgg.ParentSpanID())

	// Verify span counts
	middlewareCount, _ := middlewareAgg.Attributes().Get("aggregation.span_count")
	assert.Equal(t, int64(2), middlewareCount.Int())

	handlerCount, _ := handlerAgg.Attributes().Get("aggregation.span_count")
	assert.Equal(t, int64(4), handlerCount.Int())

	selectCount, _ := selectAgg.Attributes().Get("aggregation.span_count")
	assert.Equal(t, int64(8), selectCount.Int())
}

// Helper functions for new tests

func createTestTraceWithRecursiveAggregation(t *testing.T) ptrace.Traces {
	t.Helper()
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()

	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	rootSpanID := pcommon.SpanID([8]byte{1, 0, 0, 0, 0, 0, 0, 0})

	// Root span
	root := ss.Spans().AppendEmpty()
	root.SetTraceID(traceID)
	root.SetSpanID(rootSpanID)
	root.SetName("root")
	root.Status().SetCode(ptrace.StatusCodeOk)

	// 3x handler (OK) -> SELECT (OK, db.op=select)
	for i := 0; i < 3; i++ {
		handlerID := pcommon.SpanID([8]byte{2, byte(i), 0, 0, 0, 0, 0, 0})
		handler := ss.Spans().AppendEmpty()
		handler.SetTraceID(traceID)
		handler.SetSpanID(handlerID)
		handler.SetParentSpanID(rootSpanID)
		handler.SetName("handler")
		handler.Status().SetCode(ptrace.StatusCodeOk)

		selectSpan := ss.Spans().AppendEmpty()
		selectSpan.SetTraceID(traceID)
		selectSpan.SetSpanID(pcommon.SpanID([8]byte{3, byte(i), 0, 0, 0, 0, 0, 0}))
		selectSpan.SetParentSpanID(handlerID)
		selectSpan.SetName("SELECT")
		selectSpan.Attributes().PutStr("db.op", "select")
		selectSpan.Status().SetCode(ptrace.StatusCodeOk)
		selectSpan.SetStartTimestamp(pcommon.Timestamp(1000000000))
		selectSpan.SetEndTimestamp(pcommon.Timestamp(1000000100))
	}

	// 2x handler (Error) -> SELECT (Error, db.op=select)
	for i := 0; i < 2; i++ {
		handlerID := pcommon.SpanID([8]byte{4, byte(i), 0, 0, 0, 0, 0, 0})
		handler := ss.Spans().AppendEmpty()
		handler.SetTraceID(traceID)
		handler.SetSpanID(handlerID)
		handler.SetParentSpanID(rootSpanID)
		handler.SetName("handler")
		handler.Status().SetCode(ptrace.StatusCodeError)

		selectSpan := ss.Spans().AppendEmpty()
		selectSpan.SetTraceID(traceID)
		selectSpan.SetSpanID(pcommon.SpanID([8]byte{5, byte(i), 0, 0, 0, 0, 0, 0}))
		selectSpan.SetParentSpanID(handlerID)
		selectSpan.SetName("SELECT")
		selectSpan.Attributes().PutStr("db.op", "select")
		selectSpan.Status().SetCode(ptrace.StatusCodeError)
		selectSpan.SetStartTimestamp(pcommon.Timestamp(1000000000))
		selectSpan.SetEndTimestamp(pcommon.Timestamp(1000000100))
	}

	// 1x handler (OK) -> INSERT (OK, db.op=insert) - below threshold
	handlerID := pcommon.SpanID([8]byte{6, 0, 0, 0, 0, 0, 0, 0})
	handler := ss.Spans().AppendEmpty()
	handler.SetTraceID(traceID)
	handler.SetSpanID(handlerID)
	handler.SetParentSpanID(rootSpanID)
	handler.SetName("handler")
	handler.Status().SetCode(ptrace.StatusCodeOk)

	insertSpan := ss.Spans().AppendEmpty()
	insertSpan.SetTraceID(traceID)
	insertSpan.SetSpanID(pcommon.SpanID([8]byte{7, 0, 0, 0, 0, 0, 0, 0}))
	insertSpan.SetParentSpanID(handlerID)
	insertSpan.SetName("INSERT")
	insertSpan.Attributes().PutStr("db.op", "insert")
	insertSpan.Status().SetCode(ptrace.StatusCodeOk)
	insertSpan.SetStartTimestamp(pcommon.Timestamp(1000000000))
	insertSpan.SetEndTimestamp(pcommon.Timestamp(1000000100))

	// 1x worker (OK) -> SELECT (OK, db.op=select) - different parent name
	workerID := pcommon.SpanID([8]byte{8, 0, 0, 0, 0, 0, 0, 0})
	worker := ss.Spans().AppendEmpty()
	worker.SetTraceID(traceID)
	worker.SetSpanID(workerID)
	worker.SetParentSpanID(rootSpanID)
	worker.SetName("worker")
	worker.Status().SetCode(ptrace.StatusCodeOk)

	selectSpan := ss.Spans().AppendEmpty()
	selectSpan.SetTraceID(traceID)
	selectSpan.SetSpanID(pcommon.SpanID([8]byte{9, 0, 0, 0, 0, 0, 0, 0}))
	selectSpan.SetParentSpanID(workerID)
	selectSpan.SetName("SELECT")
	selectSpan.Attributes().PutStr("db.op", "select")
	selectSpan.Status().SetCode(ptrace.StatusCodeOk)
	selectSpan.SetStartTimestamp(pcommon.Timestamp(1000000000))
	selectSpan.SetEndTimestamp(pcommon.Timestamp(1000000100))

	return td
}

func createTestTraceWithMixedChildren(t *testing.T) ptrace.Traces {
	t.Helper()
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()

	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	rootSpanID := pcommon.SpanID([8]byte{1, 0, 0, 0, 0, 0, 0, 0})

	// Root span
	root := ss.Spans().AppendEmpty()
	root.SetTraceID(traceID)
	root.SetSpanID(rootSpanID)
	root.SetName("root")

	// Handler 1 with 3 SELECTs (will be aggregated)
	handler1ID := pcommon.SpanID([8]byte{2, 0, 0, 0, 0, 0, 0, 0})
	handler1 := ss.Spans().AppendEmpty()
	handler1.SetTraceID(traceID)
	handler1.SetSpanID(handler1ID)
	handler1.SetParentSpanID(rootSpanID)
	handler1.SetName("handler")
	handler1.Status().SetCode(ptrace.StatusCodeOk)

	for i := 0; i < 3; i++ {
		selectSpan := ss.Spans().AppendEmpty()
		selectSpan.SetTraceID(traceID)
		selectSpan.SetSpanID(pcommon.SpanID([8]byte{3, byte(i), 0, 0, 0, 0, 0, 0}))
		selectSpan.SetParentSpanID(handler1ID)
		selectSpan.SetName("SELECT")
		selectSpan.Status().SetCode(ptrace.StatusCodeOk)
		selectSpan.SetStartTimestamp(pcommon.Timestamp(1000000000))
		selectSpan.SetEndTimestamp(pcommon.Timestamp(1000000100))
	}

	// Handler 2 with 1 INSERT (not aggregated - mixed children)
	handler2ID := pcommon.SpanID([8]byte{4, 0, 0, 0, 0, 0, 0, 0})
	handler2 := ss.Spans().AppendEmpty()
	handler2.SetTraceID(traceID)
	handler2.SetSpanID(handler2ID)
	handler2.SetParentSpanID(rootSpanID)
	handler2.SetName("handler")
	handler2.Status().SetCode(ptrace.StatusCodeOk)

	insertSpan := ss.Spans().AppendEmpty()
	insertSpan.SetTraceID(traceID)
	insertSpan.SetSpanID(pcommon.SpanID([8]byte{5, 0, 0, 0, 0, 0, 0, 0}))
	insertSpan.SetParentSpanID(handler2ID)
	insertSpan.SetName("INSERT")
	insertSpan.Status().SetCode(ptrace.StatusCodeOk)
	insertSpan.SetStartTimestamp(pcommon.Timestamp(1000000000))
	insertSpan.SetEndTimestamp(pcommon.Timestamp(1000000100))

	return td
}

func createTestTraceWithMultipleRoots(t *testing.T) ptrace.Traces {
	t.Helper()
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()

	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})

	// 3 root spans, each with 2 SELECT children
	for i := 0; i < 3; i++ {
		rootID := pcommon.SpanID([8]byte{byte(i + 1), 0, 0, 0, 0, 0, 0, 0})
		root := ss.Spans().AppendEmpty()
		root.SetTraceID(traceID)
		root.SetSpanID(rootID)
		root.SetName("root")
		root.Status().SetCode(ptrace.StatusCodeOk)

		for j := 0; j < 2; j++ {
			selectSpan := ss.Spans().AppendEmpty()
			selectSpan.SetTraceID(traceID)
			selectSpan.SetSpanID(pcommon.SpanID([8]byte{byte(i + 4), byte(j), 0, 0, 0, 0, 0, 0}))
			selectSpan.SetParentSpanID(rootID)
			selectSpan.SetName("SELECT")
			selectSpan.Status().SetCode(ptrace.StatusCodeOk)
			selectSpan.SetStartTimestamp(pcommon.Timestamp(1000000000))
			selectSpan.SetEndTimestamp(pcommon.Timestamp(1000000100))
		}
	}

	return td
}

func createTestTraceWithThreeLevels(t *testing.T) ptrace.Traces {
	t.Helper()
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()

	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	rootSpanID := pcommon.SpanID([8]byte{1, 0, 0, 0, 0, 0, 0, 0})

	// Root span
	root := ss.Spans().AppendEmpty()
	root.SetTraceID(traceID)
	root.SetSpanID(rootSpanID)
	root.SetName("root")

	spanIDCounter := byte(2)

	// 2 middleware spans
	for i := 0; i < 2; i++ {
		middlewareID := pcommon.SpanID([8]byte{spanIDCounter, 0, 0, 0, 0, 0, 0, 0})
		spanIDCounter++

		middleware := ss.Spans().AppendEmpty()
		middleware.SetTraceID(traceID)
		middleware.SetSpanID(middlewareID)
		middleware.SetParentSpanID(rootSpanID)
		middleware.SetName("middleware")
		middleware.Status().SetCode(ptrace.StatusCodeOk)

		// Each middleware has 2 handler spans
		for j := 0; j < 2; j++ {
			handlerID := pcommon.SpanID([8]byte{spanIDCounter, 0, 0, 0, 0, 0, 0, 0})
			spanIDCounter++

			handler := ss.Spans().AppendEmpty()
			handler.SetTraceID(traceID)
			handler.SetSpanID(handlerID)
			handler.SetParentSpanID(middlewareID)
			handler.SetName("handler")
			handler.Status().SetCode(ptrace.StatusCodeOk)

			// Each handler has 2 SELECT spans
			for k := 0; k < 2; k++ {
				selectSpan := ss.Spans().AppendEmpty()
				selectSpan.SetTraceID(traceID)
				selectSpan.SetSpanID(pcommon.SpanID([8]byte{spanIDCounter, 0, 0, 0, 0, 0, 0, 0}))
				spanIDCounter++
				selectSpan.SetParentSpanID(handlerID)
				selectSpan.SetName("SELECT")
				selectSpan.Status().SetCode(ptrace.StatusCodeOk)
				selectSpan.SetStartTimestamp(pcommon.Timestamp(1000000000))
				selectSpan.SetEndTimestamp(pcommon.Timestamp(1000000100))
			}
		}
	}

	return td
}

func findSpanByName(td ptrace.Traces, nameSubstring string, statusCode string) (ptrace.Span, bool) {
	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		rs := rss.At(i)
		ilss := rs.ScopeSpans()
		for j := 0; j < ilss.Len(); j++ {
			ils := ilss.At(j)
			spans := ils.Spans()
			for k := 0; k < spans.Len(); k++ {
				span := spans.At(k)
				if strings.Contains(span.Name(), nameSubstring) && span.Status().Code().String() == statusCode {
					return span, true
				}
			}
		}
	}
	return ptrace.Span{}, false
}

func findSpanByExactName(td ptrace.Traces, name string) (ptrace.Span, bool) {
	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		rs := rss.At(i)
		ilss := rs.ScopeSpans()
		for j := 0; j < ilss.Len(); j++ {
			ils := ilss.At(j)
			spans := ils.Spans()
			for k := 0; k < spans.Len(); k++ {
				span := spans.At(k)
				if span.Name() == name {
					return span, true
				}
			}
		}
	}
	return ptrace.Span{}, false
}

func findAllSpansByExactName(td ptrace.Traces, name string) []ptrace.Span {
	var result []ptrace.Span
	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		rs := rss.At(i)
		ilss := rs.ScopeSpans()
		for j := 0; j < ilss.Len(); j++ {
			ils := ilss.At(j)
			spans := ils.Spans()
			for k := 0; k < spans.Len(); k++ {
				span := spans.At(k)
				if span.Name() == name {
					result = append(result, span)
				}
			}
		}
	}
	return result
}

// Tree-based edge case tests

func TestLeafSpanPruning_OrphanSpans(t *testing.T) {
	// Test: orphan spans (parent not in trace) should still be processed as potential leaves
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.MinSpansToAggregate = 2

	tp, err := factory.CreateTraces(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)

	td := createTestTraceWithOrphanSpans(t)
	originalSpanCount := countSpans(td)
	assert.Equal(t, 4, originalSpanCount) // 1 root + 3 orphan leaf spans

	err = tp.ConsumeTraces(t.Context(), td)
	require.NoError(t, err)

	// Orphan spans with same name should still be aggregated
	// 3 orphan SELECT spans -> 1 summary + 1 root
	finalSpanCount := countSpans(td)
	assert.Equal(t, 2, finalSpanCount)

	summarySpan := findSpanByNameSuffix(td, "_aggregated")
	require.NotNil(t, summarySpan)

	attrs := summarySpan.Attributes()
	spanCount, _ := attrs.Get("aggregation.span_count")
	assert.Equal(t, int64(3), spanCount.Int())
}

func TestLeafSpanPruning_MultipleRootSpans(t *testing.T) {
	// Test: multiple root spans (no parent) in a trace should be handled gracefully
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.MinSpansToAggregate = 2

	tp, err := factory.CreateTraces(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)

	td := createTestTraceWithMultipleRootsTree(t)
	originalSpanCount := countSpans(td)
	assert.Equal(t, 5, originalSpanCount) // 2 roots + 3 leaf spans under first root

	err = tp.ConsumeTraces(t.Context(), td)
	require.NoError(t, err)

	// The 3 leaf spans under first root should aggregate
	// Final: 2 roots + 1 summary = 3
	finalSpanCount := countSpans(td)
	assert.Equal(t, 3, finalSpanCount)

	summarySpan := findSpanByNameSuffix(td, "_aggregated")
	require.NotNil(t, summarySpan)

	attrs := summarySpan.Attributes()
	spanCount, _ := attrs.Get("aggregation.span_count")
	assert.Equal(t, int64(3), spanCount.Int())
}

func TestLeafSpanPruning_NoRootSpan(t *testing.T) {
	// Test: trace with no root span (all spans have parents not in trace)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.MinSpansToAggregate = 2

	tp, err := factory.CreateTraces(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)

	td := createTestTraceWithNoRoot(t)
	originalSpanCount := countSpans(td)
	assert.Equal(t, 3, originalSpanCount) // 3 orphan leaf spans (all point to missing parent)

	err = tp.ConsumeTraces(t.Context(), td)
	require.NoError(t, err)

	// Should still aggregate the 3 orphan spans
	finalSpanCount := countSpans(td)
	assert.Equal(t, 1, finalSpanCount) // 1 summary

	summarySpan := findSpanByNameSuffix(td, "_aggregated")
	require.NotNil(t, summarySpan)

	attrs := summarySpan.Attributes()
	spanCount, _ := attrs.Get("aggregation.span_count")
	assert.Equal(t, int64(3), spanCount.Int())
}

// Helper functions for tree edge case tests

func createTestTraceWithOrphanSpans(t *testing.T) ptrace.Traces {
	t.Helper()
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()

	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	rootSpanID := pcommon.SpanID([8]byte{1, 0, 0, 0, 0, 0, 0, 0})
	missingParentID := pcommon.SpanID([8]byte{99, 0, 0, 0, 0, 0, 0, 0}) // Not in trace

	// Root span
	rootSpan := ss.Spans().AppendEmpty()
	rootSpan.SetTraceID(traceID)
	rootSpan.SetSpanID(rootSpanID)
	rootSpan.SetName("root")
	rootSpan.SetStartTimestamp(pcommon.Timestamp(1000000000))
	rootSpan.SetEndTimestamp(pcommon.Timestamp(1000000500))

	// 3 orphan leaf spans (parent not in trace)
	for i := 0; i < 3; i++ {
		span := ss.Spans().AppendEmpty()
		span.SetTraceID(traceID)
		span.SetSpanID(pcommon.SpanID([8]byte{2, byte(i), 0, 0, 0, 0, 0, 0}))
		span.SetParentSpanID(missingParentID) // Parent not in trace
		span.SetName("SELECT")
		span.SetStartTimestamp(pcommon.Timestamp(1000000100))
		span.SetEndTimestamp(pcommon.Timestamp(1000000200))
	}

	return td
}

func createTestTraceWithMultipleRootsTree(t *testing.T) ptrace.Traces {
	t.Helper()
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()

	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	root1SpanID := pcommon.SpanID([8]byte{1, 0, 0, 0, 0, 0, 0, 0})
	root2SpanID := pcommon.SpanID([8]byte{2, 0, 0, 0, 0, 0, 0, 0})

	// First root span (earlier timestamp)
	root1 := ss.Spans().AppendEmpty()
	root1.SetTraceID(traceID)
	root1.SetSpanID(root1SpanID)
	root1.SetName("root1")
	root1.SetStartTimestamp(pcommon.Timestamp(1000000000))
	root1.SetEndTimestamp(pcommon.Timestamp(1000000500))

	// Second root span (later timestamp)
	root2 := ss.Spans().AppendEmpty()
	root2.SetTraceID(traceID)
	root2.SetSpanID(root2SpanID)
	root2.SetName("root2")
	root2.SetStartTimestamp(pcommon.Timestamp(1000000100))
	root2.SetEndTimestamp(pcommon.Timestamp(1000000600))

	// 3 leaf spans under first root
	for i := 0; i < 3; i++ {
		span := ss.Spans().AppendEmpty()
		span.SetTraceID(traceID)
		span.SetSpanID(pcommon.SpanID([8]byte{3, byte(i), 0, 0, 0, 0, 0, 0}))
		span.SetParentSpanID(root1SpanID)
		span.SetName("SELECT")
		span.SetStartTimestamp(pcommon.Timestamp(1000000100))
		span.SetEndTimestamp(pcommon.Timestamp(1000000200))
	}

	return td
}

func createTestTraceWithNoRoot(t *testing.T) ptrace.Traces {
	t.Helper()
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()

	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	missingParentID := pcommon.SpanID([8]byte{99, 0, 0, 0, 0, 0, 0, 0}) // Not in trace

	// 3 orphan leaf spans all pointing to missing parent
	for i := 0; i < 3; i++ {
		span := ss.Spans().AppendEmpty()
		span.SetTraceID(traceID)
		span.SetSpanID(pcommon.SpanID([8]byte{2, byte(i), 0, 0, 0, 0, 0, 0}))
		span.SetParentSpanID(missingParentID)
		span.SetName("SELECT")
		span.SetStartTimestamp(pcommon.Timestamp(1000000100 + uint64(i*10)))
		span.SetEndTimestamp(pcommon.Timestamp(1000000200 + uint64(i*10)))
	}

	return td
}
