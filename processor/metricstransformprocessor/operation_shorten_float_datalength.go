package metricstransformprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstransformprocessor"

import (
	"math"
	
	"go.opentelemetry.io/collector/pdata/pmetric"
)

// shortenFloatDataLengthOp shortens a float digits after the point by limiting the max decimal places.
// This function is applicable to sum and gauge metrics only.
func shortenFloatDataLengthOp(metric pmetric.Metric, op internalOperation) {
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

		// Generate the string representation of the metric value and add it as an attribute
		var roundedValue float64
		switch dp.ValueType() {
		case pmetric.NumberDataPointValueTypeDouble:
			roundedValue = roundTo(dp.DoubleValue(), op.configOperation.DecimalPlaces)
			dp.SetDoubleValue(roundedValue)
		}
		
	}
}

func roundTo(val float64, decimalPlaces int) float64 {
	scale := math.Pow(10, float64(decimalPlaces))
	return math.Round(val*scale) / scale
}
