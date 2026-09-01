// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampler

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAlwaysSample(t *testing.T) {
	s := NewAlwaysSample()
	require.NoError(t, s.Start())
	t.Cleanup(func() { _ = s.Stop() })

	assert.Equal(t, 1, s.GetSampleRate("anything", 1))
	assert.Equal(t, 1, s.GetSampleRate("", 100))
}

func TestDeterministic(t *testing.T) {
	tests := []struct {
		percentage float64
		wantRate   int
	}{
		{100, 1},
		{50, 2},
		{10, 10},
		{1, 100},
		{0.1, 1000},
	}
	for _, tt := range tests {
		s, err := NewDeterministic(tt.percentage)
		require.NoError(t, err)
		require.NoError(t, s.Start())
		t.Cleanup(func() { _ = s.Stop() })

		assert.Equal(t, tt.wantRate, s.GetSampleRate("k", 1), "percentage=%v", tt.percentage)
	}
}

func TestDeterministic_InvalidPercentage(t *testing.T) {
	_, err := NewDeterministic(0)
	assert.Error(t, err)
	_, err = NewDeterministic(101)
	assert.Error(t, err)
	_, err = NewDeterministic(-5)
	assert.Error(t, err)
}

func TestEMAPercentage_ReturnsPositiveRate(t *testing.T) {
	s, err := NewEMAPercentage(EMAPercentageConfig{
		GoalSamplingPercentage: 10,
		AdjustmentInterval:     15 * time.Second,
		Weight:                 0.5,
	})
	require.NoError(t, err)
	require.NoError(t, s.Start())
	t.Cleanup(func() { _ = s.Stop() })

	rate := s.GetSampleRate("svc-a", 1)
	assert.GreaterOrEqual(t, rate, 1)
}

func TestEMAPercentage_InvalidPercentage(t *testing.T) {
	_, err := NewEMAPercentage(EMAPercentageConfig{GoalSamplingPercentage: 0})
	assert.Error(t, err)
	_, err = NewEMAPercentage(EMAPercentageConfig{GoalSamplingPercentage: 200})
	assert.Error(t, err)
}

func TestEMAThroughput_ReturnsPositiveRate(t *testing.T) {
	s, err := NewEMAThroughput(EMAThroughputConfig{
		GoalThroughputPerSec: 100,
		AdjustmentInterval:   15 * time.Second,
		Weight:               0.5,
	})
	require.NoError(t, err)
	require.NoError(t, s.Start())
	t.Cleanup(func() { _ = s.Stop() })

	rate := s.GetSampleRate("svc-a", 1)
	assert.GreaterOrEqual(t, rate, 1)
}

func TestEMAThroughput_InvalidGoal(t *testing.T) {
	_, err := NewEMAThroughput(EMAThroughputConfig{GoalThroughputPerSec: 0})
	assert.Error(t, err)
	_, err = NewEMAThroughput(EMAThroughputConfig{GoalThroughputPerSec: -10})
	assert.Error(t, err)
}

func TestWindowedThroughput_ReturnsPositiveRate(t *testing.T) {
	s, err := NewWindowedThroughput(WindowedThroughputConfig{
		GoalThroughputPerSec: 100,
		UpdateFrequency:      1 * time.Second,
		LookbackFrequency:    30 * time.Second,
	})
	require.NoError(t, err)
	require.NoError(t, s.Start())
	t.Cleanup(func() { _ = s.Stop() })

	rate := s.GetSampleRate("svc-a", 1)
	assert.GreaterOrEqual(t, rate, 1)
}

func TestWindowedThroughput_InvalidGoal(t *testing.T) {
	_, err := NewWindowedThroughput(WindowedThroughputConfig{GoalThroughputPerSec: 0})
	assert.Error(t, err)
}

// zeroRateSampler stands in for the windowed throughput sampler's behavior of
// returning 0 for keys it has no computed rate for (cold start, untracked
// keys, max_keys overflow).
type zeroRateSampler struct{}

func (zeroRateSampler) Start() error                       { return nil }
func (zeroRateSampler) Stop() error                        { return nil }
func (zeroRateSampler) GetSampleRateMulti(string, int) int { return 0 }

func TestDynsamplerWrapper_FallbackRateOnZero(t *testing.T) {
	w := &dynsamplerWrapper{inner: zeroRateSampler{}, fallbackRate: 10}
	assert.Equal(t, 10, w.GetSampleRate("any-key", 1),
		"a non-positive inner rate must map to the bootstrap rate, not keep-everything")

	// An unset fallback still never returns a non-positive rate.
	w = &dynsamplerWrapper{inner: zeroRateSampler{}}
	assert.Equal(t, 1, w.GetSampleRate("any-key", 1))
}

func TestWindowedThroughput_ColdStartUsesInitialRate(t *testing.T) {
	s, err := NewWindowedThroughput(WindowedThroughputConfig{
		GoalThroughputPerSec: 100,
		InitialSamplingRate:  10,
		UpdateFrequency:      time.Second,
		LookbackFrequency:    30 * time.Second,
	})
	require.NoError(t, err)
	require.NoError(t, s.Start())
	t.Cleanup(func() { _ = s.Stop() })

	// Before the first lookback window completes, the library has no rate for
	// any key; the wrapper must apply the bootstrap instead of keeping all.
	assert.Equal(t, 10, s.GetSampleRate("svc-a", 1))
}

func TestDynsamplerWrapper_StopIsIdempotent(t *testing.T) {
	samplers := []func() (Sampler, error){
		func() (Sampler, error) {
			return NewEMAPercentage(EMAPercentageConfig{GoalSamplingPercentage: 10, AdjustmentInterval: 15 * time.Second, Weight: 0.5})
		},
		func() (Sampler, error) {
			return NewEMAThroughput(EMAThroughputConfig{GoalThroughputPerSec: 100, AdjustmentInterval: 15 * time.Second, Weight: 0.5})
		},
		func() (Sampler, error) {
			return NewWindowedThroughput(WindowedThroughputConfig{GoalThroughputPerSec: 100, UpdateFrequency: time.Second, LookbackFrequency: 30 * time.Second})
		},
	}
	for _, build := range samplers {
		s, err := build()
		require.NoError(t, err)
		require.NoError(t, s.Start())
		require.NoError(t, s.Stop())
		require.NoError(t, s.Stop())
	}
}
