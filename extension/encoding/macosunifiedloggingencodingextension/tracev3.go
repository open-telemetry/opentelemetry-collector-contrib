// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package macosunifiedloggingencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/macosunifiedloggingencodingextension"

import (
	"encoding/binary"
	"fmt"
	"strings"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

// TraceV3Header represents the complete header of a tracev3 file
// Based on the Rust implementation from mandiant/macos-UnifiedLogs
type TraceV3Header struct {
	// Main header fields (first 48 bytes)
	ChunkTag            uint32 // File magic/signature
	ChunkSubTag         uint32 // Sub-tag identifier
	ChunkDataSize       uint64 // Size of data section
	MachTimeNumerator   uint32 // Mach time conversion numerator
	MachTimeDenominator uint32 // Mach time conversion denominator
	ContinuousTime      uint64 // Continuous time value
	UnknownTime         uint64 // Possibly start time
	Unknown             uint32 // Unknown field
	BiasMin             uint32 // Time zone bias in minutes
	DaylightSavings     uint32 // DST flag (0=no DST, 1=DST)
	UnknownFlags        uint32 // Unknown flags

	// Sub-chunk 1 (0x6100) - Timing information
	SubChunkTag            uint32 // 0x6100
	SubChunkDataSize       uint32 // Data size for this sub-chunk
	SubChunkContinuousTime uint64 // Continuous time for this sub-chunk

	// Sub-chunk 2 (0x6101) - Build and hardware info
	SubChunkTag2        uint32 // 0x6101
	SubChunkDataSize2   uint32 // Data size
	Unknown2            uint32 // Unknown field
	Unknown3            uint32 // Unknown field
	BuildVersionString  string // macOS build version (16 bytes)
	HardwareModelString string // Hardware model (32 bytes)

	// Sub-chunk 3 (0x6102) - Boot UUID and process info
	SubChunkTag3      uint32 // 0x6102
	SubChunkDataSize3 uint32 // Data size
	BootUUID          string // Boot UUID (16 bytes)
	LogdPID           uint32 // logd process ID
	LogdExitStatus    uint32 // logd exit status

	// Sub-chunk 4 (0x6103) - Timezone information
	SubChunkTag4      uint32 // 0x6103
	SubChunkDataSize4 uint32 // Data size
	TimezonePath      string // Timezone path (48 bytes)
}

// TraceV3Entry represents a single log entry in the tracev3 format
type TraceV3Entry struct {
	Type         uint32 // Entry type (log, signpost, activity, etc.)
	Size         uint32 // Size of this entry
	Timestamp    uint64 // Mach absolute time
	ThreadID     uint64 // Thread identifier
	ProcessID    uint32 // Process identifier
	Message      string // Log message content
	Subsystem    string // Subsystem (e.g., com.apple.SkyLight)
	Category     string // Category within subsystem
	Level        string // Log level (Default, Info, Debug, Error, Fault)
	ChunkType    string // Type of chunk (header, firehose, oversize, statedump, simpledump, catalog, chunkset)
	MessageType  string // Message type based on log level (Default, Debug, Info, Error, Fault, etc.)
	EventType    string // Event type (logEvent, activityEvent, traceEvent, signpostEvent, lossEvent)
	TimezoneName string // Timezone name extracted from header timezone path
}

// ParseTraceV3Header parses the tracev3 file header
func ParseTraceV3Header(data []byte) (*TraceV3Header, int, error) {
	if len(data) < 48 {
		return nil, 0, fmt.Errorf("insufficient data for header: need at least 48 bytes, got %d", len(data))
	}

	header := &TraceV3Header{}
	offset := 0

	// Parse main header (48 bytes)
	header.ChunkTag = binary.LittleEndian.Uint32(data[offset:])
	offset += 4
	header.ChunkSubTag = binary.LittleEndian.Uint32(data[offset:])
	offset += 4
	header.ChunkDataSize = binary.LittleEndian.Uint64(data[offset:])
	offset += 8
	header.MachTimeNumerator = binary.LittleEndian.Uint32(data[offset:])
	offset += 4
	header.MachTimeDenominator = binary.LittleEndian.Uint32(data[offset:])
	offset += 4
	header.ContinuousTime = binary.LittleEndian.Uint64(data[offset:])
	offset += 8
	header.UnknownTime = binary.LittleEndian.Uint64(data[offset:])
	offset += 8
	header.Unknown = binary.LittleEndian.Uint32(data[offset:])
	offset += 4
	header.BiasMin = binary.LittleEndian.Uint32(data[offset:])
	offset += 4
	header.DaylightSavings = binary.LittleEndian.Uint32(data[offset:])
	offset += 4
	header.UnknownFlags = binary.LittleEndian.Uint32(data[offset:])
	offset += 4

	// Validate basic magic numbers
	if header.ChunkTag != 0x1000 && header.ChunkTag != 0x1001 {
		return nil, 0, fmt.Errorf("invalid chunk tag: expected 0x1000 or 0x1001, got 0x%x", header.ChunkTag)
	}

	// Parse sub-chunks if there's enough data
	if offset+8 <= len(data) {
		// Sub-chunk 1 (0x6100)
		header.SubChunkTag = binary.LittleEndian.Uint32(data[offset:])
		offset += 4
		header.SubChunkDataSize = binary.LittleEndian.Uint32(data[offset:])
		offset += 4

		if header.SubChunkTag == 0x6100 && offset+8 <= len(data) {
			header.SubChunkContinuousTime = binary.LittleEndian.Uint64(data[offset:])
			offset += 8

			// Sub-chunk 2 (0x6101)
			if offset+8 <= len(data) {
				header.SubChunkTag2 = binary.LittleEndian.Uint32(data[offset:])
				offset += 4
				header.SubChunkDataSize2 = binary.LittleEndian.Uint32(data[offset:])
				offset += 4

				if header.SubChunkTag2 == 0x6101 && offset+8 <= len(data) {
					header.Unknown2 = binary.LittleEndian.Uint32(data[offset:])
					offset += 4
					header.Unknown3 = binary.LittleEndian.Uint32(data[offset:])
					offset += 4

					// Build version string (16 bytes)
					if offset+16 <= len(data) {
						header.BuildVersionString = strings.TrimRight(string(data[offset:offset+16]), "\x00")
						offset += 16
					}

					// Hardware model string (32 bytes)
					if offset+32 <= len(data) {
						header.HardwareModelString = strings.TrimRight(string(data[offset:offset+32]), "\x00")
						offset += 32
					}

					// Sub-chunk 3 (0x6102)
					if offset+8 <= len(data) {
						header.SubChunkTag3 = binary.LittleEndian.Uint32(data[offset:])
						offset += 4
						header.SubChunkDataSize3 = binary.LittleEndian.Uint32(data[offset:])
						offset += 4

						if header.SubChunkTag3 == 0x6102 && offset+24 <= len(data) {
							// Boot UUID (16 bytes)
							bootUUIDBytes := data[offset : offset+16]
							header.BootUUID = fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
								binary.BigEndian.Uint32(bootUUIDBytes[0:4]),
								binary.BigEndian.Uint16(bootUUIDBytes[4:6]),
								binary.BigEndian.Uint16(bootUUIDBytes[6:8]),
								binary.BigEndian.Uint16(bootUUIDBytes[8:10]),
								bootUUIDBytes[10:16])
							offset += 16

							header.LogdPID = binary.LittleEndian.Uint32(data[offset:])
							offset += 4
							header.LogdExitStatus = binary.LittleEndian.Uint32(data[offset:])
							offset += 4

							// Sub-chunk 4 (0x6103)
							if offset+8 <= len(data) {
								header.SubChunkTag4 = binary.LittleEndian.Uint32(data[offset:])
								offset += 4
								header.SubChunkDataSize4 = binary.LittleEndian.Uint32(data[offset:])
								offset += 4

								if header.SubChunkTag4 == 0x6103 && offset+48 <= len(data) {
									// Timezone path (48 bytes)
									header.TimezonePath = strings.TrimRight(string(data[offset:offset+48]), "\x00")
									offset += 48
								}
							}
						}
					}
				}
			}
		}
	}

	return header, offset, nil
}

