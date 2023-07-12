// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewrite

import (
	"testing"
	"time"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/prompb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	prometheustranslator "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus"
)

func TestConvertBucketsLayout(t *testing.T) {
	tests := []struct {
		name       string
		buckets    func() pmetric.ExponentialHistogramDataPointBuckets
		wantSpans  []prompb.BucketSpan
		wantDeltas []int64
	}{
		{
			name: "zero offset",
			buckets: func() pmetric.ExponentialHistogramDataPointBuckets {
				b := pmetric.NewExponentialHistogramDataPointBuckets()
				b.SetOffset(0)
				b.BucketCounts().FromRaw([]uint64{4, 3, 2, 1})
				return b
			},
			wantSpans: []prompb.BucketSpan{
				{
					Offset: 1,
					Length: 4,
				},
			},
			wantDeltas: []int64{4, -1, -1, -1},
		},
		{
			name: "positive offset",
			buckets: func() pmetric.ExponentialHistogramDataPointBuckets {
				b := pmetric.NewExponentialHistogramDataPointBuckets()
				b.SetOffset(4)
				b.BucketCounts().FromRaw([]uint64{4, 2, 0, 2, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1})
				return b
			},
			wantSpans: []prompb.BucketSpan{
				{
					Offset: 5,
					Length: 4,
				},
				{
					Offset: 12,
					Length: 1,
				},
			},
			wantDeltas: []int64{4, -2, -2, 2, -1},
		},
		{
			name: "negative offset",
			buckets: func() pmetric.ExponentialHistogramDataPointBuckets {
				b := pmetric.NewExponentialHistogramDataPointBuckets()
				b.SetOffset(-2)
				b.BucketCounts().FromRaw([]uint64{3, 1, 0, 0, 0, 1})
				return b
			},
			wantSpans: []prompb.BucketSpan{
				{
					Offset: -1,
					Length: 2,
				},
				{
					Offset: 3,
					Length: 1,
				},
			},
			wantDeltas: []int64{3, -2, 0},
		},
		{
			name: "buckets with gaps of size 1",
			buckets: func() pmetric.ExponentialHistogramDataPointBuckets {
				b := pmetric.NewExponentialHistogramDataPointBuckets()
				b.SetOffset(-2)
				b.BucketCounts().FromRaw([]uint64{3, 1, 0, 1, 0, 1})
				return b
			},
			wantSpans: []prompb.BucketSpan{
				{
					Offset: -1,
					Length: 6,
				},
			},
			wantDeltas: []int64{3, -2, -1, 1, -1, 1},
		},
		{
			name: "buckets with gaps of size 2",
			buckets: func() pmetric.ExponentialHistogramDataPointBuckets {
				b := pmetric.NewExponentialHistogramDataPointBuckets()
				b.SetOffset(-2)
				b.BucketCounts().FromRaw([]uint64{3, 0, 0, 1, 0, 0, 1})
				return b
			},
			wantSpans: []prompb.BucketSpan{
				{
					Offset: -1,
					Length: 7,
				},
			},
			wantDeltas: []int64{3, -3, 0, 1, -1, 0, 1},
		},
		{
			name:       "zero buckets",
			buckets:    pmetric.NewExponentialHistogramDataPointBuckets,
			wantSpans:  nil,
			wantDeltas: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotSpans, gotDeltas := convertBucketsLayout(tt.buckets())
			assert.Equal(t, tt.wantSpans, gotSpans)
			assert.Equal(t, tt.wantDeltas, gotDeltas)
		})
	}
}

