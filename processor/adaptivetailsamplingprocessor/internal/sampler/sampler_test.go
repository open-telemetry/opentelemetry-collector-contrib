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

// zeroRateSampler stands in for a dynsampler sampler that returns 0 for keys
// it has no computed rate for (cold start, untracked keys, max_keys overflow).
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

func TestDynsamplerWrapper_StopIsIdempotent(t *testing.T) {
	s, err := NewEMAPercentage(EMAPercentageConfig{GoalSamplingPercentage: 10, AdjustmentInterval: 15 * time.Second, Weight: 0.5})
	require.NoError(t, err)
	require.NoError(t, s.Start())
	require.NoError(t, s.Stop())
	require.NoError(t, s.Stop())
}