// ParseTraceV3Data parses tracev3 binary data and extracts individual log entries
func ParseTraceV3Data(data []byte) ([]*TraceV3Entry, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty data")
	}

	// Parse the header first
	header, headerSize, err := ParseTraceV3Header(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse header: %w", err)
	}

	// Create a header info entry with the parsed details
	entries := []*TraceV3Entry{
		{
			Type:      0x0000, // Header info type
			Size:      uint32(headerSize),
			Timestamp: uint64(time.Now().UnixNano()),
			ThreadID:  0,
			ProcessID: header.LogdPID,
			Message: fmt.Sprintf("TraceV3 Header: chunk_tag=0x%x, build=%s, hardware=%s, boot_uuid=%s",
				header.ChunkTag, header.BuildVersionString, header.HardwareModelString, header.BootUUID),
			Subsystem:    "com.apple.logd",
			Category:     "header",
			Level:        "Info",
			MessageType:  "Info",
			EventType:    "logEvent",
			TimezoneName: extractTimezoneName(header.TimezonePath),
		},
	}

	// Skip header and try to parse some entries from the remaining data
	remainingData := data[headerSize:]
	parsedEntries := parseDataEntries(remainingData, header)
	entries = append(entries, parsedEntries...)

	if len(entries) == 1 {
		// Only header entry, add a summary of the data section
		entries = append(entries, &TraceV3Entry{
			Type:      0x0001,
			Size:      uint32(len(remainingData)),
			Timestamp: uint64(time.Now().UnixNano()),
			ThreadID:  0,
			ProcessID: header.LogdPID,
			Message: fmt.Sprintf("Data section: %d bytes remaining after %d-byte header",
				len(remainingData), headerSize),
			Subsystem:    "com.apple.logd",
			Category:     "data",
			Level:        "Info",
			MessageType:  "Info",
			EventType:    "logEvent",
			TimezoneName: extractTimezoneName(header.TimezonePath),
		})
	}

	return entries, nil
}

