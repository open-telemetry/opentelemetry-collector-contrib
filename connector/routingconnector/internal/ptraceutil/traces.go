// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ptraceutil // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector/internal/ptraceutil"

import "go.opentelemetry.io/collector/pdata/ptrace"

// MoveResourcesIf calls f sequentially for each ResourceSpans present in the first ptrace.Traces.
// If f returns true, the element is removed from the first ptrace.Traces and added to the second ptrace.Traces.
func MoveResourcesIf(from, to ptrace.Traces, f func(ptrace.ResourceSpans) bool) {
	from.ResourceSpans().RemoveIf(func(resoruceSpans ptrace.ResourceSpans) bool {
		if !f(resoruceSpans) {
			return false
		}
		resoruceSpans.CopyTo(to.ResourceSpans().AppendEmpty())
		return true
	})
}

// MoveSpansWithContextIf calls f sequentially for each Span present in the first ptrace.Traces.
// If f returns true, the element is removed from the first ptrace.Traces and added to the second ptrace.Traces.
// Notably, the Resource and Scope associated with the Span are created in the second ptrace.Traces only once.
// Resources or Scopes are removed from the original if they become empty. All ordering is preserved.
func MoveSpansWithContextIf(from, to ptrace.Traces, f func(ptrace.ResourceSpans, ptrace.ScopeSpans, ptrace.Span) bool) {
	resourceSpansSlice := from.ResourceSpans()
	for i := 0; i < resourceSpansSlice.Len(); i++ {
		resourceSpans := resourceSpansSlice.At(i)
		scopeSpanSlice := resourceSpans.ScopeSpans()
		var resourceSpansCopy *ptrace.ResourceSpans
		for j := 0; j < scopeSpanSlice.Len(); j++ {
			scopeSpans := scopeSpanSlice.At(j)
			spanSlice := scopeSpans.Spans()
			var scopeSpansCopy *ptrace.ScopeSpans
			spanSlice.RemoveIf(func(span ptrace.Span) bool {
				if !f(resourceSpans, scopeSpans, span) {
					return false
				}
				if resourceSpansCopy == nil {
					rmc := to.ResourceSpans().AppendEmpty()
					resourceSpansCopy = &rmc
					resourceSpans.Resource().CopyTo(resourceSpansCopy.Resource())
					resourceSpansCopy.SetSchemaUrl(resourceSpans.SchemaUrl())
				}
				if scopeSpansCopy == nil {
					smc := resourceSpansCopy.ScopeSpans().AppendEmpty()
					scopeSpansCopy = &smc
					scopeSpans.Scope().CopyTo(scopeSpansCopy.Scope())
					scopeSpansCopy.SetSchemaUrl(scopeSpans.SchemaUrl())
				}
				span.CopyTo(scopeSpansCopy.Spans().AppendEmpty())
				return true
			})
		}
		scopeSpanSlice.RemoveIf(func(sm ptrace.ScopeSpans) bool {
			return sm.Spans().Len() == 0
		})
	}
	resourceSpansSlice.RemoveIf(func(resourceSpans ptrace.ResourceSpans) bool {
		return resourceSpans.ScopeSpans().Len() == 0
	})
}
