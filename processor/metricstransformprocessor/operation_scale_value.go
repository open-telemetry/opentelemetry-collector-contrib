// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
