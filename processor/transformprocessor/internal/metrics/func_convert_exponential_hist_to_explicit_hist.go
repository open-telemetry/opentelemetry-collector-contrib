// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/metrics"

import (
	"context"
	"errors"
	"fmt"
	"math"

	"go.opentelemetry.io/collector/pdata/pmetric"
	"golang.org/x/exp/rand"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
)

type convertExponentialHistToExplicitHistArguments struct {
	DistributionFn string
	ExplicitBounds []float64
}

// distributionFnMap - map of conversion functions
var distributionFnMap = map[string]distAlgorithm{
	"upper":    upperAlgorithm,
	"midpoint": midpointAlgorithm,
	"random":   randomAlgorithm,
	"uniform":  uniformAlgorithm,
}

func newconvertExponentialHistToExplicitHistFactory() ottl.Factory[ottlmetric.TransformContext] {
	return ottl.NewFactory("convert_exponential_histogram_to_histogram",
		&convertExponentialHistToExplicitHistArguments{}, createconvertExponentialHistToExplicitHistFunction)
}

func createconvertExponentialHistToExplicitHistFunction(_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[ottlmetric.TransformContext], error) {
	args, ok := oArgs.(*convertExponentialHistToExplicitHistArguments)

	if !ok {
		return nil, errors.New("convertExponentialHistToExplicitHistFactory args must be of type *convertExponentialHistToExplicitHistArguments")
	}

	if len(args.DistributionFn) == 0 {
		args.DistributionFn = "random"
	}

	if _, ok := distributionFnMap[args.DistributionFn]; !ok {
		return nil, fmt.Errorf("invalid conversion function: %s, must be one of [upper, midpoint, random, uniform]", args.DistributionFn)
	}

	return convertExponentialHistToExplicitHist(args.DistributionFn, args.ExplicitBounds)
}

// convertExponentialHistToExplicitHist converts an exponential histogram to a bucketed histogram
func convertExponentialHistToExplicitHist(distributionFn string, explicitBounds []float64) (ottl.ExprFunc[ottlmetric.TransformContext], error) {
	if len(explicitBounds) == 0 {
		return nil, fmt.Errorf("explicit bounds cannot be empty: %v", explicitBounds)
	}

	distFn, ok := distributionFnMap[distributionFn]
	if !ok {
		return nil, fmt.Errorf("invalid distribution algorithm: %s, must be one of [upper, midpoint, random, uniform]", distributionFn)
	}

	return func(_ context.Context, tCtx ottlmetric.TransformContext) (any, error) {
		metric := tCtx.GetMetric()

		// only execute on exponential histograms
		if metric.Type() != pmetric.MetricTypeExponentialHistogram {
			return nil, nil
		}

		// create new metric and override metric
		newMetric := pmetric.NewMetric()
		newMetric.SetName(metric.Name())
		newMetric.SetDescription(metric.Description())
		newMetric.SetUnit(metric.Unit())
		explicitHist := newMetric.SetEmptyHistogram()

		dps := metric.ExponentialHistogram().DataPoints()
		explicitHist.SetAggregationTemporality(metric.ExponentialHistogram().AggregationTemporality())

		// map over each exponential histogram data point and calculate the bucket counts
		for i := 0; i < dps.Len(); i++ {
			expDataPoint := dps.At(i)
			bucketCounts := calculateBucketCounts(expDataPoint, explicitBounds, distFn)
			explicitHistDp := explicitHist.DataPoints().AppendEmpty()
			explicitHistDp.SetStartTimestamp(expDataPoint.StartTimestamp())
			explicitHistDp.SetTimestamp(expDataPoint.Timestamp())
			explicitHistDp.SetCount(expDataPoint.Count())
			explicitHistDp.SetSum(expDataPoint.Sum())
			explicitHistDp.SetMin(expDataPoint.Min())
			explicitHistDp.SetMax(expDataPoint.Max())
			expDataPoint.Exemplars().MoveAndAppendTo(explicitHistDp.Exemplars())
			explicitHistDp.ExplicitBounds().FromRaw(explicitBounds)
			explicitHistDp.BucketCounts().FromRaw(bucketCounts)
			expDataPoint.Attributes().MoveTo(explicitHistDp.Attributes())
		}

		newMetric.MoveTo(metric)

		return nil, nil
	}, nil
}

