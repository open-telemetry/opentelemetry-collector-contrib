// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package criticalpath

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/coralogixprocessor/internal/traceutil"
)

func TestApplyCriticalPathAttributes_EmptyTrace(t *testing.T) {
	result := applyCriticalPathAttributes(ptrace.NewTraces())
	assert.Equal(t, 0, result.SpanCount())
}

func TestApplyCriticalPathAttributes_SingleSpan(t *testing.T) {
	traces := newTraceBuilder().span("root", 0, 100, 1, 0).build()

	result := applyCriticalPathAttributes(traces)

	assertCriticalPathAttrs(t, spanByName(result, "root"), 100, 100)
}

func TestApplyCriticalPathAttributes_JaegerSiblingHopShape(t *testing.T) {
	traces := newTraceBuilder().
		span("root", 1, 101, 1, 0).
		span("left", 10, 50, 2, 1).
		span("right", 20, 60, 3, 1).
		build()

	result := applyCriticalPathAttributes(traces)

	assertCriticalPathAttrs(t, spanByName(result, "root"), 60, 100)
	assertNoCriticalPathAttrs(t, spanByName(result, "left"))
	assertCriticalPathAttrs(t, spanByName(result, "right"), 40, 40)
}

func TestApplyCriticalPathAttributes_NonOverlappingSiblingsReenterParent(t *testing.T) {
	traces := newTraceBuilder().
		span("root", 1, 101, 1, 0).
		span("first", 20, 40, 2, 1).
		span("second", 50, 60, 3, 1).
		build()

	result := applyCriticalPathAttributes(traces)

	assertCriticalPathAttrs(t, spanByName(result, "root"), 70, 100)
	assertCriticalPathAttrs(t, spanByName(result, "first"), 20, 20)
	assertCriticalPathAttrs(t, spanByName(result, "second"), 10, 10)
}

func TestApplyCriticalPathAttributes_OverlappingEarlierSiblingNotSelected(t *testing.T) {
	traces := newTraceBuilder().
		span("root", 0, 120, 1, 0).
		span("first", 20, 80, 2, 1).
		span("second", 50, 100, 3, 1).
		build()

	result := applyCriticalPathAttributes(traces)

	assertCriticalPathAttrs(t, spanByName(result, "root"), 70, 120)
	assertNoCriticalPathAttrs(t, spanByName(result, "first"))
	assertCriticalPathAttrs(t, spanByName(result, "second"), 50, 50)
}

func TestApplyCriticalPathAttributes_NestedDescendants(t *testing.T) {
	traces := newTraceBuilder().
		span("root", 0, 200, 1, 0).
		span("child", 20, 180, 2, 1).
		span("grandchild", 60, 160, 3, 2).
		span("leaf", 100, 150, 4, 3).
		build()

	result := applyCriticalPathAttributes(traces)

	assertCriticalPathAttrs(t, spanByName(result, "root"), 40, 200)
	assertCriticalPathAttrs(t, spanByName(result, "child"), 60, 160)
	assertCriticalPathAttrs(t, spanByName(result, "grandchild"), 50, 100)
	assertCriticalPathAttrs(t, spanByName(result, "leaf"), 50, 50)
}

func TestApplyCriticalPathAttributes_MultiRootAndMissingParent(t *testing.T) {
	traces := newTraceBuilder().
		span("root-a", 0, 100, 1, 0).
		span("child-a", 20, 60, 2, 1).
		span("root-b", 70, 130, 3, 99).
		build()

	result := applyCriticalPathAttributes(traces)

	assertCriticalPathAttrs(t, spanByName(result, "root-a"), 60, 100)
	assertCriticalPathAttrs(t, spanByName(result, "child-a"), 40, 40)
	assertCriticalPathAttrs(t, spanByName(result, "root-b"), 60, 60)
}

