// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package macosunifiedloggingencodingextension

import (
	"encoding/binary"
	"fmt"
	"strings"
)

// ParseFirehoseChunk parses a Firehose chunk (0x6001 and variants) containing multiple individual log entries
// Returns a slice of TraceV3Entry representing each individual log event within the chunk
func ParseFirehoseChunk(data []byte, entry *TraceV3Entry, header *TraceV3Header) []*TraceV3Entry {
	var entries []*TraceV3Entry

	// Parse firehose chunk with enhanced subsystem/category mapping

	if len(data) < 48 { // Minimum size for firehose preamble
		entry.Message = fmt.Sprintf("Firehose chunk too small: %d bytes", len(data))
		return []*TraceV3Entry{entry}
	}

	// Parse firehose preamble fields (same structure as before)
	firstProcID := binary.LittleEndian.Uint64(data[16:24])
	secondProcID := binary.LittleEndian.Uint32(data[24:28])
	ttl := data[28]
	collapsed := data[29]
	baseContinuousTime := binary.LittleEndian.Uint64(data[40:48])

	// Look for public data section
	if len(data) < 52 {
		entry.Message = fmt.Sprintf("Firehose chunk missing public data section: %d bytes", len(data))
		return []*TraceV3Entry{entry}
	}

	publicDataSize := binary.LittleEndian.Uint16(data[48:50])
	privateDataOffset := binary.LittleEndian.Uint16(data[50:52])

	// publicDataSize includes 16 bytes for the firehose preamble itself

	// Check for private data (0x1000 = 4096 means NO private data)
	hasPrivateData := privateDataOffset != 0x1000
	var privateData []byte

	if hasPrivateData {
		// Calculate private data location
		// private_data_offset = 0x1000 - private_data_virtual_offset
		actualPrivateDataOffset := 0x1000 - int(privateDataOffset)

		// Private data comes after public data
		privateDataStart := 52 + int(publicDataSize)
		if len(data) > privateDataStart+actualPrivateDataOffset {
			privateData = data[privateDataStart+actualPrivateDataOffset:]
		}
	}

	// Focus on private data parsing since that's where the real log content is
	if hasPrivateData && len(privateData) > 0 {
		// Parse private data to extract log messages
		privateEntries := parsePrivateData(privateData, privateDataOffset, firstProcID, secondProcID, baseContinuousTime, header)
		entries = append(entries, privateEntries...)
	}

	// If we have larger public data, try to parse it as well (but focus on private data)
	if publicDataSize > 24 && len(data) >= int(52+publicDataSize) {
		var actualPublicDataSize uint16
		var publicDataStart int

		if publicDataSize > 16 {
			actualPublicDataSize = publicDataSize - 16
			publicDataStart = 52
		} else {
			actualPublicDataSize = publicDataSize
			publicDataStart = 52
		}

		if len(data) >= int(publicDataStart)+int(actualPublicDataSize) {
			publicData := data[publicDataStart : publicDataStart+int(actualPublicDataSize)]
			publicEntries := parseIndividualFirehoseEntries(publicData, header, firstProcID, secondProcID, baseContinuousTime, ttl, collapsed)
			entries = append(entries, publicEntries...)
		}
	}

	// If no individual entries found, create a summary entry with private data info
	if len(entries) == 0 {
		entry.ThreadID = firstProcID
		entry.ProcessID = secondProcID
		entry.Timestamp = baseContinuousTime

		privateInfo := "no private data"
		if hasPrivateData {
			privateInfo = fmt.Sprintf("private data: %d bytes", len(privateData))
		}

		entry.Message = fmt.Sprintf("Firehose chunk: ttl=%d collapsed=%d publicSize=%d privateOffset=0x%x (%s)",
			ttl, collapsed, publicDataSize, privateDataOffset, privateInfo)
		entry.Level = "Info"
		entry.Category = "firehose_chunk"
		entries = []*TraceV3Entry{entry}
	}

	return entries
}

