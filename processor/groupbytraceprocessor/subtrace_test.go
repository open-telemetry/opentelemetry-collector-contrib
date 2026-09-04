// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package groupbytraceprocessor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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
func newBS(spanID, parentID pcommon.SpanID, flags uint32, serviceName string) bufferedSpan {
	r := pcommon.NewResource()
	if serviceName != "" {
		r.Attributes().PutStr("service.name", serviceName)
	}
	s := ptrace.NewSpan()
	s.SetSpanID(spanID)
	s.SetParentSpanID(parentID)
	s.SetFlags(flags)
	return bufferedSpan{resource: r, scope: pcommon.NewInstrumentationScope(), span: s}
}

// empty parent --> always local root
func TestIsLocalRoot_EmptyParent(t *testing.T) {
	bs := newBS(makeSpanID(1), pcommon.NewSpanIDEmpty(), 0, "svc-a")
	assert.True(t, isLocalRoot(bs, map[pcommon.SpanID]bufferedSpan{}))
}

// HAS_IS_REMOTE=1, IS_REMOTE=1 --> local root
func TestIsLocalRoot_RemoteFlagSet(t *testing.T) {
	bs := newBS(makeSpanID(2), makeSpanID(99), spanFlagsContextHasIsRemoteMask|spanFlagsContextIsRemoteMask, "svc-a")
	assert.True(t, isLocalRoot(bs, map[pcommon.SpanID]bufferedSpan{}))
}

// HAS_IS_REMOTE=1, IS_REMOTE=0 --> safe default: local root
func TestIsLocalRoot_LocalFlagClear(t *testing.T) {
	bs := newBS(makeSpanID(3), makeSpanID(99), spanFlagsContextHasIsRemoteMask, "svc-a")
	assert.True(t, isLocalRoot(bs, map[pcommon.SpanID]bufferedSpan{}))
}

// parent not in index --> safe default: local root
func TestIsLocalRoot_ParentNotInIndex(t *testing.T) {
	bs := newBS(makeSpanID(4), makeSpanID(99), 0, "svc-a")
	assert.True(t, isLocalRoot(bs, map[pcommon.SpanID]bufferedSpan{}))
}

// parent in index, same service.name only --> NOT local root
func TestIsLocalRoot_SameServiceNameOnly(t *testing.T) {
	parentID := makeSpanID(10)
	child := newBS(makeSpanID(5), parentID, 0, "svc-a")
	parent := newBS(parentID, pcommon.NewSpanIDEmpty(), 0, "svc-a")
	index := map[pcommon.SpanID]bufferedSpan{parentID: parent}
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
	child := bufferedSpan{resource: r, scope: pcommon.NewInstrumentationScope(), span: s}

	pr := pcommon.NewResource()
	pr.Attributes().PutStr("service.name", "svc-a")
	pr.Attributes().PutStr("service.instance.id", "inst-1")
	ps := ptrace.NewSpan()
	ps.SetSpanID(parentID)
	parentBS := bufferedSpan{resource: pr, scope: pcommon.NewInstrumentationScope(), span: ps}

	index := map[pcommon.SpanID]bufferedSpan{parentID: parentBS}
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
	child := bufferedSpan{resource: r, scope: pcommon.NewInstrumentationScope(), span: s}

	pr := pcommon.NewResource()
	pr.Attributes().PutStr("service.name", "svc-a")
	pr.Attributes().PutStr("service.instance.id", "inst-1")
	ps := ptrace.NewSpan()
	ps.SetSpanID(parentID)
	parentBS := bufferedSpan{resource: pr, scope: pcommon.NewInstrumentationScope(), span: ps}

	index := map[pcommon.SpanID]bufferedSpan{parentID: parentBS}
	assert.True(t, isLocalRoot(child, index))
}

// parent in index, different service name --> local root
func TestIsLocalRoot_DifferentServiceName(t *testing.T) {
	parentID := makeSpanID(10)
	child := newBS(makeSpanID(8), parentID, 0, "svc-b")
	parent := newBS(parentID, pcommon.NewSpanIDEmpty(), 0, "svc-a")
	index := map[pcommon.SpanID]bufferedSpan{parentID: parent}
	assert.True(t, isLocalRoot(child, index))
}

// no service.name, same attribute map --> NOT local root (same hash)
func TestIsLocalRoot_NoServiceNameSameAttrs(t *testing.T) {
	parentID := makeSpanID(10)
	child := newBS(makeSpanID(9), parentID, 0, "")
	parent := newBS(parentID, pcommon.NewSpanIDEmpty(), 0, "")
	index := map[pcommon.SpanID]bufferedSpan{parentID: parent}
	assert.False(t, isLocalRoot(child, index))
}

