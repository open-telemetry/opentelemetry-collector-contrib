// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter"

import (
	"go.opentelemetry.io/collector/pdata/pmetric"
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

// mergeMetrics concatenates two pmetric.Metrics into a single pmetric.Metrics.
func mergeMetrics(m1 pmetric.Metrics, m2 pmetric.Metrics) pmetric.Metrics {
	mergedMetrics := pmetric.NewMetrics()

	if m1.MetricCount() == 0 && m2.MetricCount() == 0 {
		return mergedMetrics
	}

	// Iterate over the first metric and append metrics to the merged metrics
	for i := 0; i < m1.ResourceMetrics().Len(); i++ {
		rs := m1.ResourceMetrics().At(i)
		newRS := mergedMetrics.ResourceMetrics().AppendEmpty()

		rs.Resource().MoveTo(newRS.Resource())
		newRS.SetSchemaUrl(rs.SchemaUrl())

		for j := 0; j < rs.ScopeMetrics().Len(); j++ {
			ils := rs.ScopeMetrics().At(j)

			newILS := newRS.ScopeMetrics().AppendEmpty()
			ils.Scope().MoveTo(newILS.Scope())
			newILS.SetSchemaUrl(ils.SchemaUrl())

			for k := 0; k < ils.Metrics().Len(); k++ {
				metric := ils.Metrics().At(k)
				newMetric := newILS.Metrics().AppendEmpty()
				metric.MoveTo(newMetric)
			}
		}
	}

	// Iterate over the second metric and append metrics to the merged metrics
	for i := 0; i < m2.ResourceMetrics().Len(); i++ {
		rs := m2.ResourceMetrics().At(i)
		newRS := mergedMetrics.ResourceMetrics().AppendEmpty()

		rs.Resource().MoveTo(newRS.Resource())
		newRS.SetSchemaUrl(rs.SchemaUrl())

		for j := 0; j < rs.ScopeMetrics().Len(); j++ {
			ils := rs.ScopeMetrics().At(j)

			newILS := newRS.ScopeMetrics().AppendEmpty()
			ils.Scope().MoveTo(newILS.Scope())
			newILS.SetSchemaUrl(ils.SchemaUrl())

			for k := 0; k < ils.Metrics().Len(); k++ {
				metric := ils.Metrics().At(k)
				newMetric := newILS.Metrics().AppendEmpty()
				metric.MoveTo(newMetric)
			}
		}
	}

	return mergedMetrics
}
