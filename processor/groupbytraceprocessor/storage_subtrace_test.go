// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package groupbytraceprocessor

import (
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func newTestSubtraceStorage() *subtraceMemoryStorage {
	return newSubtraceMemoryStorage(nil)
}

func spanIDSet(spans []*bufferedSpan) map[pcommon.SpanID]bool {
	m := make(map[pcommon.SpanID]bool, len(spans))
	for _, bs := range spans {
		m[bs.span.SpanID()] = true
	}
	return m
}

// insertTestSpan buffers one span for the given service.
func insertTestSpan(t *testing.T, st *subtraceMemoryStorage, traceID pcommon.TraceID, spanID, parentID pcommon.SpanID, service string) {
	t.Helper()
	r := pcommon.NewResource()
	if service != "" {
		r.Attributes().PutStr("service.name", service)
	}
	rctx := newResourceContext(r)
	sp := ptrace.NewSpan()
	sp.SetTraceID(traceID)
	sp.SetSpanID(spanID)
	sp.SetParentSpanID(parentID)
	id := subtraceID{traceID: traceID, serviceID: rctx.serviceID}
	require.NoError(t, st.insertSpan(id, newSpanContext(rctx, pcommon.NewInstrumentationScope()), sp))
}

func subtraceIDFor(traceID pcommon.TraceID, service string) subtraceID {
	r := pcommon.NewResource()
	r.Attributes().PutStr("service.name", service)
	return subtraceID{traceID: traceID, serviceID: serviceIdentity(r)}
}

func TestSubtraceStorage_BuffersPerService(t *testing.T) {
	st := newTestSubtraceStorage()
	tid := makeTraceID(1)

	insertTestSpan(t, st, tid, makeSpanID(1), pcommon.NewSpanIDEmpty(), "svc-a")
	insertTestSpan(t, st, tid, makeSpanID(2), makeSpanID(1), "svc-a")
	insertTestSpan(t, st, tid, makeSpanID(3), makeSpanID(2), "svc-b")

	assert.Len(t, st.subtraceIDs(), 2)

	calls, err := st.deleteSubtrace(subtraceIDFor(tid, "svc-a"))
	require.NoError(t, err)
	require.Len(t, calls, 1)
	assert.Equal(t, map[pcommon.SpanID]bool{makeSpanID(1): true, makeSpanID(2): true}, spanIDSet(calls[0]))

	// svc-b is untouched, and its entry span is still recognizable as one even
	// though the span it was called from has now been released.
	remaining := st.subtraceIDs()
	require.Len(t, remaining, 1)
	assert.Equal(t, subtraceIDFor(tid, "svc-b"), remaining[0])
}

func TestSubtraceStorage_DeleteIsIdempotent(t *testing.T) {
	st := newTestSubtraceStorage()
	tid := makeTraceID(1)
	insertTestSpan(t, st, tid, makeSpanID(1), pcommon.NewSpanIDEmpty(), "svc-a")

	first, err := st.deleteSubtrace(subtraceIDFor(tid, "svc-a"))
	require.NoError(t, err)
	require.Len(t, first, 1)

	second, err := st.deleteSubtrace(subtraceIDFor(tid, "svc-a"))
	require.NoError(t, err)
	assert.Empty(t, second)
	assert.Empty(t, st.subtraceIDs())
}

func TestSubtraceStorage_DeleteUnknownSubtrace(t *testing.T) {
	st := newTestSubtraceStorage()
	calls, err := st.deleteSubtrace(subtraceIDFor(makeTraceID(9), "svc-a"))
	require.NoError(t, err)
	assert.Empty(t, calls)
}

// A service entered twice in a trace buffers under one ID and comes back as two
// calls.
func TestSubtraceStorage_ServiceEnteredTwice(t *testing.T) {
	st := newTestSubtraceStorage()
	tid := makeTraceID(1)
	rootA := makeSpanID(1)
	entry1, entry2 := makeSpanID(2), makeSpanID(3)
	viaC := makeSpanID(4)

	insertTestSpan(t, st, tid, rootA, pcommon.NewSpanIDEmpty(), "svc-a")
	insertTestSpan(t, st, tid, entry1, rootA, "svc-b")
	insertTestSpan(t, st, tid, viaC, entry1, "svc-c")
	insertTestSpan(t, st, tid, entry2, viaC, "svc-b")

	calls, err := st.deleteSubtrace(subtraceIDFor(tid, "svc-b"))
	require.NoError(t, err)
	require.Len(t, calls, 2, "two entries into svc-b are two calls")
	for _, call := range calls {
		assert.Len(t, call, 1)
	}
}

// A span resubmitted under a different service must not end up buffered, and so
// emitted, under both.
func TestSubtraceStorage_ResubmittedUnderDifferentService(t *testing.T) {
	st := newTestSubtraceStorage()
	tid := makeTraceID(1)
	spanID := makeSpanID(1)

	insertTestSpan(t, st, tid, spanID, pcommon.NewSpanIDEmpty(), "svc-a")
	insertTestSpan(t, st, tid, spanID, pcommon.NewSpanIDEmpty(), "svc-b")

	require.Equal(t, []subtraceID{subtraceIDFor(tid, "svc-b")}, st.subtraceIDs())

	calls, err := st.deleteSubtrace(subtraceIDFor(tid, "svc-b"))
	require.NoError(t, err)
	require.Len(t, calls, 1)
	assert.Equal(t, map[pcommon.SpanID]bool{spanID: true}, spanIDSet(calls[0]))
	assert.Empty(t, st.subtraceIDs())
}

// A span resubmitted with a different parent is placed by its new parent,
// because parentage is only read when the subtrace is released.
func TestSubtraceStorage_ResubmittedWithDifferentParent(t *testing.T) {
	st := newTestSubtraceStorage()
	tid := makeTraceID(1)
	entry1, entry2, child := makeSpanID(1), makeSpanID(2), makeSpanID(3)
	caller := makeSpanID(0x10)

	insertTestSpan(t, st, tid, caller, pcommon.NewSpanIDEmpty(), "svc-a")
	insertTestSpan(t, st, tid, entry1, caller, "svc-b")
	insertTestSpan(t, st, tid, entry2, caller, "svc-b")
	insertTestSpan(t, st, tid, child, entry1, "svc-b")
	insertTestSpan(t, st, tid, child, entry2, "svc-b") // reparented

	calls, err := st.deleteSubtrace(subtraceIDFor(tid, "svc-b"))
	require.NoError(t, err)
	require.Len(t, calls, 2)
	for _, call := range calls {
		ids := spanIDSet(call)
		if ids[entry1] {
			assert.NotContains(t, ids, child, "the child should have left its old parent's call")
		}
		if ids[entry2] {
			assert.Contains(t, ids, child, "the child should travel with its new parent")
		}
	}
}

func TestSubtraceStorage_ConcurrentInsertAndDelete(t *testing.T) {
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
			_, _ = st.deleteSubtrace(subtraceIDFor(tid, "svc"))
		})
	}
	wg.Go(func() { _ = st.subtraceIDs() })
	wg.Wait()
}

