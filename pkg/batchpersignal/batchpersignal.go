// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package batchpersignal // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/batchpersignal"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// SplitTraces returns one ptrace.Traces for each trace in the given ptrace.Traces input. Each of the resulting ptrace.Traces contains exactly one trace.
func SplitTraces(batch ptrace.Traces) []ptrace.Traces {
	// for each span in the resource spans, we group them into batches of rs/ils/traceID.
	// if the same traceID exists in different ils, they land in different batches.
	var result []ptrace.Traces

	for i := 0; i < batch.ResourceSpans().Len(); i++ {
		rs := batch.ResourceSpans().At(i)

		for j := 0; j < rs.ScopeSpans().Len(); j++ {
			// the batches for this ILS
			batches := map[pcommon.TraceID]ptrace.ResourceSpans{}

			ils := rs.ScopeSpans().At(j)
			for k := 0; k < ils.Spans().Len(); k++ {
				span := ils.Spans().At(k)
				key := span.TraceID()

				// for the first traceID in the ILS, initialize the map entry
				// and add the singleTraceBatch to the result list
				if _, ok := batches[key]; !ok {
					trace := ptrace.NewTraces()
					newRS := trace.ResourceSpans().AppendEmpty()
					// currently, the ResourceSpans implementation has only a Resource and an ILS. We'll copy the Resource
					// and set our own ILS
					rs.Resource().CopyTo(newRS.Resource())
					newRS.SetSchemaUrl(rs.SchemaUrl())
					newILS := newRS.ScopeSpans().AppendEmpty()
					// currently, the ILS implementation has only an InstrumentationLibrary and spans. We'll copy the library
					// and set our own spans
					ils.Scope().CopyTo(newILS.Scope())
					newILS.SetSchemaUrl(ils.SchemaUrl())
					batches[key] = newRS

					result = append(result, trace)
				}

				// there is only one instrumentation library per batch
				tgt := batches[key].ScopeSpans().At(0).Spans().AppendEmpty()
				span.CopyTo(tgt)
			}
		}
	}

	return result
}

// SplitLogs returns one plog.Logs for each trace in the given plog.Logs input. Each of the resulting plog.Logs contains exactly one log.
func SplitLogs(batch plog.Logs) []plog.Logs {
	// for each log in the resource logs, we group them into batches of rl/sl/traceID.
	// if the same traceID exists in different sl, they land in different batches.
	var result []plog.Logs

	for i := 0; i < batch.ResourceLogs().Len(); i++ {
		rs := batch.ResourceLogs().At(i)

		for j := 0; j < rs.ScopeLogs().Len(); j++ {
			// the batches for this ILL
			batches := map[pcommon.TraceID]plog.ResourceLogs{}

			sl := rs.ScopeLogs().At(j)
			for k := 0; k < sl.LogRecords().Len(); k++ {
				log := sl.LogRecords().At(k)
				key := log.TraceID()

				// for the first traceID in the ILL, initialize the map entry
				// and add the singleTraceBatch to the result list
				if _, ok := batches[key]; !ok {
					logs := plog.NewLogs()
					newRL := logs.ResourceLogs().AppendEmpty()
					// currently, the ResourceLogs implementation has only a Resource and an ILL. We'll copy the Resource
					// and set our own ILL
					rs.Resource().CopyTo(newRL.Resource())
					newRL.SetSchemaUrl(rs.SchemaUrl())
					newILL := newRL.ScopeLogs().AppendEmpty()
					// currently, the ILL implementation has only an InstrumentationLibrary and logs. We'll copy the library
					// and set our own logs
					sl.Scope().CopyTo(newILL.Scope())
					newILL.SetSchemaUrl(sl.SchemaUrl())
					batches[key] = newRL

					result = append(result, logs)
				}

				// there is only one instrumentation library per batch
				tgt := batches[key].ScopeLogs().At(0).LogRecords().AppendEmpty()
				log.CopyTo(tgt)
			}
		}
	}

	return result
}
