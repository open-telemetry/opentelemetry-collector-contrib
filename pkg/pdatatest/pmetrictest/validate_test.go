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

	t.Run("duplicate-metric-name-same-type", func(t *testing.T) {
		md := pmetric.NewMetrics()
		rm := md.ResourceMetrics().AppendEmpty()
		sm := rm.ScopeMetrics().AppendEmpty()

		m1 := sm.Metrics().AppendEmpty()
		m1.SetName("http.requests")
		m1.SetEmptyGauge()

		m2 := sm.Metrics().AppendEmpty()
		m2.SetName("http.requests")
		m2.SetEmptyGauge()

		err := ValidateMetrics(md)
		require.Error(t, err)
		assert.Contains(t, err.Error(), `metric "http.requests" at index 1 is a duplicate of metric at index 0`)
	})

	t.Run("duplicate-metric-name-different-type", func(t *testing.T) {
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
		assert.Contains(t, err.Error(), `metric "http.requests" at index 1 is a duplicate of metric at index 0`)
	})

	t.Run("no-duplicate-different-names", func(t *testing.T) {
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

	t.Run("duplicate-metric-name-multiple-duplicates", func(t *testing.T) {
		md := pmetric.NewMetrics()
		rm := md.ResourceMetrics().AppendEmpty()
		sm := rm.ScopeMetrics().AppendEmpty()

		// Three metrics with the same name.
		for range 3 {
			m := sm.Metrics().AppendEmpty()
			m.SetName("http.requests")
			m.SetEmptyGauge()
		}

		err := ValidateMetrics(md)
		require.Error(t, err)
		assert.Contains(t, err.Error(), `metric "http.requests" at index 1 is a duplicate of metric at index 0`)
		assert.Contains(t, err.Error(), `metric "http.requests" at index 2 is a duplicate of metric at index 0`)
	})

	t.Run("duplicate-metric-names-multiple-names", func(t *testing.T) {
		md := pmetric.NewMetrics()
		rm := md.ResourceMetrics().AppendEmpty()
		sm := rm.ScopeMetrics().AppendEmpty()

		// Two pairs of duplicate names.
		m1 := sm.Metrics().AppendEmpty()
		m1.SetName("metric.a")
		m1.SetEmptyGauge()
		m2 := sm.Metrics().AppendEmpty()
		m2.SetName("metric.a")
		m2.SetEmptyGauge()

		m3 := sm.Metrics().AppendEmpty()
		m3.SetName("metric.b")
		m3.SetEmptySum()
		m4 := sm.Metrics().AppendEmpty()
		m4.SetName("metric.b")
		m4.SetEmptySum()

		err := ValidateMetrics(md)
		require.Error(t, err)
		assert.Contains(t, err.Error(), `metric "metric.a"`)
		assert.Contains(t, err.Error(), `metric "metric.b"`)
	})

	t.Run("duplicate-metric-name-includes-resource-scope-context", func(t *testing.T) {
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
		m2.SetEmptyGauge()

		err := ValidateMetrics(md)
		require.Error(t, err)
		assert.Contains(t, err.Error(), `resource "map[host.name:worker-42]"`)
		assert.Contains(t, err.Error(), `scope "my.receiver"`)
		assert.Contains(t, err.Error(), `metric "http.requests"`)
	})

	t.Run("single-metric-no-duplicate", func(t *testing.T) {
		md := pmetric.NewMetrics()
		rm := md.ResourceMetrics().AppendEmpty()
		sm := rm.ScopeMetrics().AppendEmpty()

		m := sm.Metrics().AppendEmpty()
		m.SetName("http.requests")
		m.SetEmptyGauge()

		assert.NoError(t, ValidateMetrics(md))
	})

	t.Run("duplicate-scope-same-name-and-version", func(t *testing.T) {
		md := pmetric.NewMetrics()
		rm := md.ResourceMetrics().AppendEmpty()

		sm1 := rm.ScopeMetrics().AppendEmpty()
		sm1.Scope().SetName("my.scope")
		sm1.Scope().SetVersion("1.0")

		sm2 := rm.ScopeMetrics().AppendEmpty()
		sm2.Scope().SetName("my.scope")
		sm2.Scope().SetVersion("1.0")

		err := ValidateMetrics(md)
		require.Error(t, err)
		assert.Contains(t, err.Error(), `scope "my.scope" (version "1.0") at index 1 is a duplicate of scope at index 0`)
	})

	t.Run("no-duplicate-scope-different-names", func(t *testing.T) {
		md := pmetric.NewMetrics()
		rm := md.ResourceMetrics().AppendEmpty()

		sm1 := rm.ScopeMetrics().AppendEmpty()
		sm1.Scope().SetName("scope.a")

		sm2 := rm.ScopeMetrics().AppendEmpty()
		sm2.Scope().SetName("scope.b")

		assert.NoError(t, ValidateMetrics(md))
	})

	t.Run("no-duplicate-scope-same-name-different-version", func(t *testing.T) {
		md := pmetric.NewMetrics()
		rm := md.ResourceMetrics().AppendEmpty()

		sm1 := rm.ScopeMetrics().AppendEmpty()
		sm1.Scope().SetName("my.scope")
		sm1.Scope().SetVersion("1.0")

		sm2 := rm.ScopeMetrics().AppendEmpty()
		sm2.Scope().SetName("my.scope")
		sm2.Scope().SetVersion("2.0")

		assert.NoError(t, ValidateMetrics(md))
	})

	t.Run("duplicate-scope-multiple-duplicates", func(t *testing.T) {
		md := pmetric.NewMetrics()
		rm := md.ResourceMetrics().AppendEmpty()

		for range 3 {
			sm := rm.ScopeMetrics().AppendEmpty()
			sm.Scope().SetName("my.scope")
		}

		err := ValidateMetrics(md)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "at index 1 is a duplicate of scope at index 0")
		assert.Contains(t, err.Error(), "at index 2 is a duplicate of scope at index 0")
	})

	t.Run("duplicate-scope-multiple-pairs", func(t *testing.T) {
		md := pmetric.NewMetrics()
		rm := md.ResourceMetrics().AppendEmpty()

		sm1 := rm.ScopeMetrics().AppendEmpty()
		sm1.Scope().SetName("scope.a")
		sm2 := rm.ScopeMetrics().AppendEmpty()
		sm2.Scope().SetName("scope.a")

		sm3 := rm.ScopeMetrics().AppendEmpty()
		sm3.Scope().SetName("scope.b")
		sm4 := rm.ScopeMetrics().AppendEmpty()
		sm4.Scope().SetName("scope.b")

		err := ValidateMetrics(md)
		require.Error(t, err)
		assert.Contains(t, err.Error(), `"scope.a"`)
		assert.Contains(t, err.Error(), `"scope.b"`)
	})

	t.Run("duplicate-scope-empty-names", func(t *testing.T) {
		md := pmetric.NewMetrics()
		rm := md.ResourceMetrics().AppendEmpty()

		// Two ScopeMetrics with empty scope name and version (they are duplicates)
		rm.ScopeMetrics().AppendEmpty()
		rm.ScopeMetrics().AppendEmpty()

		err := ValidateMetrics(md)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "at index 1 is a duplicate of scope at index 0")
	})

	t.Run("single-scope-no-duplicate", func(t *testing.T) {
		md := pmetric.NewMetrics()
		rm := md.ResourceMetrics().AppendEmpty()

		sm := rm.ScopeMetrics().AppendEmpty()
		sm.Scope().SetName("my.scope")

		assert.NoError(t, ValidateMetrics(md))
	})

	t.Run("duplicate-scope-includes-resource-context", func(t *testing.T) {
		md := pmetric.NewMetrics()
		rm := md.ResourceMetrics().AppendEmpty()
		rm.Resource().Attributes().PutStr("host.name", "worker-42")

		sm1 := rm.ScopeMetrics().AppendEmpty()
		sm1.Scope().SetName("my.scope")

		sm2 := rm.ScopeMetrics().AppendEmpty()
		sm2.Scope().SetName("my.scope")

		err := ValidateMetrics(md)
		require.Error(t, err)
		assert.Contains(t, err.Error(), `resource "map[host.name:worker-42]"`)
		assert.Contains(t, err.Error(), `scope "my.scope"`)
	})

	t.Run("duplicate-resource-attributes", func(t *testing.T) {
		md := pmetric.NewMetrics()

		rm1 := md.ResourceMetrics().AppendEmpty()
		rm1.Resource().Attributes().PutStr("host.name", "worker-42")

		rm2 := md.ResourceMetrics().AppendEmpty()
		rm2.Resource().Attributes().PutStr("host.name", "worker-42")

		err := ValidateMetrics(md)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "resource map[host.name:worker-42] at index 1 is a duplicate of resource at index 0")
	})

	t.Run("no-duplicate-different-resource-attributes", func(t *testing.T) {
		md := pmetric.NewMetrics()

		rm1 := md.ResourceMetrics().AppendEmpty()
		rm1.Resource().Attributes().PutStr("host.name", "worker-42")

		rm2 := md.ResourceMetrics().AppendEmpty()
		rm2.Resource().Attributes().PutStr("host.name", "worker-99")

		assert.NoError(t, ValidateMetrics(md))
	})

	t.Run("duplicate-resource-multiple-duplicates", func(t *testing.T) {
		md := pmetric.NewMetrics()

		for range 3 {
			rm := md.ResourceMetrics().AppendEmpty()
			rm.Resource().Attributes().PutStr("host.name", "worker-42")
		}

		err := ValidateMetrics(md)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "at index 1 is a duplicate of resource at index 0")
		assert.Contains(t, err.Error(), "at index 2 is a duplicate of resource at index 0")
	})

	t.Run("duplicate-resource-multiple-pairs", func(t *testing.T) {
		md := pmetric.NewMetrics()

		rm1 := md.ResourceMetrics().AppendEmpty()
		rm1.Resource().Attributes().PutStr("host.name", "worker-42")
		rm2 := md.ResourceMetrics().AppendEmpty()
		rm2.Resource().Attributes().PutStr("host.name", "worker-42")

		rm3 := md.ResourceMetrics().AppendEmpty()
		rm3.Resource().Attributes().PutStr("host.name", "worker-99")
		rm4 := md.ResourceMetrics().AppendEmpty()
		rm4.Resource().Attributes().PutStr("host.name", "worker-99")

		err := ValidateMetrics(md)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "worker-42")
		assert.Contains(t, err.Error(), "worker-99")
	})

	t.Run("duplicate-resource-empty-attributes", func(t *testing.T) {
		md := pmetric.NewMetrics()

		// Two ResourceMetrics with empty attributes — they are duplicates.
		md.ResourceMetrics().AppendEmpty()
		md.ResourceMetrics().AppendEmpty()

		err := ValidateMetrics(md)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "at index 1 is a duplicate of resource at index 0")
	})

	t.Run("single-resource-no-duplicate", func(t *testing.T) {
		md := pmetric.NewMetrics()

		rm := md.ResourceMetrics().AppendEmpty()
		rm.Resource().Attributes().PutStr("host.name", "worker-42")

		assert.NoError(t, ValidateMetrics(md))
	})
}
