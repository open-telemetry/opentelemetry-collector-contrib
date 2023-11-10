// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package protocol

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/otel/attribute"
)

func TestBuildCounterMetric(t *testing.T) {
	metricDescription := statsDMetricDescription{
		name:  "testCounter",
		attrs: attribute.NewSet(attribute.String("mykey", "myvalue")),
	}

	parsedMetric := statsDMetric{
		description: metricDescription,
		asFloat:     32,
		unit:        "meter",
	}
	isMonotonicCounter := false
	metric := buildCounterMetric(parsedMetric, isMonotonicCounter)
	expectedMetrics := pmetric.NewScopeMetrics()
	expectedMetric := expectedMetrics.Metrics().AppendEmpty()
	expectedMetric.SetName("testCounter")
	expectedMetric.SetUnit("meter")
	expectedMetric.SetEmptySum().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
	expectedMetric.Sum().SetIsMonotonic(isMonotonicCounter)
	dp := expectedMetric.Sum().DataPoints().AppendEmpty()
	dp.SetIntValue(32)
	dp.Attributes().PutStr("mykey", "myvalue")
	assert.Equal(t, metric, expectedMetrics)
}

func TestSetTimestampsForCounterMetric(t *testing.T) {
	timeNow := time.Now()
	lastUpdateInterval := timeNow.Add(-1 * time.Minute)

	parsedMetric := statsDMetric{}
	isMonotonicCounter := false
	metric := buildCounterMetric(parsedMetric, isMonotonicCounter)
	setTimestampsForCounterMetric(metric, lastUpdateInterval, timeNow)

	expectedMetrics := pmetric.NewScopeMetrics()
	expectedMetric := expectedMetrics.Metrics().AppendEmpty()
	expectedMetric.SetEmptySum().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
	dp := expectedMetric.Sum().DataPoints().AppendEmpty()
	dp.SetStartTimestamp(pcommon.NewTimestampFromTime(lastUpdateInterval))
	dp.SetTimestamp(pcommon.NewTimestampFromTime(timeNow))
	assert.Equal(t,
		metric.Metrics().At(0).Sum().DataPoints().At(0).StartTimestamp(),
		expectedMetrics.Metrics().At(0).Sum().DataPoints().At(0).StartTimestamp(),
	)
	assert.Equal(t,
		metric.Metrics().At(0).Sum().DataPoints().At(0).Timestamp(),
		expectedMetrics.Metrics().At(0).Sum().DataPoints().At(0).Timestamp(),
	)

}

func TestBuildGaugeMetric(t *testing.T) {
	timeNow := time.Now()
	metricDescription := statsDMetricDescription{
		name: "testGauge",
		attrs: attribute.NewSet(
			attribute.String("mykey", "myvalue"),
			attribute.String("mykey2", "myvalue2"),
		),
	}
	parsedMetric := statsDMetric{
		description: metricDescription,
		asFloat:     32.3,
		unit:        "meter",
	}
	metric := buildGaugeMetric(parsedMetric, timeNow)
	expectedMetrics := pmetric.NewScopeMetrics()
	expectedMetric := expectedMetrics.Metrics().AppendEmpty()
	expectedMetric.SetName("testGauge")
	expectedMetric.SetUnit("meter")
	dp := expectedMetric.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.SetDoubleValue(32.3)
	dp.SetTimestamp(pcommon.NewTimestampFromTime(timeNow))
	dp.Attributes().PutStr("mykey", "myvalue")
	dp.Attributes().PutStr("mykey2", "myvalue2")
	assert.Equal(t, metric, expectedMetrics)
}

func TestBuildSummaryMetricUnsampled(t *testing.T) {
	timeNow := time.Now()

	unsampledMetric := summaryMetric{
		points:  []float64{1, 2, 4, 6, 5, 3},
		weights: []float64{1, 1, 1, 1, 1, 1},
	}

	attrs := attribute.NewSet(
		attribute.String("mykey", "myvalue"),
		attribute.String("mykey2", "myvalue2"),
	)

	desc := statsDMetricDescription{
		name:       "testSummary",
		metricType: HistogramType,
		attrs:      attrs,
	}

	metric := pmetric.NewScopeMetrics()
	buildSummaryMetric(desc, unsampledMetric, timeNow.Add(-time.Minute), timeNow, statsDDefaultPercentiles, metric)

	expectedMetric := pmetric.NewScopeMetrics()
	m := expectedMetric.Metrics().AppendEmpty()
	m.SetName("testSummary")
	dp := m.SetEmptySummary().DataPoints().AppendEmpty()
	dp.SetSum(21)
	dp.SetCount(6)
	dp.SetStartTimestamp(pcommon.NewTimestampFromTime(timeNow.Add(-time.Minute)))
	dp.SetTimestamp(pcommon.NewTimestampFromTime(timeNow))
	for _, kv := range desc.attrs.ToSlice() {
		dp.Attributes().PutStr(string(kv.Key), kv.Value.AsString())
	}
	quantile := []float64{0, 10, 50, 90, 95, 100}
	value := []float64{1, 1, 3, 6, 6, 6}
	for int, v := range quantile {
		eachQuantile := dp.QuantileValues().AppendEmpty()
		eachQuantile.SetQuantile(v / 100)
		eachQuantileValue := value[int]
		eachQuantile.SetValue(eachQuantileValue)
	}

	assert.Equal(t, expectedMetric, metric)
}

