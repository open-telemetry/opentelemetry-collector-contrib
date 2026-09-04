// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package groupbytraceprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/groupbytraceprocessor"

import (
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// subtraceID is the composite key that uniquely identifies one service's subtrace
// within a distributed trace.
type subtraceID struct {
	traceID pcommon.TraceID
	spanID  pcommon.SpanID // the local root span of this subtrace
}

// bufferedSpan holds a deep copy of a single span together with its resource
// and instrumentation scope, enabling span-level indexing.
type bufferedSpan struct {
	resource pcommon.Resource
	scope    pcommon.InstrumentationScope
	span     ptrace.Span
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
func isLocalRoot(bs bufferedSpan, index map[pcommon.SpanID]bufferedSpan) bool {
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
	parent, ok := index[bs.span.ParentSpanID()]
	if !ok {
		return true
	}
	return serviceIdentity(bs.resource) != serviceIdentity(parent.resource)
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

// hashMapAttrs returns a deterministic string representation of all resource
// attributes, used as a fallback service identity when service.name is absent.
func hashMapAttrs(attrs pcommon.Map) string {
	// Build a sorted representation via the JSON marshaller baked into pdata.
	m := pcommon.NewMap()
	attrs.CopyTo(m)
	return fmt.Sprintf("%v", m.AsRaw())
}

// reaches reports whether the span identified by spanID is a member of the
// subtrace rooted at targetRootID. It walks up the parent chain, stopping when
// it finds the target root or when it crosses another local-root boundary.
func reaches(spanID, targetRootID pcommon.SpanID, index map[pcommon.SpanID]bufferedSpan) bool {
	cur, ok := index[spanID]
	if !ok {
		return false
	}
	visited := map[pcommon.SpanID]bool{spanID: true}
	for {
		if cur.span.SpanID() == targetRootID {
			return true
		}
		if isLocalRoot(cur, index) {
			return false // hit a different subtree boundary
		}
		pid := cur.span.ParentSpanID()
		if pid.IsEmpty() {
			return false
		}
		if visited[pid] {
			return false // cycle detected
		}
		visited[pid] = true
		parent, ok := index[pid]
		if !ok {
			return false
		}
		cur = parent
	}
}

// assemble reconstructs a ptrace.Traces from a slice of bufferedSpans,
// coalescing spans that share the same (Resource, Scope) pair.
func assemble(members []bufferedSpan) ptrace.Traces {
	td := ptrace.NewTraces()

	// Use string keys to group by (resource hash, scope hash).
	type rsKey struct{ resource, scope string }
	rsMap := map[rsKey]ptrace.ScopeSpans{}
	rsIndex := map[string]ptrace.ResourceSpans{}

	for _, bs := range members {
		rk := hashResource(bs.resource)
		sk := hashScope(bs.scope)
		key := rsKey{rk, sk}

		ss, found := rsMap[key]
		if !found {
			var rs ptrace.ResourceSpans
			if existing, ok := rsIndex[rk]; ok {
				rs = existing
			} else {
				rs = td.ResourceSpans().AppendEmpty()
				bs.resource.CopyTo(rs.Resource())
				rsIndex[rk] = rs
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

// hashResource returns a string key for a resource, used for grouping.
func hashResource(r pcommon.Resource) string {
	return fmt.Sprintf("%v", r.Attributes().AsRaw())
}

// hashScope returns a string key for an instrumentation scope, used for grouping.
func hashScope(s pcommon.InstrumentationScope) string {
	return fmt.Sprintf("%s@%s|%v", s.Name(), s.Version(), s.Attributes().AsRaw())
}
