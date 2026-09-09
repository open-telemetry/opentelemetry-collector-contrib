// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package groupbytraceprocessor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// makeSpanID returns a SpanID whose first byte is the given value.
func makeSpanID(b byte) pcommon.SpanID {
	var id pcommon.SpanID
	id[0] = b
	return id
}

// makeTraceID returns a TraceID whose first byte is the given value.
func makeTraceID(b byte) pcommon.TraceID {
	var id pcommon.TraceID
	id[0] = b
	return id
}

// newBS creates a minimal bufferedSpan with the given span ID, parent span ID,
// flags, and service name.
func newBS(spanID, parentID pcommon.SpanID, flags uint32, serviceName string) *bufferedSpan {
	r := pcommon.NewResource()
	if serviceName != "" {
		r.Attributes().PutStr("service.name", serviceName)
	}
	s := ptrace.NewSpan()
	s.SetSpanID(spanID)
	s.SetParentSpanID(parentID)
	s.SetFlags(flags)
	return newBufferedSpan(newSpanContext(newResourceContext(r), pcommon.NewInstrumentationScope()), s)
}

// empty parent --> always local root
func TestIsLocalRoot_EmptyParent(t *testing.T) {
	bs := newBS(makeSpanID(1), pcommon.NewSpanIDEmpty(), 0, "svc-a")
	assert.True(t, isLocalRoot(bs, newTraceIndex()))
}

// HAS_IS_REMOTE=1, IS_REMOTE=1 --> local root
func TestIsLocalRoot_RemoteFlagSet(t *testing.T) {
	bs := newBS(makeSpanID(2), makeSpanID(99), spanFlagsContextHasIsRemoteMask|spanFlagsContextIsRemoteMask, "svc-a")
	assert.True(t, isLocalRoot(bs, newTraceIndex()))
}

// HAS_IS_REMOTE=1, IS_REMOTE=0 --> safe default: local root
func TestIsLocalRoot_LocalFlagClear(t *testing.T) {
	bs := newBS(makeSpanID(3), makeSpanID(99), spanFlagsContextHasIsRemoteMask, "svc-a")
	assert.True(t, isLocalRoot(bs, newTraceIndex()))
}

// parent not in index --> safe default: local root
func TestIsLocalRoot_ParentNotInIndex(t *testing.T) {
	bs := newBS(makeSpanID(4), makeSpanID(99), 0, "svc-a")
	assert.True(t, isLocalRoot(bs, newTraceIndex()))
}

// parent in index, same service.name only --> NOT local root
func TestIsLocalRoot_SameServiceNameOnly(t *testing.T) {
	parentID := makeSpanID(10)
	child := newBS(makeSpanID(5), parentID, 0, "svc-a")
	parent := newBS(parentID, pcommon.NewSpanIDEmpty(), 0, "svc-a")
	index := buildIndex(parent)
	assert.False(t, isLocalRoot(child, index))
}

// parent in index, same name+instance --> NOT local root
func TestIsLocalRoot_SameServiceNameAndInstance(t *testing.T) {
	parentID := makeSpanID(10)
	r := pcommon.NewResource()
	r.Attributes().PutStr("service.name", "svc-a")
	r.Attributes().PutStr("service.instance.id", "inst-1")
	s := ptrace.NewSpan()
	s.SetSpanID(makeSpanID(6))
	s.SetParentSpanID(parentID)
	child := newBufferedSpan(newSpanContext(newResourceContext(r), pcommon.NewInstrumentationScope()), s)

	pr := pcommon.NewResource()
	pr.Attributes().PutStr("service.name", "svc-a")
	pr.Attributes().PutStr("service.instance.id", "inst-1")
	ps := ptrace.NewSpan()
	ps.SetSpanID(parentID)
	parentBS := newBufferedSpan(newSpanContext(newResourceContext(pr), pcommon.NewInstrumentationScope()), ps)

	index := buildIndex(parentBS)
	assert.False(t, isLocalRoot(child, index))
}

// parent in index, different instance, same name --> local root
func TestIsLocalRoot_DifferentInstance(t *testing.T) {
	parentID := makeSpanID(10)
	r := pcommon.NewResource()
	r.Attributes().PutStr("service.name", "svc-a")
	r.Attributes().PutStr("service.instance.id", "inst-2")
	s := ptrace.NewSpan()
	s.SetSpanID(makeSpanID(7))
	s.SetParentSpanID(parentID)
	child := newBufferedSpan(newSpanContext(newResourceContext(r), pcommon.NewInstrumentationScope()), s)

	pr := pcommon.NewResource()
	pr.Attributes().PutStr("service.name", "svc-a")
	pr.Attributes().PutStr("service.instance.id", "inst-1")
	ps := ptrace.NewSpan()
	ps.SetSpanID(parentID)
	parentBS := newBufferedSpan(newSpanContext(newResourceContext(pr), pcommon.NewInstrumentationScope()), ps)

	index := buildIndex(parentBS)
	assert.True(t, isLocalRoot(child, index))
}

