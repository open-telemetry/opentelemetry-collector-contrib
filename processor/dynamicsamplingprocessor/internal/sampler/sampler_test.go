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

func TestEMADynamic_ReturnsPositiveRate(t *testing.T) {
	s, err := NewEMADynamic(EMADynamicConfig{
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

func TestEMADynamic_InvalidPercentage(t *testing.T) {
	_, err := NewEMADynamic(EMADynamicConfig{GoalSamplingPercentage: 0})
	assert.Error(t, err)
	_, err = NewEMADynamic(EMADynamicConfig{GoalSamplingPercentage: 200})
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

func TestDynsamplerWrapper_StopIsIdempotent(t *testing.T) {
	samplers := []func() (Sampler, error){
		func() (Sampler, error) {
			return NewEMADynamic(EMADynamicConfig{GoalSamplingPercentage: 10, AdjustmentInterval: 15 * time.Second, Weight: 0.5})
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
