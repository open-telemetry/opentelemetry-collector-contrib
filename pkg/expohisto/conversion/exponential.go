// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package conversion translates exponential histogram buckets into other
// histogram bucket layouts.
package conversion // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/expohisto/conversion"

import (
	"errors"
	"fmt"
	"math"
	"math/bits"
	"math/rand/v2"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/expohisto/mapping"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/expohisto/mapping/exponent"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/expohisto/mapping/logarithm"
)

// BucketCounts provides read-only access to exponential histogram bucket counts.
type BucketCounts interface {
	Len() int
	At(int) uint64
}

// Buckets is an exponential histogram magnitude bucket range.
type Buckets struct {
	Offset int32
	Counts BucketCounts
}

// ExponentialHistogram contains the fields needed to translate an exponential
// histogram data point.
type ExponentialHistogram struct {
	Count         uint64
	Scale         int32
	ZeroThreshold float64
	ZeroCount     uint64
	Positive      Buckets
	Negative      Buckets
}

// ToExplicit converts an exponential histogram into len(bounds)+1 explicit bucket counts using the requested distribution.
func ToExplicit(input ExponentialHistogram, bounds []float64, distribution string) ([]uint64, error) {
	if err := validateBounds(bounds); err != nil {
		return nil, err
	}
	if !validDistribution(distribution) {
		return nil, fmt.Errorf("invalid distribution %q", distribution)
	}
	if math.IsNaN(input.ZeroThreshold) || math.IsInf(input.ZeroThreshold, 0) || input.ZeroThreshold < 0 {
		return nil, fmt.Errorf("invalid zero threshold: %v", input.ZeroThreshold)
	}

	mapper, err := newMapping(input.Scale)
	if err != nil {
		return nil, fmt.Errorf("invalid exponential histogram scale %d: %w", input.Scale, err)
	}

	totalCount, err := sourceBucketCount(input)
	if err != nil {
		return nil, err
	}
	if totalCount != input.Count {
		return nil, fmt.Errorf("source histogram count %d does not match bucket count %d", input.Count, totalCount)
	}

	output := make([]uint64, len(bounds)+1)
	if input.ZeroCount != 0 {
		destination := 0
		if err := distribute(output, bounds, &destination, -input.ZeroThreshold, input.ZeroThreshold, input.ZeroCount, distribution); err != nil {
			return nil, err
		}
	}

	if err := distributeBuckets(mapper, output, bounds, input.Negative, distribution, true); err != nil {
		return nil, fmt.Errorf("negative buckets: %w", err)
	}
	if err := distributeBuckets(mapper, output, bounds, input.Positive, distribution, false); err != nil {
		return nil, fmt.Errorf("positive buckets: %w", err)
	}

	return output, nil
}

// sourceBucketCount sums all source buckets and reports integer overflow.
func sourceBucketCount(input ExponentialHistogram) (uint64, error) {
	total := input.ZeroCount
	for _, buckets := range [...]Buckets{input.Negative, input.Positive} {
		if buckets.Counts == nil {
			continue
		}
		for i := 0; i < buckets.Counts.Len(); i++ {
			var overflow bool
			total, overflow = addUint64(total, buckets.Counts.At(i))
			if overflow {
				return 0, errors.New("source histogram bucket count overflow")
			}
		}
	}
	return total, nil
}

// validDistribution reports whether distribution names a supported allocation policy.
func validDistribution(distribution string) bool {
	switch distribution {
	case "upper", "midpoint", "uniform", "random":
		return true
	default:
		return false
	}
}

// validateBounds checks that bounds is non-empty, finite or infinite, and strictly increasing.
func validateBounds(bounds []float64) error {
	if len(bounds) == 0 {
		return errors.New("explicit bounds cannot be empty")
	}
	for i, bound := range bounds {
		if math.IsNaN(bound) {
			return fmt.Errorf("explicit bound %d is NaN", i)
		}
		if i != 0 && bound <= bounds[i-1] {
			return fmt.Errorf("explicit bounds are not strictly increasing at index %d: %v <= %v", i, bound, bounds[i-1])
		}
	}
	return nil
}

// newMapping constructs the exponential histogram mapping for scale.
func newMapping(scale int32) (mapping.Mapping, error) {
	if scale <= exponent.MaxScale {
		return exponent.NewMapping(scale)
	}
	return logarithm.NewMapping(scale)
}

// distributeBuckets maps one positive or negative exponential bucket range into explicit buckets.
func distributeBuckets(mapper mapping.Mapping, output []uint64, bounds []float64, buckets Buckets, distribution string, negative bool) error {
	if buckets.Counts == nil || buckets.Counts.Len() == 0 {
		return nil
	}
	destination := 0
	position := 0
	step := 1
	boundaryIndex := buckets.Offset
	if negative {
		position = buckets.Counts.Len() - 1
		step = -1
		boundaryIndex += int32(buckets.Counts.Len())
	}

	boundary, err := bucketBoundary(mapper, boundaryIndex, negative)
	if err != nil {
		return err
	}
	for remaining := buckets.Counts.Len(); remaining > 0; remaining-- {
		nextIndex := boundaryIndex + int32(step)
		next, err := bucketBoundary(mapper, nextIndex, !negative && remaining == 1)
		if err != nil {
			return err
		}
		lower, upper := boundary, next
		if negative {
			lower, upper = -boundary, -next
		}
		if count := buckets.Counts.At(position); count != 0 {
			if err := distribute(output, bounds, &destination, lower, upper, count, distribution); err != nil {
				return err
			}
		}
		position += step
		boundaryIndex = nextIndex
		boundary = next
	}
	return nil
}