func TestExponentialToNativeHistogram(t *testing.T) {
	tests := []struct {
		name            string
		exponentialHist func() pmetric.ExponentialHistogramDataPoint
		wantNativeHist  func() prompb.Histogram
		wantErrMessage  string
	}{
		{
			name: "convert exp. to native histogram",
			exponentialHist: func() pmetric.ExponentialHistogramDataPoint {
				pt := pmetric.NewExponentialHistogramDataPoint()
				pt.SetStartTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(100)))
				pt.SetTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(500)))
				pt.SetCount(2)
				pt.SetSum(10.1)
				pt.SetScale(1)
				pt.SetZeroCount(1)

				pt.Positive().BucketCounts().FromRaw([]uint64{1, 1})
				pt.Positive().SetOffset(1)

				pt.Negative().BucketCounts().FromRaw([]uint64{1, 1})
				pt.Negative().SetOffset(1)

				return pt
			},
			wantNativeHist: func() prompb.Histogram {
				return prompb.Histogram{
					Count:          &prompb.Histogram_CountInt{CountInt: 2},
					Sum:            10.1,
					Schema:         1,
					ZeroThreshold:  defaultZeroThreshold,
					ZeroCount:      &prompb.Histogram_ZeroCountInt{ZeroCountInt: 1},
					NegativeSpans:  []prompb.BucketSpan{{Offset: 2, Length: 2}},
					NegativeDeltas: []int64{1, 0},
					PositiveSpans:  []prompb.BucketSpan{{Offset: 2, Length: 2}},
					PositiveDeltas: []int64{1, 0},
					Timestamp:      500,
				}
			},
		},
		{
			name: "convert exp. to native histogram with no sum",
			exponentialHist: func() pmetric.ExponentialHistogramDataPoint {
				pt := pmetric.NewExponentialHistogramDataPoint()
				pt.SetStartTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(100)))
				pt.SetTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(500)))

				pt.SetCount(2)
				pt.SetScale(1)
				pt.SetZeroCount(1)

				pt.Positive().BucketCounts().FromRaw([]uint64{1, 1})
				pt.Positive().SetOffset(1)

				pt.Negative().BucketCounts().FromRaw([]uint64{1, 1})
				pt.Negative().SetOffset(1)

				return pt
			},
			wantNativeHist: func() prompb.Histogram {
				return prompb.Histogram{
					Count:          &prompb.Histogram_CountInt{CountInt: 2},
					Schema:         1,
					ZeroThreshold:  defaultZeroThreshold,
					ZeroCount:      &prompb.Histogram_ZeroCountInt{ZeroCountInt: 1},
					NegativeSpans:  []prompb.BucketSpan{{Offset: 2, Length: 2}},
					NegativeDeltas: []int64{1, 0},
					PositiveSpans:  []prompb.BucketSpan{{Offset: 2, Length: 2}},
					PositiveDeltas: []int64{1, 0},
					Timestamp:      500,
				}
			},
		},
		{
			name: "invalid scale",
			exponentialHist: func() pmetric.ExponentialHistogramDataPoint {
				pt := pmetric.NewExponentialHistogramDataPoint()
				pt.SetScale(-10)
				return pt
			},
			wantErrMessage: "cannot convert exponential to native histogram." +
				" Scale must be <= 8 and >= -4, was -10",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := exponentialToNativeHistogram(tt.exponentialHist())
			if tt.wantErrMessage != "" {
				assert.ErrorContains(t, err, tt.wantErrMessage)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantNativeHist(), got)
		})
	}
}

