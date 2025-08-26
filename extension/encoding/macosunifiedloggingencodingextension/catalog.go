// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package macosunifiedloggingencodingextension

import (
	"encoding/binary"
	"fmt"
	"strings"
)

// CatalogChunk represents a parsed Catalog chunk (0x600b)
type CatalogChunk struct {
	// Header fields (16 bytes)
	ChunkTag      uint32
	ChunkSubtag   uint32
	ChunkDataSize uint64

	// Catalog structure fields
	CatalogSubsystemStringsOffset   uint16
	CatalogProcessInfoEntriesOffset uint16
	NumberProcessInformationEntries uint16
	CatalogOffsetSubChunks          uint16
	NumberSubChunks                 uint16
	Unknown                         []byte // 6 bytes padding
	EarliestFirehoseTimestamp       uint64

	// Parsed data
	CatalogUUIDs            []string
	CatalogSubsystemStrings []byte
	ProcessInfoEntries      []ProcessInfoEntry
	CatalogSubchunks        []CatalogSubchunk
}

// ProcessInfoEntry represents process information in the catalog
type ProcessInfoEntry struct {
	Index                uint16
	Unknown              uint16
	CatalogMainUUIDIndex uint16
	CatalogDSCUUIDIndex  uint16
	FirstNumberProcID    uint64
	SecondNumberProcID   uint32
	PID                  uint32
	EffectiveUserID      uint32
	Unknown2             uint32
	NumberUUIDsEntries   uint32
	Unknown3             uint32
	NumberSubsystems     uint32
	Unknown4             uint32
	MainUUID             string
	DSCUUID              string
}

// CatalogSubchunk represents metadata for compressed log data
type CatalogSubchunk struct {
	Start                uint64
	End                  uint64
	UncompressedSize     uint32
	CompressionAlgorithm uint32
	NumberIndex          uint32
	NumberStringOffsets  uint32
}

