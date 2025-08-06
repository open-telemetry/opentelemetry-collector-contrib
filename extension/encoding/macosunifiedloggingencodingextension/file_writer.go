// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package macosunifiedloggingencodingextension

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

// RawFileWriter handles writing decoded traceV3 entries directly to a file
type RawFileWriter struct {
	filePath string
	file     *os.File
	encoder  *json.Encoder
	mutex    sync.Mutex
}

// RawLogEntry represents a decoded log entry in log stream format
type RawLogEntry struct {
	// Core fields for log stream format
	Timestamp         uint64 `json:"timestamp"`
	TimestampReadable string `json:"timestamp_readable"`
	ThreadID          uint32 `json:"thread_id"`
	LogLevel          string `json:"log_level"`
	ActivityID        uint64 `json:"activity_id"`
	ProcessID         uint32 `json:"process_id"`
	EffectiveUID      uint32 `json:"effective_uid"`
	ProcessName       string `json:"process_name"`
	Subsystem         string `json:"subsystem"`
	Category          string `json:"category"`
	Message           string `json:"message"`

	// Parsing metadata
	EntryType     uint8             `json:"entry_type"`
	Offset        int               `json:"offset"`
	Size          int               `json:"size"`
	UnparsedBytes int               `json:"unparsed_bytes,omitempty"`
	RawStrings    []string          `json:"raw_strings,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`

	// Log stream formatted output
	LogStreamFormat string `json:"log_stream_format"`
}

// NewRawFileWriter creates a new raw file writer
func NewRawFileWriter(filePath string) (*RawFileWriter, error) {
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %s: %w", filePath, err)
	}

	return &RawFileWriter{
		filePath: filePath,
		file:     file,
		encoder:  json.NewEncoder(file),
	}, nil
}

// WriteEntry writes a raw log entry to the file
func (w *RawFileWriter) WriteEntry(entry *RawLogEntry) error {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	// Add readable timestamp if not already set
	if entry.TimestampReadable == "" && entry.Timestamp > 0 {
		entry.TimestampReadable = time.Unix(0, int64(entry.Timestamp)).Format(time.RFC3339Nano)
	}

	return w.encoder.Encode(entry)
}

// WriteSummary writes a summary entry about the parsing operation
func (w *RawFileWriter) WriteSummary(fileSize int, entriesFound int, parseTime time.Duration) error {
	summary := map[string]interface{}{
		"summary":        true,
		"timestamp":      time.Now().Format(time.RFC3339Nano),
		"file_size":      fileSize,
		"entries_found":  entriesFound,
		"parse_time_ms":  parseTime.Milliseconds(),
		"parser_version": "basic-tracev3-v1.0",
	}

	w.mutex.Lock()
	defer w.mutex.Unlock()

	return w.encoder.Encode(summary)
}

// Close closes the file writer
func (w *RawFileWriter) Close() error {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	if w.file != nil {
		return w.file.Close()
	}
	return nil
}

// logLevelToString converts LogLevel to string
func logLevelToString(level LogLevel) string {
	switch level {
	case LogLevelDefault:
		return "Default"
	case LogLevelInfo:
		return "Info"
	case LogLevelDebug:
		return "Debug"
	case LogLevelError:
		return "Error"
	case LogLevelFault:
		return "Fault"
	default:
		return fmt.Sprintf("Unknown(%d)", level)
	}
}

// eventTypeToString converts EventType to string
func eventTypeToString(eventType EventType) string {
	switch eventType {
	case EventTypeLog:
		return "Log"
	case EventTypeSignpost:
		return "Signpost"
	case EventTypeActivity:
		return "Activity"
	case EventTypeStateDump:
		return "StateDump"
	case EventTypeLoss:
		return "Loss"
	case EventTypeSimpleDump:
		return "SimpleDump"
	default:
		return fmt.Sprintf("Unknown(%d)", eventType)
	}
}

// convertUnifiedLogEntryToRaw converts a UnifiedLogEntry to RawLogEntry
func convertUnifiedLogEntryToRaw(entry *UnifiedLogEntry, entryType uint8, offset int, size int, rawStrings []string) *RawLogEntry {
	timestamp := time.Unix(0, int64(entry.Timestamp))

	// Extract process name from process path
	processName := "unknown"
	if entry.ProcessPath != "" {
		parts := strings.Split(entry.ProcessPath, "/")
		if len(parts) > 0 {
			processName = parts[len(parts)-1]
		}
	}

	// Format subsystem for log stream format
	subsystemFormatted := ""
	if entry.Subsystem != "" {
		subsystemFormatted = fmt.Sprintf("(%s)", entry.Subsystem)
	}

	// Format category for log stream format
	categoryFormatted := ""
	if entry.Category != "" {
		categoryFormatted = fmt.Sprintf("[%s]", entry.Category)
	}

	// Calculate unparsed bytes (total size minus strings and known structure)
	unparsedBytes := size - 7 // Subtract header size
	for _, s := range rawStrings {
		unparsedBytes -= len(s) + 1 // Subtract string length + null terminator
	}
	if unparsedBytes < 0 {
		unparsedBytes = 0
	}

	rawEntry := &RawLogEntry{
		Timestamp:         entry.Timestamp,
		TimestampReadable: timestamp.Format("2006-01-02 15:04:05.000000-0700"),
		ThreadID:          uint32(entry.ThreadID),
		LogLevel:          logLevelToString(entry.LogLevel),
		ActivityID:        entry.ActivityID,
		ProcessID:         entry.ProcessID,
		EffectiveUID:      entry.EffectiveUID,
		ProcessName:       processName,
		Subsystem:         subsystemFormatted,
		Category:          categoryFormatted,
		Message:           entry.Message,
		EntryType:         entryType,
		Offset:            offset,
		Size:              size,
		UnparsedBytes:     unparsedBytes,
		RawStrings:        rawStrings,
		Metadata: map[string]string{
			"parser":     "basic-tracev3",
			"format":     "macos-unified-logging",
			"event_type": eventTypeToString(entry.EventType),
		},
	}

	// Generate log stream format
	rawEntry.LogStreamFormat = formatAsLogStream(rawEntry)

	return rawEntry
}

// formatAsLogStream formats a RawLogEntry in the style of macOS log stream command
func formatAsLogStream(entry *RawLogEntry) string {
	// Format: timestamp thread_id log_level activity_id process_id effective_uid process_name: subsystem category message
	return fmt.Sprintf("%s 0x%x  %-11s 0x%x            %-6d %-4d %s: %s %s %s",
		entry.TimestampReadable,
		entry.ThreadID,
		entry.LogLevel,
		entry.ActivityID,
		entry.ProcessID,
		entry.EffectiveUID,
		entry.ProcessName,
		entry.Subsystem,
		entry.Category,
		entry.Message,
	)
}
