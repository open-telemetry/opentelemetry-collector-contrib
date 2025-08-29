// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package macosunifiedloggingencodingextension

import (
	"encoding/binary"
	"fmt"
	"strings"
)

// GlobalCatalog stores catalog data for use by other chunk parsers
var GlobalCatalog *CatalogChunk

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
	UUIDInfoEntries      []ProcessUUIDEntry
	NumberSubsystems     uint32
	Unknown4             uint32
	SubsystemEntries     []ProcessInfoSubsystem
	MainUUID             string
	DSCUUID              string
}

// ProcessUUIDEntry represents UUID information in the catalog
type ProcessUUIDEntry struct {
	Size             uint32
	Unknown          uint32
	CatalogUUIDIndex uint16
	LoadAddress      uint64
	UUID             string
}

// ProcessInfoSubsystem represents subsystem metadata in the catalog
// This helps get the subsystem (App Bundle ID) and the log entry category
type ProcessInfoSubsystem struct {
	Identifier      uint16 // Subsystem identifier to match against log entries
	SubsystemOffset uint16 // Offset to subsystem string in catalog_subsystem_strings
	CategoryOffset  uint16 // Offset to category string in catalog_subsystem_strings
}

// CatalogSubchunk represents metadata for compressed log data
type CatalogSubchunk struct {
	Start                uint64
	End                  uint64
	UncompressedSize     uint32
	CompressionAlgorithm uint32
	NumberIndex          uint32
	Indexes              []uint16
	NumberStringOffsets  uint32
	StringOffsets        []uint16
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

	// Parse process info entries with complete subsystem parsing
	catalog.ProcessInfoEntries = make([]ProcessInfoEntry, 0, catalog.NumberProcessInformationEntries)

	// Parse all process entries to build complete subsystem mapping
	for i := 0; i < int(catalog.NumberProcessInformationEntries); i++ {
		if len(data) < offset+44 { // Minimum entry size
			break
		}

		var procEntry ProcessInfoEntry

		// Parse basic process info fields (44 bytes)
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

		// Parse UUID info entries
		procEntry.UUIDInfoEntries = make([]ProcessUUIDEntry, procEntry.NumberUUIDsEntries)
		for j := 0; j < int(procEntry.NumberUUIDsEntries); j++ {
			if len(data) < offset+20 { // UUID entry size: 4+4+2+8+2=20 bytes
				break
			}

			var uuidEntry ProcessUUIDEntry
			uuidEntry.Size = binary.LittleEndian.Uint32(data[offset : offset+4])
			offset += 4
			uuidEntry.Unknown = binary.LittleEndian.Uint32(data[offset : offset+4])
			offset += 4
			uuidEntry.CatalogUUIDIndex = binary.LittleEndian.Uint16(data[offset : offset+2])
			offset += 2
			uuidEntry.LoadAddress = binary.LittleEndian.Uint64(data[offset : offset+8])
			offset += 8

			// Get UUID from catalog array
			if int(uuidEntry.CatalogUUIDIndex) < len(catalog.CatalogUUIDs) {
				uuidEntry.UUID = catalog.CatalogUUIDs[uuidEntry.CatalogUUIDIndex]
			}

			procEntry.UUIDInfoEntries[j] = uuidEntry
		}

		// Parse subsystem count and unknown field
		if len(data) < offset+8 {
			break
		}
		procEntry.NumberSubsystems = binary.LittleEndian.Uint32(data[offset : offset+4])
		offset += 4
		procEntry.Unknown4 = binary.LittleEndian.Uint32(data[offset : offset+4])
		offset += 4

		// Parse subsystem entries - this is critical for proper subsystem/category mapping
		procEntry.SubsystemEntries = make([]ProcessInfoSubsystem, procEntry.NumberSubsystems)
		for j := 0; j < int(procEntry.NumberSubsystems); j++ {
			if len(data) < offset+6 { // Subsystem entry size: 2+2+2=6 bytes
				break
			}

			var subsysEntry ProcessInfoSubsystem
			subsysEntry.Identifier = binary.LittleEndian.Uint16(data[offset : offset+2])
			offset += 2
			subsysEntry.SubsystemOffset = binary.LittleEndian.Uint16(data[offset : offset+2])
			offset += 2
			subsysEntry.CategoryOffset = binary.LittleEndian.Uint16(data[offset : offset+2])
			offset += 2

			procEntry.SubsystemEntries[j] = subsysEntry
		}

		// Get UUIDs from catalog array
		if int(procEntry.CatalogMainUUIDIndex) < len(catalog.CatalogUUIDs) {
			procEntry.MainUUID = catalog.CatalogUUIDs[procEntry.CatalogMainUUIDIndex]
		}
		if int(procEntry.CatalogDSCUUIDIndex) < len(catalog.CatalogUUIDs) {
			procEntry.DSCUUID = catalog.CatalogUUIDs[procEntry.CatalogDSCUUIDIndex]
		}

		catalog.ProcessInfoEntries = append(catalog.ProcessInfoEntries, procEntry)

		// Limit to reasonable number for performance and to avoid excessive parsing
		if i >= 50 {
			break
		}
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

	// Parse catalog subchunks if we have offset information
	if catalog.CatalogOffsetSubChunks > 0 && len(data) > int(56+catalog.CatalogOffsetSubChunks) {
		subchunksOffset := int(56 + catalog.CatalogOffsetSubChunks)
		subchunksData := data[subchunksOffset:]
		catalog.CatalogSubchunks = parseCatalogSubchunks(subchunksData, catalog.NumberSubChunks)
	}

	// Store this catalog globally for use by other parsers
	GlobalCatalog = &catalog
}

// parseCatalogSubchunks parses the subchunk metadata used for decompression
// Based on the rust implementation's parse_catalog_subchunk method
func parseCatalogSubchunks(data []byte, numberSubChunks uint16) []CatalogSubchunk {
	subchunks := make([]CatalogSubchunk, 0, numberSubChunks)
	offset := 0

	for i := 0; i < int(numberSubChunks) && i < 10; i++ { // Limit for performance
		if len(data) < offset+32 { // Minimum subchunk size
			break
		}

		var subchunk CatalogSubchunk

		// Parse basic subchunk fields (32 bytes)
		subchunk.Start = binary.LittleEndian.Uint64(data[offset : offset+8])
		offset += 8
		subchunk.End = binary.LittleEndian.Uint64(data[offset : offset+8])
		offset += 8
		subchunk.UncompressedSize = binary.LittleEndian.Uint32(data[offset : offset+4])
		offset += 4
		subchunk.CompressionAlgorithm = binary.LittleEndian.Uint32(data[offset : offset+4])
		offset += 4
		subchunk.NumberIndex = binary.LittleEndian.Uint32(data[offset : offset+4])
		offset += 4

		// Validate compression algorithm (should be LZ4 = 0x100)
		const LZ4_COMPRESSION = 0x100
		if subchunk.CompressionAlgorithm != LZ4_COMPRESSION {
			// Skip invalid subchunk
			offset += 4 // Skip remaining field
			continue
		}

		// Parse indexes
		if len(data) >= offset+int(subchunk.NumberIndex)*2 {
			subchunk.Indexes = make([]uint16, subchunk.NumberIndex)
			for j := 0; j < int(subchunk.NumberIndex); j++ {
				subchunk.Indexes[j] = binary.LittleEndian.Uint16(data[offset : offset+2])
				offset += 2
			}
		}

		// Parse number of string offsets
		if len(data) >= offset+4 {
			subchunk.NumberStringOffsets = binary.LittleEndian.Uint32(data[offset : offset+4])
			offset += 4

			// Parse string offsets
			if len(data) >= offset+int(subchunk.NumberStringOffsets)*2 {
				subchunk.StringOffsets = make([]uint16, subchunk.NumberStringOffsets)
				for j := 0; j < int(subchunk.NumberStringOffsets); j++ {
					subchunk.StringOffsets[j] = binary.LittleEndian.Uint16(data[offset : offset+2])
					offset += 2
				}
			}
		}

		// Calculate 8-byte alignment padding (matching rust implementation)
		totalItems := subchunk.NumberIndex + subchunk.NumberStringOffsets
		const offsetSize = 2 // Each offset is 2 bytes
		padding := (8 - ((totalItems * offsetSize) & 7)) & 7
		offset += int(padding)

		subchunks = append(subchunks, subchunk)
	}

	return subchunks
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

	// Look for the process entry that matches the proc IDs
	for _, procEntry := range c.ProcessInfoEntries {
		if procEntry.FirstNumberProcID == firstProcID && procEntry.SecondNumberProcID == secondProcID {
			// Search through the subsystem entries for this process
			for _, subsysEntry := range procEntry.SubsystemEntries {
				if subsystemValue == subsysEntry.Identifier {
					// Extract subsystem string using offset
					subsystemString := extractStringAtOffset(c.CatalogSubsystemStrings, int(subsysEntry.SubsystemOffset))

					// Extract category string using offset
					categoryString := extractStringAtOffset(c.CatalogSubsystemStrings, int(subsysEntry.CategoryOffset))

					return SubsystemInfo{
						Subsystem: subsystemString,
						Category:  categoryString,
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

// extractStringAtOffset extracts a null-terminated string from a byte array at a specific offset
// This matches the rust implementation's approach for extracting subsystem and category strings
func extractStringAtOffset(data []byte, offset int) string {
	if offset >= len(data) {
		return ""
	}

	// Find null terminator starting from offset
	start := offset
	end := len(data)
	for i := start; i < len(data); i++ {
		if data[i] == 0 {
			end = i
			break
		}
	}

	if start >= end {
		return ""
	}

	return strings.TrimSpace(string(data[start:end]))
}
