// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package macosunifiedloggingencodingextension

import (
	"encoding/binary"
	"fmt"
	"strings"
)

// SimpleDumpChunk represents a parsed Simpledump chunk
type SimpleDumpChunk struct {
	FirstProcID                 uint64
	SecondProcID                uint64
	ContinuousTime              uint64
	ThreadID                    uint64
	UnknownOffset               uint32
	UnknownTTL                  uint16
	UnknownType                 uint16
	SenderUUID                  string
	DSCSharedCacheUUID          string
	UnknownNumberMessageStrings uint32
	UnknownSizeSubsystemString  uint32
	UnknownSizeMessageString    uint32
	Subsystem                   string
	MessageString               string
}

// parseUUID converts a 16-byte UUID to string format
func parseUUID(data []byte) string {
	if len(data) < 16 {
		return ""
	}

	// Read as little-endian uint128 and format as hex string
	return fmt.Sprintf("%02X%02X%02X%02X%02X%02X%02X%02X%02X%02X%02X%02X%02X%02X%02X%02X",
		data[0], data[1], data[2], data[3], data[4], data[5], data[6], data[7],
		data[8], data[9], data[10], data[11], data[12], data[13], data[14], data[15])
}

// extractString extracts a null-terminated string from binary data
func extractString(data []byte) string {
	if len(data) == 0 {
		return ""
	}

	// Find null terminator
	end := len(data)
	for i, b := range data {
		if b == 0 {
			end = i
			break
		}
	}

	return string(data[:end])
}

// ParseSimpledumpChunk parses a Simpledump chunk (0x6004) containing simple string data
// Based on the rust implementation in chunks/simpledump.rs
// Note: The data passed in is the chunk payload AFTER the chunk header has been parsed by TraceV3
func ParseSimpledumpChunk(data []byte, entry *TraceV3Entry) {
	if len(data) < 68 { // Minimum payload size: 8+8+8+8+4+2+2+16+16+4+4+4 = 84
		entry.Message = fmt.Sprintf("Simpledump chunk too small: %d bytes (need at least 68)", len(data))
		return
	}

	var chunk SimpleDumpChunk
	offset := 0

	// Parse simpledump payload (the chunk header tag, subtag, size are already parsed by TraceV3)
	chunk.FirstProcID = binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8
	chunk.SecondProcID = binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8
	chunk.ContinuousTime = binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8
	chunk.ThreadID = binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8
	chunk.UnknownOffset = binary.LittleEndian.Uint32(data[offset : offset+4])
	offset += 4
	chunk.UnknownTTL = binary.LittleEndian.Uint16(data[offset : offset+2])
	offset += 2
	chunk.UnknownType = binary.LittleEndian.Uint16(data[offset : offset+2])
	offset += 2

	// Parse UUIDs (16 bytes each)
	if len(data) < offset+32 {
		entry.Message = fmt.Sprintf("Simpledump chunk too small for UUIDs: %d bytes", len(data))
		return
	}
	chunk.SenderUUID = parseUUID(data[offset : offset+16])
	offset += 16
	chunk.DSCSharedCacheUUID = parseUUID(data[offset : offset+16])
	offset += 16

	// Parse string metadata
	if len(data) < offset+12 {
		entry.Message = fmt.Sprintf("Simpledump chunk too small for string metadata: %d bytes", len(data))
		return
	}
	chunk.UnknownNumberMessageStrings = binary.LittleEndian.Uint32(data[offset : offset+4])
	offset += 4
	chunk.UnknownSizeSubsystemString = binary.LittleEndian.Uint32(data[offset : offset+4])
	offset += 4
	chunk.UnknownSizeMessageString = binary.LittleEndian.Uint32(data[offset : offset+4])
	offset += 4

	// Parse subsystem string
	if chunk.UnknownSizeSubsystemString > 0 {
		if len(data) < offset+int(chunk.UnknownSizeSubsystemString) {
			entry.Message = fmt.Sprintf("Simpledump chunk too small for subsystem string: %d bytes", len(data))
			return
		}
		chunk.Subsystem = extractString(data[offset : offset+int(chunk.UnknownSizeSubsystemString)])
		offset += int(chunk.UnknownSizeSubsystemString)
	}

	// Parse message string
	if chunk.UnknownSizeMessageString > 0 {
		if len(data) < offset+int(chunk.UnknownSizeMessageString) {
			entry.Message = fmt.Sprintf("Simpledump chunk too small for message string: %d bytes", len(data))
			return
		}
		chunk.MessageString = extractString(data[offset : offset+int(chunk.UnknownSizeMessageString)])
		offset += int(chunk.UnknownSizeMessageString)
	}

	// Update entry with parsed data
	entry.ProcessID = uint32(chunk.FirstProcID)
	entry.ThreadID = chunk.ThreadID
	entry.Level = "Default"
	entry.MessageType = "Default"
	entry.EventType = "Simpledump"

	// Set subsystem - clean up the format to match expected output
	if chunk.Subsystem != "" {
		entry.Subsystem = strings.TrimSpace(chunk.Subsystem)
	} else {
		entry.Subsystem = "com.apple.simpledump"
	}

	// Set message
	if chunk.MessageString != "" {
		entry.Message = strings.TrimSpace(chunk.MessageString)
	} else {
		entry.Message = fmt.Sprintf("Simpledump entry: subsystem=%s thread=%d", entry.Subsystem, chunk.ThreadID)
	}

	// Set category to empty to match expected output format
	entry.Category = ""
}
