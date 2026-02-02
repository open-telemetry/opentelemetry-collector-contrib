// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusexporter

import (
	"log"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestAccumulateMetrics(t *testing.T) {
	tests := []struct {
		name   string
		metric func(time.Time, float64, pmetric.MetricSlice)
	}{
		{
			name: "IntGauge",
			metric: func(ts time.Time, v float64, metrics pmetric.MetricSlice) {
				metric := metrics.AppendEmpty()
				metric.SetName("test_metric")
				metric.SetDescription("test description")
				dp := metric.SetEmptyGauge().DataPoints().AppendEmpty()
				dp.SetIntValue(int64(v))
				dp.Attributes().PutStr("label_1", "1")
				dp.Attributes().PutStr("label_2", "2")
				dp.SetTimestamp(pcommon.NewTimestampFromTime(ts))
			},
		},
		{
			name: "Gauge",
			metric: func(ts time.Time, v float64, metrics pmetric.MetricSlice) {
				metric := metrics.AppendEmpty()
				metric.SetName("test_metric")
				metric.SetDescription("test description")
				dp := metric.SetEmptyGauge().DataPoints().AppendEmpty()
				dp.SetDoubleValue(v)
				dp.Attributes().PutStr("label_1", "1")
				dp.Attributes().PutStr("label_2", "2")
				dp.SetTimestamp(pcommon.NewTimestampFromTime(ts))
			},
		},
		{
			name: "IntSum",
			metric: func(ts time.Time, v float64, metrics pmetric.MetricSlice) {
				metric := metrics.AppendEmpty()
				metric.SetName("test_metric")
				metric.SetDescription("test description")
				metric.SetEmptySum().SetIsMonotonic(false)
				metric.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				dp := metric.Sum().DataPoints().AppendEmpty()
				dp.SetIntValue(int64(v))
				dp.Attributes().PutStr("label_1", "1")
				dp.Attributes().PutStr("label_2", "2")
				dp.SetTimestamp(pcommon.NewTimestampFromTime(ts))
			},
		},
		{
			name: "Sum",
			metric: func(ts time.Time, v float64, metrics pmetric.MetricSlice) {
				metric := metrics.AppendEmpty()
				metric.SetName("test_metric")
				metric.SetDescription("test description")
				metric.SetEmptySum().SetIsMonotonic(false)
				metric.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				dp := metric.Sum().DataPoints().AppendEmpty()
				dp.SetDoubleValue(v)
				dp.Attributes().PutStr("label_1", "1")
				dp.Attributes().PutStr("label_2", "2")
				dp.SetTimestamp(pcommon.NewTimestampFromTime(ts))
			},
		},
		{
			name: "MonotonicIntSum",
			metric: func(ts time.Time, v float64, metrics pmetric.MetricSlice) {
				metric := metrics.AppendEmpty()
				metric.SetName("test_metric")
				metric.SetDescription("test description")
				metric.SetEmptySum().SetIsMonotonic(true)
				metric.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				dp := metric.Sum().DataPoints().AppendEmpty()
				dp.SetIntValue(int64(v))
				dp.Attributes().PutStr("label_1", "1")
				dp.Attributes().PutStr("label_2", "2")
				dp.SetTimestamp(pcommon.NewTimestampFromTime(ts))
			},
		},
		{
			name: "MonotonicSum",
			metric: func(ts time.Time, v float64, metrics pmetric.MetricSlice) {
				metric := metrics.AppendEmpty()
				metric.SetName("test_metric")
				metric.SetEmptySum().SetIsMonotonic(true)
				metric.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				metric.SetDescription("test description")
				dp := metric.Sum().DataPoints().AppendEmpty()
				dp.SetDoubleValue(v)
				dp.Attributes().PutStr("label_1", "1")
				dp.Attributes().PutStr("label_2", "2")
				dp.SetTimestamp(pcommon.NewTimestampFromTime(ts))
			},
		},
		{
			name: "Histogram",
			metric: func(ts time.Time, v float64, metrics pmetric.MetricSlice) {
				metric := metrics.AppendEmpty()
				metric.SetName("test_metric")
				metric.SetEmptyHistogram().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				metric.SetDescription("test description")
				dp := metric.Histogram().DataPoints().AppendEmpty()
				dp.BucketCounts().FromRaw([]uint64{5, 2})
				dp.SetCount(7)
				dp.ExplicitBounds().FromRaw([]float64{3.5, 10.0})
				dp.SetSum(v)
				dp.Attributes().PutStr("label_1", "1")
				dp.Attributes().PutStr("label_2", "2")
				dp.SetTimestamp(pcommon.NewTimestampFromTime(ts))
			},
		},
		{
			name: "Summary",
			metric: func(ts time.Time, _ float64, metrics pmetric.MetricSlice) {
				metric := metrics.AppendEmpty()
				metric.SetName("test_metric")
				metric.SetDescription("test description")
				dp := metric.SetEmptySummary().DataPoints().AppendEmpty()
				dp.SetCount(10)
				dp.SetSum(0.012)
				dp.SetCount(10)
				dp.Attributes().PutStr("label_1", "1")
				dp.Attributes().PutStr("label_2", "2")
				dp.SetTimestamp(pcommon.NewTimestampFromTime(ts))
				fillQuantileValue := func(pN, value float64, dest pmetric.SummaryDataPointValueAtQuantile) {
					dest.SetQuantile(pN)
					dest.SetValue(value)
				}
				fillQuantileValue(0.50, 190, dp.QuantileValues().AppendEmpty())
				fillQuantileValue(0.99, 817, dp.QuantileValues().AppendEmpty())
			},
		},
		{
			name: "StalenessMarkerGauge",
			metric: func(ts time.Time, v float64, metrics pmetric.MetricSlice) {
				metric := metrics.AppendEmpty()
				metric.SetName("test_metric")
				metric.SetDescription("test description")
				dp := metric.SetEmptyGauge().DataPoints().AppendEmpty()
				dp.SetDoubleValue(v)
				dp.Attributes().PutStr("label_1", "1")
				dp.Attributes().PutStr("label_2", "2")
				dp.SetTimestamp(pcommon.NewTimestampFromTime(ts))
				dp.SetFlags(pmetric.DefaultDataPointFlags.WithNoRecordedValue(true))
			},
		},
		{
			name: "StalenessMarkerSum",
			metric: func(ts time.Time, v float64, metrics pmetric.MetricSlice) {
				metric := metrics.AppendEmpty()
				metric.SetName("test_metric")
				metric.SetDescription("test description")
				metric.SetEmptySum().SetIsMonotonic(false)
				metric.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				dp := metric.Sum().DataPoints().AppendEmpty()
				dp.SetDoubleValue(v)
				dp.Attributes().PutStr("label_1", "1")
				dp.Attributes().PutStr("label_2", "2")
				dp.SetTimestamp(pcommon.NewTimestampFromTime(ts))
				dp.SetFlags(pmetric.DefaultDataPointFlags.WithNoRecordedValue(true))
			},
		},
		{
			name: "StalenessMarkerHistogram",
			metric: func(ts time.Time, v float64, metrics pmetric.MetricSlice) {
				metric := metrics.AppendEmpty()
				metric.SetName("test_metric")
				metric.SetEmptyHistogram().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				metric.SetDescription("test description")
				dp := metric.Histogram().DataPoints().AppendEmpty()
				dp.BucketCounts().FromRaw([]uint64{5, 2})
				dp.SetCount(7)
				dp.ExplicitBounds().FromRaw([]float64{3.5, 10.0})
				dp.SetSum(v)
				dp.Attributes().PutStr("label_1", "1")
				dp.Attributes().PutStr("label_2", "2")
				dp.SetTimestamp(pcommon.NewTimestampFromTime(ts))
				dp.SetFlags(pmetric.DefaultDataPointFlags.WithNoRecordedValue(true))
			},
		},
		{
			name: "StalenessMarkerSummary",
			metric: func(ts time.Time, _ float64, metrics pmetric.MetricSlice) {
				metric := metrics.AppendEmpty()
				metric.SetName("test_metric")
				metric.SetDescription("test description")
				dp := metric.SetEmptySummary().DataPoints().AppendEmpty()
				dp.SetCount(10)
				dp.SetSum(0.012)
				dp.SetCount(10)
				dp.Attributes().PutStr("label_1", "1")
				dp.Attributes().PutStr("label_2", "2")
				dp.SetTimestamp(pcommon.NewTimestampFromTime(ts))
				dp.SetFlags(pmetric.DefaultDataPointFlags.WithNoRecordedValue(true))
				fillQuantileValue := func(pN, value float64, dest pmetric.SummaryDataPointValueAtQuantile) {
					dest.SetQuantile(pN)
					dest.SetValue(value)
				}
				fillQuantileValue(0.50, 190, dp.QuantileValues().AppendEmpty())
				fillQuantileValue(0.99, 817, dp.QuantileValues().AppendEmpty())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts1 := time.Now().Add(-3 * time.Second)
			ts2 := time.Now().Add(-2 * time.Second)
			ts3 := time.Now().Add(-1 * time.Second)

			resourceMetrics2 := pmetric.NewResourceMetrics()
			ilm2 := resourceMetrics2.ScopeMetrics().AppendEmpty()
			ilm2.Scope().SetName("test")
			tt.metric(ts2, 21, ilm2.Metrics())

			tt.metric(ts1, 13, ilm2.Metrics())

			a := newAccumulator(zap.NewNop(), 1*time.Hour).(*lastValueAccumulator)

			// 2 metric arrived
			n := a.Accumulate(resourceMetrics2)
			if strings.HasPrefix(tt.name, "StalenessMarker") {
				require.Equal(t, 0, n)
				return
			}
			require.Equal(t, 1, n)

			m2Labels, _, m2Value, m2Temporality, m2IsMonotonic := getMetricProperties(ilm2.Metrics().At(0))

			signature := timeseriesSignature(ilm2.Scope().Name(), ilm2.Scope().Version(), ilm2.SchemaUrl(), ilm2.Scope().Attributes(), ilm2.Metrics().At(0), m2Labels, pcommon.NewMap())
			m, ok := a.registeredMetrics.Load(signature)
			require.True(t, ok)

			v := m.(*accumulatedValue)
			vLabels, vTS, vValue, vTemporality, vIsMonotonic := getMetricProperties(ilm2.Metrics().At(0))

			require.Equal(t, "test", v.scopeName)
			require.Equal(t, v.value.Type(), ilm2.Metrics().At(0).Type())
			for k, v := range vLabels.All() {
				r, _ := m2Labels.Get(k)
				require.Equal(t, r, v)
			}
			require.Equal(t, m2Labels.Len(), vLabels.Len())
			require.Equal(t, m2Value, vValue)
			require.Equal(t, ts2.Unix(), vTS.Unix())
			require.Greater(t, v.updated.Unix(), vTS.Unix())
			require.Equal(t, m2Temporality, vTemporality)
			require.Equal(t, m2IsMonotonic, vIsMonotonic)

			// 3 metrics arrived
			resourceMetrics3 := pmetric.NewResourceMetrics()
			ilm3 := resourceMetrics3.ScopeMetrics().AppendEmpty()
			ilm3.Scope().SetName("test")
			tt.metric(ts2, 21, ilm3.Metrics())
			tt.metric(ts3, 34, ilm3.Metrics())
			tt.metric(ts1, 13, ilm3.Metrics())

			_, _, m3Value, _, _ := getMetricProperties(ilm3.Metrics().At(1))

			n = a.Accumulate(resourceMetrics3)
			require.Equal(t, 2, n)

			m, ok = a.registeredMetrics.Load(signature)
			require.True(t, ok)
			v = m.(*accumulatedValue)
			_, vTS, vValue, _, _ = getMetricProperties(v.value)

			require.Equal(t, m3Value, vValue)
			require.Equal(t, ts3.Unix(), vTS.Unix())
		})
	}
}

func TestAccumulateDeltaToCumulative(t *testing.T) {
	tests := []struct {
		name   string
		metric func(time.Time, time.Time, float64, pmetric.MetricSlice)
	}{
		{
			name: "MonotonicDeltaIntSum",
			metric: func(startTimestamp, ts time.Time, v float64, metrics pmetric.MetricSlice) {
				metric := metrics.AppendEmpty()
				metric.SetName("test_metric")
				metric.SetDescription("test description")
				metric.SetEmptySum().SetIsMonotonic(true)
				metric.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
				dp := metric.Sum().DataPoints().AppendEmpty()
				dp.SetIntValue(int64(v))
				dp.Attributes().PutStr("label_1", "1")
				dp.Attributes().PutStr("label_2", "2")
				dp.SetStartTimestamp(pcommon.NewTimestampFromTime(startTimestamp))
				dp.SetTimestamp(pcommon.NewTimestampFromTime(ts))
			},
		},
		{
			name: "MonotonicDeltaSum",
			metric: func(startTimestamp, timestamp time.Time, v float64, metrics pmetric.MetricSlice) {
				metric := metrics.AppendEmpty()
				metric.SetName("test_metric")
				metric.SetEmptySum().SetIsMonotonic(true)
				metric.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
				metric.SetDescription("test description")
				dp := metric.Sum().DataPoints().AppendEmpty()
				dp.SetDoubleValue(v)
				dp.Attributes().PutStr("label_1", "1")
				dp.Attributes().PutStr("label_2", "2")
				dp.SetStartTimestamp(pcommon.NewTimestampFromTime(startTimestamp))
				dp.SetTimestamp(pcommon.NewTimestampFromTime(timestamp))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts1 := time.Now().Add(-3 * time.Second)
			ts2 := time.Now().Add(-2 * time.Second)
			ts3 := time.Now().Add(-1 * time.Second)

			resourceMetrics := pmetric.NewResourceMetrics()
			ilm := resourceMetrics.ScopeMetrics().AppendEmpty()
			ilm.Scope().SetName("test")
			a := newAccumulator(zap.NewNop(), 1*time.Hour).(*lastValueAccumulator)

			dataPointValue1 := float64(11)
			dataPointValue2 := float64(32)

			// The first point arrived
			tt.metric(ts1, ts2, dataPointValue1, ilm.Metrics())
			n := a.Accumulate(resourceMetrics)

			require.Equal(t, 1, n)

			// The next point arrived
			tt.metric(ts2, ts3, dataPointValue2, ilm.Metrics())
			n = a.Accumulate(resourceMetrics)

			require.Equal(t, 2, n)

			mLabels, _, mValue, _, _ := getMetricProperties(ilm.Metrics().At(1))
			signature := timeseriesSignature(ilm.Scope().Name(), ilm.Scope().Version(), ilm.SchemaUrl(), ilm.Scope().Attributes(), ilm.Metrics().At(0), mLabels, pcommon.NewMap())
			m, ok := a.registeredMetrics.Load(signature)
			require.True(t, ok)

			v := m.(*accumulatedValue)
			vLabels, vTS, vValue, vTemporality, vIsMonotonic := getMetricProperties(v.value)

			require.Equal(t, "test", v.scopeName)
			require.Equal(t, v.value.Type(), ilm.Metrics().At(0).Type())
			require.Equal(t, v.value.Type(), ilm.Metrics().At(1).Type())

			for k, v := range vLabels.All() {
				r, _ := mLabels.Get(k)
				require.Equal(t, r, v)
			}
			require.Equal(t, mLabels.Len(), vLabels.Len())
			require.Equal(t, mValue, vValue)
			require.Equal(t, dataPointValue1+dataPointValue2, vValue)
			require.Equal(t, pmetric.AggregationTemporalityCumulative, vTemporality)
			require.True(t, vIsMonotonic)

			require.Equal(t, ts3.Unix(), vTS.Unix())
		})
	}
}

func TestAccumulateDeltaToCumulativeHistogram(t *testing.T) {
	appendDeltaHistogram := func(startTs, ts time.Time, count uint64, sum float64, counts []uint64, bounds []float64, metrics pmetric.MetricSlice) {
		metric := metrics.AppendEmpty()
		metric.SetName("test_metric")
		metric.SetEmptyHistogram().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
		metric.SetDescription("test description")
		dp := metric.Histogram().DataPoints().AppendEmpty()
		dp.ExplicitBounds().FromRaw(bounds)
		dp.BucketCounts().FromRaw(counts)
		dp.SetCount(count)
		dp.SetSum(sum)
		dp.Attributes().PutStr("label_1", "1")
		dp.Attributes().PutStr("label_2", "2")
		dp.SetTimestamp(pcommon.NewTimestampFromTime(ts))
		dp.SetStartTimestamp(pcommon.NewTimestampFromTime(startTs))
	}

	t.Run("AccumulateHappyPath", func(t *testing.T) {
		startTs := time.Now().Add(-5 * time.Second)
		ts1 := time.Now().Add(-4 * time.Second)
		ts2 := time.Now().Add(-3 * time.Second)
		resourceMetrics := pmetric.NewResourceMetrics()
		ilm := resourceMetrics.ScopeMetrics().AppendEmpty()
		ilm.Scope().SetName("test")
		appendDeltaHistogram(startTs, ts1, 5, 2.5, []uint64{1, 3, 1, 0, 0}, []float64{0.1, 0.5, 1, 10}, ilm.Metrics())
		appendDeltaHistogram(ts1, ts2, 4, 8.3, []uint64{1, 1, 2, 0, 0}, []float64{0.1, 0.5, 1, 10}, ilm.Metrics())

		m1 := ilm.Metrics().At(0).Histogram().DataPoints().At(0)
		m2 := ilm.Metrics().At(1).Histogram().DataPoints().At(0)
		signature := timeseriesSignature(ilm.Scope().Name(), ilm.Scope().Version(), ilm.SchemaUrl(), ilm.Scope().Attributes(), ilm.Metrics().At(0), m2.Attributes(), pcommon.NewMap())

		a := newAccumulator(zap.NewNop(), 1*time.Hour).(*lastValueAccumulator)
		n := a.Accumulate(resourceMetrics)
		require.Equal(t, 2, n)

		m, ok := a.registeredMetrics.Load(signature)
		v := m.(*accumulatedValue).value.Histogram().DataPoints().At(0)
		require.True(t, ok)

		require.Equal(t, m1.Sum()+m2.Sum(), v.Sum())
		require.Equal(t, m1.Count()+m2.Count(), v.Count())

		for i := 0; i < v.BucketCounts().Len(); i++ {
			require.Equal(t, m1.BucketCounts().At(i)+m2.BucketCounts().At(i), v.BucketCounts().At(i))
		}

		for i := 0; i < v.ExplicitBounds().Len(); i++ {
			require.Equal(t, m2.ExplicitBounds().At(i), v.ExplicitBounds().At(i))
		}
	})
	t.Run("ResetBuckets/Ignore", func(t *testing.T) {
		startTs := time.Now().Add(-5 * time.Second)
		ts1 := time.Now().Add(-3 * time.Second)
		ts2 := time.Now().Add(-4 * time.Second)
		resourceMetrics := pmetric.NewResourceMetrics()
		ilm := resourceMetrics.ScopeMetrics().AppendEmpty()
		ilm.Scope().SetName("test")
		appendDeltaHistogram(startTs, ts1, 5, 2.5, []uint64{1, 3, 1, 0, 0}, []float64{0.1, 0.5, 1, 10}, ilm.Metrics())
		appendDeltaHistogram(startTs, ts2, 7, 5, []uint64{3, 1, 1, 0, 0}, []float64{0.1, 0.2, 1, 10}, ilm.Metrics())

		m1 := ilm.Metrics().At(0).Histogram().DataPoints().At(0)
		m2 := ilm.Metrics().At(1).Histogram().DataPoints().At(0)
		signature := timeseriesSignature(ilm.Scope().Name(), ilm.Scope().Version(), ilm.SchemaUrl(), ilm.Scope().Attributes(), ilm.Metrics().At(0), m2.Attributes(), pcommon.NewMap())

		// should ignore metric with different buckets from the past
		a := newAccumulator(zap.NewNop(), 1*time.Hour).(*lastValueAccumulator)
		n := a.Accumulate(resourceMetrics)
		require.Equal(t, 1, n)

		m, ok := a.registeredMetrics.Load(signature)
		v := m.(*accumulatedValue).value.Histogram().DataPoints().At(0)
		require.True(t, ok)

		require.Equal(t, m1.Sum(), v.Sum())
		require.Equal(t, m1.Count(), v.Count())

		for i := 0; i < v.BucketCounts().Len(); i++ {
			require.Equal(t, m1.BucketCounts().At(i), v.BucketCounts().At(i))
		}

		for i := 0; i < v.ExplicitBounds().Len(); i++ {
			require.Equal(t, m1.ExplicitBounds().At(i), v.ExplicitBounds().At(i))
		}
	})
	t.Run("ResetBuckets/Perform", func(t *testing.T) {
		// should reset when different buckets arrive
		startTs := time.Now().Add(-5 * time.Second)
		ts1 := time.Now().Add(-3 * time.Second)
		ts2 := time.Now().Add(-2 * time.Second)
		resourceMetrics := pmetric.NewResourceMetrics()
		ilm := resourceMetrics.ScopeMetrics().AppendEmpty()
		ilm.Scope().SetName("test")
		appendDeltaHistogram(startTs, ts1, 5, 2.5, []uint64{1, 3, 1, 0, 0}, []float64{0.1, 0.5, 1, 10}, ilm.Metrics())
		appendDeltaHistogram(ts1, ts2, 7, 5, []uint64{3, 1, 1, 0, 0}, []float64{0.1, 0.2, 1, 10}, ilm.Metrics())

		m2 := ilm.Metrics().At(1).Histogram().DataPoints().At(0)
		signature := timeseriesSignature(ilm.Scope().Name(), ilm.Scope().Version(), ilm.SchemaUrl(), ilm.Scope().Attributes(), ilm.Metrics().At(0), m2.Attributes(), pcommon.NewMap())

		// should ignore metric with different buckets from the past
		a := newAccumulator(zap.NewNop(), 1*time.Hour).(*lastValueAccumulator)
		n := a.Accumulate(resourceMetrics)
		require.Equal(t, 2, n)

		m, ok := a.registeredMetrics.Load(signature)
		v := m.(*accumulatedValue).value.Histogram().DataPoints().At(0)
		require.True(t, ok)

		require.Equal(t, m2.Sum(), v.Sum())
		require.Equal(t, m2.Count(), v.Count())

		for i := 0; i < v.BucketCounts().Len(); i++ {
			require.Equal(t, m2.BucketCounts().At(i), v.BucketCounts().At(i))
		}

		for i := 0; i < v.ExplicitBounds().Len(); i++ {
			require.Equal(t, m2.ExplicitBounds().At(i), v.ExplicitBounds().At(i))
		}
	})
	t.Run("MisalignedTimestamps/Drop", func(t *testing.T) {
		// should drop data points with different start time that's before latest timestamp
		startTs1 := time.Now().Add(-5 * time.Second)
		startTs2 := time.Now().Add(-4 * time.Second)
		ts1 := time.Now().Add(-3 * time.Second)
		ts2 := time.Now().Add(-2 * time.Second)
		resourceMetrics := pmetric.NewResourceMetrics()
		ilm := resourceMetrics.ScopeMetrics().AppendEmpty()
		ilm.Scope().SetName("test")
		appendDeltaHistogram(startTs1, ts1, 5, 2.5, []uint64{1, 3, 1, 0, 0}, []float64{0.1, 0.5, 1, 10}, ilm.Metrics())
		appendDeltaHistogram(startTs2, ts2, 7, 5, []uint64{3, 1, 1, 0, 0}, []float64{0.1, 0.2, 1, 10}, ilm.Metrics())

		m1 := ilm.Metrics().At(0).Histogram().DataPoints().At(0)
		signature := timeseriesSignature(ilm.Scope().Name(), ilm.Scope().Version(), ilm.SchemaUrl(), ilm.Scope().Attributes(), ilm.Metrics().At(0), m1.Attributes(), pcommon.NewMap())

		a := newAccumulator(zap.NewNop(), 1*time.Hour).(*lastValueAccumulator)
		n := a.Accumulate(resourceMetrics)
		require.Equal(t, 1, n)

		m, ok := a.registeredMetrics.Load(signature)
		v := m.(*accumulatedValue).value.Histogram().DataPoints().At(0)
		require.True(t, ok)

		require.Equal(t, m1.Sum(), v.Sum())
		require.Equal(t, m1.Count(), v.Count())

		for i := 0; i < v.BucketCounts().Len(); i++ {
			require.Equal(t, m1.BucketCounts().At(i), v.BucketCounts().At(i))
		}

		for i := 0; i < v.ExplicitBounds().Len(); i++ {
			require.Equal(t, m1.ExplicitBounds().At(i), v.ExplicitBounds().At(i))
		}
	})
	t.Run("MisalignedTimestamps/Reset", func(t *testing.T) {
		// reset when start timestamp skips ahead
		startTs1 := time.Now().Add(-5 * time.Second)
		startTs2 := time.Now().Add(-2 * time.Second)
		ts1 := time.Now().Add(-3 * time.Second)
		ts2 := time.Now().Add(-1 * time.Second)
		resourceMetrics := pmetric.NewResourceMetrics()
		ilm := resourceMetrics.ScopeMetrics().AppendEmpty()
		ilm.Scope().SetName("test")
		appendDeltaHistogram(startTs1, ts1, 5, 2.5, []uint64{1, 3, 1, 0, 0}, []float64{0.1, 0.5, 1, 10}, ilm.Metrics())
		appendDeltaHistogram(startTs2, ts2, 7, 5, []uint64{3, 1, 1, 0, 0}, []float64{0.1, 0.2, 1, 10}, ilm.Metrics())

		m2 := ilm.Metrics().At(1).Histogram().DataPoints().At(0)
		signature := timeseriesSignature(ilm.Scope().Name(), ilm.Scope().Version(), ilm.SchemaUrl(), ilm.Scope().Attributes(), ilm.Metrics().At(0), m2.Attributes(), pcommon.NewMap())

		a := newAccumulator(zap.NewNop(), 1*time.Hour).(*lastValueAccumulator)
		n := a.Accumulate(resourceMetrics)
		require.Equal(t, 2, n)

		m, ok := a.registeredMetrics.Load(signature)
		v := m.(*accumulatedValue).value.Histogram().DataPoints().At(0)
		require.True(t, ok)

		require.Equal(t, m2.Sum(), v.Sum())
		require.Equal(t, m2.Count(), v.Count())

		for i := 0; i < v.BucketCounts().Len(); i++ {
			require.Equal(t, m2.BucketCounts().At(i), v.BucketCounts().At(i))
		}

		for i := 0; i < v.ExplicitBounds().Len(); i++ {
			require.Equal(t, m2.ExplicitBounds().At(i), v.ExplicitBounds().At(i))
		}
	})
}

func TestAccumulateDroppedMetrics(t *testing.T) {
	tests := []struct {
		name       string
		fillMetric func(time.Time, pmetric.Metric)
	}{
		{
			name: "NonMonotonicIntSum",
			fillMetric: func(ts time.Time, metric pmetric.Metric) {
				metric.SetName("test_metric")
				metric.SetEmptySum().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
				metric.Sum().SetIsMonotonic(false)
				dp := metric.Sum().DataPoints().AppendEmpty()
				dp.SetIntValue(42)
				dp.Attributes().PutStr("label_1", "1")
				dp.Attributes().PutStr("label_2", "2")
				dp.SetTimestamp(pcommon.NewTimestampFromTime(ts))
			},
		},
		{
			name: "NonMonotonicSum",
			fillMetric: func(ts time.Time, metric pmetric.Metric) {
				metric.SetName("test_metric")
				metric.SetEmptySum().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
				metric.Sum().SetIsMonotonic(false)
				dp := metric.Sum().DataPoints().AppendEmpty()
				dp.SetDoubleValue(42.42)
				dp.Attributes().PutStr("label_1", "1")
				dp.Attributes().PutStr("label_2", "2")
				dp.SetTimestamp(pcommon.NewTimestampFromTime(ts))
			},
		},
		{
			name: "UnspecifiedIntSum",
			fillMetric: func(ts time.Time, metric pmetric.Metric) {
				metric.SetName("test_metric")
				metric.SetEmptySum().SetAggregationTemporality(pmetric.AggregationTemporalityUnspecified)
				dp := metric.Sum().DataPoints().AppendEmpty()
				dp.SetIntValue(42)
				dp.Attributes().PutStr("label_1", "1")
				dp.Attributes().PutStr("label_2", "2")
				dp.SetTimestamp(pcommon.NewTimestampFromTime(ts))
			},
		},
		{
			name: "UnspecifiedSum",
			fillMetric: func(ts time.Time, metric pmetric.Metric) {
				metric.SetName("test_metric")
				metric.SetEmptySum().SetAggregationTemporality(pmetric.AggregationTemporalityUnspecified)
				dp := metric.Sum().DataPoints().AppendEmpty()
				dp.SetDoubleValue(42.42)
				dp.Attributes().PutStr("label_1", "1")
				dp.Attributes().PutStr("label_2", "2")
				dp.SetTimestamp(pcommon.NewTimestampFromTime(ts))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resourceMetrics := pmetric.NewResourceMetrics()
			ilm := resourceMetrics.ScopeMetrics().AppendEmpty()
			ilm.Scope().SetName("test")
			tt.fillMetric(time.Now(), ilm.Metrics().AppendEmpty())

			a := newAccumulator(zap.NewNop(), 1*time.Hour).(*lastValueAccumulator)
			n := a.Accumulate(resourceMetrics)
			require.Equal(t, 0, n)

			signature := timeseriesSignature(ilm.Scope().Name(), ilm.Scope().Version(), ilm.SchemaUrl(), ilm.Scope().Attributes(), ilm.Metrics().At(0), pcommon.NewMap(), pcommon.NewMap())
			v, ok := a.registeredMetrics.Load(signature)
			require.False(t, ok)
			require.Nil(t, v)
		})
	}
}

func TestAccumulateDeltaToCumulativeExponentialHistogram(t *testing.T) {
	appendDeltaNative := func(startTs, ts time.Time, scale, posOff int32, pos []uint64, negOff int32, neg []uint64,
		zeroCount, count uint64, sum float64, minSet bool, minim float64, maxSet bool, maxim float64, metrics pmetric.MetricSlice,
	) pmetric.Metric {
		metric := metrics.AppendEmpty()
		metric.SetName("test_native_hist")
		metric.SetEmptyExponentialHistogram().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
		dp := metric.ExponentialHistogram().DataPoints().AppendEmpty()
		dp.SetScale(scale)
		dp.Positive().SetOffset(posOff)
		dp.Positive().BucketCounts().FromRaw(pos)
		dp.Negative().SetOffset(negOff)
		dp.Negative().BucketCounts().FromRaw(neg)
		dp.SetZeroCount(zeroCount)
		dp.SetCount(count)
		dp.SetZeroThreshold(0)
		if minSet {
			dp.SetMin(minim)
		}
		if maxSet {
			dp.SetMax(maxim)
		}
		dp.SetSum(sum)
		dp.Attributes().PutStr("label_1", "1")
		dp.Attributes().PutStr("label_2", "2")
		dp.SetStartTimestamp(pcommon.NewTimestampFromTime(startTs))
		dp.SetTimestamp(pcommon.NewTimestampFromTime(ts))
		return metric
	}

	t.Run("AccumulateAlignedMergesAndDownscales", func(t *testing.T) {
		// Two aligned deltas with different scales; verify downscale to min scale and merge.
		startTs := time.Now().Add(-6 * time.Second)
		ts1 := time.Now().Add(-5 * time.Second)
		ts2 := time.Now().Add(-4 * time.Second)

		rm := pmetric.NewResourceMetrics()
		ilm := rm.ScopeMetrics().AppendEmpty()
		ilm.Scope().SetName("test")

		// First delta: scale=2, positive offset -2 counts [1,2,3]; negative offset 0 counts [4]
		m1 := appendDeltaNative(startTs, ts1, 2, -2, []uint64{1, 2, 3}, 0, []uint64{4}, 1, 6, 3.5, true, 0.9, true, 9.0, ilm.Metrics())
		_ = m1
		// Second delta: scale=1, positive offset -1 counts [4,5]; negative offset 0 counts [1,1]
		m2 := appendDeltaNative(ts1, ts2, 1, -1, []uint64{4, 5}, 0, []uint64{1, 1}, 2, 7, 4.5, true, 0.5, false, 0, ilm.Metrics())

		a := newAccumulator(zap.NewNop(), 1*time.Hour).(*lastValueAccumulator)
		n := a.Accumulate(rm)
		require.Equal(t, 2, n)

		// Build signature using attributes of second metric
		sig := timeseriesSignature(ilm.Scope().Name(), ilm.Scope().Version(), ilm.SchemaUrl(), ilm.Scope().Attributes(), ilm.Metrics().At(0), m2.ExponentialHistogram().DataPoints().At(0).Attributes(), pcommon.NewMap())
		got, ok := a.registeredMetrics.Load(sig)
		require.True(t, ok)
		dp := got.(*accumulatedValue).value.ExponentialHistogram().DataPoints().At(0)

		// Expect scale = min(2,1) = 1
		require.Equal(t, int32(1), dp.Scale())

		// Positive buckets: prev downscales by factor 2 -> offset -1, counts [-2,-1,0] => [1+2, 3] => [3,3]; merge with current [-1:4,0:5] => [-1:7, 0:8]
		require.Equal(t, int32(-1), dp.Positive().Offset())
		require.Equal(t, 2, dp.Positive().BucketCounts().Len())
		require.Equal(t, uint64(7), dp.Positive().BucketCounts().At(0))
		require.Equal(t, uint64(8), dp.Positive().BucketCounts().At(1))

		// Negative buckets: prev scale=2 offset 0 counts [4] downscale to scale 1 with factor 2 -> offset 0, counts [4]; merge with current offset 0 counts [1,1] -> [5,1]
		require.Equal(t, int32(0), dp.Negative().Offset())
		require.Equal(t, 2, dp.Negative().BucketCounts().Len())
		require.Equal(t, uint64(5), dp.Negative().BucketCounts().At(0))
		require.Equal(t, uint64(1), dp.Negative().BucketCounts().At(1))

		// Count, ZeroCount, Sum, Min, Max
		require.Equal(t, uint64(6+7), dp.Count())
		require.Equal(t, uint64(1+2), dp.ZeroCount())
		require.InDelta(t, 3.5+4.5, dp.Sum(), 1e-12)
		require.True(t, dp.HasMin())
		require.InDelta(t, 0.5, dp.Min(), 1e-12) // min of 0.9 and 0.5
		require.True(t, dp.HasMax())
		require.InDelta(t, 9.0, dp.Max(), 1e-12) // max from first since second had no max
		require.Equal(t, ts2.Unix(), dp.Timestamp().AsTime().Unix())
	})

	t.Run("CumulativeKeepLatest", func(t *testing.T) {
		rm := pmetric.NewResourceMetrics()
		ilm := rm.ScopeMetrics().AppendEmpty()
		ilm.Scope().SetName("test")

		metric := ilm.Metrics().AppendEmpty()
		metric.SetName("test_native_hist")
		metric.SetEmptyExponentialHistogram().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
		dp1 := metric.ExponentialHistogram().DataPoints().AppendEmpty()
		dp1.SetScale(1)
		dp1.Positive().SetOffset(0)
		dp1.Positive().BucketCounts().FromRaw([]uint64{1, 2})
		dp1.Negative().SetOffset(0)
		dp1.Negative().BucketCounts().FromRaw([]uint64{0})
		dp1.SetCount(3)
		dp1.SetZeroCount(0)
		dp1.SetSum(10)
		dp1.SetTimestamp(pcommon.NewTimestampFromTime(time.Now().Add(-3 * time.Second)))
		dp1.Attributes().PutStr("label_1", "1")

		dp2 := metric.ExponentialHistogram().DataPoints().AppendEmpty()
		dp2.SetScale(1)
		dp2.Positive().SetOffset(0)
		dp2.Positive().BucketCounts().FromRaw([]uint64{4, 5})
		dp2.Negative().SetOffset(0)
		dp2.Negative().BucketCounts().FromRaw([]uint64{0})
		dp2.SetCount(9)
		dp2.SetZeroCount(0)
		dp2.SetSum(20)
		now := time.Now().Add(-2 * time.Second)
		dp2.SetTimestamp(pcommon.NewTimestampFromTime(now))
		dp2.Attributes().PutStr("label_1", "1")

		a := newAccumulator(zap.NewNop(), 1*time.Hour).(*lastValueAccumulator)
		n := a.Accumulate(rm)
		require.Equal(t, 2, n)

		sig := timeseriesSignature(ilm.Scope().Name(), ilm.Scope().Version(), ilm.SchemaUrl(), ilm.Scope().Attributes(), metric, dp2.Attributes(), pcommon.NewMap())
		got, ok := a.registeredMetrics.Load(sig)
		require.True(t, ok)
		dp := got.(*accumulatedValue).value.ExponentialHistogram().DataPoints().At(0)
		require.Equal(t, uint64(9), dp.Count())
		require.Equal(t, uint64(0), dp.ZeroCount())
		require.InDelta(t, 20.0, dp.Sum(), 1e-12)
		require.Equal(t, now.Unix(), dp.Timestamp().AsTime().Unix())
		require.Equal(t, uint64(4), dp.Positive().BucketCounts().At(0))
		require.Equal(t, uint64(5), dp.Positive().BucketCounts().At(1))
	})

	t.Run("DeltaMisalignedDropOrReset", func(t *testing.T) {
		// First: ts1; Second: start before prev end -> drop; Third: start after prev end -> reset
		start1 := time.Now().Add(-6 * time.Second)
		ts1 := time.Now().Add(-5 * time.Second)
		start2 := time.Now().Add(-6 * time.Second) // not equal to prev end; and NOT after prev end -> drop
		ts2 := time.Now().Add(-4 * time.Second)
		start3 := time.Now().Add(-3 * time.Second) // strictly after prev end -> reset
		ts3 := time.Now().Add(-2 * time.Second)

		rm := pmetric.NewResourceMetrics()
		ilm := rm.ScopeMetrics().AppendEmpty()
		ilm.Scope().SetName("test")
		m1 := appendDeltaNative(start1, ts1, 1, 0, []uint64{1}, 0, nil, 0, 1, 1.0, false, 0, false, 0, ilm.Metrics())
		_ = m1
		m2 := appendDeltaNative(start2, ts2, 1, 0, []uint64{2}, 0, nil, 0, 2, 2.0, false, 0, false, 0, ilm.Metrics())
		m3 := appendDeltaNative(start3, ts3, 1, 0, []uint64{3}, 0, nil, 0, 3, 3.0, false, 0, false, 0, ilm.Metrics())
		_ = m2
		_ = m3

		a := newAccumulator(zap.NewNop(), 1*time.Hour).(*lastValueAccumulator)
		n := a.Accumulate(rm)
		// First stored, second dropped, third reset and stored: total 2
		require.Equal(t, 2, n)

		sig := timeseriesSignature(ilm.Scope().Name(), ilm.Scope().Version(), ilm.SchemaUrl(), ilm.Scope().Attributes(), ilm.Metrics().At(0), m3.ExponentialHistogram().DataPoints().At(0).Attributes(), pcommon.NewMap())
		got, ok := a.registeredMetrics.Load(sig)
		require.True(t, ok)
		dp := got.(*accumulatedValue).value.ExponentialHistogram().DataPoints().At(0)
		require.Equal(t, uint64(3), dp.Count())
		require.InDelta(t, 3.0, dp.Sum(), 1e-12)
		require.Equal(t, uint64(3), dp.Positive().BucketCounts().At(0))
		require.Equal(t, ts3.Unix(), dp.Timestamp().AsTime().Unix())
	})
}

func TestAccumulateExponentialHistogramZeroThresholds(t *testing.T) {
	appendDeltaNativeWithZeroThreshold := func(startTs, ts time.Time, scale, posOff int32, pos []uint64, negOff int32, neg []uint64,
		zeroCount, count uint64, sum, zeroThreshold float64, metrics pmetric.MetricSlice,
	) pmetric.Metric {
		metric := metrics.AppendEmpty()
		metric.SetName("test_native_hist_zero_threshold")
		metric.SetEmptyExponentialHistogram().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
		dp := metric.ExponentialHistogram().DataPoints().AppendEmpty()
		dp.SetScale(scale)
		dp.Positive().SetOffset(posOff)
		dp.Positive().BucketCounts().FromRaw(pos)
		dp.Negative().SetOffset(negOff)
		dp.Negative().BucketCounts().FromRaw(neg)
		dp.SetZeroCount(zeroCount)
		dp.SetCount(count)
		dp.SetZeroThreshold(zeroThreshold)
		dp.SetSum(sum)
		dp.Attributes().PutStr("label_1", "1")
		dp.Attributes().PutStr("label_2", "2")
		dp.SetStartTimestamp(pcommon.NewTimestampFromTime(startTs))
		dp.SetTimestamp(pcommon.NewTimestampFromTime(ts))
		return metric
	}

	t.Run("MergeWithHigherZeroThreshold", func(t *testing.T) {
		// Test merging two histograms where one has a higher zero threshold
		// Some buckets from the histogram with lower threshold should be moved to zero count
		startTs := time.Now().Add(-6 * time.Second)
		ts1 := time.Now().Add(-5 * time.Second)
		ts2 := time.Now().Add(-4 * time.Second)

		rm := pmetric.NewResourceMetrics()
		ilm := rm.ScopeMetrics().AppendEmpty()
		ilm.Scope().SetName("test")

		// First histogram: scale=0, offset=0, zeroThreshold=0.5, buckets representing ranges [1,2), [2,4), [4,8)
		// Bucket 0: [2^0, 2^1) = [1,2), count=2
		// Bucket 1: [2^1, 2^2) = [2,4), count=3
		// Bucket 2: [2^2, 2^3) = [4,8), count=1
		appendDeltaNativeWithZeroThreshold(startTs, ts1, 0, 0, []uint64{2, 3, 1}, 0, nil, 1, 7, 15.0, 0.5, ilm.Metrics())

		// Second histogram: scale=0, offset=0, zeroThreshold=3.0, buckets representing same ranges
		// With new zero threshold of 3.0, bucket 0 [1,2) should be moved to zero count
		// Only buckets [2,4) and [4,8) should remain
		m2 := appendDeltaNativeWithZeroThreshold(ts1, ts2, 0, 0, []uint64{1, 2, 4}, 0, nil, 2, 9, 20.0, 3.0, ilm.Metrics())

		a := newAccumulator(zap.NewNop(), 1*time.Hour).(*lastValueAccumulator)
		n := a.Accumulate(rm)
		require.Equal(t, 2, n)

		sig := timeseriesSignature(ilm.Scope().Name(), ilm.Scope().Version(), ilm.SchemaUrl(), ilm.Scope().Attributes(), ilm.Metrics().At(0), m2.ExponentialHistogram().DataPoints().At(0).Attributes(), pcommon.NewMap())
		got, ok := a.registeredMetrics.Load(sig)
		require.True(t, ok)
		dp := got.(*accumulatedValue).value.ExponentialHistogram().DataPoints().At(0)

		// Expect zero threshold to be the maximum (3.0)
		require.InDelta(t, 3.0, dp.ZeroThreshold(), 1e-12)

		// Zero count should include original zero counts plus bucket 0 from first histogram
		// Original zero counts: 1 + 2 = 3, plus bucket 0 from first histogram: 2
		require.Equal(t, uint64(3+2), dp.ZeroCount())

		// After filtering first histogram: offset=1, counts=[3,1] (bucket 0 removed)
		// Second histogram: offset=0, counts=[1,2,4]
		// Merged result: offset=0, counts=[1, 3+2, 1+4] = [1, 5, 5]
		require.Equal(t, int32(0), dp.Positive().Offset())
		require.Equal(t, 3, dp.Positive().BucketCounts().Len())
		require.Equal(t, uint64(1), dp.Positive().BucketCounts().At(0)) // bucket 0: [1,2) from second histogram
		require.Equal(t, uint64(5), dp.Positive().BucketCounts().At(1)) // bucket 1: [2,4) = 3+2
		require.Equal(t, uint64(5), dp.Positive().BucketCounts().At(2)) // bucket 2: [4,8) = 1+4

		// Total count should remain the same
		require.Equal(t, uint64(16), dp.Count())

		// Sum should be sum of both histograms
		require.InDelta(t, 35.0, dp.Sum(), 1e-12)
	})

	t.Run("MergeSameZeroThreshold", func(t *testing.T) {
		// Test merging two histograms with the same zero threshold
		// No buckets should be moved to zero count
		startTs := time.Now().Add(-6 * time.Second)
		ts1 := time.Now().Add(-5 * time.Second)
		ts2 := time.Now().Add(-4 * time.Second)

		rm := pmetric.NewResourceMetrics()
		ilm := rm.ScopeMetrics().AppendEmpty()
		ilm.Scope().SetName("test")

		appendDeltaNativeWithZeroThreshold(startTs, ts1, 0, 0, []uint64{2, 3}, 0, nil, 1, 6, 10.0, 1.0, ilm.Metrics())
		m2 := appendDeltaNativeWithZeroThreshold(ts1, ts2, 0, 0, []uint64{1, 2}, 0, nil, 2, 5, 8.0, 1.0, ilm.Metrics())

		a := newAccumulator(zap.NewNop(), 1*time.Hour).(*lastValueAccumulator)
		n := a.Accumulate(rm)
		require.Equal(t, 2, n)

		sig := timeseriesSignature(ilm.Scope().Name(), ilm.Scope().Version(), ilm.SchemaUrl(), ilm.Scope().Attributes(), ilm.Metrics().At(0), m2.ExponentialHistogram().DataPoints().At(0).Attributes(), pcommon.NewMap())
		got, ok := a.registeredMetrics.Load(sig)
		require.True(t, ok)
		dp := got.(*accumulatedValue).value.ExponentialHistogram().DataPoints().At(0)

		// Zero threshold should remain 1.0
		require.InDelta(t, 1.0, dp.ZeroThreshold(), 1e-12)

		// Zero count should just be the sum of original zero counts
		require.Equal(t, uint64(1+2), dp.ZeroCount())

		// Positive buckets should be merged normally
		require.Equal(t, int32(0), dp.Positive().Offset())
		require.Equal(t, 2, dp.Positive().BucketCounts().Len())
		require.Equal(t, uint64(3), dp.Positive().BucketCounts().At(0)) // 2+1
		require.Equal(t, uint64(5), dp.Positive().BucketCounts().At(1)) // 3+2

		// Total count
		require.Equal(t, uint64(11), dp.Count())

		// Sum
		require.InDelta(t, 18.0, dp.Sum(), 1e-12)
	})

	t.Run("AllBucketsMovedToZeroCount", func(t *testing.T) {
		// Test case where all buckets from one histogram get moved to zero count
		startTs := time.Now().Add(-6 * time.Second)
		ts1 := time.Now().Add(-5 * time.Second)
		ts2 := time.Now().Add(-4 * time.Second)

		rm := pmetric.NewResourceMetrics()
		ilm := rm.ScopeMetrics().AppendEmpty()
		ilm.Scope().SetName("test")

		// First histogram: small buckets, zero threshold = 0.1
		// Buckets: [1,2), [2,4) with counts [3, 2]
		appendDeltaNativeWithZeroThreshold(startTs, ts1, 0, 0, []uint64{3, 2}, 0, nil, 1, 6, 5.0, 0.1, ilm.Metrics())

		// Second histogram: higher zero threshold = 10.0, which is greater than all bucket upper bounds from first histogram
		// Buckets: [4,8), [8,16) with counts [1, 1]
		m2 := appendDeltaNativeWithZeroThreshold(ts1, ts2, 0, 2, []uint64{1, 1}, 0, nil, 5, 7, 12.0, 10.0, ilm.Metrics())

		a := newAccumulator(zap.NewNop(), 1*time.Hour).(*lastValueAccumulator)
		n := a.Accumulate(rm)
		require.Equal(t, 2, n)

		sig := timeseriesSignature(ilm.Scope().Name(), ilm.Scope().Version(), ilm.SchemaUrl(), ilm.Scope().Attributes(), ilm.Metrics().At(0), m2.ExponentialHistogram().DataPoints().At(0).Attributes(), pcommon.NewMap())
		got, ok := a.registeredMetrics.Load(sig)
		require.True(t, ok)
		dp := got.(*accumulatedValue).value.ExponentialHistogram().DataPoints().At(0)

		// Zero threshold should be 10.0
		require.InDelta(t, 10.0, dp.ZeroThreshold(), 1e-12)

		// Zero count should include all buckets from first histogram plus original zero counts
		// Original zero counts: 1 + 5 = 6, plus all buckets from first histogram: 3 + 2 = 5
		require.Equal(t, uint64(6+5), dp.ZeroCount())

		// Positive buckets should only include buckets from second histogram that weren't filtered
		require.Equal(t, int32(2), dp.Positive().Offset())
		require.Equal(t, 2, dp.Positive().BucketCounts().Len())
		require.Equal(t, uint64(1), dp.Positive().BucketCounts().At(0)) // bucket 2: [4,8)
		require.Equal(t, uint64(1), dp.Positive().BucketCounts().At(1)) // bucket 3: [8,16)

		// Total count
		require.Equal(t, uint64(13), dp.Count())

		// Sum
		require.InDelta(t, 17.0, dp.Sum(), 1e-12)
	})

	t.Run("LowerZeroThresholdHasNoBuckets", func(t *testing.T) {
		// Test edge case where histogram with lower zero threshold has no positive buckets
		startTs := time.Now().Add(-6 * time.Second)
		ts1 := time.Now().Add(-5 * time.Second)
		ts2 := time.Now().Add(-4 * time.Second)

		rm := pmetric.NewResourceMetrics()
		ilm := rm.ScopeMetrics().AppendEmpty()
		ilm.Scope().SetName("test")

		// First histogram: only zero count, no positive buckets
		appendDeltaNativeWithZeroThreshold(startTs, ts1, 0, 0, nil, 0, nil, 5, 5, 0.0, 0.1, ilm.Metrics())

		// Second histogram: has buckets, higher zero threshold
		m2 := appendDeltaNativeWithZeroThreshold(ts1, ts2, 0, 1, []uint64{2, 3}, 0, nil, 3, 8, 15.0, 2.0, ilm.Metrics())

		a := newAccumulator(zap.NewNop(), 1*time.Hour).(*lastValueAccumulator)
		n := a.Accumulate(rm)
		require.Equal(t, 2, n)

		sig := timeseriesSignature(ilm.Scope().Name(), ilm.Scope().Version(), ilm.SchemaUrl(), ilm.Scope().Attributes(), ilm.Metrics().At(0), m2.ExponentialHistogram().DataPoints().At(0).Attributes(), pcommon.NewMap())
		got, ok := a.registeredMetrics.Load(sig)
		require.True(t, ok)
		dp := got.(*accumulatedValue).value.ExponentialHistogram().DataPoints().At(0)

		// Zero threshold should be 2.0
		require.InDelta(t, 2.0, dp.ZeroThreshold(), 1e-12)

		// Zero count should be sum of original zero counts
		require.Equal(t, uint64(5+3), dp.ZeroCount())

		// Positive buckets should be from second histogram only
		require.Equal(t, int32(1), dp.Positive().Offset())
		require.Equal(t, 2, dp.Positive().BucketCounts().Len())
		require.Equal(t, uint64(2), dp.Positive().BucketCounts().At(0))
		require.Equal(t, uint64(3), dp.Positive().BucketCounts().At(1))

		// Total count
		require.Equal(t, uint64(13), dp.Count())

		// Sum
		require.InDelta(t, 15.0, dp.Sum(), 1e-12)
	})
}

func TestTimeseriesSignatureNotMutating(t *testing.T) {
	attrs := pcommon.NewMap()
	attrs.PutStr("label_2", "2")
	attrs.PutStr("label_1", "1")
	origAttrs := pcommon.NewMap()
	attrs.CopyTo(origAttrs)
	timeseriesSignature("test_il", "1.0.0", "http://test.com", attrs, pmetric.NewMetric(), attrs, pcommon.NewMap())
	require.Equal(t, origAttrs, attrs) // make sure attrs are not mutated
}

func getMetricProperties(metric pmetric.Metric) (
	attributes pcommon.Map,
	ts time.Time,
	value float64,
	temporality pmetric.AggregationTemporality,
	isMonotonic bool,
) {
	switch metric.Type() {
	case pmetric.MetricTypeGauge:
		attributes = metric.Gauge().DataPoints().At(0).Attributes()
		ts = metric.Gauge().DataPoints().At(0).Timestamp().AsTime()
		dp := metric.Gauge().DataPoints().At(0)
		switch dp.ValueType() {
		case pmetric.NumberDataPointValueTypeInt:
			value = float64(dp.IntValue())
		case pmetric.NumberDataPointValueTypeDouble:
			value = dp.DoubleValue()
		}
		temporality = pmetric.AggregationTemporalityUnspecified
		isMonotonic = false
	case pmetric.MetricTypeSum:
		attributes = metric.Sum().DataPoints().At(0).Attributes()
		ts = metric.Sum().DataPoints().At(0).Timestamp().AsTime()
		dp := metric.Sum().DataPoints().At(0)
		switch dp.ValueType() {
		case pmetric.NumberDataPointValueTypeInt:
			value = float64(dp.IntValue())
		case pmetric.NumberDataPointValueTypeDouble:
			value = dp.DoubleValue()
		}
		temporality = metric.Sum().AggregationTemporality()
		isMonotonic = metric.Sum().IsMonotonic()
	case pmetric.MetricTypeHistogram:
		attributes = metric.Histogram().DataPoints().At(0).Attributes()
		ts = metric.Histogram().DataPoints().At(0).Timestamp().AsTime()
		value = metric.Histogram().DataPoints().At(0).Sum()
		temporality = metric.Histogram().AggregationTemporality()
		isMonotonic = true
	case pmetric.MetricTypeSummary:
		attributes = metric.Summary().DataPoints().At(0).Attributes()
		ts = metric.Summary().DataPoints().At(0).Timestamp().AsTime()
		value = metric.Summary().DataPoints().At(0).Sum()
	default:
		log.Panicf("Invalid data type %s", metric.Type().String())
	}

	return attributes, ts, value, temporality, isMonotonic
}

func TestAccumulateSum_RefusedNonMonotonicDelta_Logging(t *testing.T) {
	// Setup logger to capture log entries
	observedZapCore, observedLogs := observer.New(zap.DebugLevel)
	logger := zap.New(observedZapCore)

	accumulator := &lastValueAccumulator{
		logger:           logger,
		metricExpiration: 5 * time.Minute,
	}

	// Create non-monotonic delta sum (will be refused)
	metric := pmetric.NewMetric()
	metric.SetName("test_refused_metric")
	sum := metric.SetEmptySum()
	sum.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
	sum.SetIsMonotonic(false)

	// Add 2 data points
	for range 2 {
		dp := sum.DataPoints().AppendEmpty()
		dp.SetDoubleValue(42.0)
		dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	}

	// Call accumulateSum - should refuse and log
	result := accumulator.accumulateSum(metric, "test", "", "", pcommon.NewMap(), pcommon.NewMap(), time.Now())
	require.Equal(t, 0, result)

	// Verify debug log was written
	logs := observedLogs.All()
	require.Len(t, logs, 1, "Expected exactly one debug log entry")

	logEntry := logs[0]
	require.Equal(t, zap.DebugLevel, logEntry.Level)
	require.Equal(t, "refusing non-monotonic delta sum metric", logEntry.Message)

	// Verify log fields
	fields := logEntry.Context
	require.Len(t, fields, 3, "Expected 3 log fields")

	fieldMap := make(map[string]any)
	for _, field := range fields {
		switch field.Type {
		case zapcore.StringType:
			fieldMap[field.Key] = field.String
		case zapcore.Int64Type:
			fieldMap[field.Key] = field.Integer
		default:
			fieldMap[field.Key] = field.Interface
		}
	}

	require.Equal(t, "test_refused_metric", fieldMap["metric_name"])
	require.Equal(t, "non-monotonic sum with delta aggregation temporality is not supported", fieldMap["reason"])
	require.Equal(t, int64(2), fieldMap["data_points_refused"])
}
