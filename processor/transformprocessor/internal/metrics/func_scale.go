// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/metrics"

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
)

type ScaleArguments struct {
	Multiplier float64
	Unit       ottl.Optional[ottl.StringGetter[ottlmetric.TransformContext]]
}

func newScaleMetricFactory() ottl.Factory[ottlmetric.TransformContext] {
	return ottl.NewFactory("scale_metric", &ScaleArguments{}, createScaleFunction)
}

func createScaleFunction(_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[ottlmetric.TransformContext], error) {
	args, ok := oArgs.(*ScaleArguments)

	if !ok {
		return nil, errors.New("ScaleFactory args must be of type *ScaleArguments[K]")
	}

	return Scale(*args)
}

func Scale(args ScaleArguments) (ottl.ExprFunc[ottlmetric.TransformContext], error) {
	return func(ctx context.Context, tCtx ottlmetric.TransformContext) (any, error) {
		metric := tCtx.GetMetric()

		var unit *string
		if !args.Unit.IsEmpty() {
			u, err := args.Unit.Get().Get(ctx, tCtx)
			if err != nil {
				return nil, fmt.Errorf("could not get unit from ScaleArguments: %w", err)
			}
			unit = &u
		}

		switch metric.Type() {
		case pmetric.MetricTypeGauge:
			scaleMetric(metric.Gauge().DataPoints(), args.Multiplier)
		case pmetric.MetricTypeHistogram:
			scaleHistogram(metric.Histogram().DataPoints(), args.Multiplier)
		case pmetric.MetricTypeSummary:
			scaleSummarySlice(metric.Summary().DataPoints(), args.Multiplier)
		case pmetric.MetricTypeSum:
			scaleMetric(metric.Sum().DataPoints(), args.Multiplier)
		case pmetric.MetricTypeExponentialHistogram:
			return nil, errors.New("exponential histograms are not supported by the 'scale_metric' function")
		default:
			return nil, fmt.Errorf("unsupported metric type: '%v'", metric.Type())
		}
		if unit != nil {
			metric.SetUnit(*unit)
		}

		return nil, nil
	}, nil
}

func scaleExemplar(ex *pmetric.Exemplar, multiplier float64) {
	switch ex.ValueType() {
	case pmetric.ExemplarValueTypeInt:
		ex.SetIntValue(int64(float64(ex.IntValue()) * multiplier))
	case pmetric.ExemplarValueTypeDouble:
		ex.SetDoubleValue(ex.DoubleValue() * multiplier)
	}
}

func scaleSummarySlice(values pmetric.SummaryDataPointSlice, multiplier float64) {
	for i := 0; i < values.Len(); i++ {
		dp := values.At(i)

		dp.SetSum(dp.Sum() * multiplier)

		for i := 0; i < dp.QuantileValues().Len(); i++ {
			qv := dp.QuantileValues().At(i)
			qv.SetValue(qv.Value() * multiplier)
		}
	}
}

func scaleHistogram(datapoints pmetric.HistogramDataPointSlice, multiplier float64) {
	for i := 0; i < datapoints.Len(); i++ {
		dp := datapoints.At(i)

		if dp.HasSum() {
			dp.SetSum(dp.Sum() * multiplier)
		}
		if dp.HasMin() {
			dp.SetMin(dp.Min() * multiplier)
		}
		if dp.HasMax() {
			dp.SetMax(dp.Max() * multiplier)
		}

		for bounds, bi := dp.ExplicitBounds(), 0; bi < bounds.Len(); bi++ {
			bounds.SetAt(bi, bounds.At(bi)*multiplier)
		}

		for exemplars, ei := dp.Exemplars(), 0; ei < exemplars.Len(); ei++ {
			exemplar := exemplars.At(ei)
			scaleExemplar(&exemplar, multiplier)
		}
	}
}

func scaleMetric(points pmetric.NumberDataPointSlice, multiplier float64) {
	for i := 0; i < points.Len(); i++ {
		dp := points.At(i)
		switch dp.ValueType() {
		case pmetric.NumberDataPointValueTypeInt:
			dp.SetIntValue(int64(float64(dp.IntValue()) * multiplier))

		case pmetric.NumberDataPointValueTypeDouble:
			dp.SetDoubleValue(dp.DoubleValue() * multiplier)
		default:
		}
	}
}
