// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
)

var nonExponentialHist = func() pmetric.Metric {
	m := pmetric.NewMetric()
	m.SetName("not-exponentialhist")
	m.SetEmptyGauge()
	return m
}

func TestUpper_convert_exponential_hist_to_explicit_hist(t *testing.T) {
	ts := pcommon.NewTimestampFromTime(time.Now())
	defaultTestMetric := func() pmetric.Metric {
		exponentialHistInput := pmetric.NewMetric()
		exponentialHistInput.SetName("response_time")
		dp := exponentialHistInput.SetEmptyExponentialHistogram().DataPoints().AppendEmpty()
		exponentialHistInput.ExponentialHistogram().SetAggregationTemporality(1)
		dp.SetCount(2)
		dp.SetScale(7)
		dp.SetSum(361)
		dp.SetMax(195)
		dp.SetMin(166)

		dp.SetTimestamp(ts)

		// set attributes
		dp.Attributes().PutStr("metric_type", "timing")

		// set bucket counts
		dp.Positive().BucketCounts().Append(
			1,
			0,
			0,
			0,
			0,
			0,
			0,
			0,
			0,
			0,
			0,
			0,
			0,
			0,
			0,
			0,
			0,
			0,
			0,
			0,
			0,
			0,
			0,
			0,
			0,
			0,
			0,
			0,
			0,
			1)

		dp.Positive().SetOffset(944)
		return exponentialHistInput
	}

	tests := []struct {
		name         string
		input        func() pmetric.Metric
		arg          []float64 // ExplicitBounds
		distribution string
		want         func(pmetric.Metric)
	}{
		{
			// having explicit bounds that are all smaller than the exponential histogram's scale
			// will results in all the exponential histogram's data points being placed in the overflow bucket
			name:         "convert exponential histogram to explicit histogram with smaller bounds with upper distribute",
			input:        defaultTestMetric,
			arg:          []float64{1.0, 2.0, 3.0, 4.0, 5.0},
			distribution: "upper",
			want: func(metric pmetric.Metric) {
				metric.SetName("response_time")
				dp := metric.SetEmptyHistogram().DataPoints().AppendEmpty()
				metric.Histogram().SetAggregationTemporality(1)
				dp.SetCount(2)
				dp.SetSum(361)
				dp.SetMax(195)
				dp.SetMin(166)
				dp.SetTimestamp(ts)

				// set attributes
				dp.Attributes().PutStr("metric_type", "timing")

				// set bucket counts
				dp.BucketCounts().Append(0, 0, 0, 0, 2) // expect all counts in the overflow bucket

				// set explictbounds
				dp.ExplicitBounds().Append(1.0, 2.0, 3.0, 4.0, 5.0)
			},
		},
		{
			// having explicit bounds that are all larger than the exponential histogram's scale
			// will results in all the exponential histogram's data points being placed in the 1st bucket
			name:         "convert exponential histogram to explicit histogram with large bounds",
			input:        defaultTestMetric,
			arg:          []float64{1000.0, 2000.0, 3000.0, 4000.0, 5000.0},
			distribution: "upper",
			want: func(metric pmetric.Metric) {
				metric.SetName("response_time")
				dp := metric.SetEmptyHistogram().DataPoints().AppendEmpty()
				metric.Histogram().SetAggregationTemporality(1)
				dp.SetCount(2)
				dp.SetSum(361)
				dp.SetMax(195)
				dp.SetMin(166)
				dp.SetTimestamp(ts)

				// set attributes
				dp.Attributes().PutStr("metric_type", "timing")

				// set bucket counts
				dp.BucketCounts().Append(2, 0, 0, 0, 0) // expect all counts in the 1st bucket

				// set explictbounds
				dp.ExplicitBounds().Append(1000.0, 2000.0, 3000.0, 4000.0, 5000.0)
			},
		},
		{
			name:         "convert exponential histogram to explicit history",
			input:        defaultTestMetric,
			arg:          []float64{160.0, 170.0, 180.0, 190.0, 200.0},
			distribution: "upper",
			want: func(metric pmetric.Metric) {
				metric.SetName("response_time")
				dp := metric.SetEmptyHistogram().DataPoints().AppendEmpty()
				metric.Histogram().SetAggregationTemporality(1)
				dp.SetCount(2)
				dp.SetSum(361)
				dp.SetMax(195)
				dp.SetMin(166)
				dp.SetTimestamp(ts)

				// set attributes
				dp.Attributes().PutStr("metric_type", "timing")

				// set bucket counts
				dp.BucketCounts().Append(1, 0, 0, 1, 0)

				// set explictbounds
				dp.ExplicitBounds().Append(160.0, 170.0, 180.0, 190.0, 200.0)
			},
		},
		{
			name:         "convert exponential histogram to explicit history with 0 scale",
			input:        defaultTestMetric,
			arg:          []float64{160.0, 170.0, 180.0, 190.0, 200.0},
			distribution: "upper",
			want: func(metric pmetric.Metric) {
				metric.SetName("response_time")
				dp := metric.SetEmptyHistogram().DataPoints().AppendEmpty()
				metric.Histogram().SetAggregationTemporality(1)
				dp.SetCount(2)
				dp.SetSum(361)
				dp.SetMax(195)
				dp.SetMin(166)
				dp.SetTimestamp(ts)

				// set attributes
				dp.Attributes().PutStr("metric_type", "timing")

				// set bucket counts
				dp.BucketCounts().Append(1, 0, 0, 1, 0)

				// set explictbounds
				dp.ExplicitBounds().Append(160.0, 170.0, 180.0, 190.0, 200.0)
			},
		},
		{
			// 0 scale exponential histogram will result in an extremely large upper bound
			// resulting in all the counts being in buckets much larger than the explicit bounds
			// thus all counts will be in the overflow bucket
			name: "0 scale exponential histogram given using upper distribute",
			input: func() pmetric.Metric {
				m := pmetric.NewMetric()
				defaultTestMetric().CopyTo(m)
				m.ExponentialHistogram().DataPoints().At(0).SetScale(0)
				return m
			},
			arg:          []float64{160.0, 170.0, 180.0, 190.0, 200.0},
			distribution: "upper",
			want: func(metric pmetric.Metric) {
				metric.SetName("response_time")
				dp := metric.SetEmptyHistogram().DataPoints().AppendEmpty()
				metric.Histogram().SetAggregationTemporality(1)
				dp.SetCount(2)
				dp.SetSum(361)
				dp.SetMax(195)
				dp.SetMin(166)
				dp.SetTimestamp(ts)

				// set attributes
				dp.Attributes().PutStr("metric_type", "timing")

				// set bucket counts
				dp.BucketCounts().Append(0, 0, 0, 0, 2)

				// set explictbounds
				dp.ExplicitBounds().Append(160.0, 170.0, 180.0, 190.0, 200.0)
			},
		},
		{
			name: "empty exponential histogram given using upper distribute",
			input: func() pmetric.Metric {
				m := pmetric.NewMetric()
				m.SetName("empty")
				m.SetEmptyExponentialHistogram()
				return m
			},
			arg:          []float64{160.0, 170.0, 180.0, 190.0, 200.0},
			distribution: "upper",
			want: func(metric pmetric.Metric) {
				metric.SetName("empty")
				metric.SetEmptyHistogram()
			},
		},
		{
			name:         "non-exponential histogram",
			arg:          []float64{0},
			distribution: "upper",
			input:        nonExponentialHist,
			want: func(metric pmetric.Metric) {
				nonExponentialHist().CopyTo(metric)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metric := pmetric.NewMetric()
			tt.input().CopyTo(metric)

			ctx := ottlmetric.NewTransformContext(metric, pmetric.NewMetricSlice(), pcommon.NewInstrumentationScope(), pcommon.NewResource(), pmetric.NewScopeMetrics(), pmetric.NewResourceMetrics())

			exprFunc, err := convertExponentialHistToExplicitHist(tt.distribution, tt.arg)
			assert.NoError(t, err)
			_, err = exprFunc(nil, ctx)
			assert.NoError(t, err)

			expected := pmetric.NewMetric()
			tt.want(expected)

			assert.Equal(t, expected, metric)
		})
	}
}

