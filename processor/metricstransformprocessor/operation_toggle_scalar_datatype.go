// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metricstransformprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstransformprocessor"

import (
	"go.opentelemetry.io/collector/pdata/pmetric"
)

// toggleScalarDataTypeOp translates the numeric value type to the opposite type, int -> double and double -> int.
// Applicable to sum and gauge metrics only.
func toggleScalarDataTypeOp(metric pmetric.Metric, f internalFilter) {
	var dps pmetric.NumberDataPointSlice
	switch metric.Type() {
	case pmetric.MetricTypeGauge:
		dps = metric.Gauge().DataPoints()
	case pmetric.MetricTypeSum:
		dps = metric.Sum().DataPoints()
	default:
		return
	}

	for i := 0; i < dps.Len(); i++ {
		dp := dps.At(i)
		if !f.matchAttrs(dp.Attributes()) {
			continue
		}

		switch dp.ValueType() {
		case pmetric.NumberDataPointValueTypeInt:
			dp.SetDoubleValue(float64(dp.IntValue()))
		case pmetric.NumberDataPointValueTypeDouble:
			dp.SetIntValue(int64(dp.DoubleValue()))
		}
	}
}
