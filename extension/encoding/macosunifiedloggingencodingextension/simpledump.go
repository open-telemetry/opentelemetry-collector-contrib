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
	// Header fields (16 bytes)
	ChunkTag      uint32
	ChunkSubtag   uint32
	ChunkDataSize uint64

	// Payload fields (matching rust implementation)
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

// parseUUID converts a 16-byte UUID to string format matching the rust implementation
func parseUUID(data []byte) string {
	if len(data) < 16 {
		return ""
	}

	// Format as uppercase hex string to match rust implementation
	// The rust code does format!("{uuid:02X?}") which creates a debug format with uppercase hex
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
// Note: The data passed in includes the complete chunk with 16-byte header (tag, subtag, size)
func ParseSimpledumpChunk(data []byte, entry *TraceV3Entry) {
	if len(data) < 84 { // Minimum size: 16-byte header + 68-byte payload
		entry.Message = fmt.Sprintf("Simpledump chunk too small: %d bytes (need at least 84)", len(data))
		return
	}

	var chunk SimpleDumpChunk
	offset := 0

	// Parse chunk header (16 bytes total)
	chunk.ChunkTag = binary.LittleEndian.Uint32(data[offset : offset+4])
	offset += 4
	chunk.ChunkSubtag = binary.LittleEndian.Uint32(data[offset : offset+4])
	offset += 4
	chunk.ChunkDataSize = binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8

	// Validate chunk tag
	if chunk.ChunkTag != 0x6004 {
		entry.Message = fmt.Sprintf("Invalid simpledump chunk tag: expected 0x6004, got 0x%x", chunk.ChunkTag)
		return
	}

	// Parse simpledump payload fields
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

	// Validate string sizes are reasonable
	if chunk.UnknownSizeSubsystemString > 1024 || chunk.UnknownSizeMessageString > 1024 {
		entry.Message = fmt.Sprintf("Simpledump string sizes too large: subsystem=%d, message=%d",
			chunk.UnknownSizeSubsystemString, chunk.UnknownSizeMessageString)
		return
	}

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

	// Use catalog to enhance process information if available
	actualPID := uint32(chunk.FirstProcID)
	subsystemName := chunk.Subsystem

	if GlobalCatalog != nil {
		// Try to resolve using the chunk's process IDs for validation
		if resolvedPID := GlobalCatalog.GetPID(chunk.FirstProcID, uint32(chunk.SecondProcID)); resolvedPID != 0 {
			actualPID = resolvedPID
		}

		// Cross-validate subsystem information from catalog
		if subsysInfo := GlobalCatalog.GetSubsystem(0, chunk.FirstProcID, uint32(chunk.SecondProcID)); subsysInfo.Subsystem != "Unknown subsystem" && subsysInfo.Subsystem != "" {
			// If we have a good catalog match and no subsystem from simpledump, use catalog
			if strings.TrimSpace(chunk.Subsystem) == "" {
				subsystemName = subsysInfo.Subsystem
			}
		}
	}

	// Update entry with parsed data (enhanced with catalog information)
	entry.ProcessID = actualPID
	entry.ThreadID = chunk.ThreadID
	entry.Level = "Default"
	entry.MessageType = "Default"
	entry.EventType = "Simpledump"

	// Set subsystem - clean up the format to match expected output
	if strings.TrimSpace(subsystemName) != "" {
		entry.Subsystem = strings.TrimSpace(subsystemName)
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
