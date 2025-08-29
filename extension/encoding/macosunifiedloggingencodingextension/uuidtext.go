// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package macosunifiedloggingencodingextension

import (
	"encoding/binary"
	"fmt"
	"path/filepath"
	"strings"
)

// UUIDText represents a parsed UUID text file containing format strings
type UUIDText struct {
	UUID                string          `json:"uuid"`
	Signature           uint32          `json:"signature"`
	UnknownMajorVersion uint32          `json:"unknown_major_version"`
	UnknownMinorVersion uint32          `json:"unknown_minor_version"`
	NumberEntries       uint32          `json:"number_entries"`
	EntryDescriptors    []UUIDTextEntry `json:"entry_descriptors"`
	FooterData          []byte          `json:"footer_data"` // Collection of strings containing format strings
}

// UUIDTextEntry represents an entry descriptor in a UUID text file
type UUIDTextEntry struct {
	RangeStartOffset uint32 `json:"range_start_offset"`
	EntrySize        uint32 `json:"entry_size"`
}

// MessageData holds resolved format string information
type MessageData struct {
	Library      string `json:"library"`
	FormatString string `json:"format_string"`
	Process      string `json:"process"`
	LibraryUUID  string `json:"library_uuid"`
	ProcessUUID  string `json:"process_uuid"`
}

// UUIDTextCache stores parsed UUID text files for format string resolution
var UUIDTextCache = make(map[string]*UUIDText)

// ParseUUIDText parses a UUID text file containing format strings
// Based on the rust implementation in uuidtext.rs
func ParseUUIDText(data []byte, uuid string) (*UUIDText, error) {
	if len(data) < 16 {
		return nil, fmt.Errorf("UUID text file too small: %d bytes", len(data))
	}

	uuidText := &UUIDText{UUID: uuid}
	offset := 0

	// Parse header (16 bytes)
	uuidText.Signature = binary.LittleEndian.Uint32(data[offset : offset+4])
	offset += 4
	uuidText.UnknownMajorVersion = binary.LittleEndian.Uint32(data[offset : offset+4])
	offset += 4
	uuidText.UnknownMinorVersion = binary.LittleEndian.Uint32(data[offset : offset+4])
	offset += 4
	uuidText.NumberEntries = binary.LittleEndian.Uint32(data[offset : offset+4])
	offset += 4

	// Validate signature
	const expectedSignature = 0x66778899
	if uuidText.Signature != expectedSignature {
		return nil, fmt.Errorf("invalid UUID text signature: expected 0x%x, got 0x%x",
			expectedSignature, uuidText.Signature)
	}

	// Parse entry descriptors
	uuidText.EntryDescriptors = make([]UUIDTextEntry, uuidText.NumberEntries)
	for i := uint32(0); i < uuidText.NumberEntries; i++ {
		if len(data) < offset+8 {
			return nil, fmt.Errorf("UUID text file too small for entry %d", i)
		}

		entry := UUIDTextEntry{
			RangeStartOffset: binary.LittleEndian.Uint32(data[offset : offset+4]),
			EntrySize:        binary.LittleEndian.Uint32(data[offset+4 : offset+8]),
		}
		uuidText.EntryDescriptors[i] = entry
		offset += 8
	}

	// Store remaining data as footer (contains the actual format strings)
	if offset < len(data) {
		uuidText.FooterData = make([]byte, len(data)-offset)
		copy(uuidText.FooterData, data[offset:])
	}

	return uuidText, nil
}

// ExtractFormatString extracts a format string from UUID text using the given offset
// Based on the rust implementation's extract_normal_strings method
func (u *UUIDText) ExtractFormatString(stringOffset uint64) (MessageData, error) {
	messageData := MessageData{
		ProcessUUID: u.UUID,
	}

	// Handle dynamic formatters (offset with high bit set means "%s")
	if stringOffset&0x80000000 != 0 {
		messageData.FormatString = "%s"
		return messageData, nil
	}

	// Search through entry descriptors to find the correct range
	var stringStart uint32 = 0
	for _, entry := range u.EntryDescriptors {
		// Check if offset falls within this entry's range
		if entry.RangeStartOffset > uint32(stringOffset) {
			stringStart += entry.EntrySize
			continue
		}

		offset := uint32(stringOffset) - entry.RangeStartOffset

		// Validate offset is within bounds
		if offset > entry.EntrySize || int(offset+stringStart) >= len(u.FooterData) {
			stringStart += entry.EntrySize
			continue
		}

		// Extract the format string from footer data
		startIdx := int(offset + stringStart)
		if startIdx >= len(u.FooterData) {
			break
		}

		// Find null terminator
		endIdx := startIdx
		for endIdx < len(u.FooterData) && u.FooterData[endIdx] != 0 {
			endIdx++
		}

		if endIdx > startIdx {
			messageData.FormatString = string(u.FooterData[startIdx:endIdx])

			// Extract process/library path from footer data (typically at the end)
			processPath := u.extractImagePath()
			messageData.Process = processPath
			messageData.Library = processPath

			return messageData, nil
		}
	}

	return messageData, fmt.Errorf("format string not found at offset %d", stringOffset)
}

