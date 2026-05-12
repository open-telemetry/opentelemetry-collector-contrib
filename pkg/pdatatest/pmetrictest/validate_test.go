// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pmetrictest

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func TestValidateMetrics(t *testing.T) {
	t.Run("valid-all-unique-attributes", func(t *testing.T) {
		md := pmetric.NewMetrics()
		rm := md.ResourceMetrics().AppendEmpty()
		sm := rm.ScopeMetrics().AppendEmpty()
		m := sm.Metrics().AppendEmpty()
		m.SetName("http.requests")
		m.SetEmptyGauge()

		dp1 := m.Gauge().DataPoints().AppendEmpty()
		dp1.Attributes().PutStr("status", "200")
		dp1.Attributes().PutStr("method", "GET")

		dp2 := m.Gauge().DataPoints().AppendEmpty()
		dp2.Attributes().PutStr("status", "404")
		dp2.Attributes().PutStr("method", "GET")

		assert.NoError(t, ValidateMetrics(md))
	})

	t.Run("duplicate-gauge-datapoints", func(t *testing.T) {
		md := pmetric.NewMetrics()
		rm := md.ResourceMetrics().AppendEmpty()
		sm := rm.ScopeMetrics().AppendEmpty()
		m := sm.Metrics().AppendEmpty()
		m.SetName("http.requests")
		m.SetEmptyGauge()

		dp1 := m.Gauge().DataPoints().AppendEmpty()
		dp1.Attributes().PutStr("status", "200")
		dp1.Attributes().PutStr("method", "GET")

		dp2 := m.Gauge().DataPoints().AppendEmpty()
		dp2.Attributes().PutStr("status", "200")
		dp2.Attributes().PutStr("method", "GET")

		err := ValidateMetrics(md)
		require.Error(t, err)
		assert.Contains(t, err.Error(), `metric "http.requests"`)
		assert.Contains(t, err.Error(), "datapoint at index 1 has duplicate attributes with datapoint at index 0")
	})

	t.Run("duplicate-sum-datapoints", func(t *testing.T) {
		md := pmetric.NewMetrics()
		rm := md.ResourceMetrics().AppendEmpty()
		sm := rm.ScopeMetrics().AppendEmpty()
		m := sm.Metrics().AppendEmpty()
		m.SetName("process.cpu.time")
		sum := m.SetEmptySum()
		sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
		sum.SetIsMonotonic(true)

		dp1 := sum.DataPoints().AppendEmpty()
		dp1.Attributes().PutStr("state", "user")

		dp2 := sum.DataPoints().AppendEmpty()
		dp2.Attributes().PutStr("state", "user")

		err := ValidateMetrics(md)
		require.Error(t, err)
		assert.Contains(t, err.Error(), `metric "process.cpu.time"`)
		assert.Contains(t, err.Error(), "datapoint at index 1 has duplicate attributes with datapoint at index 0")
	})

	t.Run("duplicate-histogram-datapoints", func(t *testing.T) {
		md := pmetric.NewMetrics()
		rm := md.ResourceMetrics().AppendEmpty()
		sm := rm.ScopeMetrics().AppendEmpty()
		m := sm.Metrics().AppendEmpty()
		m.SetName("http.request.duration")
		hist := m.SetEmptyHistogram()
		hist.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)

		dp1 := hist.DataPoints().AppendEmpty()
		dp1.Attributes().PutStr("endpoint", "/api")

		dp2 := hist.DataPoints().AppendEmpty()
		dp2.Attributes().PutStr("endpoint", "/api")

		err := ValidateMetrics(md)
		require.Error(t, err)
		assert.Contains(t, err.Error(), `metric "http.request.duration"`)
		assert.Contains(t, err.Error(), "datapoint at index 1 has duplicate attributes with datapoint at index 0")
	})

	t.Run("duplicate-exponential-histogram-datapoints", func(t *testing.T) {
		md := pmetric.NewMetrics()
		rm := md.ResourceMetrics().AppendEmpty()
		sm := rm.ScopeMetrics().AppendEmpty()
		m := sm.Metrics().AppendEmpty()
		m.SetName("http.request.size")
		expHist := m.SetEmptyExponentialHistogram()
		expHist.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)

		dp1 := expHist.DataPoints().AppendEmpty()
		dp1.Attributes().PutStr("method", "POST")

		dp2 := expHist.DataPoints().AppendEmpty()
		dp2.Attributes().PutStr("method", "POST")

		err := ValidateMetrics(md)
		require.Error(t, err)
		assert.Contains(t, err.Error(), `metric "http.request.size"`)
		assert.Contains(t, err.Error(), "datapoint at index 1 has duplicate attributes with datapoint at index 0")
	})

	t.Run("duplicate-summary-datapoints", func(t *testing.T) {
		md := pmetric.NewMetrics()
		rm := md.ResourceMetrics().AppendEmpty()
		sm := rm.ScopeMetrics().AppendEmpty()
		m := sm.Metrics().AppendEmpty()
		m.SetName("rpc.duration")
		m.SetEmptySummary()

		dp1 := m.Summary().DataPoints().AppendEmpty()
		dp1.Attributes().PutStr("service", "auth")

		dp2 := m.Summary().DataPoints().AppendEmpty()
		dp2.Attributes().PutStr("service", "auth")

		err := ValidateMetrics(md)
		require.Error(t, err)
		assert.Contains(t, err.Error(), `metric "rpc.duration"`)
		assert.Contains(t, err.Error(), "datapoint at index 1 has duplicate attributes with datapoint at index 0")
	})

	t.Run("duplicate-empty-attributes", func(t *testing.T) {
		md := pmetric.NewMetrics()
		rm := md.ResourceMetrics().AppendEmpty()
		sm := rm.ScopeMetrics().AppendEmpty()
		m := sm.Metrics().AppendEmpty()
		m.SetName("system.uptime")
		m.SetEmptyGauge()

		// Two datapoints with empty attribute maps — they are duplicates.
		m.Gauge().DataPoints().AppendEmpty()
		m.Gauge().DataPoints().AppendEmpty()

		err := ValidateMetrics(md)
		require.Error(t, err)
		assert.Contains(t, err.Error(), `metric "system.uptime"`)
		assert.Contains(t, err.Error(), "datapoint at index 1 has duplicate attributes with datapoint at index 0")
	})

	t.Run("same-attributes-different-insertion-order", func(t *testing.T) {
		md := pmetric.NewMetrics()
		rm := md.ResourceMetrics().AppendEmpty()
		sm := rm.ScopeMetrics().AppendEmpty()
		m := sm.Metrics().AppendEmpty()
		m.SetName("test.metric")
		m.SetEmptyGauge()

		// Insert {a:1, b:2} then {b:2, a:1} — same attributes, different order.
		dp1 := m.Gauge().DataPoints().AppendEmpty()
		dp1.Attributes().PutStr("a", "1")
		dp1.Attributes().PutStr("b", "2")

		dp2 := m.Gauge().DataPoints().AppendEmpty()
		dp2.Attributes().PutStr("b", "2")
		dp2.Attributes().PutStr("a", "1")

		err := ValidateMetrics(md)
		require.Error(t, err)
		assert.Contains(t, err.Error(), `metric "test.metric"`)
		assert.Contains(t, err.Error(), "datapoint at index 1 has duplicate attributes with datapoint at index 0")
	})

	t.Run("multiple-metrics-only-one-has-duplicates", func(t *testing.T) {
		md := pmetric.NewMetrics()
		rm := md.ResourceMetrics().AppendEmpty()
		sm := rm.ScopeMetrics().AppendEmpty()

		// First metric — valid, no duplicates.
		m1 := sm.Metrics().AppendEmpty()
		m1.SetName("valid.metric")
		m1.SetEmptyGauge()
		dp1 := m1.Gauge().DataPoints().AppendEmpty()
		dp1.Attributes().PutStr("key", "a")
		dp2 := m1.Gauge().DataPoints().AppendEmpty()
		dp2.Attributes().PutStr("key", "b")

		// Second metric — has duplicates.
		m2 := sm.Metrics().AppendEmpty()
		m2.SetName("bad.metric")
		m2.SetEmptySum()
		dp3 := m2.Sum().DataPoints().AppendEmpty()
		dp3.Attributes().PutStr("key", "x")
		dp4 := m2.Sum().DataPoints().AppendEmpty()
		dp4.Attributes().PutStr("key", "x")

		err := ValidateMetrics(md)
		require.Error(t, err)
		assert.Contains(t, err.Error(), `metric "bad.metric"`)
		assert.NotContains(t, err.Error(), `metric "valid.metric"`)
	})

	t.Run("no-datapoints", func(t *testing.T) {
		md := pmetric.NewMetrics()
		rm := md.ResourceMetrics().AppendEmpty()
		sm := rm.ScopeMetrics().AppendEmpty()
		m := sm.Metrics().AppendEmpty()
		m.SetName("empty.metric")
		m.SetEmptyGauge()
		// No datapoints added — should be valid.

		assert.NoError(t, ValidateMetrics(md))
	})

	t.Run("single-datapoint", func(t *testing.T) {
		md := pmetric.NewMetrics()
		rm := md.ResourceMetrics().AppendEmpty()
		sm := rm.ScopeMetrics().AppendEmpty()
		m := sm.Metrics().AppendEmpty()
		m.SetName("single.dp")
		m.SetEmptyGauge()
		dp := m.Gauge().DataPoints().AppendEmpty()
		dp.Attributes().PutStr("key", "value")

		assert.NoError(t, ValidateMetrics(md))
	})

	t.Run("empty-metrics", func(t *testing.T) {
		md := pmetric.NewMetrics()
		assert.NoError(t, ValidateMetrics(md))
	})

	t.Run("error-includes-resource-and-scope-context", func(t *testing.T) {
		md := pmetric.NewMetrics()
		rm := md.ResourceMetrics().AppendEmpty()
		rm.Resource().Attributes().PutStr("host.name", "worker-42")
		sm := rm.ScopeMetrics().AppendEmpty()
		sm.Scope().SetName("my.scope")
		m := sm.Metrics().AppendEmpty()
		m.SetName("ctx.metric")
		m.SetEmptyGauge()

		m.Gauge().DataPoints().AppendEmpty()
		m.Gauge().DataPoints().AppendEmpty()

		err := ValidateMetrics(md)
		require.Error(t, err)
		assert.Contains(t, err.Error(), `resource "map[host.name:worker-42]"`)
		assert.Contains(t, err.Error(), `scope "my.scope"`)
		assert.Contains(t, err.Error(), `metric "ctx.metric"`)
	})

	t.Run("multiple-duplicates-in-same-metric", func(t *testing.T) {
		md := pmetric.NewMetrics()
		rm := md.ResourceMetrics().AppendEmpty()
		sm := rm.ScopeMetrics().AppendEmpty()
		m := sm.Metrics().AppendEmpty()
		m.SetName("multi.dup")
		m.SetEmptyGauge()

		// Three datapoints with same attributes → indices 1 and 2 are duplicates of index 0.
		for range 3 {
			dp := m.Gauge().DataPoints().AppendEmpty()
			dp.Attributes().PutStr("key", "same")
		}

		err := ValidateMetrics(md)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "datapoint at index 1 has duplicate attributes with datapoint at index 0")
		assert.Contains(t, err.Error(), "datapoint at index 2 has duplicate attributes with datapoint at index 0")
	})

	t.Run("different-attribute-types-not-duplicate", func(t *testing.T) {
		md := pmetric.NewMetrics()
		rm := md.ResourceMetrics().AppendEmpty()
		sm := rm.ScopeMetrics().AppendEmpty()
		m := sm.Metrics().AppendEmpty()
		m.SetName("typed.metric")
		m.SetEmptyGauge()

		// String "1" vs int 1 for the same key — should NOT be duplicates.
		dp1 := m.Gauge().DataPoints().AppendEmpty()
		dp1.Attributes().PutStr("key", "1")

		dp2 := m.Gauge().DataPoints().AppendEmpty()
		dp2.Attributes().PutInt("key", 1)

		assert.NoError(t, ValidateMetrics(md))
	})

	t.Run("metric-type-empty", func(t *testing.T) {
		md := pmetric.NewMetrics()
		rm := md.ResourceMetrics().AppendEmpty()
		sm := rm.ScopeMetrics().AppendEmpty()
		sm.Metrics().AppendEmpty().SetName("empty.type")
		// MetricTypeEmpty by default — no datapoints to check.

		assert.NoError(t, ValidateMetrics(md))
	})

	t.Run("conflict-same-name-different-type", func(t *testing.T) {
		md := pmetric.NewMetrics()
		rm := md.ResourceMetrics().AppendEmpty()
		sm := rm.ScopeMetrics().AppendEmpty()

		m1 := sm.Metrics().AppendEmpty()
		m1.SetName("http.requests")
		m1.SetEmptyGauge()

		m2 := sm.Metrics().AppendEmpty()
		m2.SetName("http.requests")
		m2.SetEmptySum()

		err := ValidateMetrics(md)
		require.Error(t, err)
		assert.Contains(t, err.Error(), `metric "http.requests"`)
		assert.Contains(t, err.Error(), "type: Gauge vs Sum")
	})

	t.Run("conflict-same-name-different-unit", func(t *testing.T) {
		md := pmetric.NewMetrics()
		rm := md.ResourceMetrics().AppendEmpty()
		sm := rm.ScopeMetrics().AppendEmpty()

		m1 := sm.Metrics().AppendEmpty()
		m1.SetName("http.requests")
		m1.SetEmptySum()
		m1.SetUnit("ms")

		m2 := sm.Metrics().AppendEmpty()
		m2.SetName("http.requests")
		m2.SetEmptySum()
		m2.SetUnit("s")

		err := ValidateMetrics(md)
		require.Error(t, err)
		assert.Contains(t, err.Error(), `metric "http.requests"`)
		assert.Contains(t, err.Error(), `unit: "ms" vs "s"`)
	})

	t.Run("conflict-sum-different-temporality", func(t *testing.T) {
		md := pmetric.NewMetrics()
		rm := md.ResourceMetrics().AppendEmpty()
		sm := rm.ScopeMetrics().AppendEmpty()

		m1 := sm.Metrics().AppendEmpty()
		m1.SetName("process.cpu.time")
		m1.SetEmptySum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)

		m2 := sm.Metrics().AppendEmpty()
		m2.SetName("process.cpu.time")
		m2.SetEmptySum().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)

		err := ValidateMetrics(md)
		require.Error(t, err)
		assert.Contains(t, err.Error(), `metric "process.cpu.time"`)
		assert.Contains(t, err.Error(), "temporality: Cumulative vs Delta")
	})

	t.Run("conflict-sum-different-monotonic", func(t *testing.T) {
		md := pmetric.NewMetrics()
		rm := md.ResourceMetrics().AppendEmpty()
		sm := rm.ScopeMetrics().AppendEmpty()

		m1 := sm.Metrics().AppendEmpty()
		m1.SetName("process.cpu.time")
		sum1 := m1.SetEmptySum()
		sum1.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
		sum1.SetIsMonotonic(true)

		m2 := sm.Metrics().AppendEmpty()
		m2.SetName("process.cpu.time")
		sum2 := m2.SetEmptySum()
		sum2.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
		sum2.SetIsMonotonic(false)

		err := ValidateMetrics(md)
		require.Error(t, err)
		assert.Contains(t, err.Error(), `metric "process.cpu.time"`)
		assert.Contains(t, err.Error(), "monotonic: true vs false")
	})

	t.Run("conflict-histogram-different-temporality", func(t *testing.T) {
		md := pmetric.NewMetrics()
		rm := md.ResourceMetrics().AppendEmpty()
		sm := rm.ScopeMetrics().AppendEmpty()

		m1 := sm.Metrics().AppendEmpty()
		m1.SetName("http.request.duration")
		m1.SetEmptyHistogram().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)

		m2 := sm.Metrics().AppendEmpty()
		m2.SetName("http.request.duration")
		m2.SetEmptyHistogram().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)

		err := ValidateMetrics(md)
		require.Error(t, err)
		assert.Contains(t, err.Error(), `metric "http.request.duration"`)
		assert.Contains(t, err.Error(), "temporality: Cumulative vs Delta")
	})

	t.Run("conflict-exponential-histogram-different-temporality", func(t *testing.T) {
		md := pmetric.NewMetrics()
		rm := md.ResourceMetrics().AppendEmpty()
		sm := rm.ScopeMetrics().AppendEmpty()

		m1 := sm.Metrics().AppendEmpty()
		m1.SetName("http.request.size")
		m1.SetEmptyExponentialHistogram().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)

		m2 := sm.Metrics().AppendEmpty()
		m2.SetName("http.request.size")
		m2.SetEmptyExponentialHistogram().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)

		err := ValidateMetrics(md)
		require.Error(t, err)
		assert.Contains(t, err.Error(), `metric "http.request.size"`)
		assert.Contains(t, err.Error(), "temporality: Cumulative vs Delta")
	})

	t.Run("no-conflict-same-name-same-identity", func(t *testing.T) {
		md := pmetric.NewMetrics()
		rm := md.ResourceMetrics().AppendEmpty()
		sm := rm.ScopeMetrics().AppendEmpty()

		m1 := sm.Metrics().AppendEmpty()
		m1.SetName("http.requests")
		m1.SetUnit("1")
		m1.SetEmptyGauge()

		m2 := sm.Metrics().AppendEmpty()
		m2.SetName("http.requests")
		m2.SetUnit("1")
		m2.SetEmptyGauge()

		assert.NoError(t, ValidateMetrics(md))
	})

	t.Run("no-conflict-different-names", func(t *testing.T) {
		md := pmetric.NewMetrics()
		rm := md.ResourceMetrics().AppendEmpty()
		sm := rm.ScopeMetrics().AppendEmpty()

		m1 := sm.Metrics().AppendEmpty()
		m1.SetName("http.requests")
		m1.SetEmptyGauge()

		m2 := sm.Metrics().AppendEmpty()
		m2.SetName("http.duration")
		m2.SetEmptySum()

		assert.NoError(t, ValidateMetrics(md))
	})

	t.Run("conflict-multiple-names", func(t *testing.T) {
		md := pmetric.NewMetrics()
		rm := md.ResourceMetrics().AppendEmpty()
		sm := rm.ScopeMetrics().AppendEmpty()

		// First pair: "metric.a" with type conflict.
		m1 := sm.Metrics().AppendEmpty()
		m1.SetName("metric.a")
		m1.SetEmptyGauge()
		m2 := sm.Metrics().AppendEmpty()
		m2.SetName("metric.a")
		m2.SetEmptySum()

		// Second pair: "metric.b" with unit conflict.
		m3 := sm.Metrics().AppendEmpty()
		m3.SetName("metric.b")
		m3.SetEmptyGauge()
		m3.SetUnit("ms")
		m4 := sm.Metrics().AppendEmpty()
		m4.SetName("metric.b")
		m4.SetEmptyGauge()
		m4.SetUnit("s")

		err := ValidateMetrics(md)
		require.Error(t, err)
		assert.Contains(t, err.Error(), `metric "metric.a"`)
		assert.Contains(t, err.Error(), `metric "metric.b"`)
	})

	t.Run("conflict-includes-resource-scope-context", func(t *testing.T) {
		md := pmetric.NewMetrics()
		rm := md.ResourceMetrics().AppendEmpty()
		rm.Resource().Attributes().PutStr("host.name", "worker-42")
		sm := rm.ScopeMetrics().AppendEmpty()
		sm.Scope().SetName("my.receiver")

		m1 := sm.Metrics().AppendEmpty()
		m1.SetName("http.requests")
		m1.SetEmptyGauge()

		m2 := sm.Metrics().AppendEmpty()
		m2.SetName("http.requests")
		m2.SetEmptySum()

		err := ValidateMetrics(md)
		require.Error(t, err)
		assert.Contains(t, err.Error(), `resource "map[host.name:worker-42]"`)
		assert.Contains(t, err.Error(), `scope "my.receiver"`)
		assert.Contains(t, err.Error(), `metric "http.requests"`)
	})

	t.Run("conflict-empty-vs-gauge", func(t *testing.T) {
		md := pmetric.NewMetrics()
		rm := md.ResourceMetrics().AppendEmpty()
		sm := rm.ScopeMetrics().AppendEmpty()

		m1 := sm.Metrics().AppendEmpty()
		m1.SetName("test.metric")
		// MetricTypeEmpty by default.

		m2 := sm.Metrics().AppendEmpty()
		m2.SetName("test.metric")
		m2.SetEmptyGauge()

		err := ValidateMetrics(md)
		require.Error(t, err)
		assert.Contains(t, err.Error(), `metric "test.metric"`)
		assert.Contains(t, err.Error(), "type: Empty vs Gauge")
	})

	t.Run("no-conflict-two-empty-type", func(t *testing.T) {
		md := pmetric.NewMetrics()
		rm := md.ResourceMetrics().AppendEmpty()
		sm := rm.ScopeMetrics().AppendEmpty()

		sm.Metrics().AppendEmpty().SetName("test.metric")
		sm.Metrics().AppendEmpty().SetName("test.metric")
		// Both MetricTypeEmpty with same name — same identity, no conflict.

		assert.NoError(t, ValidateMetrics(md))
	})

	// TODO (PR 5): Add tests for multiple Metric entries with the same name that are mergeable

	// TODO (PR 4): Add tests for multiple ScopeMetrics with equal scope under the same resource

	// TODO (PR 3): Add tests for multiple ResourceMetrics with equal resource attributes
}