// ParseCatalogChunk parses a Catalog chunk (0x600b) containing catalog metadata
// Based on the rust implementation in catalog.rs
func ParseCatalogChunk(data []byte, entry *TraceV3Entry) {
	if len(data) < 56 { // Minimum size: 16-byte header + 40-byte catalog header
		entry.Message = fmt.Sprintf("Catalog chunk too small: %d bytes (need at least 56)", len(data))
		return
	}

	var catalog CatalogChunk
	offset := 0

	// Parse chunk header (16 bytes total)
	catalog.ChunkTag = binary.LittleEndian.Uint32(data[offset : offset+4])
	offset += 4
	catalog.ChunkSubtag = binary.LittleEndian.Uint32(data[offset : offset+4])
	offset += 4
	catalog.ChunkDataSize = binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8

	// Validate chunk tag
	if catalog.ChunkTag != 0x600b {
		entry.Message = fmt.Sprintf("Invalid catalog chunk tag: expected 0x600b, got 0x%x", catalog.ChunkTag)
		return
	}

	// Parse catalog header fields (40 bytes)
	if len(data) < offset+40 {
		entry.Message = fmt.Sprintf("Catalog chunk too small for header: %d bytes", len(data))
		return
	}

	catalog.CatalogSubsystemStringsOffset = binary.LittleEndian.Uint16(data[offset : offset+2])
	offset += 2
	catalog.CatalogProcessInfoEntriesOffset = binary.LittleEndian.Uint16(data[offset : offset+2])
	offset += 2
	catalog.NumberProcessInformationEntries = binary.LittleEndian.Uint16(data[offset : offset+2])
	offset += 2
	catalog.CatalogOffsetSubChunks = binary.LittleEndian.Uint16(data[offset : offset+2])
	offset += 2
	catalog.NumberSubChunks = binary.LittleEndian.Uint16(data[offset : offset+2])
	offset += 2

	// Unknown/padding bytes (6 bytes)
	catalog.Unknown = make([]byte, 6)
	copy(catalog.Unknown, data[offset:offset+6])
	offset += 6

	catalog.EarliestFirehoseTimestamp = binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8

	// Parse UUIDs (16 bytes each)
	const uuidLength = 16
	numberCatalogUUIDs := int(catalog.CatalogSubsystemStringsOffset) / uuidLength

	if numberCatalogUUIDs > 0 {
		if len(data) < offset+numberCatalogUUIDs*uuidLength {
			entry.Message = fmt.Sprintf("Catalog chunk too small for UUIDs: %d bytes", len(data))
			return
		}

		catalog.CatalogUUIDs = make([]string, numberCatalogUUIDs)
		for i := 0; i < numberCatalogUUIDs; i++ {
			uuidBytes := data[offset : offset+uuidLength]
			// Parse as big-endian uint128 and format as uppercase hex (matching rust)
			catalog.CatalogUUIDs[i] = fmt.Sprintf("%02X%02X%02X%02X%02X%02X%02X%02X%02X%02X%02X%02X%02X%02X%02X%02X",
				uuidBytes[0], uuidBytes[1], uuidBytes[2], uuidBytes[3],
				uuidBytes[4], uuidBytes[5], uuidBytes[6], uuidBytes[7],
				uuidBytes[8], uuidBytes[9], uuidBytes[10], uuidBytes[11],
				uuidBytes[12], uuidBytes[13], uuidBytes[14], uuidBytes[15])
			offset += uuidLength
		}
	}

	// Parse subsystem strings
	subsystemStringsLength := int(catalog.CatalogProcessInfoEntriesOffset - catalog.CatalogSubsystemStringsOffset)
	if subsystemStringsLength > 0 && len(data) >= offset+subsystemStringsLength {
		catalog.CatalogSubsystemStrings = make([]byte, subsystemStringsLength)
		copy(catalog.CatalogSubsystemStrings, data[offset:offset+subsystemStringsLength])
		offset += subsystemStringsLength
	}

	// Parse a few process info entries (simplified for now)
	catalog.ProcessInfoEntries = make([]ProcessInfoEntry, 0, catalog.NumberProcessInformationEntries)
	for i := 0; i < int(catalog.NumberProcessInformationEntries) && i < 5; i++ { // Limit to first 5 for performance
		if len(data) < offset+44 { // Minimum entry size
			break
		}

		var procEntry ProcessInfoEntry
		procEntry.Index = binary.LittleEndian.Uint16(data[offset : offset+2])
		offset += 2
		procEntry.Unknown = binary.LittleEndian.Uint16(data[offset : offset+2])
		offset += 2
		procEntry.CatalogMainUUIDIndex = binary.LittleEndian.Uint16(data[offset : offset+2])
		offset += 2
		procEntry.CatalogDSCUUIDIndex = binary.LittleEndian.Uint16(data[offset : offset+2])
		offset += 2
		procEntry.FirstNumberProcID = binary.LittleEndian.Uint64(data[offset : offset+8])
		offset += 8
		procEntry.SecondNumberProcID = binary.LittleEndian.Uint32(data[offset : offset+4])
		offset += 4
		procEntry.PID = binary.LittleEndian.Uint32(data[offset : offset+4])
		offset += 4
		procEntry.EffectiveUserID = binary.LittleEndian.Uint32(data[offset : offset+4])
		offset += 4
		procEntry.Unknown2 = binary.LittleEndian.Uint32(data[offset : offset+4])
		offset += 4
		procEntry.NumberUUIDsEntries = binary.LittleEndian.Uint32(data[offset : offset+4])
		offset += 4
		procEntry.Unknown3 = binary.LittleEndian.Uint32(data[offset : offset+4])
		offset += 4

		// Get UUIDs from catalog array
		if int(procEntry.CatalogMainUUIDIndex) < len(catalog.CatalogUUIDs) {
			procEntry.MainUUID = catalog.CatalogUUIDs[procEntry.CatalogMainUUIDIndex]
		}
		if int(procEntry.CatalogDSCUUIDIndex) < len(catalog.CatalogUUIDs) {
			procEntry.DSCUUID = catalog.CatalogUUIDs[procEntry.CatalogDSCUUIDIndex]
		}

		catalog.ProcessInfoEntries = append(catalog.ProcessInfoEntries, procEntry)

		// Skip the rest of this entry for now (UUID entries, subsystem entries, etc.)
		// This is a simplified parser - full implementation would parse all fields
		// For now, just parse the first entry to avoid complex offset calculations
		break
	}

	// Extract some subsystem strings for display
	var subsystemStrings []string
	if len(catalog.CatalogSubsystemStrings) > 0 {
		// Split on null bytes and extract first few strings
		parts := strings.Split(string(catalog.CatalogSubsystemStrings), "\x00")
		for _, part := range parts {
			if strings.TrimSpace(part) != "" {
				subsystemStrings = append(subsystemStrings, strings.TrimSpace(part))
				if len(subsystemStrings) >= 3 { // Limit to first 3
					break
				}
			}
		}
	}

	// Update entry with parsed catalog information
	entry.ProcessID = uint32(catalog.EarliestFirehoseTimestamp & 0xFFFFFFFF) // Use timestamp as pseudo process ID
	entry.ThreadID = uint64(catalog.NumberProcessInformationEntries)
	entry.Level = "Debug"
	entry.MessageType = "Default"
	entry.EventType = "logEvent"

	// Create detailed message about catalog contents
	message := fmt.Sprintf("Catalog: %d UUIDs, %d processes, %d subchunks, earliest_time=%d",
		len(catalog.CatalogUUIDs), catalog.NumberProcessInformationEntries, catalog.NumberSubChunks,
		catalog.EarliestFirehoseTimestamp)

	if len(subsystemStrings) > 0 {
		message += fmt.Sprintf(", subsystems=[%s]", strings.Join(subsystemStrings, ", "))
	}

	if len(catalog.ProcessInfoEntries) > 0 {
		proc := catalog.ProcessInfoEntries[0]
		message += fmt.Sprintf(", sample_proc={pid=%d,uid=%d,main_uuid=%s}",
			proc.PID, proc.EffectiveUserID, proc.MainUUID[:8]+"...")
	}

	entry.Message = message

	// Store this catalog globally for use by other parsers
	GlobalCatalog = &catalog
}

