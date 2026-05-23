// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package spanpruningprocessor

import (
	"fmt"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// testTraceID is a fixed trace ID used across all test trace generators.
var testTraceID = pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})

// makeSpanID converts a uint64 to a SpanID.
func makeSpanID(id uint64) pcommon.SpanID {
	return pcommon.SpanID([8]byte{
		byte(id >> 56), byte(id >> 48), byte(id >> 40), byte(id >> 32),
		byte(id >> 24), byte(id >> 16), byte(id >> 8), byte(id),
	})
}

// generateTestTrace creates a flat test trace: root -> parents -> leaves.
func generateTestTrace(numSpans, leafSpansPerParent int) ptrace.Traces {
	td := ptrace.NewTraces()
	ss := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty()

	// Root span
	root := ss.Spans().AppendEmpty()
	root.SetTraceID(testTraceID)
	root.SetSpanID(makeSpanID(1))
	root.SetName("root")
	root.SetKind(ptrace.SpanKindServer)
	root.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	root.SetEndTimestamp(pcommon.NewTimestampFromTime(time.Now().Add(time.Second)))
	root.Status().SetCode(ptrace.StatusCodeOk)

	spanID := uint64(2)
	numParents := max((numSpans-1)/leafSpansPerParent, 1)

	// Parent spans
	parentIDs := make([]pcommon.SpanID, 0, numParents)
	for i := 0; i < numParents && spanID < uint64(numSpans); i++ {
		span := ss.Spans().AppendEmpty()
		span.SetTraceID(testTraceID)
		id := makeSpanID(spanID)
		span.SetSpanID(id)
		span.SetParentSpanID(root.SpanID())
		span.SetName(fmt.Sprintf("parent-%d", i))
		span.SetKind(ptrace.SpanKindInternal)
		span.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		span.SetEndTimestamp(pcommon.NewTimestampFromTime(time.Now().Add(100 * time.Millisecond)))
		span.Status().SetCode(ptrace.StatusCodeOk)
		parentIDs = append(parentIDs, id)
		spanID++
	}

	// Leaf spans
	for i := 0; spanID < uint64(numSpans+1); i++ {
		span := ss.Spans().AppendEmpty()
		span.SetTraceID(testTraceID)
		span.SetSpanID(makeSpanID(spanID))
		span.SetParentSpanID(parentIDs[i%len(parentIDs)])
		span.SetName("leaf-operation")
		span.SetKind(ptrace.SpanKindClient)
		span.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		span.SetEndTimestamp(pcommon.NewTimestampFromTime(time.Now().Add(10 * time.Millisecond)))
		span.Status().SetCode(ptrace.StatusCodeOk)
		span.Attributes().PutStr("http.method", "GET")
		span.Attributes().PutStr("http.url", "/api/data")
		spanID++
	}

	return td
}

// generateSparseTrace creates a trace where only a small fraction aggregates.
func generateSparseTrace(numSpans, minSpans int) ptrace.Traces {
	td := ptrace.NewTraces()
	ss := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty()

	// Root span
	root := ss.Spans().AppendEmpty()
	root.SetTraceID(testTraceID)
	root.SetSpanID(makeSpanID(1))
	root.SetName("root")
	root.SetKind(ptrace.SpanKindServer)
	root.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	root.SetEndTimestamp(pcommon.NewTimestampFromTime(time.Now().Add(time.Second)))
	root.Status().SetCode(ptrace.StatusCodeOk)

	spanID := uint64(2)

	// Handler spans (unique names, won't aggregate)
	numHandlers := numSpans / 10
	handlerIDs := make([]pcommon.SpanID, 0, numHandlers)
	for i := range numHandlers {
		span := ss.Spans().AppendEmpty()
		span.SetTraceID(testTraceID)
		id := makeSpanID(spanID)
		span.SetSpanID(id)
		span.SetParentSpanID(root.SpanID())
		span.SetName(fmt.Sprintf("handler-%d", i))
		span.SetKind(ptrace.SpanKindInternal)
		span.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		span.SetEndTimestamp(pcommon.NewTimestampFromTime(time.Now().Add(100 * time.Millisecond)))
		span.Status().SetCode(ptrace.StatusCodeOk)
		handlerIDs = append(handlerIDs, id)
		spanID++
	}

	// Unique leaf spans (won't aggregate)
	numRepeated := minSpans * 2
	numUnique := numSpans - numHandlers - 1 - numRepeated
	for i := 0; i < numUnique && spanID < uint64(numSpans+1); i++ {
		span := ss.Spans().AppendEmpty()
		span.SetTraceID(testTraceID)
		span.SetSpanID(makeSpanID(spanID))
		span.SetParentSpanID(handlerIDs[i%len(handlerIDs)])
		span.SetName(fmt.Sprintf("unique-op-%d", i))
		span.SetKind(ptrace.SpanKindClient)
		span.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		span.SetEndTimestamp(pcommon.NewTimestampFromTime(time.Now().Add(10 * time.Millisecond)))
		span.Status().SetCode(ptrace.StatusCodeOk)
		span.Attributes().PutStr("db.system", "postgresql")
		spanID++
	}

	// Repeated leaf spans (will aggregate)
	if len(handlerIDs) > 0 {
		targetHandler := handlerIDs[0]
		for i := 0; i < numRepeated && spanID < uint64(numSpans+1); i++ {
			span := ss.Spans().AppendEmpty()
			span.SetTraceID(testTraceID)
			span.SetSpanID(makeSpanID(spanID))
			span.SetParentSpanID(targetHandler)
			span.SetName("SELECT")
			span.SetKind(ptrace.SpanKindClient)
			span.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now()))
			span.SetEndTimestamp(pcommon.NewTimestampFromTime(time.Now().Add(10 * time.Millisecond)))
			span.Status().SetCode(ptrace.StatusCodeOk)
			span.Attributes().PutStr("db.system", "postgresql")
			span.Attributes().PutStr("db.operation", "select")
			spanID++
		}
	}

	return td
}

