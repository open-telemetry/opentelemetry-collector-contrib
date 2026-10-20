// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package partitioningprocessor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

// --- parser tests ---

func TestNewMetricsPartitioner_ResourceContext(t *testing.T) {
	expressions := []string{`resource.attributes["tenant.id"]`}
	p, err := newMetricsPartitioner(expressions, componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)
	_, ok := p.(*resourceMetricsPartitioner)
	assert.True(t, ok)
}

func TestNewMetricsPartitioner_ScopeContext(t *testing.T) {
	expressions := []string{`scope.name`}
	p, err := newMetricsPartitioner(expressions, componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)
	_, ok := p.(*scopeMetricsPartitioner)
	assert.True(t, ok)
}

func TestNewMetricsPartitioner_MetricContext(t *testing.T) {
	expressions := []string{`metric.name`}
	p, err := newMetricsPartitioner(expressions, componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)
	_, ok := p.(*metricMetricsPartitioner)
	assert.True(t, ok)
}

func TestNewMetricsPartitioner_DatapointContext(t *testing.T) {
	expressions := []string{`datapoint.attributes["x"]`}
	p, err := newMetricsPartitioner(expressions, componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)
	_, ok := p.(*datapointMetricsPartitioner)
	assert.True(t, ok)
}

func TestNewMetricsPartitioner_OTelColContext(t *testing.T) {
	expressions := []string{`otelcol.client.metadata["x-tenant-id"][0]`}
	p, err := newMetricsPartitioner(expressions, componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)
	_, ok := p.(*otelcolMetricsPartitioner)
	assert.True(t, ok)
}

func TestNewMetricsPartitioner_MixedContextPromotesToDatapoint(t *testing.T) {
	expressions := []string{
		`resource.attributes["tenant.id"]`,
		`datapoint.attributes["x"]`,
	}
	p, err := newMetricsPartitioner(expressions, componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)
	_, ok := p.(*datapointMetricsPartitioner)
	assert.True(t, ok)
}

func TestNewMetricsPartitioner_InvalidExpression(t *testing.T) {
	expressions := []string{`not_a_valid_expression(`}
	_, err := newMetricsPartitioner(expressions, componenttest.NewNopTelemetrySettings())
	assert.Error(t, err)
}

// --- partitioner tests ---

func TestPartitionMetrics_ResourcePartitioning(t *testing.T) {
	md := pmetric.NewMetrics()
	for _, tenant := range []string{"t1", "t2", "t1"} {
		rm := md.ResourceMetrics().AppendEmpty()
		rm.Resource().Attributes().PutStr("tenant.id", tenant)
		m := rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
		m.SetName("req_count")
		m.SetEmptyGauge().DataPoints().AppendEmpty().SetIntValue(1)
	}

	result := partitionMetrics(t, md,
		`resource.attributes["tenant.id"]`,
	)
	require.Len(t, result, 2)

	counts := make(map[string]int)
	for _, pm := range result {
		require.Len(t, pm.values, 1)
		counts[pm.values[0].value] = pm.data.ResourceMetrics().Len()
	}
	assert.Equal(t, 2, counts["t1"])
	assert.Equal(t, 1, counts["t2"])
}

func TestPartitionMetrics_ScopePartitioning(t *testing.T) {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()

	for _, name := range []string{"scope-a", "scope-b"} {
		sm := rm.ScopeMetrics().AppendEmpty()
		sm.Scope().SetName(name)
		m := sm.Metrics().AppendEmpty()
		m.SetName("req_count")
		m.SetEmptyGauge().DataPoints().AppendEmpty().SetIntValue(1)
	}

	result := partitionMetrics(t, md,
		`scope.name`,
	)
	require.Len(t, result, 2)

	scopes := make(map[string]bool)
	for _, pm := range result {
		require.Len(t, pm.values, 1)
		scopes[pm.values[0].value] = true
	}
	assert.True(t, scopes["scope-a"])
	assert.True(t, scopes["scope-b"])
}

