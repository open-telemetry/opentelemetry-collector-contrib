// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"testing"
	"time"

	"cloud.google.com/go/monitoring/apiv3/v2/monitoringpb"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	"google.golang.org/genproto/googleapis/api/metric"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestNewMetricsBuilder(t *testing.T) {
	// Test cases
	tests := []struct {
		name   string
		logger *zap.Logger
	}{
		{
			name:   "with normal logger",
			logger: zap.NewNop(),
		},
		{
			name:   "with nil logger",
			logger: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mb := NewMetricsBuilder(tt.logger)
			assert.NotNil(t, mb, "MetricsBuilder should not be nil")
			if tt.logger != nil {
				assert.Equal(t, tt.logger, mb.logger)
			}
		})
	}
}

func TestConvertGaugeToMetrics(t *testing.T) {
	logger := zap.NewNop()
	mb := NewMetricsBuilder(logger)

	// Create test timestamps
	startTime := time.Now().Add(-1 * time.Hour)
	endTime := time.Now()

	tests := []struct {
		name     string
		input    *monitoringpb.TimeSeries
		expected func(pmetric.Metric)
	}{
		{
			name: "double value gauge",
			input: &monitoringpb.TimeSeries{
				Metric: &metric.Metric{
					Type: "custom.googleapis.com/metric/double",
				},
				Unit: "1",
				Points: []*monitoringpb.Point{
					{
						Interval: &monitoringpb.TimeInterval{
							StartTime: timestamppb.New(startTime),
							EndTime:   timestamppb.New(endTime),
						},
						Value: &monitoringpb.TypedValue{
							Value: &monitoringpb.TypedValue_DoubleValue{
								DoubleValue: 123.45,
							},
						},
					},
				},
			},
			expected: func(m pmetric.Metric) {
				assert.Equal(t, "custom.googleapis.com/metric/double", m.Name())
				assert.Equal(t, "1", m.Unit())
				assert.Equal(t, pmetric.MetricTypeGauge, m.Type())

				gauge := m.Gauge()
				assert.Equal(t, 1, gauge.DataPoints().Len())

				dp := gauge.DataPoints().At(0)
				assert.Equal(t, 123.45, dp.DoubleValue())
				assert.Equal(t, pcommon.NewTimestampFromTime(startTime), dp.StartTimestamp())
				assert.Equal(t, pcommon.NewTimestampFromTime(endTime), dp.Timestamp())
			},
		},
		{
			name: "int64 value gauge",
			input: &monitoringpb.TimeSeries{
				Metric: &metric.Metric{
					Type: "custom.googleapis.com/metric/int",
				},
				Unit: "1",
				Points: []*monitoringpb.Point{
					{
						Interval: &monitoringpb.TimeInterval{
							StartTime: timestamppb.New(startTime),
							EndTime:   timestamppb.New(endTime),
						},
						Value: &monitoringpb.TypedValue{
							Value: &monitoringpb.TypedValue_Int64Value{
								Int64Value: 123,
							},
						},
					},
				},
			},
			expected: func(m pmetric.Metric) {
				assert.Equal(t, "custom.googleapis.com/metric/int", m.Name())
				assert.Equal(t, "1", m.Unit())
				assert.Equal(t, pmetric.MetricTypeGauge, m.Type())

				gauge := m.Gauge()
				assert.Equal(t, 1, gauge.DataPoints().Len())

				dp := gauge.DataPoints().At(0)
				assert.Equal(t, int64(123), dp.IntValue())
				assert.Equal(t, pcommon.NewTimestampFromTime(startTime), dp.StartTimestamp())
				assert.Equal(t, pcommon.NewTimestampFromTime(endTime), dp.Timestamp())
			},
		},
		{
			name: "missing end time",
			input: &monitoringpb.TimeSeries{
				Metric: &metric.Metric{
					Type: "custom.googleapis.com/metric/double",
				},
				Unit: "1",
				Points: []*monitoringpb.Point{
					{
						Interval: &monitoringpb.TimeInterval{
							StartTime: timestamppb.New(startTime),
						},
						Value: &monitoringpb.TypedValue{
							Value: &monitoringpb.TypedValue_DoubleValue{
								DoubleValue: 123.45,
							},
						},
					},
				},
			},
			expected: func(m pmetric.Metric) {
				assert.Equal(t, "custom.googleapis.com/metric/double", m.Name())
				assert.Equal(t, "1", m.Unit())
				assert.Equal(t, pmetric.MetricTypeGauge, m.Type())

				gauge := m.Gauge()
				assert.Equal(t, 1, gauge.DataPoints().Len())

				dp := gauge.DataPoints().At(0)
				assert.Equal(t, 123.45, dp.DoubleValue())
				assert.Equal(t, pcommon.NewTimestampFromTime(startTime), dp.StartTimestamp())
			},
		},
		{
			name: "unhandled value type",
			input: &monitoringpb.TimeSeries{
				Metric: &metric.Metric{
					Type: "custom.googleapis.com/metric/string",
				},
				Unit: "1",
				Points: []*monitoringpb.Point{
					{
						Interval: &monitoringpb.TimeInterval{
							StartTime: timestamppb.New(startTime),
							EndTime:   timestamppb.New(endTime),
						},
						Value: &monitoringpb.TypedValue{
							Value: &monitoringpb.TypedValue_StringValue{
								StringValue: "test",
							},
						},
					},
				},
			},
			expected: func(m pmetric.Metric) {
				assert.Equal(t, "custom.googleapis.com/metric/string", m.Name())
				assert.Equal(t, "1", m.Unit())
				assert.Equal(t, pmetric.MetricTypeGauge, m.Type())

				gauge := m.Gauge()
				assert.Equal(t, 1, gauge.DataPoints().Len())
				// No value should be set for unhandled type
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metric := pmetric.NewMetric()
			result := mb.ConvertGaugeToMetrics(tt.input, metric)
			tt.expected(result)
		})
	}
}