// buildBenchTrace builds a trace of spanCount spans spread evenly over
// serviceCount services, returned as one batch per service so that the shape
// matches how the spans would actually arrive. Each service has an entry span
// whose parent is the previous service's entry span, and its remaining spans
// hang beneath it as a balanced tree of the given fanout. A fanout of 1
// degenerates into a single chain, which is the deepest a trace of that size
// can be.
func buildBenchTrace(traceID pcommon.TraceID, serviceCount, spanCount, fanout int) []ptrace.Traces {
	spanID := func(i int) pcommon.SpanID {
		var id pcommon.SpanID
		id[0], id[1], id[2] = byte(i), byte(i>>8), 0xAA
		return id
	}

	batches := make([]ptrace.Traces, 0, serviceCount)
	perService := spanCount / serviceCount
	next := 0
	var previousEntry pcommon.SpanID

	for svc := range serviceCount {
		local := make([]pcommon.SpanID, perService)
		for i := range local {
			local[i] = spanID(next)
			next++
		}

		specs := make([]spanSpec, 0, perService)
		specs = append(specs, spanSpec{id: local[0], parent: previousEntry, remote: svc > 0})
		for i := 1; i < perService; i++ {
			specs = append(specs, spanSpec{id: local[i], parent: local[(i-1)/fanout]})
		}

		batches = append(batches, buildSpecTrace(traceID, fmt.Sprintf("svc-%d", svc), specs...))
		previousEntry = local[0]
	}
	return batches
}

