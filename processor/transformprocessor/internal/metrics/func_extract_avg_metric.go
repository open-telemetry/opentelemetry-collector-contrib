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

const avgFuncName = "extract_avg_metric"

type extractAvgMetricArguments struct {
	Monotonic bool
	Suffix    ottl.Optional[string]
}

func newExtractAvgMetricFactory() ottl.Factory[ottlmetric.TransformContext] {
	return ottl.NewFactory(avgFuncName, &extractAvgMetricArguments{}, createExtractAvgMetricFunction)
}

func createExtractAvgMetricFunction(_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[ottlmetric.TransformContext], error) {
	args, ok := oArgs.(*extractAvgMetricArguments)

	if !ok {
		return nil, errors.New("extractAvgMetricFactory args must be of type *extractAvgMetricArguments")
	}

	return extractAvgMetric(args.Monotonic, args.Suffix)
}

func extractAvgMetric(monotonic bool, suffix ottl.Optional[string]) (ottl.ExprFunc[ottlmetric.TransformContext], error) {
	metricNameSuffix := "_avg"
	if !suffix.IsEmpty() {
		metricNameSuffix = suffix.Get()
	}
	return func(_ context.Context, tCtx ottlmetric.TransformContext) (any, error) {
		metric := tCtx.GetMetric()

		aggTemp := getAggregationTemporality(metric)
		if aggTemp == pmetric.AggregationTemporalityUnspecified {
			return nil, invalidMetricTypeError(avgFuncName, metric)
		}

		avgMetric := pmetric.NewMetric()
		avgMetric.SetDescription(metric.Description())
		avgMetric.SetName(metric.Name() + metricNameSuffix)
		avgMetric.SetUnit(metric.Unit())
		avgMetric.SetEmptySum().SetAggregationTemporality(aggTemp)
		avgMetric.Sum().SetIsMonotonic(monotonic)

		switch metric.Type() {
		case pmetric.MetricTypeHistogram:
			dataPoints := metric.Histogram().DataPoints()
			for i := 0; i < dataPoints.Len(); i++ {
				dataPoint := dataPoints.At(i)
				if dataPoint.HasSum() && dataPoint.Count() > 0 {
					addAvgDataPoint(dataPoint, avgMetric.Sum().DataPoints())
				}
			}
		case pmetric.MetricTypeExponentialHistogram:
			dataPoints := metric.ExponentialHistogram().DataPoints()
			for i := 0; i < dataPoints.Len(); i++ {
				dataPoint := dataPoints.At(i)
				if dataPoint.HasSum() && dataPoint.Count() > 0 {
					addAvgDataPoint(dataPoint, avgMetric.Sum().DataPoints())
				}
			}
		case pmetric.MetricTypeSummary:
			dataPoints := metric.Summary().DataPoints()
			for i := 0; i < dataPoints.Len(); i++ {
				dataPoint := dataPoints.At(i)
				// Summary requires Sum, no additional check needed
				if dataPoint.Count() > 0 {
					addAvgDataPoint(dataPoint, avgMetric.Sum().DataPoints())
				}
			}
		default:
			return nil, invalidMetricTypeError(avgFuncName, metric)
		}

		if avgMetric.Sum().DataPoints().Len() > 0 {
			avgMetric.MoveTo(tCtx.GetMetrics().AppendEmpty())
		}

		return nil, nil
	}, nil
}

func addAvgDataPoint(dataPoint SumCountDataPoint, destination pmetric.NumberDataPointSlice) {
	newDp := destination.AppendEmpty()
	dataPoint.Attributes().CopyTo(newDp.Attributes())
	newDp.SetDoubleValue(dataPoint.Sum() / float64(dataPoint.Count()))
	newDp.SetStartTimestamp(dataPoint.StartTimestamp())
	newDp.SetTimestamp(dataPoint.Timestamp())
}