func TestConvertSumToMetrics(t *testing.T) {
	// Create an observable logger to capture log messages
	core, observedLogs := observer.New(zap.InfoLevel)
	logger := zap.New(core)
	mb := NewMetricsBuilder(logger)

	startTime := time.Now().Add(-1 * time.Hour)
	endTime := time.Now()

	tests := []struct {
		name           string
		input          *monitoringpb.TimeSeries
		expectedLogs   int
		validateMetric func(*testing.T, pmetric.Metric)
	}{
		{
			name: "double value sum",
			input: &monitoringpb.TimeSeries{
				Metric: &metric.Metric{
					Type: "custom.googleapis.com/metric/double",
				},
				Unit: "bytes",
				Points: []*monitoringpb.Point{
					{
						Interval: &monitoringpb.TimeInterval{
							StartTime: timestamppb.New(startTime),
							EndTime:   timestamppb.New(endTime),
						},
						Value: &monitoringpb.TypedValue{
							Value: &monitoringpb.TypedValue_DoubleValue{
								DoubleValue: 123.45,
							},
						},
					},
				},
			},
			expectedLogs: 0,
			validateMetric: func(t *testing.T, m pmetric.Metric) {
				assert.Equal(t, "custom.googleapis.com/metric/double", m.Name())
				assert.Equal(t, "bytes", m.Unit())
				sum := m.Sum()
				assert.Equal(t, pmetric.AggregationTemporalityCumulative, sum.AggregationTemporality())
				assert.Equal(t, 1, sum.DataPoints().Len())
				dp := sum.DataPoints().At(0)
				assert.Equal(t, 123.45, dp.DoubleValue())
				assert.Equal(t, pcommon.NewTimestampFromTime(startTime), dp.StartTimestamp())
				assert.Equal(t, pcommon.NewTimestampFromTime(endTime), dp.Timestamp())
			},
		},
		{
			name: "int64 value sum",
			input: &monitoringpb.TimeSeries{
				Metric: &metric.Metric{
					Type: "custom.googleapis.com/metric/int",
				},
				Unit: "1",
				Points: []*monitoringpb.Point{
					{
						Interval: &monitoringpb.TimeInterval{
							StartTime: timestamppb.New(startTime),
							EndTime:   timestamppb.New(endTime),
						},
						Value: &monitoringpb.TypedValue{
							Value: &monitoringpb.TypedValue_Int64Value{
								Int64Value: 123,
							},
						},
					},
				},
			},
			expectedLogs: 0,
			validateMetric: func(t *testing.T, m pmetric.Metric) {
				sum := m.Sum()
				assert.Equal(t, pmetric.AggregationTemporalityCumulative, sum.AggregationTemporality())
				dp := sum.DataPoints().At(0)
				assert.Equal(t, int64(123), dp.IntValue())
			},
		},
		{
			name: "unhandled value type",
			input: &monitoringpb.TimeSeries{
				Metric: &metric.Metric{
					Type: "custom.googleapis.com/metric/string",
				},
				Unit: "1",
				Points: []*monitoringpb.Point{
					{
						Interval: &monitoringpb.TimeInterval{
							StartTime: timestamppb.New(startTime),
							EndTime:   timestamppb.New(endTime),
						},
						Value: &monitoringpb.TypedValue{
							Value: &monitoringpb.TypedValue_StringValue{
								StringValue: "test",
							},
						},
					},
				},
			},
			expectedLogs: 1,
			validateMetric: func(t *testing.T, m pmetric.Metric) {
				sum := m.Sum()
				assert.Equal(t, 1, sum.DataPoints().Len())
				// Value should not be set for unhandled type
			},
		},
		{
			name: "missing end time",
			input: &monitoringpb.TimeSeries{
				Metric: &metric.Metric{
					Type: "custom.googleapis.com/metric/double",
				},
				Unit: "1",
				Points: []*monitoringpb.Point{
					{
						Interval: &monitoringpb.TimeInterval{
							StartTime: timestamppb.New(startTime),
						},
						Value: &monitoringpb.TypedValue{
							Value: &monitoringpb.TypedValue_DoubleValue{
								DoubleValue: 123.45,
							},
						},
					},
				},
			},
			expectedLogs: 1, // Warning log for missing end time
			validateMetric: func(t *testing.T, m pmetric.Metric) {
				sum := m.Sum()
				dp := sum.DataPoints().At(0)
				assert.Equal(t, pcommon.NewTimestampFromTime(startTime), dp.StartTimestamp())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			observedLogs.TakeAll() // Clear previous logs
			metric := pmetric.NewMetric()
			result := mb.ConvertSumToMetrics(tt.input, metric)

			tt.validateMetric(t, result)

			logs := observedLogs.TakeAll()
			assert.Len(t, logs, tt.expectedLogs)
		})
	}
}

