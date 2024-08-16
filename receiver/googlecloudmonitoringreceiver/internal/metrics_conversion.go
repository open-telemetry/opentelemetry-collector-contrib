// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudmonitoringreceiver/internal"

import (
	"log"

	"go.uber.org/zap"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"cloud.google.com/go/monitoring/apiv3/v2/monitoringpb"
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
		dp.SetStartTimestamp(pcommon.NewTimestampFromTime(point.Interval.StartTime.AsTime()))
		dp.SetTimestamp(pcommon.Timestamp(point.Interval.EndTime.Seconds * 1e9)) // Convert to nanoseconds)

		switch v := point.Value.Value.(type) {
		case *monitoringpb.TypedValue_DoubleValue:
			dp.SetDoubleValue(v.DoubleValue)
		case *monitoringpb.TypedValue_Int64Value:
			dp.SetIntValue(v.Int64Value)
		default:
			log.Printf("Unhandled metric value type: %T", v)
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
		dp.SetStartTimestamp(pcommon.NewTimestampFromTime(point.Interval.StartTime.AsTime()))
		dp.SetTimestamp(pcommon.Timestamp(point.Interval.EndTime.Seconds * 1e9)) // Convert to nanoseconds)

		switch v := point.Value.Value.(type) {
		case *monitoringpb.TypedValue_DoubleValue:
			dp.SetDoubleValue(v.DoubleValue)
		case *monitoringpb.TypedValue_Int64Value:
			dp.SetIntValue(v.Int64Value)
		default:
			log.Printf("Unhandled metric value type: %T", v)
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
		dp.SetStartTimestamp(pcommon.NewTimestampFromTime(point.Interval.StartTime.AsTime()))
		dp.SetTimestamp(pcommon.Timestamp(point.Interval.EndTime.Seconds * 1e9)) // Convert to nanoseconds

		switch v := point.Value.Value.(type) {
		case *monitoringpb.TypedValue_DoubleValue:
			dp.SetDoubleValue(v.DoubleValue)
		case *monitoringpb.TypedValue_Int64Value:
			dp.SetIntValue(v.Int64Value)
		default:
			log.Printf("Unhandled metric value type: %T", v)
		}
	}

	return m
}
