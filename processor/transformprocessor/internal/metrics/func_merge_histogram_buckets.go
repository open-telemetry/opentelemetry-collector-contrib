// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/metrics"

import (
	"context"
	"errors"
	"fmt"
	"math"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottldatapoint"
)

const (
	mergeHistogramBucketsMethodRemoveExplicitBound = "remove_explicit_bound"
	mergeHistogramBucketsMethodLimitBuckets        = "limit_buckets"
)

type mergeHistogramBucketsArguments struct {
	TargetValue ottl.FloatLikeGetter[*ottldatapoint.TransformContext]
	Method      ottl.Optional[string]
}

func newMergeHistogramBucketsFactory() ottl.Factory[*ottldatapoint.TransformContext] {
	return ottl.NewFactory("merge_histogram_buckets", &mergeHistogramBucketsArguments{}, createMergeHistogramBucketsFunction)
}

func createMergeHistogramBucketsFunction(_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[*ottldatapoint.TransformContext], error) {
	args, ok := oArgs.(*mergeHistogramBucketsArguments)
	if !ok {
		return nil, errors.New("mergeHistogramBucketsFactory args must be of type *mergeHistogramBucketsArguments")
	}

	return mergeHistogramBuckets(args.TargetValue, args.Method)
}

func mergeHistogramBuckets(targetValue ottl.FloatLikeGetter[*ottldatapoint.TransformContext], method ottl.Optional[string]) (ottl.ExprFunc[*ottldatapoint.TransformContext], error) {
	methodValue := method.GetOr(mergeHistogramBucketsMethodRemoveExplicitBound)
	if methodValue != mergeHistogramBucketsMethodRemoveExplicitBound && methodValue != mergeHistogramBucketsMethodLimitBuckets {
		return nil, fmt.Errorf("unsupported method %q, expected %q or %q", methodValue, mergeHistogramBucketsMethodRemoveExplicitBound, mergeHistogramBucketsMethodLimitBuckets)
	}

	var literalTarget *float64
	var literalMaxBuckets *int64
	if target, ok := ottl.GetLiteralValue(targetValue); ok {
		if target == nil {
			return nil, errors.New("target_value must not be nil")
		}
		literalTarget = target

		if methodValue == mergeHistogramBucketsMethodLimitBuckets {
			maxBuckets, err := maxBucketsFromTargetValue(*target)
			if err != nil {
				return nil, err
			}
			literalMaxBuckets = &maxBuckets
		}
	}

	return func(ctx context.Context, tCtx *ottldatapoint.TransformContext) (any, error) {
		histogramDataPoint, ok := histogramDataPointFromContext(tCtx)
		if !ok {
			return nil, nil
		}

		target := literalTarget
		if target == nil {
			var err error
			target, err = targetValue.Get(ctx, tCtx)
			if err != nil {
				return nil, err
			}
			if target == nil {
				return nil, errors.New("target_value must not be nil")
			}
		}

		switch methodValue {
		case mergeHistogramBucketsMethodRemoveExplicitBound:
			mergeHistogramBucketsByExplicitBoundFromDataPoint(histogramDataPoint, *target)
		case mergeHistogramBucketsMethodLimitBuckets:
			maxBuckets := literalMaxBuckets
			if maxBuckets == nil {
				dynamicMaxBuckets, err := maxBucketsFromTargetValue(*target)
				if err != nil {
					return nil, err
				}
				maxBuckets = &dynamicMaxBuckets
			}
			limitHistogramBucketsFromDataPoint(histogramDataPoint, *maxBuckets)
		}
		return nil, nil
	}, nil
}

func histogramDataPointFromContext(tCtx *ottldatapoint.TransformContext) (pmetric.HistogramDataPoint, bool) {
	dataPoint := tCtx.GetDataPoint()
	if dataPoint == nil {
		return pmetric.HistogramDataPoint{}, false
	}

	histogramDataPoint, ok := dataPoint.(pmetric.HistogramDataPoint)
	return histogramDataPoint, ok
}

