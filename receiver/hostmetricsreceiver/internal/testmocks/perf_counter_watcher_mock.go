// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package testmocks // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/testmocks"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/winperfcounters"
)

type PerfCounterWatcherMock struct {
	Val       int64
	ScrapeErr error
	CloseErr  error
	closed    bool
}

// ScrapeRawValue implements winperfcounters.PerfCounterWatcher.
func (w *PerfCounterWatcherMock) ScrapeRawValue(rawValue *int64) (bool, error) {
	*rawValue = 0
	if w.ScrapeErr != nil {
		return false, w.ScrapeErr
	}

	*rawValue = w.Val
	return true, nil
}

// ScrapeRawValues implements winperfcounters.PerfCounterWatcher.
func (w *PerfCounterWatcherMock) ScrapeRawValues() ([]winperfcounters.RawCounterValue, error) {
	return nil, w.ScrapeErr
}

// ScrapeData returns scrapeErr if it's set, otherwise it returns a single countervalue with the mock's val
func (PerfCounterWatcherMock) ScrapeData() ([]winperfcounters.CounterValue, error) {
	panic("unimplemented")
}

// Reset panics; it should not be called
func (PerfCounterWatcherMock) Reset() error {
	panic("unimplemented")
}

// Path panics; It should not be called
func (PerfCounterWatcherMock) Path() string {
	panic("unimplemented")
}

// Close all counters/handles related to the query and free all associated memory.
func (w *PerfCounterWatcherMock) Close() error {
	if w.closed {
		panic("mockPerfCounterWatcher was already closed!")
	}
	w.closed = true
	return w.CloseErr
}