// parseIndividualFirehoseEntries parses multiple individual log entries from the firehose public data section
func parseIndividualFirehoseEntries(publicData []byte, header *TraceV3Header, firstProcID uint64, secondProcID uint32, baseContinuousTime uint64, ttl, collapsed uint8) []*TraceV3Entry {
	var entries []*TraceV3Entry
	offset := 0

	// Valid log types from rust implementation
	validLogTypes := map[uint8]bool{
		0x2: true, // Activity
		0x4: true, // Non-activity (logs)
		0x6: true, // Signpost
		0x7: true, // Loss
		0x3: true, // Trace
	}

	const remnantData = 0x0 // When we encounter this, stop parsing

	// Parse individual entries from public data
	for offset < len(publicData) {
		// Each individual firehose entry starts with a 24-byte header (matching rust implementation)
		// 1+1+2+4+8+4+2+2 = 24 bytes total
		if offset+24 > len(publicData) {
			break
		}

		// Parse individual firehose entry header (matches rust parse_firehose structure exactly)
		logActivityType := publicData[offset]                                          // 1 byte
		logType := publicData[offset+1]                                                // 1 byte
		flags := binary.LittleEndian.Uint16(publicData[offset+2:])                     // 2 bytes
		formatStringLocation := binary.LittleEndian.Uint32(publicData[offset+4:])      // 4 bytes
		threadID := binary.LittleEndian.Uint64(publicData[offset+8:])                  // 8 bytes
		continuousTimeDelta := binary.LittleEndian.Uint32(publicData[offset+16:])      // 4 bytes
		continuousTimeDeltaUpper := binary.LittleEndian.Uint16(publicData[offset+20:]) // 2 bytes
		dataSize := binary.LittleEndian.Uint16(publicData[offset+22:])                 // 2 bytes

		// Check for remnant data (rust implementation stops here)
		if logActivityType == remnantData {
			break
		}

		// Validate log activity type (rust implementation validation)
		if !validLogTypes[logActivityType] {
			// Invalid log type, skip this entry
			offset += 1 // More conservative - advance by 1 byte only
			continue
		}

		// Check if remaining data is sufficient (rust implementation check)
		if len(publicData)-offset < 24 {
			break
		}

		// Additional validation: check if data size is reasonable
		if dataSize > uint16(len(publicData)-offset-24) {
			// Data size exceeds available data, this entry is probably malformed
			offset += 1
			continue
		}

		// Verify we have enough data for this entry
		if offset+24+int(dataSize) > len(publicData) {
			// Not enough data for this entry, break out
			break
		}

		// Create FirehoseEntry structure for enhanced parsing
		firehoseEntry := &FirehoseEntry{
			ActivityType:         logActivityType,
			LogType:              logType,
			Flags:                flags,
			FormatStringLocation: formatStringLocation,
			ThreadID:             threadID,
			TimeDelta:            continuousTimeDelta,
			TimeDeltaUpper:       continuousTimeDeltaUpper,
			DataSize:             dataSize,
		}

		// Extract message data if present
		if dataSize > 0 {
			firehoseEntry.MessageData = publicData[offset+24 : offset+24+int(dataSize)]
		}

		// Calculate the combined continuous time (6 bytes total: 4 + 2)
		if int(dataSize) >= 0 { // Accept any valid data size
			combinedTimeDelta := uint64(continuousTimeDelta) | (uint64(continuousTimeDeltaUpper) << 32)

			// Parse subsystem ID using enhanced parsing
			subsystemID := parseSubsystemFromEntry(firehoseEntry)
			firehoseEntry.SubsystemID = subsystemID

			// Use the original parsing as fallback
			_, actualSubsystemName, actualCategoryName, useSharedCache := parseFirehoseEntrySubsystem(firehoseEntry.MessageData, flags, firstProcID, secondProcID)

			// Use catalog to resolve process information if available
			actualPID := secondProcID
			subsystemName := actualSubsystemName // Use parsed subsystem name as default
			categoryName := actualCategoryName   // Use parsed category name as default

			if GlobalCatalog != nil {
				// Try to resolve using the chunk's process IDs
				if resolvedPID := GlobalCatalog.GetPID(firstProcID, secondProcID); resolvedPID != 0 {
					actualPID = resolvedPID
				}

				// Try to resolve subsystem information using the actual subsystem ID from the log entry
				if subsystemID != 0 { // Only lookup if we have a valid subsystem ID
					if subsysInfo := GlobalCatalog.GetSubsystem(subsystemID, firstProcID, secondProcID); subsysInfo.Subsystem != "Unknown subsystem" && subsysInfo.Subsystem != "" {
						subsystemName = subsysInfo.Subsystem
						if subsysInfo.Category != "" {
							categoryName = subsysInfo.Category
						}
					}
				}
			}

			// Create individual log entry
			logEntry := &TraceV3Entry{
				Type:         0x6001,                                 // Firehose chunk type
				Size:         uint32(24 + dataSize),                  // Header + data size (24-byte header)
				Timestamp:    baseContinuousTime + combinedTimeDelta, // Calculate actual timestamp using combined delta
				ThreadID:     threadID,
				ProcessID:    actualPID, // Use catalog-resolved PID when available
				ChunkType:    "firehose",
				Subsystem:    subsystemName, // Use catalog-resolved subsystem when available
				Category:     categoryName,  // Use catalog-resolved category when available
				TimezoneName: extractTimezoneName(header.TimezonePath),
			}

			// Determine log level and message type based on log type and activity type
			logEntry.MessageType = getLogType(logType, logActivityType)
			logEntry.Level = logEntry.MessageType // Keep Level for backward compatibility

			// Determine event type based on activity type
			logEntry.EventType = getEventType(logActivityType)

			// Determine entry category based on activity type
			switch logActivityType {
			case 0x2:
				logEntry.Category = "activity"
			case 0x4:
				logEntry.Category = "log"
			case 0x6:
				logEntry.Category = "signpost"
			case 0x3:
				logEntry.Category = "trace"
			case 0x7:
				logEntry.Category = "loss"
			default:
				logEntry.Category = fmt.Sprintf("unknown(0x%x)", logActivityType)
			}

			// Determine shared cache usage with enhanced method
			useSharedCacheEnhanced := shouldUseSharedCache(firehoseEntry)
			if !useSharedCacheEnhanced {
				useSharedCacheEnhanced = useSharedCache // Fallback to original logic
			}

			// Resolve format string using UUID references (with enhanced shared cache detection)
			formatData := GetFormatString(formatStringLocation, firstProcID, secondProcID, useSharedCacheEnhanced)

			// Extract message content using enhanced parsing
			messageContent := extractMessageContent(firehoseEntry)

			// Create descriptive message with resolved format string
			catalogInfo := ""
			if GlobalCatalog != nil {
				if euid := GlobalCatalog.GetEUID(firstProcID, secondProcID); euid != 0 {
					catalogInfo = fmt.Sprintf(" uid=%d", euid)
				}
			}

			// Use resolved format string and process information
			if formatData.FormatString != "" && formatData.FormatString != fmt.Sprintf("raw_format_0x%x", formatStringLocation) {
				// We have a resolved format string
				logEntry.Message = fmt.Sprintf("Format: %s | Process: %s | Thread: %d | %s%s",
					formatData.FormatString, formatData.Process, threadID, messageContent, catalogInfo)
			} else {
				// Enhanced fallback with activity type details
				logEntry.Message = fmt.Sprintf("Firehose %s: level=%s flags=0x%x format=0x%x thread=%d delta=%d subsys_id=%d%s | %s",
					mapActivityTypeToString(logActivityType), logEntry.Level, flags, formatStringLocation,
					threadID, combinedTimeDelta, subsystemID, catalogInfo, messageContent)
			}

			entries = append(entries, logEntry)
		}

		// Move to next entry (24-byte header + data_size bytes)
		offset += 24 + int(dataSize)

		// Safety check to prevent infinite loops with reasonable limit
		if len(entries) >= 10000 {
			break
		}
	}

	return entries
}

