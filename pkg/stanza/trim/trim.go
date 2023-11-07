// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package trim // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/trim"

import (
	"bytes"
)

type Func func([]byte) []byte

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
	// TrimLeft to strip EOF whitespaces in case of using $ in regex
	// For some reason newline and carriage return are being moved to beginning of next log
	token := bytes.TrimLeft(data, "\r\n\t ")

	// TrimLeft will return nil if data is an empty slice
	if token == nil {
		return []byte{}
	}
	return token
}

func Trailing(data []byte) []byte {
	// TrimRight to strip all whitespaces from the end of log
	return bytes.TrimRight(data, "\r\n\t ")
}

func Whitespace(data []byte) []byte {
	return Leading(Trailing(data))
}
