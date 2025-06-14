// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"fmt"
	"testing"

	"cloud.google.com/go/monitoring/apiv3/v2/monitoringpb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
	"google.golang.org/genproto/googleapis/api/distribution"
	"google.golang.org/genproto/googleapis/api/metric"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestConvertGaugeToMetrics_ValidGaugePoints(t *testing.T) {
	logger := zap.NewNop()
	mb := NewMetricsBuilder(logger)

	ts := &monitoringpb.TimeSeries{
		Points: []*monitoringpb.Point{
			{
				Interval: &monitoringpb.TimeInterval{
					StartTime: &timestamppb.Timestamp{Seconds: 10},
					EndTime:   &timestamppb.Timestamp{Seconds: 20},
				},
				Value: &monitoringpb.TypedValue{
					Value: &monitoringpb.TypedValue_DoubleValue{DoubleValue: 42.0},
				},
			},
		},
	}

	m := pmetric.NewMetric()
	mb.ConvertGaugeToMetrics(ts, m)

	assert.Equal(t, 1, m.Gauge().DataPoints().Len())
	dp := m.Gauge().DataPoints().At(0)
	assert.Equal(t, int64(10), dp.StartTimestamp().AsTime().Unix())
	assert.Equal(t, int64(20), dp.Timestamp().AsTime().Unix())
	assert.Equal(t, 42.0, dp.DoubleValue())
}

func TestConvertGaugeToMetrics_InvalidEndTime(t *testing.T) {
	logger := zap.NewNop()
	mb := NewMetricsBuilder(logger)

	ts := &monitoringpb.TimeSeries{
		Points: []*monitoringpb.Point{
			{
				Interval: &monitoringpb.TimeInterval{
					StartTime: &timestamppb.Timestamp{Seconds: 10},
					EndTime:   nil,
				},
				Value: &monitoringpb.TypedValue{
					Value: &monitoringpb.TypedValue_Int64Value{Int64Value: 100},
				},
			},
		},
	}

	m := pmetric.NewMetric()
	mb.ConvertGaugeToMetrics(ts, m)

	assert.Equal(t, 1, m.Gauge().DataPoints().Len())
	dp := m.Gauge().DataPoints().At(0)
	assert.Equal(t, int64(10), dp.StartTimestamp().AsTime().Unix())
	assert.Equal(t, int64(100), dp.IntValue())
}

func TestConvertDistributionToMetrics_NoDataPoints(t *testing.T) {
	logger := zap.NewNop()
	mb := NewMetricsBuilder(logger)
	ts := &monitoringpb.TimeSeries{
		Metric: &metric.Metric{},
	}
	m := pmetric.NewMetric()
	mb.ConvertDistributionToMetrics(ts, m)
	require.Equal(t, 0, m.Histogram().DataPoints().Len())
}

func TestConvertDistributionToMetrics_InvalidConversion_NoInterval(t *testing.T) {
	logger := zap.NewNop()
	mb := NewMetricsBuilder(logger)
	ts := &monitoringpb.TimeSeries{
		Metric: &metric.Metric{},
		Points: []*monitoringpb.Point{
			{
				Value: &monitoringpb.TypedValue{
					Value: &monitoringpb.TypedValue_DistributionValue{
						DistributionValue: &distribution.Distribution{
							Count: 0,
							BucketOptions: &distribution.Distribution_BucketOptions{
								Options: &distribution.Distribution_BucketOptions_ExplicitBuckets{
									ExplicitBuckets: &distribution.Distribution_BucketOptions_Explicit{
										Bounds: []float64{},
									},
								},
							},
							BucketCounts: []int64{},
						},
					},
				},
			},
		},
	}
	m := pmetric.NewMetric()
	mb.ConvertDistributionToMetrics(ts, m)
	require.Equal(t, 0, m.Histogram().DataPoints().Len())
}

