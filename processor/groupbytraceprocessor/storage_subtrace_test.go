// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package groupbytraceprocessor

import (
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
	assert.NoError(t, st.insertSpan(traceID, r, sc, sp))
}

func TestSubtraceStorage_SingleSpan_LocalRoot(t *testing.T) {
	st := newTestSubtraceStorage()
	tid := makeTraceID(1)
	sid := makeSpanID(1)
	insertTestSpan(t, st, tid, sid, pcommon.NewSpanIDEmpty(), "svc-a")

	roots := st.localRoots(tid)
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
	require.NoError(t, st.insertSpan(tid, r, sc, sp))
	insertTestSpan(t, st, tid, childB, rootB, "svc-b")

	roots := st.localRoots(tid)
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

func spanIDSet(spans []bufferedSpan) map[pcommon.SpanID]bool {
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

	require.NoError(t, st.deleteTrace(tid))

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

	roots := st.localRoots(tid)
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
			_ = st.localRoots(tid)
		})
	}
	wg.Wait()
}
