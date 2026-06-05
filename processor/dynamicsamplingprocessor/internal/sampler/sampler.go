// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package sampler provides sampler implementations used by the dynamic sampling
// processor. Each sampler returns a positive integer sample rate where rate N
// means "keep 1 in N traces."
package sampler // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/dynamicsamplingprocessor/internal/sampler"

import (
	"errors"
	"sync"
	"time"

	dynsampler "github.com/honeycombio/dynsampler-go"
)

// Sampler returns the sample rate that should apply to a trace.
// Rate 1 means keep every trace; higher rates mean keep 1-in-N.
type Sampler interface {
	// GetSampleRate returns the sample rate for the given key. spanCount is the
	// number of spans that triggered the lookup, used by adaptive samplers to
	// weight their observations.
	GetSampleRate(key string, spanCount int) int
	// Start initialises any background state. Safe to call multiple times.
	Start() error
	// Stop releases any resources. Safe to call multiple times.
	Stop() error
}

// AlwaysSample returns rate 1 for every key.
type AlwaysSample struct{}

// NewAlwaysSample returns a sampler that keeps every trace.
func NewAlwaysSample() Sampler { return AlwaysSample{} }

// GetSampleRate implements Sampler.
func (AlwaysSample) GetSampleRate(string, int) int { return 1 }

// Start implements Sampler.
func (AlwaysSample) Start() error { return nil }

// Stop implements Sampler.
func (AlwaysSample) Stop() error { return nil }

// Deterministic returns a fixed sample rate computed from a percentage.
type Deterministic struct {
	rate int
}

// NewDeterministic constructs a deterministic sampler from a target percentage in (0, 100].
func NewDeterministic(percentage float64) (Sampler, error) {
	if percentage <= 0 || percentage > 100 {
		return nil, errors.New("deterministic sampler: percentage must be in (0, 100]")
	}
	rate := max(int(100.0/percentage), 1)
	return &Deterministic{rate: rate}, nil
}

// GetSampleRate implements Sampler.
func (d *Deterministic) GetSampleRate(string, int) int { return d.rate }

// Start implements Sampler.
func (*Deterministic) Start() error { return nil }

// Stop implements Sampler.
func (*Deterministic) Stop() error { return nil }

// dynsamplerImpl is the subset of dynsampler-go's Sampler interface used by
// this package. Declared locally so test doubles do not need to satisfy the
// full upstream interface (SaveState, LoadState, GetMetrics).
type dynsamplerImpl interface {
	Start() error
	Stop() error
	GetSampleRateMulti(key string, count int) int
}

// dynsamplerWrapper adapts any dynsampler-go sampler into our Sampler
// interface. Start/Stop are guarded by sync.Once so they are safe to call
// multiple times.
type dynsamplerWrapper struct {
	inner     dynsamplerImpl
	startOnce sync.Once
	startErr  error
	stopOnce  sync.Once
	stopErr   error
}

// GetSampleRate implements Sampler.
func (w *dynsamplerWrapper) GetSampleRate(key string, spanCount int) int {
	if spanCount <= 0 {
		spanCount = 1
	}
	return max(w.inner.GetSampleRateMulti(key, spanCount), 1)
}

// Start implements Sampler.
func (w *dynsamplerWrapper) Start() error {
	w.startOnce.Do(func() { w.startErr = w.inner.Start() })
	return w.startErr
}

// Stop implements Sampler.
func (w *dynsamplerWrapper) Stop() error {
	w.stopOnce.Do(func() { w.stopErr = w.inner.Stop() })
	return w.stopErr
}

// EMADynamicConfig configures the EMA dynamic (per-key) sampler.
type EMADynamicConfig struct {
	GoalSamplingPercentage float64
	AdjustmentInterval     time.Duration
	Weight                 float64
	MaxKeys                int
}

// NewEMADynamic constructs an EMA dynamic sampler from the supplied config.
func NewEMADynamic(cfg EMADynamicConfig) (Sampler, error) {
	if cfg.GoalSamplingPercentage <= 0 || cfg.GoalSamplingPercentage > 100 {
		return nil, errors.New("ema_dynamic sampler: goal_sampling_percentage must be in (0, 100]")
	}
	goalRate := max(int(100.0/cfg.GoalSamplingPercentage), 1)
	return &dynsamplerWrapper{
		inner: &dynsampler.EMASampleRate{
			GoalSampleRate:             goalRate,
			AdjustmentIntervalDuration: cfg.AdjustmentInterval,
			Weight:                     cfg.Weight,
			MaxKeys:                    cfg.MaxKeys,
		},
	}, nil
}

// EMAThroughputConfig configures the EMA throughput sampler, which targets a
// fixed events-per-second rate while preserving per-key proportions via EMA.
type EMAThroughputConfig struct {
	GoalThroughputPerSec int
	InitialSamplingRate  int
	AdjustmentInterval   time.Duration
	Weight               float64
	MaxKeys              int
}

// NewEMAThroughput constructs an EMA throughput sampler.
func NewEMAThroughput(cfg EMAThroughputConfig) (Sampler, error) {
	if cfg.GoalThroughputPerSec <= 0 {
		return nil, errors.New("ema_throughput sampler: goal_throughput_per_sec must be greater than zero")
	}
	return &dynsamplerWrapper{
		inner: &dynsampler.EMAThroughput{
			GoalThroughputPerSec: cfg.GoalThroughputPerSec,
			InitialSampleRate:    cfg.InitialSamplingRate,
			AdjustmentInterval:   cfg.AdjustmentInterval,
			Weight:               cfg.Weight,
			MaxKeys:              cfg.MaxKeys,
		},
	}, nil
}

// WindowedThroughputConfig configures the windowed throughput sampler, which
// adjusts rates faster than EMA by decoupling the update frequency from the
// lookback window.
type WindowedThroughputConfig struct {
	GoalThroughputPerSec float64
	UpdateFrequency      time.Duration
	LookbackFrequency    time.Duration
	MaxKeys              int
}

// NewWindowedThroughput constructs a windowed throughput sampler.
func NewWindowedThroughput(cfg WindowedThroughputConfig) (Sampler, error) {
	if cfg.GoalThroughputPerSec <= 0 {
		return nil, errors.New("windowed_throughput sampler: goal_throughput_per_sec must be greater than zero")
	}
	return &dynsamplerWrapper{
		inner: &dynsampler.WindowedThroughput{
			GoalThroughputPerSec:      cfg.GoalThroughputPerSec,
			UpdateFrequencyDuration:   cfg.UpdateFrequency,
			LookbackFrequencyDuration: cfg.LookbackFrequency,
			MaxKeys:                   cfg.MaxKeys,
		},
	}, nil
}
