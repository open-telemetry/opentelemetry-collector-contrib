// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package flush // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/flush"

import (
	"bufio"
	"time"
)

// WithPeriod wraps a bufio.SplitFunc with a timer.
// When the timer expires, an incomplete token may be returned.
// The timer will reset any time the data parameter changes.
func WithPeriod(splitFunc bufio.SplitFunc, period time.Duration) bufio.SplitFunc {
	if period <= 0 {
		return splitFunc
	}
	f := &flusher{lastDataChange: time.Now(), forcePeriod: period}
	return f.splitFunc(splitFunc)
}

type flusher struct {
	forcePeriod        time.Duration
	lastDataChange     time.Time
	previousDataLength int
}

func (f *flusher) splitFunc(splitFunc bufio.SplitFunc) bufio.SplitFunc {
	return func(data []byte, atEOF bool) (int, []byte, error) {
		advance, token, err := splitFunc(data, atEOF)

		// Don't interfere with errors
		if err != nil {
			return advance, token, err
		}

		// If there's a token, return it
		if token != nil {
			f.lastDataChange = time.Now()
			f.previousDataLength = 0
			return advance, token, err
		}

		// Can't flush something from nothing
		if atEOF && len(data) == 0 {
			f.previousDataLength = 0
			return 0, nil, nil
		}

		// Flush timed out
		if time.Since(f.lastDataChange) > f.forcePeriod {
			f.lastDataChange = time.Now()
			f.previousDataLength = 0
			return len(data), data, nil
		}

		// We're seeing new data so postpone the next flush
		if len(data) > f.previousDataLength {
			f.lastDataChange = time.Now()
			f.previousDataLength = len(data)
		}

		// Ask for more data
		return 0, nil, nil
	}
}
