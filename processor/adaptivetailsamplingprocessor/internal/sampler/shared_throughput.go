// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampler // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/adaptivetailsamplingprocessor/internal/sampler"

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// rateEngine computes per-key rate tables from a sequence of completed
// interval buckets. Engines are driven by the sync loop only and are not safe
// for concurrent use.
type rateEngine interface {
	// apply folds one completed interval bucket (consumed) and returns the
	// recomputed table plus whether to swap it in. The ema engine keeps the
	// previous table on empty intervals (traffic gaps must not decay
	// averages); the windowed engine always swaps, including to an empty
	// table, which sends every key to the bootstrap rate.
	apply(bucket map[string]float64) (map[string]int, bool)
	// interval is the cadence the sync loop should tick on.
	interval() time.Duration
}

type emaEngine struct {
	state      *emaState
	goalPerSec float64
	ival       time.Duration
}

func (e *emaEngine) apply(bucket map[string]float64) (map[string]int, bool) {
	e.state.observe(bucket)
	table := e.state.rates(e.goalPerSec, e.ival)
	return table, table != nil
}

func (e *emaEngine) interval() time.Duration { return e.ival }

type windowEngine struct {
	state      *windowState
	goalPerSec float64
	update     time.Duration
	lookback   time.Duration
}

func (e *windowEngine) apply(bucket map[string]float64) (map[string]int, bool) {
	e.state.push(bucket)
	return e.state.rates(e.goalPerSec, e.lookback), true
}

func (e *windowEngine) interval() time.Duration { return e.update }

// SharedThroughputConfig configures a throughput sampler whose per-key rates
// are computed over merged counts from a counter store, so
// GoalThroughputPerSec is the fleet budget when the store spans instances
// (and plain per-instance behavior with the in-memory store).
type SharedThroughputConfig struct {
	GoalThroughputPerSec float64
	// InitialSamplingRate applies to every key until the first table exists,
	// to keys absent from the table, and to keys beyond MaxKeys.
	InitialSamplingRate int
	MaxKeys             int
	// AdjustmentInterval and Weight configure the ema engine.
	AdjustmentInterval time.Duration
	Weight             float64
	// UpdateFrequency and LookbackFrequency configure the windowed engine.
	UpdateFrequency   time.Duration
	LookbackFrequency time.Duration
}

// SharedThroughput implements Sampler over a rate table computed from merged
// counts. It accumulates this instance's observations per interval; the sync
// loop (owned by the processor) drains them with SnapshotCounts, publishes
// them to the counter store, and applies the merged counts back with
// ApplyMergedCounts. The decide path only touches the local counts map and an
// atomically swapped rate table, never the store.
type SharedThroughput struct {
	initialRate int

	// mu guards counts, the current interval's local accumulation.
	mu     sync.Mutex
	counts map[string]float64

	// rates is the active table, swapped whole by ApplyMergedCounts.
	rates atomic.Pointer[map[string]int]

	// engine is only touched by ApplyMergedCounts; the sync loop serializes it.
	engine rateEngine
}

// NewSharedEMAThroughput constructs the sampler with the ema engine,
// mirroring dynsampler-go's defaults (interval 15s, weight 0.5, age-out =
// weight).
func NewSharedEMAThroughput(cfg SharedThroughputConfig) (*SharedThroughput, error) {
	if cfg.GoalThroughputPerSec <= 0 {
		return nil, errors.New("adaptive_throughput (ema) sampler: goal_throughput must be greater than zero")
	}
	interval := cfg.AdjustmentInterval
	if interval == 0 {
		interval = 15 * time.Second
	}
	return newSharedThroughput(cfg, &emaEngine{
		state:      newEMAState(cfg.Weight, cfg.MaxKeys),
		goalPerSec: cfg.GoalThroughputPerSec,
		ival:       interval,
	}), nil
}

// NewSharedWindowedThroughput constructs the sampler with the windowed
// engine, mirroring dynsampler-go's defaults (update 1s, lookback 30x update,
// lookback floored to a multiple of the update frequency).
func NewSharedWindowedThroughput(cfg SharedThroughputConfig) (*SharedThroughput, error) {
	if cfg.GoalThroughputPerSec <= 0 {
		return nil, errors.New("adaptive_throughput (windowed) sampler: goal_throughput must be greater than zero")
	}
	update := cfg.UpdateFrequency
	if update == 0 {
		update = time.Second
	}
	lookback := cfg.LookbackFrequency
	if lookback == 0 {
		lookback = 30 * update
	}
	lookback = update * (lookback / update)
	ticks := int(lookback / update)
	return newSharedThroughput(cfg, &windowEngine{
		state:      newWindowState(ticks, cfg.MaxKeys),
		goalPerSec: cfg.GoalThroughputPerSec,
		update:     update,
		lookback:   lookback,
	}), nil
}

func newSharedThroughput(cfg SharedThroughputConfig, engine rateEngine) *SharedThroughput {
	initialRate := cfg.InitialSamplingRate
	if initialRate < 1 {
		initialRate = 10
	}
	return &SharedThroughput{
		initialRate: initialRate,
		counts:      make(map[string]float64),
		engine:      engine,
	}
}

// GetSampleRate implements Sampler. It records the observation locally and
// answers from the current rate table, falling back to the bootstrap rate for
// keys the table does not cover.
func (s *SharedThroughput) GetSampleRate(key string, spanCount int) int {
	if spanCount <= 0 {
		spanCount = 1
	}
	s.mu.Lock()
	s.counts[key] += float64(spanCount)
	s.mu.Unlock()

	if table := s.rates.Load(); table != nil {
		if rate, ok := (*table)[key]; ok {
			return max(rate, 1)
		}
	}
	return s.initialRate
}

// SnapshotCounts returns and resets this instance's accumulated counts for
// the interval that just completed. Called by the sync loop once per tick.
func (s *SharedThroughput) SnapshotCounts() map[string]float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.counts
	s.counts = make(map[string]float64, len(out))
	return out
}

// ApplyMergedCounts folds one completed interval of merged counts into the
// engine and swaps in the recomputed rate table when the engine produces one.
// merged is consumed. Only the sync loop may call this; it is not safe for
// concurrent use with itself.
func (s *SharedThroughput) ApplyMergedCounts(merged map[string]float64) {
	if table, swap := s.engine.apply(merged); swap {
		s.rates.Store(&table)
	}
}

// Interval returns the cadence the sync loop should tick on.
func (s *SharedThroughput) Interval() time.Duration { return s.engine.interval() }

// Start implements Sampler.
func (*SharedThroughput) Start() error { return nil }

// Stop implements Sampler.
func (*SharedThroughput) Stop() error { return nil }
