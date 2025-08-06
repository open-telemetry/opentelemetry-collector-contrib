// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package macosunifiedloggingencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/macosunifiedloggingencodingextension"

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

// TraceV3 file format constants based on research
const (
	TraceV3Magic       = 0x600dbeef // Magic number for traceV3 files
	TraceV3HeaderSize  = 32         // Basic header size
	LogEntryMinSize    = 16         // Minimum log entry size
	MaxReasonableEntry = 1024 * 64  // 64KB max per entry
)

// LogLevel represents the log level from macOS Unified Logging
type LogLevel uint8

const (
	LogLevelDefault LogLevel = 0
	LogLevelInfo    LogLevel = 1
	LogLevelDebug   LogLevel = 2
	LogLevelError   LogLevel = 16
	LogLevelFault   LogLevel = 17
)

// EventType represents the event type from macOS Unified Logging
type EventType uint8

const (
	EventTypeLog        EventType = 0
	EventTypeSignpost   EventType = 1
	EventTypeActivity   EventType = 2
	EventTypeStateDump  EventType = 3
	EventTypeLoss       EventType = 4
	EventTypeSimpleDump EventType = 5
)

// UnifiedLogEntry represents a parsed macOS Unified Log entry
type UnifiedLogEntry struct {
	Timestamp    uint64
	ThreadID     uint64
	ProcessID    uint32
	EffectiveUID uint32
	LogLevel     LogLevel
	EventType    EventType
	ActivityID   uint64
	Message      string
	Subsystem    string
	Category     string
	ProcessPath  string
	LibraryPath  string
}

type macosUnifiedLoggingDecoder struct {
	config *Config
}

func (d *macosUnifiedLoggingDecoder) UnmarshalLogs(buf []byte) (plog.Logs, error) {
	logs := plog.NewLogs()

	if len(buf) < TraceV3HeaderSize {
		return logs, fmt.Errorf("buffer too small for traceV3 header: %d bytes", len(buf))
	}

	// Parse traceV3 header
	reader := bytes.NewReader(buf)

	var magic uint32
	if err := binary.Read(reader, binary.LittleEndian, &magic); err != nil {
		return logs, fmt.Errorf("failed to read magic number: %w", err)
	}

	if magic != TraceV3Magic {
		return logs, fmt.Errorf("invalid traceV3 magic number: 0x%x", magic)
	}

	// Skip remaining header fields for now (timestamp base, etc.)
	reader.Seek(TraceV3HeaderSize, 0)

	// Parse log entries
	entries, err := d.parseLogEntries(buf[TraceV3HeaderSize:])
	if err != nil {
		return logs, fmt.Errorf("failed to parse log entries: %w", err)
	}

	// Convert to OpenTelemetry log format
	d.convertToOTelLogs(entries, logs)

	return logs, nil
}

func (d *macosUnifiedLoggingDecoder) parseLogEntries(data []byte) ([]*UnifiedLogEntry, error) {
	var entries []*UnifiedLogEntry
	reader := bytes.NewReader(data)

	for reader.Len() > LogEntryMinSize {
		entry, err := d.parseSingleEntry(reader)
		if err != nil {
			// Log parsing error but continue with remaining data
			continue
		}

		if entry != nil {
			// Apply configuration filters
			if d.shouldIncludeEntry(entry) {
				entries = append(entries, entry)
			}
		}

		// Safety check to prevent infinite loops
		if len(entries) > 1000000 { // 1M entries max
			break
		}
	}

	return entries, nil
}

func (d *macosUnifiedLoggingDecoder) parseSingleEntry(reader *bytes.Reader) (*UnifiedLogEntry, error) {
	if reader.Len() < LogEntryMinSize {
		return nil, fmt.Errorf("insufficient data for log entry")
	}

	entry := &UnifiedLogEntry{}

	// Read basic entry header (simplified version based on research)
	var entrySize uint32
	if err := binary.Read(reader, binary.LittleEndian, &entrySize); err != nil {
		return nil, err
	}

	if entrySize > MaxReasonableEntry || entrySize < LogEntryMinSize {
		return nil, fmt.Errorf("invalid entry size: %d", entrySize)
	}

	// Read timestamp (Mach absolute time)
	if err := binary.Read(reader, binary.LittleEndian, &entry.Timestamp); err != nil {
		return nil, err
	}

	// Read thread ID
	if err := binary.Read(reader, binary.LittleEndian, &entry.ThreadID); err != nil {
		return nil, err
	}

	// Read remaining fields (simplified parsing)
	var flags uint32
	if err := binary.Read(reader, binary.LittleEndian, &flags); err != nil {
		return nil, err
	}

	// Extract log level and event type from flags
	entry.LogLevel = LogLevel(flags & 0xFF)
	entry.EventType = EventType((flags >> 8) & 0xFF)

	// Read process ID and EUID
	if err := binary.Read(reader, binary.LittleEndian, &entry.ProcessID); err != nil {
		return nil, err
	}
	if err := binary.Read(reader, binary.LittleEndian, &entry.EffectiveUID); err != nil {
		return nil, err
	}

	// Calculate remaining data size for message and metadata
	remainingSize := int(entrySize) - 26 // 26 bytes read so far
	if remainingSize > 0 && remainingSize <= reader.Len() {
		messageData := make([]byte, remainingSize)
		if _, err := reader.Read(messageData); err != nil {
			return nil, err
		}

		// Parse message and metadata from remaining data
		d.parseMessageAndMetadata(messageData, entry)
	}

	return entry, nil
}

