// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dns

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/lookupprocessor/lookupsource"
)

// BenchmarkDNSLookup benchmarks DNS lookup performance.
// These are integration tests that require network access.
// Run with: go test -bench=. -benchtime=10s
func BenchmarkDNSLookup(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping DNS benchmark in short mode")
	}

	b.Run("PTR_no_cache", func(b *testing.B) {
		source := createBenchmarkSource(b, false)
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			// Use Google's DNS which has PTR records
			_, _, _ = source.Lookup(b.Context(), "8.8.8.8")
		}
	})

	b.Run("PTR_with_cache", func(b *testing.B) {
		source := createBenchmarkSource(b, true)
		// Prime the cache
		_, _, _ = source.Lookup(b.Context(), "8.8.8.8")
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			_, _, _ = source.Lookup(b.Context(), "8.8.8.8")
		}
	})
}

// BenchmarkDNSLookupParallel benchmarks concurrent DNS lookups.
func BenchmarkDNSLookupParallel(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping DNS benchmark in short mode")
	}

	b.Run("with_cache", func(b *testing.B) {
		source := createBenchmarkSource(b, true)
		// Prime the cache
		_, _, _ = source.Lookup(b.Context(), "8.8.8.8")

		b.ReportAllocs()
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_, _, _ = source.Lookup(b.Context(), "8.8.8.8")
			}
		})
	})
}

// BenchmarkCacheEffectiveness measures the performance difference between
// cached and uncached DNS lookups.
func BenchmarkCacheEffectiveness(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping DNS benchmark in short mode")
	}

	uncached := createBenchmarkSource(b, false)
	cached := createBenchmarkSource(b, true)

	// Prime the cache
	_, _, _ = cached.Lookup(b.Context(), "8.8.8.8")

	b.Run("uncached", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			_, _, _ = uncached.Lookup(b.Context(), "8.8.8.8")
		}
	})

	b.Run("cached", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			_, _, _ = cached.Lookup(b.Context(), "8.8.8.8")
		}
	})
}

func createBenchmarkSource(b *testing.B, cacheEnabled bool) lookupsource.Source {
	b.Helper()

	factory := NewFactory()
	cfg := &Config{
		RecordType: RecordTypePTR,
		Timeout:    5 * time.Second,
		Cache: lookupsource.CacheConfig{
			Enabled:     cacheEnabled,
			Size:        10000,
			TTL:         5 * time.Minute,
			NegativeTTL: 1 * time.Minute,
		},
	}

	source, err := factory.CreateSource(b.Context(), lookupsource.CreateSettings{}, cfg)
	require.NoError(b, err)

	return source
}
