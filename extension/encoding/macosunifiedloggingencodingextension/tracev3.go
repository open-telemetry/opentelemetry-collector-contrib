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
	Type      uint32 // Entry type (log, signpost, activity, etc.)
	Size      uint32 // Size of this entry
	Timestamp uint64 // Mach absolute time
	ThreadID  uint64 // Thread identifier
	ProcessID uint32 // Process identifier
	Message   string // Log message content
	Subsystem string // Subsystem (e.g., com.apple.SkyLight)
	Category  string // Category within subsystem
	Level     string // Log level (Default, Info, Debug, Error, Fault)
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
			Subsystem: "com.apple.logd",
			Category:  "header",
			Level:     "Info",
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
			Subsystem: "com.apple.logd",
			Category:  "data",
			Level:     "Info",
		})
	}

	return entries, nil
}

// parseDataEntries attempts to parse individual entries from the data section
func parseDataEntries(data []byte, header *TraceV3Header) []*TraceV3Entry {
	entries := []*TraceV3Entry{}
	offset := 0
	entryCount := 0
	maxEntries := 10 // Limit for now

	for offset < len(data) && entryCount < maxEntries {
		// Try to parse a basic entry structure
		if offset+8 > len(data) {
			break
		}

		entryType := binary.LittleEndian.Uint32(data[offset:])
		entrySize := binary.LittleEndian.Uint32(data[offset+4:])

		// Sanity check the entry size
		if entrySize == 0 || entrySize > uint32(len(data)-offset) || entrySize > 0x100000 {
			// Invalid entry size, skip ahead
			offset += 8
			continue
		}

		entry := &TraceV3Entry{
			Type:      entryType,
			Size:      entrySize,
			Timestamp: uint64(time.Now().UnixNano()) + uint64(entryCount)*1000, // Fake incrementing timestamps
			ThreadID:  0,
			ProcessID: header.LogdPID,
			Message:   fmt.Sprintf("Entry #%d (type: 0x%x, size: %d bytes)", entryCount+1, entryType, entrySize),
			Subsystem: "com.apple.unknown",
			Category:  "parsed_entry",
			Level:     "Info",
		}

		entries = append(entries, entry)
		offset += int(entrySize)
		entryCount++
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

		// Set attributes
		logRecord.Attributes().PutStr("source", "macos_unified_logging")
		logRecord.Attributes().PutStr("subsystem", entry.Subsystem)
		logRecord.Attributes().PutStr("category", entry.Category)
		logRecord.Attributes().PutInt("entry.type", int64(entry.Type))
		logRecord.Attributes().PutInt("entry.size", int64(entry.Size))
		logRecord.Attributes().PutInt("thread.id", int64(entry.ThreadID))
		logRecord.Attributes().PutInt("process.id", int64(entry.ProcessID))
		logRecord.Attributes().PutBool("decoded", true)
	}

	return logs
}
