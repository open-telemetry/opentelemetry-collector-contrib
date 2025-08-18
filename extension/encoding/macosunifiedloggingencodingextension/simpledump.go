// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package macosunifiedloggingencodingextension

import (
	"fmt"
)

// ParseSimpledumpChunk parses a Simpledump chunk (0x6004) containing simple string data
func ParseSimpledumpChunk(data []byte, entry *TraceV3Entry) {
	if len(data) < 16 {
		entry.Message = fmt.Sprintf("Simpledump chunk too small: %d bytes", len(data))
		return
	}

	entry.Message = fmt.Sprintf("Simpledump entry: simple string data (%d bytes)", len(data))
	entry.Level = "Info"
}
