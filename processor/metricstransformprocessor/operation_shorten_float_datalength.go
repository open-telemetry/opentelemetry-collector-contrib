package metricstransformprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstransformprocessor"

import (
	"fmt"
	"strconv"
	"strings"
	
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
		var valueStr string
		switch dp.ValueType() {
		case pmetric.NumberDataPointValueTypeDouble:
			valueStr = fmt.Sprintf("%f", dp.DoubleValue())
			valueStr = truncateFloatString(valueStr, op.configOperation.DecimalPlaces)
			valueFloat, err := strconv.ParseFloat(valueStr, 64) // Convert string to float64
			if err != nil {
				fmt.Println("Error converting string to float:", err)
				return
			}
			dp.SetDoubleValue(valueFloat)
		}
		
	}
}

func truncateFloatString(s string, decimalPlaces int) string {
	// Find the index of the decimal point.
	pointIndex := strings.Index(s, ".")
	if pointIndex == -1 {
		// No decimal point found; return the original string.
		return s
	}

	// Calculate the total length by adding the index of the point and the desired decimal places + 1 (for the point itself).
	totalLength := pointIndex + decimalPlaces + 1
	if totalLength >= len(s) {
		// If the total length is beyond the string's length, return the original string.
		return s
	}
	// Otherwise, return the substring up to the calculated total length.
	return s[:totalLength]
}
