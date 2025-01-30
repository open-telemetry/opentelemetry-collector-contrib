// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tokenlen // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/tokenlen"

import "bufio"

// State tracks the potential length of a token before any terminator checking
type State struct {
	PotentialLength int
}

func (s *State) Copy() *State {
	if s == nil {
		return nil
	}
	return &State{
		PotentialLength: s.PotentialLength,
	}
}

// Func wraps a bufio.SplitFunc to track potential token lengths
// Records the length of the data before delegating to the wrapped function
func (s *State) Func(splitFunc bufio.SplitFunc) bufio.SplitFunc {
	if s == nil {
		return splitFunc
	}

	return func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		// Note the potential token length but don't update state yet
		potentialLen := len(data)

		// Delegate to the wrapped split function
		advance, token, err = splitFunc(data, atEOF)

		// Only update state if we didn't find a token (delegate returned 0, nil, nil)
		if advance == 0 && token == nil && err == nil {
			s.PotentialLength = potentialLen
		} else {
			// Clear the state if we got a token or error
			s.PotentialLength = 0
		}

		return advance, token, err
	}
}
