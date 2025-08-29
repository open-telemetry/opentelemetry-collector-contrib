// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package macosunifiedloggingencodingextension

import (
	"encoding/binary"
	"fmt"
	"path/filepath"
	"strings"
)

// SharedCacheStrings represents parsed DSC (shared cache) string data
type SharedCacheStrings struct {
	DSCUUID   string             `json:"dsc_uuid"`
	Signature uint32             `json:"signature"`
	UUIDs     []SharedCacheUUID  `json:"uuids"`
	Ranges    []SharedCacheRange `json:"ranges"`
}

// SharedCacheUUID represents a UUID entry in the shared cache
type SharedCacheUUID struct {
	UUID       string `json:"uuid"`
	PathString string `json:"path_string"`
}

// SharedCacheRange represents a string range in the shared cache
type SharedCacheRange struct {
	RangeOffset      uint64 `json:"range_offset"`
	RangeSize        uint32 `json:"range_size"`
	UnknownUUIDIndex uint32 `json:"unknown_uuid_index"`
	Strings          []byte `json:"strings"`
}

// DSCCache stores parsed DSC files for shared string resolution
var DSCCache = make(map[string]*SharedCacheStrings)

// ParseDSC parses a DSC (shared cache) file containing shared format strings
// Based on the rust implementation in dsc.rs
func ParseDSC(data []byte, uuid string) (*SharedCacheStrings, error) {
	if len(data) < 8 {
		return nil, fmt.Errorf("DSC file too small: %d bytes", len(data))
	}

	dsc := &SharedCacheStrings{DSCUUID: uuid}
	offset := 0

	// Parse signature
	dsc.Signature = binary.LittleEndian.Uint32(data[offset : offset+4])
	offset += 4

	// Validate signature
	const expectedSignature = 0x68736964 // "dsih" in little endian
	if dsc.Signature != expectedSignature {
		return nil, fmt.Errorf("invalid DSC signature: expected 0x%x, got 0x%x",
			expectedSignature, dsc.Signature)
	}

	// Parse number of UUIDs
	if len(data) < offset+4 {
		return nil, fmt.Errorf("DSC file too small for UUID count")
	}
	numUUIDs := binary.LittleEndian.Uint32(data[offset : offset+4])
	offset += 4

	// Parse UUIDs
	dsc.UUIDs = make([]SharedCacheUUID, numUUIDs)
	for i := uint32(0); i < numUUIDs; i++ {
		if len(data) < offset+16 {
			return nil, fmt.Errorf("DSC file too small for UUID %d", i)
		}

		// Parse UUID (16 bytes)
		uuidBytes := data[offset : offset+16]
		uuid := fmt.Sprintf("%02X%02X%02X%02X%02X%02X%02X%02X%02X%02X%02X%02X%02X%02X%02X%02X",
			uuidBytes[0], uuidBytes[1], uuidBytes[2], uuidBytes[3],
			uuidBytes[4], uuidBytes[5], uuidBytes[6], uuidBytes[7],
			uuidBytes[8], uuidBytes[9], uuidBytes[10], uuidBytes[11],
			uuidBytes[12], uuidBytes[13], uuidBytes[14], uuidBytes[15])
		offset += 16

		dsc.UUIDs[i] = SharedCacheUUID{UUID: uuid}
	}

	// Parse path strings for UUIDs
	for i := range dsc.UUIDs {
		if offset >= len(data) {
			break
		}

		// Find null terminator
		end := offset
		for end < len(data) && data[end] != 0 {
			end++
		}

		if end > offset {
			dsc.UUIDs[i].PathString = string(data[offset:end])
		}
		offset = end + 1 // Skip null terminator
	}

	// Parse ranges (simplified - in full implementation would parse all range data)
	// For now, create a single range covering remaining data
	if offset < len(data) {
		remainingData := data[offset:]
		dscRange := SharedCacheRange{
			RangeOffset:      0,
			RangeSize:        uint32(len(remainingData)),
			UnknownUUIDIndex: 0,
			Strings:          remainingData,
		}
		dsc.Ranges = append(dsc.Ranges, dscRange)
	}

	return dsc, nil
}