func (d *macosUnifiedLoggingDecoder) parseMessageAndMetadata(data []byte, entry *UnifiedLogEntry) {
	// Simplified message parsing - in reality this is much more complex
	// involving string tables, format strings, and argument substitution

	reader := bytes.NewReader(data)

	// Try to extract null-terminated strings
	var message bytes.Buffer
	var subsystem bytes.Buffer
	var category bytes.Buffer

	stringCount := 0
	for reader.Len() > 0 && stringCount < 10 { // Safety limit
		b, err := reader.ReadByte()
		if err != nil {
			break
		}

		if b == 0 { // Null terminator
			stringCount++
			continue
		}

		// Simple heuristic to assign strings
		if stringCount == 0 && message.Len() < 512 {
			message.WriteByte(b)
		} else if stringCount == 1 && subsystem.Len() < 256 {
			subsystem.WriteByte(b)
		} else if stringCount == 2 && category.Len() < 256 {
			category.WriteByte(b)
		}
	}

	entry.Message = message.String()
	entry.Subsystem = subsystem.String()
	entry.Category = category.String()

	// If message is empty or looks like binary data, provide a fallback
	if entry.Message == "" || len(entry.Message) < 3 {
		entry.Message = fmt.Sprintf("Binary log entry (size: %d bytes)", len(data))
	}
}

func (d *macosUnifiedLoggingDecoder) shouldIncludeEntry(entry *UnifiedLogEntry) bool {
	// Apply configuration filters
	if !d.config.IncludeSignpostEvents && entry.EventType == EventTypeSignpost {
		return false
	}

	if !d.config.IncludeActivityEvents && entry.EventType == EventTypeActivity {
		return false
	}

	// Filter by message size
	if len(entry.Message) > d.config.MaxLogSize {
		return false
	}

	return true
}

func (d *macosUnifiedLoggingDecoder) convertToOTelLogs(entries []*UnifiedLogEntry, logs plog.Logs) {
	resourceLogs := logs.ResourceLogs().AppendEmpty()
	scopeLogs := resourceLogs.ScopeLogs().AppendEmpty()

	for _, entry := range entries {
		logRecord := scopeLogs.LogRecords().AppendEmpty()

		// Convert Mach absolute time to Unix timestamp
		// This is a simplified conversion - in reality it requires boot time reference
		timestamp := d.machTimeToUnixNano(entry.Timestamp)
		logRecord.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(0, int64(timestamp))))
		logRecord.SetObservedTimestamp(pcommon.NewTimestampFromTime(time.Now()))

		// Set log body
		logRecord.Body().SetStr(entry.Message)

		// Set severity
		logRecord.SetSeverityNumber(d.logLevelToOTelSeverity(entry.LogLevel))
		logRecord.SetSeverityText(d.logLevelToString(entry.LogLevel))

		// Set attributes
		attrs := logRecord.Attributes()
		attrs.PutInt("process_id", int64(entry.ProcessID))
		attrs.PutInt("thread_id", int64(entry.ThreadID))
		attrs.PutInt("effective_uid", int64(entry.EffectiveUID))
		attrs.PutInt("activity_id", int64(entry.ActivityID))
		attrs.PutStr("event_type", d.eventTypeToString(entry.EventType))

		if entry.Subsystem != "" {
			attrs.PutStr("subsystem", entry.Subsystem)
		}
		if entry.Category != "" {
			attrs.PutStr("category", entry.Category)
		}
		if entry.ProcessPath != "" {
			attrs.PutStr("process_path", entry.ProcessPath)
		}
		if entry.LibraryPath != "" {
			attrs.PutStr("library_path", entry.LibraryPath)
		}
	}
}

func (d *macosUnifiedLoggingDecoder) machTimeToUnixNano(machTime uint64) uint64 {
	// Simplified conversion - in reality this requires system boot time
	// and proper Mach timebase conversion
	return machTime
}

func (d *macosUnifiedLoggingDecoder) logLevelToOTelSeverity(level LogLevel) plog.SeverityNumber {
	switch level {
	case LogLevelDebug:
		return plog.SeverityNumberDebug
	case LogLevelInfo:
		return plog.SeverityNumberInfo
	case LogLevelDefault:
		return plog.SeverityNumberInfo
	case LogLevelError:
		return plog.SeverityNumberError
	case LogLevelFault:
		return plog.SeverityNumberFatal
	default:
		return plog.SeverityNumberUnspecified
	}
}

func (d *macosUnifiedLoggingDecoder) logLevelToString(level LogLevel) string {
	switch level {
	case LogLevelDebug:
		return "Debug"
	case LogLevelInfo:
		return "Info"
	case LogLevelDefault:
		return "Default"
	case LogLevelError:
		return "Error"
	case LogLevelFault:
		return "Fault"
	default:
		return "Unknown"
	}
}

func (d *macosUnifiedLoggingDecoder) eventTypeToString(eventType EventType) string {
	switch eventType {
	case EventTypeLog:
		return "Log"
	case EventTypeSignpost:
		return "Signpost"
	case EventTypeActivity:
		return "Activity"
	case EventTypeStateDump:
		return "StateDump"
	case EventTypeSimpleDump:
		return "SimpleDump"
	case EventTypeLoss:
		return "Loss"
	default:
		return "Unknown"
	}
}