func TestApplyCriticalPathAttributes_InvalidIntervalsIgnored(t *testing.T) {
	traces := newTraceBuilder().
		span("root", 0, 100, 1, 0).
		span("zero", 80, 80, 2, 1).
		span("invalid", 90, 70, 3, 1).
		build()

	result := applyCriticalPathAttributes(traces)

	assertCriticalPathAttrs(t, spanByName(result, "root"), 100, 100)
	assertNoCriticalPathAttrs(t, spanByName(result, "zero"))
	assertNoCriticalPathAttrs(t, spanByName(result, "invalid"))
}

func TestApplyCriticalPathAttributesByTraceID(t *testing.T) {
	traces := newTraceBuilder().
		span("root", 0, 100, 1, 0).
		span("child", 20, 60, 2, 1).
		build()

	ApplyCriticalPathAttributesByTraceID(traceutil.GroupSpansByTraceID(traces), zap.NewNop())

	assertCriticalPathAttrs(t, spanByName(traces, "root"), 60, 100)
	assertCriticalPathAttrs(t, spanByName(traces, "child"), 40, 40)
}

func TestApplyCriticalPathAttributesByTraceID_MalformedChild(t *testing.T) {
	traces := newTraceBuilder().
		span("root", 0, 100, 1, 0).
		span("overflow", 80, 120, 2, 1).
		build()

	ApplyCriticalPathAttributesByTraceID(traceutil.GroupSpansByTraceID(traces), zap.NewNop())

	assertCriticalPathAttrs(t, spanByName(traces, "root"), 80, 100)
	assertCriticalPathAttrs(t, spanByName(traces, "overflow"), 20, 20)
}

func TestApplyCriticalPathAttributesToTree(t *testing.T) {
	traces := newTraceBuilder().
		span("root", 0, 100, 1, 0).
		span("child", 20, 60, 2, 1).
		build()
	child := spanByName(traces, "child")
	child.Attributes().PutBool(AttributeCriticalPathIsOnPath, true)
	child.Attributes().PutInt(AttributeCriticalPathExclusiveDurationNS, 1)
	child.Attributes().PutInt(AttributeCriticalPathInclusiveDurationNS, 1)

	ApplyCriticalPathAttributesToTree(
		pcommon.TraceID([16]byte{1}),
		traceutil.BuildTraceTree([]ptrace.Span{spanByName(traces, "root"), child}),
		zap.NewNop(),
	)

	assertCriticalPathAttrs(t, spanByName(traces, "root"), 60, 100)
	assertCriticalPathAttrs(t, child, 40, 40)
}

func TestApplyCriticalPathAttributesToTree_MalformedChild(t *testing.T) {
	traces := newTraceBuilder().
		span("root", 0, 100, 1, 0).
		span("overflow", 80, 120, 2, 1).
		build()

	ApplyCriticalPathAttributesToTree(
		pcommon.TraceID([16]byte{1}),
		traceutil.BuildTraceTree([]ptrace.Span{spanByName(traces, "root"), spanByName(traces, "overflow")}),
		zap.NewNop(),
	)

	assertCriticalPathAttrs(t, spanByName(traces, "root"), 80, 100)
	assertCriticalPathAttrs(t, spanByName(traces, "overflow"), 20, 20)
}

func TestApplyCriticalPathAttributes_RemovesStaleAttributes(t *testing.T) {
	traces := newTraceBuilder().
		span("root", 0, 100, 1, 0).
		span("child", 20, 60, 2, 1).
		build()
	stale := spanByName(traces, "child")
	stale.Attributes().PutBool(AttributeCriticalPathIsOnPath, true)
	stale.Attributes().PutInt(AttributeCriticalPathExclusiveDurationNS, 999)
	stale.Attributes().PutInt(AttributeCriticalPathInclusiveDurationNS, 999)

	result := applyCriticalPathAttributes(traces)

	assertCriticalPathAttrs(t, spanByName(result, "child"), 40, 40)
}

func applyCriticalPathAttributes(traces ptrace.Traces) ptrace.Traces {
	ApplyCriticalPathAttributesByTraceID(traceutil.GroupSpansByTraceID(traces), zap.NewNop())
	return traces
}

func TestBuildTraceGraph_Empty(t *testing.T) {
	graph := traceutil.BuildTraceTree(nil)

	assert.Empty(t, graph.Roots)
	assert.Empty(t, graph.Nodes)
}

