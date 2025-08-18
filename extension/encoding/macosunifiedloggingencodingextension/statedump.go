// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package macosunifiedloggingencodingextension

import (
	"fmt"
)

// ParseStatedumpChunk parses a Statedump chunk (0x6003) containing system state information
func ParseStatedumpChunk(data []byte, entry *TraceV3Entry) {
	if len(data) < 16 {
		entry.Message = fmt.Sprintf("Statedump chunk too small: %d bytes", len(data))
		return
	}

	entry.Message = fmt.Sprintf("Statedump entry: system state data (%d bytes)", len(data))
	entry.Level = "Debug"
}