func TestMidpoint_convert_exponential_hist_to_explicit_hist(t *testing.T) {
	ts := pcommon.NewTimestampFromTime(time.Now())
	defaultTestMetric := func() pmetric.Metric {
		m := pmetric.NewMetric()
		m.SetName("test-metric")
		dp := m.SetEmptyExponentialHistogram().DataPoints().AppendEmpty()
		m.ExponentialHistogram().SetAggregationTemporality(1)
		dp.SetCount(44)
		dp.SetScale(0)
		dp.SetSum(999)
		dp.SetMax(245)
		dp.SetMin(40)

		dp.SetTimestamp(ts)

		dp.Attributes().PutStr("metric_type", "timing")
		dp.Positive().SetOffset(5)
		dp.Positive().BucketCounts().FromRaw([]uint64{10, 22, 12})
		return m
	}

	tests := []struct {
		name         string
		input        func() pmetric.Metric
		arg          []float64 // ExplicitBounds
		distribution string
		want         func(pmetric.Metric)
	}{
		{
			// having explicit bounds that are all smaller than the exponential histogram's scale
			// will results in all the exponential histogram's data points being placed in the overflow bucket
			name:         "convert exponential histogram to explicit histogram with smaller bounds",
			input:        defaultTestMetric,
			arg:          []float64{1.0, 2.0, 3.0, 4.0, 5.0},
			distribution: "midpoint",
			want: func(metric pmetric.Metric) {
				metric.SetName("test-metric")
				dp := metric.SetEmptyHistogram().DataPoints().AppendEmpty()
				metric.Histogram().SetAggregationTemporality(1)
				dp.SetCount(44)
				dp.SetSum(999)
				dp.SetMax(245)
				dp.SetMin(40)
				dp.SetTimestamp(ts)

				// set attributes
				dp.Attributes().PutStr("metric_type", "timing")

				// set bucket counts
				dp.BucketCounts().Append(0, 0, 0, 0, 44) // expect all counts in the overflow bucket

				// set explictbounds
				dp.ExplicitBounds().Append(1.0, 2.0, 3.0, 4.0, 5.0)
			},
		},
		{
			// having explicit bounds that are all larger than the exponential histogram's scale
			// will results in all the exponential histogram's data points being placed in the 1st bucket
			name:         "convert exponential histogram to explicit histogram with large bounds",
			input:        defaultTestMetric,
			arg:          []float64{1000.0, 2000.0, 3000.0, 4000.0, 5000.0},
			distribution: "midpoint",
			want: func(metric pmetric.Metric) {
				metric.SetName("test-metric")
				dp := metric.SetEmptyHistogram().DataPoints().AppendEmpty()
				metric.Histogram().SetAggregationTemporality(1)
				dp.SetCount(44)
				dp.SetSum(999)
				dp.SetMax(245)
				dp.SetMin(40)
				dp.SetTimestamp(ts)

				// set attributes
				dp.Attributes().PutStr("metric_type", "timing")

				// set bucket counts
				dp.BucketCounts().Append(44, 0, 0, 0, 0) // expect all counts in the 1st bucket

				// set explictbounds
				dp.ExplicitBounds().Append(1000.0, 2000.0, 3000.0, 4000.0, 5000.0)
			},
		},
		{
			name:         "convert exponential histogram to explicit hist",
			input:        defaultTestMetric,
			arg:          []float64{10.0, 20.0, 30.0, 40.0, 50.0, 60.0, 70.0, 80.0, 90.0, 100.0},
			distribution: "midpoint",
			want: func(metric pmetric.Metric) {
				metric.SetName("test-metric")
				dp := metric.SetEmptyHistogram().DataPoints().AppendEmpty()
				metric.Histogram().SetAggregationTemporality(1)
				dp.SetCount(44)
				dp.SetSum(999)
				dp.SetMax(245)
				dp.SetMin(40)
				dp.SetTimestamp(ts)

				// set attributes
				dp.Attributes().PutStr("metric_type", "timing")

				// set bucket counts
				dp.BucketCounts().Append(0, 0, 0, 10, 0, 0, 0, 0, 22, 12)

				// set explictbounds
				dp.ExplicitBounds().Append(10.0, 20.0, 30.0, 40.0, 50.0, 60.0, 70.0, 80.0, 90.0, 100.0)
			},
		},
		{
			name: "convert exponential histogram to explicit hist with zero count",
			input: func() pmetric.Metric {
				m := defaultTestMetric()
				m.ExponentialHistogram().DataPoints().At(0).SetZeroCount(5)
				return m
			},
			arg:          []float64{0, 10.0, 20.0, 30.0, 40.0, 50.0, 60.0, 70.0, 80.0, 90.0, 100.0},
			distribution: "midpoint",
			want: func(metric pmetric.Metric) {
				metric.SetName("test-metric")
				dp := metric.SetEmptyHistogram().DataPoints().AppendEmpty()
				metric.Histogram().SetAggregationTemporality(1)
				dp.SetCount(44)
				dp.SetSum(999)
				dp.SetMax(245)
				dp.SetMin(40)
				dp.SetTimestamp(ts)

				// set attributes
				dp.Attributes().PutStr("metric_type", "timing")

				// set bucket counts
				dp.BucketCounts().Append(5, 0, 0, 0, 10, 0, 0, 0, 0, 22, 12)

				// set explictbounds
				dp.ExplicitBounds().Append(0, 10.0, 20.0, 30.0, 40.0, 50.0, 60.0, 70.0, 80.0, 90.0, 100.0)
			},
		},
		{
			name: "empty exponential histogram given",
			input: func() pmetric.Metric {
				m := pmetric.NewMetric()
				m.SetName("empty")
				m.SetEmptyExponentialHistogram()
				return m
			},
			arg:          []float64{160.0, 170.0, 180.0, 190.0, 200.0},
			distribution: "midpoint",
			want: func(metric pmetric.Metric) {
				metric.SetName("empty")
				metric.SetEmptyHistogram()
			},
		},
		{
			name:         "non-exponential histogram given using upper distribute",
			arg:          []float64{0},
			distribution: "midpoint",
			input:        nonExponentialHist,
			want: func(metric pmetric.Metric) {
				nonExponentialHist().CopyTo(metric)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metric := pmetric.NewMetric()
			tt.input().CopyTo(metric)

			ctx := ottlmetric.NewTransformContext(metric, pmetric.NewMetricSlice(), pcommon.NewInstrumentationScope(), pcommon.NewResource(), pmetric.NewScopeMetrics(), pmetric.NewResourceMetrics())

			exprFunc, err := convertExponentialHistToExplicitHist(tt.distribution, tt.arg)
			assert.NoError(t, err)
			_, err = exprFunc(nil, ctx)
			assert.NoError(t, err)

			expected := pmetric.NewMetric()
			tt.want(expected)

			assert.Equal(t, expected, metric)
		})
	}
}

