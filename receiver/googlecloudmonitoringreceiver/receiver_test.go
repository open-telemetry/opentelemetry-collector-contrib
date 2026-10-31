// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlecloudmonitoringreceiver

import (
	"testing"
	"time"

	"cloud.google.com/go/monitoring/apiv3/v2/monitoringpb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
	"google.golang.org/genproto/googleapis/api/distribution"
	"google.golang.org/genproto/googleapis/api/metric"
	monitoredres "google.golang.org/genproto/googleapis/api/monitoredres"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestConvertGCPTimeSeriesToMetrics_DistributionSum(t *testing.T) {
	for _, tt := range []struct {
		name    string
		count   int64
		mean    float64
		wantSum float64
	}{
		{name: "single observation", count: 1, mean: 12.5, wantSum: 12.5},
		{name: "nonzero mean", count: 4, mean: 12.5, wantSum: 50},
		{name: "zero mean", count: 4},
		{name: "empty distribution"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			mr := newGoogleCloudMonitoringReceiver(&Config{}, zap.NewNop())
			metrics := pmetric.NewMetrics()
			ts := &monitoringpb.TimeSeries{
				Metric:     &metric.Metric{Type: "custom.googleapis.com/latency"},
				Resource:   &monitoredres.MonitoredResource{Type: "global"},
				MetricKind: metric.MetricDescriptor_DELTA,
				ValueType:  metric.MetricDescriptor_DISTRIBUTION,
				Points: []*monitoringpb.Point{{
					Interval: &monitoringpb.TimeInterval{
						StartTime: &timestamppb.Timestamp{Seconds: 100},
						EndTime:   &timestamppb.Timestamp{Seconds: 160},
					},
					Value: &monitoringpb.TypedValue{Value: &monitoringpb.TypedValue_DistributionValue{
						DistributionValue: &distribution.Distribution{
							Count: tt.count,
							Mean:  tt.mean,
							BucketOptions: &distribution.Distribution_BucketOptions{
								Options: &distribution.Distribution_BucketOptions_ExplicitBuckets{
									ExplicitBuckets: &distribution.Distribution_BucketOptions_Explicit{Bounds: []float64{20}},
								},
							},
							BucketCounts: []int64{tt.count},
						},
					}},
				}},
			}

			mr.convertGCPTimeSeriesToMetrics(metrics, &metric.MetricDescriptor{}, ts)

			require.Equal(t, 1, metrics.DataPointCount())
			dp := metrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Histogram().DataPoints().At(0)
			assert.Equal(t, uint64(tt.count), dp.Count())
			assert.Equal(t, tt.count > 0, dp.HasSum())
			assert.Equal(t, tt.wantSum, dp.Sum())
		})
	}
}

