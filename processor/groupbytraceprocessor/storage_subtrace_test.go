// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package groupbytraceprocessor

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func newTestSubtraceStorage() *subtraceMemoryStorage {
	return newSubtraceMemoryStorage(nil)
}

// allLocalRoots classifies every span buffered for a trace, which is what these
// tests want to assert about. The processor only ever classifies the spans that
// just arrived, so it passes a narrower candidate list.
func allLocalRoots(st *subtraceMemoryStorage, traceID pcommon.TraceID) []pcommon.SpanID {
	st.RLock()
	idx, ok := st.traces[traceID]
	if !ok {
		// The concurrency test races this against the first insert, so the trace
		// may not exist yet.
		st.RUnlock()
		return nil
	}
	candidates := make([]pcommon.SpanID, 0, idx.len())
	for spanID := range idx.spans {
		candidates = append(candidates, spanID)
	}
	st.RUnlock()
	return st.localRoots(traceID, candidates)
}

func insertTestSpan(t *testing.T, st *subtraceMemoryStorage, traceID pcommon.TraceID, spanID, parentID pcommon.SpanID, svcName string) {
	t.Helper()
	r := pcommon.NewResource()
	if svcName != "" {
		r.Attributes().PutStr("service.name", svcName)
	}
	sc := pcommon.NewInstrumentationScope()
	sp := ptrace.NewSpan()
	sp.SetTraceID(traceID)
	sp.SetSpanID(spanID)
	sp.SetParentSpanID(parentID)
	assert.NoError(t, st.insertSpan(traceID, newSpanContext(newResourceContext(r), sc), sp))
}

func TestSubtraceStorage_SingleSpan_LocalRoot(t *testing.T) {
	st := newTestSubtraceStorage()
	tid := makeTraceID(1)
	sid := makeSpanID(1)
	insertTestSpan(t, st, tid, sid, pcommon.NewSpanIDEmpty(), "svc-a")

	roots := allLocalRoots(st, tid)
	require.Len(t, roots, 1)
	assert.Equal(t, sid, roots[0])
}

func TestSubtraceStorage_MultiService_DisjointSubtraces(t *testing.T) {
	st := newTestSubtraceStorage()
	tid := makeTraceID(1)

	rootA := makeSpanID(1)
	childA := makeSpanID(2)
	rootB := makeSpanID(3)
	childB := makeSpanID(4)

	// service A
	insertTestSpan(t, st, tid, rootA, pcommon.NewSpanIDEmpty(), "svc-a")
	insertTestSpan(t, st, tid, childA, rootA, "svc-a")
	// service B (remote from A)
	r := pcommon.NewResource()
	r.Attributes().PutStr("service.name", "svc-b")
	sc := pcommon.NewInstrumentationScope()
	sp := ptrace.NewSpan()
	sp.SetTraceID(tid)
	sp.SetSpanID(rootB)
	sp.SetParentSpanID(rootA)
	sp.SetFlags(spanFlagsContextHasIsRemoteMask | spanFlagsContextIsRemoteMask)
	require.NoError(t, st.insertSpan(tid, newSpanContext(newResourceContext(r), sc), sp))
	insertTestSpan(t, st, tid, childB, rootB, "svc-b")

	roots := allLocalRoots(st, tid)
	assert.Len(t, roots, 2)

	membersA, err := st.getSubtrace(tid, rootA)
	require.NoError(t, err)
	membersB, err := st.getSubtrace(tid, rootB)
	require.NoError(t, err)

	idsA := spanIDSet(membersA)
	idsB := spanIDSet(membersB)
	assert.Contains(t, idsA, rootA)
	assert.Contains(t, idsA, childA)
	assert.NotContains(t, idsA, rootB)
	assert.NotContains(t, idsA, childB)

	assert.Contains(t, idsB, rootB)
	assert.Contains(t, idsB, childB)
	assert.NotContains(t, idsB, rootA)
	assert.NotContains(t, idsB, childA)
}

func spanIDSet(spans []*bufferedSpan) map[pcommon.SpanID]bool {
	m := make(map[pcommon.SpanID]bool, len(spans))
	for _, bs := range spans {
		m[bs.span.SpanID()] = true
	}
	return m
}

