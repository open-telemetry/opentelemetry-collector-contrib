// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package aggregation

import (
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

// GaugeAggregationConfig holds configuration for gauge aggregation
type GaugeAggregationConfig struct {
	Logger *zap.Logger
}

// AggregateGauge aggregates gauge metrics - keep last value
func AggregateGauge(gauge pmetric.Gauge, agg Aggregator, config GaugeAggregationConfig) error {
	dps := gauge.DataPoints()
	for i := 0; i < dps.Len(); i++ {
		dp := dps.At(i)

		// Get the value based on the data point type
		var value float64
		switch dp.ValueType() {
		case pmetric.NumberDataPointValueTypeInt:
			value = float64(dp.IntValue())
		case pmetric.NumberDataPointValueTypeDouble:
			value = dp.DoubleValue()
		default:
			continue // Skip if neither int nor double
		}

		agg.UpdateLast(value, dp.Timestamp())

		config.Logger.Debug("Aggregating gauge",
			zap.Float64("value", value),
			zap.String("timestamp", dp.Timestamp().AsTime().String()),
		)
	}
	return nil
}