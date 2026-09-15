// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampler

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Golden vector: dynsampler-go v0.6.4 TestHappyPath. Goal 2/s, 1s update, 5s
// lookback; buckets of 20, 10, 50, then three empty, then 40 produce rates
// 2, 3, 8, 6 (after the gap) and 9, matching the library's expected sequence
// at each tick.
func TestWindowState_GoldenHappyPath(t *testing.T) {
	s, err := NewSharedWindowedThroughput(SharedThroughputConfig{
		GoalThroughputPerSec: 2,
		InitialSamplingRate:  10,
		UpdateFrequency:      time.Second,
		LookbackFrequency:    5 * time.Second,
	})
	require.NoError(t, err)
	const key = "test_key"

	// No table yet: bootstrap.
	assert.Equal(t, 10, s.GetSampleRate(key, 1))

	s.ApplyMergedCounts(map[string]float64{key: 20})
	assert.Equal(t, 2, s.GetSampleRate(key, 1))

	s.ApplyMergedCounts(map[string]float64{key: 10})
	assert.Equal(t, 3, s.GetSampleRate(key, 1))

	s.ApplyMergedCounts(map[string]float64{key: 50})
	assert.Equal(t, 8, s.GetSampleRate(key, 1))

	// Three empty ticks: at the third, the window is 10+50 = 60 -> rate 6.
	s.ApplyMergedCounts(map[string]float64{})
	s.ApplyMergedCounts(map[string]float64{})
	s.ApplyMergedCounts(map[string]float64{})
	assert.Equal(t, 6, s.GetSampleRate(key, 1))

	s.ApplyMergedCounts(map[string]float64{key: 40})
	assert.Equal(t, 9, s.GetSampleRate(key, 1))
}

// Golden vector: dynsampler-go v0.6.4 TestDropsOldBlocks. Once the only
// traffic falls out of the lookback window, the table resets and keys return
// to the bootstrap rate (the library returns 0 there; the sampler maps that
// to the bootstrap).
func TestWindowState_GoldenDropsOldBlocks(t *testing.T) {
	s, err := NewSharedWindowedThroughput(SharedThroughputConfig{
		GoalThroughputPerSec: 2,
		InitialSamplingRate:  10,
		UpdateFrequency:      time.Second,
		LookbackFrequency:    5 * time.Second,
	})
	require.NoError(t, err)
	const key = "test_key"

	s.ApplyMergedCounts(map[string]float64{key: 20})
	assert.Equal(t, 2, s.GetSampleRate(key, 1))
	for range 6 {
		s.ApplyMergedCounts(map[string]float64{})
	}
	assert.Equal(t, 10, s.GetSampleRate(key, 1),
		"traffic aged out of the window must return to the bootstrap rate")
}

// max_keys admission is deterministic (sorted) and overflow keys answer at
// the bootstrap rate; eviction of old buckets frees slots.
func TestWindowState_MaxKeysOverflowFallsBack(t *testing.T) {
	s, err := NewSharedWindowedThroughput(SharedThroughputConfig{
		GoalThroughputPerSec: 100,
		InitialSamplingRate:  7,
		MaxKeys:              2,
		UpdateFrequency:      time.Second,
		LookbackFrequency:    2 * time.Second,
	})
	require.NoError(t, err)

	s.ApplyMergedCounts(map[string]float64{"a": 1000, "b": 1000, "c": 1000})
	table := *s.rates.Load()
	assert.Len(t, table, 2)
	assert.Contains(t, table, "a")
	assert.Contains(t, table, "b")
	assert.Equal(t, 7, s.GetSampleRate("c", 1), "overflow key uses the bootstrap rate")

	// Two empty ticks age out a and b, freeing slots for c.
	s.ApplyMergedCounts(map[string]float64{})
	s.ApplyMergedCounts(map[string]float64{})
	s.ApplyMergedCounts(map[string]float64{"c": 1000})
	table = *s.rates.Load()
	assert.Contains(t, table, "c", "aged-out keys free max_keys slots")
}

// Two windowed instances applying identical merged buckets converge on
// identical tables, including under max_keys pressure.
func TestWindowState_TwoInstancesConverge(t *testing.T) {
	mk := func() *SharedThroughput {
		s, err := NewSharedWindowedThroughput(SharedThroughputConfig{
			GoalThroughputPerSec: 50,
			MaxKeys:              3,
			UpdateFrequency:      time.Second,
			LookbackFrequency:    5 * time.Second,
		})
		require.NoError(t, err)
		return s
	}
	a, b := mk(), mk()
	for range 10 {
		a.ApplyMergedCounts(map[string]float64{"k1": 900, "k2": 600, "k3": 30, "k4": 5})
		b.ApplyMergedCounts(map[string]float64{"k1": 900, "k2": 600, "k3": 30, "k4": 5})
	}
	for _, key := range []string{"k1", "k2", "k3", "k4"} {
		assert.Equal(t, a.GetSampleRate(key, 1), b.GetSampleRate(key, 1), key)
	}
}
