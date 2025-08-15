// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package slidingwindow // import "github.com/platformbuilds/opentelemetry-collector-contrib/processor/alertsprocessor/slidingwindow"

import (
	"sync"
	"time"

	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// Config is local to avoid importing the parent processor package.
type Config struct {
	Duration         time.Duration
	MaxSamples       int
	OverflowBehavior string // "ring_buffer" | "drop_new"
}

type Window struct {
	cfg Config
	mu  sync.RWMutex

	mets []tm[pmetric.Metrics]
	logs []tm[plog.Logs]
	trcs []tm[ptrace.Traces]
}

type tm[T any] struct {
	at time.Time
	v  T
}

func New(cfg Config) *Window { return &Window{cfg: cfg} }

func (w *Window) IngestMetrics(md pmetric.Metrics) {
	w.mu.Lock()
	w.mets = append(w.mets, tm[pmetric.Metrics]{at: time.Now(), v: md})
	w.trim()
	w.mu.Unlock()
}

func (w *Window) IngestLogs(ld plog.Logs) {
	w.mu.Lock()
	w.logs = append(w.logs, tm[plog.Logs]{at: time.Now(), v: ld})
	w.trim()
	w.mu.Unlock()
}

func (w *Window) IngestTraces(td ptrace.Traces) {
	w.mu.Lock()
	w.trcs = append(w.trcs, tm[ptrace.Traces]{at: time.Now(), v: td})
	w.trim()
	w.mu.Unlock()
}

func (w *Window) SnapshotMetrics() []pmetric.Metrics {
	w.mu.RLock()
	defer w.mu.RUnlock()
	cut := time.Now().Add(-w.cfg.Duration)
	out := make([]pmetric.Metrics, 0, len(w.mets))
	for _, x := range w.mets {
		if x.at.After(cut) {
			out = append(out, x.v)
		}
	}
	return out
}

func (w *Window) SnapshotLogs() []plog.Logs {
	w.mu.RLock()
	defer w.mu.RUnlock()
	cut := time.Now().Add(-w.cfg.Duration)
	out := make([]plog.Logs, 0, len(w.logs))
	for _, x := range w.logs {
		if x.at.After(cut) {
			out = append(out, x.v)
		}
	}
	return out
}

func (w *Window) SnapshotTraces() []ptrace.Traces {
	w.mu.RLock()
	defer w.mu.RUnlock()
	cut := time.Now().Add(-w.cfg.Duration)
	out := make([]ptrace.Traces, 0, len(w.trcs))
	for _, x := range w.trcs {
		if x.at.After(cut) {
			out = append(out, x.v)
		}
	}
	return out
}

func (w *Window) trim() {
	cut := time.Now().Add(-w.cfg.Duration)
	w.mets = crop(w.mets, cut, w.cfg.MaxSamples, w.cfg.OverflowBehavior)
	w.logs = crop(w.logs, cut, w.cfg.MaxSamples, w.cfg.OverflowBehavior)
	w.trcs = crop(w.trcs, cut, w.cfg.MaxSamples, w.cfg.OverflowBehavior)
}

// crop is a generic helper that (1) removes entries older than `cut`,
// and (2) enforces MaxSamples with either ring-buffer or drop_new behavior.
func crop[T any](buf []tm[T], cut time.Time, maxSamples int, overflowBehavior string) []tm[T] {
	// time-based trim
	first := 0
	for ; first < len(buf); first++ {
		if buf[first].at.After(cut) {
			break
		}
	}
	if first > 0 {
		// re-slice to drop old entries; copy to avoid holding references
		newBuf := make([]tm[T], len(buf)-first)
		copy(newBuf, buf[first:])
		buf = newBuf
	}

	// size-based trim
	if maxSamples > 0 && len(buf) > maxSamples {
		if overflowBehavior == "ring_buffer" {
			// keep the most recent maxSamples
			buf = buf[len(buf)-maxSamples:]
		} else {
			// "drop_new": keep the earliest maxSamples
			buf = buf[:maxSamples]
		}
	}
	return buf
}
