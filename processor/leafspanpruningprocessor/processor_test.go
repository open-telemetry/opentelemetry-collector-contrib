// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package leafspanpruningprocessor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/leafspanpruningprocessor/internal/metadata"
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
	// Test: if any span has error, summary should have error
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.MinSpansToAggregate = 2

	tp, err := factory.CreateTraces(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)

	td := createTestTraceWithErrorSpan(t)

	err = tp.ConsumeTraces(t.Context(), td)
	require.NoError(t, err)

	summarySpan := findSpanByNameSuffix(td, "_aggregated")
	require.NotNil(t, summarySpan)
	assert.Equal(t, ptrace.StatusCodeError, summarySpan.Status().Code())
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