func TestConvertGCPTimeSeriesToMetrics_DeltaPointsOrdering(t *testing.T) {
	mr := newGoogleCloudMonitoringReceiver(&Config{}, zap.NewNop())
	metrics := pmetric.NewMetrics()

	metricDesc := &metric.MetricDescriptor{
		Name:        "storage.googleapis.com/api/request_count",
		Type:        "storage.googleapis.com/api/request_count",
		MetricKind:  metric.MetricDescriptor_DELTA,
		ValueType:   metric.MetricDescriptor_INT64,
		Unit:        "1",
		Description: "Delta request count",
	}

	// GCP returns points in reverse chronological order (newest first)
	timeSeries := &monitoringpb.TimeSeries{
		Metric: &metric.Metric{
			Type:   "storage.googleapis.com/api/request_count",
			Labels: map[string]string{"response_code": "200"},
		},
		Resource: &monitoredres.MonitoredResource{
			Type:   "gcs_bucket",
			Labels: map[string]string{"bucket_name": "my-bucket"},
		},
		MetricKind: metric.MetricDescriptor_DELTA,
		ValueType:  metric.MetricDescriptor_INT64,
		Points: []*monitoringpb.Point{
			{
				Interval: &monitoringpb.TimeInterval{
					StartTime: &timestamppb.Timestamp{Seconds: 100},
					EndTime:   &timestamppb.Timestamp{Seconds: 160},
				},
				Value: &monitoringpb.TypedValue{
					Value: &monitoringpb.TypedValue_Int64Value{Int64Value: 4},
				},
			},
			{
				Interval: &monitoringpb.TimeInterval{
					StartTime: &timestamppb.Timestamp{Seconds: 40},
					EndTime:   &timestamppb.Timestamp{Seconds: 100},
				},
				Value: &monitoringpb.TypedValue{
					Value: &monitoringpb.TypedValue_Int64Value{Int64Value: 6},
				},
			},
		},
	}

	mr.convertGCPTimeSeriesToMetrics(metrics, metricDesc, timeSeries)

	require.Equal(t, 1, metrics.ResourceMetrics().Len())
	sm := metrics.ResourceMetrics().At(0).ScopeMetrics()
	require.Equal(t, 1, sm.Len())
	require.Equal(t, 1, sm.At(0).Metrics().Len())

	m := sm.At(0).Metrics().At(0)
	require.Equal(t, pmetric.MetricTypeSum, m.Type())
	require.Equal(t, pmetric.AggregationTemporalityDelta, m.Sum().AggregationTemporality())

	dps := m.Sum().DataPoints()
	require.Equal(t, 2, dps.Len())

	// Points MUST be in ascending chronological order (oldest first)
	assert.Equal(t, pcommon.NewTimestampFromTime(timestamppb.New(timeFromSec(40)).AsTime()), dps.At(0).StartTimestamp())
	assert.Equal(t, pcommon.NewTimestampFromTime(timestamppb.New(timeFromSec(100)).AsTime()), dps.At(0).Timestamp())
	assert.Equal(t, int64(6), dps.At(0).IntValue())

	assert.Equal(t, pcommon.NewTimestampFromTime(timestamppb.New(timeFromSec(100)).AsTime()), dps.At(1).StartTimestamp())
	assert.Equal(t, pcommon.NewTimestampFromTime(timestamppb.New(timeFromSec(160)).AsTime()), dps.At(1).Timestamp())
	assert.Equal(t, int64(4), dps.At(1).IntValue())

	// Strict downstream invariant: Timestamp must be strictly increasing
	assert.Greater(t, dps.At(1).Timestamp(), dps.At(0).Timestamp(), "Timestamp must be strictly increasing")
	assert.GreaterOrEqual(t, dps.At(1).StartTimestamp(), dps.At(0).StartTimestamp(), "StartTimestamp must be non-decreasing")
}

