// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package groupbytraceprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/groupbytraceprocessor"

import (
	"encoding/hex"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil"
)

// subtraceID identifies one service's spans within a distributed trace.
//
// A service entered more than once in a trace buffers under a single ID. The
// separate calls are told apart when the buffer is released, rather than as
// spans arrive, because until then the picture is still incomplete: a span may
// turn up before its parent, or its parent may never turn up at all.
type subtraceID struct {
	traceID   pcommon.TraceID
	serviceID string
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
// Every span under one ResourceSpans shares all of this, so it is built once per
// resource and shared by that resource's spans rather than being rebuilt for
// each scope or each span. The copy must not be mutated afterwards: it backs
// every bufferedSpan that shares it, and the derived keys would go stale.
type resourceContext struct {
	resource pcommon.Resource

	// serviceID is what spans are grouped by; resourceKey is what assemble groups
	// resources on when rebuilding a batch.
	serviceID   string
	resourceKey string
}

// newResourceContext deep-copies the resource so the result is self-contained
// and the caller can recycle its pdata objects, then derives the keys used to
// group the spans reported under it.
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
// describes everything a span carries beyond the span itself. Every span in one
// ScopeSpans shares it, under the same no-mutation rule.
type spanContext struct {
	resourceContext

	scope    pcommon.InstrumentationScope
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
// instrumentation scope it was reported under.
type bufferedSpan struct {
	spanContext
	span ptrace.Span

	// arrivedAt is when the span was buffered. Each call to a service waits out
	// wait_duration from its own first span, so a trace that comes back to a
	// service much later doesn't inherit the earlier visit's deadline.
	arrivedAt time.Time
}

// newBufferedSpan deep-copies the span so the caller can recycle its pdata
// objects. The context is shared as-is with the other spans reported under it.
func newBufferedSpan(ctx spanContext, span ptrace.Span) *bufferedSpan {
	spCopy := ptrace.NewSpan()
	span.CopyTo(spCopy)

	return &bufferedSpan{spanContext: ctx, span: spCopy, arrivedAt: time.Now()}
}

// firstArrival returns when the earliest of the given spans was buffered. A
// call's deadline runs from the first thing seen of it, which may be a child
// that arrived before the entry span explaining it.
func firstArrival(call []*bufferedSpan) time.Time {
	var first time.Time
	for _, bs := range call {
		if first.IsZero() || bs.arrivedAt.Before(first) {
			first = bs.arrivedAt
		}
	}
	return first
}

const (
	// spanFlagsContextHasIsRemoteMask is set when the IS_REMOTE flag is explicitly present.
	spanFlagsContextHasIsRemoteMask uint32 = 0x00000100
	// spanFlagsContextIsRemoteMask is set when the parent context came from a remote caller.
	spanFlagsContextIsRemoteMask uint32 = 0x00000200
)

// hasRemoteParent reports whether the span says its parent context arrived from
// another process. That makes it a service-entry span even when the parent
// carries the same service identity, which is what a service calling another
// instance of itself looks like.
func hasRemoteParent(bs *bufferedSpan) bool {
	flags := bs.span.Flags()
	return flags&spanFlagsContextHasIsRemoteMask != 0 && flags&spanFlagsContextIsRemoteMask != 0
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

// splitCalls divides one service's buffered spans into separate calls, so that
// a service entered more than once within a trace is not emitted as though it
// were entered once.
//
// A span heads a call when its parent is not among the service's own spans, or
// when it reports that the parent context was remote. Everything else descends
// from one of those entry spans.
//
// Spans whose parent is nowhere in the trace get best-effort treatment: nothing
// distinguishes one from another, so they leave together in a single call rather
// than one batch per span. That batch may therefore hold spans from more than
// one call to the service, which is the price of the information not being
// there; the alternative loses the grouping entirely.
//
// spanToService maps every span ID buffered for the trace to the service holding
// it, and is what tells "entered from another service" apart from "the parent
// never arrived".
func splitCalls(serviceSpans map[pcommon.SpanID]*bufferedSpan, spanToService map[pcommon.SpanID]string) [][]*bufferedSpan {
	children := make(map[pcommon.SpanID][]pcommon.SpanID)
	var entries, parentless []pcommon.SpanID

	for spanID, bs := range serviceSpans {
		parent := bs.span.ParentSpanID()
		if _, sameService := serviceSpans[parent]; sameService && !hasRemoteParent(bs) {
			children[parent] = append(children[parent], spanID)
			continue
		}
		if _, elsewhere := spanToService[parent]; parent.IsEmpty() || elsewhere || hasRemoteParent(bs) {
			entries = append(entries, spanID)
		} else {
			parentless = append(parentless, spanID)
		}
	}

	// An entry span is never recorded as anyone's child, so descending from one
	// can't wander into another call.
	visited := make(map[pcommon.SpanID]struct{}, len(serviceSpans))
	descend := func(roots []pcommon.SpanID) []*bufferedSpan {
		var call []*bufferedSpan
		pending := append([]pcommon.SpanID(nil), roots...)
		for len(pending) > 0 {
			spanID := pending[len(pending)-1]
			pending = pending[:len(pending)-1]
			if _, seen := visited[spanID]; seen {
				continue
			}
			visited[spanID] = struct{}{}
			call = append(call, serviceSpans[spanID])
			pending = append(pending, children[spanID]...)
		}
		return call
	}

	var calls [][]*bufferedSpan
	for _, entry := range entries {
		if call := descend([]pcommon.SpanID{entry}); len(call) > 0 {
			calls = append(calls, call)
		}
	}
	if call := descend(parentless); len(call) > 0 {
		calls = append(calls, call)
	}

	// Malformed input can make a span its own ancestor, leaving a ring that
	// nothing heads. Release it rather than hold it forever.
	var unreachable []pcommon.SpanID
	for spanID := range serviceSpans {
		if _, seen := visited[spanID]; !seen {
			unreachable = append(unreachable, spanID)
		}
	}
	if call := descend(unreachable); len(call) > 0 {
		calls = append(calls, call)
	}

	return calls
}

// assemble reconstructs a ptrace.Traces from a slice of bufferedSpans,
// coalescing spans that share the same (Resource, Scope) pair.
//
// It takes ownership of the spans: each is moved into the result rather than
// copied again, which halves the copying a span is put through on its way
// through the processor. Callers pass spans that storage has already handed
// over, and must not read them afterwards. The resource and scope are still
// copied, because bufferedSpans that share a resource or scope share the same
// pdata object and it may back other calls still buffered.
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

		bs.span.MoveTo(ss.Spans().AppendEmpty())
	}

	return td
}
