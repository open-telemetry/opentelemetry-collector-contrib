// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package slidingwindow

import (
	"sync"
	"time"
)

type Window struct { dur time.Duration; mu sync.Mutex; times []time.Time }

func New(d time.Duration) *Window { return &Window{ dur:d } }

func (w *Window) Add(t time.Time) { w.mu.Lock(); defer w.mu.Unlock(); w.times = append(w.times, t); w.compact(t) }

func (w *Window) Count(now time.Time) int { w.mu.Lock(); defer w.mu.Unlock(); w.compact(now); return len(w.times) }

func (w *Window) compact(now time.Time) { cut := now.Add(-w.dur); out := w.times[:0]; for _, ts := range w.times { if ts.After(cut) { out = append(out, ts) } } ; w.times = out }
