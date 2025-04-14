// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/metrics"

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
)

const sumCountName = "extract_count_metric"

type extractCountMetricArguments struct {
	Monotonic bool
}

func newExtractCountMetricFactory() ottl.Factory[ottlmetric.TransformContext] {
	return ottl.NewFactory(sumCountName, &extractCountMetricArguments{}, createExtractCountMetricFunction)
}

func createExtractCountMetricFunction(_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[ottlmetric.TransformContext], error) {
	args, ok := oArgs.(*extractCountMetricArguments)

	if !ok {
		return nil, errors.New("extractCountMetricFactory args must be of type *extractCountMetricArguments")
	}

	return extractCountMetric(args.Monotonic)
}

func extractCountMetric(monotonic bool) (ottl.ExprFunc[ottlmetric.TransformContext], error) {
	return func(_ context.Context, tCtx ottlmetric.TransformContext) (any, error) {
		metric := tCtx.GetMetric()

		aggTemp := getAggregationTemporality(metric)
		if aggTemp == pmetric.AggregationTemporalityUnspecified {
			return nil, invalidMetricTypeError(sumCountName, metric)
		}

		countMetric := pmetric.NewMetric()
		countMetric.SetDescription(metric.Description())
		countMetric.SetName(metric.Name() + "_count")
		// Use the default unit as the original metric unit does not apply to the 'count' field
		countMetric.SetUnit("1")
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
			return nil, invalidMetricTypeError(sumCountName, metric)
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