// paddingSize8 calculates 8-byte alignment padding like the rust implementation
func paddingSize8(dataSize uint64) uint64 {
	alignment := uint64(8)
	return (alignment - (dataSize & (alignment - 1))) & (alignment - 1)
}

// parseDataEntries attempts to parse individual entries from the data section
// Based on the Rust implementation logic from mandiant/macos-UnifiedLogs
func parseDataEntries(data []byte, header *TraceV3Header) []*TraceV3Entry {
	entries := []*TraceV3Entry{}
	offset := 0
	entryCount := 0
	chunkPreambleSize := 16 // Always 16 bytes for preamble

	for offset < len(data) {
		// Need at least 16 bytes for preamble (matching rust implementation)
		if offset+chunkPreambleSize > len(data) {
			break
		}

		// Parse preamble (detect_preamble equivalent)
		chunkTag := binary.LittleEndian.Uint32(data[offset:])
		chunkSubTag := binary.LittleEndian.Uint32(data[offset+4:])
		chunkDataSize := binary.LittleEndian.Uint64(data[offset+8:])

		// Validate chunk data size (matching rust validation)
		if chunkDataSize == 0 {
			// Skip invalid chunks
			offset += 4
			continue
		}

		// Calculate total chunk size (preamble + data, matching rust logic)
		totalChunkSize := chunkPreambleSize + int(chunkDataSize)
		if offset+totalChunkSize > len(data) {
			// Not enough data for complete chunk
			break
		}

		// Additional safety check for reasonable chunk sizes
		if chunkDataSize > uint64(100*1024*1024) { // 100MB max per chunk
			offset += 4
			continue
		}

		// Extract basic entry information
		entry := &TraceV3Entry{
			Type:         chunkTag,
			Size:         uint32(chunkDataSize),
			Timestamp:    header.ContinuousTime + uint64(entryCount)*1000000, // Use header time + offset
			ThreadID:     0,
			ProcessID:    header.LogdPID,
			Level:        "Info",
			MessageType:  "Default",
			EventType:    "logEvent",
			TimezoneName: extractTimezoneName(header.TimezonePath),
		}

		// Determine chunk type and parse accordingly
		switch chunkTag {
		case 0x6001:
			// Firehose chunk - can contain multiple individual log entries
			entry.ChunkType = "firehose"
			entry.Subsystem = "com.apple.firehose"
			entry.Category = "entry"
			entry.Message = fmt.Sprintf("Firehose chunk found: tag=0x%x sub_tag=0x%x size=%d", chunkTag, chunkSubTag, chunkDataSize)
			firehoseEntries := ParseFirehoseChunk(data[offset:offset+totalChunkSize], entry, header)
			// Add all individual firehose entries to our result
			entries = append(entries, firehoseEntries...)
			// Continue to next chunk without adding the template entry
			offset += totalChunkSize
			entryCount++
			continue
		case 0x6002:
			// Oversize chunk
			entry.ChunkType = "oversize"
			entry.Subsystem = "com.apple.oversize"
			entry.Category = "oversize_data"
			ParseOversizeChunk(data[offset:offset+int(chunkDataSize)], entry)
		case 0x6003:
			// Statedump chunk
			entry.ChunkType = "statedump"
			entry.Subsystem = "com.apple.statedump"
			entry.Category = "system_state"
			ParseStatedumpChunk(data[offset:offset+int(chunkDataSize)], entry)
		case 0x6004:
			// Simpledump chunk
			entry.ChunkType = "simpledump"
			ParseSimpledumpChunk(data[offset:offset+int(chunkDataSize)], entry)
		case 0x600b:
			// Catalog chunk
			entry.ChunkType = "catalog"
			entry.Subsystem = "com.apple.catalog"
			entry.Category = "catalog_data"
			ParseCatalogChunk(data[offset:offset+int(chunkDataSize)], entry)
		case 0x600d:
			// ChunkSet chunk - can contain many compressed individual log entries
			entry.ChunkType = "chunkset"
			entry.Subsystem = "com.apple.chunkset"
			entry.Category = "chunkset_data"
			chunksetEntries := ParseChunksetChunk(data[offset:offset+totalChunkSize], entry, header)
			// Add all individual chunkset entries to our result
			entries = append(entries, chunksetEntries...)
			// Continue to next chunk without adding the template entry
			offset += totalChunkSize
			entryCount++
			continue
		default:
			// Unknown chunk type
			entry.ChunkType = "unknown"
			entry.Subsystem = "com.apple.unknown"
			entry.Category = fmt.Sprintf("unknown_0x%x", chunkTag)
			entry.Message = fmt.Sprintf("Unknown chunk: tag=0x%x sub_tag=0x%x size=%d", chunkTag, chunkSubTag, chunkDataSize)
		}

		// Try to extract more detailed information from the firehose entry
		entryData := data[offset : offset+int(chunkDataSize)]
		if len(entryData) >= 48 { // Minimum size for firehose preamble
			// Parse firehose preamble fields
			firstProcID := binary.LittleEndian.Uint64(entryData[16:24])
			secondProcID := binary.LittleEndian.Uint32(entryData[24:28])
			baseContinuousTime := binary.LittleEndian.Uint64(entryData[40:48])

			// Look for public data section
			if len(entryData) >= 52 {
				publicDataSize := binary.LittleEndian.Uint16(entryData[48:50])
				privateDataOffset := binary.LittleEndian.Uint16(entryData[50:52])

				entry.ThreadID = firstProcID
				entry.ProcessID = secondProcID
				entry.Timestamp = baseContinuousTime

				// Try to extract log type and message information
				if publicDataSize > 0 && len(entryData) >= int(52+publicDataSize) {
					publicData := entryData[52 : 52+publicDataSize]
					if len(publicData) >= 20 {
						// Parse firehose log entry header
						logActivityType := publicData[0]
						logType := publicData[1]
						flags := binary.LittleEndian.Uint16(publicData[2:4])
						formatStringLocation := binary.LittleEndian.Uint32(publicData[4:8])
						threadID := binary.LittleEndian.Uint64(publicData[8:16])
						dataSize := binary.LittleEndian.Uint16(publicData[18:20])

						entry.ThreadID = threadID

						// Determine log level and message type based on log type and activity type
						entry.MessageType = getLogType(logType, logActivityType)
						entry.Level = entry.MessageType // Keep Level for backward compatibility

						// Determine event type based on activity type
						entry.EventType = getEventType(logActivityType)

						// Determine entry category based on activity type
						switch logActivityType {
						case 0x2:
							entry.Category = "activity"
						case 0x4:
							entry.Category = "log"
						case 0x6:
							entry.Category = "signpost"
						case 0x3:
							entry.Category = "trace"
						case 0x7:
							entry.Category = "loss"
						default:
							entry.Category = fmt.Sprintf("unknown(0x%x)", logActivityType)
						}

						entry.Message = fmt.Sprintf("Firehose entry: type=%s level=%s flags=0x%x format=0x%x thread=%d dataSize=%d",
							entry.Category, entry.Level, flags, formatStringLocation, threadID, dataSize)

						// If there's private data, note it
						if privateDataOffset != 0x1000 {
							entry.Message += fmt.Sprintf(" [has private data at offset 0x%x]", privateDataOffset)
						}
					}
				}
			}
		}

		if entry.Message == "" {
			entry.Message = fmt.Sprintf("%s chunk: tag=0x%x sub_tag=0x%x size=%d", entry.ChunkType, chunkTag, chunkSubTag, chunkDataSize)
		}

		entries = append(entries, entry)

		// Move to next chunk position (preamble + data)
		offset += totalChunkSize

		// Handle 8-byte alignment padding (critical for boundary detection)
		// This matches the rust implementation's padding_size_8 logic
		paddingBytes := paddingSize8(chunkDataSize)
		if offset+int(paddingBytes) > len(data) {
			// Not enough data for padding, stop parsing
			break
		}
		offset += int(paddingBytes)

		entryCount++

		// Safety limit to prevent infinite loops
		if entryCount >= 1000 {
			break
		}
	}

	// If we didn't find any firehose entries, create a summary entry
	if len(entries) == 0 && len(data) > 0 {
		entries = append(entries, &TraceV3Entry{
			Type:         0x0000,
			Size:         uint32(len(data)),
			Timestamp:    header.ContinuousTime,
			ThreadID:     0,
			ProcessID:    header.LogdPID,
			Message:      fmt.Sprintf("Unparsed data section: %d bytes (no valid firehose entries found)", len(data)),
			Subsystem:    "com.apple.logd",
			Category:     "unparsed",
			Level:        "Info",
			MessageType:  "Info",
			EventType:    "logEvent",
			TimezoneName: extractTimezoneName(header.TimezonePath),
		})
	}

	return entries
}

