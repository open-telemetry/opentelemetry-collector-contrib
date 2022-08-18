// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package prometheusexporter

import (
	"log"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

func TestInvalidDataType(t *testing.T) {
	a := newAccumulator(zap.NewNop(), 1*time.Hour).(*lastValueAccumulator)
	metric := pmetric.NewMetric()
	metric.SetDataType(-100)
	n := a.addMetric(metric, pcommon.NewInstrumentationScope(), pcommon.NewMap(), time.Now())
	require.Zero(t, n)
}

func TestAccumulateHistogram(t *testing.T) {
	appendHistogram := func(ts time.Time, count uint64, sum float64, counts []uint64, bounds []float64, metrics pmetric.MetricSlice) {
		metric := metrics.AppendEmpty()
		metric.SetName("test_metric")
		metric.SetDataType(pmetric.MetricDataTypeHistogram)
		metric.Histogram().SetAggregationTemporality(pmetric.MetricAggregationTemporalityDelta)
		metric.SetDescription("test description")
		dp := metric.Histogram().DataPoints().AppendEmpty()
		dp.SetBucketCounts(pcommon.NewImmutableUInt64Slice(counts))
		dp.SetCount(count)
		dp.SetExplicitBounds(pcommon.NewImmutableFloat64Slice(bounds))
		dp.SetSum(sum)
		dp.Attributes().InsertString("label_1", "1")
		dp.Attributes().InsertString("label_2", "2")
		dp.SetTimestamp(pcommon.NewTimestampFromTime(ts))
	}

	ts1 := time.Now().Add(-4 * time.Second)
	ts2 := time.Now().Add(-3 * time.Second)
	ts3 := time.Now().Add(-2 * time.Second)
	ts4 := time.Now().Add(-1 * time.Second)

	a := newAccumulator(zap.NewNop(), 1*time.Hour).(*lastValueAccumulator)

	resourceMetrics1 := pmetric.NewResourceMetrics()
	ilm1 := resourceMetrics1.ScopeMetrics().AppendEmpty()
	ilm1.Scope().SetName("test")
	appendHistogram(ts3, 5, 2.5, []uint64{1, 3, 1, 0}, []float64{0.1, 0.5, 1, 10}, ilm1.Metrics())
	appendHistogram(ts2, 4, 8.3, []uint64{1, 1, 2, 0}, []float64{0.1, 0.5, 1, 10}, ilm1.Metrics())

	m3 := ilm1.Metrics().At(0).Histogram().DataPoints().At(0)
	m2 := ilm1.Metrics().At(1).Histogram().DataPoints().At(0)
	signature := timeseriesSignature(ilm1.Scope().Name(), ilm1.Metrics().At(0), m2.Attributes(), pcommon.NewMap())

	// different buckets from the past
	resourceMetrics2 := pmetric.NewResourceMetrics()
	ilm2 := resourceMetrics2.ScopeMetrics().AppendEmpty()
	ilm2.Scope().SetName("test")
	appendHistogram(ts1, 7, 5, []uint64{3, 1, 1, 0}, []float64{0.1, 0.2, 1, 10}, ilm2.Metrics())

	// add extra buckets
	resourceMetrics3 := pmetric.NewResourceMetrics()
	ilm3 := resourceMetrics3.ScopeMetrics().AppendEmpty()
	ilm3.Scope().SetName("test")
	appendHistogram(ts4, 7, 5, []uint64{3, 1, 1, 0, 0}, []float64{0.1, 0.2, 1, 10, 15}, ilm3.Metrics())

	m4 := ilm3.Metrics().At(0).Histogram().DataPoints().At(0)

	t.Run("Accumulate", func(t *testing.T) {
		n := a.Accumulate(resourceMetrics1)
		require.Equal(t, 2, n)

		m, ok := a.registeredMetrics.Load(signature)
		v := m.(*accumulatedValue).value.Histogram().DataPoints().At(0)
		require.True(t, ok)

		require.Equal(t, m3.Sum()+m2.Sum(), v.Sum())
		require.Equal(t, m3.Count()+m2.Count(), v.Count())

		for i := 0; i < v.BucketCounts().Len(); i++ {
			require.Equal(t, m3.BucketCounts().At(i)+m2.BucketCounts().At(i), v.BucketCounts().At(i))
		}

		for i := 0; i < v.ExplicitBounds().Len(); i++ {
			require.Equal(t, m3.ExplicitBounds().At(i), v.ExplicitBounds().At(i))
		}
	})
	t.Run("ResetBuckets/Ignore", func(t *testing.T) {
		// should ignore metric from the past
		n := a.Accumulate(resourceMetrics2)
		require.Equal(t, 1, n)

		m, ok := a.registeredMetrics.Load(signature)
		v := m.(*accumulatedValue).value.Histogram().DataPoints().At(0)
		require.True(t, ok)

		require.Equal(t, m3.Sum()+m2.Sum(), v.Sum())
		require.Equal(t, m3.Count()+m2.Count(), v.Count())

		for i := 0; i < v.BucketCounts().Len(); i++ {
			require.Equal(t, m3.BucketCounts().At(i)+m2.BucketCounts().At(i), v.BucketCounts().At(i))
		}

		for i := 0; i < v.ExplicitBounds().Len(); i++ {
			require.Equal(t, m3.ExplicitBounds().At(i), v.ExplicitBounds().At(i))
		}
	})
	t.Run("ResetBuckets/Perform", func(t *testing.T) {
		// should reset when different buckets arrive
		n := a.Accumulate(resourceMetrics3)
		require.Equal(t, 1, n)

		m, ok := a.registeredMetrics.Load(signature)
		v := m.(*accumulatedValue).value.Histogram().DataPoints().At(0)
		require.True(t, ok)

		require.Equal(t, m4.Sum(), v.Sum())
		require.Equal(t, m4.Count(), v.Count())

		for i := 0; i < v.BucketCounts().Len(); i++ {
			require.Equal(t, m4.BucketCounts().At(i), v.BucketCounts().At(i))
		}

		for i := 0; i < v.ExplicitBounds().Len(); i++ {
			require.Equal(t, m4.ExplicitBounds().At(i), v.ExplicitBounds().At(i))
		}
	})
}

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
				metric.SetDataType(pmetric.MetricDataTypeGauge)
				metric.SetDescription("test description")
				dp := metric.Gauge().DataPoints().AppendEmpty()
				dp.SetIntVal(int64(v))
				dp.Attributes().InsertString("label_1", "1")
				dp.Attributes().InsertString("label_2", "2")
				dp.SetTimestamp(pcommon.NewTimestampFromTime(ts))
			},
		},
		{
			name: "Gauge",
			metric: func(ts time.Time, v float64, metrics pmetric.MetricSlice) {
				metric := metrics.AppendEmpty()
				metric.SetName("test_metric")
				metric.SetDataType(pmetric.MetricDataTypeGauge)
				metric.SetDescription("test description")
				dp := metric.Gauge().DataPoints().AppendEmpty()
				dp.SetDoubleVal(v)
				dp.Attributes().InsertString("label_1", "1")
				dp.Attributes().InsertString("label_2", "2")
				dp.SetTimestamp(pcommon.NewTimestampFromTime(ts))
			},
		},
		{
			name: "IntSum",
			metric: func(ts time.Time, v float64, metrics pmetric.MetricSlice) {
				metric := metrics.AppendEmpty()
				metric.SetName("test_metric")
				metric.SetDataType(pmetric.MetricDataTypeSum)
				metric.SetDescription("test description")
				metric.Sum().SetIsMonotonic(false)
				metric.Sum().SetAggregationTemporality(pmetric.MetricAggregationTemporalityCumulative)
				dp := metric.Sum().DataPoints().AppendEmpty()
				dp.SetIntVal(int64(v))
				dp.Attributes().InsertString("label_1", "1")
				dp.Attributes().InsertString("label_2", "2")
				dp.SetTimestamp(pcommon.NewTimestampFromTime(ts))
			},
		},
		{
			name: "Sum",
			metric: func(ts time.Time, v float64, metrics pmetric.MetricSlice) {
				metric := metrics.AppendEmpty()
				metric.SetName("test_metric")
				metric.SetDataType(pmetric.MetricDataTypeSum)
				metric.SetDescription("test description")
				metric.Sum().SetIsMonotonic(false)
				metric.Sum().SetAggregationTemporality(pmetric.MetricAggregationTemporalityCumulative)
				dp := metric.Sum().DataPoints().AppendEmpty()
				dp.SetDoubleVal(v)
				dp.Attributes().InsertString("label_1", "1")
				dp.Attributes().InsertString("label_2", "2")
				dp.SetTimestamp(pcommon.NewTimestampFromTime(ts))
			},
		},
		{
			name: "MonotonicIntSum",
			metric: func(ts time.Time, v float64, metrics pmetric.MetricSlice) {
				metric := metrics.AppendEmpty()
				metric.SetName("test_metric")
				metric.SetDataType(pmetric.MetricDataTypeSum)
				metric.SetDescription("test description")
				metric.Sum().SetIsMonotonic(true)
				metric.Sum().SetAggregationTemporality(pmetric.MetricAggregationTemporalityCumulative)
				dp := metric.Sum().DataPoints().AppendEmpty()
				dp.SetIntVal(int64(v))
				dp.Attributes().InsertString("label_1", "1")
				dp.Attributes().InsertString("label_2", "2")
				dp.SetTimestamp(pcommon.NewTimestampFromTime(ts))
			},
		},
		{
			name: "MonotonicSum",
			metric: func(ts time.Time, v float64, metrics pmetric.MetricSlice) {
				metric := metrics.AppendEmpty()
				metric.SetName("test_metric")
				metric.SetDataType(pmetric.MetricDataTypeSum)
				metric.Sum().SetIsMonotonic(true)
				metric.Sum().SetAggregationTemporality(pmetric.MetricAggregationTemporalityCumulative)
				metric.SetDescription("test description")
				dp := metric.Sum().DataPoints().AppendEmpty()
				dp.SetDoubleVal(v)
				dp.Attributes().InsertString("label_1", "1")
				dp.Attributes().InsertString("label_2", "2")
				dp.SetTimestamp(pcommon.NewTimestampFromTime(ts))
			},
		},
		{
			name: "Histogram",
			metric: func(ts time.Time, v float64, metrics pmetric.MetricSlice) {
				metric := metrics.AppendEmpty()
				metric.SetName("test_metric")
				metric.SetDataType(pmetric.MetricDataTypeHistogram)
				metric.Histogram().SetAggregationTemporality(pmetric.MetricAggregationTemporalityCumulative)
				metric.SetDescription("test description")
				dp := metric.Histogram().DataPoints().AppendEmpty()
				dp.SetBucketCounts(pcommon.NewImmutableUInt64Slice([]uint64{5, 2}))
				dp.SetCount(7)
				dp.SetExplicitBounds(pcommon.NewImmutableFloat64Slice([]float64{3.5, 10.0}))
				dp.SetSum(v)
				dp.Attributes().InsertString("label_1", "1")
				dp.Attributes().InsertString("label_2", "2")
				dp.SetTimestamp(pcommon.NewTimestampFromTime(ts))
			},
		},
		{
			name: "Summary",
			metric: func(ts time.Time, v float64, metrics pmetric.MetricSlice) {
				metric := metrics.AppendEmpty()
				metric.SetName("test_metric")
				metric.SetDataType(pmetric.MetricDataTypeSummary)
				metric.SetDescription("test description")
				dp := metric.Summary().DataPoints().AppendEmpty()
				dp.SetCount(10)
				dp.SetSum(0.012)
				dp.SetCount(10)
				dp.Attributes().InsertString("label_1", "1")
				dp.Attributes().InsertString("label_2", "2")
				dp.SetTimestamp(pcommon.NewTimestampFromTime(ts))
				fillQuantileValue := func(pN, value float64, dest pmetric.ValueAtQuantile) {
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
				metric.SetDataType(pmetric.MetricDataTypeGauge)
				metric.SetDescription("test description")
				dp := metric.Gauge().DataPoints().AppendEmpty()
				dp.SetDoubleVal(v)
				dp.Attributes().InsertString("label_1", "1")
				dp.Attributes().InsertString("label_2", "2")
				dp.SetTimestamp(pcommon.NewTimestampFromTime(ts))
				dp.Flags().SetNoRecordedValue(true)
			},
		},
		{
			name: "StalenessMarkerSum",
			metric: func(ts time.Time, v float64, metrics pmetric.MetricSlice) {
				metric := metrics.AppendEmpty()
				metric.SetName("test_metric")
				metric.SetDataType(pmetric.MetricDataTypeSum)
				metric.SetDescription("test description")
				metric.Sum().SetIsMonotonic(false)
				metric.Sum().SetAggregationTemporality(pmetric.MetricAggregationTemporalityCumulative)
				dp := metric.Sum().DataPoints().AppendEmpty()
				dp.SetDoubleVal(v)
				dp.Attributes().InsertString("label_1", "1")
				dp.Attributes().InsertString("label_2", "2")
				dp.SetTimestamp(pcommon.NewTimestampFromTime(ts))
				dp.Flags().SetNoRecordedValue(true)
			},
		},
		{
			name: "StalenessMarkerHistogram",
			metric: func(ts time.Time, v float64, metrics pmetric.MetricSlice) {
				metric := metrics.AppendEmpty()
				metric.SetName("test_metric")
				metric.SetDataType(pmetric.MetricDataTypeHistogram)
				metric.Histogram().SetAggregationTemporality(pmetric.MetricAggregationTemporalityCumulative)
				metric.SetDescription("test description")
				dp := metric.Histogram().DataPoints().AppendEmpty()
				dp.SetBucketCounts(pcommon.NewImmutableUInt64Slice([]uint64{5, 2}))
				dp.SetCount(7)
				dp.SetExplicitBounds(pcommon.NewImmutableFloat64Slice([]float64{3.5, 10.0}))
				dp.SetSum(v)
				dp.Attributes().InsertString("label_1", "1")
				dp.Attributes().InsertString("label_2", "2")
				dp.SetTimestamp(pcommon.NewTimestampFromTime(ts))
				dp.Flags().SetNoRecordedValue(true)
			},
		},
		{
			name: "StalenessMarkerSummary",
			metric: func(ts time.Time, v float64, metrics pmetric.MetricSlice) {
				metric := metrics.AppendEmpty()
				metric.SetName("test_metric")
				metric.SetDataType(pmetric.MetricDataTypeSummary)
				metric.SetDescription("test description")
				dp := metric.Summary().DataPoints().AppendEmpty()
				dp.SetCount(10)
				dp.SetSum(0.012)
				dp.SetCount(10)
				dp.Attributes().InsertString("label_1", "1")
				dp.Attributes().InsertString("label_2", "2")
				dp.SetTimestamp(pcommon.NewTimestampFromTime(ts))
				dp.Flags().SetNoRecordedValue(true)
				fillQuantileValue := func(pN, value float64, dest pmetric.ValueAtQuantile) {
					dest.SetQuantile(pN)
					dest.SetValue(value)
				}
				fillQuantileValue(0.50, 190, dp.QuantileValues().AppendEmpty())
				fillQuantileValue(0.99, 817, dp.QuantileValues().AppendEmpty())
			},
		},
		{
			name: "DeltaIntSum",
			metric: func(ts time.Time, v float64, metrics pmetric.MetricSlice) {
				metric := metrics.AppendEmpty()
				metric.SetName("test_metric")
				metric.SetDataType(pmetric.MetricDataTypeSum)
				metric.Sum().SetAggregationTemporality(pmetric.MetricAggregationTemporalityDelta)
				metric.Sum().SetIsMonotonic(true)
				dp := metric.Sum().DataPoints().AppendEmpty()
				dp.SetIntVal(int64(v))
				dp.Attributes().InsertString("label_1", "1")
				dp.Attributes().InsertString("label_2", "2")
				dp.SetTimestamp(pcommon.NewTimestampFromTime(ts))
			},
		},
		{
			name: "DeltaSum",
			metric: func(ts time.Time, v float64, metrics pmetric.MetricSlice) {
				metric := metrics.AppendEmpty()
				metric.SetName("test_metric")
				metric.SetDataType(pmetric.MetricDataTypeSum)
				metric.Sum().SetAggregationTemporality(pmetric.MetricAggregationTemporalityDelta)
				metric.Sum().SetIsMonotonic(true)
				dp := metric.Sum().DataPoints().AppendEmpty()
				dp.SetDoubleVal(v)
				dp.Attributes().InsertString("label_1", "1")
				dp.Attributes().InsertString("label_2", "2")
				dp.SetTimestamp(pcommon.NewTimestampFromTime(ts))
			},
		},
		{
			name: "DeltaHistogram",
			metric: func(ts time.Time, v float64, metrics pmetric.MetricSlice) {
				metric := metrics.AppendEmpty()
				metric.SetName("test_metric")
				metric.SetDataType(pmetric.MetricDataTypeHistogram)
				metric.Histogram().SetAggregationTemporality(pmetric.MetricAggregationTemporalityDelta)
				metric.SetDescription("test description")
				dp := metric.Histogram().DataPoints().AppendEmpty()
				dp.SetBucketCounts(pcommon.NewImmutableUInt64Slice([]uint64{5, 2}))
				dp.SetCount(7)
				dp.SetExplicitBounds(pcommon.NewImmutableFloat64Slice([]float64{3.5, 10.0}))
				dp.SetSum(v)
				dp.Attributes().InsertString("label_1", "1")
				dp.Attributes().InsertString("label_2", "2")
				dp.SetTimestamp(pcommon.NewTimestampFromTime(ts))
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

			m2Labels, _, m2Value, m2Temporality, m2IsMonotonic, m2Flags := getMetricProperties(ilm2.Metrics().At(0))
			_, _, m1Value, _, _, _ := getMetricProperties(ilm2.Metrics().At(1))

			signature := timeseriesSignature(ilm2.Scope().Name(), ilm2.Metrics().At(0), m2Labels, pcommon.NewMap())

			resourceMetrics3 := pmetric.NewResourceMetrics()
			ilm3 := resourceMetrics3.ScopeMetrics().AppendEmpty()
			ilm3.Scope().SetName("test")

			tt.metric(ts3, 34, ilm3.Metrics())

			_, _, m3Value, _, _, _ := getMetricProperties(ilm3.Metrics().At(0))

			a := newAccumulator(zap.NewNop(), 1*time.Hour).(*lastValueAccumulator)

			t.Run("OutOfOrder", func(t *testing.T) {
				n := a.Accumulate(resourceMetrics2)

				if m2Flags.NoRecordedValue() {
					require.Zero(t, n)
					return
				}

				m, ok := a.registeredMetrics.Load(signature)
				require.True(t, ok)

				v := m.(*accumulatedValue)
				vLabels, vTS, vValue, vTemporality, vIsMonotonic, _ := getMetricProperties(v.value)

				require.Equal(t, v.scope.Name(), "test")
				require.Equal(t, v.value.DataType(), ilm2.Metrics().At(0).DataType())
				vLabels.Range(func(k string, v pcommon.Value) bool {
					r, _ := m2Labels.Get(k)
					require.Equal(t, r, v)
					return true
				})
				require.Equal(t, m2Labels.Len(), vLabels.Len())
				require.Equal(t, ts2.Unix(), vTS.Unix())
				require.Greater(t, v.updated.Unix(), vTS.Unix())
				require.Equal(t, m2IsMonotonic, vIsMonotonic)

				switch {
				case m2Temporality == pmetric.MetricAggregationTemporalityDelta:
					require.Equal(t, 2, n)
					require.Equal(t, m1Value+m2Value, vValue)
					require.Equal(t, pmetric.MetricAggregationTemporalityCumulative, vTemporality)
				default:
					require.Equal(t, 1, n)
					require.Equal(t, m2Value, vValue)
					require.Equal(t, m2Temporality, vTemporality)
				}
			})

			t.Run("NewDataPoint", func(t *testing.T) {
				n := a.Accumulate(resourceMetrics3)
				if m2Flags.NoRecordedValue() {
					require.Zero(t, n)
					return
				}

				require.Equal(t, 1, n)

				m, ok := a.registeredMetrics.Load(signature)
				require.True(t, ok)
				v := m.(*accumulatedValue)
				_, vTS, vValue, _, _, _ := getMetricProperties(v.value)

				switch {
				case m2Temporality == pmetric.MetricAggregationTemporalityDelta:
					require.Equal(t, m1Value+m2Value+m3Value, vValue)
				default:
					require.Equal(t, m3Value, vValue)
				}
				require.Equal(t, ts3.Unix(), vTS.Unix())
			})
		})
	}
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
				metric.SetDataType(pmetric.MetricDataTypeSum)
				metric.Sum().SetAggregationTemporality(pmetric.MetricAggregationTemporalityDelta)
				metric.Sum().SetIsMonotonic(false)
				dp := metric.Sum().DataPoints().AppendEmpty()
				dp.SetIntVal(42)
				dp.Attributes().InsertString("label_1", "1")
				dp.Attributes().InsertString("label_2", "2")
				dp.SetTimestamp(pcommon.NewTimestampFromTime(ts))
			},
		},
		{
			name: "NonMonotonicSum",
			fillMetric: func(ts time.Time, metric pmetric.Metric) {
				metric.SetName("test_metric")
				metric.SetDataType(pmetric.MetricDataTypeSum)
				metric.Sum().SetAggregationTemporality(pmetric.MetricAggregationTemporalityDelta)
				metric.Sum().SetIsMonotonic(false)
				dp := metric.Sum().DataPoints().AppendEmpty()
				dp.SetDoubleVal(42.42)
				dp.Attributes().InsertString("label_1", "1")
				dp.Attributes().InsertString("label_2", "2")
				dp.SetTimestamp(pcommon.NewTimestampFromTime(ts))
			},
		},
		{
			name: "UnspecifiedIntSum",
			fillMetric: func(ts time.Time, metric pmetric.Metric) {
				metric.SetName("test_metric")
				metric.SetDataType(pmetric.MetricDataTypeSum)
				metric.Sum().SetAggregationTemporality(pmetric.MetricAggregationTemporalityUnspecified)
				dp := metric.Sum().DataPoints().AppendEmpty()
				dp.SetIntVal(42)
				dp.Attributes().InsertString("label_1", "1")
				dp.Attributes().InsertString("label_2", "2")
				dp.SetTimestamp(pcommon.NewTimestampFromTime(ts))
			},
		},
		{
			name: "UnspecifiedSum",
			fillMetric: func(ts time.Time, metric pmetric.Metric) {
				metric.SetName("test_metric")
				metric.SetDataType(pmetric.MetricDataTypeSum)
				metric.Sum().SetAggregationTemporality(pmetric.MetricAggregationTemporalityUnspecified)
				dp := metric.Sum().DataPoints().AppendEmpty()
				dp.SetDoubleVal(42.42)
				dp.Attributes().InsertString("label_1", "1")
				dp.Attributes().InsertString("label_2", "2")
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

func getMetricProperties(metric pmetric.Metric) (
	attributes pcommon.Map,
	ts time.Time,
	value float64,
	temporality pmetric.MetricAggregationTemporality,
	isMonotonic bool,
	flags pmetric.MetricDataPointFlags,
) {
	switch metric.DataType() {
	case pmetric.MetricDataTypeGauge:
		attributes = metric.Gauge().DataPoints().At(0).Attributes()
		ts = metric.Gauge().DataPoints().At(0).Timestamp().AsTime()
		dp := metric.Gauge().DataPoints().At(0)
		switch dp.ValueType() {
		case pmetric.NumberDataPointValueTypeInt:
			value = float64(dp.IntVal())
		case pmetric.NumberDataPointValueTypeDouble:
			value = dp.DoubleVal()
		}
		temporality = pmetric.MetricAggregationTemporalityUnspecified
		isMonotonic = false
		flags = dp.Flags()
	case pmetric.MetricDataTypeSum:
		attributes = metric.Sum().DataPoints().At(0).Attributes()
		ts = metric.Sum().DataPoints().At(0).Timestamp().AsTime()
		dp := metric.Sum().DataPoints().At(0)
		switch dp.ValueType() {
		case pmetric.NumberDataPointValueTypeInt:
			value = float64(dp.IntVal())
		case pmetric.NumberDataPointValueTypeDouble:
			value = dp.DoubleVal()
		}
		temporality = metric.Sum().AggregationTemporality()
		isMonotonic = metric.Sum().IsMonotonic()
		flags = dp.Flags()
	case pmetric.MetricDataTypeHistogram:
		attributes = metric.Histogram().DataPoints().At(0).Attributes()
		ts = metric.Histogram().DataPoints().At(0).Timestamp().AsTime()
		value = metric.Histogram().DataPoints().At(0).Sum()
		temporality = metric.Histogram().AggregationTemporality()
		flags = metric.Histogram().DataPoints().At(0).Flags()
		isMonotonic = true
	case pmetric.MetricDataTypeSummary:
		attributes = metric.Summary().DataPoints().At(0).Attributes()
		ts = metric.Summary().DataPoints().At(0).Timestamp().AsTime()
		value = metric.Summary().DataPoints().At(0).Sum()
		flags = metric.Summary().DataPoints().At(0).Flags()
	default:
		log.Panicf("Invalid data type %s", metric.DataType().String())
	}

	return
}
