// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package aggregation

import (
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

// HistogramAggregationConfig holds configuration for histogram aggregation
type HistogramAggregationConfig struct {
	StaleDataThreshold time.Duration
	Logger             *zap.Logger
}

// AggregateHistogram aggregates histogram metrics - merge buckets
func AggregateHistogram(hist pmetric.Histogram, agg Aggregator, config HistogramAggregationConfig) error {
	dps := hist.DataPoints()
	temporality := hist.AggregationTemporality()

	// Debug logging to understand what's being aggregated
	config.Logger.Debug("Aggregating histogram",
		zap.Int("data_points", dps.Len()),
		zap.String("temporality", temporality.String()),
	)

	for i := 0; i < dps.Len(); i++ {
		dp := dps.At(i)

		// Log each data point being aggregated
		attrs := make(map[string]string)
		dp.Attributes().Range(func(k string, v pcommon.Value) bool {
			attrs[k] = v.AsString()
			return true
		})

		config.Logger.Debug("Merging histogram data point",
			zap.Uint64("count", dp.Count()),
			zap.Float64("sum", dp.Sum()),
			zap.Any("attributes", attrs),
			zap.String("temporality", temporality.String()),
		)

		// Use the proven logic for histogram merging with gap detection
		agg.MergeHistogramWithTemporalityAndGapDetection(dp, temporality, config.StaleDataThreshold)
	}
	return nil
}