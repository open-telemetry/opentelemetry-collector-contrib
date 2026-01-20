// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"math"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func TestGetStochasticAdjustedCount(t *testing.T) {
	tests := []struct {
		name           string
		tracestate     string
		expectAdjusted bool
	}{
		{
			name:           "empty tracestate",
			tracestate:     "",
			expectAdjusted: false,
		},
		{
			name:           "invalid tracestate",
			tracestate:     "invalid",
			expectAdjusted: false,
		},
		{
			name:           "tracestate without ot field",
			tracestate:     "other=value",
			expectAdjusted: false,
		},
		{
			name:           "tracestate with ot but no th field",
			tracestate:     "ot=rv:abcdabcdabcdff",
			expectAdjusted: false,
		},
		{
			name:           "valid tracestate with th:1 (100% sampling, adjusted count ~1)",
			tracestate:     "ot=th:1",
			expectAdjusted: true,
		},
		{
			name:           "valid tracestate with th:c (25% sampling, adjusted count ~4)",
			tracestate:     "ot=th:c",
			expectAdjusted: true,
		},
		{
			name:           "valid tracestate with th:8 (50% sampling, adjusted count ~2)",
			tracestate:     "ot=th:8",
			expectAdjusted: true,
		},
		{
			name:           "valid tracestate with other fields",
			tracestate:     "ot=th:8,other=value",
			expectAdjusted: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			span := ptrace.NewSpan()
			span.TraceState().FromRaw(tt.tracestate)

			count, adjusted := GetStochasticAdjustedCount(&span)

			assert.Equal(t, tt.expectAdjusted, adjusted, "adjusted flag mismatch")
			if tt.expectAdjusted {
				assert.GreaterOrEqual(t, count, uint64(1), "adjusted count should be >= 1 when adjusted is true")
			} else {
				assert.Equal(t, uint64(1), count, "count should be 1 (representing the span itself) when adjusted is false")
			}
		})
	}
}

func TestGetStochasticAdjustedCount_StatisticalBehavior(t *testing.T) {
	// Test that over many iterations, the average adjusted count
	// converges to the expected value for th:c (adjusted count = 4.0)
	const iterations = 10000
	var total uint64
	var adjustedCount int

	for i := 0; i < iterations; i++ {
		span := ptrace.NewSpan()
		span.TraceState().FromRaw("ot=th:c")
		count, adjusted := GetStochasticAdjustedCount(&span)
		if adjusted {
			total += count
			adjustedCount++
		}
	}

	// All iterations should have adjusted=true for valid tracestate
	assert.Equal(t, iterations, adjustedCount, "all iterations should return adjusted=true")

	average := float64(total) / float64(iterations)
	// Expected adjusted count for th:c is 4.0
	// Allow 10% tolerance for statistical variation
	assert.InDelta(t, 4.0, average, 0.4, "average adjusted count should be close to 4.0")
}

