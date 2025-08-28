// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package macosunifiedloggingencodingextension

import (
	"encoding/binary"
	"fmt"
)

// ParseChunksetChunk parses a Chunkset chunk (0x600d) containing compressed log data
// Based on the Rust implementation from mandiant/macos-UnifiedLogs
// Enhanced with subchunk metadata for intelligent decompression
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

	// Validate chunkset signature using enhanced validation
	isCompressed, isValid, err := ValidateChunksetSignature(signature)
	if !isValid {
		entry.Message = fmt.Sprintf("Invalid chunkset signature: %v", err)
		return []*TraceV3Entry{entry}
	}

	var decompressedData []byte
	var decompressionInfo *SubchunkDecompressionInfo

	if !isCompressed {
		// Data is already uncompressed
		if len(data) < offset+int(uncompressSize) {
			entry.Message = fmt.Sprintf("Chunkset uncompressed data too small: need %d, have %d",
				offset+int(uncompressSize), len(data))
			return []*TraceV3Entry{entry}
		}
		decompressedData = data[offset : offset+int(uncompressSize)]

		// Update statistics for uncompressed data
		GlobalCompressionStats.UncompressedChunks++
		GlobalCompressionStats.TotalBytesDecompressed += uint64(len(decompressedData))

	} else {
		// Data is compressed, need to decompress using enhanced LZ4 decompression
		if len(data) < offset+int(blockSize) {
			entry.Message = fmt.Sprintf("Chunkset compressed data too small: need %d, have %d",
				offset+int(blockSize), len(data))
			return []*TraceV3Entry{entry}
		}

		compressedData := data[offset : offset+int(blockSize)]

		// Use enhanced decompression with subchunk metadata if available
		relevantSubchunk := findRelevantSubchunkForSize(uncompressSize)
		if relevantSubchunk != nil {
			var err error
			decompressedData, decompressionInfo, err = DecompressWithSubchunkInfo(compressedData, relevantSubchunk)
			if err != nil {
				entry.Message = fmt.Sprintf("Failed to decompress chunkset data with subchunk info: %v", err)
				return []*TraceV3Entry{entry}
			}
		} else {
			// Fallback to standard decompression
			const LZ4_COMPRESSION = 0x100
			decompressedData, err = DecompressChunksetData(compressedData, uncompressSize, LZ4_COMPRESSION)
			if err != nil {
				entry.Message = fmt.Sprintf("Failed to decompress chunkset data: %v", err)
				return []*TraceV3Entry{entry}
			}
		}
	}

	// Parse individual log entries from decompressed data
	// The decompressed data contains multiple log chunks that need to be parsed
	entries := parseDecompressedChunksetData(decompressedData, header, entry)

	// Add decompression information if available
	if decompressionInfo != nil && len(entries) > 0 {
		// Update the first entry with decompression details
		firstEntry := entries[0]
		if firstEntry.ChunkType == "chunkset_summary" {
			firstEntry.Message += fmt.Sprintf(" | Decompression: success=%t time=%v actual_size=%d",
				decompressionInfo.DecompressionSuccess, decompressionInfo.DecompressionTime, decompressionInfo.ActualSize)
		}
	}

	return entries
}

