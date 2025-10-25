// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package aggregation

import (
	"go.opentelemetry.io/collector/pdata/pmetric"
)

// AggregateSummary aggregates summary metrics
func AggregateSummary(summary pmetric.Summary, agg Aggregator) error {
	dps := summary.DataPoints()
	for i := 0; i < dps.Len(); i++ {
		dp := dps.At(i)
		agg.UpdateSum(dp.Sum())
		agg.UpdateCount(dp.Count())
	}
	return nil
}