// generateDeepTrace creates a trace with specified depth and branching factor.
// Each level has spans with the same name, enabling parent aggregation.
func generateDeepTrace(depth, branchingFactor, leafsPerBranch, maxSpans int) ptrace.Traces {
	td := ptrace.NewTraces()
	ss := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty()

	spanID := uint64(1)
	totalSpans := 0

	// Root span
	root := ss.Spans().AppendEmpty()
	root.SetTraceID(testTraceID)
	root.SetSpanID(makeSpanID(spanID))
	root.SetName("root")
	root.SetKind(ptrace.SpanKindServer)
	root.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	root.SetEndTimestamp(pcommon.NewTimestampFromTime(time.Now().Add(time.Second)))
	root.Status().SetCode(ptrace.StatusCodeOk)
	spanID++
	totalSpans++

	// Build tree level by level
	currentLevel := []pcommon.SpanID{root.SpanID()}

	for d := 1; d < depth && totalSpans < maxSpans; d++ {
		nextLevel := make([]pcommon.SpanID, 0, len(currentLevel)*branchingFactor)
		levelName := fmt.Sprintf("level-%d", d)

		for _, parentID := range currentLevel {
			for b := 0; b < branchingFactor && totalSpans < maxSpans; b++ {
				span := ss.Spans().AppendEmpty()
				span.SetTraceID(testTraceID)
				id := makeSpanID(spanID)
				span.SetSpanID(id)
				span.SetParentSpanID(parentID)
				span.SetName(levelName)
				span.SetKind(ptrace.SpanKindInternal)
				span.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now()))
				span.SetEndTimestamp(pcommon.NewTimestampFromTime(time.Now().Add(100 * time.Millisecond)))
				span.Status().SetCode(ptrace.StatusCodeOk)
				nextLevel = append(nextLevel, id)
				spanID++
				totalSpans++
			}
		}
		currentLevel = nextLevel
	}

	// Add leaf spans
	for _, parentID := range currentLevel {
		for l := 0; l < leafsPerBranch && totalSpans < maxSpans; l++ {
			span := ss.Spans().AppendEmpty()
			span.SetTraceID(testTraceID)
			span.SetSpanID(makeSpanID(spanID))
			span.SetParentSpanID(parentID)
			span.SetName("db-query")
			span.SetKind(ptrace.SpanKindClient)
			span.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now()))
			span.SetEndTimestamp(pcommon.NewTimestampFromTime(time.Now().Add(10 * time.Millisecond)))
			span.Status().SetCode(ptrace.StatusCodeOk)
			span.Attributes().PutStr("db.system", "postgresql")
			span.Attributes().PutStr("db.operation", "select")
			spanID++
			totalSpans++
		}
	}

	return td
}

// generateTestSpans extracts spanInfo slice from a trace for tree benchmarks.
func generateTestSpans(numSpans, leafSpansPerParent int) []spanInfo {
	td := generateTestTrace(numSpans, leafSpansPerParent)
	spans := make([]spanInfo, 0, numSpans)

	for i := 0; i < td.ResourceSpans().Len(); i++ {
		for j := 0; j < td.ResourceSpans().At(i).ScopeSpans().Len(); j++ {
			ss := td.ResourceSpans().At(i).ScopeSpans().At(j)
			for k := 0; k < ss.Spans().Len(); k++ {
				spans = append(spans, spanInfo{
					span:       ss.Spans().At(k),
					scopeSpans: ss,
				})
			}
		}
	}

	return spans
}
