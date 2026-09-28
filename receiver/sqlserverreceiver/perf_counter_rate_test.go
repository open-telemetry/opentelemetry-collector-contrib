// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sqlserverreceiver

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPerfCounterRate(t *testing.T) {
	const key = "Backup/Restore Throughput/sec-master"

	t.Run("first sample has no baseline", func(t *testing.T) {
		s := &sqlServerScraperHelper{perfCounterCache: newCache(perfCounterCacheSize)}

		_, ok := s.perfCounterRate(key, 1000, 0)
		assert.False(t, ok, "nothing can be derived from a single sample")
	})

	t.Run("derives per-second rate from the delta", func(t *testing.T) {
		s := &sqlServerScraperHelper{perfCounterCache: newCache(perfCounterCacheSize)}

		_, ok := s.perfCounterRate(key, 1000, 0)
		require.False(t, ok)

		// 87,146,496 bytes over 10s, the shape of a single backup between scrapes.
		rate, ok := s.perfCounterRate(key, 1000+87146496, 10*time.Second)
		require.True(t, ok)
		assert.InDelta(t, 8714649.6, rate, 0.01)
	})

	t.Run("unchanged counter yields zero, not the running total", func(t *testing.T) {
		s := &sqlServerScraperHelper{perfCounterCache: newCache(perfCounterCacheSize)}

		_, ok := s.perfCounterRate(key, 609990872, 0)
		require.False(t, ok)

		rate, ok := s.perfCounterRate(key, 609990872, 10*time.Second)
		require.True(t, ok)
		assert.Zero(t, rate, "an idle counter must report no throughput")
	})

	t.Run("counter reset is suppressed rather than reported as negative", func(t *testing.T) {
		s := &sqlServerScraperHelper{perfCounterCache: newCache(perfCounterCacheSize)}

		_, ok := s.perfCounterRate(key, 500000, 0)
		require.False(t, ok)

		// SQL Server restarted, so the counter went backwards.
		_, ok = s.perfCounterRate(key, 1000, 10*time.Second)
		assert.False(t, ok, "a reset must not produce a negative rate")

		// The lower value becomes the new baseline.
		rate, ok := s.perfCounterRate(key, 3000, 10*time.Second)
		require.True(t, ok)
		assert.InDelta(t, 200.0, rate, 0.01)
	})

	t.Run("non-positive elapsed time is rejected", func(t *testing.T) {
		s := &sqlServerScraperHelper{perfCounterCache: newCache(perfCounterCacheSize)}

		_, ok := s.perfCounterRate(key, 1000, 0)
		require.False(t, ok)

		_, ok = s.perfCounterRate(key, 2000, 0)
		assert.False(t, ok, "dividing by a zero interval must be avoided")
	})

	t.Run("series are tracked independently per database", func(t *testing.T) {
		s := &sqlServerScraperHelper{perfCounterCache: newCache(perfCounterCacheSize)}

		_, ok := s.perfCounterRate("counter-master", 100, 0)
		require.False(t, ok)
		_, ok = s.perfCounterRate("counter-tempdb", 5000, 0)
		require.False(t, ok)

		master, ok := s.perfCounterRate("counter-master", 200, 10*time.Second)
		require.True(t, ok)
		tempdb, ok := s.perfCounterRate("counter-tempdb", 5000, 10*time.Second)
		require.True(t, ok)

		assert.InDelta(t, 10.0, master, 0.01)
		assert.Zero(t, tempdb, "one database's activity must not leak into another's rate")
	})
}
