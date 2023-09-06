// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package trim // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/trim"

import (
	"bytes"
)

type Func func([]byte) []byte

func Whitespace(preserveLeading, preserveTrailing bool) Func {
	if preserveLeading && preserveTrailing {
		return noTrim
	}
	if preserveLeading {
		return trimTrailingWhitespacesFunc
	}
	if preserveTrailing {
		return trimLeadingWhitespacesFunc
	}
	return trimWhitespacesFunc
}

func noTrim(token []byte) []byte {
	return token
}

func trimLeadingWhitespacesFunc(data []byte) []byte {
	// TrimLeft to strip EOF whitespaces in case of using $ in regex
	// For some reason newline and carriage return are being moved to beginning of next log
	token := bytes.TrimLeft(data, "\r\n\t ")

	// TrimLeft will return nil if data is an empty slice
	if token == nil {
		return []byte{}
	}
	return token
}

func trimTrailingWhitespacesFunc(data []byte) []byte {
	// TrimRight to strip all whitespaces from the end of log
	return bytes.TrimRight(data, "\r\n\t ")
}

func trimWhitespacesFunc(data []byte) []byte {
	return trimLeadingWhitespacesFunc(trimTrailingWhitespacesFunc(data))
}
