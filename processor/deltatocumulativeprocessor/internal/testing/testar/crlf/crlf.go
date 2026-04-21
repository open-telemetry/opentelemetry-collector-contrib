// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package crlf // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/testing/testar/crlf"

import (
	"bytes"
	"strings"
)

const (
	CR   = '\r'
	LF   = '\n'
	CRLF = "\r\n"
)

// Strip turns CRLF line endings (\r\n) into LF (\n)
func Strip(data []byte) []byte {
	at := bytes.IndexByte(data, LF)
	if at == 0 || data[at-1] != CR {
		return data
	}
	return bytes.ReplaceAll(data, []byte(CRLF), []byte{LF})
}

// Join concats all lines with the [CRLF] separator
func Join(lines ...string) []byte {
	return []byte(strings.Join(lines, CRLF))
}
