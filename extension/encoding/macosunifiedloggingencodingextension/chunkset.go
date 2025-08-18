// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package macosunifiedloggingencodingextension

import (
	"encoding/binary"
	"fmt"

	"github.com/pierrec/lz4/v4"
)

// ParseChunksetChunk parses a Chunkset chunk (0x600d) containing compressed log data
// Based on the Rust implementation from mandiant/macos-UnifiedLogs
func ParseChunksetChunk(data []byte, entry *TraceV3Entry, header *TraceV3Header) []*TraceV3Entry {
	if len(data) < 32 { // Need at least 32 bytes for chunkset header
		entry.Message = fmt.Sprintf("Chunkset chunk too small: %d bytes", len(data))
		return []*TraceV3Entry{entry}
	}

	// Parse chunkset header structure (similar to rust parse_chunkset)
	offset := 16 // Skip preamble which was already parsed
	signature := binary.LittleEndian.Uint32(data[offset:])
	uncompressSize := binary.LittleEndian.Uint32(data[offset+4:])
	blockSize := binary.LittleEndian.Uint32(data[offset+8:])

	offset += 12

	// Check for bv41 compression signature
	bv41 := uint32(825521762)             // "bv41" signature for compressed data
	bv41Uncompressed := uint32(758412898) // "bv41-" signature for uncompressed data

	var decompressedData []byte

	if signature == bv41Uncompressed {
		// Data is already uncompressed
		if len(data) < offset+int(uncompressSize)+4 {
			entry.Message = fmt.Sprintf("Chunkset uncompressed data too small: need %d, have %d", offset+int(uncompressSize)+4, len(data))
			return []*TraceV3Entry{entry}
		}
		decompressedData = data[offset : offset+int(uncompressSize)]

	} else if signature == bv41 {
		// Data is compressed, need to decompress using LZ4
		if len(data) < offset+int(blockSize)+4 {
			entry.Message = fmt.Sprintf("Chunkset compressed data too small: need %d, have %d", offset+int(blockSize)+4, len(data))
			return []*TraceV3Entry{entry}
		}

		compressedData := data[offset : offset+int(blockSize)]
		decompressedData = make([]byte, uncompressSize)

		// Decompress using LZ4
		n, err := lz4.UncompressBlock(compressedData, decompressedData)
		if err != nil {
			entry.Message = fmt.Sprintf("Failed to decompress chunkset data: %v", err)
			return []*TraceV3Entry{entry}
		}
		decompressedData = decompressedData[:n]

	} else {
		entry.Message = fmt.Sprintf("Unknown chunkset signature: 0x%x (expected 0x%x or 0x%x)", signature, bv41, bv41Uncompressed)
		return []*TraceV3Entry{entry}
	}

	// Parse individual log entries from decompressed data
	// The decompressed data contains multiple log chunks that need to be parsed
	entries := parseDataEntries(decompressedData, header)

	return entries
}