func TestConvertDistributionToMetrics_ValidConversion_ExplicitBuckets_SingleDataPoint_WithExemplars(t *testing.T) {
	logger := zap.NewNop()
	mb := NewMetricsBuilder(logger)

	sourceProjectID := "test-project"
	sourceTraceID := "1234567890abcdef1234567890abcdef"
	sourceSpanID := "abcdef1234567890"
	sourceSpanName := fmt.Sprintf("projects/%s/traces/%s/spans/%s", sourceProjectID, sourceTraceID, sourceSpanID)
	sourceExemplarAttachmentTraceContext, err := anypb.New(&monitoringpb.SpanContext{
		SpanName: sourceSpanName,
	})
	require.NoError(t, err)
	sourceDroppedLabels := map[string]string{
		"dropped_key_1": "dropped value 1",
		"dropped_key_2": "dropped value 2",
	}
	sourceExemplarAttachmentDroppedLabels, err := anypb.New(&monitoringpb.DroppedLabels{
		Label: sourceDroppedLabels,
	})
	require.NoError(t, err)
	sourceArbitraryValue, err := structpb.NewValue(map[string]any{
		"string": "value",
		"number": 13,
	})
	require.NoError(t, err)
	exemplarAttachmentArbitrary, err := anypb.New(sourceArbitraryValue)
	require.NoError(t, err)

	sourceBucketCounts := []int64{
		5,  // [-inifinity, 11.1)
		10, // [11.1, 22.2)
		15, // [22.2, 33)
		11, // [33.3, +infinity)
	}
	sourceCountTotal := int64(0)
	for _, bucketCount := range sourceBucketCounts {
		sourceCountTotal += bucketCount
	}
	ts := &monitoringpb.TimeSeries{
		Metric: &metric.Metric{
			Labels: map[string]string{"key1": "value1", "key2": "value2"},
		},
		Points: []*monitoringpb.Point{
			{
				Interval: &monitoringpb.TimeInterval{
					StartTime: &timestamppb.Timestamp{Seconds: 13},
					EndTime:   &timestamppb.Timestamp{Seconds: 73},
				},
				Value: &monitoringpb.TypedValue{
					Value: &monitoringpb.TypedValue_DistributionValue{
						DistributionValue: &distribution.Distribution{
							Count: sourceCountTotal,
							BucketOptions: &distribution.Distribution_BucketOptions{
								Options: &distribution.Distribution_BucketOptions_ExplicitBuckets{
									ExplicitBuckets: &distribution.Distribution_BucketOptions_Explicit{
										Bounds: []float64{11.1, 22.2, 33.3},
									},
								},
							},
							BucketCounts: sourceBucketCounts,

							Exemplars: []*distribution.Distribution_Exemplar{
								{
									Timestamp:   &timestamppb.Timestamp{Seconds: 22},
									Value:       15.5,
									Attachments: []*anypb.Any{sourceExemplarAttachmentTraceContext, sourceExemplarAttachmentDroppedLabels},
								},
								{
									Timestamp:   &timestamppb.Timestamp{Seconds: 33},
									Value:       23.3,
									Attachments: []*anypb.Any{exemplarAttachmentArbitrary},
								},
							},
						},
					},
				},
			},
		},
	}

	m := pmetric.NewMetric()
	mb.ConvertDistributionToMetrics(ts, m)

	histogram := m.Histogram()
	assert.Equal(t, pmetric.AggregationTemporalityDelta, histogram.AggregationTemporality())
	require.Equal(t, 1, histogram.DataPoints().Len())
	targetDataPoint := histogram.DataPoints().At(0)

	attributes := targetDataPoint.Attributes()
	require.Equal(t, 2, attributes.Len())
	attr, ok := attributes.Get("key1")
	assert.True(t, ok)
	assert.Equal(t, "value1", attr.Str())
	attr, ok = attributes.Get("key2")
	assert.True(t, ok)
	assert.Equal(t, "value2", attr.Str())

	assert.Equal(t, int64(13), targetDataPoint.StartTimestamp().AsTime().Unix())
	assert.Equal(t, int64(73), targetDataPoint.Timestamp().AsTime().Unix())
	assert.Equal(t, uint64(41), targetDataPoint.Count())

	bucketCounts := targetDataPoint.BucketCounts()
	require.Equal(t, len(sourceBucketCounts), bucketCounts.Len())
	for i, countValue := range sourceBucketCounts {
		assert.Equal(t, uint64(countValue), bucketCounts.At(i))
	}

	bounds := targetDataPoint.ExplicitBounds()
	require.Equal(t, 3, bounds.Len())
	assert.Equal(t, 11.1, bounds.At(0))
	assert.Equal(t, 22.2, bounds.At(1))
	assert.Equal(t, 33.3, bounds.At(2))

	assert.False(t, targetDataPoint.HasSum())
	assert.False(t, targetDataPoint.HasMin())
	assert.False(t, targetDataPoint.HasMax())

	targetExemplars := targetDataPoint.Exemplars()
	require.Equal(t, 2, targetExemplars.Len())
	targetExemplar1 := targetExemplars.At(0)
	assert.Equal(t, pmetric.ExemplarValueTypeDouble, targetExemplar1.ValueType())
	assert.Equal(t, float64(15.5), targetExemplar1.DoubleValue())
	assert.False(t, targetExemplar1.TraceID().IsEmpty())
	assert.Equal(t, sourceTraceID, targetExemplar1.TraceID().String())
	assert.False(t, targetExemplar1.SpanID().IsEmpty())
	assert.Equal(t, sourceSpanID, targetExemplar1.SpanID().String())
	targetExemplar1FilteredAttributes := targetExemplar1.FilteredAttributes()
	assert.Equal(t, 1, targetExemplar1FilteredAttributes.Len())
	targetDroppedLabelsAttr, ok := targetExemplar1FilteredAttributes.Get("type.googleapis.com/google.monitoring.v3.DroppedLabels")
	assert.True(t, ok)
	targetDroppedLabelsMap := targetDroppedLabelsAttr.Map()
	assert.Equal(t, 2, targetDroppedLabelsMap.Len())
	dl1, ok := targetDroppedLabelsMap.Get("dropped_key_1")
	require.True(t, ok)
	assert.Equal(t, "dropped value 1", dl1.Str())
	dl2, ok := targetDroppedLabelsMap.Get("dropped_key_2")
	require.True(t, ok)
	assert.Equal(t, "dropped value 2", dl2.Str())
	targetExemplar2 := targetExemplars.At(1)
	assert.Equal(t, pmetric.ExemplarValueTypeDouble, targetExemplar2.ValueType())
	assert.Equal(t, float64(23.3), targetExemplar2.DoubleValue())
	assert.Equal(t, 0, targetExemplar2.FilteredAttributes().Len())
}

