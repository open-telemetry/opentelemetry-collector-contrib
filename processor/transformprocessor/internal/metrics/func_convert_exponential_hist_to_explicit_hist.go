// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"context"
	"fmt"
	"math"
	"time"

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
var distributionFnMap = map[string]func(pmetric.ExponentialHistogramDataPoint, []float64) []uint64{
	"upper":    calculateBucketCountsWithUpperBounds,
	"midpoint": calculateBucketCountsWithMidpoint,
	"random":   calculateBucketCountsWithRandomDistribution,
	"uniform":  calculateBucketCountsWithUniformDistribution,
}

func newconvertExponentialHistToExplicitHistFactory() ottl.Factory[ottlmetric.TransformContext] {
	return ottl.NewFactory("convert_exponential_hist_to_explicit_hist", &convertExponentialHistToExplicitHistArguments{}, createconvertExponentialHistToExplicitHistFunction)
}

func createconvertExponentialHistToExplicitHistFunction(_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[ottlmetric.TransformContext], error) {
	args, ok := oArgs.(*convertExponentialHistToExplicitHistArguments)

	if !ok {
		return nil, fmt.Errorf("convertExponentialHistToExplicitHistFactory args must be of type *convertExponentialHistToExplicitHistArguments")
	}

	if len(args.DistributionFn) == 0 {
		args.DistributionFn = "upper"
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
		return nil, fmt.Errorf("invalid conversion function: %s, must be one of [upper, midpoint, random, uniform]", distributionFn)
	}

	return func(_ context.Context, tCtx ottlmetric.TransformContext) (any, error) {
		metric := tCtx.GetMetric()

		// only execute on exponential histograms
		if metric.Type() != pmetric.MetricTypeExponentialHistogram {
			return nil, nil
		}

		bucketedHist := pmetric.NewHistogram()
		dps := metric.ExponentialHistogram().DataPoints()
		bucketedHist.SetAggregationTemporality(metric.ExponentialHistogram().AggregationTemporality())

		// map over each exponential histogram data point and calculate the bucket counts
		for i := 0; i < dps.Len(); i++ {
			expDataPoint := dps.At(i)
			bucketCounts := distFn(expDataPoint, explicitBounds)
			bucketHistDatapoint := bucketedHist.DataPoints().AppendEmpty()
			bucketHistDatapoint.SetStartTimestamp(expDataPoint.StartTimestamp())
			bucketHistDatapoint.SetTimestamp(expDataPoint.Timestamp())
			bucketHistDatapoint.SetCount(expDataPoint.Count())
			bucketHistDatapoint.SetSum(expDataPoint.Sum())
			bucketHistDatapoint.SetMin(expDataPoint.Min())
			bucketHistDatapoint.SetMax(expDataPoint.Max())
			bucketHistDatapoint.ExplicitBounds().FromRaw(explicitBounds)
			bucketHistDatapoint.BucketCounts().FromRaw(bucketCounts)
			expDataPoint.Attributes().CopyTo(bucketHistDatapoint.Attributes())
		}

		// create new metric and override metric
		newMetric := pmetric.NewMetric()
		newMetric.SetName(metric.Name())
		newMetric.SetDescription(metric.Description())
		newMetric.SetUnit(metric.Unit())
		bucketedHist.CopyTo(newMetric.SetEmptyHistogram())
		newMetric.CopyTo(metric)

		return nil, nil
	}, nil
}

// calculateBucketCountsWithUpperBounds function calculates the bucket counts for a given exponential histogram data point.
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
func calculateBucketCountsWithUpperBounds(dp pmetric.ExponentialHistogramDataPoint, boundaries []float64) []uint64 {
	scale := int(dp.Scale())
	factor := math.Ldexp(math.Ln2, -scale)
	posB := dp.Positive().BucketCounts()
	bucketCounts := make([]uint64, len(boundaries)+1) // +1 for the overflow bucket

	for pos := 0; pos < posB.Len(); pos++ {
		index := dp.Positive().Offset() + int32(pos)
		// calculate the upper bound of the bucket
		upper := math.Exp(float64(index+1) * factor)
		count := posB.At(pos)

		// At this point we know that the upper bound represents the highest value that can be in this bucket, so we take the
		// upper bound and compare it to each of the explicit boundaries provided by the user until we find a boundary
		// that fits, that is, the first instance where upper bound <= explicit boundary.
		for j, boundary := range boundaries {
			if upper <= boundary {
				bucketCounts[j] += count
				break
			}
			if j == len(boundaries)-1 {
				bucketCounts[j+1] += count // Overflow bucket
			}
		}
	}

	return bucketCounts
}

