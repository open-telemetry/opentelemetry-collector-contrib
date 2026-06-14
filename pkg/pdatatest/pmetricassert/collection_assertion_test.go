// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pmetricassert

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func TestMetricsAssertionValidate(t *testing.T) {
	actual := buildMetricSlice(t, []metricAssertion{
		{Name: "svc.requests", Type: "sum", Unit: "{requests}", Temporality: "cumulative", Monotonic: boolPtr(true)},
		{Name: "svc.active", Type: "gauge", Unit: "1"},
	})

	t.Run("exact order insensitive", func(t *testing.T) {
		exact := []MetricAssertion{
			{Name: "svc.active", Type: "gauge", Unit: "1"},
			{Name: "svc.requests", Type: "sum", Unit: "{requests}", Temporality: "cumulative", Monotonic: boolPtr(true)},
		}
		assertion := MetricsAssertion{Exact: exact}
		require.NoError(t, assertion.Validate(actual))
	})

	t.Run("include subset", func(t *testing.T) {
		include := []MetricAssertion{{Name: "svc.active", Type: "gauge", Unit: "1"}}
		assertion := MetricsAssertion{Include: include}
		require.NoError(t, assertion.Validate(actual))
	})

	t.Run("include missing", func(t *testing.T) {
		assertion := MetricsAssertion{Include: []MetricAssertion{{Name: "svc.latency", Type: "histogram"}}}
		err := assertion.Validate(actual)
		require.Error(t, err)
		require.Contains(t, err.Error(), "included metric[0] (\"svc.latency\") was not found")
	})
}

func buildMetricSlice(t *testing.T, assertions []metricAssertion) pmetric.MetricSlice {
	t.Helper()

	m := pmetric.NewMetrics()
	rm := m.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	ms := sm.Metrics()

	for i := range assertions {
		a := assertions[i]
		metric := ms.AppendEmpty()
		metric.SetName(a.Name)
		metric.SetUnit(a.Unit)

		switch a.Type {
		case "gauge":
			metric.SetEmptyGauge().DataPoints().AppendEmpty().SetIntValue(1)
		case "sum":
			sum := metric.SetEmptySum()
			sum.SetIsMonotonic(a.Monotonic != nil && *a.Monotonic)
			if a.Temporality == "delta" {
				sum.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
			} else {
				sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
			}
			sum.DataPoints().AppendEmpty().SetIntValue(1)
		default:
			t.Fatalf("unsupported metric type for test helper: %q", a.Type)
		}
	}

	return sm.Metrics()
}

func boolPtr(v bool) *bool { return &v }
