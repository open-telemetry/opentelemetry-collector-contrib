// Copyright OpenTelemetry Authors
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

package azuremonitorexporter

import "go.opentelemetry.io/collector/model/pdata"

/*
	Encapsulates iteration over the Spans inside pdata.Traces from the underlying representation.
	Everyone is doing the same kind of iteration and checking over a set traces.
*/

// TraceVisitor interface defines a iteration callback when walking through traces
type TraceVisitor interface {
	// Called for each tuple of Resource, InstrumentationLibrary, and Span
	// If Visit returns false, the iteration is short-circuited
	visit(resource pdata.Resource, instrumentationLibrary pdata.InstrumentationLibrary, span pdata.Span) (ok bool)
}

// Accept method is called to start the iteration process
func Accept(traces pdata.Traces, v TraceVisitor) {
	resourceSpans := traces.ResourceSpans()

	// Walk each ResourceSpans instance
	for i := 0; i < resourceSpans.Len(); i++ {
		rs := resourceSpans.At(i)
		resource := rs.Resource()
		instrumentationLibrarySpansSlice := rs.InstrumentationLibrarySpans()

		for j := 0; j < instrumentationLibrarySpansSlice.Len(); j++ {
			instrumentationLibrarySpans := instrumentationLibrarySpansSlice.At(j)
			// instrumentation library is optional
			instrumentationLibrary := instrumentationLibrarySpans.InstrumentationLibrary()
			spansSlice := instrumentationLibrarySpans.Spans()
			if spansSlice.Len() == 0 {
				continue
			}

			for k := 0; k < spansSlice.Len(); k++ {
				if ok := v.visit(resource, instrumentationLibrary, spansSlice.At(k)); !ok {
					return
				}
			}
		}
	}
}