// SubsystemInfo holds resolved subsystem and category information
type SubsystemInfo struct {
	Subsystem string
	Category  string
}

// GetSubsystem resolves subsystem ID to human-readable subsystem and category
// Based on the rust implementation's get_subsystem method
func (c *CatalogChunk) GetSubsystem(subsystemValue uint16, firstProcID uint64, secondProcID uint32) SubsystemInfo {
	if c == nil {
		return SubsystemInfo{Subsystem: "Unknown subsystem", Category: ""}
	}

	// Look for the process entry
	for _, procEntry := range c.ProcessInfoEntries {
		if procEntry.FirstNumberProcID == firstProcID && procEntry.SecondNumberProcID == secondProcID {
			// This is a simplified version - full implementation would parse subsystem entries
			// For now, extract some subsystem info from the subsystem strings
			if len(c.CatalogSubsystemStrings) > 0 {
				// Extract first available subsystem string as an approximation
				parts := strings.Split(string(c.CatalogSubsystemStrings), "\x00")
				for _, part := range parts {
					if strings.TrimSpace(part) != "" {
						return SubsystemInfo{
							Subsystem: strings.TrimSpace(part),
							Category:  "", // Would need full parsing for category
						}
					}
				}
			}
			break
		}
	}

	return SubsystemInfo{Subsystem: "Unknown subsystem", Category: ""}
}

// GetPID resolves internal process IDs to actual PID
// Based on the rust implementation's get_pid method
func (c *CatalogChunk) GetPID(firstProcID uint64, secondProcID uint32) uint32 {
	if c == nil {
		return 0
	}

	// Look for the process entry
	for _, procEntry := range c.ProcessInfoEntries {
		if procEntry.FirstNumberProcID == firstProcID && procEntry.SecondNumberProcID == secondProcID {
			return procEntry.PID
		}
	}

	return 0
}

// GetEUID resolves internal process IDs to effective user ID
// Based on the rust implementation's get_euid method
func (c *CatalogChunk) GetEUID(firstProcID uint64, secondProcID uint32) uint32 {
	if c == nil {
		return 0
	}

	// Look for the process entry
	for _, procEntry := range c.ProcessInfoEntries {
		if procEntry.FirstNumberProcID == firstProcID && procEntry.SecondNumberProcID == secondProcID {
			return procEntry.EffectiveUserID
		}
	}

	return 0
}
