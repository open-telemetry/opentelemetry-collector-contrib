// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuremonitorexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azuremonitorexporter"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

/*
	Encapsulates iteration over the Spans inside ptrace.Traces from the underlying representation.
	Everyone is doing the same kind of iteration and checking over a set traces.
*/

// TraceVisitor interface defines a iteration callback when walking through traces
type TraceVisitor interface {
	// Called for each tuple of Resource, InstrumentationScope, and Span
	// If Visit returns false, the iteration is short-circuited
	visit(resource pcommon.Resource, scope pcommon.InstrumentationScope, span ptrace.Span) (ok bool)
}

// Accept method is called to start the iteration process
func Accept(traces ptrace.Traces, v TraceVisitor) {
	resourceSpans := traces.ResourceSpans()

	// Walk each ResourceSpans instance
	for i := 0; i < resourceSpans.Len(); i++ {
		rs := resourceSpans.At(i)
		resource := rs.Resource()
		scopeSpansSlice := rs.ScopeSpans()

		for j := 0; j < scopeSpansSlice.Len(); j++ {
			scopeSpans := scopeSpansSlice.At(j)
			// instrumentation library is optional
			scope := scopeSpans.Scope()
			spansSlice := scopeSpans.Spans()
			if spansSlice.Len() == 0 {
				continue
			}

			for k := 0; k < spansSlice.Len(); k++ {
				if ok := v.visit(resource, scope, spansSlice.At(k)); !ok {
					return
				}
			}
		}
	}
}
