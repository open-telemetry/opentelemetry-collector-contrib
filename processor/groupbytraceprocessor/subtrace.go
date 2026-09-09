// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package groupbytraceprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/groupbytraceprocessor"

import (
	"encoding/hex"
	"slices"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil"
)

// subtraceID is the composite key that uniquely identifies one service's
// subtrace within a distributed trace.
//
// Most subtraces are identified by their local root span. Spans whose parent
// never made it into the buffer have no such span to point at, and there can be
// any number of them, so those are grouped per service instead and identified by
// serviceID with an empty spanID. Grouping them keeps a service's spans in one
// batch instead of emitting each parentless span on its own.
type subtraceID struct {
	traceID pcommon.TraceID
	spanID  pcommon.SpanID // the local root span, or empty for an orphan group
	// serviceID is set only for orphan groups, and is empty otherwise.
	serviceID string
}

// isOrphanGroup reports whether the id refers to a service's parentless spans
// rather than to a single local root.
func (id subtraceID) isOrphanGroup() bool {
	return id.spanID.IsEmpty()
}

// scopeKey identifies an instrumentation scope for grouping purposes. The parts
// are kept separate rather than concatenated so that, for instance, the scope
// named "lib@2" with no version can't collide with "lib" at version "2".
type scopeKey struct {
	name     string
	version  string
	attrHash string
}

// resourceContext holds the resource a span was reported under, together with
// the keys derived from it.
type resourceContext struct {
	resource pcommon.Resource

	// serviceID is derived up front because it is hot: local root detection
	// compares it for every span that arrives. resourceKey is what assemble
	// groups resources on.
	serviceID   string
	resourceKey string
}

// newResourceContext deep-copies the resource so the result is self-contained
// and the caller can recycle its pdata objects, then derives the keys used to
// classify and group the spans reported under it.
func newResourceContext(resource pcommon.Resource) resourceContext {
	rCopy := pcommon.NewResource()
	resource.CopyTo(rCopy)

	return resourceContext{
		resource:    rCopy,
		serviceID:   serviceIdentity(rCopy),
		resourceKey: hashMapAttrs(rCopy.Attributes()),
	}
}

// spanContext adds the instrumentation scope to a resourceContext, so that it
// describes everything a span carries beyond the span itself.
type spanContext struct {
	resourceContext

	scope pcommon.InstrumentationScope

	// scopeKey is what assemble groups scopes on, within a resource.
	scopeKey scopeKey
}

// newSpanContext deep-copies the scope and pairs it with an already-built
// resourceContext, which it shares as-is with the other scopes under that
// resource.
func newSpanContext(rctx resourceContext, scope pcommon.InstrumentationScope) spanContext {
	sCopy := pcommon.NewInstrumentationScope()
	scope.CopyTo(sCopy)

	return spanContext{
		resourceContext: rctx,
		scope:           sCopy,
		scopeKey: scopeKey{
			name:     sCopy.Name(),
			version:  sCopy.Version(),
			attrHash: hashMapAttrs(sCopy.Attributes()),
		},
	}
}

// bufferedSpan holds a deep copy of a single span together with the resource and
// instrumentation scope it was reported under, enabling span-level indexing.
type bufferedSpan struct {
	spanContext
	span ptrace.Span

	// ownRootConfirmed records that this span was seen to head a subtrace of its
	// own while its parent was still buffered. Once its parent is released the
	// index can no longer tell "entered from another service" apart from "parent
	// hasn't arrived yet", and without this the span would fall back to looking
	// merely parentless. Only ever written under the storage write lock.
	ownRootConfirmed bool
}

// newBufferedSpan deep-copies the span so the caller can recycle its pdata
// objects. The context is shared as-is with the other spans reported under it.
func newBufferedSpan(ctx spanContext, span ptrace.Span) *bufferedSpan {
	spCopy := ptrace.NewSpan()
	span.CopyTo(spCopy)

	return &bufferedSpan{spanContext: ctx, span: spCopy}
}

// traceIndex holds every span buffered for one trace, keyed by span ID, plus a
// parent-to-children mapping.
type traceIndex struct {
	spans    map[pcommon.SpanID]*bufferedSpan
	children map[pcommon.SpanID][]pcommon.SpanID
}

func newTraceIndex() *traceIndex {
	return &traceIndex{
		spans:    make(map[pcommon.SpanID]*bufferedSpan),
		children: make(map[pcommon.SpanID][]pcommon.SpanID),
	}
}

// insert indexes bs, replacing any span already held under the same span ID.
func (idx *traceIndex) insert(bs *bufferedSpan) {
	spanID := bs.span.SpanID()
	parent := bs.span.ParentSpanID()

	if previous, replaced := idx.spans[spanID]; replaced {
		// A replacement usually repeats the span verbatim, in which case the child
		// link already holds. If it names a different parent, though, leaving the
		// old link in place would put the span in the wrong subtrace, so move it.
		previousParent := previous.span.ParentSpanID()
		if previousParent == parent {
			idx.spans[spanID] = bs
			return
		}
		idx.unlinkChild(previousParent, spanID)
	}

	if !parent.IsEmpty() {
		idx.children[parent] = append(idx.children[parent], spanID)
	}
	idx.spans[spanID] = bs
}