func TestAddSingleExponentialHistogramDataPoint(t *testing.T) {
	tests := []struct {
		name       string
		metric     func() pmetric.Metric
		wantSeries func() map[string]*prompb.TimeSeries
	}{
		{
			name: "histogram data points with same labels",
			metric: func() pmetric.Metric {
				metric := pmetric.NewMetric()
				metric.SetName("test_hist")
				metric.SetEmptyExponentialHistogram().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)

				pt := metric.ExponentialHistogram().DataPoints().AppendEmpty()
				pt.SetCount(7)
				pt.SetScale(1)
				pt.Positive().SetOffset(-1)
				pt.Positive().BucketCounts().FromRaw([]uint64{4, 2})
				pt.Exemplars().AppendEmpty().SetDoubleValue(1)
				pt.Attributes().PutStr("attr", "test_attr")

				pt = metric.ExponentialHistogram().DataPoints().AppendEmpty()
				pt.SetCount(4)
				pt.SetScale(1)
				pt.Positive().SetOffset(-1)
				pt.Positive().BucketCounts().FromRaw([]uint64{4, 2, 1})
				pt.Exemplars().AppendEmpty().SetDoubleValue(2)
				pt.Attributes().PutStr("attr", "test_attr")

				return metric
			},
			wantSeries: func() map[string]*prompb.TimeSeries {
				labels := []prompb.Label{
					{Name: model.MetricNameLabel, Value: "test_hist"},
					{Name: "attr", Value: "test_attr"},
				}
				return map[string]*prompb.TimeSeries{
					timeSeriesSignature(pmetric.MetricTypeExponentialHistogram.String(), &labels): {
						Labels: labels,
						Histograms: []prompb.Histogram{
							{
								Count:          &prompb.Histogram_CountInt{CountInt: 7},
								Schema:         1,
								ZeroThreshold:  defaultZeroThreshold,
								ZeroCount:      &prompb.Histogram_ZeroCountInt{ZeroCountInt: 0},
								PositiveSpans:  []prompb.BucketSpan{{Offset: 0, Length: 2}},
								PositiveDeltas: []int64{4, -2},
							},
							{
								Count:          &prompb.Histogram_CountInt{CountInt: 4},
								Schema:         1,
								ZeroThreshold:  defaultZeroThreshold,
								ZeroCount:      &prompb.Histogram_ZeroCountInt{ZeroCountInt: 0},
								PositiveSpans:  []prompb.BucketSpan{{Offset: 0, Length: 3}},
								PositiveDeltas: []int64{4, -2, -1},
							},
						},
						Exemplars: []prompb.Exemplar{
							{Value: 1},
							{Value: 2},
						},
					},
				}
			},
		},
		{
			name: "histogram data points with different labels",
			metric: func() pmetric.Metric {
				metric := pmetric.NewMetric()
				metric.SetName("test_hist")
				metric.SetEmptyExponentialHistogram().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)

				pt := metric.ExponentialHistogram().DataPoints().AppendEmpty()
				pt.SetCount(7)
				pt.SetScale(1)
				pt.Positive().SetOffset(-1)
				pt.Positive().BucketCounts().FromRaw([]uint64{4, 2})
				pt.Exemplars().AppendEmpty().SetDoubleValue(1)
				pt.Attributes().PutStr("attr", "test_attr")

				pt = metric.ExponentialHistogram().DataPoints().AppendEmpty()
				pt.SetCount(4)
				pt.SetScale(1)
				pt.Negative().SetOffset(-1)
				pt.Negative().BucketCounts().FromRaw([]uint64{4, 2, 1})
				pt.Exemplars().AppendEmpty().SetDoubleValue(2)
				pt.Attributes().PutStr("attr", "test_attr_two")

				return metric
			},
			wantSeries: func() map[string]*prompb.TimeSeries {
				labels := []prompb.Label{
					{Name: model.MetricNameLabel, Value: "test_hist"},
					{Name: "attr", Value: "test_attr"},
				}
				labelsAnother := []prompb.Label{
					{Name: model.MetricNameLabel, Value: "test_hist"},
					{Name: "attr", Value: "test_attr_two"},
				}

				return map[string]*prompb.TimeSeries{
					timeSeriesSignature(pmetric.MetricTypeExponentialHistogram.String(), &labels): {
						Labels: labels,
						Histograms: []prompb.Histogram{
							{
								Count:          &prompb.Histogram_CountInt{CountInt: 7},
								Schema:         1,
								ZeroThreshold:  defaultZeroThreshold,
								ZeroCount:      &prompb.Histogram_ZeroCountInt{ZeroCountInt: 0},
								PositiveSpans:  []prompb.BucketSpan{{Offset: 0, Length: 2}},
								PositiveDeltas: []int64{4, -2},
							},
						},
						Exemplars: []prompb.Exemplar{
							{Value: 1},
						},
					},
					timeSeriesSignature(pmetric.MetricTypeExponentialHistogram.String(), &labelsAnother): {
						Labels: labelsAnother,
						Histograms: []prompb.Histogram{
							{
								Count:          &prompb.Histogram_CountInt{CountInt: 4},
								Schema:         1,
								ZeroThreshold:  defaultZeroThreshold,
								ZeroCount:      &prompb.Histogram_ZeroCountInt{ZeroCountInt: 0},
								NegativeSpans:  []prompb.BucketSpan{{Offset: 0, Length: 3}},
								NegativeDeltas: []int64{4, -2, -1},
							},
						},
						Exemplars: []prompb.Exemplar{
							{Value: 2},
						},
					},
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metric := tt.metric()

			gotSeries := make(map[string]*prompb.TimeSeries)

			for x := 0; x < metric.ExponentialHistogram().DataPoints().Len(); x++ {
				err := addSingleExponentialHistogramDataPoint(
					prometheustranslator.BuildPromCompliantName(metric, ""),
					metric.ExponentialHistogram().DataPoints().At(x),
					pcommon.NewResource(),
					pcommon.NewInstrumentationScope(),
					Settings{},
					gotSeries,
				)
				require.NoError(t, err)
			}

			assert.Equal(t, tt.wantSeries(), gotSeries)
		})
	}
}
