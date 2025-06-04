// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/metrics"

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"

	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
)

type DropBucketArguments struct {
	Pattern string
}

func newDropHistogramBucketsFactory() ottl.Factory[ottlmetric.TransformContext] {
	return ottl.NewFactory(
		"drop_histogram_buckets",
		&DropBucketArguments{},
		createDropHistogramBucketsFunction,
	)
}

func createDropHistogramBucketsFunction(_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[ottlmetric.TransformContext], error) {
	args, ok := oArgs.(*DropBucketArguments)

	if !ok {
		return nil, errors.New("DropHistogramBucketsFactory args must be of type *DropHistogramBucketsArguments[K]")
	}

	compiledPattern, err := regexp.Compile(args.Pattern)
	if err != nil {
		return nil, fmt.Errorf("the regex pattern supplied to drop_histogram_buckets is not a valid pattern: %w", err)
	}

	return dropHistogramBucketsFunc(compiledPattern), nil
}

func dropHistogramBucketsFunc(compiledPattern *regexp.Regexp) ottl.ExprFunc[ottlmetric.TransformContext] {
	return func(_ context.Context, tCtx ottlmetric.TransformContext) (any, error) {
		metric := tCtx.GetMetric()
		if metric.Type() != pmetric.MetricTypeHistogram {
			return nil, nil
		}

		dps := metric.Histogram().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			hdp := dps.At(i)
			bounds := hdp.ExplicitBounds()
			counts := hdp.BucketCounts()

			boundsSlice := bounds.AsRaw()
			countsSlice := counts.AsRaw()

			if len(countsSlice) == 0 {
				continue
			}

			newBounds := make([]float64, 0, len(boundsSlice))
			newCounts := make([]uint64, 0, len(countsSlice))
			count := uint64(0)

			count += countsSlice[0]
			newCounts = append(newCounts, countsSlice[0])

			for i := 0; i < len(boundsSlice); i++ {
				bound := boundsSlice[i]
				if !compiledPattern.MatchString(strconv.FormatFloat(bound, 'f', -1, 64)) {
					newBounds = append(newBounds, bound)
					count += countsSlice[i+1]
					newCounts = append(newCounts, countsSlice[i+1])
				}
			}

			if len(boundsSlice) == len(newBounds) {
				continue
			}

			hdp.ExplicitBounds().FromRaw(newBounds)
			hdp.BucketCounts().FromRaw(newCounts)
			hdp.SetCount(count)

			// Remove sum since it would be incorrect after removing buckets.
			// We can't accurately calculate the new sum because histogram buckets only contain counts,
			// not the actual values. The values in dropped buckets could be any number within their bounds,
			// making it impossible to know their exact contribution to the total sum.
			hdp.RemoveSum()
		}

		return nil, nil
	}
}
