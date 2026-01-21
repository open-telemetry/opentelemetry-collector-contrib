// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ptraceutil // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector/internal/ptraceutil"

import (
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// MoveResourcesIf calls f sequentially for each ResourceSpans present in the first ptrace.Traces.
// If f returns true, the element is removed from the first ptrace.Traces and added to the second ptrace.Traces.
func MoveResourcesIf(from, to ptrace.Traces, f func(ptrace.ResourceSpans) bool) {
	from.ResourceSpans().RemoveIf(func(resourceSpans ptrace.ResourceSpans) bool {
		if !f(resourceSpans) {
			return false
		}
		resourceSpans.MoveTo(to.ResourceSpans().AppendEmpty())
		return true
	})
}

// CopyResourcesIf calls f sequentially for each ResourceSpans present in the first ptrace.Traces.
// If f returns true, the element is copied from the first ptrace.Traces to the second ptrace.Traces.
func CopyResourcesIf(from, to ptrace.Traces, f func(ptrace.ResourceSpans) bool) {
	for i := 0; i < from.ResourceSpans().Len(); i++ {
		rs := from.ResourceSpans().At(i)
		if f(rs) {
			rs.CopyTo(to.ResourceSpans().AppendEmpty())
		}
	}
}

// MoveSpansWithContextIf calls f sequentially for each Span present in the first ptrace.Traces.
// If f returns true, the element is removed from the first ptrace.Traces and added to the second ptrace.Traces.
// Notably, the Resource and Scope associated with the Span are created in the second ptrace.Traces only once.
// Resources or Scopes are removed from the original if they become empty. All ordering is preserved.
func MoveSpansWithContextIf(from, to ptrace.Traces, f func(ptrace.ResourceSpans, ptrace.ScopeSpans, ptrace.Span) bool) {
	resourceSpansSlice := from.ResourceSpans()
	resourceSpansSlice.RemoveIf(func(rs ptrace.ResourceSpans) bool {
		scopeSpanSlice := rs.ScopeSpans()
		var resourceSpansCopy *ptrace.ResourceSpans
		scopeSpanSlice.RemoveIf(func(ss ptrace.ScopeSpans) bool {
			spanSlice := ss.Spans()
			var scopeSpansCopy *ptrace.ScopeSpans
			spanSlice.RemoveIf(func(span ptrace.Span) bool {
				if !f(rs, ss, span) {
					return false
				}
				if resourceSpansCopy == nil {
					rmc := to.ResourceSpans().AppendEmpty()
					resourceSpansCopy = &rmc
					rs.Resource().CopyTo(resourceSpansCopy.Resource())
					resourceSpansCopy.SetSchemaUrl(rs.SchemaUrl())
				}
				if scopeSpansCopy == nil {
					smc := resourceSpansCopy.ScopeSpans().AppendEmpty()
					scopeSpansCopy = &smc
					ss.Scope().CopyTo(scopeSpansCopy.Scope())
					scopeSpansCopy.SetSchemaUrl(ss.SchemaUrl())
				}
				span.MoveTo(scopeSpansCopy.Spans().AppendEmpty())
				return true
			})
			return ss.Spans().Len() == 0
		})
		return rs.ScopeSpans().Len() == 0
	})
}

// CopySpansWithContextIf calls f sequentially for each Span present in the first ptrace.Traces.
// If f returns true, the element is copied from the first ptrace.Traces to the second ptrace.Traces.
// Notably, the Resource and Scope associated with the Span are created in the second ptrace.Traces only once.
func CopySpansWithContextIf(from, to ptrace.Traces, f func(ptrace.ResourceSpans, ptrace.ScopeSpans, ptrace.Span) bool) {
	for i := 0; i < from.ResourceSpans().Len(); i++ {
		rs := from.ResourceSpans().At(i)
		var resourceSpansCopy *ptrace.ResourceSpans
		for j := 0; j < rs.ScopeSpans().Len(); j++ {
			ss := rs.ScopeSpans().At(j)
			var scopeSpansCopy *ptrace.ScopeSpans
			for k := 0; k < ss.Spans().Len(); k++ {
				s := ss.Spans().At(k)
				if f(rs, ss, s) {
					if resourceSpansCopy == nil {
						rmc := to.ResourceSpans().AppendEmpty()
						resourceSpansCopy = &rmc
						rs.Resource().CopyTo(resourceSpansCopy.Resource())
						resourceSpansCopy.SetSchemaUrl(rs.SchemaUrl())
					}
					if scopeSpansCopy == nil {
						smc := resourceSpansCopy.ScopeSpans().AppendEmpty()
						scopeSpansCopy = &smc
						ss.Scope().CopyTo(scopeSpansCopy.Scope())
						scopeSpansCopy.SetSchemaUrl(ss.SchemaUrl())
					}
					s.CopyTo(scopeSpansCopy.Spans().AppendEmpty())
				}
			}
		}
	}
}