type distAlgorithm func(count uint64, upper, lower float64, boundaries []float64, bucketCountsDst *[]uint64)

func calculateBucketCounts(dp pmetric.ExponentialHistogramDataPoint, boundaries []float64, distFn distAlgorithm) []uint64 {
	scale := int(dp.Scale())
	factor := math.Ldexp(math.Ln2, -scale)
	posB := dp.Positive().BucketCounts()
	bucketCounts := make([]uint64, len(boundaries))

	// add zerocount if boundary starts at zero
	if zerocount := dp.ZeroCount(); zerocount > 0 && boundaries[0] == 0 {
		bucketCounts[0] += zerocount
	}

	for pos := 0; pos < posB.Len(); pos++ {
		index := dp.Positive().Offset() + int32(pos)
		upper := math.Exp(float64(index+1) * factor)
		lower := math.Exp(float64(index) * factor)
		count := posB.At(pos)
		runDistFn := true

		// if the lower bound is greater than the last boundary, add the count to the overflow bucket
		if lower > boundaries[len(boundaries)-1] {
			bucketCounts[len(boundaries)-1] += count
			continue
		}

		// check if lower and upper bounds are within the boundaries
		for bIndex := 1; bIndex < len(boundaries); bIndex++ {
			if lower > boundaries[bIndex-1] && upper <= boundaries[bIndex] {
				bucketCounts[bIndex-1] += count
				runDistFn = false
				break
			}
		}

		if runDistFn {
			distFn(count, upper, lower, boundaries, &bucketCounts)
		}
	}

	return bucketCounts
}

// upperAlgorithm function calculates the bucket counts for a given exponential histogram data point.
// The algorithm is inspired by the logExponentialHistogramDataPoints function used to Print Exponential Histograms in Otel.
// found here: https://github.com/open-telemetry/opentelemetry-collector/blob/main/exporter/internal/otlptext/databuffer.go#L144-L201
//
// - factor is calculated as math.Ldexp(math.Ln2, -scale)
//
// - next we iterate the bucket counts and positions (pos) in the exponential histogram datapoint.
//
//   - the index is calculated by adding the exponential offset to the positive bucket position (pos)
//
//   - the factor is then used to calculate the upper bound of the bucket which is calculated as
//     upper = math.Exp((index+1) * factor)
var upperAlgorithm distAlgorithm = func(count uint64,
	upper, _ float64, boundaries []float64,
	bucketCountsDst *[]uint64,
) {
	// count := bucketCountsSrc.At(index)

	// At this point we know that the upper bound represents the highest value that can be in this bucket, so we take the
	// upper bound and compare it to each of the explicit boundaries provided by the user until we find a boundary
	// that fits, that is, the first instance where upper bound <= explicit boundary.
	for j, boundary := range boundaries {
		if upper <= boundary {
			(*bucketCountsDst)[j] += count
			return
		}
	}
	(*bucketCountsDst)[len(boundaries)-1] += count // Overflow bucket
}

// midpointAlgorithm calculates the bucket counts for a given exponential histogram data point.
// This algorithm is similar to calculateBucketCountsWithUpperBounds, but instead of using the upper bound of the bucket
// to determine the bucket, it uses the midpoint of the upper and lower bounds.
// The midpoint is calculated as (upper + lower) / 2.
var midpointAlgorithm distAlgorithm = func(count uint64,
	upper, lower float64, boundaries []float64,
	bucketCountsDst *[]uint64,
) {
	midpoint := (upper + lower) / 2

	for j, boundary := range boundaries {
		if midpoint <= boundary {
			if j > 0 {
				(*bucketCountsDst)[j-1] += count
				return
			}
			(*bucketCountsDst)[j] += count
			return
		}
	}
	(*bucketCountsDst)[len(boundaries)-1] += count // Overflow bucket
}