func buildIndex(spans ...bufferedSpan) map[pcommon.SpanID]bufferedSpan {
	idx := make(map[pcommon.SpanID]bufferedSpan, len(spans))
	for _, bs := range spans {
		idx[bs.span.SpanID()] = bs
	}
	return idx
}

// Walk succeeds for a direct parent-child within same service.
func TestReaches_DirectParent(t *testing.T) {
	rootID := makeSpanID(1)
	childID := makeSpanID(2)
	root := newBS(rootID, pcommon.NewSpanIDEmpty(), 0, "svc-a")
	child := newBS(childID, rootID, 0, "svc-a")
	idx := buildIndex(root, child)
	assert.True(t, reaches(childID, rootID, idx))
}

// Walk succeeds for multi-hop chain within same service.
func TestReaches_MultiHop(t *testing.T) {
	rootID := makeSpanID(1)
	midID := makeSpanID(2)
	leafID := makeSpanID(3)
	root := newBS(rootID, pcommon.NewSpanIDEmpty(), 0, "svc-a")
	mid := newBS(midID, rootID, 0, "svc-a")
	leaf := newBS(leafID, midID, 0, "svc-a")
	idx := buildIndex(root, mid, leaf)
	assert.True(t, reaches(leafID, rootID, idx))
}

// Walk stops at a different local root (crosses service boundary).
func TestReaches_StopsAtDifferentLocalRoot(t *testing.T) {
	rootA := makeSpanID(1)
	rootB := makeSpanID(2)
	childB := makeSpanID(3)
	a := newBS(rootA, pcommon.NewSpanIDEmpty(), 0, "svc-a")
	b := newBS(rootB, rootA, spanFlagsContextHasIsRemoteMask|spanFlagsContextIsRemoteMask, "svc-b")
	c := newBS(childB, rootB, 0, "svc-b")
	idx := buildIndex(a, b, c)
	// c reaches rootB but not rootA
	assert.True(t, reaches(childB, rootB, idx))
	assert.False(t, reaches(childB, rootA, idx))
}

// Walk fails when parent not in index.
func TestReaches_ParentNotInIndex(t *testing.T) {
	childID := makeSpanID(2)
	missingID := makeSpanID(99)
	child := newBS(childID, missingID, 0, "svc-a")
	idx := buildIndex(child)
	assert.False(t, reaches(childID, missingID, idx))
}

// Span not in index returns false immediately.
func TestReaches_SpanNotInIndex(t *testing.T) {
	assert.False(t, reaches(makeSpanID(42), makeSpanID(1), map[pcommon.SpanID]bufferedSpan{}))
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

	var members []bufferedSpan
	for i := byte(1); i <= 3; i++ {
		members = append(members, bufferedSpan{resource: r, scope: sc, span: makeSpan(i)})
	}

	td := assemble(members)
	assert.Equal(t, 1, td.ResourceSpans().Len())
	assert.Equal(t, 1, td.ResourceSpans().At(0).ScopeSpans().Len())
	assert.Equal(t, 3, td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().Len())
}

// TestReaches_CyclicParents verifies that reaches() terminates instead of
// looping forever when spans form a mutual parent cycle.
func TestReaches_CyclicParents(t *testing.T) {
	aID := makeSpanID(0x0A)
	bID := makeSpanID(0x0B)
	unreachableID := makeSpanID(0x0C) // not in the index, not the target
	// A's parent is B, B's parent is A — a cycle within the same service.
	a := newBS(aID, bID, 0, "svc-a")
	b := newBS(bID, aID, 0, "svc-a")
	idx := buildIndex(a, b)

	// Searching for an ID that is neither A nor B forces the walker to loop
	// through the A→B→A cycle. Without cycle detection this hangs forever.
	done := make(chan bool, 1)
	go func() { done <- reaches(aID, unreachableID, idx) }()
	select {
	case result := <-done:
		assert.False(t, result)
	case <-time.After(5 * time.Second):
		t.Fatal("reaches() did not terminate: infinite loop on cyclic parent references")
	}
}

func TestAssemble_SeparatesDistinctResources(t *testing.T) {
	r1 := pcommon.NewResource()
	r1.Attributes().PutStr("service.name", "svc-a")
	r2 := pcommon.NewResource()
	r2.Attributes().PutStr("service.name", "svc-b")
	sc := pcommon.NewInstrumentationScope()

	makeSpanBS := func(r pcommon.Resource, id byte) bufferedSpan {
		s := ptrace.NewSpan()
		s.SetSpanID(makeSpanID(id))
		return bufferedSpan{resource: r, scope: sc, span: s}
	}

	members := []bufferedSpan{makeSpanBS(r1, 1), makeSpanBS(r2, 2)}
	td := assemble(members)
	assert.Equal(t, 2, td.ResourceSpans().Len())
}
