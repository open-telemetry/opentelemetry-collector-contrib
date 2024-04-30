// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter"

import (
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// mergeTraces concatenates two ptrace.Traces into a single ptrace.Traces.
func mergeTraces(t1 ptrace.Traces, t2 ptrace.Traces) ptrace.Traces {
	mergedTraces := ptrace.NewTraces()

	if t1.SpanCount() == 0 && t2.SpanCount() == 0 {
		return mergedTraces
	}

	// Iterate over the first trace and append spans to the merged traces
	for i := 0; i < t1.ResourceSpans().Len(); i++ {
		rs := t1.ResourceSpans().At(i)
		newRS := mergedTraces.ResourceSpans().AppendEmpty()

		rs.Resource().MoveTo(newRS.Resource())
		newRS.SetSchemaUrl(rs.SchemaUrl())

		for j := 0; j < rs.ScopeSpans().Len(); j++ {
			ils := rs.ScopeSpans().At(j)

			newILS := newRS.ScopeSpans().AppendEmpty()
			ils.Scope().MoveTo(newILS.Scope())
			newILS.SetSchemaUrl(ils.SchemaUrl())

			for k := 0; k < ils.Spans().Len(); k++ {
				span := ils.Spans().At(k)
				newSpan := newILS.Spans().AppendEmpty()
				span.MoveTo(newSpan)
			}
		}
	}

	// Iterate over the second trace and append spans to the merged traces
	for i := 0; i < t2.ResourceSpans().Len(); i++ {
		rs := t2.ResourceSpans().At(i)
		newRS := mergedTraces.ResourceSpans().AppendEmpty()

		rs.Resource().MoveTo(newRS.Resource())
		newRS.SetSchemaUrl(rs.SchemaUrl())

		for j := 0; j < rs.ScopeSpans().Len(); j++ {
			ils := rs.ScopeSpans().At(j)

			newILS := newRS.ScopeSpans().AppendEmpty()
			ils.Scope().MoveTo(newILS.Scope())
			newILS.SetSchemaUrl(ils.SchemaUrl())

			for k := 0; k < ils.Spans().Len(); k++ {
				span := ils.Spans().At(k)
				newSpan := newILS.Spans().AppendEmpty()
				span.MoveTo(newSpan)
			}
		}
	}

	return mergedTraces
}
