// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sumologicexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sumologicexporter"

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

const (
	timestamp1 = 1618124444.169 * 1e9
	timestamp2 = 1608424699.186 * 1e9
)

func TestHistogramDecomposeNoHistogram(t *testing.T) {
	metric, resourceAttributes := exampleIntGaugeMetric()
	metrics := pmetric.NewMetrics()
	resourceAttributes.CopyTo(metrics.ResourceMetrics().AppendEmpty().Resource().Attributes())
	metric.MoveTo(metrics.ResourceMetrics().At(0).ScopeMetrics().AppendEmpty().Metrics().AppendEmpty())
	decomposedMetrics := decomposeHistograms(metrics)
	assert.Equal(t, metrics, decomposedMetrics)
}

func TestHistogramDecompose(t *testing.T) {
	metrics := metricsWithHistogram()
	decomposedMetrics := decomposeHistograms(metrics)
	assert.Equal(t, metrics.ResourceMetrics().At(0).Resource(), decomposedMetrics.ResourceMetrics().At(0).Resource())
	expectedMetrics := pmetric.NewMetrics()
	expectedResourceMetric := expectedMetrics.ResourceMetrics().AppendEmpty()
	metrics.ResourceMetrics().At(0).Resource().Attributes().CopyTo(expectedResourceMetric.Resource().Attributes())
	expectedMetricSlice := expectedResourceMetric.ScopeMetrics().AppendEmpty().Metrics()
	addExpectedHistogramSum(expectedMetricSlice)
	addExpectedHistogramCount(expectedMetricSlice)
	addExpectedHistogramBuckets(expectedMetricSlice)
	assert.Equal(t, expectedMetrics, decomposedMetrics)
}

func metricsWithHistogram() pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	resourceMetric := metrics.ResourceMetrics().AppendEmpty()
	resourceMetric.Resource().Attributes().PutStr("key", "value")
	scopeMetric := resourceMetric.ScopeMetrics().AppendEmpty()
	metric := scopeMetric.Metrics().AppendEmpty()

	metric.SetEmptyHistogram()
	metric.SetUnit("unit")
	metric.SetName("histogram_metric_double_test")
	metric.SetDescription("Test histogram metric")

	dp := metric.Histogram().DataPoints().AppendEmpty()
	dp.Attributes().PutStr("container", "dolor")

	si := pcommon.NewUInt64Slice()
	si.FromRaw([]uint64{0, 12, 7, 5, 8, 13})
	si.CopyTo(dp.BucketCounts())

	sf := pcommon.NewFloat64Slice()
	sf.FromRaw([]float64{0.1, 0.2, 0.5, 0.8, 1})
	sf.CopyTo(dp.ExplicitBounds())

	dp.SetTimestamp(timestamp1)
	dp.SetSum(45.6)
	dp.SetCount(45)

	dp = metric.Histogram().DataPoints().AppendEmpty()
	dp.Attributes().PutStr("container", "sit")

	si = pcommon.NewUInt64Slice()
	si.FromRaw([]uint64{0, 10, 1, 1, 4, 6})
	si.CopyTo(dp.BucketCounts())

	sf = pcommon.NewFloat64Slice()
	sf.FromRaw([]float64{0.1, 0.2, 0.5, 0.8, 1})
	sf.CopyTo(dp.ExplicitBounds())

	dp.SetTimestamp(timestamp2)
	dp.SetSum(54.1)
	dp.SetCount(22)

	return metrics
}

func addExpectedHistogramSum(metrics pmetric.MetricSlice) {
	metric := metrics.AppendEmpty()
	metric.SetName("histogram_metric_double_test_sum")
	metric.SetDescription("Test histogram metric")
	metric.SetUnit("unit")
	metric.SetEmptyGauge()

	dataPoint := metric.Gauge().DataPoints().AppendEmpty()
	dataPoint.Attributes().PutStr("container", "dolor")
	dataPoint.SetTimestamp(timestamp1)
	dataPoint.SetDoubleValue(45.6)

	dataPoint = metric.Gauge().DataPoints().AppendEmpty()
	dataPoint.Attributes().PutStr("container", "sit")
	dataPoint.SetTimestamp(timestamp2)
	dataPoint.SetDoubleValue(54.1)
}

