// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/metrics"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
)

type extractCountMetricArguments struct {
	Monotonic bool `ottlarg:"0"`
}

func newExtractCountMetricFactory() ottl.Factory[ottlmetric.TransformContext] {
	return ottl.NewFactory("extract_count_metric", &extractCountMetricArguments{}, createExtractCountMetricFunction)
}

func createExtractCountMetricFunction(_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[ottlmetric.TransformContext], error) {
	args, ok := oArgs.(*extractCountMetricArguments)

	if !ok {
		return nil, fmt.Errorf("extractCountMetricFactory args must be of type *extractCountMetricArguments")
	}

	return extractCountMetric(args.Monotonic)
}

func extractCountMetric(monotonic bool) (ottl.ExprFunc[ottlmetric.TransformContext], error) {
	return func(_ context.Context, tCtx ottlmetric.TransformContext) (interface{}, error) {
		var aggTemp pmetric.AggregationTemporality
		metric := tCtx.GetMetric()
		invalidMetricTypeError := fmt.Errorf("extract_count_metric requires an input metric of type Histogram, ExponentialHistogram or Summary, got %s", metric.Type())

		switch metric.Type() {
		case pmetric.MetricTypeHistogram:
			aggTemp = metric.Histogram().AggregationTemporality()
		case pmetric.MetricTypeExponentialHistogram:
			aggTemp = metric.ExponentialHistogram().AggregationTemporality()
		case pmetric.MetricTypeSummary:
			// Summaries don't have an aggregation temporality, but they *should* be cumulative based on the Openmetrics spec.
			// This should become an optional argument once those are available in OTTL.
			aggTemp = pmetric.AggregationTemporalityCumulative
		default:
			return nil, invalidMetricTypeError
		}

		countMetric := pmetric.NewMetric()
		countMetric.SetDescription(metric.Description())
		countMetric.SetName(metric.Name() + "_count")
		countMetric.SetUnit(metric.Unit())
		countMetric.SetEmptySum().SetAggregationTemporality(aggTemp)
		countMetric.Sum().SetIsMonotonic(monotonic)

		switch metric.Type() {
		case pmetric.MetricTypeHistogram:
			dataPoints := metric.Histogram().DataPoints()
			for i := 0; i < dataPoints.Len(); i++ {
				addCountDataPoint(dataPoints.At(i), countMetric.Sum().DataPoints())
			}
		case pmetric.MetricTypeExponentialHistogram:
			dataPoints := metric.ExponentialHistogram().DataPoints()
			for i := 0; i < dataPoints.Len(); i++ {
				addCountDataPoint(dataPoints.At(i), countMetric.Sum().DataPoints())
			}
		case pmetric.MetricTypeSummary:
			dataPoints := metric.Summary().DataPoints()
			for i := 0; i < dataPoints.Len(); i++ {
				addCountDataPoint(dataPoints.At(i), countMetric.Sum().DataPoints())
			}
		default:
			return nil, invalidMetricTypeError
		}

		if countMetric.Sum().DataPoints().Len() > 0 {
			countMetric.MoveTo(tCtx.GetMetrics().AppendEmpty())
		}

		return nil, nil
	}, nil
}

func addCountDataPoint(dataPoint SumCountDataPoint, destination pmetric.NumberDataPointSlice) {
	newDp := destination.AppendEmpty()
	dataPoint.Attributes().CopyTo(newDp.Attributes())
	newDp.SetIntValue(int64(dataPoint.Count()))
	newDp.SetStartTimestamp(dataPoint.StartTimestamp())
	newDp.SetTimestamp(dataPoint.Timestamp())
}
