// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package batchpertrace

import "go.opentelemetry.io/collector/consumer/pdata"

// Split returns one pdata.Traces for each trace in the given pdata.Traces input. Each of the resulting pdata.Traces contains exactly one trace.
func Split(batch pdata.Traces) []pdata.Traces {
	// for each span in the resource spans, we group them into batches of rs/ils/traceID.
	// if the same traceID exists in different ils, they land in different batches.
	var result []pdata.Traces

	for i := 0; i < batch.ResourceSpans().Len(); i++ {
		rs := batch.ResourceSpans().At(i)

		for j := 0; j < rs.InstrumentationLibrarySpans().Len(); j++ {
			// the batches for this ILS
			batches := map[[16]byte]pdata.ResourceSpans{}

			ils := rs.InstrumentationLibrarySpans().At(j)
			for k := 0; k < ils.Spans().Len(); k++ {
				span := ils.Spans().At(k)
				key := span.TraceID().Bytes()

				// for the first traceID in the ILS, initialize the map entry
				// and add the singleTraceBatch to the result list
				if _, ok := batches[key]; !ok {
					newRS := pdata.NewResourceSpans()
					// currently, the ResourceSpans implementation has only a Resource and an ILS. We'll copy the Resource
					// and set our own ILS
					rs.Resource().CopyTo(newRS.Resource())

					newILS := pdata.NewInstrumentationLibrarySpans()
					// currently, the ILS implementation has only an InstrumentationLibrary and spans. We'll copy the library
					// and set our own spans
					ils.InstrumentationLibrary().CopyTo(newILS.InstrumentationLibrary())
					newRS.InstrumentationLibrarySpans().Append(newILS)
					batches[key] = newRS

					trace := pdata.NewTraces()
					trace.ResourceSpans().Append(newRS)

					result = append(result, trace)
				}

				// there is only one instrumentation library per batch
				batches[key].InstrumentationLibrarySpans().At(0).Spans().Append(span)
			}
		}
	}

	return result
}