func TestConvertDistributionToMetrics_ValidConversion_ExplicitBuckets_SingleDataPoint_ZeroBoundsZeroCounts(t *testing.T) {
	logger := zap.NewNop()
	mb := NewMetricsBuilder(logger)

	ts := &monitoringpb.TimeSeries{
		Metric: &metric.Metric{},
		Points: []*monitoringpb.Point{
			{
				Interval: &monitoringpb.TimeInterval{
					StartTime: &timestamppb.Timestamp{Seconds: 13},
					EndTime:   &timestamppb.Timestamp{Seconds: 73},
				},
				Value: &monitoringpb.TypedValue{
					Value: &monitoringpb.TypedValue_DistributionValue{
						DistributionValue: &distribution.Distribution{
							Count: int64(0),
							BucketOptions: &distribution.Distribution_BucketOptions{
								Options: &distribution.Distribution_BucketOptions_ExplicitBuckets{
									ExplicitBuckets: &distribution.Distribution_BucketOptions_Explicit{
										Bounds: []float64{},
									},
								},
							},
							BucketCounts: []int64{},
						},
					},
				},
			},
		},
	}

	m := pmetric.NewMetric()
	mb.ConvertDistributionToMetrics(ts, m)

	histogram := m.Histogram()
	assert.Equal(t, pmetric.AggregationTemporalityDelta, histogram.AggregationTemporality())
	require.Equal(t, 1, histogram.DataPoints().Len())
	targetDataPoint := histogram.DataPoints().At(0)

	attributes := targetDataPoint.Attributes()
	require.Equal(t, 0, attributes.Len())

	assert.Equal(t, int64(13), targetDataPoint.StartTimestamp().AsTime().Unix())
	assert.Equal(t, int64(73), targetDataPoint.Timestamp().AsTime().Unix())
	assert.Equal(t, uint64(0), targetDataPoint.Count())

	bucketCounts := targetDataPoint.BucketCounts()
	require.Equal(t, 0, bucketCounts.Len())
	bounds := targetDataPoint.ExplicitBounds()
	require.Equal(t, 0, bounds.Len())

	assert.False(t, targetDataPoint.HasSum())
	assert.False(t, targetDataPoint.HasMin())
	assert.False(t, targetDataPoint.HasMax())
	assert.Equal(t, 0, targetDataPoint.Exemplars().Len())
}

