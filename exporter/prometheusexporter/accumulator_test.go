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

			signature := timeseriesSignature(ilm2.Scope().Name(), ilm2.Metrics().At(0), m2Labels, pcommon.NewMap())
			m, ok := a.registeredMetrics.Load(signature)
			require.True(t, ok)

			v := m.(*accumulatedValue)
			vLabels, vTS, vValue, vTemporality, vIsMonotonic := getMetricProperties(ilm2.Metrics().At(0))

			require.Equal(t, "test", v.scope.Name())
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
			signature := timeseriesSignature(ilm.Scope().Name(), ilm.Metrics().At(0), mLabels, pcommon.NewMap())
			m, ok := a.registeredMetrics.Load(signature)
			require.True(t, ok)

			v := m.(*accumulatedValue)
			vLabels, vTS, vValue, vTemporality, vIsMonotonic := getMetricProperties(v.value)

			require.Equal(t, "test", v.scope.Name())
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
	appendDeltaHistogram := func(startTs time.Time, ts time.Time, count uint64, sum float64, counts []uint64, bounds []float64, metrics pmetric.MetricSlice) {
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
		signature := timeseriesSignature(ilm.Scope().Name(), ilm.Metrics().At(0), m2.Attributes(), pcommon.NewMap())

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
		signature := timeseriesSignature(ilm.Scope().Name(), ilm.Metrics().At(0), m2.Attributes(), pcommon.NewMap())

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
		signature := timeseriesSignature(ilm.Scope().Name(), ilm.Metrics().At(0), m2.Attributes(), pcommon.NewMap())

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
		signature := timeseriesSignature(ilm.Scope().Name(), ilm.Metrics().At(0), m1.Attributes(), pcommon.NewMap())

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
		signature := timeseriesSignature(ilm.Scope().Name(), ilm.Metrics().At(0), m2.Attributes(), pcommon.NewMap())

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

			signature := timeseriesSignature(ilm.Scope().Name(), ilm.Metrics().At(0), pcommon.NewMap(), pcommon.NewMap())
			v, ok := a.registeredMetrics.Load(signature)
			require.False(t, ok)
			require.Nil(t, v)
		})
	}
}

func TestTimeseriesSignatureNotMutating(t *testing.T) {
	attrs := pcommon.NewMap()
	attrs.PutStr("label_2", "2")
	attrs.PutStr("label_1", "1")
	origAttrs := pcommon.NewMap()
	attrs.CopyTo(origAttrs)
	timeseriesSignature("test_il", pmetric.NewMetric(), attrs, attrs)
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

	return
}