// parent in index, different service name --> local root
func TestIsLocalRoot_DifferentServiceName(t *testing.T) {
	parentID := makeSpanID(10)
	child := newBS(makeSpanID(8), parentID, 0, "svc-b")
	parent := newBS(parentID, pcommon.NewSpanIDEmpty(), 0, "svc-a")
	index := buildIndex(parent)
	assert.True(t, isLocalRoot(child, index))
}

// no service.name, same attribute map --> NOT local root (same hash)
func TestIsLocalRoot_NoServiceNameSameAttrs(t *testing.T) {
	parentID := makeSpanID(10)
	child := newBS(makeSpanID(9), parentID, 0, "")
	parent := newBS(parentID, pcommon.NewSpanIDEmpty(), 0, "")
	index := buildIndex(parent)
	assert.False(t, isLocalRoot(child, index))
}

func buildIndex(spans ...*bufferedSpan) *traceIndex {
	idx := newTraceIndex()
	for _, bs := range spans {
		idx.insert(bs)
	}
	return idx
}

// memberIDs returns the span IDs of the subtrace rooted at rootID.
func memberIDs(rootID pcommon.SpanID, idx *traceIndex) map[pcommon.SpanID]bool {
	return spanIDSet(subtraceMembers(rootID, idx))
}

// Membership extends to a direct child within the same service.
func TestSubtraceMembers_DirectParent(t *testing.T) {
	rootID := makeSpanID(1)
	childID := makeSpanID(2)
	root := newBS(rootID, pcommon.NewSpanIDEmpty(), 0, "svc-a")
	child := newBS(childID, rootID, 0, "svc-a")
	idx := buildIndex(root, child)
	assert.Equal(t, map[pcommon.SpanID]bool{rootID: true, childID: true}, memberIDs(rootID, idx))
}

// Membership follows a multi-hop chain within the same service.
func TestSubtraceMembers_MultiHop(t *testing.T) {
	rootID := makeSpanID(1)
	midID := makeSpanID(2)
	leafID := makeSpanID(3)
	root := newBS(rootID, pcommon.NewSpanIDEmpty(), 0, "svc-a")
	mid := newBS(midID, rootID, 0, "svc-a")
	leaf := newBS(leafID, midID, 0, "svc-a")
	idx := buildIndex(root, mid, leaf)
	assert.Equal(t, map[pcommon.SpanID]bool{rootID: true, midID: true, leafID: true}, memberIDs(rootID, idx))
}

// Membership branches out to every child, not just the first.
func TestSubtraceMembers_Siblings(t *testing.T) {
	rootID := makeSpanID(1)
	leftID := makeSpanID(2)
	rightID := makeSpanID(3)
	root := newBS(rootID, pcommon.NewSpanIDEmpty(), 0, "svc-a")
	left := newBS(leftID, rootID, 0, "svc-a")
	right := newBS(rightID, rootID, 0, "svc-a")
	idx := buildIndex(root, left, right)
	assert.Equal(t, map[pcommon.SpanID]bool{rootID: true, leftID: true, rightID: true}, memberIDs(rootID, idx))
}

// Membership stops at a different local root, and that root's own subtrace does
// not reach back up past its remote parent.
func TestSubtraceMembers_StopsAtDifferentLocalRoot(t *testing.T) {
	rootA := makeSpanID(1)
	rootB := makeSpanID(2)
	childB := makeSpanID(3)
	a := newBS(rootA, pcommon.NewSpanIDEmpty(), 0, "svc-a")
	b := newBS(rootB, rootA, spanFlagsContextHasIsRemoteMask|spanFlagsContextIsRemoteMask, "svc-b")
	c := newBS(childB, rootB, 0, "svc-b")
	idx := buildIndex(a, b, c)

	assert.Equal(t, map[pcommon.SpanID]bool{rootA: true}, memberIDs(rootA, idx))
	assert.Equal(t, map[pcommon.SpanID]bool{rootB: true, childB: true}, memberIDs(rootB, idx))
}