func TestConvertDistributionToMetrics_ValidConversion_ExplicitBuckets_SingleDataPoint_OnlyUnderAndOverflow(t *testing.T) {
	logger := zap.NewNop()
	mb := NewMetricsBuilder(logger)

	sourceBucketCounts := []int64{
		5,  // [-inifinity, 22.2)
		11, // [22.2, +infinity)
	}
	sourceCountTotal := int64(0)
	for _, bucketCount := range sourceBucketCounts {
		sourceCountTotal += bucketCount
	}
	ts := &monitoringpb.TimeSeries{
		Metric: &metric.Metric{},
		Points: []*monitoringpb.Point{
			{
				Interval: &monitoringpb.TimeInterval{
					StartTime: &timestamppb.Timestamp{Seconds: 13},
					EndTime:   &timestamppb.Timestamp{Seconds: 73},
				},
				Value: &monitoringpb.TypedValue{
					Value: &monitoringpb.TypedValue_DistributionValue{
						DistributionValue: &distribution.Distribution{
							Count: sourceCountTotal,
							BucketOptions: &distribution.Distribution_BucketOptions{
								Options: &distribution.Distribution_BucketOptions_ExplicitBuckets{
									ExplicitBuckets: &distribution.Distribution_BucketOptions_Explicit{
										Bounds: []float64{22.2},
									},
								},
							},
							BucketCounts: sourceBucketCounts,
						},
					},
				},
			},
		},
	}

	m := pmetric.NewMetric()
	mb.ConvertDistributionToMetrics(ts, m)

	histogram := m.Histogram()
	assert.Equal(t, pmetric.AggregationTemporalityDelta, histogram.AggregationTemporality())
	require.Equal(t, 1, histogram.DataPoints().Len())
	targetDataPoint := histogram.DataPoints().At(0)

	attributes := targetDataPoint.Attributes()
	require.Equal(t, 0, attributes.Len())

	assert.Equal(t, int64(13), targetDataPoint.StartTimestamp().AsTime().Unix())
	assert.Equal(t, int64(73), targetDataPoint.Timestamp().AsTime().Unix())
	assert.Equal(t, uint64(16), targetDataPoint.Count())

	bucketCounts := targetDataPoint.BucketCounts()
	require.Equal(t, len(sourceBucketCounts), bucketCounts.Len())
	for i, countValue := range sourceBucketCounts {
		assert.Equal(t, uint64(countValue), bucketCounts.At(i))
	}

	bounds := targetDataPoint.ExplicitBounds()
	require.Equal(t, 1, bounds.Len())
	assert.Equal(t, 22.2, bounds.At(0))

	assert.False(t, targetDataPoint.HasSum())
	assert.False(t, targetDataPoint.HasMin())
	assert.False(t, targetDataPoint.HasMax())
	assert.Equal(t, 0, targetDataPoint.Exemplars().Len())
}

