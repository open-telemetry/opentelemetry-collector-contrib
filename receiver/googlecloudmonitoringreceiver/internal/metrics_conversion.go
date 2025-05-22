// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudmonitoringreceiver/internal"

import (
	"cloud.google.com/go/monitoring/apiv3/v2/monitoringpb"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
	"google.golang.org/genproto/googleapis/api/metric"
)

type MetricsBuilder struct {
	logger *zap.Logger
}

func NewMetricsBuilder(logger *zap.Logger) *MetricsBuilder {
	return &MetricsBuilder{
		logger: logger,
	}
}

func (mb *MetricsBuilder) ConvertGaugeToMetrics(ts *monitoringpb.TimeSeries, m pmetric.Metric) pmetric.Metric {
	m.SetName(ts.GetMetric().GetType())
	m.SetUnit(ts.GetUnit())
	gauge := m.SetEmptyGauge()

	for _, point := range ts.GetPoints() {
		dp := gauge.DataPoints().AppendEmpty()

		// Directly check and set the StartTimestamp if valid
		if point.Interval.StartTime != nil && point.Interval.StartTime.IsValid() {
			dp.SetStartTimestamp(pcommon.NewTimestampFromTime(point.Interval.StartTime.AsTime()))
		}

		// Check if EndTime is set and valid
		if point.Interval.EndTime != nil && point.Interval.EndTime.IsValid() {
			dp.SetTimestamp(pcommon.NewTimestampFromTime(point.Interval.EndTime.AsTime()))
		} else {
			mb.logger.Warn("EndTime is invalid for metric:", zap.String("Metric", ts.GetMetric().GetType()))
		}

		switch v := point.Value.Value.(type) {
		case *monitoringpb.TypedValue_DoubleValue:
			dp.SetDoubleValue(v.DoubleValue)
		case *monitoringpb.TypedValue_Int64Value:
			dp.SetIntValue(v.Int64Value)
		default:
			mb.logger.Info("Unhandled metric value type:", zap.Reflect("Type", v))
		}
	}

	return m
}

func (mb *MetricsBuilder) ConvertSumToMetrics(ts *monitoringpb.TimeSeries, m pmetric.Metric) pmetric.Metric {
	m.SetName(ts.GetMetric().GetType())
	m.SetUnit(ts.GetUnit())
	sum := m.SetEmptySum()
	sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)

	for _, point := range ts.GetPoints() {
		dp := sum.DataPoints().AppendEmpty()

		// Directly check and set the StartTimestamp if valid
		if point.Interval.StartTime != nil && point.Interval.StartTime.IsValid() {
			dp.SetStartTimestamp(pcommon.NewTimestampFromTime(point.Interval.StartTime.AsTime()))
		}

		// Check if EndTime is set and valid
		if point.Interval.EndTime != nil && point.Interval.EndTime.IsValid() {
			dp.SetTimestamp(pcommon.NewTimestampFromTime(point.Interval.EndTime.AsTime()))
		} else {
			mb.logger.Warn("EndTime is invalid for metric:", zap.String("Metric", ts.GetMetric().GetType()))
		}

		switch v := point.Value.Value.(type) {
		case *monitoringpb.TypedValue_DoubleValue:
			dp.SetDoubleValue(v.DoubleValue)
		case *monitoringpb.TypedValue_Int64Value:
			dp.SetIntValue(v.Int64Value)
		default:
			mb.logger.Info("Unhandled metric value type:", zap.Reflect("Type", v))
		}
	}

	return m
}

func (mb *MetricsBuilder) ConvertDeltaToMetrics(ts *monitoringpb.TimeSeries, m pmetric.Metric) pmetric.Metric {
	m.SetName(ts.GetMetric().GetType())
	m.SetUnit(ts.GetUnit())
	sum := m.SetEmptySum()
	sum.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)

	for _, point := range ts.GetPoints() {
		dp := sum.DataPoints().AppendEmpty()

		// Directly check and set the StartTimestamp if valid
		if point.Interval.StartTime != nil && point.Interval.StartTime.IsValid() {
			dp.SetStartTimestamp(pcommon.NewTimestampFromTime(point.Interval.StartTime.AsTime()))
		}

		// Check if EndTime is set and valid
		if point.Interval.EndTime != nil && point.Interval.EndTime.IsValid() {
			dp.SetTimestamp(pcommon.NewTimestampFromTime(point.Interval.EndTime.AsTime()))
		} else {
			mb.logger.Warn("EndTime is invalid for metric:", zap.String("Metric", ts.GetMetric().GetType()))
		}

		switch v := point.Value.Value.(type) {
		case *monitoringpb.TypedValue_DoubleValue:
			dp.SetDoubleValue(v.DoubleValue)
		case *monitoringpb.TypedValue_Int64Value:
			dp.SetIntValue(v.Int64Value)
		default:
			mb.logger.Info("Unhandled metric value type:", zap.Reflect("Type", v))
		}
	}

	return m
}