// ConvertTraceV3EntriesToLogs converts parsed tracev3 entries to OpenTelemetry log records
func ConvertTraceV3EntriesToLogs(entries []*TraceV3Entry) plog.Logs {
	logs := plog.NewLogs()

	for _, entry := range entries {
		resourceLogs := logs.ResourceLogs().AppendEmpty()
		scopeLogs := resourceLogs.ScopeLogs().AppendEmpty()
		logRecord := scopeLogs.LogRecords().AppendEmpty()

		// Set timestamps
		logRecord.SetTimestamp(pcommon.Timestamp(entry.Timestamp))
		logRecord.SetObservedTimestamp(pcommon.NewTimestampFromTime(time.Now()))

		// Set severity based on entry type or level
		switch entry.Level {
		case "Error", "Fault":
			logRecord.SetSeverityNumber(plog.SeverityNumberError)
			logRecord.SetSeverityText("ERROR")
		case "Debug":
			logRecord.SetSeverityNumber(plog.SeverityNumberDebug)
			logRecord.SetSeverityText("DEBUG")
		default:
			logRecord.SetSeverityNumber(plog.SeverityNumberInfo)
			logRecord.SetSeverityText("INFO")
		}

		// Set message body
		logRecord.Body().SetStr(entry.Message)

		// Set attributes including new standard fields
		logRecord.Attributes().PutStr("source", "macos_unified_logging")
		logRecord.Attributes().PutStr("subsystem", entry.Subsystem)
		logRecord.Attributes().PutStr("category", entry.Category)
		logRecord.Attributes().PutStr("chunk.type", entry.ChunkType)
		logRecord.Attributes().PutInt("entry.type", int64(entry.Type))
		logRecord.Attributes().PutInt("entry.size", int64(entry.Size))
		logRecord.Attributes().PutInt("thread.id", int64(entry.ThreadID))
		logRecord.Attributes().PutInt("process.id", int64(entry.ProcessID))
		logRecord.Attributes().PutBool("decoded", true)

		// Add new standard fields that match log command output
		logRecord.Attributes().PutStr("timezoneName", entry.TimezoneName)
		logRecord.Attributes().PutStr("messageType", entry.MessageType)
		logRecord.Attributes().PutStr("eventType", entry.EventType)
	}

	return logs
}

