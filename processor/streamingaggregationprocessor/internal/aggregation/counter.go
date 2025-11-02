// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package aggregation

import (
	"time"

	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

// CounterAggregationConfig holds configuration for counter aggregation
type CounterAggregationConfig struct {
	StaleDataThreshold time.Duration
	Logger             *zap.Logger
}

// AggregateSum aggregates sum/counter metrics - sum values
func AggregateSum(sum pmetric.Sum, agg Aggregator, config CounterAggregationConfig, forceExportFunc func()) error {
	dps := sum.DataPoints()
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

		// Handle based on monotonic property and temporality
		if !sum.IsMonotonic() {
			// UpDownCounter - track the net change within the window
			// For UpDownCounters, we want to track the difference between the first
			// and last value seen in the window, not accumulate all changes
			if sum.AggregationTemporality() == pmetric.AggregationTemporalityCumulative {
				// For cumulative UpDownCounters, track first and last values
				agg.UpdateUpDownCounter(value, dp.Timestamp())
			} else {
				// For delta UpDownCounters, sum the deltas
				agg.UpdateSum(value)
			}

			config.Logger.Debug("Aggregating UpDownCounter",
				zap.Float64("value", value),
				zap.String("temporality", sum.AggregationTemporality().String()),
				zap.Bool("is_monotonic", sum.IsMonotonic()),
			)
		} else {
			// Regular counter (monotonic) - accumulate deltas
			if sum.AggregationTemporality() == pmetric.AggregationTemporalityCumulative {
				// For cumulative, compute the delta from the previous value with gap detection
				// ComputeDeltaFromCumulativeWithGapDetection already updates counterWindowDeltaSum internally
				deltaValue, wasReset := agg.ComputeDeltaFromCumulativeWithGapDetection(value, dp.Timestamp(), config.StaleDataThreshold)
				// Don't call UpdateSum here - ComputeDeltaFromCumulativeWithGapDetection handles the accumulation

				// If cumulative state was reset due to gap, trigger immediate export
				if wasReset && forceExportFunc != nil {
					go forceExportFunc() // Export active window immediately in background
				}

				config.Logger.Debug("Aggregating cumulative counter",
					zap.Float64("cumulative_value", value),
					zap.Float64("delta_value", deltaValue),
					zap.String("temporality", sum.AggregationTemporality().String()),
					zap.Bool("is_monotonic", sum.IsMonotonic()),
				)
			} else {
				// For delta temporality, use the value directly
				// For delta counters, we need to accumulate in the counter-specific fields
				// We treat delta values as increments to the total sum
				if err := agg.ProcessDeltaCounter(value); err != nil {
					config.Logger.Error("Failed to process delta counter", zap.Error(err))
					continue
				}

				config.Logger.Debug("Aggregating delta counter",
					zap.Float64("value", value),
					zap.String("temporality", sum.AggregationTemporality().String()),
					zap.Bool("is_monotonic", sum.IsMonotonic()),
				)
			}
		}
	}
	return nil
}