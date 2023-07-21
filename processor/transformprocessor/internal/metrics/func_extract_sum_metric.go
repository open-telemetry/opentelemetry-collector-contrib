// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/metrics"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
)

type extractSumMetricArguments struct {
	AggTemp   ottl.Enum `ottlarg:"0"`
	Monotonic bool      `ottlarg:"1"`
}

func newExtractSumMetricFactory() ottl.Factory[ottlmetric.TransformContext] {
	return ottl.NewFactory("extract_sum_metric", &extractSumMetricArguments{}, createExtractSumMetricFunction)
}

func createExtractSumMetricFunction(_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[ottlmetric.TransformContext], error) {
	args, ok := oArgs.(*extractSumMetricArguments)

	if !ok {
		return nil, fmt.Errorf("extractSumMetricFactory args must be of type *extractSumMetricArguments")
	}

	return extractSumMetric(args.AggTemp, args.Monotonic)
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

func extractSumMetric(aggTempEnum ottl.Enum, monotonic bool) (ottl.ExprFunc[ottlmetric.TransformContext], error) {
	aggTemp := pmetric.AggregationTemporality(aggTempEnum)
	switch aggTemp {
	case pmetric.AggregationTemporalityDelta, pmetric.AggregationTemporalityCumulative:
	default:
		return nil, fmt.Errorf("unknown aggregation temporality: %s", aggTemp.String())
	}

	return func(_ context.Context, tCtx ottlmetric.TransformContext) (interface{}, error) {
		metric := tCtx.GetMetric()

		switch metric.Type() {
		case pmetric.MetricTypeHistogram, pmetric.MetricTypeExponentialHistogram, pmetric.MetricTypeSummary:
		default:
			return nil, nil
		}

		sumMetric := createSumMetric(
			tCtx.GetMetrics(),
			metric.Name()+"_sum",
			metric.Description(),
			metric.Unit(),
			aggTemp,
			monotonic,
		)

		switch metric.Type() {
		case pmetric.MetricTypeHistogram:
			dataPoints := metric.Histogram().DataPoints()
			for i := 0; i < dataPoints.Len(); i++ {
				addSumDataPoint(dataPoints.At(i), sumMetric.Sum().DataPoints())
			}
		case pmetric.MetricTypeExponentialHistogram:
			dataPoints := metric.ExponentialHistogram().DataPoints()
			for i := 0; i < dataPoints.Len(); i++ {
				addSumDataPoint(dataPoints.At(i), sumMetric.Sum().DataPoints())
			}
		case pmetric.MetricTypeSummary:
			dataPoints := metric.Summary().DataPoints()
			for i := 0; i < dataPoints.Len(); i++ {
				addSumDataPoint(dataPoints.At(i), sumMetric.Sum().DataPoints())
			}
		default:
			return nil, nil
		}

		return nil, nil
	}, nil
}

func createSumMetric(
	metrics pmetric.MetricSlice,
	name string,
	description string,
	unit string,
	aggregationTemporality pmetric.AggregationTemporality,
	isMonotonic bool,
) pmetric.Metric {
	sumMetric := metrics.AppendEmpty()
	sumMetric.SetDescription(description)
	sumMetric.SetName(name)
	sumMetric.SetUnit(unit)
	sumMetric.SetEmptySum().SetAggregationTemporality(aggregationTemporality)
	sumMetric.Sum().SetIsMonotonic(isMonotonic)
	return sumMetric
}

func addSumDataPoint(dataPoint SumCountDataPoint, destination pmetric.NumberDataPointSlice) {
	newDp := destination.AppendEmpty()
	dataPoint.Attributes().CopyTo(newDp.Attributes())
	newDp.SetDoubleValue(dataPoint.Sum())
	newDp.SetStartTimestamp(dataPoint.StartTimestamp())
	newDp.SetTimestamp(dataPoint.Timestamp())
}