// parseFirehoseMessageData attempts to extract readable message data from firehose entry data
func parseFirehoseMessageData(data []byte) string {
	if len(data) == 0 {
		return ""
	}

	// For now, return a simple representation - this will be enhanced later
	// to parse actual message items (strings, numbers, etc.) based on the format string
	if len(data) < 8 {
		return fmt.Sprintf("raw:%x", data)
	}

	// Try to extract basic message structure
	// The message format depends on the number of items and their types
	numberItems := data[1] // Second byte typically contains number of items

	if numberItems == 0 {
		return "empty"
	}

	return fmt.Sprintf("items:%d raw:%x", numberItems, data[:min(len(data), 16)])
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// parseFirehoseEntrySubsystem extracts subsystem information from firehose entry data
// This implements the logic from the Rust FirehoseNonActivity parser to extract subsystem_value
func parseFirehoseEntrySubsystem(entryData []byte, flags uint16, firstProcID uint64, secondProcID uint32) (uint16, string, string, bool) {
	// Default values
	subsystemID := uint16(0)
	subsystemName := "com.apple.firehose"
	categoryName := "log"
	useSharedCache := false

	if len(entryData) == 0 {
		return subsystemID, subsystemName, categoryName, useSharedCache
	}

	offset := 0

	// Parse based on firehose flags following the Rust implementation pattern
	// Check for has_current_aid flag (0x0001)
	const activityIDCurrent = 0x0001
	if (flags & activityIDCurrent) != 0 {
		// Skip unknown_activity_id (4 bytes) and unknown_sentinel (4 bytes)
		offset += 8
		if offset > len(entryData) {
			return subsystemID, subsystemName, categoryName, useSharedCache
		}
	}

	// Check for has_private_data flag (0x0100)
	const privateStringRange = 0x0100
	if (flags & privateStringRange) != 0 {
		// Skip private_strings_offset (2 bytes) and private_strings_size (2 bytes)
		offset += 4
		if offset > len(entryData) {
			return subsystemID, subsystemName, categoryName, useSharedCache
		}
	}

	// Skip unknown_pc_id (4 bytes) - always present
	offset += 4
	if offset > len(entryData) {
		return subsystemID, subsystemName, categoryName, useSharedCache
	}

	// Parse formatter flags to determine if shared cache is used
	// Based on the rust implementation's FirehoseFormatters logic
	if offset < len(entryData) {
		// Check for shared cache or main executable flags in the formatter
		// This is a simplified implementation - full parsing would be more complex
		if offset+1 < len(entryData) {
			formatterByte := entryData[offset]
			// Check for shared_cache flag (bit 0) or main_exe flag
			useSharedCache = (formatterByte & 0x01) != 0
			offset += 1 // Skip formatter data for now
		}
	}

	// Check for has_subsystem flag (0x0200) - this is what we need!
	const hasSubsystem = 0x0200
	if (flags & hasSubsystem) != 0 {
		// The subsystem_value is stored as a uint16 at this position
		if offset+2 <= len(entryData) {
			subsystemID = binary.LittleEndian.Uint16(entryData[offset : offset+2])
		}
	}

	return subsystemID, subsystemName, categoryName, useSharedCache
}

// FirehoseEntry represents a parsed firehose log entry
type FirehoseEntry struct {
	ActivityType         uint8
	LogType              uint8
	Flags                uint16
	FormatStringLocation uint32
	ThreadID             uint64
	TimeDelta            uint32
	TimeDeltaUpper       uint16
	DataSize             uint16
	SubsystemID          uint16
	MessageData          []byte
}

// parseSubsystemFromEntry extracts subsystem ID from firehose entry data based on activity type
func parseSubsystemFromEntry(entry *FirehoseEntry) uint16 {
	if len(entry.MessageData) < 8 {
		return 0
	}

	offset := 0

	// Parse based on activity type (following rust implementation patterns)
	switch entry.ActivityType {
	case 0x4: // Non-activity (regular logs)
		// Skip activity ID and sentinel if present
		if (entry.Flags & 0x0001) != 0 { // has_current_aid
			offset += 8 // unknown_activity_id (4) + unknown_sentinel (4)
		}

		// Skip private data offsets if present
		if (entry.Flags & 0x0100) != 0 { // has_private_data
			offset += 4 // private_strings_offset (2) + private_strings_size (2)
		}

		// Skip unknown_pc_id (4 bytes) - always present
		offset += 4

		// Parse formatter data (simplified)
		if offset < len(entry.MessageData) {
			offset += 1 // Skip formatter for now
		}

		// Check for subsystem flag
		if (entry.Flags&0x0200) != 0 && offset+2 <= len(entry.MessageData) { // has_subsystem
			return binary.LittleEndian.Uint16(entry.MessageData[offset : offset+2])
		}

	case 0x2: // Activity
		// Activity parsing would be more complex, for now return 0
		return 0

	case 0x6: // Signpost
		// Signpost parsing would be different, for now return 0
		return 0
	}

	return 0
}

// shouldUseSharedCache determines if shared cache should be used for format string resolution
func shouldUseSharedCache(entry *FirehoseEntry) bool {
	if len(entry.MessageData) < 8 {
		return false
	}

	offset := 0

	// Skip to formatter section based on flags
	if (entry.Flags & 0x0001) != 0 { // has_current_aid
		offset += 8
	}

	if (entry.Flags & 0x0100) != 0 { // has_private_data
		offset += 4
	}

	offset += 4 // Skip unknown_pc_id

	// Check formatter flags (simplified)
	if offset < len(entry.MessageData) {
		formatterByte := entry.MessageData[offset]
		// Check for shared_cache flag (bit 0)
		return (formatterByte & 0x01) != 0
	}

	return false
}

// extractMessageContent extracts readable message content from firehose entry data
func extractMessageContent(entry *FirehoseEntry) string {
	if len(entry.MessageData) == 0 {
		return "empty"
	}

	// Try to extract any string data from the message data
	var parts []string

	// Look for null-terminated strings in the data
	start := 0
	for i := 0; i < len(entry.MessageData); i++ {
		if entry.MessageData[i] == 0 {
			if i > start {
				str := string(entry.MessageData[start:i])
				if isPrintableString(str) && len(str) > 2 {
					parts = append(parts, str)
				}
			}
			start = i + 1
		}
	}

	// If we found strings, return them
	if len(parts) > 0 {
		return strings.Join(parts, " | ")
	}

	// Otherwise return size and activity type info
	return fmt.Sprintf("data_size=%d activity_type=0x%x flags=0x%x",
		len(entry.MessageData), entry.ActivityType, entry.Flags)
}

// isPrintableString checks if a string contains mostly printable characters
func isPrintableString(s string) bool {
	if len(s) < 3 {
		return false
	}

	printableCount := 0
	for _, r := range s {
		if r >= 32 && r <= 126 { // Printable ASCII range
			printableCount++
		}
	}

	return float64(printableCount)/float64(len(s)) > 0.7 // At least 70% printable
}

// mapActivityTypeToCategory maps activity type to a human-readable category
func mapActivityTypeToCategory(activityType uint8) string {
	switch activityType {
	case 0x2:
		return "activity"
	case 0x4:
		return "log"
	case 0x6:
		return "signpost"
	case 0x7:
		return "loss"
	case 0x3:
		return "trace"
	default:
		return fmt.Sprintf("unknown_0x%x", activityType)
	}
}

// mapActivityTypeToString maps activity type to a descriptive string
func mapActivityTypeToString(activityType uint8) string {
	switch activityType {
	case 0x2:
		return "Activity"
	case 0x4:
		return "Log"
	case 0x6:
		return "Signpost"
	case 0x7:
		return "Loss"
	case 0x3:
		return "Trace"
	default:
		return fmt.Sprintf("Unknown(0x%x)", activityType)
	}
}

// parsePrivateData extracts log messages from firehose private data sections
// This is where the actual log content is stored in most firehose chunks
func parsePrivateData(privateData []byte, privateDataOffset uint16, firstProcID uint64, secondProcID uint32, baseContinuousTime uint64, header *TraceV3Header) []*TraceV3Entry {
	var entries []*TraceV3Entry

	if len(privateData) == 0 {
		return entries
	}

	// Skip any null padding at the beginning
	offset := 0
	for offset < len(privateData) && privateData[offset] == 0 {
		offset++
	}

	if offset >= len(privateData) {
		return entries
	}

	// Look for strings and other log content in the private data
	strings := extractStringsFromPrivateData(privateData[offset:])

	if len(strings) > 0 {
		// Create log entries from extracted strings
		for i, str := range strings {
			if len(str) < 3 || !isPrintableString(str) {
				continue // Skip very short or non-printable strings
			}

			// Use catalog to resolve process information if available
			actualPID := secondProcID
			subsystemName := "com.apple.firehose.private"
			categoryName := "log"

			if GlobalCatalog != nil {
				if resolvedPID := GlobalCatalog.GetPID(firstProcID, secondProcID); resolvedPID != 0 {
					actualPID = resolvedPID
				}

				// Try to get subsystem info (subsystem ID would come from associated public data)
				if subsysInfo := GlobalCatalog.GetSubsystem(0, firstProcID, secondProcID); subsysInfo.Subsystem != "Unknown subsystem" && subsysInfo.Subsystem != "" {
					subsystemName = subsysInfo.Subsystem
					if subsysInfo.Category != "" {
						categoryName = subsysInfo.Category
					}
				}
			}

			logEntry := &TraceV3Entry{
				Type:         0x6001,
				Size:         uint32(len(str)),
				Timestamp:    baseContinuousTime + uint64(i)*1000, // Small time deltas for multiple strings
				ThreadID:     0,
				ProcessID:    actualPID,
				ChunkType:    "firehose_private",
				Subsystem:    subsystemName,
				Category:     categoryName,
				Level:        "Info",
				MessageType:  "Log",
				EventType:    "firehose",
				TimezoneName: extractTimezoneName(header.TimezonePath),
				Message:      str,
			}

			entries = append(entries, logEntry)
		}
	}

	// If no strings found, create a summary entry about the private data
	if len(entries) == 0 {
		logEntry := &TraceV3Entry{
			Type:         0x6001,
			Size:         uint32(len(privateData)),
			Timestamp:    baseContinuousTime,
			ThreadID:     0,
			ProcessID:    secondProcID,
			ChunkType:    "firehose_private",
			Subsystem:    "com.apple.firehose.private",
			Category:     "data",
			Level:        "Info",
			MessageType:  "Log",
			EventType:    "firehose",
			TimezoneName: extractTimezoneName(header.TimezonePath),
			Message:      fmt.Sprintf("Private data: %d bytes, privateOffset=0x%x", len(privateData), privateDataOffset),
		}

		entries = append(entries, logEntry)
	}

	return entries
}

// extractStringsFromPrivateData finds null-terminated strings in private data
func extractStringsFromPrivateData(data []byte) []string {
	var strings []string

	if len(data) == 0 {
		return strings
	}

	start := 0
	for i := 0; i < len(data); i++ {
		if data[i] == 0 {
			// Found null terminator
			if i > start {
				str := string(data[start:i])
				// Only include strings that look like log content
				if isLogString(str) {
					strings = append(strings, str)
				}
			}
			start = i + 1
		}
	}

	// Handle case where there's no null terminator at the end
	if start < len(data) {
		str := string(data[start:])
		if isLogString(str) {
			strings = append(strings, str)
		}
	}

	return strings
}

// isLogString determines if a string looks like actual log content
func isLogString(s string) bool {
	if len(s) < 3 {
		return false
	}

	// Check for common log content patterns
	logIndicators := []string{
		"error", "warn", "info", "debug", "trace",
		"failed", "success", "start", "stop", "end",
		"process", "service", "event", "message",
		"config", "init", "load", "save", "open", "close",
		"connect", "disconnect", "request", "response",
		"exception", "crash", "panic", "abort",
	}

	lowerStr := strings.ToLower(s)

	// Check if it contains common log indicators
	for _, indicator := range logIndicators {
		if strings.Contains(lowerStr, indicator) {
			return true
		}
	}

	// Check if it looks like a path, URL, or identifier
	if strings.Contains(s, "/") || strings.Contains(s, ".") || strings.Contains(s, ":") {
		return isPrintableString(s)
	}

	// Check if it's mostly printable and has reasonable content
	if isPrintableString(s) && len(s) >= 10 {
		// Count alphabetic characters
		alphaCount := 0
		for _, r := range s {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				alphaCount++
			}
		}

		// Must have at least 30% alphabetic characters for longer strings
		return float64(alphaCount)/float64(len(s)) >= 0.3
	}

	return false
}