func (mb *MetricsBuilder) ConvertDistributionToMetrics(metricKind metric.MetricDescriptor_MetricKind, ts *monitoringpb.TimeSeries, m pmetric.Metric) pmetric.Metric {
	m.SetName(ts.GetMetric().GetType())
	m.SetUnit(ts.GetUnit())
	dist := m.SetEmptyHistogram()

	switch metricKind {
	case metric.MetricDescriptor_CUMULATIVE:
		dist.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	case metric.MetricDescriptor_DELTA:
		dist.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
	default:
		mb.logger.Warn("Unsupported metric kind:", zap.String("Metric", ts.GetMetric().GetType()))
		return m
	}

	for _, point := range ts.GetPoints() {
		dp := dist.DataPoints().AppendEmpty()

		// Set timestamps if valid
		if point.Interval.StartTime != nil && point.Interval.StartTime.IsValid() {
			dp.SetStartTimestamp(pcommon.NewTimestampFromTime(point.Interval.StartTime.AsTime()))
		}

		if point.Interval.EndTime != nil && point.Interval.EndTime.IsValid() {
			dp.SetTimestamp(pcommon.NewTimestampFromTime(point.Interval.EndTime.AsTime()))
		} else {
			mb.logger.Warn("EndTime is invalid for metric:", zap.String("Metric", ts.GetMetric().GetType()))
		}

		distValue := point.Value.GetDistributionValue()
		if distValue == nil {
			mb.logger.Warn("Distribution value is nil for metric:", zap.String("Metric", ts.GetMetric().GetType()))
			continue
		}

		dp.SetCount(uint64(distValue.Count))

		// Handle bucket options based on type
		if bucketOpts := distValue.BucketOptions; bucketOpts != nil {
			if expBuckets := bucketOpts.GetExponentialBuckets(); expBuckets != nil {
				// Initialize all buckets with zeros for exponential buckets
				numBuckets := int(expBuckets.NumFiniteBuckets)
				dp.BucketCounts().EnsureCapacity(numBuckets)
				for i := 0; i < numBuckets; i++ {
					dp.BucketCounts().Append(0)
				}
			} else if linearBuckets := bucketOpts.GetLinearBuckets(); linearBuckets != nil {
				// Initialize all buckets with zeros for linear buckets
				numBuckets := int(linearBuckets.NumFiniteBuckets)
				dp.BucketCounts().EnsureCapacity(numBuckets)
				for i := 0; i < numBuckets; i++ {
					dp.BucketCounts().Append(0)
				}
			} else if explicitBuckets := bucketOpts.GetExplicitBuckets(); explicitBuckets != nil {
				bounds := explicitBuckets.Bounds
				dp.ExplicitBounds().EnsureCapacity(len(bounds))
				for _, bucket := range bounds {
					dp.ExplicitBounds().Append(bucket)
				}
			}
		}

		// Set bucket counts from the distribution data
		if len(distValue.BucketCounts) > 0 {
			// Update existing bucket counts instead of appending
			for i, count := range distValue.BucketCounts {
				if i < dp.BucketCounts().Len() {
					dp.BucketCounts().SetAt(i, uint64(count))
				} else {
					// If we somehow have more counts than buckets, append the extras
					dp.BucketCounts().Append(uint64(count))
				}
			}
		}

		// Set exemplars
		if exemplars := distValue.Exemplars; exemplars != nil {
			for _, exemplar := range exemplars {
				dp.Exemplars().AppendEmpty().SetDoubleValue(float64(exemplar.Value))
			}
		}

		// Set sum and range values
		dp.SetSum(float64(distValue.Mean) * float64(distValue.Count))
		if rangeValue := distValue.GetRange(); rangeValue != nil {
			dp.SetMin(float64(rangeValue.Min))
			dp.SetMax(float64(rangeValue.Max))
		}
	}

	return m
}
