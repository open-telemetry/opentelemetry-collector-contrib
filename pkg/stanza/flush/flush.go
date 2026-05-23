// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package flush // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/flush"

import (
	"bufio"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/internal/stanzatime"
)

type State struct {
	LastDataChange time.Time
	LastDataLength int
}

// Func wraps a bufio.SplitFunc with a timer.
// When the timer expires, an incomplete token may be returned.
// The timer will reset any time the data parameter changes.
func (s *State) Func(splitFunc bufio.SplitFunc, period time.Duration) bufio.SplitFunc {
	if s == nil || period <= 0 {
		return splitFunc
	}

	return func(data []byte, atEOF bool) (int, []byte, error) {
		advance, token, err := splitFunc(data, atEOF)
		// Don't interfere with errors
		if err != nil {
			return advance, token, err
		}

		// If there's a token, return it
		if token != nil {
			s.LastDataChange = stanzatime.Now()
			s.LastDataLength = 0
			return advance, token, err
		}

		// Can't flush something from nothing
		if atEOF && len(data) == 0 {
			s.LastDataLength = 0
			return 0, nil, nil
		}

		// We're seeing new data so postpone the next flush
		if len(data) > s.LastDataLength {
			s.LastDataChange = stanzatime.Now()
			s.LastDataLength = len(data)
			return 0, nil, nil
		}

		// Flush timed out
		if stanzatime.Since(s.LastDataChange) > period {
			s.LastDataChange = stanzatime.Now()
			s.LastDataLength = 0
			return len(data), data, nil
		}

		// Ask for more data
		return 0, nil, nil
	}
}