func TestConvertDistributionToMetrics_ValidConversion_ExplicitBuckets_MultipleDataPoint(t *testing.T) {
	logger := zap.NewNop()
	mb := NewMetricsBuilder(logger)

	sourceBucketCountsAllDataPoints := [][]int64{
		{5, 0, 15, 11},
		{6, -1, 16, 12},
		{7, 0, 17, 13},
	}
	boundsAllDataPoints := [][]float64{
		{11.1, 22.2, 33.3},
		{111.1, 122.2, 133.3},
		{211.1, 222.2, 233.3},
	}
	sourceCountTotalAllDataPoints := []int64{0, 0, 0}
	for i, sourceBucketCountsForDataPoint := range sourceBucketCountsAllDataPoints {
		for _, bucketCount := range sourceBucketCountsForDataPoint {
			sourceCountTotalAllDataPoints[i] += bucketCount
		}
	}
	dataPoints := make([]*monitoringpb.Point, 0, len(sourceBucketCountsAllDataPoints))
	for i := range sourceBucketCountsAllDataPoints {
		dataPoints = append(dataPoints, &monitoringpb.Point{
			Interval: &monitoringpb.TimeInterval{
				StartTime: &timestamppb.Timestamp{Seconds: 13 + int64(i)*60},
				EndTime:   &timestamppb.Timestamp{Seconds: 13 + int64(i+1)*60},
			},
			Value: &monitoringpb.TypedValue{
				Value: &monitoringpb.TypedValue_DistributionValue{
					DistributionValue: &distribution.Distribution{
						Count: sourceCountTotalAllDataPoints[i],
						BucketOptions: &distribution.Distribution_BucketOptions{
							Options: &distribution.Distribution_BucketOptions_ExplicitBuckets{
								ExplicitBuckets: &distribution.Distribution_BucketOptions_Explicit{
									Bounds: boundsAllDataPoints[i],
								},
							},
						},
						BucketCounts: sourceBucketCountsAllDataPoints[i],
					},
				},
			},
		})
	}
	ts := &monitoringpb.TimeSeries{
		Metric: &metric.Metric{
			Labels: map[string]string{"key1": "value1", "key2": "value2"},
		},
		Points: dataPoints,
	}

	m := pmetric.NewMetric()
	mb.ConvertDistributionToMetrics(ts, m)

	histogram := m.Histogram()
	assert.Equal(t, pmetric.AggregationTemporalityDelta, histogram.AggregationTemporality())
	require.Equal(t, 3, histogram.DataPoints().Len())

	for i := 0; i < histogram.DataPoints().Len(); i++ {
		targetDataPoint := histogram.DataPoints().At(i)
		attributes := targetDataPoint.Attributes()
		require.Equal(t, 2, attributes.Len())
		attr, ok := attributes.Get("key1")
		assert.True(t, ok)
		assert.Equal(t, "value1", attr.Str())
		attr, ok = attributes.Get("key2")
		assert.True(t, ok)
		assert.Equal(t, "value2", attr.Str())

		assert.Equal(t, 13+60*int64(i), targetDataPoint.StartTimestamp().AsTime().Unix())
		assert.Equal(t, 13+60*int64(i+1), targetDataPoint.Timestamp().AsTime().Unix())
		assert.Equal(t, 31+3*i, int(targetDataPoint.Count()))

		bucketCounts := targetDataPoint.BucketCounts()
		require.Equal(t, len(sourceBucketCountsAllDataPoints[i]), bucketCounts.Len())
		for i, sourceCountValue := range sourceBucketCountsAllDataPoints[i] {
			if sourceCountValue < 0 {
				assert.Equal(t, uint64(0), bucketCounts.At(i))
			} else {
				assert.Equal(t, uint64(sourceCountValue), bucketCounts.At(i))
			}
		}

		bounds := targetDataPoint.ExplicitBounds()
		require.Equal(t, 3, bounds.Len())
		assert.Equal(t, boundsAllDataPoints[i][0], bounds.At(0))
		assert.Equal(t, boundsAllDataPoints[i][1], bounds.At(1))
		assert.Equal(t, boundsAllDataPoints[i][2], bounds.At(2))

		assert.False(t, targetDataPoint.HasSum())
		assert.False(t, targetDataPoint.HasMin())
		assert.False(t, targetDataPoint.HasMax())
		assert.Equal(t, 0, targetDataPoint.Exemplars().Len())
	}
}