func TestBuildTraceGraph_UpdatesHierarchy(t *testing.T) {
	traces := newTraceBuilder().
		span("middle", 50, 60, 1, 0).
		span("early", 10, 40, 2, 0).
		span("late-child", 20, 100, 3, 1).
		build()

	graph := traceutil.BuildTraceTree([]ptrace.Span{
		spanByName(traces, "middle"),
		spanByName(traces, "early"),
		spanByName(traces, "late-child"),
	})

	require.Len(t, graph.Roots, 2)
	middle := graph.Nodes[testSpanID(1)]
	require.NotNil(t, middle)
	require.Len(t, middle.Children, 1)
	assert.Equal(t, "late-child", middle.Children[0].Span.Name())
	assert.Equal(t, middle, middle.Children[0].Parent)
}

func TestComputeCriticalPath_EmptyGraph(t *testing.T) {
	assert.Empty(t, computeCriticalPath(traceutil.TraceTree{}))
}

func TestComputeCriticalPathSections_MatchesJaegerOrder(t *testing.T) {
	graph := traceutil.BuildTraceTree([]ptrace.Span{
		newStandaloneSpan(1, 1, 101),
		newStandaloneChildSpan(2, 10, 50, 1),
		newStandaloneChildSpan(3, 20, 60, 1),
	})

	sections := computeCriticalPath(graph)
	require.Len(t, sections, 3)
	assert.Equal(t, testSpanID(1), sections[0].spanID)
	assert.Equal(t, int64(60), sections[0].sectionStartNS)
	assert.Equal(t, int64(101), sections[0].sectionEndNS)
	assert.Equal(t, testSpanID(3), sections[1].spanID)
	assert.Equal(t, int64(20), sections[1].sectionStartNS)
	assert.Equal(t, int64(60), sections[1].sectionEndNS)
	assert.Equal(t, testSpanID(1), sections[2].spanID)
	assert.Equal(t, int64(1), sections[2].sectionStartNS)
	assert.Equal(t, int64(20), sections[2].sectionEndNS)
}

func TestComputeCriticalPathSections_IgnoresNilAndZeroLengthNodes(t *testing.T) {
	assert.Empty(t, computeCriticalPathSections(nil, nil, nil))

	node := &traceutil.TraceTreeNode{
		Span:    newStandaloneSpan(1, 10, 10),
		StartNS: 10,
		EndNS:   10,
	}
	assert.Empty(t, computeCriticalPathSections(node, nil, nil))
}

func TestComputeCriticalPathSections_SkipsParentSectionWhenChildEndsAtReturnBoundary(t *testing.T) {
	parent := &traceutil.TraceTreeNode{
		Span:    newStandaloneSpan(1, 0, 100),
		StartNS: 0,
		EndNS:   100,
	}
	child := &traceutil.TraceTreeNode{
		Span:    newStandaloneSpan(2, 10, 40),
		StartNS: 10,
		EndNS:   40,
		Parent:  parent,
	}
	parent.Children = []*traceutil.TraceTreeNode{child}
	returningStart := int64(40)

	sections := computeCriticalPathSections(parent, nil, &returningStart)
	require.Len(t, sections, 1)
	assert.Equal(t, testSpanID(1), sections[0].spanID)
	assert.Equal(t, int64(0), sections[0].sectionStartNS)
	assert.Equal(t, int64(40), sections[0].sectionEndNS)
}

func TestFindLastFinishingChild_WithReturningStartRequiresStrictlyEarlierEnd(t *testing.T) {
	parent := &traceutil.TraceTreeNode{
		Children: []*traceutil.TraceTreeNode{
			{Span: newStandaloneSpan(2, 10, 40), StartNS: 10, EndNS: 40},
			{Span: newStandaloneSpan(3, 20, 60), StartNS: 20, EndNS: 60},
		},
	}
	returningStart := int64(20)

	assert.Nil(t, findLastFinishingChild(parent, &returningStart))
}

