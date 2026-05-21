// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pmetricassert

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"gopkg.in/yaml.v3"
)

func TestCountAssertionValidate(t *testing.T) {
	t.Run("exact pass", func(t *testing.T) {
		exact := 2
		c := &CountAssertion{Exact: &exact}
		require.NoError(t, c.Validate(2))
	})

	t.Run("exact fail", func(t *testing.T) {
		exact := 1
		c := &CountAssertion{Exact: &exact}
		err := c.Validate(2)
		require.Error(t, err)
		require.Contains(t, err.Error(), "expected exactly 1 items, got 2")
	})

	t.Run("range pass", func(t *testing.T) {
		minVal := 1
		maxVal := 3
		c := &CountAssertion{Min: &minVal, Max: &maxVal}
		require.NoError(t, c.Validate(2))
	})

	t.Run("range fail", func(t *testing.T) {
		minVal := 3
		c := &CountAssertion{Min: &minVal}
		err := c.Validate(2)
		require.Error(t, err)
		require.Contains(t, err.Error(), "expected at least 3 items, got 2")
	})

	t.Run("invalid exact with min", func(t *testing.T) {
		exact := 1
		minVal := 0
		c := &CountAssertion{Exact: &exact, Min: &minVal}
		err := c.Validate(1)
		require.Error(t, err)
		require.Contains(t, err.Error(), "exact cannot be combined with min/max")
	})
}

func TestMetricsAssertionUnmarshalYAML(t *testing.T) {
	var a MetricsAssertion
	err := yaml.Unmarshal([]byte(`
metrics/exact:
  - name: svc.active
    type: gauge
metrics/count:
  min: 1
  max: 3
`), &a)
	require.NoError(t, err)
	require.Len(t, a.Exact, 1)
	require.Equal(t, "svc.active", a.Exact[0].Name)
	require.NotNil(t, a.Count)
	require.NotNil(t, a.Count.Min)
	require.NotNil(t, a.Count.Max)
	require.Equal(t, 1, *a.Count.Min)
	require.Equal(t, 3, *a.Count.Max)
}

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

	t.Run("count exact pass", func(t *testing.T) {
		exactCount := 2
		assertion := MetricsAssertion{Count: &CountAssertion{Exact: &exactCount}}
		require.NoError(t, assertion.Validate(actual))
	})

	t.Run("count exact fail", func(t *testing.T) {
		exactCount := 1
		assertion := MetricsAssertion{Count: &CountAssertion{Exact: &exactCount}}
		err := assertion.Validate(actual)
		require.Error(t, err)
		require.Contains(t, err.Error(), "expected exactly 1 items, got 2")
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

func boolPtr(v bool) *bool {
	return &v
}
