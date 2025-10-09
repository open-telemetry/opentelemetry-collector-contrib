// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package aggregation

import (
	"fmt"

	"go.opentelemetry.io/collector/pdata/pmetric"
)

// AggregateExponentialHistogram aggregates exponential histogram metrics
func AggregateExponentialHistogram(metric pmetric.Metric, hist pmetric.ExponentialHistogram, agg Aggregator) error {
	dps := hist.DataPoints()

	for i := 0; i < dps.Len(); i++ {
		dp := dps.At(i)
		// Pass metric so accumulator can capture name/description/unit
		if err := agg.MergeExponentialHistogram(metric, dp); err != nil {
			return fmt.Errorf("failed to merge exponential histogram: %w", err)
		}
	}
	return nil
}