func TestAccumulateContributions_IgnoresZeroDurationAndMissingNode(t *testing.T) {
	graph := traceutil.TraceTree{
		Nodes: map[pcommon.SpanID]*traceutil.TraceTreeNode{
			testSpanID(1): {
				Span: newStandaloneSpan(1, 0, 100),
			},
		},
	}

	contributions := accumulateContributions(graph, []criticalPathSection{
		{spanID: testSpanID(1), sectionStartNS: 10, sectionEndNS: 10},
		{spanID: testSpanID(2), sectionStartNS: 10, sectionEndNS: 20},
	})

	assert.Empty(t, contributions)
}

func TestAccumulateInclusiveContributions_NilNode(t *testing.T) {
	assert.Zero(t, accumulateInclusiveContributions(nil, map[pcommon.SpanID]contribution{}))
}

func TestSanitizeOverflowingChildren_TruncatesAndDrops(t *testing.T) {
	traces := newTraceBuilder().
		span("root", 10, 100, 1, 0).
		span("truncate-start", 0, 40, 2, 1).
		span("truncate-end", 80, 120, 3, 1).
		span("drop", 120, 130, 4, 1).
		build()
	graph := traceutil.BuildTraceTree([]ptrace.Span{
		spanByName(traces, "root"),
		spanByName(traces, "truncate-start"),
		spanByName(traces, "truncate-end"),
		spanByName(traces, "drop"),
	})

	stats := sanitizeOverflowingChildren(graph)

	assert.Equal(t, int64(10), graph.Nodes[testSpanID(2)].StartNS)
	assert.Equal(t, int64(40), graph.Nodes[testSpanID(2)].EndNS)
	assert.Equal(t, int64(80), graph.Nodes[testSpanID(3)].StartNS)
	assert.Equal(t, int64(100), graph.Nodes[testSpanID(3)].EndNS)
	_, ok := graph.Nodes[testSpanID(4)]
	assert.False(t, ok)
	assert.Equal(t, 1, stats.droppedChildren)
	assert.Equal(t, 2, stats.truncatedChildren)
}

func TestSanitizeOverflowingChildren_CoversRemainingBranches(t *testing.T) {
	traces := newTraceBuilder().
		span("root", 10, 100, 1, 0).
		span("drop-before", 0, 5, 2, 1).
		span("cover-parent", 0, 150, 3, 1).
		build()
	graph := traceutil.BuildTraceTree([]ptrace.Span{
		spanByName(traces, "root"),
		spanByName(traces, "drop-before"),
		spanByName(traces, "cover-parent"),
	})

	stats := sanitizeOverflowingChildren(graph)

	_, ok := graph.Nodes[testSpanID(2)]
	assert.False(t, ok)
	assert.Equal(t, int64(10), graph.Nodes[testSpanID(3)].StartNS)
	assert.Equal(t, int64(100), graph.Nodes[testSpanID(3)].EndNS)
	assert.Equal(t, 1, stats.droppedChildren)
	assert.Equal(t, 1, stats.truncatedChildren)
}

func TestSanitizeOverflowingChildren_RevalidatesDescendantsAfterParentTruncation(t *testing.T) {
	traces := newTraceBuilder().
		span("root", 10, 100, 1, 0).
		span("child", 0, 150, 2, 1).
		span("grandchild", 120, 140, 3, 2).
		build()
	graph := traceutil.BuildTraceTree([]ptrace.Span{
		spanByName(traces, "root"),
		spanByName(traces, "child"),
		spanByName(traces, "grandchild"),
	})

	stats := sanitizeOverflowingChildren(graph)

	require.NotNil(t, graph.Nodes[testSpanID(2)])
	assert.Equal(t, int64(10), graph.Nodes[testSpanID(2)].StartNS)
	assert.Equal(t, int64(100), graph.Nodes[testSpanID(2)].EndNS)
	_, ok := graph.Nodes[testSpanID(3)]
	assert.False(t, ok)
	assert.Equal(t, 1, stats.droppedChildren)
	assert.Equal(t, 1, stats.truncatedChildren)
}