// A span whose parent never arrived is a local root, so it forms its own
// subtrace rather than joining the missing parent's.
func TestSubtraceMembers_ParentNotInIndex(t *testing.T) {
	childID := makeSpanID(2)
	missingID := makeSpanID(99)
	child := newBS(childID, missingID, 0, "svc-a")
	idx := buildIndex(child)

	assert.Empty(t, subtraceMembers(missingID, idx))
	assert.Equal(t, map[pcommon.SpanID]bool{childID: true}, memberIDs(childID, idx))
}

// A root that isn't in the index has no members.
func TestSubtraceMembers_RootNotInIndex(t *testing.T) {
	assert.Empty(t, subtraceMembers(makeSpanID(42), newTraceIndex()))
}

// TestSubtraceMembers_CyclicParents verifies that collection terminates instead
// of looping forever when spans form a mutual parent cycle.
func TestSubtraceMembers_CyclicParents(t *testing.T) {
	aID := makeSpanID(0x0A)
	bID := makeSpanID(0x0B)
	// A's parent is B, B's parent is A — a cycle within the same service. Neither
	// is a local root, so nothing claims them, but asking for either must still
	// terminate.
	a := newBS(aID, bID, 0, "svc-a")
	b := newBS(bID, aID, 0, "svc-a")
	idx := buildIndex(a, b)

	done := make(chan map[pcommon.SpanID]bool, 1)
	go func() { done <- memberIDs(aID, idx) }()
	select {
	case members := <-done:
		// A is returned as the requested root; B is reachable from it as a child.
		assert.Equal(t, map[pcommon.SpanID]bool{aID: true, bID: true}, members)
	case <-time.After(5 * time.Second):
		t.Fatal("subtraceMembers() did not terminate: infinite loop on cyclic parent references")
	}
}

func TestAssemble_CoalescesSameResourceScope(t *testing.T) {
	r := pcommon.NewResource()
	r.Attributes().PutStr("service.name", "svc-a")
	sc := pcommon.NewInstrumentationScope()
	sc.SetName("lib")

	makeSpan := func(id byte) ptrace.Span {
		s := ptrace.NewSpan()
		s.SetSpanID(makeSpanID(id))
		s.SetTraceID(makeTraceID(1))
		return s
	}

	var members []*bufferedSpan
	for i := byte(1); i <= 3; i++ {
		members = append(members, newBufferedSpan(newSpanContext(newResourceContext(r), sc), makeSpan(i)))
	}

	td := assemble(members)
	assert.Equal(t, 1, td.ResourceSpans().Len())
	assert.Equal(t, 1, td.ResourceSpans().At(0).ScopeSpans().Len())
	assert.Equal(t, 3, td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().Len())
}

func TestAssemble_SeparatesDistinctResources(t *testing.T) {
	r1 := pcommon.NewResource()
	r1.Attributes().PutStr("service.name", "svc-a")
	r2 := pcommon.NewResource()
	r2.Attributes().PutStr("service.name", "svc-b")
	sc := pcommon.NewInstrumentationScope()

	makeSpanBS := func(r pcommon.Resource, id byte) *bufferedSpan {
		s := ptrace.NewSpan()
		s.SetSpanID(makeSpanID(id))
		return newBufferedSpan(newSpanContext(newResourceContext(r), sc), s)
	}

	members := []*bufferedSpan{makeSpanBS(r1, 1), makeSpanBS(r2, 2)}
	td := assemble(members)
	assert.Equal(t, 2, td.ResourceSpans().Len())
}

// Scope name and version are distinct parts of the grouping key, so a name that
// happens to contain the separator can't be confused with a versioned scope.
func TestAssemble_SeparatesAmbiguousScopeNameAndVersion(t *testing.T) {
	r := pcommon.NewResource()
	r.Attributes().PutStr("service.name", "svc-a")

	sc1 := pcommon.NewInstrumentationScope()
	sc1.SetName("lib@2")

	sc2 := pcommon.NewInstrumentationScope()
	sc2.SetName("lib")
	sc2.SetVersion("2")

	makeSpanBS := func(sc pcommon.InstrumentationScope, id byte) *bufferedSpan {
		s := ptrace.NewSpan()
		s.SetSpanID(makeSpanID(id))
		return newBufferedSpan(newSpanContext(newResourceContext(r), sc), s)
	}

	td := assemble([]*bufferedSpan{makeSpanBS(sc1, 1), makeSpanBS(sc2, 2)})
	require.Equal(t, 1, td.ResourceSpans().Len())
	assert.Equal(t, 2, td.ResourceSpans().At(0).ScopeSpans().Len())
}