func TestConvertDeltaToMetrics(t *testing.T) {
	core, observedLogs := observer.New(zap.InfoLevel)
	logger := zap.New(core)
	mb := NewMetricsBuilder(logger)

	startTime := time.Now().Add(-1 * time.Hour)
	endTime := time.Now()

	tests := []struct {
		name           string
		input          *monitoringpb.TimeSeries
		expectedLogs   int
		validateMetric func(*testing.T, pmetric.Metric)
	}{
		{
			name: "double value delta",
			input: &monitoringpb.TimeSeries{
				Metric: &metric.Metric{
					Type: "custom.googleapis.com/metric/double",
				},
				Unit: "bytes",
				Points: []*monitoringpb.Point{
					{
						Interval: &monitoringpb.TimeInterval{
							StartTime: timestamppb.New(startTime),
							EndTime:   timestamppb.New(endTime),
						},
						Value: &monitoringpb.TypedValue{
							Value: &monitoringpb.TypedValue_DoubleValue{
								DoubleValue: 123.45,
							},
						},
					},
				},
			},
			expectedLogs: 0,
			validateMetric: func(t *testing.T, m pmetric.Metric) {
				assert.Equal(t, "custom.googleapis.com/metric/double", m.Name())
				assert.Equal(t, "bytes", m.Unit())
				sum := m.Sum()
				assert.Equal(t, pmetric.AggregationTemporalityDelta, sum.AggregationTemporality())
				assert.Equal(t, 1, sum.DataPoints().Len())
				dp := sum.DataPoints().At(0)
				assert.Equal(t, 123.45, dp.DoubleValue())
			},
		},
		{
			name: "invalid end time",
			input: &monitoringpb.TimeSeries{
				Metric: &metric.Metric{
					Type: "custom.googleapis.com/metric/double",
				},
				Unit: "bytes",
				Points: []*monitoringpb.Point{
					{
						Interval: &monitoringpb.TimeInterval{
							StartTime: timestamppb.New(startTime),
							EndTime:   nil, // Invalid end time
						},
						Value: &monitoringpb.TypedValue{
							Value: &monitoringpb.TypedValue_DoubleValue{
								DoubleValue: 123.45,
							},
						},
					},
				},
			},
			expectedLogs: 1, // Expect one warning log for invalid end time
			validateMetric: func(t *testing.T, m pmetric.Metric) {
				assert.Equal(t, "custom.googleapis.com/metric/double", m.Name())
				assert.Equal(t, "bytes", m.Unit())
				sum := m.Sum()
				assert.Equal(t, pmetric.AggregationTemporalityDelta, sum.AggregationTemporality())
				assert.Equal(t, 1, sum.DataPoints().Len())
				dp := sum.DataPoints().At(0)
				assert.Equal(t, 123.45, dp.DoubleValue())
				// Timestamp should not be set due to invalid end time
				assert.Equal(t, pcommon.Timestamp(0), dp.Timestamp())
			},
		},
		{
			name: "int64 value delta",
			input: &monitoringpb.TimeSeries{
				Metric: &metric.Metric{
					Type: "custom.googleapis.com/metric/int",
				},
				Unit: "count",
				Points: []*monitoringpb.Point{
					{
						Interval: &monitoringpb.TimeInterval{
							StartTime: timestamppb.New(startTime),
							EndTime:   timestamppb.New(endTime),
						},
						Value: &monitoringpb.TypedValue{
							Value: &monitoringpb.TypedValue_Int64Value{
								Int64Value: 456,
							},
						},
					},
				},
			},
			expectedLogs: 0,
			validateMetric: func(t *testing.T, m pmetric.Metric) {
				assert.Equal(t, "custom.googleapis.com/metric/int", m.Name())
				assert.Equal(t, "count", m.Unit())
				sum := m.Sum()
				assert.Equal(t, pmetric.AggregationTemporalityDelta, sum.AggregationTemporality())
				assert.Equal(t, 1, sum.DataPoints().Len())
				dp := sum.DataPoints().At(0)
				assert.Equal(t, int64(456), dp.IntValue())
				assert.Equal(t, pcommon.NewTimestampFromTime(endTime), dp.Timestamp())
			},
		},
		{
			name: "unhandled value type",
			input: &monitoringpb.TimeSeries{
				Metric: &metric.Metric{
					Type: "custom.googleapis.com/metric/bool",
				},
				Unit: "1",
				Points: []*monitoringpb.Point{
					{
						Interval: &monitoringpb.TimeInterval{
							StartTime: timestamppb.New(startTime),
							EndTime:   timestamppb.New(endTime),
						},
						Value: &monitoringpb.TypedValue{
							Value: &monitoringpb.TypedValue_BoolValue{
								BoolValue: true,
							},
						},
					},
				},
			},
			expectedLogs: 1,
			validateMetric: func(t *testing.T, m pmetric.Metric) {
				sum := m.Sum()
				assert.Equal(t, pmetric.AggregationTemporalityDelta, sum.AggregationTemporality())
				assert.Equal(t, 1, sum.DataPoints().Len())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			observedLogs.TakeAll() // Clear previous logs
			metric := pmetric.NewMetric()
			result := mb.ConvertDeltaToMetrics(tt.input, metric)

			tt.validateMetric(t, result)

			logs := observedLogs.TakeAll()
			assert.Len(t, logs, tt.expectedLogs)
		})
	}
}
