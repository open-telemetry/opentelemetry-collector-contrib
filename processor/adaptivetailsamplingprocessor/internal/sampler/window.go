// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampler // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/adaptivetailsamplingprocessor/internal/sampler"

import (
	"math"
	"sort"
	"time"
)

// windowState ports the windowed throughput math from dynsampler-go v0.6.4
// (WindowedThroughput.updateMaps over a BlockList): per-update-tick count
// buckets over a lookback window, aggregated by summation, with the lookback
// goal split equally across the keys present in the window. Golden tests in
// window_test.go pin parity against the library's own test vectors. Not safe
// for concurrent use; callers serialize access.
type windowState struct {
	// buckets is a ring of completed tick buckets, length = lookback ticks.
	buckets []map[string]float64
	pos     int
	// maxKeys bounds the distinct keys tracked across the window; 0 means
	// unbounded. Keys beyond the cap are dropped at insertion, in sorted
	// order so instances applying identical merged buckets stay in
	// agreement. active tracks per-key presence across ring buckets.
	maxKeys int
	active  map[string]int
}

func newWindowState(lookbackTicks, maxKeys int) *windowState {
	if lookbackTicks < 1 {
		lookbackTicks = 1
	}
	return &windowState{
		buckets: make([]map[string]float64, lookbackTicks),
		maxKeys: maxKeys,
		active:  make(map[string]int),
	}
}

// push installs the just-completed tick bucket, evicting the oldest. bucket
// is consumed. When maxKeys is set, keys not already tracked are admitted in
// sorted order until the cap; the rest are dropped (the library's bounded
// BlockList rejects new keys the same way, by arrival order).
func (w *windowState) push(bucket map[string]float64) {
	if old := w.buckets[w.pos]; old != nil {
		for k := range old {
			w.active[k]--
			if w.active[k] <= 0 {
				delete(w.active, k)
			}
		}
	}
	if w.maxKeys > 0 {
		keys := make([]string, 0, len(bucket))
		for k := range bucket {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if _, tracked := w.active[k]; tracked {
				w.active[k]++
				continue
			}
			if len(w.active) >= w.maxKeys {
				delete(bucket, k)
				continue
			}
			w.active[k]++
		}
	} else {
		for k := range bucket {
			w.active[k]++
		}
	}
	w.buckets[w.pos] = bucket
	w.pos = (w.pos + 1) % len(w.buckets)
}

// rates computes the equal-split allocation over the aggregated window.
// Unlike the EMA engine it always returns a non-nil table: windowed
// semantics reset rates to empty when the window has no traffic, and the
// caller's bootstrap fallback covers every key.
func (w *windowState) rates(goalPerSec float64, lookback time.Duration) map[string]int {
	agg := make(map[string]float64)
	for _, b := range w.buckets {
		for k, v := range b {
			agg[k] += v
		}
	}
	table := make(map[string]int, len(agg))
	if len(agg) == 0 {
		return table
	}
	totalGoal := goalPerSec * lookback.Seconds()
	perKey := totalGoal / float64(len(agg))
	for k, v := range agg {
		// int truncation, not Ceil: the library truncates here.
		table[k] = int(math.Max(1, v/perKey))
	}
	return table
}