func TestConvertDistributionToMetrics_ValidConversion_LinearBuckets_SingleDataPoint(t *testing.T) {
	logger := zap.NewNop()
	mb := NewMetricsBuilder(logger)

	sourceBucketCounts := []int64{
		5,  // [-infinity, 11.1)
		10, // [11.1, 18.8)
		15, // [18.8, 26.5)
		11, // [26.5 +infinity)
	}
	sourceCountTotal := int64(0)
	for _, bucketCount := range sourceBucketCounts {
		sourceCountTotal += bucketCount
	}
	ts := &monitoringpb.TimeSeries{
		Metric: &metric.Metric{
			Labels: map[string]string{"key1": "value1", "key2": "value2"},
		},
		Points: []*monitoringpb.Point{
			{
				Interval: &monitoringpb.TimeInterval{
					StartTime: &timestamppb.Timestamp{Seconds: 13},
					EndTime:   &timestamppb.Timestamp{Seconds: 73},
				},
				Value: &monitoringpb.TypedValue{
					Value: &monitoringpb.TypedValue_DistributionValue{
						DistributionValue: &distribution.Distribution{
							Count: sourceCountTotal,
							BucketOptions: &distribution.Distribution_BucketOptions{
								Options: &distribution.Distribution_BucketOptions_LinearBuckets{
									LinearBuckets: &distribution.Distribution_BucketOptions_Linear{
										// [-infinity, 11.1), [11.1, 18.8), [18.8, 26.5), [26.5 +infinity)
										NumFiniteBuckets: 2,
										Offset:           11.1,
										Width:            7.7,
									},
								},
							},
							BucketCounts: sourceBucketCounts,
						},
					},
				},
			},
		},
	}

	m := pmetric.NewMetric()
	mb.ConvertDistributionToMetrics(ts, m)

	histogram := m.Histogram()
	assert.Equal(t, pmetric.AggregationTemporalityDelta, histogram.AggregationTemporality())
	require.Equal(t, 1, histogram.DataPoints().Len())
	targetDataPoint := histogram.DataPoints().At(0)

	attributes := targetDataPoint.Attributes()
	require.Equal(t, 2, attributes.Len())
	attr, ok := attributes.Get("key1")
	assert.True(t, ok)
	assert.Equal(t, "value1", attr.Str())
	attr, ok = attributes.Get("key2")
	assert.True(t, ok)
	assert.Equal(t, "value2", attr.Str())

	assert.Equal(t, int64(13), targetDataPoint.StartTimestamp().AsTime().Unix())
	assert.Equal(t, int64(73), targetDataPoint.Timestamp().AsTime().Unix())
	assert.Equal(t, uint64(41), targetDataPoint.Count())

	bucketCounts := targetDataPoint.BucketCounts()
	require.Equal(t, len(sourceBucketCounts), bucketCounts.Len())
	for i, countValue := range sourceBucketCounts {
		assert.Equal(t, uint64(countValue), bucketCounts.At(i))
	}

	bounds := targetDataPoint.ExplicitBounds()
	require.Equal(t, 3, bounds.Len())
	assert.Equal(t, 11.1, bounds.At(0))
	assert.Equal(t, 18.8, bounds.At(1))
	assert.Equal(t, 26.5, bounds.At(2))

	assert.False(t, targetDataPoint.HasSum())
	assert.False(t, targetDataPoint.HasMin())
	assert.False(t, targetDataPoint.HasMax())
	assert.Equal(t, 0, targetDataPoint.Exemplars().Len())
}

