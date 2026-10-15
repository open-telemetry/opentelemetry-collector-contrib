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

func timeFromSec(sec int64) time.Time {
	return time.Unix(sec, 0).UTC()
}
