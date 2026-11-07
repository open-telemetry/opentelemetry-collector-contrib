// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsecscontainermetrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsecscontainermetricsreceiver/internal/awsecscontainermetrics"

import (
	"sync"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

type counterState struct {
	prevVal   int64
	startTime pcommon.Timestamp
}

// TaskResetTracker tracks the previous values and start times of task-level cumulative counters.
// When a task counter decreases (e.g. because a container restarted or stopped), it detects
// the reset and updates the start time to the previous scrape timestamp.
type TaskResetTracker struct {
	mu             sync.Mutex
	prevScrapeTime pcommon.Timestamp
	counters       map[string]counterState
}

// NewTaskResetTracker creates a new TaskResetTracker.
func NewTaskResetTracker() *TaskResetTracker {
	return &TaskResetTracker{
		counters: make(map[string]counterState),
	}
}

// Adjust checks whether a counter has reset (decreased) since the last scrape.
// If it has decreased, the start time moves forward to the previous scrape time (or current timestamp if none).
// Otherwise, the existing start time is retained.
func (t *TaskResetTracker) Adjust(metricName string, val int64, currentTs, initialStartTime pcommon.Timestamp) pcommon.Timestamp {
	t.mu.Lock()
	defer t.mu.Unlock()

	state, exists := t.counters[metricName]
	if !exists {
		start := initialStartTime
		if start == 0 {
			start = currentTs
		}
		t.counters[metricName] = counterState{
			prevVal:   val,
			startTime: start,
		}
		return start
	}

	if val < state.prevVal {
		if t.prevScrapeTime != 0 {
			state.startTime = t.prevScrapeTime
		} else {
			state.startTime = currentTs
		}
	}
	state.prevVal = val
	t.counters[metricName] = state
	return state.startTime
}

// FinishScrape records the timestamp of the completed scrape.
func (t *TaskResetTracker) FinishScrape(scrapeTime pcommon.Timestamp) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.prevScrapeTime = scrapeTime
}
