// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter

import (
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// mergeTraces concatenates multiple ptrace.Traces into a single ptrace.Traces.
func mergeTraces(traces ...ptrace.Traces) ptrace.Traces {
	merged := ptrace.NewTraces()
	empty := ptrace.Traces{}

	for _, trace := range traces {

		if trace == empty {
			continue
		}

		for i := 0; i < trace.ResourceSpans().Len(); i++ {
			rs := trace.ResourceSpans().At(i)

			newRS := merged.ResourceSpans().AppendEmpty()
			rs.Resource().CopyTo(newRS.Resource())
			newRS.SetSchemaUrl(rs.SchemaUrl())

			for j := 0; j < rs.ScopeSpans().Len(); j++ {
				ils := rs.ScopeSpans().At(j)

				newILS := newRS.ScopeSpans().AppendEmpty()
				ils.Scope().CopyTo(newILS.Scope())
				newILS.SetSchemaUrl(ils.SchemaUrl())

				for k := 0; k < ils.Spans().Len(); k++ {
					span := ils.Spans().At(k)
					newSpan := newILS.Spans().AppendEmpty()
					span.CopyTo(newSpan)
				}
			}
		}

	}

	return merged
}

// mergeTraces concatenates multiple pmetric.Metrics into a single pmetric.Metrics.
func mergeMetrics(metrics ...pmetric.Metrics) pmetric.Metrics {
	merged := pmetric.NewMetrics()
	empty := pmetric.Metrics{}

	for _, metric := range metrics {

		if metric == empty {
			continue
		}

		for i := 0; i < metric.ResourceMetrics().Len(); i++ {
			rs := metric.ResourceMetrics().At(i)

			newRM := merged.ResourceMetrics().AppendEmpty()
			rs.Resource().CopyTo(newRM.Resource())
			newRM.SetSchemaUrl(rs.SchemaUrl())

			for j := 0; j < rs.ScopeMetrics().Len(); j++ {
				ilm := rs.ScopeMetrics().At(j)

				newILM := newRM.ScopeMetrics().AppendEmpty()
				ilm.Scope().CopyTo(newILM.Scope())
				newILM.SetSchemaUrl(ilm.SchemaUrl())

				for k := 0; k < ilm.Metrics().Len(); k++ {
					m := ilm.Metrics().At(k)
					newMetric := newILM.Metrics().AppendEmpty()
					m.CopyTo(newMetric)
				}
			}
		}

	}

	return merged
}
