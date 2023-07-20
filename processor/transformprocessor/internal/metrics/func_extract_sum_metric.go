// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/metrics"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottldatapoint"
)

type extractSumMetricArguments struct {
	StringAggTemp string `ottlarg:"0"`
	Monotonic     bool   `ottlarg:"1"`
}

func newExtractSumMetricFactory() ottl.Factory[ottldatapoint.TransformContext] {
	return ottl.NewFactory("extract_sum_metric", &extractSumMetricArguments{}, createExtractSumMetricFunction)
}

func createExtractSumMetricFunction(_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[ottldatapoint.TransformContext], error) {
	args, ok := oArgs.(*extractSumMetricArguments)

	if !ok {
		return nil, fmt.Errorf("extractSumMetricFactory args must be of type *extractSumMetricArguments")
	}

	return extractSumMetric(args.StringAggTemp, args.Monotonic)
}

// this interface helps unify the logic for extracting data from different histogram types
// all supported metric types' datapoints implement it
type SumCountDataPoint interface {
	Attributes() pcommon.Map
	Sum() float64
	Count() uint64
	StartTimestamp() pcommon.Timestamp
	Timestamp() pcommon.Timestamp
}

func extractSumMetric(stringAggTemp string, monotonic bool) (ottl.ExprFunc[ottldatapoint.TransformContext], error) {
	var aggTemp pmetric.AggregationTemporality
	switch stringAggTemp {
	case "delta":
		aggTemp = pmetric.AggregationTemporalityDelta
	case "cumulative":
		aggTemp = pmetric.AggregationTemporalityCumulative
	default:
		return nil, fmt.Errorf("unknown aggregation temporality: %s", stringAggTemp)
	}
	return func(_ context.Context, tCtx ottldatapoint.TransformContext) (interface{}, error) {
		metric := tCtx.GetMetric()

		switch metric.Type() {
		case pmetric.MetricTypeHistogram, pmetric.MetricTypeExponentialHistogram, pmetric.MetricTypeSummary:
		default:
			return nil, nil
		}

		// We're in a datapoint context, but we don't want to create a new Metric for each datapoint
		// so we first check if it doesn't already exist
		sumMetric := getOrCreateSumMetric(
			tCtx.GetMetrics(),
			metric.Name()+"_sum",
			metric.Description(),
			metric.Unit(),
			aggTemp,
			monotonic,
		)

		histogramDatapoint := tCtx.GetDataPoint().(SumCountDataPoint)
		sumDp := sumMetric.Sum().DataPoints().AppendEmpty()
		histogramDatapoint.Attributes().CopyTo(sumDp.Attributes())
		sumDp.SetDoubleValue(histogramDatapoint.Sum())
		sumDp.SetStartTimestamp(histogramDatapoint.StartTimestamp())
		sumDp.SetTimestamp(histogramDatapoint.Timestamp())

		return nil, nil
	}, nil
}

// getOrCreateSumMetric looks for the sum metric in the metric slice, and creates one if it can't find it
func getOrCreateSumMetric(
	metrics pmetric.MetricSlice,
	name string,
	description string,
	unit string,
	aggregationTemporality pmetric.AggregationTemporality,
	isMonotonic bool,
) pmetric.Metric {
	// try to find the metric in the slice
	for i := 0; i < metrics.Len(); i++ {
		metric := metrics.At(i)
		found := (metric.Name() == name &&
			metric.Description() == description &&
			metric.Unit() == unit &&
			metric.Type() == pmetric.MetricTypeSum &&
			metric.Sum().IsMonotonic() == isMonotonic)
		if found {
			return metric
		}
	}

	// couldn't find the metric, create it
	sumMetric := metrics.AppendEmpty()
	sumMetric.SetDescription(description)
	sumMetric.SetName(name)
	sumMetric.SetUnit(unit)
	sumMetric.SetEmptySum().SetAggregationTemporality(aggregationTemporality)
	sumMetric.Sum().SetIsMonotonic(isMonotonic)
	return sumMetric
}
