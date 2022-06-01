package metrics

import (
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func convertGaugeToSum(stringAggTemp string, monotonic bool) (common.ExprFunc, error) {
	var aggTemp pmetric.MetricAggregationTemporality
	switch stringAggTemp {
	case "delta":
		aggTemp = pmetric.MetricAggregationTemporalityDelta
	case "cumulative":
		aggTemp = pmetric.MetricAggregationTemporalityCumulative
	default:
		return nil, fmt.Errorf("unknown aggregation temporality: %s", stringAggTemp)
	}

	return func(ctx common.TransformContext) interface{} {
		mtc, ok := ctx.(metricTransformContext)
		if !ok {
			return nil
		}

		metric := mtc.GetMetric()
		if metric.DataType() != pmetric.MetricDataTypeGauge {
			return nil
		}

		dps := metric.Gauge().DataPoints()

		metric.SetDataType(pmetric.MetricDataTypeSum)
		metric.Sum().SetAggregationTemporality(aggTemp)
		metric.Sum().SetIsMonotonic(monotonic)

		// Setting the data type removed all the data points, so we must copy them back to the metric.
		dps.CopyTo(metric.Sum().DataPoints())

		return nil
	}, nil
}