func TestPartitionMetrics_MetricPartitioning(t *testing.T) {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()

	for _, name := range []string{"http_requests", "cpu_usage", "http_requests"} {
		m := sm.Metrics().AppendEmpty()
		m.SetName(name)
		m.SetEmptyGauge().DataPoints().AppendEmpty().SetIntValue(1)
	}

	result := partitionMetrics(t, md,
		`metric.name`,
	)
	require.Len(t, result, 2)

	names := make(map[string]bool)
	for _, pm := range result {
		require.Len(t, pm.values, 1)
		names[pm.values[0].value] = true
	}
	assert.True(t, names["http_requests"])
	assert.True(t, names["cpu_usage"])
}

func TestPartitionMetrics_DatapointPartitioning_Gauge(t *testing.T) {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	m := sm.Metrics().AppendEmpty()
	m.SetName("req_count")
	gauge := m.SetEmptyGauge()
	for _, env := range []string{"prod", "staging", "prod"} {
		dp := gauge.DataPoints().AppendEmpty()
		dp.Attributes().PutStr("env", env)
		dp.SetIntValue(1)
	}

	result := partitionMetrics(t, md,
		`datapoint.attributes["env"]`,
	)
	require.Len(t, result, 2)

	counts := make(map[string]int)
	for _, pm := range result {
		require.Len(t, pm.values, 1)
		require.Equal(t, 1, pm.data.ResourceMetrics().Len())
		destRM := pm.data.ResourceMetrics().At(0)
		require.Equal(t, 1, destRM.ScopeMetrics().Len())
		destM := destRM.ScopeMetrics().At(0).Metrics().At(0)
		counts[pm.values[0].value] = destM.Gauge().DataPoints().Len()
	}
	assert.Equal(t, 2, counts["prod"])
	assert.Equal(t, 1, counts["staging"])
}

func TestPartitionMetrics_DatapointPartitioning_PreservesMetricMetadata(t *testing.T) {
	md := pmetric.NewMetrics()
	m := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	m.SetName("req_count")
	m.Metadata().PutStr("prometheus.type", "counter")
	gauge := m.SetEmptyGauge()
	for _, env := range []string{"prod", "staging"} {
		gauge.DataPoints().AppendEmpty().Attributes().PutStr("env", env)
	}

	result := partitionMetrics(t, md, `datapoint.attributes["env"]`)
	require.Len(t, result, 2)
	for _, pm := range result {
		destM := pm.data.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)
		v, ok := destM.Metadata().Get("prometheus.type")
		require.True(t, ok)
		assert.Equal(t, "counter", v.Str())
	}
}

func TestPartitionMetrics_DatapointPartitioning_Sum(t *testing.T) {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	m := sm.Metrics().AppendEmpty()
	m.SetName("req_count")
	sum := m.SetEmptySum()
	sum.SetIsMonotonic(true)
	sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	for _, env := range []string{"prod", "staging"} {
		dp := sum.DataPoints().AppendEmpty()
		dp.Attributes().PutStr("env", env)
		dp.SetIntValue(1)
	}

	result := partitionMetrics(t, md,
		`datapoint.attributes["env"]`,
	)
	require.Len(t, result, 2)
	for _, pm := range result {
		destM := pm.data.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)
		assert.Equal(t, pmetric.MetricTypeSum, destM.Type())
		assert.True(t, destM.Sum().IsMonotonic())
		assert.Equal(t, pmetric.AggregationTemporalityCumulative, destM.Sum().AggregationTemporality())
		assert.Equal(t, 1, destM.Sum().DataPoints().Len())
	}
}

func TestPartitionMetrics_DatapointPartitioning_Histogram(t *testing.T) {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	m := sm.Metrics().AppendEmpty()
	m.SetName("latency")
	hist := m.SetEmptyHistogram()
	hist.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
	for i, env := range []string{"prod", "staging"} {
		dp := hist.DataPoints().AppendEmpty()
		dp.Attributes().PutStr("env", env)
		dp.SetCount(1)
		dp.SetSum(float64(i + 1))
	}

	result := partitionMetrics(t, md,
		`datapoint.attributes["env"]`,
	)
	require.Len(t, result, 2)
	for _, pm := range result {
		destM := pm.data.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)
		assert.Equal(t, pmetric.MetricTypeHistogram, destM.Type())
		assert.Equal(t, pmetric.AggregationTemporalityDelta, destM.Histogram().AggregationTemporality())
		assert.Equal(t, 1, destM.Histogram().DataPoints().Len())
	}
}