// calculateBucketCountsWithMidpoint function calculates the bucket counts for a given exponential histogram data point.
// This algorithm is similar to calculateBucketCountsWithUpperBounds, but instead of using the upper bound of the bucket
// to determine the bucket, it uses the midpoint of the upper and lower bounds.
// The midpoint is calculated as (upper + lower) / 2.
func calculateBucketCountsWithMidpoint(dp pmetric.ExponentialHistogramDataPoint, boundaries []float64) []uint64 {
	scale := int(dp.Scale())
	factor := math.Ldexp(math.Ln2, -scale)
	posB := dp.Positive().BucketCounts()
	bucketCounts := make([]uint64, len(boundaries)+1) // +1 for the overflow bucket

	for pos := 0; pos < posB.Len(); pos++ {
		index := dp.Positive().Offset() + int32(pos)
		upper := math.Exp(float64(index+1) * factor)
		lower := math.Exp(float64(index) * factor)
		midpoint := (upper + lower) / 2
		count := posB.At(pos)

		for j, boundary := range boundaries {
			if midpoint <= boundary {
				bucketCounts[j] += count
				break
			}
			if j == len(boundaries)-1 {
				bucketCounts[j+1] += count // Overflow bucket
			}
		}
	}

	return bucketCounts
}

// calculateBucketCountsWithUniformDistribution distributes counts from an exponential histogram data point into a set of linear boundaries using uniform distribution
func calculateBucketCountsWithUniformDistribution(dp pmetric.ExponentialHistogramDataPoint, boundaries []float64) []uint64 {
	scale := int(dp.Scale())
	factor := math.Ldexp(math.Ln2, -scale)
	posB := dp.Positive().BucketCounts()
	bucketCounts := make([]uint64, len(boundaries)+1) // +1 for the overflow bucket

	for pos := 0; pos < posB.Len(); pos++ {
		index := dp.Positive().Offset() + int32(pos)
		lower := math.Exp(float64(index) * factor)
		upper := math.Exp(float64(index+1) * factor)
		count := posB.At(pos)

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

		// Distribute the count uniformly across the intersecting boundaries
		if end > start {
			countPerBoundary := count / uint64(end-start+1)
			remainder := count % uint64(end-start+1)

			for j := start; j <= end; j++ {
				bucketCounts[j] += countPerBoundary
				if remainder > 0 {
					bucketCounts[j]++
					remainder--
				}
			}
		} else {
			// Handle the case where the bucket range does not intersect with any boundaries
			bucketCounts[start] += count
		}
	}

	return bucketCounts
}

// calculateBucketCountsWithRandomDistribution distributes counts from an exponential histogram data point into a set of linear boundaries using random distribution
func calculateBucketCountsWithRandomDistribution(dp pmetric.ExponentialHistogramDataPoint, boundaries []float64) []uint64 {
	rand.Seed(uint64(time.Now().UnixNano())) // Seed the random number generator
	scale := int(dp.Scale())
	// factor is used to scale the exponential histogram
	factor := math.Ldexp(math.Ln2, -scale)
	posB := dp.Positive().BucketCounts()
	bucketCounts := make([]uint64, len(boundaries)+1) // +1 for the overflow bucket

	for pos := 0; pos < posB.Len(); pos++ {
		// Calculate the lower and upper bounds of the current bucket
		index := dp.Positive().Offset() + int32(pos)
		lower := math.Exp(float64(index) * factor)
		upper := math.Exp(float64(index+1) * factor)
		count := posB.At(pos)

		// Find the boundaries that intersect with the bucket range
		start := 0
		for start < len(boundaries) && boundaries[start] < lower {
			start++
		}
		end := start
		for end < len(boundaries) && boundaries[end] < upper {
			end++
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
				bucketCounts[j] += randomlyAllocatedCount
				totalAllocated += randomlyAllocatedCount
			}

			// Distribute any remaining count
			remainingCount := count - totalAllocated
			for remainingCount > 0 {
				randomBoundary := rand.Intn(end-start+1) + start
				bucketCounts[randomBoundary]++
				remainingCount--
			}
		} else {
			// If the bucket range does not intersect with any boundaries, assign the entire count to the start boundary
			bucketCounts[start] += count
		}
	}

	return bucketCounts
}