func TestBuildDatadogSummaryMetricSampled(t *testing.T) {
	timeNow := time.Now()

	type testCase struct {
		points      []float64
		weights     []float64
		count       uint64
		sum         float64
		percentiles []float64
		values      []float64
	}

	for _, test := range []testCase{
		{
			points:      []float64{1, 2, 3},
			weights:     []float64{100, 1, 100},
			count:       201,
			sum:         402,
			percentiles: []float64{0, 1, 49, 50, 51, 99, 100},
			values:      []float64{1, 1, 1, 2, 3, 3, 3},
		},
		{
			points:      []float64{1, 2},
			weights:     []float64{99, 1},
			count:       100,
			sum:         101,
			percentiles: []float64{0, 98, 99, 100},
			values:      []float64{1, 1, 1, 2},
		},
		{
			points:      []float64{0, 1, 2, 3, 4, 5},
			weights:     []float64{1, 9, 40, 40, 5, 5},
			count:       100,
			sum:         254,
			percentiles: statsDDefaultPercentiles,
			values:      []float64{0, 1, 2, 3, 4, 5},
		},
	} {
		sampledMetric := summaryMetric{
			points:  test.points,
			weights: test.weights,
		}

		attrs := attribute.NewSet(
			attribute.String("mykey", "myvalue"),
			attribute.String("mykey2", "myvalue2"),
		)

		desc := statsDMetricDescription{
			name:       "testSummary",
			metricType: HistogramType,
			attrs:      attrs,
		}

		metric := pmetric.NewScopeMetrics()
		buildSummaryMetric(desc, sampledMetric, timeNow.Add(-time.Minute), timeNow, test.percentiles, metric)

		expectedMetric := pmetric.NewScopeMetrics()
		m := expectedMetric.Metrics().AppendEmpty()
		m.SetName("testSummary")
		dp := m.SetEmptySummary().DataPoints().AppendEmpty()

		dp.SetSum(test.sum)
		dp.SetCount(test.count)

		dp.SetStartTimestamp(pcommon.NewTimestampFromTime(timeNow.Add(-time.Minute)))
		dp.SetTimestamp(pcommon.NewTimestampFromTime(timeNow))
		for _, kv := range desc.attrs.ToSlice() {
			dp.Attributes().PutStr(string(kv.Key), kv.Value.AsString())
		}
		for i := range test.percentiles {
			eachQuantile := dp.QuantileValues().AppendEmpty()
			eachQuantile.SetQuantile(test.percentiles[i] / 100)
			eachQuantile.SetValue(test.values[i])
		}

		assert.Equal(t, expectedMetric, metric)
	}
}

func TestBuildDatadogSummaryMetricUnsampled(t *testing.T) {
	timeNow := time.Now()

	unsampledMetric := summaryMetric{
		points:  []float64{1, 2, 4, 6, 5, 3},
		weights: []float64{1, 1, 1, 1, 1, 1},
	}

	attrs := attribute.NewSet(
		attribute.String("mykey", "myvalue"),
		attribute.String("mykey2", "myvalue2"),
	)

	desc := statsDMetricDescription{
		name:       "testSummary",
		metricType: HistogramType,
		attrs:      attrs,
	}

	metric := pmetric.NewScopeMetrics()
	buildDatadogSummaryMetric(desc, unsampledMetric, timeNow.Add(-time.Minute), timeNow, metric)

	expectedMetric := pmetric.NewScopeMetrics()
	startTimestamp := pcommon.NewTimestampFromTime(timeNow.Add(-time.Minute))
	nowTimestamp := pcommon.NewTimestampFromTime(timeNow)

	mCount := expectedMetric.Metrics().AppendEmpty()
	mCount.SetName("testSummary.count")
	mCount.SetEmptySum()
	mCount.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
	mCount.Sum().SetIsMonotonic(true)
	dpCount := mCount.Sum().DataPoints().AppendEmpty()
	dpCount.SetIntValue(6)
	setCommonDatapointFields(dpCount, startTimestamp, nowTimestamp, desc)

	mAvg := expectedMetric.Metrics().AppendEmpty()
	mAvg.SetName("testSummary.avg")
	mAvg.SetEmptyGauge()
	dpAvg := mAvg.Gauge().DataPoints().AppendEmpty()
	dpAvg.SetDoubleValue(3.5)
	setCommonDatapointFields(dpAvg, startTimestamp, nowTimestamp, desc)

	mMedian := expectedMetric.Metrics().AppendEmpty()
	mMedian.SetName("testSummary.median")
	mMedian.SetEmptyGauge()
	dpMedian := mMedian.Gauge().DataPoints().AppendEmpty()
	dpMedian.SetDoubleValue(3.5)
	setCommonDatapointFields(dpMedian, startTimestamp, nowTimestamp, desc)

	mMax := expectedMetric.Metrics().AppendEmpty()
	mMax.SetName("testSummary.max")
	mMax.SetEmptyGauge()
	dpMax := mMax.Gauge().DataPoints().AppendEmpty()
	dpMax.SetDoubleValue(6)
	setCommonDatapointFields(dpMax, startTimestamp, nowTimestamp, desc)

	mP95 := expectedMetric.Metrics().AppendEmpty()
	mP95.SetName("testSummary.95percentile")
	mP95.SetEmptyGauge()
	dpP95 := mP95.Gauge().DataPoints().AppendEmpty()
	dpP95.SetDoubleValue(6)
	setCommonDatapointFields(dpP95, startTimestamp, nowTimestamp, desc)

	assert.Equal(t, expectedMetric, metric)
}

