// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudmonitoringreceiver/internal"

import (
	"cloud.google.com/go/monitoring/apiv3/v2/monitoringpb"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
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
