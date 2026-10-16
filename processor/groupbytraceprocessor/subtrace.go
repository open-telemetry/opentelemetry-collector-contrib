// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package groupbytraceprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/groupbytraceprocessor"

import (
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/xpdata/xhash"
)

// subtraceID identifies one service's spans within a distributed trace.
//
// A service entered more than once in a trace buffers under a single ID. The
// separate calls are told apart when the buffer is released, rather than as
// spans arrive, because until then the picture is still incomplete: a span may
// turn up before its parent, or its parent may never turn up at all.
type subtraceID struct {
	traceID   pcommon.TraceID
	serviceID [16]byte
}

// scopeKey identifies an instrumentation scope for grouping purposes. The parts
// are kept separate rather than concatenated so that, for instance, the scope
// named "lib@2" with no version can't collide with "lib" at version "2".
type scopeKey struct {
	name             string
	version          string
	attrHash         [16]byte
	schemaURL        string
	droppedAttrCount uint32
}

// resourceContext holds the resource a span was reported under, together with
// the keys derived from it. The copy must not be mutated: it backs every
// bufferedSpan that shares it, and the derived keys would go stale.
type resourceContext struct {
	resource  pcommon.Resource
	schemaURL string

	// serviceID is what spans are grouped by; resourceKey is what assemble groups
	// resources on when rebuilding a batch.
	serviceID   [16]byte
	resourceKey [16]byte
}