func TestSubtraceStorage_DeleteSubtrace_LeavesRemainder(t *testing.T) {
	st := newTestSubtraceStorage()
	tid := makeTraceID(1)
	rootA := makeSpanID(1)
	childA := makeSpanID(2)
	rootB := makeSpanID(3)

	insertTestSpan(t, st, tid, rootA, pcommon.NewSpanIDEmpty(), "svc-a")
	insertTestSpan(t, st, tid, childA, rootA, "svc-a")
	insertTestSpan(t, st, tid, rootB, pcommon.NewSpanIDEmpty(), "svc-b")

	deleted, err := st.deleteSubtrace(tid, rootA)
	require.NoError(t, err)
	assert.Len(t, deleted, 2)

	remainder, err := st.getRemainder(tid)
	require.NoError(t, err)
	require.Len(t, remainder, 1)
	assert.Equal(t, rootB, remainder[0].span.SpanID())
}

func TestSubtraceStorage_GetRemainder(t *testing.T) {
	st := newTestSubtraceStorage()
	tid := makeTraceID(1)
	insertTestSpan(t, st, tid, makeSpanID(1), pcommon.NewSpanIDEmpty(), "svc-a")
	insertTestSpan(t, st, tid, makeSpanID(2), pcommon.NewSpanIDEmpty(), "svc-b")

	// Delete one subtrace.
	_, err := st.deleteSubtrace(tid, makeSpanID(1))
	require.NoError(t, err)

	remainder, err := st.getRemainder(tid)
	require.NoError(t, err)
	assert.Len(t, remainder, 1)
}

func TestSubtraceStorage_DeleteTrace(t *testing.T) {
	st := newTestSubtraceStorage()
	tid := makeTraceID(1)
	insertTestSpan(t, st, tid, makeSpanID(1), pcommon.NewSpanIDEmpty(), "svc-a")
	insertTestSpan(t, st, tid, makeSpanID(2), makeSpanID(1), "svc-a")

	removed, err := st.deleteTrace(tid)
	require.NoError(t, err)
	assert.Equal(t, map[pcommon.SpanID]bool{makeSpanID(1): true, makeSpanID(2): true}, spanIDSet(removed))

	remainder, err := st.getRemainder(tid)
	require.NoError(t, err)
	assert.Empty(t, remainder)
	assert.Empty(t, st.traceIDs())
}

// TestSubtraceStorage_ABCBCallChain_FourSubtraces verifies that the call chain
// A --> B --> C --> B (service B is called twice) produces exactly four local roots and
// therefore four distinct subtraces.
func TestSubtraceStorage_ABCBCallChain_FourSubtraces(t *testing.T) {
	st := newTestSubtraceStorage()
	tid := makeTraceID(1)

	rootA := makeSpanID(0x01)
	rootB1 := makeSpanID(0x02) // B's first entry, called by A
	rootC := makeSpanID(0x03)  // C's entry, called by B
	rootB2 := makeSpanID(0x04) // B's second entry, called by C

	insertTestSpan(t, st, tid, rootA, pcommon.NewSpanIDEmpty(), "svc-a")
	insertTestSpan(t, st, tid, rootB1, rootA, "svc-b")
	insertTestSpan(t, st, tid, rootC, rootB1, "svc-c")
	insertTestSpan(t, st, tid, rootB2, rootC, "svc-b")

	roots := allLocalRoots(st, tid)
	require.Len(t, roots, 4)

	rootSet := make(map[pcommon.SpanID]bool, len(roots))
	for _, r := range roots {
		rootSet[r] = true
	}
	assert.True(t, rootSet[rootA], "svc-a root should be a local root")
	assert.True(t, rootSet[rootB1], "svc-b first entry should be a local root")
	assert.True(t, rootSet[rootC], "svc-c root should be a local root")
	assert.True(t, rootSet[rootB2], "svc-b second entry should be a local root")
}

func TestSubtraceStorage_ConcurrentInsertAndLocalRoots(t *testing.T) {
	st := newTestSubtraceStorage()
	tid := makeTraceID(1)

	var wg sync.WaitGroup
	for i := byte(1); i <= 20; i++ {
		wg.Go(func() {
			insertTestSpan(t, st, tid, makeSpanID(i), pcommon.NewSpanIDEmpty(), "svc")
		})
	}
	for range 5 {
		wg.Go(func() {
			_ = allLocalRoots(st, tid)
		})
	}
	wg.Wait()
}

