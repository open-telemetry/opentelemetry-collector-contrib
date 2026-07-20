// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package lookupsource

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func BenchmarkCacheSet(b *testing.B) {
	b.Run("new_entry", func(b *testing.B) {
		cache := NewCache(CacheConfig{
			Enabled: true,
			Size:    b.N + 100,
			TTL:     5 * time.Minute,
		})
		b.ReportAllocs()
		b.ResetTimer()
		for i := range b.N {
			cache.set(fmt.Sprintf("key%d", i), "value", true)
		}
	})

	b.Run("update_existing", func(b *testing.B) {
		cache := NewCache(CacheConfig{
			Enabled: true,
			Size:    1000,
			TTL:     5 * time.Minute,
		})
		cache.set("key", "initial", true)
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			cache.set("key", "updated", true)
		}
	})

	b.Run("with_eviction", func(b *testing.B) {
		cache := NewCache(CacheConfig{
			Enabled: true,
			Size:    100,
			TTL:     5 * time.Minute,
		})
		// Fill cache
		for i := range 100 {
			cache.set(fmt.Sprintf("pre%d", i), "value", true)
		}
		b.ReportAllocs()
		b.ResetTimer()
		for i := range b.N {
			cache.set(fmt.Sprintf("key%d", i), "value", true)
		}
	})
}

func BenchmarkCacheGet(b *testing.B) {
	cache := NewCache(CacheConfig{
		Enabled:     true,
		Size:        10000,
		TTL:         5 * time.Minute,
		NegativeTTL: 1 * time.Minute,
	})

	// Pre-populate with positive and negative entries
	for i := range 500 {
		cache.set(fmt.Sprintf("found%d", i), fmt.Sprintf("value%d", i), true)
		cache.set(fmt.Sprintf("notfound%d", i), nil, false)
	}

	b.Run("positive_hit", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			_, _, _ = cache.get("found250")
		}
	})

	b.Run("negative_hit", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			_, _, _ = cache.get("notfound250")
		}
	})

	b.Run("miss", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			_, _, _ = cache.get("nonexistent")
		}
	})
}

func BenchmarkWrapWithCache(b *testing.B) {
	lookupFn := func(_ context.Context, key string) (any, bool, error) {
		return "value-" + key, true, nil
	}

	b.Run("disabled", func(b *testing.B) {
		cache := NewCache(CacheConfig{Enabled: false})
		wrapped := WrapWithCache(cache, lookupFn)
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			_, _, _ = wrapped(b.Context(), "key")
		}
	})

	b.Run("enabled_miss", func(b *testing.B) {
		cache := NewCache(CacheConfig{
			Enabled: true,
			Size:    b.N + 100,
			TTL:     5 * time.Minute,
		})
		wrapped := WrapWithCache(cache, lookupFn)
		b.ReportAllocs()
		b.ResetTimer()
		for i := range b.N {
			_, _, _ = wrapped(b.Context(), fmt.Sprintf("key%d", i))
		}
	})

	b.Run("enabled_hit", func(b *testing.B) {
		cache := NewCache(CacheConfig{
			Enabled: true,
			Size:    1000,
			TTL:     5 * time.Minute,
		})
		wrapped := WrapWithCache(cache, lookupFn)
		// Prime the cache
		_, _, _ = wrapped(b.Context(), "key")
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			_, _, _ = wrapped(b.Context(), "key")
		}
	})
}

func BenchmarkCacheParallel(b *testing.B) {
	cache := NewCache(CacheConfig{
		Enabled: true,
		Size:    10000,
		TTL:     5 * time.Minute,
	})

	// Pre-populate cache
	for i := range 1000 {
		cache.set(fmt.Sprintf("key%d", i), fmt.Sprintf("value%d", i), true)
	}

	b.Run("get_parallel", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				_, _, _ = cache.get(fmt.Sprintf("key%d", i%1000))
				i++
			}
		})
	})

	b.Run("mixed_parallel", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				if i%10 == 0 {
					cache.set(fmt.Sprintf("new%d", i), "value", true)
				} else {
					_, _, _ = cache.get(fmt.Sprintf("key%d", i%1000))
				}
				i++
			}
		})
	})
}