// uniformAlgorithm distributes counts from a given set of bucket sources into a set of linear boundaries using uniform distribution
var uniformAlgorithm distAlgorithm = func(count uint64,
	upper, lower float64, boundaries []float64,
	bucketCountsDst *[]uint64,
) {
	// Find the boundaries that intersect with the bucket range
	var start, end int
	for start = 0; start < len(boundaries); start++ {
		if lower <= boundaries[start] {
			break
		}
	}

	for end = start; end < len(boundaries); end++ {
		if upper <= boundaries[end] {
			break
		}
	}

	// make sure end value does not exceed the length of the boundaries
	if end > len(boundaries)-1 {
		end = len(boundaries) - 1
	}

	// Distribute the count uniformly across the intersecting boundaries
	if end > start {
		countPerBoundary := count / uint64(end-start+1)
		remainder := count % uint64(end-start+1)

		for j := start; j <= end; j++ {
			(*bucketCountsDst)[j] += countPerBoundary
			if remainder > 0 {
				(*bucketCountsDst)[j]++
				remainder--
			}
		}
	} else {
		// Handle the case where the bucket range does not intersect with any boundaries
		(*bucketCountsDst)[start] += count
	}
}

// randomAlgorithm distributes counts from a given set of bucket sources into a set of linear boundaries using random distribution
var randomAlgorithm distAlgorithm = func(count uint64,
	upper, lower float64, boundaries []float64,
	bucketCountsDst *[]uint64,
) {
	// Find the boundaries that intersect with the bucket range
	start := 0
	for start < len(boundaries) && boundaries[start] < lower {
		start++
	}
	end := start
	for end < len(boundaries) && boundaries[end] < upper {
		end++
	}

	// make sure end value does not exceed the length of the boundaries
	if end > len(boundaries)-1 {
		end = len(boundaries) - 1
	}

	// Randomly distribute the count across the intersecting boundaries
	if end > start {
		rangeWidth := upper - lower
		totalAllocated := uint64(0)

		for j := start; j <= end; j++ {
			var boundaryLower, boundaryUpper float64
			if j == 0 {
				// For the first boundary, set the lower limit to the bucket's lower bound
				boundaryLower = lower
			} else {
				// Otherwise, set it to the previous boundary
				boundaryLower = boundaries[j-1]
			}
			if j == len(boundaries) {
				// For the last boundary, set the upper limit to the bucket's upper bound
				boundaryUpper = upper
			} else {
				// Otherwise, set it to the current boundary
				boundaryUpper = boundaries[j]
			}

			// Calculate the overlap width between the boundary range and the bucket range
			overlapWidth := math.Min(boundaryUpper, upper) - math.Max(boundaryLower, lower)
			// Proportionally allocate the count based on the overlap width
			allocatedCount := uint64(float64(count) * (overlapWidth / rangeWidth))

			// Randomly assign the counts to the boundaries
			randomlyAllocatedCount := uint64(rand.Float64() * float64(allocatedCount))
			(*bucketCountsDst)[j] += randomlyAllocatedCount
			totalAllocated += randomlyAllocatedCount
		}

		// Distribute any remaining count
		remainingCount := count - totalAllocated
		for remainingCount > 0 {
			randomBoundary := rand.Intn(end-start+1) + start
			(*bucketCountsDst)[randomBoundary]++
			remainingCount--
		}
	} else {
		// If the bucket range does not intersect with any boundaries, assign the entire count to the start boundary
		(*bucketCountsDst)[start] += count
	}
}