// extractImagePath extracts the process/library path from the footer data
// The image path is typically stored at the end of the footer data
func (u *UUIDText) extractImagePath() string {
	if len(u.FooterData) == 0 {
		return ""
	}

	// Work backwards from the end to find the image path
	// The image path is usually the last null-terminated string
	data := u.FooterData

	// Find the last non-null byte
	end := len(data) - 1
	for end >= 0 && data[end] == 0 {
		end--
	}

	if end < 0 {
		return ""
	}

	// Find the start of the last string
	start := end
	for start >= 0 && data[start] != 0 {
		start--
	}
	start++ // Move past the null byte

	if start <= end {
		path := string(data[start : end+1])
		// Clean up the path
		path = strings.TrimSpace(path)
		if path != "" {
			return path
		}
	}

	return ""
}

// GetFormatString resolves a format string using the catalog and UUID references
// This is the main entry point for format string resolution
// Based on the rust implementation's get_firehose_nonactivity_strings method
func GetFormatString(formatStringLocation uint32, firstProcID uint64, secondProcID uint32, useSharedCache bool) MessageData {
	messageData := MessageData{
		FormatString: fmt.Sprintf("raw_format_0x%x", formatStringLocation),
		Process:      "unknown",
		Library:      "unknown",
	}

	if GlobalCatalog == nil {
		return messageData
	}

	// Determine whether to use shared cache or UUID text based on flags
	if useSharedCache {
		// Use shared cache (DSC) for format string resolution
		return GetSharedFormatString(formatStringLocation, firstProcID, secondProcID)
	}

	// Get the process entry from catalog to find the main UUID
	var mainUUID string
	for _, procEntry := range GlobalCatalog.ProcessInfoEntries {
		if procEntry.FirstNumberProcID == firstProcID && procEntry.SecondNumberProcID == secondProcID {
			mainUUID = procEntry.MainUUID
			messageData.ProcessUUID = mainUUID
			messageData.LibraryUUID = procEntry.DSCUUID
			break
		}
	}

	if mainUUID == "" {
		return messageData
	}

	// Check if we have the UUID text file cached
	uuidText, exists := UUIDTextCache[mainUUID]
	if !exists {
		// In a full implementation, we would load the UUID text file here
		// For now, return what we have
		messageData.FormatString = fmt.Sprintf("uuid_%s_offset_0x%x", mainUUID[:8], formatStringLocation)
		return messageData
	}

	// Extract the format string using the location offset
	resolvedData, err := uuidText.ExtractFormatString(uint64(formatStringLocation))
	if err != nil {
		messageData.FormatString = fmt.Sprintf("error_%s", err.Error())
		return messageData
	}

	// Merge the resolved data
	if resolvedData.FormatString != "" {
		messageData.FormatString = resolvedData.FormatString
	}
	if resolvedData.Process != "" {
		messageData.Process = resolvedData.Process
	}
	if resolvedData.Library != "" {
		messageData.Library = resolvedData.Library
	}

	return messageData
}

// LoadUUIDTextFile loads and parses a UUID text file into the cache
// This would be called when scanning a logarchive directory structure
func LoadUUIDTextFile(filePath string, data []byte) error {
	// Extract UUID from filename
	filename := filepath.Base(filePath)
	uuid := strings.ToUpper(filename)

	// Parse the UUID text file
	uuidText, err := ParseUUIDText(data, uuid)
	if err != nil {
		return fmt.Errorf("failed to parse UUID text file %s: %w", filePath, err)
	}

	// Cache the parsed data
	UUIDTextCache[uuid] = uuidText

	return nil
}