func mergeHistogramBucketsByExplicitBoundFromDataPoint(dp pmetric.HistogramDataPoint, bound float64) {
	explicitBounds := dp.ExplicitBounds()
	bucketCounts := dp.BucketCounts()

	if explicitBounds.Len()+1 != bucketCounts.Len() {
		return
	}

	if bucketCounts.Len() == 1 {
		return
	}

	bounds := explicitBounds.AsRaw()
	targetIndex := findBoundIndex(&bounds, bound)
	if targetIndex == -1 {
		return
	}

	counts := bucketCounts.AsRaw()

	nextBucketIndex := targetIndex + 1
	counts[nextBucketIndex] += counts[targetIndex]

	newBounds := make([]float64, 0, len(bounds)-1)
	newBounds = append(newBounds, bounds[:targetIndex]...)
	newBounds = append(newBounds, bounds[targetIndex+1:]...)

	newCounts := make([]uint64, 0, len(counts)-1)
	newCounts = append(newCounts, counts[:targetIndex]...)
	newCounts = append(newCounts, counts[targetIndex+1:]...)

	explicitBounds.FromRaw(newBounds)
	bucketCounts.FromRaw(newCounts)
}

func maxBucketsFromTargetValue(targetValue float64) (int64, error) {
	if targetValue < 1 || math.Trunc(targetValue) != targetValue || targetValue > float64(math.MaxInt64) {
		return 0, fmt.Errorf("target_value must be a positive integer when method is %q, got %v", mergeHistogramBucketsMethodLimitBuckets, targetValue)
	}
	return int64(targetValue), nil
}

func limitHistogramBucketsFromDataPoint(dp pmetric.HistogramDataPoint, maxBuckets int64) {
	explicitBounds := dp.ExplicitBounds()
	bucketCounts := dp.BucketCounts()

	if explicitBounds.Len()+1 != bucketCounts.Len() {
		return
	}

	if int64(bucketCounts.Len()) <= maxBuckets || bucketCounts.Len() <= 1 {
		return
	}

	if !strictlyIncreasing(explicitBounds) {
		return
	}

	divisor := ceilDiv(bucketCounts.Len(), int(maxBuckets))
	compactHistogramBuckets(explicitBounds, bucketCounts, divisor)
}

func ceilDiv(dividend, divisor int) int {
	return (dividend-1)/divisor + 1
}

func compactHistogramBuckets(bounds pcommon.Float64Slice, counts pcommon.UInt64Slice, divisor int) {
	compactCounts := pcommon.NewUInt64Slice()
	compactCounts.EnsureCapacity(ceilDiv(counts.Len(), divisor))
	for i := 0; i < counts.Len(); i += divisor {
		end := min(i+divisor, counts.Len())
		var count uint64
		for j := i; j < end; j++ {
			count += counts.At(j)
		}
		compactCounts.Append(count)
	}

	compactBounds := pcommon.NewFloat64Slice()
	compactBounds.EnsureCapacity(compactCounts.Len() - 1)
	for i := divisor - 1; i < bounds.Len(); i += divisor {
		compactBounds.Append(bounds.At(i))
	}

	compactBounds.MoveTo(bounds)
	compactCounts.MoveTo(counts)
}

func strictlyIncreasing(bounds pcommon.Float64Slice) bool {
	for i := 1; i < bounds.Len(); i++ {
		if !(bounds.At(i) > bounds.At(i-1)) {
			return false
		}
	}
	return true
}

// findBoundIndex finds the index of a target bound in the bounds slice with epsilon tolerance
func findBoundIndex(bounds *[]float64, target float64) int {
	const epsilon = 1e-12

	for i, bound := range *bounds {
		if math.Abs(bound-target) < epsilon {
			return i
		}
	}
	return -1
}
