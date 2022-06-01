package metrics

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func convertSumToGauge() (common.ExprFunc, error) {
	return func(ctx common.TransformContext) interface{} {
		mtc, ok := ctx.(metricTransformContext)
		if !ok {
			return nil
		}

		metric := mtc.GetMetric()
		if metric.DataType() != pmetric.MetricDataTypeSum {
			return nil
		}

		dps := metric.Sum().DataPoints()

		metric.SetDataType(pmetric.MetricDataTypeGauge)
		// Setting the data type removed all the data points, so we must copy them back to the metric.
		dps.CopyTo(metric.Gauge().DataPoints())

		return nil
	}, nil
}
