// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package macosunifiedloggingencodingextension

import (
	"encoding/hex"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func TestActualTraceV3Format(t *testing.T) {
	// Load our test sample
	data, err := ioutil.ReadFile("../../../local/test_sample.tracev3")
	require.NoError(t, err, "Failed to read test sample")
	require.True(t, len(data) > 0, "Test sample is empty")

	t.Logf("Loaded %d bytes of traceV3 data", len(data))

	// Analyze the structure
	analyzeTraceV3Structure(t, data)
}

func analyzeTraceV3Structure(t *testing.T, data []byte) {
	t.Log("=== TraceV3 Structure Analysis ===")

	// Examine the first 16 bytes (potential header)
	if len(data) >= 16 {
		header := data[0:16]
		t.Logf("Header (0x00-0x0F): %s", hex.EncodeToString(header))

		// Parse as potential header fields
		if len(header) >= 4 {
			field1 := uint32(header[0]) | uint32(header[1])<<8 | uint32(header[2])<<16 | uint32(header[3])<<24
			t.Logf("  Field 1 (0x00-0x03): 0x%08x (%d)", field1, field1)
		}
		if len(header) >= 8 {
			field2 := uint32(header[4]) | uint32(header[5])<<8 | uint32(header[6])<<16 | uint32(header[7])<<24
			t.Logf("  Field 2 (0x04-0x07): 0x%08x (%d)", field2, field2)
		}
	}

	// Look for patterns that might indicate entries or chunks
	findPotentialEntries(t, data)

	// Look for readable strings
	findReadableStrings(t, data)
}

func findPotentialEntries(t *testing.T, data []byte) {
	t.Log("\n=== Potential Entry Analysis ===")

	// Look for patterns like "XX 61 00 00" which appear to be entry markers
	entryPattern := []byte{0x61, 0x00, 0x00}

	for i := 1; i < len(data)-3; i++ {
		if data[i] == entryPattern[0] && data[i+1] == entryPattern[1] && data[i+2] == entryPattern[2] {
			// Found potential entry marker
			entryType := data[i-1]
			t.Logf("Found potential entry at offset 0x%04x: type=0x%02x", i-1, entryType)

			// Try to read the size field that might follow
			if i+6 < len(data) {
				size := uint32(data[i+3]) | uint32(data[i+4])<<8 | uint32(data[i+5])<<16 | uint32(data[i+6])<<24
				t.Logf("  Potential size field: %d bytes", size)

				// Show a hex dump of the next 32 bytes or until end
				end := i + 32
				if end > len(data) {
					end = len(data)
				}
				t.Logf("  Data: %s", hex.EncodeToString(data[i-1:end]))
			}
		}
	}
}

func findReadableStrings(t *testing.T, data []byte) {
	t.Log("\n=== Readable Strings Analysis ===")

	var currentString []byte
	stringStart := -1

	for i, b := range data {
		if b >= 32 && b <= 126 { // Printable ASCII
			if stringStart == -1 {
				stringStart = i
			}
			currentString = append(currentString, b)
		} else {
			if len(currentString) >= 4 { // Only report strings of 4+ chars
				t.Logf("String at 0x%04x: \"%s\"", stringStart, string(currentString))
			}
			currentString = nil
			stringStart = -1
		}
	}

	// Handle string at end of data
	if len(currentString) >= 4 {
		t.Logf("String at 0x%04x: \"%s\"", stringStart, string(currentString))
	}
}

func TestDecoderWithActualData(t *testing.T) {
	// Load our test sample
	data, err := ioutil.ReadFile("../../../local/test_sample.tracev3")
	require.NoError(t, err, "Failed to read test sample")

	config := &Config{
		ParsePrivateLogs:      false,
		IncludeSignpostEvents: true,
		IncludeActivityEvents: true,
		MaxLogSize:            65536,
	}

	decoder := &macosUnifiedLoggingDecoder{config: config}

	// Test that our decoder can handle the actual data without crashing
	logs, err := decoder.UnmarshalLogs(data)
	require.NoError(t, err, "Decoder should handle actual traceV3 data")

	assert.True(t, logs.LogRecordCount() > 0, "Should produce at least one log record")

	// Examine the log record
	resourceLogs := logs.ResourceLogs().At(0)
	scopeLogs := resourceLogs.ScopeLogs().At(0)
	logRecord := scopeLogs.LogRecords().At(0)

	t.Logf("Generated log message: %s", logRecord.Body().AsString())
	t.Logf("Log attributes count: %d", logRecord.Attributes().Len())

	// The message should indicate we parsed traceV3 data
	assert.Contains(t, logRecord.Body().AsString(), "TraceV3 file parsed")
	assert.Contains(t, logRecord.Body().AsString(), "1024 bytes") // Our sample size
}

func TestChunkHeaderAnalysis(t *testing.T) {
	// Load our test sample
	data, err := ioutil.ReadFile("../../../local/test_sample.tracev3")
	require.NoError(t, err, "Failed to read test sample")

	t.Log("=== Chunk Header Analysis ===")

	// Based on research, let's try to identify chunk boundaries
	// Look for the pattern that was causing our error: 0x600b
	problemBytes := []byte{0x0b, 0x60, 0x00, 0x00}

	for i := 0; i <= len(data)-4; i++ {
		if data[i] == problemBytes[0] && data[i+1] == problemBytes[1] &&
			data[i+2] == problemBytes[2] && data[i+3] == problemBytes[3] {
			t.Logf("Found 0x600b pattern at offset 0x%04x", i)

			// Show context around this location
			start := i - 16
			if start < 0 {
				start = 0
			}
			end := i + 32
			if end > len(data) {
				end = len(data)
			}

			t.Logf("Context: %s", hex.EncodeToString(data[start:end]))
		}
	}
}

func TestComprehensiveParsingResults(t *testing.T) {
	// Load our test sample
	data, err := ioutil.ReadFile("../../../local/test_sample.tracev3")
	require.NoError(t, err, "Failed to read test sample")

	config := &Config{
		ParsePrivateLogs:      false,
		IncludeSignpostEvents: true,
		IncludeActivityEvents: true,
		MaxLogSize:            65536,
	}

	decoder := &macosUnifiedLoggingDecoder{config: config}

	// Test that our decoder can extract multiple entries
	logs, err := decoder.UnmarshalLogs(data)
	require.NoError(t, err, "Decoder should handle actual traceV3 data")

	t.Logf("Total log records generated: %d", logs.LogRecordCount())

	// Examine all log records
	resourceLogs := logs.ResourceLogs().At(0)
	scopeLogs := resourceLogs.ScopeLogs().At(0)

	for i := 0; i < scopeLogs.LogRecords().Len(); i++ {
		logRecord := scopeLogs.LogRecords().At(i)

		t.Logf("=== Log Record %d ===", i)
		t.Logf("Message: %s", logRecord.Body().AsString())
		t.Logf("Severity: %s", logRecord.SeverityText())

		// Log all attributes
		attrs := logRecord.Attributes()
		attrs.Range(func(k string, v pcommon.Value) bool {
			t.Logf("  %s: %s", k, v.AsString())
			return true
		})
	}

	// We should have found multiple entries based on our analysis
	assert.True(t, logs.LogRecordCount() > 1, "Should find multiple log entries")
}
