// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/metrics"

import (
	"context"
	"errors"
	"math"

	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottldatapoint"
)

type mergeHistogramBucketsArguments struct {
	Bound float64
}

func newMergeHistogramBucketsFactory() ottl.Factory[ottldatapoint.TransformContext] {
	return ottl.NewFactory("merge_histogram_buckets", &mergeHistogramBucketsArguments{}, createMergeHistogramBucketsFunction)
}

func createMergeHistogramBucketsFunction(_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[ottldatapoint.TransformContext], error) {
	args, ok := oArgs.(*mergeHistogramBucketsArguments)
	if !ok {
		return nil, errors.New("mergeHistogramBucketsFactory args must be of type *mergeHistogramBucketsArguments")
	}

	return mergeHistogramBuckets(args.Bound)
}

func mergeHistogramBuckets(bound float64) (ottl.ExprFunc[ottldatapoint.TransformContext], error) {
	return func(_ context.Context, tCtx ottldatapoint.TransformContext) (any, error) {
		dataPoint := tCtx.GetDataPoint()
		if dataPoint == nil {
			return nil, nil
		}

		histogramDataPoint, ok := dataPoint.(pmetric.HistogramDataPoint)
		if !ok {
			return nil, nil
		}

		mergeHistogramBucketsFromDataPoint(histogramDataPoint, bound)
		return nil, nil
	}, nil
}

func mergeHistogramBucketsFromDataPoint(dp pmetric.HistogramDataPoint, bound float64) {
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
