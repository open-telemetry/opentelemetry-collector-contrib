package metrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/metrics"

import (
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func convertSummarySumValToSum(stringAggTemp string, monotonic bool) (common.ExprFunc, error) {
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
		if metric.DataType() != pmetric.MetricDataTypeSummary {
			return nil
		}

		sumMetric := mtc.GetMetrics().AppendEmpty()
		sumMetric.SetDescription(metric.Description())
		sumMetric.SetName(metric.Name() + "_sum")
		sumMetric.SetDataType(pmetric.MetricDataTypeSum)
		sumMetric.Sum().SetAggregationTemporality(aggTemp)
		sumMetric.Sum().SetIsMonotonic(monotonic)

		sumDps := sumMetric.Sum().DataPoints()
		dps := metric.Summary().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			dp := dps.At(i)
			sumDp := sumDps.AppendEmpty()
			dp.Attributes().CopyTo(sumDp.Attributes())
			sumDp.SetDoubleVal(dp.Sum())
		}
		return nil
	}, nil
}

func convertSummaryCountValToSum(stringAggTemp string, monotonic bool) (common.ExprFunc, error) {
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
		if metric.DataType() != pmetric.MetricDataTypeSummary {
			return nil
		}

		sumMetric := mtc.GetMetrics().AppendEmpty()
		sumMetric.SetDescription(metric.Description())
		sumMetric.SetName(metric.Name() + "_count")
		sumMetric.SetDataType(pmetric.MetricDataTypeSum)
		sumMetric.Sum().SetAggregationTemporality(aggTemp)
		sumMetric.Sum().SetIsMonotonic(monotonic)

		sumDps := sumMetric.Sum().DataPoints()
		dps := metric.Summary().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			dp := dps.At(i)
			sumDp := sumDps.AppendEmpty()
			dp.Attributes().CopyTo(sumDp.Attributes())
			sumDp.SetIntVal(int64(dp.Count()))
		}
		return nil
	}, nil
}
