// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewrite

import (
	"fmt"
	"testing"
	"time"

	"github.com/prometheus/common/model"
	"github.com/prometheus/otlptranslator"
	"github.com/prometheus/prometheus/prompb"
	writev2 "github.com/prometheus/prometheus/prompb/io/prometheus/write/v2"
	prom "github.com/prometheus/prometheus/storage/remote/otlptranslator/prometheusremotewrite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

type expectedBucketLayoutV2 struct {
	wantSpans  []writev2.BucketSpan
	wantDeltas []int64
}

func TestConvertBucketsLayoutV2(t *testing.T) {
	tests := []struct {
		name       string
		buckets    func() pmetric.ExponentialHistogramDataPointBuckets
		wantLayout map[int32]expectedBucketLayoutV2
	}{
		{
			name: "zero offset",
			buckets: func() pmetric.ExponentialHistogramDataPointBuckets {
				b := pmetric.NewExponentialHistogramDataPointBuckets()
				b.SetOffset(0)
				b.BucketCounts().FromRaw([]uint64{4, 3, 2, 1})
				return b
			},
			wantLayout: map[int32]expectedBucketLayoutV2{
				0: {
					wantSpans: []writev2.BucketSpan{
						{
							Offset: 1,
							Length: 4,
						},
					},
					wantDeltas: []int64{4, -1, -1, -1},
				},
				1: {
					wantSpans: []writev2.BucketSpan{
						{
							Offset: 1,
							Length: 2,
						},
					},
					// 4+3, 2+1 = 7, 3 =delta= 7, -4
					wantDeltas: []int64{7, -4},
				},
				2: {
					wantSpans: []writev2.BucketSpan{
						{
							Offset: 1,
							Length: 1,
						},
					},
					// 4+3+2+1 = 10 =delta= 10
					wantDeltas: []int64{10},
				},
			},
		},
		{
			name: "offset 1",
			buckets: func() pmetric.ExponentialHistogramDataPointBuckets {
				b := pmetric.NewExponentialHistogramDataPointBuckets()
				b.SetOffset(1)
				b.BucketCounts().FromRaw([]uint64{4, 3, 2, 1})
				return b
			},
			wantLayout: map[int32]expectedBucketLayoutV2{
				0: {
					wantSpans: []writev2.BucketSpan{
						{
							Offset: 2,
							Length: 4,
						},
					},
					wantDeltas: []int64{4, -1, -1, -1},
				},
				1: {
					wantSpans: []writev2.BucketSpan{
						{
							Offset: 1,
							Length: 3,
						},
					},
					wantDeltas: []int64{4, 1, -4}, // 0+4, 3+2, 1+0 = 4, 5, 1
				},
				2: {
					wantSpans: []writev2.BucketSpan{
						{
							Offset: 1,
							Length: 2,
						},
					},
					wantDeltas: []int64{9, -8}, // 0+4+3+2, 1+0+0+0 = 9, 1
				},
			},
		},
		{
			name: "positive offset",
			buckets: func() pmetric.ExponentialHistogramDataPointBuckets {
				b := pmetric.NewExponentialHistogramDataPointBuckets()
				b.SetOffset(4)
				b.BucketCounts().FromRaw([]uint64{4, 2, 0, 2, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1})
				return b
			},
			wantLayout: map[int32]expectedBucketLayoutV2{
				0: {
					wantSpans: []writev2.BucketSpan{
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
				1: {
					wantSpans: []writev2.BucketSpan{
						{
							Offset: 3,
							Length: 2,
						},
						{
							Offset: 6,
							Length: 1,
						},
					},
					// Downscale:
					// 4+2, 0+2, 0+0, 0+0, 0+0, 0+0, 0+0, 0+0, 1+0 = 6, 2, 0, 0, 0, 0, 0, 0, 1
					wantDeltas: []int64{6, -4, -1},
				},
				2: {
					wantSpans: []writev2.BucketSpan{
						{
							Offset: 2,
							Length: 1,
						},
						{
							Offset: 3,
							Length: 1,
						},
					},
					// Downscale:
					// 4+2+0+2, 0+0+0+0, 0+0+0+0, 0+0+0+0, 1+0+0+0 = 8, 0, 0, 0, 1
					// Check from sclaing from previous: 6+2, 0+0, 0+0, 0+0, 1+0 = 8, 0, 0, 0, 1
					wantDeltas: []int64{8, -7},
				},
			},
		},
		{
			name: "scaledown merges spans",
			buckets: func() pmetric.ExponentialHistogramDataPointBuckets {
				b := pmetric.NewExponentialHistogramDataPointBuckets()
				b.SetOffset(4)
				b.BucketCounts().FromRaw([]uint64{4, 2, 0, 2, 0, 0, 0, 0, 0, 0, 0, 0, 1})
				return b
			},
			wantLayout: map[int32]expectedBucketLayoutV2{
				0: {
					wantSpans: []writev2.BucketSpan{
						{
							Offset: 5,
							Length: 4,
						},
						{
							Offset: 8,
							Length: 1,
						},
					},
					wantDeltas: []int64{4, -2, -2, 2, -1},
				},
				1: {
					wantSpans: []writev2.BucketSpan{
						{
							Offset: 3,
							Length: 2,
						},
						{
							Offset: 4,
							Length: 1,
						},
					},
					// Downscale:
					// 4+2, 0+2, 0+0, 0+0, 0+0, 0+0, 1+0 = 6, 2, 0, 0, 0, 0, 1
					wantDeltas: []int64{6, -4, -1},
				},
				2: {
					wantSpans: []writev2.BucketSpan{
						{
							Offset: 2,
							Length: 4,
						},
					},
					// Downscale:
					// 4+2+0+2, 0+0+0+0, 0+0+0+0, 1+0+0+0 = 8, 0, 0, 1
					// Check from sclaing from previous: 6+2, 0+0, 0+0, 1+0 = 8, 0, 0, 1
					wantDeltas: []int64{8, -8, 0, 1},
				},
			},
		},
		{
			name: "negative offset",
			buckets: func() pmetric.ExponentialHistogramDataPointBuckets {
				b := pmetric.NewExponentialHistogramDataPointBuckets()
				b.SetOffset(-2)
				b.BucketCounts().FromRaw([]uint64{3, 1, 0, 0, 0, 1})
				return b
			},
			wantLayout: map[int32]expectedBucketLayoutV2{
				0: {
					wantSpans: []writev2.BucketSpan{
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
				1: {
					wantSpans: []writev2.BucketSpan{
						{
							Offset: 0,
							Length: 3,
						},
					},
					// Downscale:
					// 3+1, 0+0, 0+1 = 4, 0, 1
					wantDeltas: []int64{4, -4, 1},
				},
				2: {
					wantSpans: []writev2.BucketSpan{
						{
							Offset: 0,
							Length: 2,
						},
					},
					// Downscale:
					// 0+0+3+1, 0+0+0+0 = 4, 1
					wantDeltas: []int64{4, -3},
				},
			},
		},
		{
			name: "buckets with gaps of size 1",
			buckets: func() pmetric.ExponentialHistogramDataPointBuckets {
				b := pmetric.NewExponentialHistogramDataPointBuckets()
				b.SetOffset(-2)
				b.BucketCounts().FromRaw([]uint64{3, 1, 0, 1, 0, 1})
				return b
			},
			wantLayout: map[int32]expectedBucketLayoutV2{
				0: {
					wantSpans: []writev2.BucketSpan{
						{
							Offset: -1,
							Length: 6,
						},
					},
					wantDeltas: []int64{3, -2, -1, 1, -1, 1},
				},
				1: {
					wantSpans: []writev2.BucketSpan{
						{
							Offset: 0,
							Length: 3,
						},
					},
					// Downscale:
					// 3+1, 0+1, 0+1 = 4, 1, 1
					wantDeltas: []int64{4, -3, 0},
				},
				2: {
					wantSpans: []writev2.BucketSpan{
						{
							Offset: 0,
							Length: 2,
						},
					},
					// Downscale:
					// 0+0+3+1, 0+1+0+1 = 4, 2
					wantDeltas: []int64{4, -2},
				},
			},
		},
		{
			name: "buckets with gaps of size 2",
			buckets: func() pmetric.ExponentialHistogramDataPointBuckets {
				b := pmetric.NewExponentialHistogramDataPointBuckets()
				b.SetOffset(-2)
				b.BucketCounts().FromRaw([]uint64{3, 0, 0, 1, 0, 0, 1})
				return b
			},
			wantLayout: map[int32]expectedBucketLayoutV2{
				0: {
					wantSpans: []writev2.BucketSpan{
						{
							Offset: -1,
							Length: 7,
						},
					},
					wantDeltas: []int64{3, -3, 0, 1, -1, 0, 1},
				},
				1: {
					wantSpans: []writev2.BucketSpan{
						{
							Offset: 0,
							Length: 4,
						},
					},
					// Downscale:
					// 3+0, 0+1, 0+0, 0+1 = 3, 1, 0, 1
					wantDeltas: []int64{3, -2, -1, 1},
				},
				2: {
					wantSpans: []writev2.BucketSpan{
						{
							Offset: 0,
							Length: 3,
						},
					},
					// Downscale:
					// 0+0+3+0, 0+1+0+0, 1+0+0+0 = 3, 1, 1
					wantDeltas: []int64{3, -2, 0},
				},
			},
		},
		{
			name:    "zero buckets",
			buckets: pmetric.NewExponentialHistogramDataPointBuckets,
			wantLayout: map[int32]expectedBucketLayoutV2{
				0: {
					wantSpans:  nil,
					wantDeltas: nil,
				},
				1: {
					wantSpans:  nil,
					wantDeltas: nil,
				},
				2: {
					wantSpans:  nil,
					wantDeltas: nil,
				},
			},
		},
	}
	for _, tt := range tests {
		for scaleDown, wantLayout := range tt.wantLayout {
			t.Run(fmt.Sprintf("%s-scaleby-%d", tt.name, scaleDown), func(t *testing.T) {
				gotSpans, gotDeltas := convertBucketsLayoutV2(tt.buckets(), scaleDown)
				assert.Equal(t, wantLayout.wantSpans, gotSpans)
				assert.Equal(t, wantLayout.wantDeltas, gotDeltas)
			})
		}
	}
}

func BenchmarkConvertBucketLayoutV2(b *testing.B) {
	scenarios := []struct {
		gap int
	}{
		{gap: 0},
		{gap: 1},
		{gap: 2},
		{gap: 3},
	}

	for _, scenario := range scenarios {
		buckets := pmetric.NewExponentialHistogramDataPointBuckets()
		buckets.SetOffset(0)
		for i := range 1000 {
			if i%(scenario.gap+1) == 0 {
				buckets.BucketCounts().Append(10)
			} else {
				buckets.BucketCounts().Append(0)
			}
		}
		b.Run(fmt.Sprintf("gap %d", scenario.gap), func(b *testing.B) {
			for b.Loop() {
				convertBucketsLayout(buckets, 0)
			}
		})
	}
}

func TestExponentialToNativeHistogramV2(t *testing.T) {
	tests := []struct {
		name            string
		exponentialHist func() pmetric.ExponentialHistogramDataPoint
		wantNativeHist  func() writev2.Histogram
		wantErrMessage  string
	}{
		{
			name: "convert exp. to native histogram",
			exponentialHist: func() pmetric.ExponentialHistogramDataPoint {
				pt := pmetric.NewExponentialHistogramDataPoint()
				pt.SetStartTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(100)))
				pt.SetTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(500)))
				pt.SetCount(4)
				pt.SetSum(10.1)
				pt.SetScale(1)
				pt.SetZeroCount(1)

				pt.Positive().BucketCounts().FromRaw([]uint64{1, 1})
				pt.Positive().SetOffset(1)

				pt.Negative().BucketCounts().FromRaw([]uint64{1, 1})
				pt.Negative().SetOffset(1)

				return pt
			},
			wantNativeHist: func() writev2.Histogram {
				return writev2.Histogram{
					Count:          &writev2.Histogram_CountInt{CountInt: 4},
					Sum:            10.1,
					Schema:         1,
					ZeroThreshold:  defaultZeroThreshold,
					ZeroCount:      &writev2.Histogram_ZeroCountInt{ZeroCountInt: 1},
					NegativeSpans:  []writev2.BucketSpan{{Offset: 2, Length: 2}},
					NegativeDeltas: []int64{1, 0},
					PositiveSpans:  []writev2.BucketSpan{{Offset: 2, Length: 2}},
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

				pt.SetCount(4)
				pt.SetScale(1)
				pt.SetZeroCount(1)

				pt.Positive().BucketCounts().FromRaw([]uint64{1, 1})
				pt.Positive().SetOffset(1)

				pt.Negative().BucketCounts().FromRaw([]uint64{1, 1})
				pt.Negative().SetOffset(1)

				return pt
			},
			wantNativeHist: func() writev2.Histogram {
				return writev2.Histogram{
					Count:          &writev2.Histogram_CountInt{CountInt: 4},
					Schema:         1,
					ZeroThreshold:  defaultZeroThreshold,
					ZeroCount:      &writev2.Histogram_ZeroCountInt{ZeroCountInt: 1},
					NegativeSpans:  []writev2.BucketSpan{{Offset: 2, Length: 2}},
					NegativeDeltas: []int64{1, 0},
					PositiveSpans:  []writev2.BucketSpan{{Offset: 2, Length: 2}},
					PositiveDeltas: []int64{1, 0},
					Timestamp:      500,
				}
			},
		},
		{
			name: "invalid negative scale",
			exponentialHist: func() pmetric.ExponentialHistogramDataPoint {
				pt := pmetric.NewExponentialHistogramDataPoint()
				pt.SetScale(-10)
				return pt
			},
			wantErrMessage: "cannot convert exponential to native histogram." +
				" Scale must be >= -4, was -10",
		},
		{
			name: "no downscaling at scale 8",
			exponentialHist: func() pmetric.ExponentialHistogramDataPoint {
				pt := pmetric.NewExponentialHistogramDataPoint()
				pt.SetTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(500)))
				pt.SetCount(6)
				pt.SetSum(10.1)
				pt.SetScale(8)
				pt.SetZeroCount(1)

				pt.Positive().BucketCounts().FromRaw([]uint64{1, 1, 1})
				pt.Positive().SetOffset(1)

				pt.Negative().BucketCounts().FromRaw([]uint64{1, 1, 1})
				pt.Negative().SetOffset(2)
				return pt
			},
			wantNativeHist: func() writev2.Histogram {
				return writev2.Histogram{
					Count:          &writev2.Histogram_CountInt{CountInt: 6},
					Sum:            10.1,
					Schema:         8,
					ZeroThreshold:  defaultZeroThreshold,
					ZeroCount:      &writev2.Histogram_ZeroCountInt{ZeroCountInt: 1},
					PositiveSpans:  []writev2.BucketSpan{{Offset: 2, Length: 3}},
					PositiveDeltas: []int64{1, 0, 0}, // 1, 1, 1
					NegativeSpans:  []writev2.BucketSpan{{Offset: 3, Length: 3}},
					NegativeDeltas: []int64{1, 0, 0}, // 1, 1, 1
					Timestamp:      500,
				}
			},
		},
		{
			name: "downsample if scale is more than 8",
			exponentialHist: func() pmetric.ExponentialHistogramDataPoint {
				pt := pmetric.NewExponentialHistogramDataPoint()
				pt.SetTimestamp(pcommon.NewTimestampFromTime(time.UnixMilli(500)))
				pt.SetCount(6)
				pt.SetSum(10.1)
				pt.SetScale(9)
				pt.SetZeroCount(1)

				pt.Positive().BucketCounts().FromRaw([]uint64{1, 1, 1})
				pt.Positive().SetOffset(1)

				pt.Negative().BucketCounts().FromRaw([]uint64{1, 1, 1})
				pt.Negative().SetOffset(2)
				return pt
			},
			wantNativeHist: func() writev2.Histogram {
				return writev2.Histogram{
					Count:          &writev2.Histogram_CountInt{CountInt: 6},
					Sum:            10.1,
					Schema:         8,
					ZeroThreshold:  defaultZeroThreshold,
					ZeroCount:      &writev2.Histogram_ZeroCountInt{ZeroCountInt: 1},
					PositiveSpans:  []writev2.BucketSpan{{Offset: 1, Length: 2}},
					PositiveDeltas: []int64{1, 1}, // 0+1, 1+1 = 1, 2
					NegativeSpans:  []writev2.BucketSpan{{Offset: 2, Length: 2}},
					NegativeDeltas: []int64{2, -1}, // 1+1, 1+0 = 2, 1
					Timestamp:      500,
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validateExponentialHistogramCountV2(t, tt.exponentialHist()) // Sanity check.
			got, err := exponentialToNativeHistogramV2(tt.exponentialHist())
			if tt.wantErrMessage != "" {
				assert.ErrorContains(t, err, tt.wantErrMessage)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantNativeHist(), got)
			validateNativeHistogramCountV2(t, got)
		})
	}
}

func validateExponentialHistogramCountV2(t *testing.T, h pmetric.ExponentialHistogramDataPoint) {
	actualCount := uint64(0)
	for _, bucket := range h.Positive().BucketCounts().AsRaw() {
		actualCount += bucket
	}
	for _, bucket := range h.Negative().BucketCounts().AsRaw() {
		actualCount += bucket
	}
	require.Equal(t, h.Count(), actualCount, "exponential histogram count mismatch")
}

func validateNativeHistogramCountV2(t *testing.T, h writev2.Histogram) {
	require.NotNil(t, h.Count)
	require.IsType(t, &writev2.Histogram_CountInt{}, h.Count)
	want := h.Count.(*writev2.Histogram_CountInt).CountInt
	var (
		actualCount uint64
		prevBucket  int64
	)
	for _, delta := range h.PositiveDeltas {
		prevBucket += delta
		actualCount += uint64(prevBucket)
	}
	prevBucket = 0
	for _, delta := range h.NegativeDeltas {
		prevBucket += delta
		actualCount += uint64(prevBucket)
	}
	assert.Equal(t, want, actualCount, "native histogram count mismatch")
}

func TestPrometheusConverterV2_addExponentialHistogramDataPoints(t *testing.T) {
	tests := []struct {
		name       string
		metric     func() pmetric.Metric
		wantSeries func() map[uint64]*writev2.TimeSeries
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
			wantSeries: func() map[uint64]*writev2.TimeSeries {
				labels := []prompb.Label{
					{Name: model.MetricNameLabel, Value: "test_hist"},
					{Name: "attr", Value: "test_attr"},
				}
				return map[uint64]*writev2.TimeSeries{
					timeSeriesSignature(labels): {
						LabelsRefs: []uint32{1, 2, 3, 4},
						Histograms: []writev2.Histogram{
							{
								Count:          &writev2.Histogram_CountInt{CountInt: 7},
								Schema:         1,
								ZeroThreshold:  defaultZeroThreshold,
								ZeroCount:      &writev2.Histogram_ZeroCountInt{ZeroCountInt: 0},
								PositiveSpans:  []writev2.BucketSpan{{Offset: 0, Length: 2}},
								PositiveDeltas: []int64{4, -2},
							},
							{
								Count:          &writev2.Histogram_CountInt{CountInt: 4},
								Schema:         1,
								ZeroThreshold:  defaultZeroThreshold,
								ZeroCount:      &writev2.Histogram_ZeroCountInt{ZeroCountInt: 0},
								PositiveSpans:  []writev2.BucketSpan{{Offset: 0, Length: 3}},
								PositiveDeltas: []int64{4, -2, -1},
							},
						},
						Metadata: writev2.Metadata{
							Type: writev2.Metadata_METRIC_TYPE_HISTOGRAM,
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
			wantSeries: func() map[uint64]*writev2.TimeSeries {
				labels := []prompb.Label{
					{Name: model.MetricNameLabel, Value: "test_hist"},
					{Name: "attr", Value: "test_attr"},
				}
				labelsAnother := []prompb.Label{
					{Name: model.MetricNameLabel, Value: "test_hist"},
					{Name: "attr", Value: "test_attr_two"},
				}

				return map[uint64]*writev2.TimeSeries{
					timeSeriesSignature(labels): {
						LabelsRefs: []uint32{1, 2, 3, 4},
						Histograms: []writev2.Histogram{
							{
								Count:          &writev2.Histogram_CountInt{CountInt: 7},
								Schema:         1,
								ZeroThreshold:  defaultZeroThreshold,
								ZeroCount:      &writev2.Histogram_ZeroCountInt{ZeroCountInt: 0},
								PositiveSpans:  []writev2.BucketSpan{{Offset: 0, Length: 2}},
								PositiveDeltas: []int64{4, -2},
							},
						},
						Metadata: writev2.Metadata{
							Type: writev2.Metadata_METRIC_TYPE_HISTOGRAM,
						},
					},
					timeSeriesSignature(labelsAnother): {
						LabelsRefs: []uint32{1, 2, 3, 5},
						Histograms: []writev2.Histogram{
							{
								Count:          &writev2.Histogram_CountInt{CountInt: 4},
								Schema:         1,
								ZeroThreshold:  defaultZeroThreshold,
								ZeroCount:      &writev2.Histogram_ZeroCountInt{ZeroCountInt: 0},
								NegativeSpans:  []writev2.BucketSpan{{Offset: 0, Length: 3}},
								NegativeDeltas: []int64{4, -2, -1},
							},
						},
						Metadata: writev2.Metadata{
							Type: writev2.Metadata_METRIC_TYPE_HISTOGRAM,
						},
					},
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metric := tt.metric()
			unitNamer := otlptranslator.UnitNamer{}
			m := metadata{
				Type: otelMetricTypeToPromMetricTypeV2(metric),
				Help: metric.Description(),
				Unit: unitNamer.Build(metric.Unit()),
			}
			converter := newPrometheusConverterV2(Settings{})
			metricNamer := otlptranslator.MetricNamer{WithMetricSuffixes: true}
			metricName, err := metricNamer.Build(prom.TranslatorMetricFromOtelMetric(metric))
			require.NoError(t, err)
			require.NoError(t, converter.addExponentialHistogramDataPoints(
				metric.ExponentialHistogram().DataPoints(),
				pcommon.NewResource(),
				pcommon.NewInstrumentationScope(),
				Settings{},
				metricName,
				m,
			))

			x := tt.wantSeries()
			assert.Equal(t, x, converter.unique)
			assert.Empty(t, converter.conflicts)
		})
	}
}