// bucketBoundary returns one mapped boundary and optionally substitutes the largest finite value for overflow.
func bucketBoundary(mapper mapping.Mapping, index int32, allowOverflow bool) (float64, error) {
	boundary, err := mapper.LowerBoundary(index)
	if allowOverflow && errors.Is(err, mapping.ErrOverflow) {
		return math.MaxFloat64, nil
	}
	if err != nil {
		return 0, fmt.Errorf("bucket boundary index %d: %w", index, err)
	}
	return boundary, nil
}

// distribute allocates one source bucket according to the selected distribution.
func distribute(output []uint64, bounds []float64, destination *int, lower, upper float64, count uint64, distribution string) error {
	if lower > upper {
		return fmt.Errorf("invalid bucket interval (%v, %v]", lower, upper)
	}
	if lower == upper {
		return distributePoint(output, bounds, destination, lower, count)
	}
	switch distribution {
	case "upper":
		return distributePoint(output, bounds, destination, upper, count)
	case "midpoint":
		return distributePoint(output, bounds, destination, lower/2+upper/2, count)
	case "uniform":
		return distributeWeighted(output, bounds, destination, lower, upper, count, false)
	case "random":
		return distributeWeighted(output, bounds, destination, lower, upper, count, true)
	default:
		panic("validated distribution")
	}
}

// distributePoint places a point mass in its explicit bucket while advancing the sweep cursor.
func distributePoint(output []uint64, bounds []float64, destination *int, value float64, count uint64) error {
	for *destination < len(bounds) && value > bounds[*destination] {
		(*destination)++
	}
	return addToBucket(output, *destination, count)
}

// distributeWeighted apportions a source count by linear overlap using systematic rounding.
func distributeWeighted(output []uint64, bounds []float64, destination *int, lower, upper float64, count uint64, randomized bool) error {
	total := upper - lower
	if !(total > 0) || math.IsInf(total, 0) || math.IsNaN(total) {
		return fmt.Errorf("cannot distribute bucket interval (%v, %v]", lower, upper)
	}

	for *destination < len(bounds) && lower >= bounds[*destination] {
		(*destination)++
	}
	destinationUpper := explicitUpperBound(bounds, *destination)
	if upper <= destinationUpper {
		return addToBucket(output, *destination, count)
	}

	roundingOffset := uint64(math.MaxUint64)
	if randomized {
		roundingOffset = rand.Uint64()
	}
	allocated := uint64(0)
	cumulative := float64(0)
	// Round cumulative expectations to preserve the exact total.
	overlapLower := lower
	for {
		overlapUpper := min(upper, destinationUpper)
		var next uint64
		if overlapUpper == upper {
			next = count
		} else {
			cumulative += (overlapUpper - overlapLower) / total
			next = roundedPrefix(count, cumulative, roundingOffset)
			next = max(next, allocated)
		}
		if err := addToBucket(output, *destination, next-allocated); err != nil {
			return err
		}
		if overlapUpper == upper {
			return nil
		}
		allocated = next
		overlapLower = overlapUpper
		(*destination)++
		destinationUpper = explicitUpperBound(bounds, *destination)
	}
}

// explicitUpperBound returns the numeric upper bound of an explicit bucket.
func explicitUpperBound(bounds []float64, bucket int) float64 {
	if bucket == len(bounds) {
		return math.Inf(1)
	}
	return bounds[bucket]
}

// roundedPrefix rounds a cumulative expected count using a fixed-point offset.
func roundedPrefix(count uint64, cumulative float64, roundingOffset uint64) uint64 {
	if cumulative <= 0 {
		return 0
	}
	if cumulative >= 1 {
		return count
	}
	scaled := math.Ldexp(cumulative, 64)
	var fraction uint64
	if scaled >= 0x1p+64 {
		fraction = math.MaxUint64
	} else {
		fraction = uint64(scaled)
	}
	high, low := bits.Mul64(count, fraction)
	_, carry := bits.Add64(low, roundingOffset, 0)
	if carry != 0 && high != math.MaxUint64 {
		high++
	}
	return min(high, count)
}

// addToBucket adds count to one output bucket and reports integer overflow.
func addToBucket(output []uint64, bucket int, count uint64) error {
	sum, overflow := addUint64(output[bucket], count)
	if overflow {
		return fmt.Errorf("explicit histogram bucket %d count overflow", bucket)
	}
	output[bucket] = sum
	return nil
}

// addUint64 returns the sum and whether unsigned addition overflowed.
func addUint64(left, right uint64) (uint64, bool) {
	sum, carry := bits.Add64(left, right, 0)
	return sum, carry != 0
}