func TestConvertGCPTimeSeriesToMetrics_DistributionPointsOrdering(t *testing.T) {
	mr := newGoogleCloudMonitoringReceiver(&Config{}, zap.NewNop())
	metrics := pmetric.NewMetrics()

	metricDesc := &metric.MetricDescriptor{
		Name:        "custom.googleapis.com/latency",
		Type:        "custom.googleapis.com/latency",
		MetricKind:  metric.MetricDescriptor_DELTA,
		ValueType:   metric.MetricDescriptor_DISTRIBUTION,
		Unit:        "ms",
		Description: "Delta latency distribution",
	}

	dist := &distribution.Distribution{
		Count: 10,
		BucketOptions: &distribution.Distribution_BucketOptions{
			Options: &distribution.Distribution_BucketOptions_ExplicitBuckets{
				ExplicitBuckets: &distribution.Distribution_BucketOptions_Explicit{
					Bounds: []float64{10, 20},
				},
			},
		},
		BucketCounts: []int64{2, 5, 3},
	}

	timeSeries := &monitoringpb.TimeSeries{
		Metric: &metric.Metric{
			Type: "custom.googleapis.com/latency",
		},
		Resource: &monitoredres.MonitoredResource{
			Type: "global",
		},
		MetricKind: metric.MetricDescriptor_DELTA,
		ValueType:  metric.MetricDescriptor_DISTRIBUTION,
		Points: []*monitoringpb.Point{
			{
				Interval: &monitoringpb.TimeInterval{
					StartTime: &timestamppb.Timestamp{Seconds: 100},
					EndTime:   &timestamppb.Timestamp{Seconds: 160},
				},
				Value: &monitoringpb.TypedValue{
					Value: &monitoringpb.TypedValue_DistributionValue{DistributionValue: dist},
				},
			},
			{
				Interval: &monitoringpb.TimeInterval{
					StartTime: &timestamppb.Timestamp{Seconds: 40},
					EndTime:   &timestamppb.Timestamp{Seconds: 100},
				},
				Value: &monitoringpb.TypedValue{
					Value: &monitoringpb.TypedValue_DistributionValue{DistributionValue: dist},
				},
			},
		},
	}

	mr.convertGCPTimeSeriesToMetrics(metrics, metricDesc, timeSeries)

	m := metrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)
	require.Equal(t, pmetric.MetricTypeHistogram, m.Type())
	dps := m.Histogram().DataPoints()
	require.Equal(t, 2, dps.Len())

	// Points MUST be in ascending chronological order
	assert.Equal(t, pcommon.NewTimestampFromTime(timestamppb.New(timeFromSec(40)).AsTime()), dps.At(0).StartTimestamp())
	assert.Equal(t, pcommon.NewTimestampFromTime(timestamppb.New(timeFromSec(100)).AsTime()), dps.At(0).Timestamp())

	assert.Equal(t, pcommon.NewTimestampFromTime(timestamppb.New(timeFromSec(100)).AsTime()), dps.At(1).StartTimestamp())
	assert.Equal(t, pcommon.NewTimestampFromTime(timestamppb.New(timeFromSec(160)).AsTime()), dps.At(1).Timestamp())

	assert.Greater(t, dps.At(1).Timestamp(), dps.At(0).Timestamp(), "Timestamp must be strictly increasing")
	assert.GreaterOrEqual(t, dps.At(1).StartTimestamp(), dps.At(0).StartTimestamp(), "StartTimestamp must be non-decreasing")
}

func TestConvertGCPTimeSeriesToMetrics_GaugePointsOrdering(t *testing.T) {
	mr := newGoogleCloudMonitoringReceiver(&Config{}, zap.NewNop())
	metrics := pmetric.NewMetrics()

	metricDesc := &metric.MetricDescriptor{
		Name:       "compute.googleapis.com/instance/cpu/utilization",
		Type:       "compute.googleapis.com/instance/cpu/utilization",
		MetricKind: metric.MetricDescriptor_GAUGE,
		ValueType:  metric.MetricDescriptor_DOUBLE,
	}

	timeSeries := &monitoringpb.TimeSeries{
		Metric: &metric.Metric{
			Type: "compute.googleapis.com/instance/cpu/utilization",
		},
		Resource: &monitoredres.MonitoredResource{
			Type: "gce_instance",
		},
		MetricKind: metric.MetricDescriptor_GAUGE,
		ValueType:  metric.MetricDescriptor_DOUBLE,
		Points: []*monitoringpb.Point{
			{
				Interval: &monitoringpb.TimeInterval{
					StartTime: &timestamppb.Timestamp{Seconds: 160},
					EndTime:   &timestamppb.Timestamp{Seconds: 160},
				},
				Value: &monitoringpb.TypedValue{
					Value: &monitoringpb.TypedValue_DoubleValue{DoubleValue: 0.8},
				},
			},
			{
				Interval: &monitoringpb.TimeInterval{
					StartTime: &timestamppb.Timestamp{Seconds: 100},
					EndTime:   &timestamppb.Timestamp{Seconds: 100},
				},
				Value: &monitoringpb.TypedValue{
					Value: &monitoringpb.TypedValue_DoubleValue{DoubleValue: 0.5},
				},
			},
		},
	}

	mr.convertGCPTimeSeriesToMetrics(metrics, metricDesc, timeSeries)

	m := metrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)
	require.Equal(t, pmetric.MetricTypeGauge, m.Type())
	dps := m.Gauge().DataPoints()
	require.Equal(t, 2, dps.Len())

	assert.Equal(t, 0.5, dps.At(0).DoubleValue())
	assert.Equal(t, 0.8, dps.At(1).DoubleValue())
	assert.Greater(t, dps.At(1).Timestamp(), dps.At(0).Timestamp(), "Timestamp must be strictly increasing")
}