func TestPartitionMetrics_DatapointPartitioning_ExponentialHistogram(t *testing.T) {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	m := sm.Metrics().AppendEmpty()
	m.SetName("latency")
	expHist := m.SetEmptyExponentialHistogram()
	expHist.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	for i, env := range []string{"prod", "staging"} {
		dp := expHist.DataPoints().AppendEmpty()
		dp.Attributes().PutStr("env", env)
		dp.SetCount(1)
		dp.SetSum(float64(i + 1))
		dp.SetScale(1)
		dp.SetZeroCount(1)
		dp.Positive().SetOffset(0)
		dp.Positive().BucketCounts().Append(1)
	}

	result := partitionMetrics(t, md,
		`datapoint.attributes["env"]`,
	)
	require.Len(t, result, 2)
	for _, pm := range result {
		destM := pm.data.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)
		assert.Equal(t, pmetric.MetricTypeExponentialHistogram, destM.Type())
		assert.Equal(t, pmetric.AggregationTemporalityCumulative, destM.ExponentialHistogram().AggregationTemporality())
		assert.Equal(t, 1, destM.ExponentialHistogram().DataPoints().Len())
	}
}

func TestPartitionMetrics_DatapointPartitioning_Summary(t *testing.T) {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	m := sm.Metrics().AppendEmpty()
	m.SetName("latency")
	summary := m.SetEmptySummary()
	for i, env := range []string{"prod", "staging"} {
		dp := summary.DataPoints().AppendEmpty()
		dp.Attributes().PutStr("env", env)
		dp.SetCount(1)
		dp.SetSum(float64(i + 1))
		qv := dp.QuantileValues().AppendEmpty()
		qv.SetQuantile(0.5)
		qv.SetValue(float64(i + 1))
	}

	result := partitionMetrics(t, md,
		`datapoint.attributes["env"]`,
	)
	require.Len(t, result, 2)
	for _, pm := range result {
		destM := pm.data.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)
		assert.Equal(t, pmetric.MetricTypeSummary, destM.Type())
		assert.Equal(t, 1, destM.Summary().DataPoints().Len())
		assert.Equal(t, 1, destM.Summary().DataPoints().At(0).QuantileValues().Len())
	}
}

func TestPartitionMetrics_PreservesSchemaURL(t *testing.T) {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	rm.SetSchemaUrl("https://example.com/resource-schema")
	sm := rm.ScopeMetrics().AppendEmpty()
	sm.SetSchemaUrl("https://example.com/scope-schema")
	m := sm.Metrics().AppendEmpty()
	m.SetName("req_count")
	m.SetEmptyGauge().DataPoints().AppendEmpty().SetIntValue(1)

	result := partitionMetrics(t, md,
		`metric.name`,
	)
	require.Len(t, result, 1)
	destRM := result[0].data.ResourceMetrics().At(0)
	assert.Equal(t, "https://example.com/resource-schema", destRM.SchemaUrl())
	assert.Equal(t, "https://example.com/scope-schema", destRM.ScopeMetrics().At(0).SchemaUrl())
}

func TestPartitionMetrics_EmptyInput(t *testing.T) {
	result := partitionMetrics(t, pmetric.NewMetrics(),
		`resource.attributes["tenant.id"]`,
	)
	assert.Empty(t, result)
}

func partitionMetrics(t *testing.T, md pmetric.Metrics, expressions ...string) []partitionedMetrics {
	t.Helper()
	p, err := newMetricsPartitioner(expressions, componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)
	result, err := p.partitionMetrics(t.Context(), md)
	require.NoError(t, err)
	return result
}
