// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tailsamplingprocessor

import (
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// splitResourceSpansByTraceCopy is the reference for splitResourceSpansByTrace:
// it moves the spans but copies the resource and every scope for every trace.
func splitResourceSpansByTraceCopy(rss ptrace.ResourceSpans) []traceBatch {
	type builder struct {
		rs        ptrace.ResourceSpans
		destScope ptrace.ScopeSpans
		lastScope int
		count     int64
		hasRoot   bool
	}
	builders := make(map[pcommon.TraceID]*builder)
	srcScopes := rss.ScopeSpans()
	for j := 0; j < srcScopes.Len(); j++ {
		srcScope := srcScopes.At(j)
		spans := srcScope.Spans()
		for k := 0; k < spans.Len(); k++ {
			span := spans.At(k)
			id := span.TraceID()
			b, ok := builders[id]
			if !ok {
				rs := ptrace.NewResourceSpans()
				rss.Resource().CopyTo(rs.Resource())
				rs.SetSchemaUrl(rss.SchemaUrl())
				b = &builder{rs: rs, lastScope: -1}
				builders[id] = b
			}
			if b.lastScope != j {
				dest := b.rs.ScopeSpans().AppendEmpty()
				srcScope.Scope().CopyTo(dest.Scope())
				dest.SetSchemaUrl(srcScope.SchemaUrl())
				b.destScope = dest
				b.lastScope = j
			}
			destSpan := b.destScope.Spans().AppendEmpty()
			span.MoveTo(destSpan)
			b.count++
			if destSpan.ParentSpanID().IsEmpty() {
				b.hasRoot = true
			}
		}
	}
	batches := make([]traceBatch, 0, len(builders))
	for id, b := range builders {
		batches = append(batches, traceBatch{id: id, hasRoot: b.hasRoot, rss: b.rs, spanCount: b.count})
	}
	return batches
}

// resourceSpansJSON serializes everything in rs that the split must preserve:
// resource and scope attributes, dropped counts, schema URLs, and the order of
// scopes and spans.
func resourceSpansJSON(tb testing.TB, rs ptrace.ResourceSpans) string {
	td := ptrace.NewTraces()
	rs.CopyTo(td.ResourceSpans().AppendEmpty())
	out, err := (&ptrace.JSONMarshaler{}).MarshalTraces(td)
	require.NoError(tb, err)
	return string(out)
}

func batchesByID(tb testing.TB, batches []traceBatch) map[pcommon.TraceID]traceBatch {
	byID := make(map[pcommon.TraceID]traceBatch, len(batches))
	for _, b := range batches {
		_, dup := byID[b.id]
		require.False(tb, dup, "trace %s is returned twice", b.id)
		byID[b.id] = b
	}
	return byID
}

// requireSameSplit splits a copy of rss with the fast and the reference
// implementation and requires the same batches, and that no batch shares
// memory with another.
func requireSameSplit(tb testing.TB, rss ptrace.ResourceSpans) {
	want := ptrace.NewResourceSpans()
	rss.CopyTo(want)
	got := ptrace.NewResourceSpans()
	rss.CopyTo(got)

	wantBatches := batchesByID(tb, splitResourceSpansByTraceCopy(want))
	gotSplit := splitResourceSpansByTrace(got)
	gotBatches := batchesByID(tb, gotSplit)

	require.Len(tb, gotBatches, len(wantBatches))
	for id, w := range wantBatches {
		g, ok := gotBatches[id]
		require.True(tb, ok, "trace %s is missing", id)
		require.Equal(tb, w.spanCount, g.spanCount)
		require.Equal(tb, w.hasRoot, g.hasRoot)
		require.JSONEq(tb, resourceSpansJSON(tb, w.rss), resourceSpansJSON(tb, g.rss))
	}
	requireIndependentBatches(tb, gotSplit)
}

// requireIndependentBatches mutates the resource, scopes and spans of every
// batch in turn and requires that no other batch changes.
func requireIndependentBatches(tb testing.TB, batches []traceBatch) {
	snapshot := func() []string {
		out := make([]string, len(batches))
		for i, b := range batches {
			out[i] = resourceSpansJSON(tb, b.rss)
		}
		return out
	}
	for i, b := range batches {
		before := snapshot()
		b.rss.Resource().Attributes().PutStr("mutated", "resource")
		b.rss.Resource().Attributes().PutStr("service.name", "mutated")
		b.rss.Resource().SetDroppedAttributesCount(99)
		b.rss.SetSchemaUrl("mutated")
		for _, ss := range b.rss.ScopeSpans().All() {
			ss.Scope().SetName("mutated")
			ss.Scope().Attributes().PutStr("mutated", "scope")
			ss.Scope().SetDroppedAttributesCount(99)
			ss.SetSchemaUrl("mutated")
			for _, span := range ss.Spans().All() {
				span.SetName("mutated")
			}
		}
		after := snapshot()
		for k := range batches {
			if k != i {
				require.JSONEq(tb, before[k], after[k], "mutating batch %d changed batch %d", i, k)
			}
		}
	}
}

// testSpan is a span of the trace with the given number.
type testSpan struct {
	trace  uint64
	isRoot bool
}

// rootOf returns a span of the given trace that has no parent.
func rootOf(trace uint64) testSpan { return testSpan{trace: trace, isRoot: true} }

// childOf returns a span of the given trace that has a parent.
func childOf(trace uint64) testSpan { return testSpan{trace: trace} }

// splitInput describes a ResourceSpans: one slice of spans for each scope, in
// order. For example {{rootOf(1), childOf(2)}, {childOf(1)}} has two scopes;
// the first holds the root of trace 1 and a child span of trace 2, and the
// second holds a child span of trace 1.
type splitInput [][]testSpan

func (in splitInput) build() ptrace.ResourceSpans {
	rs := ptrace.NewResourceSpans()
	rs.SetSchemaUrl("https://example.com/resource")
	rs.Resource().Attributes().PutStr("service.name", "frontend")
	rs.Resource().Attributes().PutInt("n", 1)
	rs.Resource().SetDroppedAttributesCount(3)
	var spanID uint64
	for i, scope := range in {
		ss := rs.ScopeSpans().AppendEmpty()
		ss.SetSchemaUrl(fmt.Sprintf("https://example.com/scope-%d", i))
		ss.Scope().SetName(fmt.Sprintf("lib-%d", i))
		ss.Scope().SetVersion("1.0")
		ss.Scope().Attributes().PutStr("k", fmt.Sprintf("v%d", i))
		ss.Scope().SetDroppedAttributesCount(uint32(i + 1))
		for _, spec := range scope {
			spanID++
			span := ss.Spans().AppendEmpty()
			span.SetSpanID(uInt64ToSpanID(spanID))
			if !spec.isRoot {
				span.SetParentSpanID(uInt64ToSpanID(spanID + 1000))
			}
			span.SetTraceID(uInt64ToTraceID(spec.trace))
			span.SetName(fmt.Sprintf("span-%d", spanID))
		}
	}
	return rs
}

func TestSplitResourceSpansByTraceMatchesCopy(t *testing.T) {
	tests := []struct {
		name  string
		input splitInput
	}{
		{name: "no scopes", input: splitInput{}},
		{name: "empty scope", input: splitInput{{}}},
		{name: "single span", input: splitInput{{rootOf(1)}}},
		{name: "single trace single scope", input: splitInput{{rootOf(1), childOf(1), childOf(1)}}},
		{
			name: "single trace several scopes",
			input: splitInput{
				{rootOf(1), childOf(1)},
				{childOf(1)},
				{childOf(1), childOf(1)},
			},
		},
		{
			name: "single trace without root",
			input: splitInput{
				{childOf(1), childOf(1)},
				{childOf(1)},
			},
		},
		{
			name: "single trace with empty scope",
			input: splitInput{
				{rootOf(1)},
				{},
				{childOf(1)},
			},
		},
		{
			name: "scope per trace",
			input: splitInput{
				{rootOf(1), childOf(1)},
				{rootOf(2)},
				{rootOf(3), childOf(3)},
			},
		},
		{
			name: "trace in several scopes",
			input: splitInput{
				{rootOf(1)},
				{childOf(1)},
				{rootOf(2)},
				{childOf(1)},
			},
		},
		{name: "mixed scope", input: splitInput{{rootOf(1), rootOf(2), childOf(1), childOf(2), rootOf(3)}}},
		{
			name: "mixed and uniform scopes",
			input: splitInput{
				{rootOf(1), rootOf(2), childOf(1)},
				{rootOf(1)},
				{childOf(2), childOf(2)},
				{rootOf(3), rootOf(1)},
			},
		},
		{
			name: "uniform scope between mixed",
			input: splitInput{
				{rootOf(1), rootOf(2)},
				{rootOf(2), childOf(2)},
				{rootOf(1), rootOf(2)},
			},
		},
		{
			name: "empty scopes between traces",
			input: splitInput{
				{},
				{rootOf(1)},
				{},
				{rootOf(2)},
				{},
			},
		},
		{
			name: "root only in later scope",
			input: splitInput{
				{childOf(1), childOf(2)},
				{childOf(1), rootOf(2)},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requireSameSplit(t, tt.input.build())
		})
	}
}

func TestSplitResourceSpansByTraceRandomMatchesCopy(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	for range 2000 {
		numTraces := 1 + r.IntN(4)
		in := make(splitInput, r.IntN(5))
		for i := range in {
			in[i] = make([]testSpan, r.IntN(6))
			for j := range in[i] {
				in[i][j] = testSpan{trace: uint64(1 + r.IntN(numTraces)), isRoot: r.IntN(2) == 0}
			}
		}
		requireSameSplit(t, in.build())
	}
}

func TestSplitResourceSpansByTraceMovesWholeResourceSpans(t *testing.T) {
	rss := splitInput{{rootOf(1), childOf(1)}, {childOf(1)}}.build()
	batches := splitResourceSpansByTrace(rss)
	require.Len(t, batches, 1)
	require.EqualValues(t, 3, batches[0].spanCount)
	require.True(t, batches[0].hasRoot)
	require.Equal(t, 2, batches[0].rss.ScopeSpans().Len())
	// The source is consumed.
	require.Zero(t, rss.ScopeSpans().Len())
}

func TestSplitResourceSpansByTraceDoesNotShareResourceOrScope(t *testing.T) {
	for name, in := range map[string]splitInput{
		"one resource with several traces":       {{rootOf(1), rootOf(2), rootOf(3)}, {rootOf(1), rootOf(2), rootOf(3)}},
		"single-trace scopes of several traces":  {{rootOf(1), childOf(1)}, {rootOf(2), childOf(2)}, {rootOf(3)}},
		"mixed scope next to single-trace scope": {{rootOf(1), rootOf(2)}, {rootOf(1)}},
	} {
		t.Run(name, func(t *testing.T) {
			batches := splitResourceSpansByTrace(in.build())
			require.Greater(t, len(batches), 1)
			requireIndependentBatches(t, batches)
		})
	}
}