func TestConvertGCPTimeSeriesToMetrics_CumulativePointsOrdering(t *testing.T) {
	mr := newGoogleCloudMonitoringReceiver(&Config{}, zap.NewNop())
	metrics := pmetric.NewMetrics()

	metricDesc := &metric.MetricDescriptor{
		Name:       "compute.googleapis.com/instance/disk/read_bytes_count",
		Type:       "compute.googleapis.com/instance/disk/read_bytes_count",
		MetricKind: metric.MetricDescriptor_CUMULATIVE,
		ValueType:  metric.MetricDescriptor_INT64,
	}

	timeSeries := &monitoringpb.TimeSeries{
		Metric: &metric.Metric{
			Type: "compute.googleapis.com/instance/disk/read_bytes_count",
		},
		Resource: &monitoredres.MonitoredResource{
			Type: "gce_instance",
		},
		MetricKind: metric.MetricDescriptor_CUMULATIVE,
		ValueType:  metric.MetricDescriptor_INT64,
		Points: []*monitoringpb.Point{
			{
				Interval: &monitoringpb.TimeInterval{
					StartTime: &timestamppb.Timestamp{Seconds: 0},
					EndTime:   &timestamppb.Timestamp{Seconds: 160},
				},
				Value: &monitoringpb.TypedValue{
					Value: &monitoringpb.TypedValue_Int64Value{Int64Value: 2000},
				},
			},
			{
				Interval: &monitoringpb.TimeInterval{
					StartTime: &timestamppb.Timestamp{Seconds: 0},
					EndTime:   &timestamppb.Timestamp{Seconds: 100},
				},
				Value: &monitoringpb.TypedValue{
					Value: &monitoringpb.TypedValue_Int64Value{Int64Value: 1000},
				},
			},
		},
	}

	mr.convertGCPTimeSeriesToMetrics(metrics, metricDesc, timeSeries)

	m := metrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)
	require.Equal(t, pmetric.MetricTypeSum, m.Type())
	require.Equal(t, pmetric.AggregationTemporalityCumulative, m.Sum().AggregationTemporality())
	dps := m.Sum().DataPoints()
	require.Equal(t, 2, dps.Len())

	assert.Equal(t, int64(1000), dps.At(0).IntValue())
	assert.Equal(t, int64(2000), dps.At(1).IntValue())
	assert.Greater(t, dps.At(1).Timestamp(), dps.At(0).Timestamp(), "Timestamp must be strictly increasing")
}

