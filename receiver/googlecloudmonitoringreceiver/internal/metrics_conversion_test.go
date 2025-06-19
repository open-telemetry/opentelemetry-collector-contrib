// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"fmt"
	"path/filepath"
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

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

func TestConvertGaugeToMetrics(t *testing.T) {
	for _, tt := range []struct {
		name             string
		ts               *monitoringpb.TimeSeries
		fileNameExpected string
	}{
		{
			name: "valid gauge points",
			ts: &monitoringpb.TimeSeries{
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
			},
			fileNameExpected: "TestConvertGaugeToMetrics_ValidGaugePoints.yaml",
		},
		{
			name: "invalid end time",
			ts: &monitoringpb.TimeSeries{
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
			},
			fileNameExpected: "TestConvertGaugeToMetrics_InvalidEndTime.yaml",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			logger := zap.NewNop()
			mb := NewMetricsBuilder(logger)

			m := pmetric.NewMetric()
			mb.ConvertGaugeToMetrics(tt.ts, m)

			expectedFile := filepath.Join("testdata", tt.fileNameExpected)
			// Uncomment to regenerate the yaml file with the expected metrics:
			// require.NoError(t, golden.WriteMetrics(t, expectedFile, wrapMetric(m)))
			expectedMetrics, err := golden.ReadMetrics(expectedFile)
			require.NoError(t, err)
			assert.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, wrapMetric(m)))
		})
	}
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

	expectedFile := filepath.Join("testdata", "TestConvertDistributionToMetrics_ValidConversion_ExplicitBuckets_SingleDataPoint_WithExemplars.yaml")
	// Uncomment to regenerate the yaml file with the expected metrics:
	// require.NoError(t, golden.WriteMetrics(t, expectedFile, wrapMetric(m)))
	expectedMetrics, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err)
	assert.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, wrapMetric(m)))
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

	expectedFile := filepath.Join("testdata", "TestConvertDistributionToMetrics_ValidConversion_ExplicitBuckets_SingleDataPoint_ZeroBoundsZeroCounts.yaml")
	// Uncomment to regenerate the yaml file with the expected metrics:
	// require.NoError(t, golden.WriteMetrics(t, expectedFile, wrapMetric(m)))
	expectedMetrics, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err)
	assert.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, wrapMetric(m)))
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

	expectedFile := filepath.Join("testdata", "TestConvertDistributionToMetrics_ValidConversion_ExplicitBuckets_SingleDataPoint_OnlyUnderAndOverflow.yaml")
	// Uncomment to regenerate the yaml file with the expected metrics:
	// require.NoError(t, golden.WriteMetrics(t, expectedFile, wrapMetric(m)))
	expectedMetrics, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err)
	assert.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, wrapMetric(m)))
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

	expectedFile := filepath.Join("testdata", "TestConvertDistributionToMetrics_ValidConversion_ExplicitBuckets_MultipleDataPoint.yaml")
	// Uncomment to regenerate the yaml file with the expected metrics:
	// require.NoError(t, golden.WriteMetrics(t, expectedFile, wrapMetric(m)))
	expectedMetrics, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err)
	assert.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, wrapMetric(m)))
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

	expectedFile := filepath.Join("testdata", "TestConvertDistributionToMetrics_ValidConversion_LinearBuckets_SingleDataPoint.yaml")
	// Uncomment to regenerate the yaml file with the expected metrics:
	// require.NoError(t, golden.WriteMetrics(t, expectedFile, wrapMetric(m)))
	expectedMetrics, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err)
	assert.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, wrapMetric(m)))
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

	expectedFile := filepath.Join("testdata", "TestConvertDistributionToMetrics_ValidConversion_LinearBuckets_SingleDataPoint_OnlyUnderAndOverflow.yaml")
	// Uncomment to regenerate the yaml file with the expected metrics:
	// require.NoError(t, golden.WriteMetrics(t, expectedFile, wrapMetric(m)))
	expectedMetrics, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err)
	assert.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, wrapMetric(m)))
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

	expectedFile := filepath.Join("testdata", "TestConvertDistributionToMetrics_ValidConversion_ExponentialBuckets_SingleDataPoint.yaml")
	// Uncomment to regenerate the yaml file with the expected metrics:
	// require.NoError(t, golden.WriteMetrics(t, expectedFile, wrapMetric(m)))
	expectedMetrics, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err)
	assert.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, wrapMetric(m)))
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

	expectedFile := filepath.Join("testdata", "TestConvertDistributionToMetrics_ValidConversion_ExponentialBuckets_SingleDataPoint_OnlyUnderAndOverflow.yaml")
	// Uncomment to regenerate the yaml file with the expected metrics:
	// require.NoError(t, golden.WriteMetrics(t, expectedFile, wrapMetric(m)))
	expectedMetrics, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err)
	assert.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, wrapMetric(m)))
}

func wrapMetric(m pmetric.Metric) pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	sm := metrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty()
	m.CopyTo(sm.Metrics().AppendEmpty())
	return metrics
}