func TestUniform_convert_exponential_hist_to_explicit_hist(t *testing.T) {
	ts := pcommon.NewTimestampFromTime(time.Now())
	defaultTestMetric := func() pmetric.Metric {
		m := pmetric.NewMetric()
		m.SetName("test-metric")
		dp := m.SetEmptyExponentialHistogram().DataPoints().AppendEmpty()
		m.ExponentialHistogram().SetAggregationTemporality(1)
		dp.SetCount(44)
		dp.SetScale(0)
		dp.SetSum(999)
		dp.SetMax(245)
		dp.SetMin(40)

		dp.SetTimestamp(ts)

		dp.Attributes().PutStr("metric_type", "timing")
		dp.Positive().SetOffset(5)
		dp.Positive().BucketCounts().FromRaw([]uint64{10, 22, 12})
		return m
	}

	tests := []struct {
		name         string
		input        func() pmetric.Metric
		arg          []float64 // ExplicitBounds
		distribution string
		want         func(pmetric.Metric)
	}{
		{
			// having explicit bounds that are all smaller than the exponential histogram's scale
			// will results in all the exponential histogram's data points being placed in the overflow bucket
			name:         "convert exponential histogram to explicit histogram with smaller bounds",
			input:        defaultTestMetric,
			arg:          []float64{1.0, 2.0, 3.0, 4.0, 5.0},
			distribution: "uniform",
			want: func(metric pmetric.Metric) {
				metric.SetName("test-metric")
				dp := metric.SetEmptyHistogram().DataPoints().AppendEmpty()
				metric.Histogram().SetAggregationTemporality(1)
				dp.SetCount(44)
				dp.SetSum(999)
				dp.SetMax(245)
				dp.SetMin(40)
				dp.SetTimestamp(ts)

				// set attributes
				dp.Attributes().PutStr("metric_type", "timing")

				// set bucket counts
				dp.BucketCounts().Append(0, 0, 0, 0, 44) // expect all counts in the overflow bucket

				// set explictbounds
				dp.ExplicitBounds().Append(1.0, 2.0, 3.0, 4.0, 5.0)
			},
		},
		{
			// having explicit bounds that are all larger than the exponential histogram's scale
			// will results in all the exponential histogram's data points being placed in the 1st bucket
			name:         "convert exponential histogram to explicit histogram with large bounds",
			input:        defaultTestMetric,
			arg:          []float64{1000.0, 2000.0, 3000.0, 4000.0, 5000.0},
			distribution: "uniform",
			want: func(metric pmetric.Metric) {
				metric.SetName("test-metric")
				dp := metric.SetEmptyHistogram().DataPoints().AppendEmpty()
				metric.Histogram().SetAggregationTemporality(1)
				dp.SetCount(44)
				dp.SetSum(999)
				dp.SetMax(245)
				dp.SetMin(40)
				dp.SetTimestamp(ts)

				// set attributes
				dp.Attributes().PutStr("metric_type", "timing")

				// set bucket counts
				dp.BucketCounts().Append(44, 0, 0, 0, 0) // expect all counts in the 1st bucket

				// set explictbounds
				dp.ExplicitBounds().Append(1000.0, 2000.0, 3000.0, 4000.0, 5000.0)
			},
		},
		{
			name:         "convert exponential histogram to explicit hist",
			input:        defaultTestMetric,
			arg:          []float64{10.0, 20.0, 30.0, 40.0, 50.0, 60.0, 70.0, 80.0, 90.0, 100.0},
			distribution: "uniform",
			want: func(metric pmetric.Metric) {
				metric.SetName("test-metric")
				dp := metric.SetEmptyHistogram().DataPoints().AppendEmpty()
				metric.Histogram().SetAggregationTemporality(1)
				dp.SetCount(44)
				dp.SetSum(999)
				dp.SetMax(245)
				dp.SetMin(40)
				dp.SetTimestamp(ts)

				// set attributes
				dp.Attributes().PutStr("metric_type", "timing")

				// set bucket counts
				dp.BucketCounts().Append(0, 0, 0, 3, 3, 2, 8, 6, 5, 17)

				// set explictbounds
				dp.ExplicitBounds().Append(10.0, 20.0, 30.0, 40.0, 50.0, 60.0, 70.0, 80.0, 90.0, 100.0)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metric := pmetric.NewMetric()
			tt.input().CopyTo(metric)

			ctx := ottlmetric.NewTransformContext(metric, pmetric.NewMetricSlice(), pcommon.NewInstrumentationScope(), pcommon.NewResource(), pmetric.NewScopeMetrics(), pmetric.NewResourceMetrics())

			exprFunc, err := convertExponentialHistToExplicitHist(tt.distribution, tt.arg)
			assert.NoError(t, err)
			_, err = exprFunc(nil, ctx)
			assert.NoError(t, err)

			expected := pmetric.NewMetric()
			tt.want(expected)

			assert.Equal(t, expected, metric)
		})
	}
}

