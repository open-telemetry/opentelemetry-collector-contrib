// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package macosunifiedloggingencodingextension

import (
	"encoding/binary"
	"fmt"
)

// ParseFirehoseChunk parses a Firehose chunk (0x6001 and variants) containing multiple individual log entries
// Returns a slice of TraceV3Entry representing each individual log event within the chunk
func ParseFirehoseChunk(data []byte, entry *TraceV3Entry, header *TraceV3Header) []*TraceV3Entry {
	var entries []*TraceV3Entry

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

	// Parse individual firehose entries from the public data section
	if publicDataSize > 0 && len(data) >= int(52+publicDataSize) {
		publicData := data[52 : 52+publicDataSize]
		entries = parseIndividualFirehoseEntries(publicData, header, firstProcID, secondProcID, baseContinuousTime, ttl, collapsed)
	}

	// If no individual entries found, create a summary entry
	if len(entries) == 0 {
		entry.ThreadID = firstProcID
		entry.ProcessID = secondProcID
		entry.Timestamp = baseContinuousTime
		entry.Message = fmt.Sprintf("Firehose chunk: ttl=%d collapsed=%d publicSize=%d privateOffset=0x%x (no parseable entries)",
			ttl, collapsed, publicDataSize, privateDataOffset)
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

	// Parse individual entries from public data
	for offset < len(publicData) {
		// Each individual firehose entry starts with a 20-byte header
		if offset+20 > len(publicData) {
			break
		}

		// Parse individual firehose entry header (matches rust parse_firehose structure)
		logActivityType := publicData[offset]
		logType := publicData[offset+1]
		flags := binary.LittleEndian.Uint16(publicData[offset+2:])
		formatStringLocation := binary.LittleEndian.Uint32(publicData[offset+4:])
		threadID := binary.LittleEndian.Uint64(publicData[offset+8:])
		continuousTimeDelta := binary.LittleEndian.Uint32(publicData[offset+16:])
		dataSize := binary.LittleEndian.Uint16(publicData[offset+18:])

		// Verify we have enough data for this entry
		if offset+20+int(dataSize) > len(publicData) {
			// Not enough data for this entry, break out
			break
		}

		// Calculate the combined continuous time (6 bytes total: 4 + 2)
		continuousTimeDeltaUpper := binary.LittleEndian.Uint16(publicData[offset+20:])
		if int(dataSize) >= 2 {
			combinedTimeDelta := uint64(continuousTimeDelta) | (uint64(continuousTimeDeltaUpper) << 32)

			// Create individual log entry
			logEntry := &TraceV3Entry{
				Type:         0x6001,                                 // Firehose chunk type
				Size:         uint32(20 + dataSize),                  // Header + data size
				Timestamp:    baseContinuousTime + combinedTimeDelta, // Calculate actual timestamp using combined delta
				ThreadID:     threadID,
				ProcessID:    secondProcID,
				ChunkType:    "firehose",
				Subsystem:    "com.apple.firehose", // Default, should be extracted from format string catalog
				Category:     "log",
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

			// Parse message data if available (skip the first 2 bytes which are the upper time delta)
			messageData := ""
			if dataSize > 2 {
				rawData := publicData[offset+22 : offset+20+int(dataSize)]
				messageData = parseFirehoseMessageData(rawData)
			}

			// Create descriptive message with available information
			logEntry.Message = fmt.Sprintf("Firehose entry: type=%s level=%s flags=0x%x format=0x%x thread=%d delta=%d data=%q",
				logEntry.Category, logEntry.Level, flags, formatStringLocation, threadID, combinedTimeDelta, messageData)

			entries = append(entries, logEntry)
		}

		// Move to next entry (20-byte header + data_size bytes)
		offset += 20 + int(dataSize)

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