// newResourceContext deep-copies the resource so the caller may recycle its batch.
func newResourceContext(rs ptrace.ResourceSpans) resourceContext {
	rCopy := pcommon.NewResource()
	rs.Resource().CopyTo(rCopy)
	attrHash := xhash.MapHash(rCopy.Attributes())

	return resourceContext{
		resource:    rCopy,
		schemaURL:   rs.SchemaUrl(),
		serviceID:   serviceIdentity(rCopy.Attributes(), attrHash),
		resourceKey: attrHash,
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

// newSpanContext deep-copies the scope; the resourceContext is shared as-is
// across all scopes under the same resource, so the no-mutation rule applies.
func newSpanContext(rctx resourceContext, scope pcommon.InstrumentationScope, schemaURL string) *spanContext {
	sCopy := pcommon.NewInstrumentationScope()
	scope.CopyTo(sCopy)

	return &spanContext{
		resourceContext: rctx,
		scope:           sCopy,
		scopeKey: scopeKey{
			name:             sCopy.Name(),
			version:          sCopy.Version(),
			attrHash:         xhash.MapHash(sCopy.Attributes()),
			schemaURL:        schemaURL,
			droppedAttrCount: sCopy.DroppedAttributesCount(),
		},
	}
}

// bufferedSpan holds a deep copy of a single span together with the resource and
// instrumentation scope it was reported under.
type bufferedSpan struct {
	*spanContext
	span ptrace.Span

	// arrivedAt is when the batch carrying the span was received. Each call to a
	// service waits out wait_duration from its own first span, so a trace that
	// comes back to a service much later doesn't inherit the earlier visit's
	// deadline.
	arrivedAt time.Time
}

// newBufferedSpan deep-copies the span so the caller can recycle its pdata
// objects.
func newBufferedSpan(ctx *spanContext, span ptrace.Span, arrivedAt time.Time) *bufferedSpan {
	spCopy := ptrace.NewSpan()
	span.CopyTo(spCopy)

	return &bufferedSpan{spanContext: ctx, span: spCopy, arrivedAt: arrivedAt}
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

func hasRemoteParent(bs *bufferedSpan) bool {
	flags := bs.span.Flags()
	return flags&spanFlagsContextHasIsRemoteMask != 0 && flags&spanFlagsContextIsRemoteMask != 0
}

func serviceIdentity(attrs pcommon.Map, attrHash [16]byte) [16]byte {
	name, hasName := attrs.Get("service.name")
	if !hasName {
		return attrHash
	}
	var namespace, id string
	if v, ok := attrs.Get("service.namespace"); ok {
		namespace = v.AsString()
	}
	if v, ok := attrs.Get("service.instance.id"); ok {
		id = v.AsString()
	}
	return xhash.Hash(xhash.WithString(namespace), xhash.WithString(name.AsString()), xhash.WithString(id))
}

const (
	// callInProgress marks a span whose call is currently being resolved. Meeting
	// one means the walk has come back on itself.
	callInProgress = -1
	// callParentless collects the spans whose parent is nowhere in the trace.
	callParentless = -2
	// callUnreachable collects spans that no entry span can account for, which
	// malformed input produces by making a span its own ancestor.
	callUnreachable = -3
)

// splitCalls divides one service's buffered spans into separate calls, so that
// a service entered more than once within a trace is not emitted as though it
// were entered once.
//
// A span heads a call when its parent is not among the service's own spans, or
// when it reports that the parent context was remote. Every other span belongs
// to the call of whichever such span it descends from.
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
func splitCalls(serviceSpans map[pcommon.SpanID][]*bufferedSpan, spanToService map[pcommon.SpanID][16]byte) [][]*bufferedSpan {
	callOf := make(map[pcommon.SpanID]int32, len(serviceSpans))
	var calls [][]*bufferedSpan

	// path holds the spans walked over on the way to an answer, so that all of
	// them can be given it at once. It is reused across walks.
	var path []pcommon.SpanID

	for spanID := range serviceSpans {
		if _, resolved := callOf[spanID]; resolved {
			continue
		}

		path = path[:0]
		var call int32
		for cur := spanID; ; {
			if idx, resolved := callOf[cur]; resolved {
				// Coming back to a span still being resolved means the parent links
				// form a ring, so nothing on this path descends from an entry span.
				call = idx
				if idx == callInProgress {
					call = callUnreachable
				}
				break
			}

			// Use the most recently arrived copy to determine the call.
			bsList := serviceSpans[cur]
			bs := bsList[len(bsList)-1]
			parent := bs.span.ParentSpanID()
			// Guard against a buffered empty-ID span acting as the parent of every
			// legitimate root span (which also has an empty ParentSpanID).
			if !parent.IsEmpty() {
				if _, sameService := serviceSpans[parent]; sameService && !hasRemoteParent(bs) {
					callOf[cur] = callInProgress
					path = append(path, cur)
					cur = parent
					continue
				}
			}

			// cur heads a call, either its own or the parentless one.
			_, elsewhere := spanToService[parent]
			if parent.IsEmpty() || elsewhere || hasRemoteParent(bs) {
				call = int32(len(calls))
				calls = append(calls, nil)
			} else {
				call = callParentless
			}
			callOf[cur] = call
			break
		}

		for _, walked := range path {
			callOf[walked] = call
		}
	}

	var parentless, unreachable []*bufferedSpan
	for spanID, bsList := range serviceSpans {
		switch call := callOf[spanID]; call {
		case callParentless:
			parentless = append(parentless, bsList...)
		case callUnreachable, callInProgress:
			unreachable = append(unreachable, bsList...)
		default:
			calls[call] = append(calls[call], bsList...)
		}
	}
	if len(parentless) > 0 {
		calls = append(calls, parentless)
	}
	if len(unreachable) > 0 {
		calls = append(calls, unreachable)
	}

	return calls
}

// assemble reconstructs a ptrace.Traces from a slice of bufferedSpans,
// coalescing spans that share the same (Resource, Scope) pair.
//
// It takes ownership of the spans: each is moved rather than copied again,
// halving the copies a span goes through. Callers must not read spans
// afterwards. The resource and scope are still copied because bufferedSpans
// sharing them point to the same pdata object, which may back other calls
// still buffered.
func assemble(members []*bufferedSpan) ptrace.Traces {
	td := ptrace.NewTraces()

	type rsKey struct {
		resource     [16]byte
		schemaURL    string
		droppedAttrs uint32
		scope        scopeKey
	}
	rsMap := map[rsKey]ptrace.ScopeSpans{}
	type rsIndexKey struct {
		resource     [16]byte
		schemaURL    string
		droppedAttrs uint32
	}
	rsIndex := map[rsIndexKey]ptrace.ResourceSpans{}

	for _, bs := range members {
		key := rsKey{resource: bs.resourceKey, schemaURL: bs.schemaURL, droppedAttrs: bs.resource.DroppedAttributesCount(), scope: bs.scopeKey}

		ss, found := rsMap[key]
		if !found {
			idxKey := rsIndexKey{resource: bs.resourceKey, schemaURL: bs.schemaURL, droppedAttrs: bs.resource.DroppedAttributesCount()}
			rs, ok := rsIndex[idxKey]
			if !ok {
				rs = td.ResourceSpans().AppendEmpty()
				bs.resource.CopyTo(rs.Resource())
				rs.SetSchemaUrl(bs.schemaURL)
				rsIndex[idxKey] = rs
			}
			ss = rs.ScopeSpans().AppendEmpty()
			bs.scope.CopyTo(ss.Scope())
			ss.SetSchemaUrl(bs.scopeKey.schemaURL)
			rsMap[key] = ss
		}

		bs.span.MoveTo(ss.Spans().AppendEmpty())
	}

	return td
}
