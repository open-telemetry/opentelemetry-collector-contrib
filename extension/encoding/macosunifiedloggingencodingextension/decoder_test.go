// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package macosunifiedloggingencodingextension

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDecoder_UnmarshalLogs_InvalidMagic(t *testing.T) {
	decoder := &macosUnifiedLoggingDecoder{
		config: &Config{
			MaxLogSize: 65536,
		},
	}

	// Create buffer with invalid magic number
	buf := make([]byte, 32)
	binary.LittleEndian.PutUint32(buf[0:4], 0xdeadbeef) // Invalid magic

	logs, err := decoder.UnmarshalLogs(buf)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid traceV3 magic number")
	assert.Equal(t, 0, logs.LogRecordCount())
}

func TestDecoder_UnmarshalLogs_TooSmall(t *testing.T) {
	decoder := &macosUnifiedLoggingDecoder{
		config: &Config{
			MaxLogSize: 65536,
		},
	}

	// Create buffer too small for header
	buf := make([]byte, 10)

	logs, err := decoder.UnmarshalLogs(buf)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "buffer too small")
	assert.Equal(t, 0, logs.LogRecordCount())
}

func TestDecoder_UnmarshalLogs_ValidHeader(t *testing.T) {
	decoder := &macosUnifiedLoggingDecoder{
		config: &Config{
			MaxLogSize:            65536,
			IncludeSignpostEvents: true,
			IncludeActivityEvents: true,
		},
	}

	// Create buffer with valid magic number but no log entries
	buf := make([]byte, 64)
	binary.LittleEndian.PutUint32(buf[0:4], TraceV3Magic)

	logs, err := decoder.UnmarshalLogs(buf)
	assert.NoError(t, err)
	assert.Equal(t, 0, logs.LogRecordCount()) // No log entries to parse
}

func TestLogLevelToOTelSeverity(t *testing.T) {
	decoder := &macosUnifiedLoggingDecoder{}

	tests := []struct {
		level    LogLevel
		expected string
	}{
		{LogLevelDebug, "Debug"},
		{LogLevelInfo, "Info"},
		{LogLevelDefault, "Default"},
		{LogLevelError, "Error"},
		{LogLevelFault, "Fault"},
		{LogLevel(99), "Unknown"},
	}

	for _, test := range tests {
		result := decoder.logLevelToString(test.level)
		assert.Equal(t, test.expected, result)
	}
}

func TestEventTypeToString(t *testing.T) {
	decoder := &macosUnifiedLoggingDecoder{}

	tests := []struct {
		eventType EventType
		expected  string
	}{
		{EventTypeLog, "Log"},
		{EventTypeSignpost, "Signpost"},
		{EventTypeActivity, "Activity"},
		{EventTypeStateDump, "StateDump"},
		{EventTypeSimpleDump, "SimpleDump"},
		{EventTypeLoss, "Loss"},
		{EventType(99), "Unknown"},
	}

	for _, test := range tests {
		result := decoder.eventTypeToString(test.eventType)
		assert.Equal(t, test.expected, result)
	}
}

func TestShouldIncludeEntry(t *testing.T) {
	tests := []struct {
		name     string
		config   *Config
		entry    *UnifiedLogEntry
		expected bool
	}{
		{
			name: "include signpost enabled",
			config: &Config{
				IncludeSignpostEvents: true,
				IncludeActivityEvents: true,
				MaxLogSize:            1000,
			},
			entry: &UnifiedLogEntry{
				EventType: EventTypeSignpost,
				Message:   "test message",
			},
			expected: true,
		},
		{
			name: "exclude signpost disabled",
			config: &Config{
				IncludeSignpostEvents: false,
				IncludeActivityEvents: true,
				MaxLogSize:            1000,
			},
			entry: &UnifiedLogEntry{
				EventType: EventTypeSignpost,
				Message:   "test message",
			},
			expected: false,
		},
		{
			name: "exclude activity disabled",
			config: &Config{
				IncludeSignpostEvents: true,
				IncludeActivityEvents: false,
				MaxLogSize:            1000,
			},
			entry: &UnifiedLogEntry{
				EventType: EventTypeActivity,
				Message:   "test message",
			},
			expected: false,
		},
		{
			name: "exclude message too large",
			config: &Config{
				IncludeSignpostEvents: true,
				IncludeActivityEvents: true,
				MaxLogSize:            10,
			},
			entry: &UnifiedLogEntry{
				EventType: EventTypeLog,
				Message:   "this message is too long",
			},
			expected: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			decoder := &macosUnifiedLoggingDecoder{config: test.config}
			result := decoder.shouldIncludeEntry(test.entry)
			assert.Equal(t, test.expected, result)
		})
	}
}

func TestParseMessageAndMetadata(t *testing.T) {
	decoder := &macosUnifiedLoggingDecoder{}
	entry := &UnifiedLogEntry{}

	// Test with simple null-terminated strings
	data := []byte("Hello World\x00com.example.app\x00category\x00")
	decoder.parseMessageAndMetadata(data, entry)

	assert.Equal(t, "Hello World", entry.Message)
	assert.Equal(t, "com.example.app", entry.Subsystem)
	assert.Equal(t, "category", entry.Category)
}

func TestParseMessageAndMetadata_BinaryData(t *testing.T) {
	decoder := &macosUnifiedLoggingDecoder{}
	entry := &UnifiedLogEntry{}

	// Test with binary data that produces no readable strings (empty message)
	data := []byte{0x00, 0x00} // Only null terminators
	decoder.parseMessageAndMetadata(data, entry)

	// The function should detect empty message and provide fallback
	assert.Contains(t, entry.Message, "Binary log entry")
}