// getLogType returns the LogType (MessageType) based on log type and activity type
// Based on the Rust implementation from mandiant/macos-UnifiedLogs
func getLogType(logType uint8, activityType uint8) string {
	switch logType {
	case 0x1:
		if activityType == 2 {
			return "Create"
		}
		return "Info"
	case 0x2:
		return "Debug"
	case 0x3:
		return "Useraction"
	case 0x10:
		return "Error"
	case 0x11:
		return "Fault"
	case 0x80:
		return "ProcessSignpostEvent"
	case 0x81:
		return "ProcessSignpostStart"
	case 0x82:
		return "ProcessSignpostEnd"
	case 0xc0:
		return "SystemSignpostEvent"
	case 0xc1:
		return "SystemSignpostStart"
	case 0xc2:
		return "SystemSignpostEnd"
	case 0x40:
		return "ThreadSignpostEvent"
	case 0x41:
		return "ThreadSignpostStart"
	case 0x42:
		return "ThreadSignpostEnd"
	default:
		return "Default"
	}
}

// getEventType returns the EventType based on activity type
// Based on the Rust implementation from mandiant/macos-UnifiedLogs
func getEventType(activityType uint8) string {
	switch activityType {
	case 0x4:
		return "logEvent"
	case 0x2:
		return "activityEvent"
	case 0x3:
		return "traceEvent"
	case 0x6:
		return "signpostEvent"
	case 0x7:
		return "lossEvent"
	default:
		return "logEvent" // Default to logEvent like the rust parser
	}
}

// extractTimezoneName extracts the timezone name from timezone path
// Similar to the Rust implementation which gets the last path component
func extractTimezoneName(timezonePath string) string {
	if timezonePath == "" {
		return ""
	}

	// Split path and get the last component
	parts := strings.Split(timezonePath, "/")
	if len(parts) > 0 {
		name := parts[len(parts)-1]
		if name != "" {
			return name
		}
	}

	return ""
}