func addExpectedHistogramCount(metrics pmetric.MetricSlice) {
	metric := metrics.AppendEmpty()
	metric.SetName("histogram_metric_double_test_count")
	metric.SetDescription("Test histogram metric")
	metric.SetUnit("unit")
	metric.SetEmptyGauge()

	dataPoint := metric.Gauge().DataPoints().AppendEmpty()
	dataPoint.Attributes().PutStr("container", "dolor")
	dataPoint.SetTimestamp(timestamp1)
	dataPoint.SetIntValue(45)

	dataPoint = metric.Gauge().DataPoints().AppendEmpty()
	dataPoint.Attributes().PutStr("container", "sit")
	dataPoint.SetTimestamp(timestamp2)
	dataPoint.SetIntValue(22)
}

func addExpectedHistogramBuckets(metrics pmetric.MetricSlice) {
	metric := metrics.AppendEmpty()
	metric.SetName("histogram_metric_double_test_bucket")
	metric.SetDescription("Test histogram metric")
	metric.SetUnit("unit")
	metric.SetEmptyGauge()
	histogramBuckets := []struct {
		float64
		int64
	}{
		{0.1, 0},
		{0.2, 12},
		{0.5, 19},
		{0.8, 24},
		{1, 32},
		{math.Inf(1), 45},
	}
	for _, pair := range histogramBuckets {
		bound, bucketCount := pair.float64, pair.int64
		dataPoint := metric.Gauge().DataPoints().AppendEmpty()
		dataPoint.Attributes().PutStr("container", "dolor")

		if math.IsInf(bound, 1) {
			dataPoint.Attributes().PutStr(prometheusLeTag, prometheusInfValue)
		} else {
			dataPoint.Attributes().PutDouble(prometheusLeTag, bound)
		}

		dataPoint.SetTimestamp(timestamp1)
		dataPoint.SetIntValue(bucketCount)
	}

	histogramBuckets = []struct {
		float64
		int64
	}{
		{0.1, 0},
		{0.2, 10},
		{0.5, 11},
		{0.8, 12},
		{1, 16},
		{math.Inf(1), 22},
	}
	for _, pair := range histogramBuckets {
		bound, bucketCount := pair.float64, pair.int64
		dataPoint := metric.Gauge().DataPoints().AppendEmpty()
		dataPoint.Attributes().PutStr("container", "sit")

		if math.IsInf(bound, 1) {
			dataPoint.Attributes().PutStr(prometheusLeTag, prometheusInfValue)
		} else {
			dataPoint.Attributes().PutDouble(prometheusLeTag, bound)
		}

		dataPoint.SetTimestamp(timestamp2)
		dataPoint.SetIntValue(bucketCount)
	}
}

func TestSummaryDecomposeNoSummary(t *testing.T) {
	// Test that decomposition is a no-op when no summaries are present
	metrics := pmetric.NewMetrics()
	resourceMetric := metrics.ResourceMetrics().AppendEmpty()
	resourceMetric.Resource().Attributes().PutStr("key", "value")
	scopeMetric := resourceMetric.ScopeMetrics().AppendEmpty()
	metric := scopeMetric.Metrics().AppendEmpty()

	// Add a gauge metric (not a summary)
	metric.SetEmptyGauge()
	metric.SetName("test_gauge")
	dp := metric.Gauge().DataPoints().AppendEmpty()
	dp.SetIntValue(42)
	dp.SetTimestamp(timestamp1)

	decomposedMetrics := decomposeSummaries(metrics)
	assert.Equal(t, metrics, decomposedMetrics)
}

func TestSummaryDecompose(t *testing.T) {
	metrics := metricsWithSummary()
	decomposedMetrics := decomposeSummaries(metrics)

	// Verify resource attributes are preserved
	assert.Equal(t, metrics.ResourceMetrics().At(0).Resource(), decomposedMetrics.ResourceMetrics().At(0).Resource())

	// Build expected metrics
	expectedMetrics := pmetric.NewMetrics()
	expectedResourceMetric := expectedMetrics.ResourceMetrics().AppendEmpty()
	metrics.ResourceMetrics().At(0).Resource().Attributes().CopyTo(expectedResourceMetric.Resource().Attributes())
	expectedMetricSlice := expectedResourceMetric.ScopeMetrics().AppendEmpty().Metrics()

	addExpectedSummaryQuantiles(expectedMetricSlice)
	addExpectedSummaryCount(expectedMetricSlice)
	addExpectedSummarySum(expectedMetricSlice)

	assert.Equal(t, expectedMetrics, decomposedMetrics)
}

