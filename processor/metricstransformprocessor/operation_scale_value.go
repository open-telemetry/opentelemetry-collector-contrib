// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metricstransformprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstransformprocessor"

import (
	"math"

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
	case pmetric.MetricTypeExponentialHistogram:
		scaleExpHistogramOp(metric, op, f)
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
	dps := metric.Histogram().DataPoints()

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

		scaleExemplars(dp.Exemplars(), &op)
	}
}

func scaleExpHistogramOp(metric pmetric.Metric, op internalOperation, f internalFilter) {
	dps := metric.ExponentialHistogram().DataPoints()
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

		dp.SetZeroThreshold(dp.ZeroThreshold() * op.configOperation.Scale)

		// For the buckets, we only need to change the offset.
		// The bucket counts and the scale remain the same.
		if len(dp.Positive().BucketCounts().AsRaw()) != 0 {
			dp.Positive().SetOffset(updateOffset(dp.Scale(), dp.Positive().Offset(), &op))
		}

		if len(dp.Negative().BucketCounts().AsRaw()) != 0 {
			dp.Negative().SetOffset(updateOffset(dp.Scale(), dp.Negative().Offset(), &op))
		}

		scaleExemplars(dp.Exemplars(), &op)
	}
}

func updateOffset(scale int32, offset int32, op *internalOperation) int32 {
	// Take the middle of the first bucket.
	base := math.Pow(2, math.Pow(2, float64(-scale)))
	value := (math.Pow(base, float64(offset)) + math.Pow(base, float64(offset+1))) / 2

	// Scale it according to the config.
	scaledValue := value * op.configOperation.Scale

	// Find the new offset by mapping the scaled value.
	return mapToIndex(scaledValue, int(scale))
}

// mapToIndex returns the index that corresponds to the given value on the scale.
// See https://opentelemetry.io/docs/specs/otel/metrics/data-model/#all-scales-use-the-logarithm-function.
func mapToIndex(value float64, scale int) int32 {
	scaleFactor := math.Ldexp(math.Log2E, scale)
	return int32(math.Ceil(math.Log(value)*scaleFactor) - 1)
}

func scaleExemplars(exemplars pmetric.ExemplarSlice, op *internalOperation) {
	for e, ei := exemplars, 0; ei < e.Len(); ei++ {
		exemplar := e.At(ei)
		switch exemplar.ValueType() {
		case pmetric.ExemplarValueTypeInt:
			exemplar.SetIntValue(int64(float64(exemplar.IntValue()) * op.configOperation.Scale))
		case pmetric.ExemplarValueTypeDouble:
			exemplar.SetDoubleValue(exemplar.DoubleValue() * op.configOperation.Scale)
		}
	}
}
