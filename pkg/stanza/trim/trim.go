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