func TestConvertDistributionToMetrics_ValidConversion_LinearBuckets_SingleDataPoint_OnlyUnderAndOverflow(t *testing.T) {
	logger := zap.NewNop()
	mb := NewMetricsBuilder(logger)

	sourceBucketCounts := []int64{
		5,  // [-infinity, 11.1)
		11, // [11.1 +infinity)
	}
	sourceCountTotal := int64(0)
	for _, bucketCount := range sourceBucketCounts {
		sourceCountTotal += bucketCount
	}
	ts := &monitoringpb.TimeSeries{
		Metric: &metric.Metric{},
		Points: []*monitoringpb.Point{
			{
				Interval: &monitoringpb.TimeInterval{
					StartTime: &timestamppb.Timestamp{Seconds: 13},
					EndTime:   &timestamppb.Timestamp{Seconds: 73},
				},
				Value: &monitoringpb.TypedValue{
					Value: &monitoringpb.TypedValue_DistributionValue{
						DistributionValue: &distribution.Distribution{
							Count: sourceCountTotal,
							BucketOptions: &distribution.Distribution_BucketOptions{
								Options: &distribution.Distribution_BucketOptions_LinearBuckets{
									LinearBuckets: &distribution.Distribution_BucketOptions_Linear{
										// [-infinity, 11.1), [11.1, +infinity)
										NumFiniteBuckets: 0,
										Offset:           11.1,
										Width:            7.7,
									},
								},
							},
							BucketCounts: sourceBucketCounts,
						},
					},
				},
			},
		},
	}

	m := pmetric.NewMetric()
	mb.ConvertDistributionToMetrics(ts, m)

	histogram := m.Histogram()
	assert.Equal(t, pmetric.AggregationTemporalityDelta, histogram.AggregationTemporality())
	require.Equal(t, 1, histogram.DataPoints().Len())
	targetDataPoint := histogram.DataPoints().At(0)

	attributes := targetDataPoint.Attributes()
	require.Equal(t, 0, attributes.Len())

	assert.Equal(t, int64(13), targetDataPoint.StartTimestamp().AsTime().Unix())
	assert.Equal(t, int64(73), targetDataPoint.Timestamp().AsTime().Unix())
	assert.Equal(t, uint64(16), targetDataPoint.Count())

	bucketCounts := targetDataPoint.BucketCounts()
	require.Equal(t, len(sourceBucketCounts), bucketCounts.Len())
	for i, countValue := range sourceBucketCounts {
		assert.Equal(t, uint64(countValue), bucketCounts.At(i))
	}

	bounds := targetDataPoint.ExplicitBounds()
	require.Equal(t, 1, bounds.Len())
	assert.Equal(t, 11.1, bounds.At(0))

	assert.False(t, targetDataPoint.HasSum())
	assert.False(t, targetDataPoint.HasMin())
	assert.False(t, targetDataPoint.HasMax())
	assert.Equal(t, 0, targetDataPoint.Exemplars().Len())
}

func TestConvertDistributionToMetrics_ValidConversion_ExponentialBuckets_SingleDataPoint(t *testing.T) {
	logger := zap.NewNop()
	mb := NewMetricsBuilder(logger)

	sourceBucketCounts := []int64{
		5,  // [-infinity, 10)
		10, // [10, 12)
		15, // [12, 14.4)
		11, // [14.4 +infinity)
	}
	sourceCountTotal := int64(0)
	for _, bucketCount := range sourceBucketCounts {
		sourceCountTotal += bucketCount
	}
	ts := &monitoringpb.TimeSeries{
		Metric: &metric.Metric{
			Labels: map[string]string{"key1": "value1", "key2": "value2"},
		},
		Points: []*monitoringpb.Point{
			{
				Interval: &monitoringpb.TimeInterval{
					StartTime: &timestamppb.Timestamp{Seconds: 13},
					EndTime:   &timestamppb.Timestamp{Seconds: 73},
				},
				Value: &monitoringpb.TypedValue{
					Value: &monitoringpb.TypedValue_DistributionValue{
						DistributionValue: &distribution.Distribution{
							Count: sourceCountTotal,
							BucketOptions: &distribution.Distribution_BucketOptions{
								Options: &distribution.Distribution_BucketOptions_ExponentialBuckets{
									ExponentialBuckets: &distribution.Distribution_BucketOptions_Exponential{
										// [-infinity, 10), [10, 12), [12, 14.4), [14.4 +infinity)
										NumFiniteBuckets: 2,
										GrowthFactor:     1.2,
										Scale:            10,
									},
								},
							},
							BucketCounts: sourceBucketCounts,
						},
					},
				},
			},
		},
	}

	m := pmetric.NewMetric()
	mb.ConvertDistributionToMetrics(ts, m)

	histogram := m.Histogram()
	assert.Equal(t, pmetric.AggregationTemporalityDelta, histogram.AggregationTemporality())
	require.Equal(t, 1, histogram.DataPoints().Len())
	targetDataPoint := histogram.DataPoints().At(0)

	attributes := targetDataPoint.Attributes()
	require.Equal(t, 2, attributes.Len())
	attr, ok := attributes.Get("key1")
	assert.True(t, ok)
	assert.Equal(t, "value1", attr.Str())
	attr, ok = attributes.Get("key2")
	assert.True(t, ok)
	assert.Equal(t, "value2", attr.Str())

	assert.Equal(t, int64(13), targetDataPoint.StartTimestamp().AsTime().Unix())
	assert.Equal(t, int64(73), targetDataPoint.Timestamp().AsTime().Unix())
	assert.Equal(t, uint64(41), targetDataPoint.Count())

	bucketCounts := targetDataPoint.BucketCounts()
	require.Equal(t, len(sourceBucketCounts), bucketCounts.Len())
	for i, countValue := range sourceBucketCounts {
		assert.Equal(t, uint64(countValue), bucketCounts.At(i))
	}

	bounds := targetDataPoint.ExplicitBounds()
	require.Equal(t, 3, bounds.Len())
	assert.Equal(t, float64(10), bounds.At(0))
	assert.Equal(t, float64(12), bounds.At(1))
	assert.InDelta(t, 14.4, bounds.At(2), 0.00000001, "explicit bound is '%f', expected 14.4", bounds.At(2))

	assert.False(t, targetDataPoint.HasSum())
	assert.False(t, targetDataPoint.HasMin())
	assert.False(t, targetDataPoint.HasMax())
	assert.Equal(t, 0, targetDataPoint.Exemplars().Len())
}

