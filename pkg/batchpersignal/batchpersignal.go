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

package batchpersignal

import "go.opentelemetry.io/collector/model/pdata"

// SplitTraces returns one pdata.Traces for each trace in the given pdata.Traces input. Each of the resulting pdata.Traces contains exactly one trace.
func SplitTraces(batch pdata.Traces) []pdata.Traces {
	// for each span in the resource spans, we group them into batches of rs/ils/traceID.
	// if the same traceID exists in different ils, they land in different batches.
	var result []pdata.Traces

	for i := 0; i < batch.ResourceSpans().Len(); i++ {
		rs := batch.ResourceSpans().At(i)

		for j := 0; j < rs.InstrumentationLibrarySpans().Len(); j++ {
			// the batches for this ILS
			batches := map[pdata.TraceID]pdata.ResourceSpans{}

			ils := rs.InstrumentationLibrarySpans().At(j)
			for k := 0; k < ils.Spans().Len(); k++ {
				span := ils.Spans().At(k)
				key := span.TraceID()

				// for the first traceID in the ILS, initialize the map entry
				// and add the singleTraceBatch to the result list
				if _, ok := batches[key]; !ok {
					trace := pdata.NewTraces()
					newRS := trace.ResourceSpans().AppendEmpty()
					// currently, the ResourceSpans implementation has only a Resource and an ILS. We'll copy the Resource
					// and set our own ILS
					rs.Resource().CopyTo(newRS.Resource())

					newILS := newRS.InstrumentationLibrarySpans().AppendEmpty()
					// currently, the ILS implementation has only an InstrumentationLibrary and spans. We'll copy the library
					// and set our own spans
					ils.InstrumentationLibrary().CopyTo(newILS.InstrumentationLibrary())
					batches[key] = newRS

					result = append(result, trace)
				}

				// there is only one instrumentation library per batch
				tgt := batches[key].InstrumentationLibrarySpans().At(0).Spans().AppendEmpty()
				span.CopyTo(tgt)
			}
		}
	}

	return result
}

// SplitLogs returns one pdata.Logs for each trace in the given pdata.Logs input. Each of the resulting pdata.Logs contains exactly one trace.
func SplitLogs(batch pdata.Logs) []pdata.Logs {
	// for each log in the resource logs, we group them into batches of rl/ill/traceID.
	// if the same traceID exists in different ill, they land in different batches.
	var result []pdata.Logs

	for i := 0; i < batch.ResourceLogs().Len(); i++ {
		rs := batch.ResourceLogs().At(i)

		for j := 0; j < rs.InstrumentationLibraryLogs().Len(); j++ {
			// the batches for this ILL
			batches := map[pdata.TraceID]pdata.ResourceLogs{}

			ill := rs.InstrumentationLibraryLogs().At(j)
			for k := 0; k < ill.Logs().Len(); k++ {
				log := ill.Logs().At(k)
				key := log.TraceID()

				// for the first traceID in the ILL, initialize the map entry
				// and add the singleTraceBatch to the result list
				if _, ok := batches[key]; !ok {
					logs := pdata.NewLogs()
					newRL := logs.ResourceLogs().AppendEmpty()
					// currently, the ResourceLogs implementation has only a Resource and an ILL. We'll copy the Resource
					// and set our own ILL
					rs.Resource().CopyTo(newRL.Resource())

					newILL := newRL.InstrumentationLibraryLogs().AppendEmpty()
					// currently, the ILL implementation has only an InstrumentationLibrary and logs. We'll copy the library
					// and set our own logs
					ill.InstrumentationLibrary().CopyTo(newILL.InstrumentationLibrary())
					batches[key] = newRL

					result = append(result, logs)
				}

				// there is only one instrumentation library per batch
				tgt := batches[key].InstrumentationLibraryLogs().At(0).Logs().AppendEmpty()
				log.CopyTo(tgt)
			}
		}
	}

	return result
}