// BenchmarkSubtraceBufferAndRelease measures everything the service strategy
// does with a trace of a given size: buffering each span as its batch arrives,
// then dividing each service's spans into calls and reassembling them on
// release. Divide ns/op by spans/op for the per-span cost.
//
// The two shapes bracket what dividing into calls has to cope with. "tree" is
// what ordinary instrumentation produces, a handler calling a handful of
// clients. "chain" is one span per level, as a long sequential pipeline would
// produce.
func BenchmarkSubtraceBufferAndRelease(b *testing.B) {
	const serviceCount = 4

	for _, shape := range []struct {
		name   string
		fanout int
	}{
		{name: "tree", fanout: 8},
		{name: "chain", fanout: 1},
	} {
		for _, spanCount := range []int{40, 400, 4000} {
			b.Run(fmt.Sprintf("%s/%s", shape.name, strconv.Itoa(spanCount)), func(b *testing.B) {
				traceID := makeTraceID(1)
				batches := buildBenchTrace(traceID, serviceCount, spanCount, shape.fanout)

				b.ReportAllocs()
				for b.Loop() {
					st := newSubtraceMemoryStorage(nil)

					var ids []subtraceID
					for _, td := range batches {
						rs := td.ResourceSpans().At(0)
						rctx := newResourceContext(rs.Resource())
						id := subtraceID{traceID: traceID, serviceID: rctx.serviceID}
						ids = append(ids, id)

						ss := rs.ScopeSpans().At(0)
						sctx := newSpanContext(rctx, ss.Scope())
						for k := 0; k < ss.Spans().Len(); k++ {
							if err := st.insertSpan(id, sctx, ss.Spans().At(k)); err != nil {
								b.Fatal(err)
							}
						}
					}

					for _, id := range ids {
						calls, err := st.deleteSubtrace(id)
						if err != nil {
							b.Fatal(err)
						}
						for _, call := range calls {
							assemble(call)
						}
					}
				}
				b.ReportMetric(float64(spanCount), "spans/op")
			})
		}
	}
}

// Services are released one at a time, usually the caller before the callee. A
// span whose parent has already been released must still be recognized as an
// entry span, or two calls into a service would collapse into one batch.
func TestSubtraceStorage_EntrySurvivesParentRelease(t *testing.T) {
	st := newTestSubtraceStorage()
	tid := makeTraceID(1)
	caller1, caller2 := makeSpanID(1), makeSpanID(2)
	entry1, entry2 := makeSpanID(3), makeSpanID(4)

	insertTestSpan(t, st, tid, caller1, pcommon.NewSpanIDEmpty(), "svc-a")
	insertTestSpan(t, st, tid, caller2, caller1, "svc-a")
	insertTestSpan(t, st, tid, entry1, caller1, "svc-b")
	insertTestSpan(t, st, tid, entry2, caller2, "svc-b")

	// svc-a goes first, taking both of svc-b's callers with it.
	_, err := st.deleteSubtrace(subtraceIDFor(tid, "svc-a"))
	require.NoError(t, err)

	calls, err := st.deleteSubtrace(subtraceIDFor(tid, "svc-b"))
	require.NoError(t, err)
	require.Len(t, calls, 2, "both entry spans must survive their callers being released")
}

// Once nothing is buffered for a trace, the record of its span IDs goes too, so
// that a long-lived trace ID doesn't accumulate them without limit.
func TestSubtraceStorage_TraceForgottenWhenEmpty(t *testing.T) {
	st := newTestSubtraceStorage()
	tid := makeTraceID(1)

	insertTestSpan(t, st, tid, makeSpanID(1), pcommon.NewSpanIDEmpty(), "svc-a")
	insertTestSpan(t, st, tid, makeSpanID(2), makeSpanID(1), "svc-b")

	_, err := st.deleteSubtrace(subtraceIDFor(tid, "svc-a"))
	require.NoError(t, err)
	st.RLock()
	require.NotNil(t, st.traces[tid], "svc-b is still buffered, so the trace stays")
	st.RUnlock()

	_, err = st.deleteSubtrace(subtraceIDFor(tid, "svc-b"))
	require.NoError(t, err)
	st.RLock()
	assert.Nil(t, st.traces[tid])
	st.RUnlock()
	assert.Empty(t, st.subtraceIDs())
}

