// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/metrics"

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/expohisto/conversion"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
)

type convertExponentialHistToExplicitHistArguments struct {
	DistributionFn string
	ExplicitBounds []float64
}

func newconvertExponentialHistToExplicitHistFactory() ottl.Factory[*ottlmetric.TransformContext] {
	return ottl.NewFactory("convert_exponential_histogram_to_histogram",
		&convertExponentialHistToExplicitHistArguments{}, createconvertExponentialHistToExplicitHistFunction)
}

func createconvertExponentialHistToExplicitHistFunction(_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[*ottlmetric.TransformContext], error) {
	args, ok := oArgs.(*convertExponentialHistToExplicitHistArguments)
	if !ok {
		return nil, errors.New("convertExponentialHistToExplicitHistFactory args must be of type *convertExponentialHistToExplicitHistArguments")
	}
	if args.DistributionFn == "" {
		args.DistributionFn = "random"
	}
	return convertExponentialHistToExplicitHist(args.DistributionFn, args.ExplicitBounds)
}

// convertExponentialHistToExplicitHist converts an exponential histogram to a bucketed histogram.
func convertExponentialHistToExplicitHist(distribution string, explicitBounds []float64) (ottl.ExprFunc[*ottlmetric.TransformContext], error) {
	// Validate arguments before the function encounters metric data.
	if _, err := conversion.ToExplicit(conversion.ExponentialHistogram{Scale: 0}, explicitBounds, distribution); err != nil {
		return nil, err
	}

	return func(_ context.Context, tCtx *ottlmetric.TransformContext) (any, error) {
		metric := tCtx.GetMetric()
		if metric.Type() != pmetric.MetricTypeExponentialHistogram {
			return nil, nil
		}

		newMetric := pmetric.NewMetric()
		newMetric.SetName(metric.Name())
		newMetric.SetDescription(metric.Description())
		newMetric.SetUnit(metric.Unit())
		explicitHist := newMetric.SetEmptyHistogram()
		exponentialHist := metric.ExponentialHistogram()
		explicitHist.SetAggregationTemporality(exponentialHist.AggregationTemporality())

		dps := exponentialHist.DataPoints()
		converted := make([][]uint64, dps.Len())
		for i := 0; i < dps.Len(); i++ {
			source := dps.At(i)
			input := conversion.ExponentialHistogram{
				Count:         source.Count(),
				Scale:         source.Scale(),
				ZeroThreshold: source.ZeroThreshold(),
				ZeroCount:     source.ZeroCount(),
				Positive: conversion.Buckets{
					Offset: source.Positive().Offset(),
				},
				Negative: conversion.Buckets{
					Offset: source.Negative().Offset(),
				},
			}
			if counts := source.Positive().BucketCounts(); counts.Len() != 0 {
				input.Positive.Counts = counts
			}
			if counts := source.Negative().BucketCounts(); counts.Len() != 0 {
				input.Negative.Counts = counts
			}
			bucketCounts, err := conversion.ToExplicit(input, explicitBounds, distribution)
			if err != nil {
				return nil, fmt.Errorf("converting exponential histogram data point %d: %w", i, err)
			}
			converted[i] = bucketCounts
		}

		for i := 0; i < dps.Len(); i++ {
			source := dps.At(i)
			destination := explicitHist.DataPoints().AppendEmpty()
			destination.SetStartTimestamp(source.StartTimestamp())
			destination.SetTimestamp(source.Timestamp())
			destination.SetCount(source.Count())
			destination.SetFlags(source.Flags())
			if source.HasSum() {
				destination.SetSum(source.Sum())
			}
			if source.HasMin() {
				destination.SetMin(source.Min())
			}
			if source.HasMax() {
				destination.SetMax(source.Max())
			}
			source.Exemplars().MoveAndAppendTo(destination.Exemplars())
			source.Attributes().MoveTo(destination.Attributes())
			destination.ExplicitBounds().FromRaw(explicitBounds)
			destination.BucketCounts().FromRaw(converted[i])
		}

		newMetric.MoveTo(metric)
		return nil, nil
	}, nil
}