// parseDecompressedChunksetData parses individual log entries from decompressed chunkset data
// Enhanced with subchunk metadata awareness for better parsing
func parseDecompressedChunksetData(decompressedData []byte, header *TraceV3Header, templateEntry *TraceV3Entry) []*TraceV3Entry {
	var entries []*TraceV3Entry

	if len(decompressedData) == 0 {
		templateEntry.Message = "Empty decompressed chunkset data"
		return []*TraceV3Entry{templateEntry}
	}

	// Use subchunk metadata if available to optimize parsing
	var subchunkInfo *CatalogSubchunk
	if GlobalCatalog != nil && len(GlobalCatalog.CatalogSubchunks) > 0 {
		// Find the relevant subchunk for this data
		subchunkInfo = findRelevantSubchunk(decompressedData)
	}

	// Parse individual chunks from the decompressed data
	offset := 0
	chunkCount := 0
	const maxChunks = 1000 // Safety limit

	for offset < len(decompressedData) && chunkCount < maxChunks {
		// Need at least 16 bytes for chunk preamble
		if offset+16 > len(decompressedData) {
			break
		}

		// Parse chunk preamble
		chunkTag := binary.LittleEndian.Uint32(decompressedData[offset:])
		chunkSubTag := binary.LittleEndian.Uint32(decompressedData[offset+4:])
		chunkDataSize := binary.LittleEndian.Uint64(decompressedData[offset+8:])

		// Validate chunk data size
		if chunkDataSize == 0 || chunkDataSize > uint64(len(decompressedData)) {
			offset += 4
			continue
		}

		// Calculate total chunk size (preamble + data)
		totalChunkSize := 16 + int(chunkDataSize)
		if offset+totalChunkSize > len(decompressedData) {
			break
		}

		// Extract chunk data
		chunkData := decompressedData[offset : offset+totalChunkSize]

		// Create base entry
		chunkEntry := &TraceV3Entry{
			Type:         chunkTag,
			Size:         uint32(chunkDataSize),
			Timestamp:    header.ContinuousTime + uint64(chunkCount)*1000000,
			ThreadID:     0,
			ProcessID:    header.LogdPID,
			Level:        "Info",
			MessageType:  "Default",
			EventType:    "logEvent",
			TimezoneName: extractTimezoneName(header.TimezonePath),
		}

		// Parse based on chunk type with enhanced processing
		switch chunkTag {
		case 0x6001:
			// Firehose chunk - contains individual log entries
			chunkEntry.ChunkType = "firehose"
			chunkEntry.Subsystem = "com.apple.firehose.decompressed"
			chunkEntry.Category = "entry"

			// Use enhanced firehose parsing
			firehoseEntries := ParseFirehoseChunk(chunkData, chunkEntry, header)
			entries = append(entries, firehoseEntries...)

		case 0x6002:
			// Oversize chunk
			chunkEntry.ChunkType = "oversize"
			chunkEntry.Subsystem = "com.apple.oversize.decompressed"
			chunkEntry.Category = "oversize_data"
			ParseOversizeChunk(chunkData, chunkEntry)
			entries = append(entries, chunkEntry)

		case 0x6003:
			// Statedump chunk
			chunkEntry.ChunkType = "statedump"
			chunkEntry.Subsystem = "com.apple.statedump.decompressed"
			chunkEntry.Category = "system_state"
			ParseStatedumpChunk(chunkData, chunkEntry)
			entries = append(entries, chunkEntry)

		case 0x6004:
			// Simpledump chunk
			chunkEntry.ChunkType = "simpledump"
			ParseSimpledumpChunk(chunkData, chunkEntry)
			entries = append(entries, chunkEntry)

		default:
			// Unknown chunk type
			chunkEntry.ChunkType = "unknown_decompressed"
			chunkEntry.Subsystem = "com.apple.unknown.decompressed"
			chunkEntry.Category = fmt.Sprintf("unknown_0x%x", chunkTag)
			chunkEntry.Message = fmt.Sprintf("Unknown decompressed chunk: tag=0x%x sub_tag=0x%x size=%d",
				chunkTag, chunkSubTag, chunkDataSize)
			entries = append(entries, chunkEntry)
		}

		// Move to next chunk with 8-byte alignment padding
		offset += totalChunkSize
		paddingBytes := (8 - (chunkDataSize & 7)) & 7
		offset += int(paddingBytes)

		// Skip any zero padding
		for offset < len(decompressedData) && decompressedData[offset] == 0 {
			offset++
		}

		chunkCount++
	}

	// Add summary information if we used subchunk metadata
	if subchunkInfo != nil && len(entries) > 0 {
		summaryEntry := &TraceV3Entry{
			Type:         0x600d,
			Size:         uint32(len(decompressedData)),
			Timestamp:    header.ContinuousTime,
			ThreadID:     0,
			ProcessID:    header.LogdPID,
			ChunkType:    "chunkset_summary",
			Subsystem:    "com.apple.chunkset.decompressed",
			Category:     "decompression_info",
			Level:        "Info",
			MessageType:  "Info",
			EventType:    "logEvent",
			TimezoneName: extractTimezoneName(header.TimezonePath),
			Message: fmt.Sprintf("Decompressed chunkset: uncompressed_size=%d algorithm=0x%x chunks=%d indexes=%d string_offsets=%d",
				subchunkInfo.UncompressedSize, subchunkInfo.CompressionAlgorithm,
				chunkCount, len(subchunkInfo.Indexes), len(subchunkInfo.StringOffsets)),
		}
		entries = append([]*TraceV3Entry{summaryEntry}, entries...)
	}

	return entries
}

// findRelevantSubchunk finds the catalog subchunk that matches the decompressed data
func findRelevantSubchunk(decompressedData []byte) *CatalogSubchunk {
	if GlobalCatalog == nil || len(GlobalCatalog.CatalogSubchunks) == 0 {
		return nil
	}

	dataSize := uint32(len(decompressedData))

	// Find subchunk with matching uncompressed size
	for _, subchunk := range GlobalCatalog.CatalogSubchunks {
		if subchunk.UncompressedSize == dataSize {
			return &subchunk
		}
	}

	// If no exact match, find the closest one
	var bestMatch *CatalogSubchunk
	var smallestDiff uint32 = ^uint32(0) // Max uint32

	for _, subchunk := range GlobalCatalog.CatalogSubchunks {
		diff := uint32(0)
		if subchunk.UncompressedSize > dataSize {
			diff = subchunk.UncompressedSize - dataSize
		} else {
			diff = dataSize - subchunk.UncompressedSize
		}

		if diff < smallestDiff {
			smallestDiff = diff
			bestMatch = &subchunk
		}
	}

	return bestMatch
}

// findRelevantSubchunkForSize finds a catalog subchunk that matches the given uncompressed size
func findRelevantSubchunkForSize(uncompressedSize uint32) *CatalogSubchunk {
	if GlobalCatalog == nil || len(GlobalCatalog.CatalogSubchunks) == 0 {
		return nil
	}

	// Find exact match first
	for _, subchunk := range GlobalCatalog.CatalogSubchunks {
		if subchunk.UncompressedSize == uncompressedSize {
			return &subchunk
		}
	}

	return nil // Return nil if no exact match (could enhance with fuzzy matching if needed)
}