func TestStochasticIncrement(t *testing.T) {
	tests := []struct {
		name     string
		weight   float64
		expected uint64
	}{
		{
			name:     "zero weight",
			weight:   0,
			expected: 0,
		},
		{
			name:     "integer weight 1",
			weight:   1.0,
			expected: 1,
		},
		{
			name:     "integer weight 5",
			weight:   5.0,
			expected: 5,
		},
		{
			name:     "integer weight 100",
			weight:   100.0,
			expected: 100,
		},
		{
			name:     "negative zero fraction",
			weight:   3.0,
			expected: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := stochasticIncrement(tt.weight)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestStochasticIncrement_FractionalWeights(t *testing.T) {
	// For fractional weights, the result should be either floor or ceil
	tests := []struct {
		name      string
		weight    float64
		minResult uint64
		maxResult uint64
	}{
		{
			name:      "weight 0.5",
			weight:    0.5,
			minResult: 0,
			maxResult: 1,
		},
		{
			name:      "weight 2.7",
			weight:    2.7,
			minResult: 2,
			maxResult: 3,
		},
		{
			name:      "weight 10.1",
			weight:    10.1,
			minResult: 10,
			maxResult: 11,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Run multiple times to ensure we're getting valid results
			for i := 0; i < 100; i++ {
				result := stochasticIncrement(tt.weight)
				assert.GreaterOrEqual(t, result, tt.minResult)
				assert.LessOrEqual(t, result, tt.maxResult)
			}
		})
	}
}

func TestStochasticIncrement_StatisticalConvergence(t *testing.T) {
	// Test that over many iterations, the average converges to the weight
	tests := []struct {
		name           string
		weight         float64
		iterations     int
		toleranceRatio float64
	}{
		{
			name:           "weight 0.3",
			weight:         0.3,
			iterations:     10000,
			toleranceRatio: 0.1,
		},
		{
			name:           "weight 2.5",
			weight:         2.5,
			iterations:     10000,
			toleranceRatio: 0.05,
		},
		{
			name:           "weight 7.8",
			weight:         7.8,
			iterations:     10000,
			toleranceRatio: 0.05,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var total uint64
			for i := 0; i < tt.iterations; i++ {
				total += stochasticIncrement(tt.weight)
			}
			average := float64(total) / float64(tt.iterations)
			tolerance := tt.weight * tt.toleranceRatio
			assert.InDelta(t, tt.weight, average, tolerance,
				"average should converge to weight %.2f", tt.weight)
		})
	}
}

func TestXorshift64star(t *testing.T) {
	// Test that the PRNG produces non-zero values and different values on consecutive calls
	var rng xorshift64star = 12345 // Non-zero seed

	values := make(map[uint64]struct{})
	for i := 0; i < 1000; i++ {
		val := rng.next()
		assert.NotEqual(t, uint64(0), val, "PRNG should not produce zero")
		values[val] = struct{}{}
	}

	// Should have many unique values (allowing for some small probability of collision)
	assert.Greater(t, len(values), 990, "PRNG should produce mostly unique values")
}

func TestXorshift64star_DifferentSeeds(t *testing.T) {
	// Test that different seeds produce different sequences
	var rng1 xorshift64star = 1
	var rng2 xorshift64star = 2

	// Get first value from each
	val1 := rng1.next()
	val2 := rng2.next()

	assert.NotEqual(t, val1, val2, "different seeds should produce different values")
}

func TestPrngPool_Concurrency(t *testing.T) {
	// Test that the PRNG pool works correctly under concurrent access
	const goroutines = 100
	const iterationsPerGoroutine = 1000

	var wg sync.WaitGroup
	wg.Add(goroutines)

	results := make([][]uint64, goroutines)

	for g := 0; g < goroutines; g++ {
		results[g] = make([]uint64, iterationsPerGoroutine)
		go func(idx int) {
			defer wg.Done()
			for i := 0; i < iterationsPerGoroutine; i++ {
				results[idx][i] = stochasticIncrement(1.5) // Should be 1 or 2
			}
		}(g)
	}

	wg.Wait()

	// Verify all results are valid (either 1 or 2)
	for g := 0; g < goroutines; g++ {
		for i := 0; i < iterationsPerGoroutine; i++ {
			assert.True(t, results[g][i] == 1 || results[g][i] == 2,
				"result should be 1 or 2, got %d", results[g][i])
		}
	}
}

func TestStochasticIncrement_EdgeCases(t *testing.T) {
	t.Run("very small fraction", func(t *testing.T) {
		// With a very small fraction, we should almost always get the floor
		const iterations = 1000
		var ones int
		for i := 0; i < iterations; i++ {
			if stochasticIncrement(0.001) == 1 {
				ones++
			}
		}
		// Should be roughly 0.1% ones, so expect less than 5 (allowing for variance)
		assert.Less(t, ones, 20, "very small fraction should rarely round up")
	})

	t.Run("very large fraction", func(t *testing.T) {
		// With a fraction close to 1, we should almost always round up
		const iterations = 1000
		var twos int
		for i := 0; i < iterations; i++ {
			if stochasticIncrement(1.999) == 2 {
				twos++
			}
		}
		// Should be roughly 99.9% twos, so expect more than 980
		assert.Greater(t, twos, 980, "large fraction should usually round up")
	})

	t.Run("large integer weight", func(t *testing.T) {
		result := stochasticIncrement(1000000.0)
		assert.Equal(t, uint64(1000000), result)
	})

	t.Run("weight with max float precision", func(t *testing.T) {
		// Test that we handle floating point precision correctly
		weight := 1.0 + math.SmallestNonzeroFloat64
		result := stochasticIncrement(weight)
		require.True(t, result == 1 || result == 2)
	})
}
