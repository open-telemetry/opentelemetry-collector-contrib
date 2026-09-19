// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampler

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSharedThroughput_BootstrapUntilFirstTable(t *testing.T) {
	s, err := NewSharedEMAThroughput(SharedThroughputConfig{
		GoalThroughputPerSec: 100,
		InitialSamplingRate:  10,
	})
	require.NoError(t, err)

	assert.Equal(t, 10, s.GetSampleRate("svc-a", 1), "no table yet: bootstrap rate applies")

	// One merged interval produces a table; known keys answer from it,
	// unknown keys keep the bootstrap.
	s.ApplyMergedCounts(map[string]float64{"svc-a": 50000})
	assert.NotEqual(t, 10, s.GetSampleRate("svc-a", 1), "table rate replaces bootstrap")
	assert.Equal(t, 10, s.GetSampleRate("svc-new", 1), "keys absent from the table use the bootstrap")
}

func TestSharedThroughput_SnapshotDrainsCounts(t *testing.T) {
	s, err := NewSharedEMAThroughput(SharedThroughputConfig{GoalThroughputPerSec: 100})
	require.NoError(t, err)

	s.GetSampleRate("svc-a", 3)
	s.GetSampleRate("svc-a", 2)
	s.GetSampleRate("svc-b", 0) // non-positive span counts observe as 1

	counts := s.SnapshotCounts()
	assert.Equal(t, map[string]float64{"svc-a": 5, "svc-b": 1}, counts)
	assert.Empty(t, s.SnapshotCounts(), "snapshot resets the accumulator")
}

func TestSharedThroughput_EmptyMergedIntervalKeepsTable(t *testing.T) {
	s, err := NewSharedEMAThroughput(SharedThroughputConfig{GoalThroughputPerSec: 100})
	require.NoError(t, err)

	s.ApplyMergedCounts(map[string]float64{"svc-a": 50000})
	rate := s.GetSampleRate("svc-a", 1)
	s.ApplyMergedCounts(map[string]float64{})
	assert.Equal(t, rate, s.GetSampleRate("svc-a", 1), "empty interval must not change or drop the table")
}

// Two samplers fed the same merged counts converge on identical tables, the
// core leaderless property: shared inputs, deterministic computation.
func TestSharedThroughput_TwoInstancesConverge(t *testing.T) {
	newS := func() *SharedThroughput {
		s, err := NewSharedEMAThroughput(SharedThroughputConfig{
			GoalThroughputPerSec: 200,
			AdjustmentInterval:   15 * time.Second,
			Weight:               0.5,
		})
		require.NoError(t, err)
		return s
	}
	a, b := newS(), newS()

	// Instance A sees svc-1 heavy, instance B sees svc-2 heavy; the merged
	// view is identical for both.
	for range 20 {
		merged := map[string]float64{"svc-1": 90000, "svc-2": 60000, "svc-3": 300}
		mergedCopy := map[string]float64{"svc-1": 90000, "svc-2": 60000, "svc-3": 300}
		a.ApplyMergedCounts(merged)
		b.ApplyMergedCounts(mergedCopy)
	}

	for _, key := range []string{"svc-1", "svc-2", "svc-3"} {
		assert.Equal(t, a.GetSampleRate(key, 1), b.GetSampleRate(key, 1),
			"instances with the same merged counts must produce identical rates for %s", key)
	}
}

func TestSharedThroughput_InvalidGoal(t *testing.T) {
	_, err := NewSharedEMAThroughput(SharedThroughputConfig{GoalThroughputPerSec: 0})
	assert.Error(t, err)
}
