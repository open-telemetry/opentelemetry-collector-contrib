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
	skipping := false
	return func(data []byte, atEOF bool) (int, []byte, error) {
		if skipping {
			advance, token, err := splitFunc(data, atEOF)
			if advance > 0 {
				// Check if we've reached the end of the line (newline found)
				// For ScanLines, when a newline is found: advance = len(token) + 1, token = line content
				// So if advance > len(token), we've found the newline and finished skipping
				if advance > len(token) {
					skipping = false
					// Return advance with nil token to skip the remainder and newline
					return advance, nil, nil
				}
				// Still skipping - we're in the middle of the remainder
				// Return 0 to force scanner to read more data until we find the newline
				// This prevents creating multiple tokens from chunks of the remainder
				return 0, nil, nil
			}
			if err != nil {
				return 0, nil, err
			}
			// No advance possible - skip all remaining data
			return len(data), nil, nil
		}

		advance, token, err := splitFunc(data, atEOF)
		if (advance == 0 && token == nil && err == nil) && len(data) >= maxLength {
			skipping = true
			return maxLength, data[:maxLength], nil
		}
		if len(token) > maxLength {
			skipping = true
			return maxLength, token[:maxLength], nil
		}
		return advance, token, err
	}
}