func metricsWithSummary() pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	resourceMetric := metrics.ResourceMetrics().AppendEmpty()
	resourceMetric.Resource().Attributes().PutStr("key", "value")
	scopeMetric := resourceMetric.ScopeMetrics().AppendEmpty()
	metric := scopeMetric.Metrics().AppendEmpty()

	metric.SetEmptySummary()
	metric.SetUnit("ms")
	metric.SetName("request_duration")
	metric.SetDescription("Request duration summary")

	// First data point
	dp := metric.Summary().DataPoints().AppendEmpty()
	dp.Attributes().PutStr("service", "api")
	dp.SetTimestamp(timestamp1)
	dp.SetCount(100)
	dp.SetSum(1500.5)

	// Add quantiles for first data point
	qv := dp.QuantileValues().AppendEmpty()
	qv.SetQuantile(0.5)
	qv.SetValue(10.0)

	qv = dp.QuantileValues().AppendEmpty()
	qv.SetQuantile(0.9)
	qv.SetValue(25.5)

	qv = dp.QuantileValues().AppendEmpty()
	qv.SetQuantile(0.99)
	qv.SetValue(50.0)

	// Second data point
	dp = metric.Summary().DataPoints().AppendEmpty()
	dp.Attributes().PutStr("service", "web")
	dp.SetTimestamp(timestamp2)
	dp.SetCount(200)
	dp.SetSum(3000.75)

	// Add quantiles for second data point
	qv = dp.QuantileValues().AppendEmpty()
	qv.SetQuantile(0.5)
	qv.SetValue(12.0)

	qv = dp.QuantileValues().AppendEmpty()
	qv.SetQuantile(0.9)
	qv.SetValue(30.0)

	qv = dp.QuantileValues().AppendEmpty()
	qv.SetQuantile(0.99)
	qv.SetValue(75.5)

	return metrics
}

func addExpectedSummaryQuantiles(metrics pmetric.MetricSlice) {
	metric := metrics.AppendEmpty()
	metric.SetName("request_duration")
	metric.SetDescription("Request duration summary")
	metric.SetUnit("ms")
	metric.SetEmptyGauge()

	// Quantiles for first data point (service=api)
	summaryQuantiles := []struct {
		quantile float64
		value    float64
	}{
		{0.5, 10.0},
		{0.9, 25.5},
		{0.99, 50.0},
	}
	for _, pair := range summaryQuantiles {
		dataPoint := metric.Gauge().DataPoints().AppendEmpty()
		dataPoint.Attributes().PutStr("service", "api")
		dataPoint.Attributes().PutDouble(prometheusQuantileTag, pair.quantile)
		dataPoint.SetTimestamp(timestamp1)
		dataPoint.SetDoubleValue(pair.value)
	}

	// Quantiles for second data point (service=web)
	summaryQuantiles = []struct {
		quantile float64
		value    float64
	}{
		{0.5, 12.0},
		{0.9, 30.0},
		{0.99, 75.5},
	}
	for _, pair := range summaryQuantiles {
		dataPoint := metric.Gauge().DataPoints().AppendEmpty()
		dataPoint.Attributes().PutStr("service", "web")
		dataPoint.Attributes().PutDouble(prometheusQuantileTag, pair.quantile)
		dataPoint.SetTimestamp(timestamp2)
		dataPoint.SetDoubleValue(pair.value)
	}
}

func addExpectedSummaryCount(metrics pmetric.MetricSlice) {
	metric := metrics.AppendEmpty()
	metric.SetName("request_duration_count")
	metric.SetDescription("Request duration summary")
	metric.SetUnit("1")
	metric.SetEmptySum()
	metric.Sum().SetIsMonotonic(true)
	metric.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)

	dataPoint := metric.Sum().DataPoints().AppendEmpty()
	dataPoint.Attributes().PutStr("service", "api")
	dataPoint.SetTimestamp(timestamp1)
	dataPoint.SetIntValue(100)

	dataPoint = metric.Sum().DataPoints().AppendEmpty()
	dataPoint.Attributes().PutStr("service", "web")
	dataPoint.SetTimestamp(timestamp2)
	dataPoint.SetIntValue(200)
}

func addExpectedSummarySum(metrics pmetric.MetricSlice) {
	metric := metrics.AppendEmpty()
	metric.SetName("request_duration_sum")
	metric.SetDescription("Request duration summary")
	metric.SetUnit("ms")
	metric.SetEmptySum()
	metric.Sum().SetIsMonotonic(true)
	metric.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)

	dataPoint := metric.Sum().DataPoints().AppendEmpty()
	dataPoint.Attributes().PutStr("service", "api")
	dataPoint.SetTimestamp(timestamp1)
	dataPoint.SetDoubleValue(1500.5)

	dataPoint = metric.Sum().DataPoints().AppendEmpty()
	dataPoint.Attributes().PutStr("service", "web")
	dataPoint.SetTimestamp(timestamp2)
	dataPoint.SetDoubleValue(3000.75)
}
