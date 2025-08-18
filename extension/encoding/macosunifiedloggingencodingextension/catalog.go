// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package macosunifiedloggingencodingextension

import (
	"fmt"
)

// ParseCatalogChunk parses a Catalog chunk (0x600b) containing catalog data
func ParseCatalogChunk(data []byte, entry *TraceV3Entry) {
	if len(data) < 16 {
		entry.Message = fmt.Sprintf("Catalog chunk too small: %d bytes", len(data))
		return
	}

	entry.Message = fmt.Sprintf("Catalog entry: catalog data (%d bytes)", len(data))
	entry.Level = "Debug"
}