// ExtractSharedString extracts a format string from shared cache using the given offset
// Based on the rust implementation's extract_shared_strings method
func (d *SharedCacheStrings) ExtractSharedString(stringOffset uint64) (MessageData, error) {
	messageData := MessageData{}

	// Handle dynamic formatters (offset with high bit set means "%s")
	if stringOffset&0x80000000 != 0 {
		messageData.FormatString = "%s"
		if len(d.UUIDs) > 0 {
			messageData.Library = d.UUIDs[0].PathString
			messageData.LibraryUUID = d.UUIDs[0].UUID
		}
		return messageData, nil
	}

	// Search through ranges to find the correct string
	for _, r := range d.Ranges {
		if stringOffset >= r.RangeOffset && stringOffset < (r.RangeOffset+uint64(r.RangeSize)) {
			offset := stringOffset - r.RangeOffset

			if int(offset) >= len(r.Strings) {
				continue
			}

			// Extract the format string
			startIdx := int(offset)
			endIdx := startIdx
			for endIdx < len(r.Strings) && r.Strings[endIdx] != 0 {
				endIdx++
			}

			if endIdx > startIdx {
				messageData.FormatString = string(r.Strings[startIdx:endIdx])

				// Get library information from UUID
				if int(r.UnknownUUIDIndex) < len(d.UUIDs) {
					messageData.Library = d.UUIDs[r.UnknownUUIDIndex].PathString
					messageData.LibraryUUID = d.UUIDs[r.UnknownUUIDIndex].UUID
				}

				return messageData, nil
			}
		}
	}

	return messageData, fmt.Errorf("shared string not found at offset %d", stringOffset)
}

// GetSharedFormatString resolves a format string using the catalog and DSC references
// This handles shared cache format string resolution
func GetSharedFormatString(formatStringLocation uint32, firstProcID uint64, secondProcID uint32) MessageData {
	messageData := MessageData{
		FormatString: fmt.Sprintf("shared_format_0x%x", formatStringLocation),
		Process:      "unknown",
		Library:      "unknown",
	}

	if GlobalCatalog == nil {
		return messageData
	}

	// Get the process entry from catalog to find the DSC UUID
	var dscUUID string
	var mainUUID string
	for _, procEntry := range GlobalCatalog.ProcessInfoEntries {
		if procEntry.FirstNumberProcID == firstProcID && procEntry.SecondNumberProcID == secondProcID {
			dscUUID = procEntry.DSCUUID
			mainUUID = procEntry.MainUUID
			messageData.ProcessUUID = mainUUID
			break
		}
	}

	if dscUUID == "" {
		return messageData
	}

	// Check if we have the DSC file cached
	dscData, exists := DSCCache[dscUUID]
	if !exists {
		// In a full implementation, we would load the DSC file here
		messageData.FormatString = fmt.Sprintf("dsc_%s_offset_0x%x", dscUUID[:8], formatStringLocation)
		return messageData
	}

	// Extract the format string using the location offset
	resolvedData, err := dscData.ExtractSharedString(uint64(formatStringLocation))
	if err != nil {
		messageData.FormatString = fmt.Sprintf("shared_error_%s", err.Error())
		return messageData
	}

	// Merge the resolved data
	if resolvedData.FormatString != "" {
		messageData.FormatString = resolvedData.FormatString
	}
	if resolvedData.Library != "" {
		messageData.Library = resolvedData.Library
	}
	if resolvedData.LibraryUUID != "" {
		messageData.LibraryUUID = resolvedData.LibraryUUID
	}

	return messageData
}

// LoadDSCFile loads and parses a DSC file into the cache
// This would be called when scanning a logarchive directory structure
func LoadDSCFile(filePath string, data []byte) error {
	// Extract UUID from filename
	filename := strings.TrimSuffix(strings.ToUpper(filepath.Base(filePath)), ".DSC")

	// Parse the DSC file
	dscData, err := ParseDSC(data, filename)
	if err != nil {
		return fmt.Errorf("failed to parse DSC file %s: %w", filePath, err)
	}

	// Cache the parsed data
	DSCCache[filename] = dscData

	return nil
}
