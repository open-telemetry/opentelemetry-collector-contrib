// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package groupbytraceprocessor

import (
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/xpdata/xhash"
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

// callInput describes one span to hand to splitCalls.
type callInput struct {
	id     pcommon.SpanID
	parent pcommon.SpanID
	remote bool
}

// resourceContextFor builds a resourceContext from a bare resource, for the
// tests that have no ResourceSpans of their own and no interest in a schema URL.
func resourceContextFor(r pcommon.Resource) resourceContext {
	rs := ptrace.NewResourceSpans()
	r.CopyTo(rs.Resource())
	return newResourceContext(rs)
}

// buildCallInput turns the given spans into the arguments splitCalls takes. Span
// IDs listed in elsewhere stand for spans buffered under a different service in
// the same trace, which is what makes an entry span distinguishable from a span
// whose parent never arrived.
func buildCallInput(service string, elsewhere []pcommon.SpanID, inputs ...callInput) (map[pcommon.SpanID][]*bufferedSpan, map[pcommon.SpanID][16]byte) {
	r := pcommon.NewResource()
	r.Attributes().PutStr("service.name", service)
	ctx := newSpanContext(resourceContextFor(r), pcommon.NewInstrumentationScope(), "")

	// otherServiceID is a sentinel for spans that live in a different service.
	// It only needs to be distinct from ctx.serviceID; splitCalls checks
	// existence only, not the exact value.
	otherServiceID := xhash.Hash(xhash.WithString("other-service"))

	spans := map[pcommon.SpanID][]*bufferedSpan{}
	traceSpanIDs := map[pcommon.SpanID][16]byte{}
	for _, in := range inputs {
		s := ptrace.NewSpan()
		s.SetSpanID(in.id)
		s.SetParentSpanID(in.parent)
		if in.remote {
			s.SetFlags(spanFlagsContextHasIsRemoteMask | spanFlagsContextIsRemoteMask)
		}
		spans[in.id] = append(spans[in.id], newBufferedSpan(ctx, s, time.Now()))
		traceSpanIDs[in.id] = ctx.serviceID
	}
	for _, id := range elsewhere {
		traceSpanIDs[id] = otherServiceID
	}
	return spans, traceSpanIDs
}

// callIDSets returns each call as a set of span IDs, ordered by size then by
// lowest span ID so that assertions don't depend on map iteration order.
func callIDSets(calls [][]*bufferedSpan) []map[pcommon.SpanID]bool {
	sets := make([]map[pcommon.SpanID]bool, 0, len(calls))
	for _, call := range calls {
		sets = append(sets, spanIDSet(call))
	}
	slices.SortFunc(sets, func(a, b map[pcommon.SpanID]bool) int {
		if len(a) != len(b) {
			return len(a) - len(b)
		}
		idsA, idsB := sortedIDs(a), sortedIDs(b)
		for i := range idsA {
			if c := slices.Compare(idsA[i][:], idsB[i][:]); c != 0 {
				return c
			}
		}
		return 0
	})
	return sets
}

func sortedIDs(set map[pcommon.SpanID]bool) []pcommon.SpanID {
	ids := make([]pcommon.SpanID, 0, len(set))
	for id := range set {
		ids = append(ids, id)
	}
	slices.SortFunc(ids, func(a, b pcommon.SpanID) int { return slices.Compare(a[:], b[:]) })
	return ids
}

func TestSplitCalls_SingleCall(t *testing.T) {
	root, mid, leaf := makeSpanID(1), makeSpanID(2), makeSpanID(3)
	spans, ids := buildCallInput("svc-a", nil,
		callInput{id: root},
		callInput{id: mid, parent: root},
		callInput{id: leaf, parent: mid},
	)

	assert.Equal(t, []map[pcommon.SpanID]bool{
		{root: true, mid: true, leaf: true},
	}, callIDSets(splitCalls(spans, ids)))
}

func TestSplitCalls_Siblings(t *testing.T) {
	root, left, right := makeSpanID(1), makeSpanID(2), makeSpanID(3)
	spans, ids := buildCallInput("svc-a", nil,
		callInput{id: root},
		callInput{id: left, parent: root},
		callInput{id: right, parent: root},
	)

	assert.Equal(t, []map[pcommon.SpanID]bool{
		{root: true, left: true, right: true},
	}, callIDSets(splitCalls(spans, ids)))
}

func TestSplitCalls_ServiceEnteredTwice(t *testing.T) {
	callerA, callerB := makeSpanID(0x10), makeSpanID(0x11)
	entry1, child1 := makeSpanID(1), makeSpanID(2)
	entry2, child2 := makeSpanID(3), makeSpanID(4)

	spans, ids := buildCallInput("svc-b", []pcommon.SpanID{callerA, callerB},
		callInput{id: entry1, parent: callerA},
		callInput{id: child1, parent: entry1},
		callInput{id: entry2, parent: callerB},
		callInput{id: child2, parent: entry2},
	)

	assert.Equal(t, []map[pcommon.SpanID]bool{
		{entry1: true, child1: true},
		{entry2: true, child2: true},
	}, callIDSets(splitCalls(spans, ids)))
}

func TestSplitCalls_TwoEntriesOneCaller(t *testing.T) {
	caller := makeSpanID(0x10)
	entry1, entry2 := makeSpanID(1), makeSpanID(2)

	spans, ids := buildCallInput("svc-b", []pcommon.SpanID{caller},
		callInput{id: entry1, parent: caller},
		callInput{id: entry2, parent: caller},
	)

	assert.Len(t, splitCalls(spans, ids), 2)
}

func TestSplitCalls_RemoteParentInSameService(t *testing.T) {
	root, entry := makeSpanID(1), makeSpanID(2)
	spans, ids := buildCallInput("svc-a", nil,
		callInput{id: root},
		callInput{id: entry, parent: root, remote: true},
	)

	assert.Equal(t, []map[pcommon.SpanID]bool{
		{root: true},
		{entry: true},
	}, callIDSets(splitCalls(spans, ids)))
}

func TestSplitCalls_ParentlessSpansStayTogether(t *testing.T) {
	missing := makeSpanID(0x63)
	a, b, c := makeSpanID(1), makeSpanID(2), makeSpanID(3)
	spans, ids := buildCallInput("svc-a", nil,
		callInput{id: a, parent: missing},
		callInput{id: b, parent: missing},
		callInput{id: c, parent: makeSpanID(0x64)},
	)

	assert.Equal(t, []map[pcommon.SpanID]bool{
		{a: true, b: true, c: true},
	}, callIDSets(splitCalls(spans, ids)))
}

func TestSplitCalls_ParentlessSpansSeparateFromKnownCall(t *testing.T) {
	caller, missing := makeSpanID(0x10), makeSpanID(0x63)
	entry, child, orphan := makeSpanID(1), makeSpanID(2), makeSpanID(3)

	spans, ids := buildCallInput("svc-b", []pcommon.SpanID{caller},
		callInput{id: entry, parent: caller},
		callInput{id: child, parent: entry},
		callInput{id: orphan, parent: missing},
	)

	assert.Equal(t, []map[pcommon.SpanID]bool{
		{orphan: true},
		{entry: true, child: true},
	}, callIDSets(splitCalls(spans, ids)))
}

// Malformed input where a span is its own ancestor leaves a ring that nothing
// heads. It must still be released rather than held forever.
func TestSplitCalls_CycleIsStillReleased(t *testing.T) {
	x, y := makeSpanID(1), makeSpanID(2)
	spans, ids := buildCallInput("svc-a", nil,
		callInput{id: x, parent: y},
		callInput{id: y, parent: x},
	)

	assert.Equal(t, []map[pcommon.SpanID]bool{
		{x: true, y: true},
	}, callIDSets(splitCalls(spans, ids)))
}

func TestSplitCalls_CycleBesideRealCall(t *testing.T) {
	root, x, y := makeSpanID(1), makeSpanID(2), makeSpanID(3)
	spans, ids := buildCallInput("svc-a", nil,
		callInput{id: root},
		callInput{id: x, parent: y},
		callInput{id: y, parent: x},
	)

	calls := splitCalls(spans, ids)
	assert.Equal(t, []map[pcommon.SpanID]bool{
		{root: true},
		{x: true, y: true},
	}, callIDSets(calls))
}

func TestSplitCalls_Empty(t *testing.T) {
	assert.Empty(t, splitCalls(map[pcommon.SpanID][]*bufferedSpan{}, map[pcommon.SpanID][16]byte{}))
}

// Every span goes into exactly one call, whatever the shape.
func TestSplitCalls_PartitionsEverySpan(t *testing.T) {
	caller, missing := makeSpanID(0x10), makeSpanID(0x63)
	spans, ids := buildCallInput("svc-a", []pcommon.SpanID{caller},
		callInput{id: makeSpanID(1)},
		callInput{id: makeSpanID(2), parent: makeSpanID(1)},
		callInput{id: makeSpanID(3), parent: caller},
		callInput{id: makeSpanID(4), parent: missing},
		callInput{id: makeSpanID(5), parent: makeSpanID(6)},
		callInput{id: makeSpanID(6), parent: makeSpanID(5)},
	)

	seen := map[pcommon.SpanID]int{}
	for _, call := range splitCalls(spans, ids) {
		for _, bs := range call {
			seen[bs.span.SpanID()]++
		}
	}
	require.Len(t, seen, len(spans))
	for id, n := range seen {
		assert.Equal(t, 1, n, "span %v appeared in %d calls", id, n)
	}
}

func serviceIDOf(t *testing.T, attrs map[string]string) [16]byte {
	t.Helper()
	r := pcommon.NewResource()
	for k, v := range attrs {
		r.Attributes().PutStr(k, v)
	}
	a := r.Attributes()
	return serviceIdentity(a, xhash.MapHash(a))
}

func TestServiceIdentity_IgnoresNonServiceAttributes(t *testing.T) {
	a := serviceIDOf(t, map[string]string{"service.name": "svc-a", "k8s.pod.name": "pod-1"})
	b := serviceIDOf(t, map[string]string{"service.name": "svc-a", "k8s.pod.name": "pod-2"})
	assert.Equal(t, a, b)
}

func TestServiceIdentity_DistinguishesNamespaceAndInstance(t *testing.T) {
	base := serviceIDOf(t, map[string]string{"service.name": "svc-a"})
	assert.NotEqual(t, base, serviceIDOf(t, map[string]string{"service.name": "svc-b"}))
	assert.NotEqual(t, base, serviceIDOf(t, map[string]string{"service.name": "svc-a", "service.namespace": "prod"}))
	assert.NotEqual(t, base, serviceIDOf(t, map[string]string{"service.name": "svc-a", "service.instance.id": "i-1"}))
}

func TestServiceIdentity_FallsBackToResourceHash(t *testing.T) {
	a := serviceIDOf(t, map[string]string{"host.name": "node-1"})
	assert.Equal(t, a, serviceIDOf(t, map[string]string{"host.name": "node-1"}))
	assert.NotEqual(t, a, serviceIDOf(t, map[string]string{"host.name": "node-2"}))
}

func TestAssemble_CoalescesSameResourceScope(t *testing.T) {
	r := pcommon.NewResource()
	r.Attributes().PutStr("service.name", "svc-a")
	sc := pcommon.NewInstrumentationScope()
	sc.SetName("lib")
	ctx := newSpanContext(resourceContextFor(r), sc, "")

	var members []*bufferedSpan
	for i := byte(1); i <= 3; i++ {
		s := ptrace.NewSpan()
		s.SetSpanID(makeSpanID(i))
		s.SetTraceID(makeTraceID(1))
		members = append(members, newBufferedSpan(ctx, s, time.Now()))
	}

	td := assemble(members)
	require.Equal(t, 1, td.ResourceSpans().Len())
	require.Equal(t, 1, td.ResourceSpans().At(0).ScopeSpans().Len())
	assert.Equal(t, 3, td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().Len())
}

func TestAssemble_SeparatesDistinctResources(t *testing.T) {
	makeBS := func(service string, id byte) *bufferedSpan {
		r := pcommon.NewResource()
		r.Attributes().PutStr("service.name", service)
		s := ptrace.NewSpan()
		s.SetSpanID(makeSpanID(id))
		return newBufferedSpan(newSpanContext(resourceContextFor(r), pcommon.NewInstrumentationScope(), ""), s, time.Now())
	}

	td := assemble([]*bufferedSpan{makeBS("svc-a", 1), makeBS("svc-b", 2)})
	assert.Equal(t, 2, td.ResourceSpans().Len())
}

func TestAssemble_SeparatesResourcesWithDistinctSchemaURL(t *testing.T) {
	// Two ResourceSpans with identical attributes but different schema URLs must
	// not merge into one.
	makeBS := func(rsSchemaURL string, id byte) *bufferedSpan {
		rs := ptrace.NewResourceSpans()
		rs.Resource().Attributes().PutStr("service.name", "svc-a")
		rs.SetSchemaUrl(rsSchemaURL)
		s := ptrace.NewSpan()
		s.SetSpanID(makeSpanID(id))
		return newBufferedSpan(newSpanContext(newResourceContext(rs), pcommon.NewInstrumentationScope(), ""), s, time.Now())
	}

	td := assemble([]*bufferedSpan{makeBS("https://opentelemetry.io/schemas/1.24.0", 1), makeBS("https://opentelemetry.io/schemas/1.25.0", 2)})
	assert.Equal(t, 2, td.ResourceSpans().Len())
}

func TestAssemble_SeparatesResourcesWithDistinctDroppedAttributesCount(t *testing.T) {
	// Two ResourceSpans with identical attributes but different DroppedAttributesCount
	// must not be merged, or the count on one would be silently lost.
	makeBS := func(dropped uint32, id byte) *bufferedSpan {
		rs := ptrace.NewResourceSpans()
		rs.Resource().Attributes().PutStr("service.name", "svc-a")
		rs.Resource().SetDroppedAttributesCount(dropped)
		s := ptrace.NewSpan()
		s.SetSpanID(makeSpanID(id))
		return newBufferedSpan(newSpanContext(newResourceContext(rs), pcommon.NewInstrumentationScope(), ""), s, time.Now())
	}

	td := assemble([]*bufferedSpan{makeBS(0, 1), makeBS(3, 2)})
	require.Equal(t, 2, td.ResourceSpans().Len())
	counts := map[uint32]bool{}
	for i := range td.ResourceSpans().Len() {
		counts[td.ResourceSpans().At(i).Resource().DroppedAttributesCount()] = true
	}
	assert.Equal(t, map[uint32]bool{0: true, 3: true}, counts)
}

func TestAssemble_SeparatesScopesWithDistinctDroppedAttributesCount(t *testing.T) {
	// Two ScopeSpans with the same name/version/attributes but different
	// DroppedAttributesCount must not be merged.
	r := pcommon.NewResource()
	r.Attributes().PutStr("service.name", "svc-a")
	rctx := resourceContextFor(r)

	makeBS := func(dropped uint32, id byte) *bufferedSpan {
		sc := pcommon.NewInstrumentationScope()
		sc.SetName("lib")
		sc.SetDroppedAttributesCount(dropped)
		s := ptrace.NewSpan()
		s.SetSpanID(makeSpanID(id))
		return newBufferedSpan(newSpanContext(rctx, sc, ""), s, time.Now())
	}

	td := assemble([]*bufferedSpan{makeBS(0, 1), makeBS(7, 2)})
	require.Equal(t, 1, td.ResourceSpans().Len())
	require.Equal(t, 2, td.ResourceSpans().At(0).ScopeSpans().Len())
	counts := map[uint32]bool{}
	for i := range td.ResourceSpans().At(0).ScopeSpans().Len() {
		counts[td.ResourceSpans().At(0).ScopeSpans().At(i).Scope().DroppedAttributesCount()] = true
	}
	assert.Equal(t, map[uint32]bool{0: true, 7: true}, counts)
}

func TestAssemble_PreservesScopeSchemaURL(t *testing.T) {
	r := pcommon.NewResource()
	r.Attributes().PutStr("service.name", "svc-a")
	rctx := resourceContextFor(r)

	sc := pcommon.NewInstrumentationScope()
	sc.SetName("lib")
	s := ptrace.NewSpan()
	s.SetSpanID(makeSpanID(1))

	td := assemble([]*bufferedSpan{newBufferedSpan(newSpanContext(rctx, sc, "https://opentelemetry.io/schemas/1.24.0"), s, time.Now())})
	require.Equal(t, 1, td.ResourceSpans().Len())
	require.Equal(t, 1, td.ResourceSpans().At(0).ScopeSpans().Len())
	assert.Equal(t, "https://opentelemetry.io/schemas/1.24.0", td.ResourceSpans().At(0).ScopeSpans().At(0).SchemaUrl())
}

func TestAssemble_SeparatesAmbiguousScopeNameAndVersion(t *testing.T) {
	r := pcommon.NewResource()
	r.Attributes().PutStr("service.name", "svc-a")
	rctx := resourceContextFor(r)

	makeBS := func(name, version string, id byte) *bufferedSpan {
		sc := pcommon.NewInstrumentationScope()
		sc.SetName(name)
		sc.SetVersion(version)
		s := ptrace.NewSpan()
		s.SetSpanID(makeSpanID(id))
		return newBufferedSpan(newSpanContext(rctx, sc, ""), s, time.Now())
	}

	td := assemble([]*bufferedSpan{makeBS("lib@2", "", 1), makeBS("lib", "2", 2)})
	require.Equal(t, 1, td.ResourceSpans().Len())
	assert.Equal(t, 2, td.ResourceSpans().At(0).ScopeSpans().Len())
}

func TestSplitCalls_TerminatesOnCycle(t *testing.T) {
	spans, ids := buildCallInput("svc-a", nil,
		callInput{id: makeSpanID(1), parent: makeSpanID(2)},
		callInput{id: makeSpanID(2), parent: makeSpanID(3)},
		callInput{id: makeSpanID(3), parent: makeSpanID(1)},
	)

	done := make(chan int, 1)
	go func() { done <- len(splitCalls(spans, ids)) }()
	select {
	case n := <-done:
		assert.Equal(t, 1, n)
	case <-time.After(5 * time.Second):
		t.Fatal("splitCalls() did not terminate on a cycle")
	}
}

func TestAssemble_PreservesSpanPayload(t *testing.T) {
	r := pcommon.NewResource()
	r.Attributes().PutStr("service.name", "svc-a")
	sc := pcommon.NewInstrumentationScope()
	sc.SetName("lib")
	sc.SetVersion("1.2.3")

	s := ptrace.NewSpan()
	s.SetTraceID(makeTraceID(1))
	s.SetSpanID(makeSpanID(1))
	s.SetParentSpanID(makeSpanID(2))
	s.SetName("GET /api/checkout")
	s.SetKind(ptrace.SpanKindServer)
	s.SetStartTimestamp(pcommon.Timestamp(1_000))
	s.SetEndTimestamp(pcommon.Timestamp(2_000))
	s.TraceState().FromRaw("vendor=foo")
	s.SetFlags(spanFlagsContextHasIsRemoteMask | spanFlagsContextIsRemoteMask)
	s.Status().SetCode(ptrace.StatusCodeError)
	s.Status().SetMessage("boom")
	s.Attributes().PutStr("http.request.method", "GET")
	s.Attributes().PutInt("http.response.status_code", 500)
	s.SetDroppedAttributesCount(3)
	ev := s.Events().AppendEmpty()
	ev.SetName("exception")
	ev.SetTimestamp(pcommon.Timestamp(1_500))
	ev.Attributes().PutStr("exception.type", "TimeoutError")
	ev.SetDroppedAttributesCount(2)
	s.SetDroppedEventsCount(4)
	lk := s.Links().AppendEmpty()
	lk.SetTraceID(makeTraceID(9))
	lk.SetSpanID(makeSpanID(7))
	lk.TraceState().FromRaw("vendor=bar")
	lk.Attributes().PutStr("link.kind", "follows_from")
	lk.SetDroppedAttributesCount(1)
	lk.SetFlags(spanFlagsContextHasIsRemoteMask)
	s.SetDroppedLinksCount(5)

	td := assemble([]*bufferedSpan{newBufferedSpan(newSpanContext(resourceContextFor(r), sc, ""), s, time.Now())})

	require.Equal(t, 1, td.ResourceSpans().Len())
	rs := td.ResourceSpans().At(0)
	svc, ok := rs.Resource().Attributes().Get("service.name")
	require.True(t, ok)
	assert.Equal(t, "svc-a", svc.Str())

	require.Equal(t, 1, rs.ScopeSpans().Len())
	ss := rs.ScopeSpans().At(0)
	assert.Equal(t, "lib", ss.Scope().Name())
	assert.Equal(t, "1.2.3", ss.Scope().Version())

	require.Equal(t, 1, ss.Spans().Len())
	got := ss.Spans().At(0)
	assert.Equal(t, makeTraceID(1), got.TraceID())
	assert.Equal(t, makeSpanID(1), got.SpanID())
	assert.Equal(t, makeSpanID(2), got.ParentSpanID())
	assert.Equal(t, "GET /api/checkout", got.Name())
	assert.Equal(t, ptrace.SpanKindServer, got.Kind())
	assert.Equal(t, pcommon.Timestamp(1_000), got.StartTimestamp())
	assert.Equal(t, pcommon.Timestamp(2_000), got.EndTimestamp())
	assert.Equal(t, "vendor=foo", got.TraceState().AsRaw())
	assert.Equal(t, spanFlagsContextHasIsRemoteMask|spanFlagsContextIsRemoteMask, got.Flags())
	assert.Equal(t, ptrace.StatusCodeError, got.Status().Code())
	assert.Equal(t, "boom", got.Status().Message())
	assert.Equal(t, map[string]any{
		"http.request.method":       "GET",
		"http.response.status_code": int64(500),
	}, got.Attributes().AsRaw())
	assert.Equal(t, uint32(3), got.DroppedAttributesCount())
	assert.Equal(t, uint32(4), got.DroppedEventsCount())
	assert.Equal(t, uint32(5), got.DroppedLinksCount())

	require.Equal(t, 1, got.Events().Len())
	gotEv := got.Events().At(0)
	assert.Equal(t, "exception", gotEv.Name())
	assert.Equal(t, pcommon.Timestamp(1_500), gotEv.Timestamp())
	assert.Equal(t, map[string]any{"exception.type": "TimeoutError"}, gotEv.Attributes().AsRaw())
	assert.Equal(t, uint32(2), gotEv.DroppedAttributesCount())

	require.Equal(t, 1, got.Links().Len())
	gotLk := got.Links().At(0)
	assert.Equal(t, makeTraceID(9), gotLk.TraceID())
	assert.Equal(t, makeSpanID(7), gotLk.SpanID())
	assert.Equal(t, "vendor=bar", gotLk.TraceState().AsRaw())
	assert.Equal(t, map[string]any{"link.kind": "follows_from"}, gotLk.Attributes().AsRaw())
	assert.Equal(t, uint32(1), gotLk.DroppedAttributesCount())
	assert.Equal(t, spanFlagsContextHasIsRemoteMask, gotLk.Flags())
}

func TestAssemble_TakesOwnershipOfSpans(t *testing.T) {
	r := pcommon.NewResource()
	r.Attributes().PutStr("service.name", "svc-a")
	ctx := newSpanContext(resourceContextFor(r), pcommon.NewInstrumentationScope(), "")

	s := ptrace.NewSpan()
	s.SetSpanID(makeSpanID(1))
	s.SetName("GET /api/checkout")
	s.Attributes().PutStr("http.request.method", "GET")
	bs := newBufferedSpan(ctx, s, time.Now())

	td := assemble([]*bufferedSpan{bs})
	require.Equal(t, 1, td.SpanCount())

	assert.True(t, bs.span.SpanID().IsEmpty(), "the span should have been moved out, not copied")
	assert.Empty(t, bs.span.Name())
	assert.Equal(t, 0, bs.span.Attributes().Len())

	// The resource and scope are shared with other buffered spans, so they are
	// copied and must survive.
	assert.Equal(t, map[string]any{"service.name": "svc-a"}, bs.resource.Attributes().AsRaw())
}

// A span deep under an entry span must be resolved to that entry's call, and
// resolving it must not depend on the order spans happen to be visited in.
func TestSplitCalls_DeepChainResolvesToOneCall(t *testing.T) {
	const depth = 200
	inputs := make([]callInput, 0, depth)
	for i := range depth {
		in := callInput{id: makeSpanID(byte(i + 1))}
		if i > 0 {
			in.parent = makeSpanID(byte(i))
		}
		inputs = append(inputs, in)
	}
	spans, ids := buildCallInput("svc-a", nil, inputs...)

	calls := splitCalls(spans, ids)
	require.Len(t, calls, 1)
	assert.Len(t, calls[0], depth)
}

// Two deep chains under two separate entry spans must stay apart, which is what
// memoising the walk has to get right when the chains are resolved in any order.
func TestSplitCalls_TwoDeepChainsStaySeparate(t *testing.T) {
	const depth = 100
	callerOne, callerTwo := makeSpanID(0xF1), makeSpanID(0xF2)

	inputs := make([]callInput, 0, 2*depth)
	for i := range depth {
		first := callInput{id: makeSpanID(byte(i + 1)), parent: makeSpanID(byte(i))}
		second := callInput{id: makeSpanID(byte(i + 1 + depth)), parent: makeSpanID(byte(i + depth))}
		if i == 0 {
			first.parent, second.parent = callerOne, callerTwo
		}
		inputs = append(inputs, first, second)
	}
	spans, ids := buildCallInput("svc-b", []pcommon.SpanID{callerOne, callerTwo}, inputs...)

	calls := splitCalls(spans, ids)
	require.Len(t, calls, 2, "two entry spans means two calls, however deep each runs")
	for _, call := range calls {
		assert.Len(t, call, depth)
	}
	// And no span is in both.
	seen := map[pcommon.SpanID]int{}
	for _, call := range calls {
		for _, bs := range call {
			seen[bs.span.SpanID()]++
		}
	}
	require.Len(t, seen, 2*depth)
	for id, n := range seen {
		assert.Equal(t, 1, n, "span %v appeared in %d calls", id, n)
	}
}

// A span with an empty span ID must not be treated as the parent of legitimate
// root spans (which also have an empty ParentSpanID), collapsing separate calls
// into one unreachable batch.
func TestSplitCalls_EmptySpanIDDoesNotCollapseGrouping(t *testing.T) {
	rootA, childA, rootB := makeSpanID(1), makeSpanID(2), makeSpanID(3)
	var empty pcommon.SpanID // all zeros — invalid/missing span ID

	spans, ids := buildCallInput("svc-a", nil,
		callInput{id: rootA},
		callInput{id: childA, parent: rootA},
		callInput{id: rootB},
		callInput{id: empty},
	)

	// rootA+childA and rootB must each head their own call; the empty-ID span
	// goes into its own call as well. Without the fix, all four collapse into one
	// unreachable batch because every root's empty ParentSpanID resolves to the
	// buffered empty-ID span, forming a cycle.
	assert.Equal(t, []map[pcommon.SpanID]bool{
		{empty: true},
		{rootB: true},
		{rootA: true, childA: true},
	}, callIDSets(splitCalls(spans, ids)))
}

// A chain that runs into a ring partway up belongs with the ring: no entry span
// accounts for any of it.
func TestSplitCalls_ChainIntoCycleIsUnreachable(t *testing.T) {
	x, y := makeSpanID(1), makeSpanID(2)
	hangingOff := makeSpanID(3)
	spans, ids := buildCallInput("svc-a", nil,
		callInput{id: x, parent: y},
		callInput{id: y, parent: x},
		callInput{id: hangingOff, parent: y},
	)

	calls := splitCalls(spans, ids)
	require.Len(t, calls, 1)
	assert.Equal(t, map[pcommon.SpanID]bool{x: true, y: true, hangingOff: true}, spanIDSet(calls[0]))
}
