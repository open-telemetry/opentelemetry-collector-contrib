// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package protocol

import (
	"testing"
	"time"

	"github.com/lightstep/go-expohisto/structure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	assert.Equal(t, expectedMetrics, metric)
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
	assert.Equal(t, expectedMetrics, metric)
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

func TestBuildSummaryMetricSampled(t *testing.T) {
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

func TestBuildHistogramMetric(t *testing.T) {
	timeNow := time.Now()
	startTime := timeNow.Add(-5 * time.Second)

	attrs := attribute.NewSet(
		attribute.String("mykey", "myvalue"),
		attribute.String("mykey2", "myvalue2"),
	)

	desc := statsDMetricDescription{
		name:       "testHistogram",
		metricType: HistogramType,
		attrs:      attrs,
	}

	agg := new(histogramStructure)
	cfg := structure.NewConfig(structure.WithMaxSize(10))
	agg.Init(cfg)
	agg.UpdateByIncr(2, 2)
	agg.UpdateByIncr(2, 1)
	agg.UpdateByIncr(2, 4)
	agg.UpdateByIncr(1, 1)
	agg.UpdateByIncr(8, 3)
	agg.UpdateByIncr(0.5, 8)
	agg.UpdateByIncr(-5, 8)

	histMetric := histogramMetric{
		agg: agg,
	}

	ilm := pmetric.NewScopeMetrics()

	buildHistogramMetric(desc, histMetric, startTime, timeNow, ilm)

	require.NotNil(t, ilm.Metrics())
	require.Equal(t, "testHistogram", ilm.Metrics().At(0).Name())

	hist := ilm.Metrics().At(0).ExponentialHistogram()
	require.NotNil(t, hist)
	require.Equal(t, 1, hist.DataPoints().Len())
	require.Equal(t, pmetric.AggregationTemporalityDelta, hist.AggregationTemporality())

	datapoint := hist.DataPoints().At(0)
	require.Equal(t, uint64(27), datapoint.Count())
	require.Equal(t, int32(1), datapoint.Scale())
	require.Equal(t, 3.0, datapoint.Sum())
	require.Equal(t, int32(-3), datapoint.Positive().Offset())
	require.Equal(t, int32(4), datapoint.Negative().Offset())
	require.Equal(t, -5.0, datapoint.Min())
	require.Equal(t, 8.0, datapoint.Max())
	val, _ := datapoint.Attributes().Get("mykey")
	require.Equal(t, "myvalue", val.Str())
	val, _ = datapoint.Attributes().Get("mykey2")
	require.Equal(t, "myvalue2", val.Str())

}
