// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
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

	for range iterations {
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

	maxCount := uint64(6742351) // make up a number
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := stochasticDiv(maxCount, uint64(float64(maxCount)/tt.weight))
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

	maxCount := uint64(6742351) // make up a number

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Run multiple times to ensure we're getting valid results
			for range 100 {
				result := stochasticDiv(maxCount, uint64(float64(maxCount)/tt.weight))
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

	maxCount := uint64(6742351) // make up a number

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var total uint64
			for range tt.iterations {
				total += stochasticDiv(maxCount, uint64(float64(maxCount)/tt.weight))
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
	for range 1000 {
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

	for g := range goroutines {
		results[g] = make([]uint64, iterationsPerGoroutine)
		go func(idx int) {
			defer wg.Done()
			for i := range iterationsPerGoroutine {
				results[idx][i] = stochasticDiv(3, 2) // Should be 1 or 2
			}
		}(g)
	}

	wg.Wait()

	// Verify all results are valid (either 1 or 2)
	for g := range goroutines {
		for i := range iterationsPerGoroutine {
			assert.True(t, results[g][i] == 1 || results[g][i] == 2,
				"result should be 1 or 2, got %d", results[g][i])
		}
	}
}

func TestStochasticDiv_EdgeCases(t *testing.T) {
	t.Run("very small fraction", func(t *testing.T) {
		// With a very small fraction (1/1000 = 0.001), we should almost always get the floor
		const iterations = 1000
		var ones int
		for range iterations {
			if stochasticDiv(1, 1000) == 1 {
				ones++
			}
		}
		// Should be roughly 0.1% ones, so expect less than 20 (allowing for variance)
		assert.Less(t, ones, 20, "very small fraction should rarely round up")
	})

	t.Run("very large fraction", func(t *testing.T) {
		// With a fraction close to 2 (1999/1000 = 1.999), we should almost always round up
		const iterations = 1000
		var twos int
		for range iterations {
			if stochasticDiv(1999, 1000) == 2 {
				twos++
			}
		}
		// Should be roughly 99.9% twos, so expect more than 980
		assert.Greater(t, twos, 980, "large fraction should usually round up")
	})

	t.Run("large integer quotient", func(t *testing.T) {
		result := stochasticDiv(1000000, 1)
		assert.Equal(t, uint64(1000000), result)
	})

	t.Run("tiny remainder", func(t *testing.T) {
		// Test with smallest possible non-zero remainder relative to denominator
		// 1000001 / 1000000 = 1 remainder 1, so rounds up with 0.0001% probability
		result := stochasticDiv(1000001, 1000000)
		require.True(t, result == 1 || result == 2)
	})
}

// Benchmarks for GetStochasticAdjustedCount

func BenchmarkGetStochasticAdjustedCount_ValidTracestate(b *testing.B) {
	// Benchmark with a valid tracestate containing th:8 (50% sampling, adjusted count = 2)
	span := ptrace.NewSpan()
	span.TraceState().FromRaw("ot=th:8")

	b.ResetTimer()
	for b.Loop() {
		_, _ = GetStochasticAdjustedCount(&span)
	}
}

func BenchmarkGetStochasticAdjustedCount_EmptyTracestate(b *testing.B) {
	// Benchmark with empty tracestate (fast path - returns early)
	span := ptrace.NewSpan()

	b.ResetTimer()
	for b.Loop() {
		_, _ = GetStochasticAdjustedCount(&span)
	}
}

func BenchmarkGetStochasticAdjustedCount_NoThreshold(b *testing.B) {
	// Benchmark with valid tracestate but no threshold (returns early after parsing)
	span := ptrace.NewSpan()
	span.TraceState().FromRaw("ot=rv:abcdabcdabcdff")

	b.ResetTimer()
	for b.Loop() {
		_, _ = GetStochasticAdjustedCount(&span)
	}
}

func BenchmarkGetStochasticAdjustedCount_ComplexTracestate(b *testing.B) {
	// Benchmark with a more complex tracestate containing multiple fields
	span := ptrace.NewSpan()
	span.TraceState().FromRaw("ot=th:c;rv:abcdabcdabcdff,vendor1=value1,vendor2=value2")

	b.ResetTimer()
	for b.Loop() {
		_, _ = GetStochasticAdjustedCount(&span)
	}
}

func BenchmarkGetStochasticAdjustedCount_Parallel(b *testing.B) {
	// Benchmark parallel access to test prngPool performance
	span := ptrace.NewSpan()
	span.TraceState().FromRaw("ot=th:8")

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		// Each goroutine gets its own span to avoid contention on span access
		localSpan := ptrace.NewSpan()
		localSpan.TraceState().FromRaw("ot=th:8")
		for pb.Next() {
			_, _ = GetStochasticAdjustedCount(&localSpan)
		}
	})
}

func BenchmarkStochasticDiv_IntegerQuotient(b *testing.B) {
	// Benchmark stochasticDiv with integer quotient (no random needed)
	b.ResetTimer()
	for b.Loop() {
		_ = stochasticDiv(4, 1)
	}
}

func BenchmarkStochasticDiv_FractionalQuotient(b *testing.B) {
	// Benchmark stochasticDiv with fractional quotient (requires PRNG)
	b.ResetTimer()
	for b.Loop() {
		_ = stochasticDiv(9, 2) // 4.5
	}
}

func BenchmarkStochasticDiv_Parallel(b *testing.B) {
	// Benchmark parallel stochasticDiv to test prngPool under load
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = stochasticDiv(9, 2) // 4.5
		}
	})
}

func BenchmarkXorshift64star(b *testing.B) {
	// Benchmark the raw PRNG performance
	var rng xorshift64star = 12345
	b.ResetTimer()
	for b.Loop() {
		_ = rng.next()
	}
}

func BenchmarkGetStochasticAdjustedCount(b *testing.B) {
	// Create spans with realistic tracestates
	spans := make([]ptrace.Span, 100)
	for i := range spans {
		span := ptrace.NewSpan()
		span.TraceState().FromRaw("ot=th:c;rv:d29d6a7215ced0")
		spans[i] = span
	}

	b.Run("NoCache", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			for j := range spans {
				GetStochasticAdjustedCount(&spans[j])
			}
		}
	})

	b.Run("WithCache", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			cache := NewAdjustedCountCache()
			for j := range spans {
				GetStochasticAdjustedCountWithCache(&spans[j], &cache)
			}
		}
	})
}

func BenchmarkAdjustedCountCache(b *testing.B) {
	span := ptrace.NewSpan()
	span.TraceState().FromRaw("ot=th:c;rv:d29d6a7215ced0;pn:abc,zz=vendorcontent")

	b.Run("CacheHit", func(b *testing.B) {
		cache := NewAdjustedCountCache()
		// Prime the cache
		GetStochasticAdjustedCountWithCache(&span, &cache)

		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			GetStochasticAdjustedCountWithCache(&span, &cache)
		}
	})

	b.Run("CacheMiss", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			// Force cache miss by using fresh cache each time
			cache := NewAdjustedCountCache()
			GetStochasticAdjustedCountWithCache(&span, &cache)
		}
	})
}
