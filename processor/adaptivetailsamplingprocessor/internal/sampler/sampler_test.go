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

func TestDynsamplerWrapper_GetMetrics(t *testing.T) {
	tests := []struct {
		name       string
		build      func() (Sampler, error)
		wantKeys   []string
		absentKeys []string
	}{
		{
			name: "ema_percentage",
			build: func() (Sampler, error) {
				return NewEMAPercentage(EMAPercentageConfig{GoalSamplingPercentage: 10, AdjustmentInterval: 15 * time.Second, Weight: 0.5})
			},
			wantKeys: []string{"request_count", "event_count", "keyspace_size", "burst_count", "interval_count"},
		},
		{
			name: "ema_throughput",
			build: func() (Sampler, error) {
				return NewEMAThroughput(EMAThroughputConfig{GoalThroughputPerSec: 100, AdjustmentInterval: 15 * time.Second, Weight: 0.5})
			},
			wantKeys: []string{"request_count", "event_count", "keyspace_size", "burst_count", "interval_count"},
		},
		{
			name: "windowed_throughput",
			build: func() (Sampler, error) {
				return NewWindowedThroughput(WindowedThroughputConfig{GoalThroughputPerSec: 100, UpdateFrequency: time.Second, LookbackFrequency: 30 * time.Second})
			},
			wantKeys:   []string{"request_count", "event_count", "keyspace_size"},
			absentKeys: []string{"burst_count", "interval_count"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := tt.build()
			require.NoError(t, err)
			require.NoError(t, s.Start())
			t.Cleanup(func() { _ = s.Stop() })

			mp, ok := s.(MetricsProvider)
			require.True(t, ok, "dynsampler-backed sampler must implement MetricsProvider")

			s.GetSampleRate("svc-a", 1)
			// "" matches the prefix the processor actually passes to GetMetrics.
			metrics := mp.GetMetrics("")
			for _, k := range tt.wantKeys {
				assert.Contains(t, metrics, k)
			}
			for _, k := range tt.absentKeys {
				assert.NotContains(t, metrics, k)
			}
			assert.EqualValues(t, 1, metrics["request_count"])
		})
	}
}

func TestNonDynsamplerSamplers_DoNotImplementMetricsProvider(t *testing.T) {
	_, ok := NewAlwaysSample().(MetricsProvider)
	assert.False(t, ok)

	d, err := NewDeterministic(50)
	require.NoError(t, err)
	_, ok = d.(MetricsProvider)
	assert.False(t, ok)
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
