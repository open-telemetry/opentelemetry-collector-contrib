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

func newDropBucketFactory() ottl.Factory[ottlmetric.TransformContext] {
	return ottl.NewFactory(
		"drop_bucket",
		&DropBucketArguments{},
		createDropBucketFunction,
	)
}

func createDropBucketFunction(_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[ottlmetric.TransformContext], error) {
	args, ok := oArgs.(*DropBucketArguments)

	if !ok {
		return nil, errors.New("DropBucketFactory args must be of type *DropBucketArguments[K]")
	}

	return dropBucketFunc(args.Pattern), nil
}

func dropBucketFunc(pattern string) ottl.ExprFunc[ottlmetric.TransformContext] {
	return func(_ context.Context, tCtx ottlmetric.TransformContext) (any, error) {
		metric := tCtx.GetMetric()
		if metric.Type() != pmetric.MetricTypeHistogram {
			return nil, nil
		}

		compiledPattern, err := regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("the regex pattern supplied to drop_bucket is not a valid pattern: %w", err)
		}

		dps := metric.Histogram().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			hdp := dps.At(i)
			bounds := hdp.ExplicitBounds()
			counts := hdp.BucketCounts()

			boundsSlice := bounds.AsRaw()
			countsSlice := counts.AsRaw()

			countsToRemove := make(map[int]bool)
			idx := 1

			newBounds := RemoveIfFloat64(boundsSlice, func(bound float64, _ int) bool {
				defer func() {
					idx++
				}()
				if compiledPattern.MatchString(strconv.FormatFloat(bound, 'f', -1, 64)) {
					countsToRemove[idx] = true
					return true
				}

				return false
			})

			if len(countsToRemove) == 0 {
				continue
			}

			count := uint64(0)
			newCounts := RemoveIfUint64(countsSlice, func(c uint64, idx int) bool {
				if countsToRemove[idx] {
					return true
				}
				count += c
				return false
			})

			hdp.ExplicitBounds().FromRaw(newBounds)
			hdp.BucketCounts().FromRaw(newCounts)
			hdp.SetCount(count)

			// Remove sum since it would be incorrect after removing buckets
			hdp.RemoveSum()
		}

		return nil, nil
	}
}