func TestRandom_convert_exponential_hist_to_explicit_hist(t *testing.T) {
	ts := pcommon.NewTimestampFromTime(time.Now())
	defaultTestMetric := func() pmetric.Metric {
		m := pmetric.NewMetric()
		m.SetName("test-metric")
		dp := m.SetEmptyExponentialHistogram().DataPoints().AppendEmpty()
		m.ExponentialHistogram().SetAggregationTemporality(1)
		dp.SetCount(44)
		dp.SetScale(0)
		dp.SetSum(999)
		dp.SetMax(245)
		dp.SetMin(40)

		dp.SetTimestamp(ts)

		dp.Attributes().PutStr("metric_type", "timing")
		dp.Positive().SetOffset(5)
		dp.Positive().BucketCounts().FromRaw([]uint64{10, 22, 12})
		return m
	}

	tests := []struct {
		name         string
		input        func() pmetric.Metric
		arg          []float64 // ExplicitBounds
		distribution string
		want         func(pmetric.Metric)
	}{
		{
			// having explicit bounds that are all smaller than the exponential histogram's scale
			// will results in all the exponential histogram's data points being placed in the overflow bucket
			name:         "convert exponential histogram to explicit histogram with smaller bounds",
			input:        defaultTestMetric,
			arg:          []float64{1.0, 2.0, 3.0, 4.0, 5.0},
			distribution: "random",
			want: func(metric pmetric.Metric) {
				metric.SetName("test-metric")
				dp := metric.SetEmptyHistogram().DataPoints().AppendEmpty()
				metric.Histogram().SetAggregationTemporality(1)
				dp.SetCount(44)
				dp.SetSum(999)
				dp.SetMax(245)
				dp.SetMin(40)
				dp.SetTimestamp(ts)

				// set attributes
				dp.Attributes().PutStr("metric_type", "timing")

				// set bucket counts
				dp.BucketCounts().Append(0, 0, 0, 0, 44) // expect all counts in the overflow bucket

				// set explictbounds
				dp.ExplicitBounds().Append(1.0, 2.0, 3.0, 4.0, 5.0)
			},
		},
		{
			// having explicit bounds that are all larger than the exponential histogram's scale
			// will results in all the exponential histogram's data points being placed in the 1st bucket
			name:         "convert exponential histogram to explicit histogram with large bounds",
			input:        defaultTestMetric,
			arg:          []float64{1000.0, 2000.0, 3000.0, 4000.0, 5000.0},
			distribution: "random",
			want: func(metric pmetric.Metric) {
				metric.SetName("test-metric")
				dp := metric.SetEmptyHistogram().DataPoints().AppendEmpty()
				metric.Histogram().SetAggregationTemporality(1)
				dp.SetCount(44)
				dp.SetSum(999)
				dp.SetMax(245)
				dp.SetMin(40)
				dp.SetTimestamp(ts)

				// set attributes
				dp.Attributes().PutStr("metric_type", "timing")

				// set bucket counts
				dp.BucketCounts().Append(44, 0, 0, 0, 0) // expect all counts in the 1st bucket

				// set explictbounds
				dp.ExplicitBounds().Append(1000.0, 2000.0, 3000.0, 4000.0, 5000.0)
			},
		},
		{
			name:         "convert exponential histogram to explicit hist",
			input:        defaultTestMetric,
			arg:          []float64{10.0, 20.0, 30.0, 40.0, 50.0, 60.0, 70.0, 80.0, 90.0, 100.0},
			distribution: "random",
			want: func(metric pmetric.Metric) {
				metric.SetName("test-metric")
				dp := metric.SetEmptyHistogram().DataPoints().AppendEmpty()
				metric.Histogram().SetAggregationTemporality(1)
				dp.SetCount(44)
				dp.SetSum(999)
				dp.SetMax(245)
				dp.SetMin(40)
				dp.SetTimestamp(ts)

				// set attributes
				dp.Attributes().PutStr("metric_type", "timing")

				// set bucket counts
				dp.BucketCounts().Append(0, 0, 3, 3, 2, 7, 5, 4, 4, 16)

				// set explictbounds
				dp.ExplicitBounds().Append(10.0, 20.0, 30.0, 40.0, 50.0, 60.0, 70.0, 80.0, 90.0, 100.0)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metric := pmetric.NewMetric()
			tt.input().CopyTo(metric)

			ctx := ottlmetric.NewTransformContext(metric, pmetric.NewMetricSlice(), pcommon.NewInstrumentationScope(), pcommon.NewResource(), pmetric.NewScopeMetrics(), pmetric.NewResourceMetrics())

			exprFunc, err := convertExponentialHistToExplicitHist(tt.distribution, tt.arg)
			assert.NoError(t, err)
			_, err = exprFunc(nil, ctx)
			assert.NoError(t, err)

			expected := pmetric.NewMetric()
			tt.want(expected)

			// since the bucket counts are randomly distributed, we can't predict the exact output
			// thus we only check if the metric dimensions are as expected.
			if tt.name == "convert exponential histogram to explicit hist" {
				expectedDp := expected.Histogram().DataPoints().At(0)
				dp := metric.Histogram().DataPoints().At(0)
				assert.Equal(t,
					expectedDp.BucketCounts().Len(),
					dp.BucketCounts().Len())

				var count uint64
				for i := 0; i < dp.BucketCounts().Len(); i++ {
					count += dp.BucketCounts().At(i)
				}

				assert.Equal(t, expectedDp.Count(), count)
				assert.Equal(t, expectedDp.ExplicitBounds().Len(), dp.ExplicitBounds().Len())

				// even though the distribution is random, we know that for this
				// particular test case, the min value is 40, therefore the 1st 3 bucket
				// counts should be 0, as they represent values 10 - 30
				for i := 0; i < 3; i++ {
					assert.Equal(t, uint64(0), dp.BucketCounts().At(i), "bucket %d", i)
				}

				// since the max value in the exponential histogram is 245
				// we can assert that the overflow bucket has a count > 0
				overflow := dp.BucketCounts().At(dp.BucketCounts().Len() - 1)
				assert.Positive(t, overflow, "overflow bucket count should be > 0")
				return
			}

			assert.Equal(t, expected, metric)
		})
	}
}

func Test_convertExponentialHistToExplicitHist_validate(t *testing.T) {
	tests := []struct {
		name                    string
		sliceExplicitBoundsArgs []float64
	}{
		{
			name:                    "empty explicit bounds",
			sliceExplicitBoundsArgs: []float64{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := convertExponentialHistToExplicitHist("random", tt.sliceExplicitBoundsArgs)
			assert.ErrorContains(t, err, "explicit bounds cannot be empty")
		})
	}
}