// unlinkChild drops spanID from its parent's list of children.
func (idx *traceIndex) unlinkChild(parent, spanID pcommon.SpanID) {
	if parent.IsEmpty() {
		return
	}
	siblings, ok := idx.children[parent]
	if !ok {
		return
	}
	siblings = slices.DeleteFunc(siblings, func(id pcommon.SpanID) bool { return id == spanID })
	if len(siblings) == 0 {
		delete(idx.children, parent)
	} else {
		idx.children[parent] = siblings
	}
}

// remove drops the given spans and every reference to them.
func (idx *traceIndex) remove(spans []*bufferedSpan) {
	removed := make(map[pcommon.SpanID]struct{}, len(spans))
	for _, bs := range spans {
		removed[bs.span.SpanID()] = struct{}{}
	}

	for _, bs := range spans {
		spanID := bs.span.SpanID()

		// A child left behind by a removed parent was left behind because it heads
		// a subtrace of its own; anything else would have been collected with it.
		// Record that while the parent is still here to prove it.
		for _, childID := range idx.children[spanID] {
			if _, going := removed[childID]; going {
				continue
			}
			if child, ok := idx.spans[childID]; ok {
				child.ownRootConfirmed = true
			}
		}

		delete(idx.spans, spanID)
		delete(idx.children, spanID)

		// Unlink from the parent as well, which matters when the parent outlives
		// this span. Members removed together take their own entries with them, so
		// this only ever prunes at the edge of the removed set.
		idx.unlinkChild(bs.span.ParentSpanID(), spanID)
	}
}

func (idx *traceIndex) len() int {
	return len(idx.spans)
}

// unclaimed returns the spans that no local root in the trace can collect.
//
// A subtrace only ever collects spans reachable downwards from its root, so a
// span whose ancestry never arrives at a local root belongs to no subtrace and
// no subtrace timer will ever release it. That happens for malformed input where
// a span is its own ancestor: nothing in such a cycle qualifies as a local root,
// so nothing claims any of it.
//
// This is deliberately defined by reachability rather than by age, so that
// sweeping the result can never take spans a pending subtrace was about to
// collect, no matter how the timers interleave.
func (idx *traceIndex) unclaimed() []*bufferedSpan {
	// Marking downwards from every local root costs one visit per span in total,
	// because the subtree under each root is disjoint from every other. Orphan
	// roots are marked individually rather than a group at a time for the same
	// reason: the group is exactly the union of their subtrees.
	claimed := make(map[pcommon.SpanID]struct{}, len(idx.spans))
	for spanID, bs := range idx.spans {
		if !isLocalRoot(bs, idx) {
			continue
		}
		for _, member := range descendFrom(spanID, idx) {
			claimed[member.span.SpanID()] = struct{}{}
		}
	}

	var unclaimed []*bufferedSpan
	for spanID, bs := range idx.spans {
		if _, ok := claimed[spanID]; !ok {
			unclaimed = append(unclaimed, bs)
		}
	}
	return unclaimed
}

const (
	// spanFlagsContextHasIsRemoteMask is set when the IS_REMOTE flag is explicitly present.
	spanFlagsContextHasIsRemoteMask uint32 = 0x00000100
	// spanFlagsContextIsRemoteMask is set when the parent context came from a remote caller.
	spanFlagsContextIsRemoteMask uint32 = 0x00000200
)

// rootKind describes whether, and how, a span heads a subtrace.
type rootKind int

const (
	// notRoot: the span is collected as part of an ancestor's subtrace.
	notRoot rootKind = iota
	// ownRoot: the span is a service-entry span and heads a subtrace of its own.
	ownRoot
	// orphanRoot: the span looks like an entry span only because its parent is
	// not buffered, so it is grouped with the service's other parentless spans
	// rather than heading a subtrace by itself.
	orphanRoot
)

// classifyRoot decides whether bs heads a subtrace. A span is a local root when:
//   - its parent span ID is empty (global root), OR
//   - the IS_REMOTE flag is set (parent is in another service), OR
//   - its parent belongs to a different service identity, OR
//   - its parent is not in the index (safe default: treat as a local root).
//
// The last of those is a guess rather than a statement about the trace, which is
// why it is reported separately: the parent may still arrive, and until it does
// there is nothing to distinguish one such span from another in the same
// service.
func classifyRoot(bs *bufferedSpan, idx *traceIndex) rootKind {
	if bs.span.ParentSpanID().IsEmpty() {
		return ownRoot
	}
	flags := bs.span.Flags()
	// If IS_REMOTE is set, consider it authoritative.
	// If it is not set, default to treating this span as a local root if any of the
	// remaining checks are met.
	if flags&spanFlagsContextHasIsRemoteMask != 0 && flags&spanFlagsContextIsRemoteMask != 0 {
		return ownRoot
	}
	if bs.ownRootConfirmed {
		return ownRoot
	}
	parent, ok := idx.spans[bs.span.ParentSpanID()]
	if !ok {
		return orphanRoot
	}
	if bs.serviceID != parent.serviceID {
		return ownRoot
	}
	return notRoot
}

