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
package conversion

import (
	"errors"
	"fmt"
	"math"
	"math/bits"
	"math/rand/v2"
	"sort"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/expohisto/mapping"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/expohisto/mapping/exponent"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/expohisto/mapping/logarithm"
)

// Buckets is an exponential histogram magnitude bucket range.
type Buckets struct {
	Offset int32
	Counts []uint64
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
		if err := distribute(output, bounds, -input.ZeroThreshold, input.ZeroThreshold, input.ZeroCount, distribution); err != nil {
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
		for _, count := range buckets.Counts {
			var overflow bool
			total, overflow = addUint64(total, count)
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
	if len(buckets.Counts) == 0 {
		return nil
	}
	lower, err := mapper.LowerBoundary(buckets.Offset)
	if err != nil {
		return fmt.Errorf("bucket boundary index %d: %w", buckets.Offset, err)
	}
	for pos, count := range buckets.Counts {
		upperIndex := buckets.Offset + int32(pos) + 1
		upper, err := mapper.LowerBoundary(upperIndex)
		if errors.Is(err, mapping.ErrOverflow) && pos == len(buckets.Counts)-1 {
			upper = math.MaxFloat64
		} else if err != nil {
			return fmt.Errorf("bucket boundary index %d: %w", upperIndex, err)
		}
		if !(lower < upper) {
			return fmt.Errorf("invalid bucket interval (%v, %v]", lower, upper)
		}
		if count != 0 {
			bucketLower, bucketUpper := lower, upper
			if negative {
				// Negative buckets reverse the magnitude bounds.
				bucketLower, bucketUpper = -upper, -lower
			}
			if err := distribute(output, bounds, bucketLower, bucketUpper, count, distribution); err != nil {
				return err
			}
		}
		lower = upper
	}
	return nil
}

// distribute allocates one source bucket according to the selected distribution.
func distribute(output []uint64, bounds []float64, lower, upper float64, count uint64, distribution string) error {
	if lower == upper {
		return addToBucket(output, explicitBucket(bounds, lower), count)
	}
	switch distribution {
	case "upper":
		return addToBucket(output, explicitBucket(bounds, upper), count)
	case "midpoint":
		return addToBucket(output, explicitBucket(bounds, lower/2+upper/2), count)
	case "uniform":
		return distributeWeighted(output, bounds, lower, upper, count, math.MaxUint64)
	case "random":
		return distributeWeighted(output, bounds, lower, upper, count, rand.Uint64())
	default:
		panic("validated distribution")
	}
}

// distributeWeighted apportions a source count by linear overlap using systematic rounding.
func distributeWeighted(output []uint64, bounds []float64, lower, upper float64, count uint64, roundingOffset uint64) error {
	first := explicitBucket(bounds, lower)
	last := explicitBucket(bounds, upper)
	total := upper - lower
	if !(total > 0) || math.IsInf(total, 0) || math.IsNaN(total) {
		return fmt.Errorf("cannot distribute bucket interval (%v, %v]", lower, upper)
	}

	allocated := uint64(0)
	cumulative := float64(0)
	// Round cumulative expectations to preserve the exact total.
	for bucket := first; bucket <= last; bucket++ {
		bucketLower := math.Inf(-1)
		if bucket != 0 {
			bucketLower = bounds[bucket-1]
		}
		bucketUpper := math.Inf(1)
		if bucket != len(bounds) {
			bucketUpper = bounds[bucket]
		}
		overlapLower := max(lower, bucketLower)
		overlapUpper := min(upper, bucketUpper)
		if overlapLower >= overlapUpper {
			continue
		}

		var next uint64
		if bucket == last {
			next = count
		} else {
			cumulative += (overlapUpper - overlapLower) / total
			next = roundedPrefix(count, cumulative, roundingOffset)
			if next < allocated {
				next = allocated
			}
		}
		if err := addToBucket(output, bucket, next-allocated); err != nil {
			return err
		}
		allocated = next
	}
	return nil
}

// explicitBucket returns the index of the explicit bucket containing value.
func explicitBucket(bounds []float64, value float64) int {
	return sort.Search(len(bounds), func(i int) bool {
		return value <= bounds[i]
	})
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
