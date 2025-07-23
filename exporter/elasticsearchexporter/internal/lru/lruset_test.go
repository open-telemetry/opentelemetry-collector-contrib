// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package lru

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLRUSet(t *testing.T) {
	cache, err := NewLRUSet(5, time.Minute)
	require.NoError(t, err)

	err = cache.WithLock(func(lock LockedLRUSet) error {
		assert.False(t, lock.CheckAndAdd("a"))
		assert.True(t, lock.CheckAndAdd("a"))
		assert.False(t, lock.CheckAndAdd("b"))

		assert.InDelta(t, 0.0, testing.AllocsPerRun(5, func() {
			_ = lock.CheckAndAdd("c")
		}), 0)

		return nil
	})

	assert.NoError(t, err)
}

func TestLRUSetLifeTime(t *testing.T) {
	const lifetime = 100 * time.Millisecond
	cache, err := NewLRUSet(5, lifetime)
	require.NoError(t, err)

	err = cache.WithLock(func(lock LockedLRUSet) error {
		assert.False(t, lock.CheckAndAdd("a"))
		assert.True(t, lock.CheckAndAdd("a"))
		return nil
	})
	require.NoError(t, err)

	// Wait until cache item is expired.
	time.Sleep(lifetime)
	err = cache.WithLock(func(lock LockedLRUSet) error {
		assert.False(t, lock.CheckAndAdd("a"))
		assert.True(t, lock.CheckAndAdd("a"))
		return nil
	})
	require.NoError(t, err)

	// Wait 50% of the lifetime, so the item is not expired.
	time.Sleep(lifetime / 2)
	err = cache.WithLock(func(lock LockedLRUSet) error {
		assert.True(t, lock.CheckAndAdd("a"))
		return nil
	})
	require.NoError(t, err)

	// Wait another 50% of the lifetime, so the item should be expired.
	time.Sleep(lifetime / 2)
	err = cache.WithLock(func(lock LockedLRUSet) error {
		assert.False(t, lock.CheckAndAdd("a"))
		return nil
	})
	require.NoError(t, err)
}

func BenchmarkLRUSetCheck(b *testing.B) {
	cache, err := NewLRUSet(5, time.Minute)
	require.NoError(b, err)

	_ = cache.WithLock(func(lock LockedLRUSet) error {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			lock.CheckAndAdd("a")
		}

		return nil
	})
}
