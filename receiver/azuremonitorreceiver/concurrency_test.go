// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuremonitorreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azuremonitorreceiver"

import (
	"math/rand/v2"
	"strconv"
	"testing"
)

func benchmarkMapImpl(b *testing.B, m concurrentMetricsBuilderMap[int]) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			key := strconv.Itoa(rand.IntN(100000))
			m.Set(key, rand.Int())
			_, _ = m.Get(key)
		}
	})
}

func BenchmarkConcurrentMapImpl(b *testing.B) {
	m := newConcurrentMapImpl[int]()
	benchmarkMapImpl(b, m)
}

func BenchmarkSyncMapImpl(b *testing.B) {
	m := newSyncMapImpl[int]()
	benchmarkMapImpl(b, m)
}

func BenchmarkMutexMapImpl(b *testing.B) {
	m := newMutexMapImpl[int]()
	benchmarkMapImpl(b, m)
}

func benchmarkMapImplLarge(b *testing.B, m concurrentMetricsBuilderMap[int]) {
	// Pre-fill with 1 million entries
	for i := 0; i < 1_000_000; i++ {
		key := strconv.Itoa(i)
		m.Set(key, i)
	}
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// Randomly access existing and new keys
			if rand.IntN(2) == 0 {
				key := strconv.Itoa(rand.IntN(1_000_000)) // existing
				m.Set(key, rand.Int())
				_, _ = m.Get(key)
			} else {
				key := strconv.Itoa(rand.IntN(10_000_000)) // possibly new
				m.Set(key, rand.Int())
				_, _ = m.Get(key)
			}
		}
	})
}

func BenchmarkConcurrentMapImplLarge(b *testing.B) {
	m := newConcurrentMapImpl[int]()
	benchmarkMapImplLarge(b, m)
}

func BenchmarkSyncMapImplLarge(b *testing.B) {
	m := newSyncMapImpl[int]()
	benchmarkMapImplLarge(b, m)
}

func BenchmarkMutexMapImplLarge(b *testing.B) {
	m := newMutexMapImpl[int]()
	benchmarkMapImplLarge(b, m)
}
