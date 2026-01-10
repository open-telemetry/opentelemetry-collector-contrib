// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package trim // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/trim"

import (
	"bufio"
	"bytes"
)

type Func func([]byte) []byte

func WithFunc(splitFunc bufio.SplitFunc, trimFunc Func) bufio.SplitFunc {
	if trimFunc == nil {
		return splitFunc
	}
	return func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		advance, token, err = splitFunc(data, atEOF)
		if advance == 0 && token == nil && err == nil {
			return 0, nil, nil
		}
		return advance, trimFunc(token), err
	}
}

type Config struct {
	PreserveLeading  bool `mapstructure:"preserve_leading_whitespaces,omitempty"`
	PreserveTrailing bool `mapstructure:"preserve_trailing_whitespaces,omitempty"`
}

func (c Config) Func() Func {
	if c.PreserveLeading && c.PreserveTrailing {
		return Nop
	}
	if c.PreserveLeading {
		return Trailing
	}
	if c.PreserveTrailing {
		return Leading
	}
	return Whitespace
}

func Nop(token []byte) []byte {
	return token
}

func Leading(data []byte) []byte {
	token := bytes.TrimLeft(data, "\r\n\t ")
	if token == nil {
		// TrimLeft sometimes overwrites something with nothing.
		// We need to override this behavior in order to preserve empty tokens.
		return data
	}
	return token
}

func Trailing(data []byte) []byte {
	return bytes.TrimRight(data, "\r\n\t ")
}

func Whitespace(data []byte) []byte {
	return Leading(Trailing(data))
}

func ToLength(splitFunc bufio.SplitFunc, maxLength int) bufio.SplitFunc {
	if maxLength <= 0 {
		return splitFunc
	}
	return func(data []byte, atEOF bool) (int, []byte, error) {
		advance, token, err := splitFunc(data, atEOF)
		if (advance == 0 && token == nil && err == nil) && len(data) >= maxLength {
			// No token was found, but we have enough data to return a token of max length.
			return maxLength, data[:maxLength], nil
		}
		if len(token) > maxLength {
			// A token was found but it is longer than the max length.
			return maxLength, token[:maxLength], nil
		}
		return advance, token, err
	}
}

// ToLengthWithTruncate wraps a bufio.SplitFunc to truncate tokens that exceed maxLength.
// Unlike ToLength which splits oversized content into multiple tokens, this function
// returns only the truncated portion (up to maxLength) and advances past the entire
// original content, effectively dropping the remainder.
// The skipping parameter is a pointer to a bool that tracks whether we're currently
// skipping the remainder of an oversized line. This allows the state to be persisted
// across multiple reader recreations (e.g., between poll cycles).
func ToLengthWithTruncate(splitFunc bufio.SplitFunc, maxLength int, skipping *bool) bufio.SplitFunc {
	if maxLength <= 0 {
		return splitFunc
	}

	return func(data []byte, atEOF bool) (int, []byte, error) {
		// If we're in skip mode, look for the next newline to resume normal processing
		if *skipping {
			for i := range len(data) {
				if data[i] == '\n' {
					// Found the end of the oversized line, resume normal processing
					*skipping = false
					// Advance past the newline and continue
					return i + 1, nil, nil
				}
			}
			// No newline found yet
			if atEOF {
				// At EOF, done skipping
				*skipping = false
				return len(data), nil, nil
			}
			// Skip all data we have and request more
			return len(data), nil, nil
		}

		advance, token, err := splitFunc(data, atEOF)

		if (advance == 0 && token == nil && err == nil) && len(data) >= maxLength {
			// No token was found, but we have enough data.
			// This means we have an oversized line that exceeds maxLength.

			// First check if newline is in the current buffer after maxLength
			for i := maxLength; i < len(data); i++ {
				if data[i] == '\n' {
					// Found a newline, return truncated token and advance past the whole line including newline
					return i + 1, data[:maxLength], nil
				}
			}

			// No newline found in current buffer
			// Emit truncated token and enter skip mode
			*skipping = true
			return len(data), data[:maxLength], nil
		}
		if len(token) > maxLength {
			// A token was found but it is longer than the max length.
			// Return truncated token but advance past the entire original token.
			return advance, token[:maxLength], nil
		}
		return advance, token, err
	}
}
