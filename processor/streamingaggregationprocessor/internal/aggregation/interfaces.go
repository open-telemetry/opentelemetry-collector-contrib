// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package aggregation

import (
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

// Aggregator interface defines the methods needed for aggregation
// This allows internal packages to work with the main Aggregator without circular dependencies
type Aggregator interface {
	// Basic aggregation methods
	UpdateSum(value float64)
	UpdateLast(value float64, timestamp pcommon.Timestamp)
	UpdateUpDownCounter(value float64, timestamp pcommon.Timestamp)
	UpdateCount(count uint64)
	UpdateMean(value float64)

	// Counter-specific methods
	ComputeDeltaFromCumulativeWithGapDetection(cumulativeValue float64, timestamp pcommon.Timestamp, staleThreshold time.Duration) (float64, bool)
	ProcessDeltaCounter(value float64) error

	// Histogram methods
	MergeHistogramWithTemporalityAndGapDetection(dp pmetric.HistogramDataPoint, temporality pmetric.AggregationTemporality, staleThreshold time.Duration)

	// Exponential histogram methods
	MergeExponentialHistogram(metric pmetric.Metric, dp pmetric.ExponentialHistogramDataPoint) error
}