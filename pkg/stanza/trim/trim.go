// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package trim // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/trim"

import (
	"bufio"
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

func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\r' || c == '\n'
}

func Leading(data []byte) []byte {
	for len(data) > 0 && isSpace(data[0]) {
		data = data[1:]
	}
	return data
}

func Trailing(data []byte) []byte {
	for len(data) > 0 && isSpace(data[len(data)-1]) {
		data = data[:len(data)-1]
	}
	return data
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
// skipping the remainder of an oversized entry. This allows the state to be persisted
// across multiple reader recreations (e.g., between poll cycles).
func ToLengthWithTruncate(splitFunc bufio.SplitFunc, maxLength int, skipping *bool) bufio.SplitFunc {
	if maxLength <= 0 {
		return splitFunc
	}

	// Use local state for lastDataLen tracking (not persisted across reader recreations)
	lastDataLen := 0

	return func(data []byte, atEOF bool) (int, []byte, error) {
		dataLen := len(data)
		defer func() { lastDataLen = dataLen }()

		// If we're in skip mode, use splitFunc to find the next entry boundary.
		// This is important for multiline patterns where entries can contain newlines.
		if *skipping {
			advance, token, err := splitFunc(data, atEOF)

			if advance > 0 || token != nil || err != nil {
				// splitFunc found a boundary or returned a token.
				// This token is the remainder of the truncated entry - discard it.
				*skipping = false
				return advance, nil, err
			}

			// splitFunc needs more data but didn't return anything.
			// If buffer appears to be at capacity, we must advance to avoid "token too long" error.
			// Buffer is at capacity if: dataLen == maxLength (buffer limited to maxLength),
			// OR dataLen == lastDataLen (buffer couldn't grow between calls).
			if dataLen == maxLength || (dataLen > maxLength && dataLen == lastDataLen) {
				// Buffer at capacity, skip all available data and stay in skip mode
				return dataLen, nil, nil
			}

			// At EOF, done skipping
			if atEOF {
				*skipping = false
				return dataLen, nil, nil
			}

			// Request more data, stay in skip mode
			return 0, nil, nil
		}

		advance, token, err := splitFunc(data, atEOF)

		if (advance == 0 && token == nil && err == nil) && dataLen >= maxLength {
			// No token was found (splitFunc needs more data to find entry boundary),
			// but we have enough data to exceed maxLength.

			// Determine if we should truncate now or wait for more data.
			// Truncate if:
			// - dataLen == maxLength: buffer is exactly at max capacity (common case for limited buffers)
			// - dataLen == lastDataLen: buffer couldn't grow between calls
			// - atEOF: no more data coming
			if dataLen == maxLength || dataLen == lastDataLen || atEOF {
				// Buffer is at max capacity or EOF, truncate and enter skip mode.
				*skipping = true
				return maxLength, data[:maxLength], nil
			}

			// Buffer is larger than maxLength but might still grow.
			// Let scanner try to read more data.
			return 0, nil, nil
		}
		if len(token) > maxLength {
			// A token was found but it is longer than the max length.
			// Return truncated token but advance past the entire original token.
			return advance, token[:maxLength], nil
		}
		return advance, token, err
	}
}