func TestBuildSummaryMetricSampled(t *testing.T) {
	timeNow := time.Now()

	sampledMetric := summaryMetric{
		points:  []float64{0, 1, 2, 3, 4, 5},
		weights: []float64{1, 9, 40, 40, 5, 5},
	}

	attrs := attribute.NewSet(
		attribute.String("mykey", "myvalue"),
		attribute.String("mykey2", "myvalue2"),
	)

	desc := statsDMetricDescription{
		name:       "testSummary",
		metricType: HistogramType,
		attrs:      attrs,
	}

	metric := pmetric.NewScopeMetrics()
	buildDatadogSummaryMetric(desc, sampledMetric, timeNow.Add(-time.Minute), timeNow, metric)

	expectedMetric := pmetric.NewScopeMetrics()
	startTimestamp := pcommon.NewTimestampFromTime(timeNow.Add(-time.Minute))
	nowTimestamp := pcommon.NewTimestampFromTime(timeNow)

	mCount := expectedMetric.Metrics().AppendEmpty()
	mCount.SetName("testSummary.count")
	mCount.SetEmptySum()
	mCount.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
	mCount.Sum().SetIsMonotonic(true)
	dpCount := mCount.Sum().DataPoints().AppendEmpty()
	dpCount.SetIntValue(100)
	setCommonDatapointFields(dpCount, startTimestamp, nowTimestamp, desc)

	mAvg := expectedMetric.Metrics().AppendEmpty()
	mAvg.SetName("testSummary.avg")
	mAvg.SetEmptyGauge()
	dpAvg := mAvg.Gauge().DataPoints().AppendEmpty()
	dpAvg.SetDoubleValue(42.333333333333336)
	setCommonDatapointFields(dpAvg, startTimestamp, nowTimestamp, desc)

	mMedian := expectedMetric.Metrics().AppendEmpty()
	mMedian.SetName("testSummary.median")
	mMedian.SetEmptyGauge()
	dpMedian := mMedian.Gauge().DataPoints().AppendEmpty()
	dpMedian.SetDoubleValue(22.5)
	setCommonDatapointFields(dpMedian, startTimestamp, nowTimestamp, desc)

	mMax := expectedMetric.Metrics().AppendEmpty()
	mMax.SetName("testSummary.max")
	mMax.SetEmptyGauge()
	dpMax := mMax.Gauge().DataPoints().AppendEmpty()
	dpMax.SetDoubleValue(120)
	setCommonDatapointFields(dpMax, startTimestamp, nowTimestamp, desc)

	mP95 := expectedMetric.Metrics().AppendEmpty()
	mP95.SetName("testSummary.95percentile")
	mP95.SetEmptyGauge()
	dpP95 := mP95.Gauge().DataPoints().AppendEmpty()
	dpP95.SetDoubleValue(120)
	setCommonDatapointFields(dpP95, startTimestamp, nowTimestamp, desc)

	assert.Equal(t, expectedMetric, metric)
}

func setCommonDatapointFields(dp pmetric.NumberDataPoint, start pcommon.Timestamp, now pcommon.Timestamp, desc statsDMetricDescription) {
	dp.SetStartTimestamp(start)
	dp.SetTimestamp(now)
	for _, kv := range desc.attrs.ToSlice() {
		dp.Attributes().PutStr(string(kv.Key), kv.Value.AsString())
	}
}
