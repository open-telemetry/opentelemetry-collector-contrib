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

	var timeSet time.Time

	// Wait until cache item is expired.
	time.Sleep(lifetime)
	err = cache.WithLock(func(lock LockedLRUSet) error {
		timeSet = time.Now()
		assert.False(t, lock.CheckAndAdd("a"))
		assert.True(t, lock.CheckAndAdd("a"))
		return nil
	})
	require.NoError(t, err)

	// Keep checking for expiry and assert that first expiry happens after at least lifetime
	assert.EventuallyWithT(t, func(tt *assert.CollectT) {
		var val bool
		err = cache.WithLock(func(lock LockedLRUSet) error {
			val = lock.CheckAndAdd("a")
			return nil
		})
		require.NoError(tt, err)
		assert.False(tt, val)
	}, lifetime*2, lifetime/100) // tick is kept at 1% of lifetime
	// Assert with epsilon to somewhat account for NTP adjustments.
	// Also see: https://github.com/elastic/go-freelru/issues/1
	assert.InEpsilon(t, lifetime, time.Since(timeSet), 0.015) // epsilon is kept at 1.5%
}

func BenchmarkLRUSetCheck(b *testing.B) {
	cache, err := NewLRUSet(5, time.Minute)
	require.NoError(b, err)

	_ = cache.WithLock(func(lock LockedLRUSet) error {
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			lock.CheckAndAdd("a")
		}

		return nil
	})
}