func TestSortNodesByEndDesc_TieBreakers(t *testing.T) {
	nodes := []*traceutil.TraceTreeNode{
		{Span: newStandaloneSpan(1, 10, 20), StartNS: 10, EndNS: 20},
		{Span: newStandaloneSpan(2, 15, 20), StartNS: 15, EndNS: 20},
		{Span: newStandaloneSpan(3, 15, 20), StartNS: 15, EndNS: 20},
	}

	sortNodesByEndDesc(nodes)

	assert.Equal(t, testSpanID(3), nodes[0].Span.SpanID())
	assert.Equal(t, testSpanID(2), nodes[1].Span.SpanID())
	assert.Equal(t, testSpanID(1), nodes[2].Span.SpanID())
}

type traceBuilder struct {
	traces  ptrace.Traces
	traceID pcommon.TraceID
	spans   ptrace.SpanSlice
}

func newTraceBuilder() *traceBuilder {
	traces := ptrace.NewTraces()
	scopeSpans := traces.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty()
	return &traceBuilder{
		traces:  traces,
		traceID: pcommon.TraceID([16]byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}),
		spans:   scopeSpans.Spans(),
	}
}

func (tb *traceBuilder) span(name string, startNS, endNS int64, spanID, parentSpanID byte) *traceBuilder {
	span := tb.spans.AppendEmpty()
	span.SetTraceID(tb.traceID)
	span.SetSpanID(testSpanID(spanID))
	if parentSpanID != 0 {
		span.SetParentSpanID(testSpanID(parentSpanID))
	}
	span.SetName(name)
	span.SetStartTimestamp(pcommon.Timestamp(startNS))
	span.SetEndTimestamp(pcommon.Timestamp(endNS))
	return tb
}

func (tb *traceBuilder) build() ptrace.Traces {
	return tb.traces
}

func testSpanID(id byte) pcommon.SpanID {
	return pcommon.SpanID([8]byte{id})
}

func newStandaloneSpan(id byte, startNS, endNS int64) ptrace.Span {
	span := ptrace.NewSpan()
	span.SetSpanID(testSpanID(id))
	span.SetStartTimestamp(pcommon.Timestamp(startNS))
	span.SetEndTimestamp(pcommon.Timestamp(endNS))
	return span
}

func newStandaloneChildSpan(id byte, startNS, endNS int64, parentID byte) ptrace.Span {
	span := newStandaloneSpan(id, startNS, endNS)
	span.SetParentSpanID(testSpanID(parentID))
	return span
}

func spanByName(traces ptrace.Traces, name string) ptrace.Span {
	resourceSpans := traces.ResourceSpans()
	for i := 0; i < resourceSpans.Len(); i++ {
		scopeSpans := resourceSpans.At(i).ScopeSpans()
		for j := 0; j < scopeSpans.Len(); j++ {
			spans := scopeSpans.At(j).Spans()
			for k := 0; k < spans.Len(); k++ {
				span := spans.At(k)
				if span.Name() == name {
					return span
				}
			}
		}
	}

	panic("span not found: " + name)
}

func assertCriticalPathAttrs(t *testing.T, span ptrace.Span, exclusiveNS, inclusiveNS int64) {
	t.Helper()

	isCritical, ok := span.Attributes().Get(AttributeCriticalPathIsOnPath)
	require.True(t, ok)
	assert.True(t, isCritical.Bool())

	exclusive, ok := span.Attributes().Get(AttributeCriticalPathExclusiveDurationNS)
	require.True(t, ok)
	assert.Equal(t, exclusiveNS, exclusive.Int())

	inclusive, ok := span.Attributes().Get(AttributeCriticalPathInclusiveDurationNS)
	require.True(t, ok)
	assert.Equal(t, inclusiveNS, inclusive.Int())
}

func assertNoCriticalPathAttrs(t *testing.T, span ptrace.Span) {
	t.Helper()

	_, ok := span.Attributes().Get(AttributeCriticalPathIsOnPath)
	assert.False(t, ok)
	_, ok = span.Attributes().Get(AttributeCriticalPathExclusiveDurationNS)
	assert.False(t, ok)
	_, ok = span.Attributes().Get(AttributeCriticalPathInclusiveDurationNS)
	assert.False(t, ok)
}