func TestConvertDistributionToMetrics_ValidConversion_ExponentialBuckets_SingleDataPoint_OnlyUnderAndOverflow(t *testing.T) {
	logger := zap.NewNop()
	mb := NewMetricsBuilder(logger)

	sourceBucketCounts := []int64{
		5,  // [-infinity, 10)
		11, // [10 +infinity)
	}
	sourceCountTotal := int64(0)
	for _, bucketCount := range sourceBucketCounts {
		sourceCountTotal += bucketCount
	}
	ts := &monitoringpb.TimeSeries{
		Metric: &metric.Metric{},
		Points: []*monitoringpb.Point{
			{
				Interval: &monitoringpb.TimeInterval{
					StartTime: &timestamppb.Timestamp{Seconds: 13},
					EndTime:   &timestamppb.Timestamp{Seconds: 73},
				},
				Value: &monitoringpb.TypedValue{
					Value: &monitoringpb.TypedValue_DistributionValue{
						DistributionValue: &distribution.Distribution{
							Count: sourceCountTotal,
							BucketOptions: &distribution.Distribution_BucketOptions{
								Options: &distribution.Distribution_BucketOptions_ExponentialBuckets{
									ExponentialBuckets: &distribution.Distribution_BucketOptions_Exponential{
										// [-infinity, 10), [10 +infinity)
										NumFiniteBuckets: 0,
										GrowthFactor:     1.2,
										Scale:            10,
									},
								},
							},
							BucketCounts: sourceBucketCounts,
						},
					},
				},
			},
		},
	}

	m := pmetric.NewMetric()
	mb.ConvertDistributionToMetrics(ts, m)

	histogram := m.Histogram()
	assert.Equal(t, pmetric.AggregationTemporalityDelta, histogram.AggregationTemporality())
	require.Equal(t, 1, histogram.DataPoints().Len())
	targetDataPoint := histogram.DataPoints().At(0)

	attributes := targetDataPoint.Attributes()
	require.Equal(t, 0, attributes.Len())

	assert.Equal(t, int64(13), targetDataPoint.StartTimestamp().AsTime().Unix())
	assert.Equal(t, int64(73), targetDataPoint.Timestamp().AsTime().Unix())
	assert.Equal(t, uint64(16), targetDataPoint.Count())

	bucketCounts := targetDataPoint.BucketCounts()
	require.Equal(t, len(sourceBucketCounts), bucketCounts.Len())
	for i, countValue := range sourceBucketCounts {
		assert.Equal(t, uint64(countValue), bucketCounts.At(i))
	}

	bounds := targetDataPoint.ExplicitBounds()
	require.Equal(t, 1, bounds.Len())
	assert.Equal(t, float64(10), bounds.At(0))

	assert.False(t, targetDataPoint.HasSum())
	assert.False(t, targetDataPoint.HasMin())
	assert.False(t, targetDataPoint.HasMax())
	assert.Equal(t, 0, targetDataPoint.Exemplars().Len())
}