// releaseDue hands back only the calls whose first span is old enough, and
// reports when the next one becomes due.
func TestSubtraceStorage_ReleaseDueHoldsBackLaterCalls(t *testing.T) {
	st := newTestSubtraceStorage()
	tid := makeTraceID(1)
	caller1, caller2 := makeSpanID(1), makeSpanID(2)
	early, late := makeSpanID(3), makeSpanID(4)

	insertTestSpan(t, st, tid, caller1, pcommon.NewSpanIDEmpty(), "svc-a")
	insertTestSpan(t, st, tid, caller2, caller1, "svc-a")
	insertTestSpan(t, st, tid, early, caller1, "svc-b")

	// Put a cutoff between the two arrivals.
	time.Sleep(5 * time.Millisecond)
	cutoff := time.Now()
	time.Sleep(5 * time.Millisecond)

	insertTestSpan(t, st, tid, late, caller2, "svc-b")

	due, nextArrival, err := st.releaseDue(subtraceIDFor(tid, "svc-b"), cutoff)
	require.NoError(t, err)
	require.Len(t, due, 1, "only the call that started before the cutoff is due")
	assert.Equal(t, map[pcommon.SpanID]bool{early: true}, spanIDSet(due[0]))
	assert.False(t, nextArrival.IsZero(), "the later call's first arrival should be reported")
	assert.True(t, nextArrival.After(cutoff))

	// The later call is still buffered, and comes out once its own time is up.
	due, nextArrival, err = st.releaseDue(subtraceIDFor(tid, "svc-b"), time.Now())
	require.NoError(t, err)
	require.Len(t, due, 1)
	assert.Equal(t, map[pcommon.SpanID]bool{late: true}, spanIDSet(due[0]))
	assert.True(t, nextArrival.IsZero(), "nothing left to wait for")
}

// A span arriving after its call's siblings does not reset that call's deadline.
func TestSubtraceStorage_LateSpanDoesNotExtendItsCall(t *testing.T) {
	st := newTestSubtraceStorage()
	tid := makeTraceID(1)
	entry, child := makeSpanID(1), makeSpanID(2)

	insertTestSpan(t, st, tid, entry, pcommon.NewSpanIDEmpty(), "svc-a")
	time.Sleep(5 * time.Millisecond)
	cutoff := time.Now()
	insertTestSpan(t, st, tid, child, entry, "svc-a")

	// The call's deadline runs from its first span, so it is due even though the
	// child arrived after the cutoff.
	due, nextArrival, err := st.releaseDue(subtraceIDFor(tid, "svc-a"), cutoff)
	require.NoError(t, err)
	require.Len(t, due, 1)
	assert.Equal(t, map[pcommon.SpanID]bool{entry: true, child: true}, spanIDSet(due[0]))
	assert.True(t, nextArrival.IsZero())
}

// deleteSubtrace takes everything regardless of age, which is what eviction and
// shutdown need.
func TestSubtraceStorage_DeleteSubtraceIgnoresAge(t *testing.T) {
	st := newTestSubtraceStorage()
	tid := makeTraceID(1)
	insertTestSpan(t, st, tid, makeSpanID(1), pcommon.NewSpanIDEmpty(), "svc-a")

	calls, err := st.deleteSubtrace(subtraceIDFor(tid, "svc-a"))
	require.NoError(t, err)
	require.Len(t, calls, 1)
	assert.Empty(t, st.subtraceIDs())
}

func TestSubtraceStorage_SpanIDReusedAcrossTraces(t *testing.T) {
	st := newTestSubtraceStorage()
	traceOne, traceTwo := makeTraceID(1), makeTraceID(2)
	spanID := makeSpanID(1)

	insertTestSpan(t, st, traceOne, spanID, pcommon.NewSpanIDEmpty(), "svc-a")
	insertTestSpan(t, st, traceTwo, spanID, pcommon.NewSpanIDEmpty(), "svc-a")

	require.Len(t, st.subtraceIDs(), 2, "one subtrace per trace, not one shared")

	calls, err := st.deleteSubtrace(subtraceIDFor(traceOne, "svc-a"))
	require.NoError(t, err)
	require.Len(t, calls, 1)
	require.Len(t, calls[0], 1)
	assert.Equal(t, traceOne, calls[0][0].span.TraceID())

	// The other trace is untouched.
	require.Equal(t, []subtraceID{subtraceIDFor(traceTwo, "svc-a")}, st.subtraceIDs())
	calls, err = st.deleteSubtrace(subtraceIDFor(traceTwo, "svc-a"))
	require.NoError(t, err)
	require.Len(t, calls, 1)
	require.Len(t, calls[0], 1)
	assert.Equal(t, traceTwo, calls[0][0].span.TraceID())
}

