// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package flush // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/flush"

import (
	"bufio"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/trim"
)

// Wrap a bufio.SplitFunc with a flusher
func WithPeriod(splitFunc bufio.SplitFunc, trimFunc trim.Func, period time.Duration) bufio.SplitFunc {
	if period <= 0 {
		return splitFunc
	}
	f := &flusher{
		lastDataChange:     time.Now(),
		forcePeriod:        period,
		previousDataLength: 0,
	}
	return f.splitFunc(splitFunc, trimFunc)
}

// flusher keeps information about flush state
type flusher struct {
	// forcePeriod defines time from last flush which should pass before setting force to true.
	// Never forces if forcePeriod is set to 0
	forcePeriod time.Duration

	// lastDataChange tracks date of last data change (including new data and flushes)
	lastDataChange time.Time

	// previousDataLength:
	// if previousDataLength = 0 - no new data have been received after flush
	// if previousDataLength > 0 - there is data which has not been flushed yet and it doesn't changed since lastDataChange
	previousDataLength int
}

func (f *flusher) updateDataChangeTime(length int) {
	// Skip if length is greater than 0 and didn't changed
	if length > 0 && length == f.previousDataLength {
		return
	}

	// update internal properties with new values if data length changed
	// because it means that data is flowing and being processed
	f.previousDataLength = length
	f.lastDataChange = time.Now()
}

// Flushed reset data length
func (f *flusher) flushed() {
	f.updateDataChangeTime(0)
}

// ShouldFlush returns true if data should be forcefully flushed
func (f *flusher) shouldFlush() bool {
	// Returns true if there is f.forcePeriod after f.lastDataChange and data length is greater than 0
	return f.forcePeriod > 0 && time.Since(f.lastDataChange) > f.forcePeriod && f.previousDataLength > 0
}

func (f *flusher) splitFunc(splitFunc bufio.SplitFunc, trimFunc trim.Func) bufio.SplitFunc {
	return func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		advance, token, err = splitFunc(data, atEOF)

		// Return as it is in case of error
		if err != nil {
			return
		}

		// Return token
		if token != nil {
			// Inform flusher that we just flushed
			f.flushed()
			return
		}

		// If there is no token, force flush eventually
		if f.shouldFlush() {
			// Inform flusher that we just flushed
			f.flushed()
			token = trimFunc(data)
			advance = len(data)
			return
		}

		// Inform flusher that we didn't flushed
		f.updateDataChangeTime(len(data))
		return
	}
}