func TestConvertGCPTimeSeriesToMetrics_CumulativeDistribution(t *testing.T) {
	mr := newGoogleCloudMonitoringReceiver(&Config{}, zap.NewNop())
	metrics := pmetric.NewMetrics()

	metricDesc := &metric.MetricDescriptor{
		Name:        "dataflow.googleapis.com/job/streaming_engine/stage_end_to_end_latencies",
		Type:        "dataflow.googleapis.com/job/streaming_engine/stage_end_to_end_latencies",
		MetricKind:  metric.MetricDescriptor_CUMULATIVE,
		ValueType:   metric.MetricDescriptor_DISTRIBUTION,
		Unit:        "ms",
		Description: "Cumulative latency distribution",
	}

	timeSeries := &monitoringpb.TimeSeries{
		Metric: &metric.Metric{
			Type: "dataflow.googleapis.com/job/streaming_engine/stage_end_to_end_latencies",
		},
		Resource: &monitoredres.MonitoredResource{
			Type: "dataflow_job",
		},
		MetricKind: metric.MetricDescriptor_CUMULATIVE,
		ValueType:  metric.MetricDescriptor_DISTRIBUTION,
		Points: []*monitoringpb.Point{
			{
				Interval: &monitoringpb.TimeInterval{
					StartTime: &timestamppb.Timestamp{Seconds: 0},
					EndTime:   &timestamppb.Timestamp{Seconds: 100},
				},
				Value: &monitoringpb.TypedValue{
					Value: &monitoringpb.TypedValue_DistributionValue{
						DistributionValue: &distribution.Distribution{
							Count: 10,
							Mean:  15.5,
							BucketOptions: &distribution.Distribution_BucketOptions{
								Options: &distribution.Distribution_BucketOptions_ExplicitBuckets{
									ExplicitBuckets: &distribution.Distribution_BucketOptions_Explicit{
										Bounds: []float64{10, 20},
									},
								},
							},
							BucketCounts: []int64{2, 5, 3},
						},
					},
				},
			},
		},
	}

	mr.convertGCPTimeSeriesToMetrics(metrics, metricDesc, timeSeries)

	require.Equal(t, 1, metrics.DataPointCount())
	m := metrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)
	require.Equal(t, pmetric.MetricTypeHistogram, m.Type())
	histogram := m.Histogram()
	require.Equal(t, pmetric.AggregationTemporalityCumulative, histogram.AggregationTemporality())
	dps := histogram.DataPoints()
	require.Equal(t, 1, dps.Len())
	assert.Equal(t, uint64(10), dps.At(0).Count())
	assert.Equal(t, 155.0, dps.At(0).Sum())
	assert.Equal(t, []uint64{2, 5, 3}, dps.At(0).BucketCounts().AsRaw())
	assert.Equal(t, []float64{10, 20}, dps.At(0).ExplicitBounds().AsRaw())
}

func TestConvertGCPTimeSeriesToMetrics_UnsupportedCombinationsSkipped(t *testing.T) {
	for _, tt := range []struct {
		name       string
		valueType  metric.MetricDescriptor_ValueType
		metricKind metric.MetricDescriptor_MetricKind
	}{
		{name: "distribution gauge", valueType: metric.MetricDescriptor_DISTRIBUTION, metricKind: metric.MetricDescriptor_GAUGE},
		{name: "distribution unspecified kind", valueType: metric.MetricDescriptor_DISTRIBUTION, metricKind: metric.MetricDescriptor_METRIC_KIND_UNSPECIFIED},
		{name: "scalar unspecified kind", valueType: metric.MetricDescriptor_INT64, metricKind: metric.MetricDescriptor_METRIC_KIND_UNSPECIFIED},
	} {
		t.Run(tt.name, func(t *testing.T) {
			mr := newGoogleCloudMonitoringReceiver(&Config{}, zap.NewNop())
			metrics := pmetric.NewMetrics()

			timeSeries := &monitoringpb.TimeSeries{
				Metric:     &metric.Metric{Type: "custom.googleapis.com/unsupported"},
				Resource:   &monitoredres.MonitoredResource{Type: "global"},
				MetricKind: tt.metricKind,
				ValueType:  tt.valueType,
				Points: []*monitoringpb.Point{{
					Interval: &monitoringpb.TimeInterval{
						StartTime: &timestamppb.Timestamp{Seconds: 100},
						EndTime:   &timestamppb.Timestamp{Seconds: 160},
					},
					Value: &monitoringpb.TypedValue{Value: &monitoringpb.TypedValue_Int64Value{Int64Value: 1}},
				}},
			}

			mr.convertGCPTimeSeriesToMetrics(metrics, &metric.MetricDescriptor{}, timeSeries)

			assert.Equal(t, 0, metrics.ResourceMetrics().Len(),
				"unsupported value type / metric kind combinations must not emit any metrics")
		})
	}
}

func timeFromSec(sec int64) time.Time {
	return time.Unix(sec, 0).UTC()
}