// One trace's spans must never be treated as another's entry-span evidence.
func TestSubtraceStorage_ParentInAnotherTraceIsNotFound(t *testing.T) {
	st := newTestSubtraceStorage()
	traceOne, traceTwo := makeTraceID(1), makeTraceID(2)
	caller := makeSpanID(1)
	entry, other := makeSpanID(2), makeSpanID(3)

	// traceOne holds the only span carrying the caller's ID.
	insertTestSpan(t, st, traceOne, caller, pcommon.NewSpanIDEmpty(), "svc-a")
	// traceTwo has two spans naming it as parent. Since it isn't in traceTwo,
	// they are parentless there and stay together.
	insertTestSpan(t, st, traceTwo, entry, caller, "svc-b")
	insertTestSpan(t, st, traceTwo, other, caller, "svc-b")

	calls, err := st.deleteSubtrace(subtraceIDFor(traceTwo, "svc-b"))
	require.NoError(t, err)
	require.Len(t, calls, 1, "a parent in a different trace must not make these separate calls")
	assert.Equal(t, map[pcommon.SpanID]bool{entry: true, other: true}, spanIDSet(calls[0]))
}

// A trace that always has some service buffered must not accumulate every span
// ID it has ever carried. What it retains is bounded by what it still holds.
func TestSubtraceStorage_RetainedSpanIDsStayBounded(t *testing.T) {
	const (
		rounds   = 50
		perRound = 200
	)
	st := newTestSubtraceStorage()
	tid := makeTraceID(1)

	// A service that never drains, so the trace buffer is never discarded.
	insertTestSpan(t, st, tid, makeSpanID(0xFF), pcommon.NewSpanIDEmpty(), "keeper")

	churn := subtraceIDFor(tid, "svc-a")
	for r := range rounds {
		for i := range perRound {
			insertTestSpan(t, st, tid, spanIDAt(r*perRound+i), pcommon.NewSpanIDEmpty(), "svc-a")
		}
		_, _, err := st.releaseDue(churn, time.Now().Add(time.Hour))
		require.NoError(t, err)
	}

	st.RLock()
	retained := len(st.traces[tid].spanIDs)
	live := st.traces[tid].liveSpans()
	st.RUnlock()

	assert.LessOrEqual(t, retained, 2*live+perRound,
		"released %d spans but still remembers %d IDs against %d live", rounds*perRound, retained, live)
	assert.Less(t, retained, rounds*perRound/10, "retention is tracking total throughput, not live data")
}

// Pruning must not forget a released span that something still buffered points
// at, which is what keeps two calls into a service apart.
func TestSubtraceStorage_PruningKeepsReferencedParents(t *testing.T) {
	st := newTestSubtraceStorage()
	tid := makeTraceID(1)
	caller1, caller2 := makeSpanID(1), makeSpanID(2)
	entry1, entry2 := makeSpanID(3), makeSpanID(4)

	insertTestSpan(t, st, tid, caller1, pcommon.NewSpanIDEmpty(), "svc-a")
	insertTestSpan(t, st, tid, caller2, caller1, "svc-a")
	insertTestSpan(t, st, tid, entry1, caller1, "svc-b")
	insertTestSpan(t, st, tid, entry2, caller2, "svc-b")

	// Fill svc-a with spans nothing points at, so releasing it triggers a prune.
	for i := 0; i < 50; i++ {
		insertTestSpan(t, st, tid, spanIDAt(100+i), caller1, "svc-a")
	}

	_, _, err := st.releaseDue(subtraceIDFor(tid, "svc-a"), time.Now().Add(time.Hour))
	require.NoError(t, err)

	st.RLock()
	_, keptCaller1 := st.traces[tid].spanIDs[caller1]
	_, keptCaller2 := st.traces[tid].spanIDs[caller2]
	_, keptUnreferenced := st.traces[tid].spanIDs[spanIDAt(100)]
	st.RUnlock()

	assert.True(t, keptCaller1, "a released span still pointed at must be remembered")
	assert.True(t, keptCaller2, "a released span still pointed at must be remembered")
	assert.False(t, keptUnreferenced, "a released span nothing points at should be forgotten")

	// And svc-b still splits into two calls because of it.
	calls, err := st.deleteSubtrace(subtraceIDFor(tid, "svc-b"))
	require.NoError(t, err)
	assert.Len(t, calls, 2)
}
