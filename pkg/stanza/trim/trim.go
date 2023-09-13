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

var Nop Func = func(token []byte) []byte {
	return token
}

var Leading Func = func(data []byte) []byte {
	token := bytes.TrimLeft(data, "\r\n\t ")
	if token == nil {
		// TrimLeft sometimes overwrites something with nothing.
		// We need to override this behavior in order to preserve empty tokens.
		return data
	}
	return token
}

var Trailing Func = func(data []byte) []byte {
	return bytes.TrimRight(data, "\r\n\t ")
}

var Whitespace Func = func(data []byte) []byte {
	return Leading(Trailing(data))
}
