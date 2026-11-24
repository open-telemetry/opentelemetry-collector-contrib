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

package conversion

import (
	"math"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToExplicitAlignedBuckets(t *testing.T) {
	for _, distribution := range distributions {
		counts, err := ToExplicit(ExponentialHistogram{
			Count: 8,
			Scale: 0,
			Positive: Buckets{
				Counts: bucketCounts{8},
			},
		}, []float64{1, 2}, distribution)
		require.NoError(t, err)
		assert.Equal(t, []uint64{0, 8, 0}, counts)
	}
}

func TestToExplicitAllRanges(t *testing.T) {
	counts, err := ToExplicit(ExponentialHistogram{
		Count:         16,
		Scale:         0,
		ZeroThreshold: 1,
		ZeroCount:     8,
		Positive: Buckets{
			Counts: bucketCounts{4},
		},
		Negative: Buckets{
			Counts: bucketCounts{4},
		},
	}, []float64{-2, -1, 0, 1, 2}, "uniform")
	require.NoError(t, err)
	assert.Equal(t, []uint64{0, 4, 4, 4, 4, 0}, counts)
}

func TestToExplicitOverflow(t *testing.T) {
	for _, distribution := range distributions {
		counts, err := ToExplicit(ExponentialHistogram{
			Count: 5,
			Scale: 0,
			Positive: Buckets{
				Offset: 10,
				Counts: bucketCounts{5},
			},
		}, []float64{1, 2, 3}, distribution)
		require.NoError(t, err)
		assert.Equal(t, []uint64{0, 0, 0, 5}, counts)
	}
}

func TestToExplicitZeroPoint(t *testing.T) {
	for _, distribution := range distributions {
		counts, err := ToExplicit(ExponentialHistogram{
			Count:     3,
			Scale:     0,
			ZeroCount: 3,
		}, []float64{-1, 0, 1}, distribution)
		require.NoError(t, err)
		assert.Equal(t, []uint64{0, 3, 0, 0}, counts)
	}
}

func TestToExplicitConservesCounts(t *testing.T) {
	input := ExponentialHistogram{
		Count:         39,
		Scale:         3,
		ZeroThreshold: 0.01,
		ZeroCount:     7,
		Positive: Buckets{
			Offset: -5,
			Counts: bucketCounts{2, 3, 5, 7},
		},
		Negative: Buckets{
			Offset: -2,
			Counts: bucketCounts{4, 5, 6},
		},
	}
	bounds := []float64{-10, -1, -0.1, 0, 0.1, 1, 10}
	for _, distribution := range distributions {
		counts, err := ToExplicit(input, bounds, distribution)
		require.NoError(t, err)
		require.Len(t, counts, len(bounds)+1)
		var total uint64
		for _, count := range counts {
			total += count
		}
		assert.Equal(t, input.Count, total)
	}
}

func TestToExplicitSignSymmetry(t *testing.T) {
	bounds := []float64{-4, -2, -1, 0, 1, 2, 4}
	positive, err := ToExplicit(ExponentialHistogram{
		Count: 9,
		Scale: 0,
		Positive: Buckets{
			Counts: bucketCounts{4, 5},
		},
	}, bounds, "uniform")
	require.NoError(t, err)
	negative, err := ToExplicit(ExponentialHistogram{
		Count: 9,
		Scale: 0,
		Negative: Buckets{
			Counts: bucketCounts{4, 5},
		},
	}, bounds, "uniform")
	require.NoError(t, err)
	slices.Reverse(negative)
	assert.Equal(t, positive, negative)
}

func TestToExplicitGeneratedInvariants(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	for range 1000 {
		scale := int32(rng.IntN(31) - 10)
		maxBuckets := 8
		offset := int32(rng.IntN(8) - 4)
		if scale < -6 {
			maxBuckets = 1
			offset = 0
		}
		positive := make([]uint64, rng.IntN(maxBuckets+1))
		negative := make([]uint64, rng.IntN(maxBuckets+1))
		zeroCount := rng.Uint64N(100)
		count := zeroCount
		for i := range positive {
			positive[i] = rng.Uint64N(100)
			count += positive[i]
		}
		for i := range negative {
			negative[i] = rng.Uint64N(100)
			count += negative[i]
		}

		bounds := make([]float64, rng.IntN(20)+1)
		for i := range bounds {
			bounds[i] = float64(i*10+rng.IntN(9)) - float64(len(bounds)*5)
		}
		slices.Sort(bounds)

		input := ExponentialHistogram{
			Count:         count,
			Scale:         scale,
			ZeroThreshold: rng.Float64(),
			ZeroCount:     zeroCount,
			Positive: Buckets{
				Offset: offset,
				Counts: bucketCounts(positive),
			},
			Negative: Buckets{
				Offset: offset,
				Counts: bucketCounts(negative),
			},
		}
		for _, distribution := range distributions {
			counts, err := ToExplicit(input, bounds, distribution)
			require.NoError(t, err)
			require.Len(t, counts, len(bounds)+1)
			var actual uint64
			for _, bucketCount := range counts {
				actual += bucketCount
			}
			assert.Equal(t, count, actual)
		}
	}
}

func TestSystematicRounding(t *testing.T) {
	const count = uint64(101)
	probabilities := []float64{0.1, 0.2, 0.3, 0.4}
	sums := make([]uint64, len(probabilities))
	const trials = 4096
	for trial := range trials {
		roundingOffset := uint64(trial) << 52
		var cumulative float64
		var previous uint64
		for i, probability := range probabilities {
			var next uint64
			if i == len(probabilities)-1 {
				next = count
			} else {
				cumulative += probability
				next = roundedPrefix(count, cumulative, roundingOffset)
			}
			sums[i] += next - previous
			previous = next
		}
	}
	for i, probability := range probabilities {
		mean := float64(sums[i]) / trials
		assert.InDelta(t, float64(count)*probability, mean, 0.01)
	}
}

func TestToExplicitValidation(t *testing.T) {
	tests := []struct {
		name         string
		input        ExponentialHistogram
		bounds       []float64
		distribution string
	}{
		{
			name:         "empty bounds",
			input:        ExponentialHistogram{Scale: 0},
			distribution: "random",
		},
		{
			name:         "unordered bounds",
			input:        ExponentialHistogram{Scale: 0},
			bounds:       []float64{2, 1},
			distribution: "random",
		},
		{
			name:         "NaN bound",
			input:        ExponentialHistogram{Scale: 0},
			bounds:       []float64{math.NaN()},
			distribution: "random",
		},
		{
			name:         "invalid scale",
			input:        ExponentialHistogram{Scale: 21},
			bounds:       []float64{1},
			distribution: "random",
		},
		{
			name:         "count mismatch",
			input:        ExponentialHistogram{Count: 2, Scale: 0, ZeroCount: 1},
			bounds:       []float64{1},
			distribution: "random",
		},
		{
			name:         "invalid distribution",
			input:        ExponentialHistogram{Scale: 0},
			bounds:       []float64{1},
			distribution: "invalid",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ToExplicit(test.input, test.bounds, test.distribution)
			require.Error(t, err)
		})
	}
}

var distributions = []string{"upper", "midpoint", "uniform", "random"}

type bucketCounts []uint64

func (c bucketCounts) Len() int {
	return len(c)
}

func (c bucketCounts) At(i int) uint64 {
	return c[i]
}