// isLocalRoot returns true if bs heads a subtrace, of either kind.
func isLocalRoot(bs *bufferedSpan, idx *traceIndex) bool {
	return classifyRoot(bs, idx) != notRoot
}

// subtraceIDFor returns the subtrace that bs heads, if it heads one.
func subtraceIDFor(traceID pcommon.TraceID, bs *bufferedSpan, idx *traceIndex) (subtraceID, bool) {
	switch classifyRoot(bs, idx) {
	case ownRoot:
		return subtraceID{traceID: traceID, spanID: bs.span.SpanID()}, true
	case orphanRoot:
		return subtraceID{traceID: traceID, serviceID: bs.serviceID}, true
	case notRoot:
	}
	return subtraceID{}, false
}

// serviceIdentity returns a string that uniquely identifies the service for a
// given resource. It uses service.namespace, service.name, and
// service.instance.id when present; otherwise it falls back to a hash of all
// resource attributes.
func serviceIdentity(r pcommon.Resource) string {
	attrs := r.Attributes()
	name, hasName := attrs.Get("service.name")
	if !hasName {
		return hashMapAttrs(attrs)
	}
	var namespace, id string
	if v, ok := attrs.Get("service.namespace"); ok {
		namespace = v.AsString()
	}
	if v, ok := attrs.Get("service.instance.id"); ok {
		id = v.AsString()
	}
	return namespace + "|" + name.AsString() + "|" + id
}

// hashMapAttrs returns a deterministic hash of an attribute map, used as a
// fallback service identity when service.name is absent and as a grouping key
// for resources and scopes.
func hashMapAttrs(attrs pcommon.Map) string {
	h := pdatautil.MapHash(attrs)
	return hex.EncodeToString(h[:])
}

// subtraceMembers returns the spans belonging to the given subtrace, whether it
// is headed by a single local root or is a service's group of parentless spans.
func subtraceMembers(id subtraceID, idx *traceIndex) []*bufferedSpan {
	if !id.isOrphanGroup() {
		return descendFrom(id.spanID, idx)
	}

	// The group is whatever is currently parentless for this service. A span that
	// has since acquired a parent is no longer an orphan root, so it drops out of
	// the group here and is collected by the subtrace its parent belongs to.
	var members []*bufferedSpan
	for spanID, bs := range idx.spans {
		if bs.serviceID != id.serviceID || classifyRoot(bs, idx) != orphanRoot {
			continue
		}
		members = append(members, descendFrom(spanID, idx)...)
	}
	return members
}

// descendFrom returns every span that belongs to the subtrace rooted at
// rootID, including the root itself.
//
// It descends from the root through the children index, stopping wherever it
// meets another local root, since that span begins a subtrace of its own. The
// cost is therefore proportional to the subtrace being collected rather than to
// the whole trace.
func descendFrom(rootID pcommon.SpanID, idx *traceIndex) []*bufferedSpan {
	root, ok := idx.spans[rootID]
	if !ok {
		return nil
	}

	members := []*bufferedSpan{root}
	// Malformed input can make a span its own ancestor, which would otherwise
	// send this walk round the cycle forever.
	visited := map[pcommon.SpanID]bool{rootID: true}

	pending := []pcommon.SpanID{rootID}
	for len(pending) > 0 {
		parent := pending[len(pending)-1]
		pending = pending[:len(pending)-1]

		for _, childID := range idx.children[parent] {
			if visited[childID] {
				continue
			}
			child, ok := idx.spans[childID]
			if !ok {
				continue
			}
			if isLocalRoot(child, idx) {
				continue // begins its own subtrace
			}
			visited[childID] = true
			members = append(members, child)
			pending = append(pending, childID)
		}
	}

	return members
}

// assemble reconstructs a ptrace.Traces from a slice of bufferedSpans,
// coalescing spans that share the same (Resource, Scope) pair.
func assemble(members []*bufferedSpan) ptrace.Traces {
	td := ptrace.NewTraces()

	type rsKey struct {
		resource string
		scope    scopeKey
	}
	rsMap := map[rsKey]ptrace.ScopeSpans{}
	rsIndex := map[string]ptrace.ResourceSpans{}

	for _, bs := range members {
		key := rsKey{resource: bs.resourceKey, scope: bs.scopeKey}

		ss, found := rsMap[key]
		if !found {
			rs, ok := rsIndex[bs.resourceKey]
			if !ok {
				rs = td.ResourceSpans().AppendEmpty()
				bs.resource.CopyTo(rs.Resource())
				rsIndex[bs.resourceKey] = rs
			}
			ss = rs.ScopeSpans().AppendEmpty()
			bs.scope.CopyTo(ss.Scope())
			rsMap[key] = ss
		}

		dest := ss.Spans().AppendEmpty()
		bs.span.CopyTo(dest)
	}

	return td
}
