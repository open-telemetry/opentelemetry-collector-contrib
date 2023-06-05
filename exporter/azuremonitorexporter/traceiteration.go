// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
