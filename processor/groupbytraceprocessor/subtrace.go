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

// subtraceID is the composite key that uniquely identifies one service's subtrace
// within a distributed trace.
type subtraceID struct {
	traceID pcommon.TraceID
	spanID  pcommon.SpanID // the local root span of this subtrace
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
//
// Every span under one ResourceSpans shares all of this, whatever scope it came
// from, so it is built once per resource and shared by that resource's spans
// rather than being rebuilt for each scope or each span. The copy must not be
// mutated afterwards: it backs every bufferedSpan that shares it, and the
// derived keys would go stale.
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
//
// Every span in one ScopeSpans shares this, so it is built once per scope and
// shared by that scope's spans. The same no-mutation rule as resourceContext
// applies to the scope copy.
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
}

// newBufferedSpan deep-copies the span so the caller can recycle its pdata
// objects. The context is shared as-is with the other spans reported under it.
//
// The result is a pointer because bufferedSpan is large enough that storing it
// in the span index by value, and copying it out again on every lookup, costs
// more than the indirection: the index is walked once per span on insert and
// once per hop when reassembling a subtrace.
func newBufferedSpan(ctx spanContext, span ptrace.Span) *bufferedSpan {
	spCopy := ptrace.NewSpan()
	span.CopyTo(spCopy)

	return &bufferedSpan{spanContext: ctx, span: spCopy}
}

// traceIndex holds every span buffered for one trace, keyed by span ID, plus a
// parent-to-children mapping.
//
// The children mapping is what lets a subtrace be collected by walking down from
// its local root. Walking up from every span in the trace instead costs
// O(spans x depth) per local root, which for a deep trace, such as a long
// sequential pipeline, degrades into quadratic behaviour.
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
	if _, replaced := idx.spans[spanID]; !replaced {
		if parent := bs.span.ParentSpanID(); !parent.IsEmpty() {
			idx.children[parent] = append(idx.children[parent], spanID)
		}
	}
	idx.spans[spanID] = bs
}

// remove drops the given spans and every reference to them.
func (idx *traceIndex) remove(spans []*bufferedSpan) {
	for _, bs := range spans {
		spanID := bs.span.SpanID()
		delete(idx.spans, spanID)
		delete(idx.children, spanID)

		// Unlink from the parent as well, which matters when the parent outlives
		// this span. Members removed together take their own entries with them, so
		// this only ever prunes at the edge of the removed set.
		parent := bs.span.ParentSpanID()
		if parent.IsEmpty() {
			continue
		}
		siblings, ok := idx.children[parent]
		if !ok {
			continue
		}
		siblings = slices.DeleteFunc(siblings, func(id pcommon.SpanID) bool { return id == spanID })
		if len(siblings) == 0 {
			delete(idx.children, parent)
		} else {
			idx.children[parent] = siblings
		}
	}
}

func (idx *traceIndex) len() int {
	return len(idx.spans)
}

const (
	// spanFlagsContextHasIsRemoteMask is set when the IS_REMOTE flag is explicitly present.
	spanFlagsContextHasIsRemoteMask uint32 = 0x00000100
	// spanFlagsContextIsRemoteMask is set when the parent context came from a remote caller.
	spanFlagsContextIsRemoteMask uint32 = 0x00000200
)

// isLocalRoot returns true if bs is the service-entry span for its subtrace.
// A span is a local root when:
//   - its parent span ID is empty (global root), OR
//   - the IS_REMOTE flag is set (parent is in another service), OR
//   - its parent is not in the index (safe default: treat as local root), OR
//   - its parent belongs to a different service identity.
func isLocalRoot(bs *bufferedSpan, idx *traceIndex) bool {
	if bs.span.ParentSpanID().IsEmpty() {
		return true
	}
	flags := bs.span.Flags()
	// If IS_REMOTE is set, consider it authoritative.
	// If it is not set, default to treating this span as a local root if any of the
	// remaining checks are met.
	if flags&spanFlagsContextHasIsRemoteMask != 0 && flags&spanFlagsContextIsRemoteMask != 0 {
		return true
	}
	parent, ok := idx.spans[bs.span.ParentSpanID()]
	if !ok {
		return true
	}
	return bs.serviceID != parent.serviceID
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

// subtraceMembers returns every span that belongs to the subtrace rooted at
// rootID, including the root itself.
//
// It descends from the root through the children index, stopping wherever it
// meets another local root, since that span begins a subtrace of its own. The
// cost is therefore proportional to the subtrace being collected rather than to
// the whole trace.
func subtraceMembers(rootID pcommon.SpanID, idx *traceIndex) []*bufferedSpan {
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
