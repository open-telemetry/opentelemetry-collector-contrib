// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tokenlen // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/tokenlen"

import "bufio"

// State tracks the potential length of a token before any terminator checking
type State struct {
	MinimumLength int
}

// Func wraps a bufio.SplitFunc to track potential token lengths
// Records the length of the data before delegating to the wrapped function
func (s *State) Func(splitFunc bufio.SplitFunc) bufio.SplitFunc {
	if s == nil {
		return splitFunc
	}

	return func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		// Note the potential token length but don't update state until we know
		// whether or not a token is actually returned
		potentialLen := len(data)

		advance, token, err = splitFunc(data, atEOF)
		if advance == 0 && token == nil && err == nil {
			// The splitFunc is asking for more data. Remember how much
			// we saw previously so the buffer can be sized appropriately.
			s.MinimumLength = potentialLen
		} else {
			// A token was returned. This state represented that token, so clear it.
			s.MinimumLength = 0
		}
		return advance, token, err
	}
}
