// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metricstransformprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstransformprocessor"

import (
	"go.opentelemetry.io/collector/pdata/pmetric"
)

// scaleValueOp scales the numeric metric value of sum and gauge metrics.
// For histograms it scales the value of the sum and the explicit bounds.
func scaleValueOp(metric pmetric.Metric, op internalOperation, f internalFilter) {
	var dps pmetric.NumberDataPointSlice
	switch metric.Type() {
	case pmetric.MetricTypeGauge:
		dps = metric.Gauge().DataPoints()
	case pmetric.MetricTypeSum:
		dps = metric.Sum().DataPoints()
	case pmetric.MetricTypeHistogram:
		scaleHistogramOp(metric, op, f)
		return
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
			dp.SetIntValue(int64(float64(dp.IntValue()) * op.configOperation.Scale))
		case pmetric.NumberDataPointValueTypeDouble:
			dp.SetDoubleValue(dp.DoubleValue() * op.configOperation.Scale)
		}
	}
}

func scaleHistogramOp(metric pmetric.Metric, op internalOperation, f internalFilter) {
	var dps = metric.Histogram().DataPoints()

	for i := 0; i < dps.Len(); i++ {
		dp := dps.At(i)
		if !f.matchAttrs(dp.Attributes()) {
			continue
		}

		if dp.HasSum() {
			dp.SetSum(dp.Sum() * op.configOperation.Scale)
		}
		if dp.HasMin() {
			dp.SetMin(dp.Min() * op.configOperation.Scale)
		}
		if dp.HasMax() {
			dp.SetMax(dp.Max() * op.configOperation.Scale)
		}

		for bounds, bi := dp.ExplicitBounds(), 0; bi < bounds.Len(); bi++ {
			bounds.SetAt(bi, bounds.At(bi)*op.configOperation.Scale)
		}

		for exemplars, ei := dp.Exemplars(), 0; ei < exemplars.Len(); ei++ {
			exemplar := exemplars.At(ei)
			switch exemplar.ValueType() {
			case pmetric.ExemplarValueTypeInt:
				exemplar.SetIntValue(int64(float64(exemplar.IntValue()) * op.configOperation.Scale))
			case pmetric.ExemplarValueTypeDouble:
				exemplar.SetDoubleValue(exemplar.DoubleValue() * op.configOperation.Scale)
			}
		}
	}
}
