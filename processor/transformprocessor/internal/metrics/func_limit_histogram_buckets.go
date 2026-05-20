// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/metrics"

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottldatapoint"
)

type limitHistogramBucketsArguments struct {
	MaxBuckets int64
}

func newLimitHistogramBucketsFactory() ottl.Factory[*ottldatapoint.TransformContext] {
	return ottl.NewFactory("limit_histogram_buckets", &limitHistogramBucketsArguments{}, createLimitHistogramBucketsFunction)
}

func createLimitHistogramBucketsFunction(_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[*ottldatapoint.TransformContext], error) {
	args, ok := oArgs.(*limitHistogramBucketsArguments)
	if !ok {
		return nil, errors.New("limitHistogramBucketsFactory args must be of type *limitHistogramBucketsArguments")
	}

	return limitHistogramBuckets(args.MaxBuckets)
}

func limitHistogramBuckets(maxBuckets int64) (ottl.ExprFunc[*ottldatapoint.TransformContext], error) {
	if maxBuckets < 1 {
		return nil, fmt.Errorf("max_buckets must be greater than 0, got %d", maxBuckets)
	}

	return func(_ context.Context, tCtx *ottldatapoint.TransformContext) (any, error) {
		dataPoint := tCtx.GetDataPoint()
		if dataPoint == nil {
			return nil, nil
		}

		histogramDataPoint, ok := dataPoint.(pmetric.HistogramDataPoint)
		if !ok {
			return nil, nil
		}

		limitHistogramBucketsFromDataPoint(histogramDataPoint, maxBuckets)
		return nil, nil
	}, nil
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

	for int64(bucketCounts.Len()) > maxBuckets {
		compactHistogramBuckets(explicitBounds, bucketCounts)
	}
}

func compactHistogramBuckets(bounds pcommon.Float64Slice, counts pcommon.UInt64Slice) {
	compactCounts := pcommon.NewUInt64Slice()
	compactCounts.EnsureCapacity((counts.Len() + 1) / 2)
	for i := 0; i < counts.Len(); i += 2 {
		if i+1 == counts.Len() {
			compactCounts.Append(counts.At(i))
			continue
		}
		compactCounts.Append(counts.At(i) + counts.At(i+1))
	}

	compactBounds := pcommon.NewFloat64Slice()
	compactBounds.EnsureCapacity(compactCounts.Len() - 1)
	for i := 1; i < bounds.Len(); i += 2 {
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