// TestSubtraceStorage_DeleteSubtrace_SkipsDemotedRoot verifies that a span which
// was a local root when its timer was scheduled, but stopped being one once its
// parent arrived, is left in place for the parent's subtrace to release.
func TestSubtraceStorage_DeleteSubtrace_SkipsDemotedRoot(t *testing.T) {
	st := newTestSubtraceStorage()
	tid := makeTraceID(1)
	rootID := makeSpanID(1)
	childID := makeSpanID(2)

	// The child arrives first, so its parent is missing and it looks like a root.
	insertTestSpan(t, st, tid, childID, rootID, "svc-a")
	require.Equal(t, []pcommon.SpanID{childID}, allLocalRoots(st, tid))

	// The parent arrives, demoting the child.
	insertTestSpan(t, st, tid, rootID, pcommon.NewSpanIDEmpty(), "svc-a")
	require.Equal(t, []pcommon.SpanID{rootID}, allLocalRoots(st, tid))

	demoted, err := st.deleteSubtrace(tid, childID)
	require.NoError(t, err)
	assert.Empty(t, demoted, "a demoted root must not claim any spans")

	released, err := st.deleteSubtrace(tid, rootID)
	require.NoError(t, err)
	assert.Equal(t, map[pcommon.SpanID]bool{rootID: true, childID: true}, spanIDSet(released))
	assert.Empty(t, st.traceIDs())
}

// buildBenchTrace builds a trace of spanCount spans spread evenly over
// serviceCount services, returned as one batch per service so that the shape
// matches how the spans would actually arrive. Each service has a local root
// whose parent is the previous service's root, flagged remote, and its remaining
// spans hang beneath that root as a balanced tree of the given fanout. A fanout
// of 1 degenerates into a single chain, which is the deepest a trace of that
// size can be.
func buildBenchTrace(traceID pcommon.TraceID, serviceCount, spanCount, fanout int) []ptrace.Traces {
	spanID := func(i int) pcommon.SpanID {
		var id pcommon.SpanID
		id[0], id[1], id[2] = byte(i), byte(i>>8), 0xAA
		return id
	}

	batches := make([]ptrace.Traces, 0, serviceCount)
	perService := spanCount / serviceCount
	next := 0
	var previousRoot pcommon.SpanID

	for svc := range serviceCount {
		local := make([]pcommon.SpanID, perService)
		for i := range local {
			local[i] = spanID(next)
			next++
		}

		specs := make([]spanSpec, 0, perService)
		// The first service's root is the global root; the rest enter from the
		// service before them.
		specs = append(specs, spanSpec{id: local[0], parent: previousRoot, remote: svc > 0})
		for i := 1; i < perService; i++ {
			specs = append(specs, spanSpec{id: local[i], parent: local[(i-1)/fanout]})
		}

		batches = append(batches, buildSpecTrace(traceID, fmt.Sprintf("svc-%d", svc), specs...))
		previousRoot = local[0]
	}
	return batches
}

// BenchmarkSubtraceIndexAndRelease measures everything the service strategy does
// with a trace of a given size: indexing each span as its batch arrives,
// classifying the spans that arrived with it, then collecting and reassembling
// each service's subtrace on release. Divide ns/op by spans/op for the per-span
// cost.
//
// The two shapes bracket what the collection walk has to cope with. "tree" is
// what ordinary instrumentation produces, a handler calling a handful of
// clients. "chain" is one span per level, as a long sequential pipeline would
// produce, and is the worst case for anything that has to traverse ancestry.
func BenchmarkSubtraceIndexAndRelease(b *testing.B) {
	const serviceCount = 4

	for _, shape := range []struct {
		name   string
		fanout int
	}{
		{name: "tree", fanout: 8},
		{name: "chain", fanout: 1},
	} {
		for _, spanCount := range []int{40, 400, 4000} {
			b.Run(fmt.Sprintf("%s/%d", shape.name, spanCount), func(b *testing.B) {
				traceID := makeTraceID(1)
				batches := buildBenchTrace(traceID, serviceCount, spanCount, shape.fanout)

				b.ReportAllocs()
				for b.Loop() {
					st := newSubtraceMemoryStorage(nil)

					var roots []pcommon.SpanID
					for _, td := range batches {
						var arrived []pcommon.SpanID
						for i := 0; i < td.ResourceSpans().Len(); i++ {
							rs := td.ResourceSpans().At(i)
							rctx := newResourceContext(rs.Resource())
							for j := 0; j < rs.ScopeSpans().Len(); j++ {
								ss := rs.ScopeSpans().At(j)
								sctx := newSpanContext(rctx, ss.Scope())
								for k := 0; k < ss.Spans().Len(); k++ {
									span := ss.Spans().At(k)
									if err := st.insertSpan(traceID, sctx, span); err != nil {
										b.Fatal(err)
									}
									arrived = append(arrived, span.SpanID())
								}
							}
						}
						roots = append(roots, st.localRoots(traceID, arrived)...)
					}

					for _, root := range roots {
						members, err := st.deleteSubtrace(traceID, root)
						if err != nil {
							b.Fatal(err)
						}
						assemble(members)
					}
				}
				b.ReportMetric(float64(spanCount), "spans/op")
			})
		}
	}
}
