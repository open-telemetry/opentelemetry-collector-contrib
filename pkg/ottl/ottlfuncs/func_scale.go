// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type ScaleArguments[K any] struct {
	Value      ottl.Getter[K]
	Multiplier float64
}

func NewScaleFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("Scale", &ScaleArguments[K]{}, createScaleFunction[K])
}

func createScaleFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*ScaleArguments[K])

	if !ok {
		return nil, fmt.Errorf("ScaleFactory args must be of type *ScaleArguments[K]")
	}

	return Scale(args.Value, args.Multiplier)
}

func Scale[K any](getter ottl.Getter[K], multiplier float64) (ottl.ExprFunc[K], error) {
	return func(ctx context.Context, tCtx K) (any, error) {
		got, err := getter.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}

		switch value := got.(type) {
		case float64:
			return value * multiplier, nil
		case int64:
			return float64(value) * multiplier, nil
		case pmetric.NumberDataPointSlice:
			scaledMetric := pmetric.NewNumberDataPointSlice()
			value.CopyTo(scaledMetric)
			scaleMetric(scaledMetric, multiplier)
			return scaledMetric, nil
		case pmetric.HistogramDataPointSlice:
			scaledMetric := pmetric.NewHistogramDataPointSlice()
			value.CopyTo(scaledMetric)
			scaleHistogram(scaledMetric, multiplier)
			return scaledMetric, nil
		case pmetric.SummaryDataPointValueAtQuantileSlice:
			scaledMetric := pmetric.NewSummaryDataPointValueAtQuantileSlice()
			value.CopyTo(scaledMetric)
			scaleSummaryDataPointValueAtQuantileSlice(scaledMetric, multiplier)
			return scaledMetric, nil
		case pmetric.ExemplarSlice:
			scaledSlice := pmetric.NewExemplarSlice()
			value.CopyTo(scaledSlice)
			scaleExemplarSlice(scaledSlice, multiplier)
			return scaledSlice, nil
		case pmetric.ExponentialHistogramDataPointSlice:
			return nil, errors.New("exponential histograms are not supported by the 'Scale' function")
		default:
			return nil, fmt.Errorf("unsupported data type: '%T'", value)
		}
	}, nil
}

func scaleExemplarSlice(values pmetric.ExemplarSlice, multiplier float64) {
	for i := 0; i < values.Len(); i++ {
		ex := values.At(i)
		scaleExemplar(&ex, multiplier)
	}
}

func scaleExemplar(ex *pmetric.Exemplar, multiplier float64) {
	switch ex.ValueType() {
	case pmetric.ExemplarValueTypeInt:
		ex.SetIntValue(int64(float64(ex.IntValue()) * multiplier))
	case pmetric.ExemplarValueTypeDouble:
		ex.SetDoubleValue(ex.DoubleValue() * multiplier)
	}
}

func scaleSummaryDataPointValueAtQuantileSlice(values pmetric.SummaryDataPointValueAtQuantileSlice, multiplier float64) {
	for i := 0; i < values.Len(); i++ {
		dp := values.At(i)

		dp.SetValue(dp.Value() * multiplier)
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
