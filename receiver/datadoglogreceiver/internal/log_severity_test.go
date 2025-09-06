// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"testing"
)

func TestExtractSeverity(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// Standard cases
		{"Standard ERROR", "ERROR: Something went wrong", "ERROR"},
		{"Standard WARN", "WARN: This is a warning", "WARN"},
		{"Standard INFO", "INFO: Just letting you know", "INFO"},
		{"Standard DEBUG", "DEBUG: Here's what's happening", "DEBUG"},
		{"Standard TRACE", "TRACE: Detailed info for tracing", "TRACE"},
		{"Standard UNKNOWN", "UNKNOWN: This is unknown", "UNKNOWN"},
		// Lowercase variations
		{"Lowercase error", "error: lowercase error", "ERROR"},
		{"Lowercase warn", "warn: lowercase warning", "WARN"},
		{"Lowercase info", "info: lowercase info", "INFO"},
		{"Lowercase debug", "debug: lowercase debug", "DEBUG"},
		{"Lowercase trace", "trace: lowercase trace", "TRACE"},

		// Mixed case variations
		{"Mixed case ERROR", "ErRoR: Mixed case error", "ERROR"},
		{"Mixed case WARN", "WaRn: Mixed case warning", "WARN"},
		{"Mixed case INFO", "InFo: Mixed case info", "INFO"},
		{"Mixed case DEBUG", "DeBuG: Mixed case debug", "DEBUG"},
		{"Mixed case TRACE", "TrAcE: Mixed case trace", "TRACE"},

		// Abbreviated variations
		{"Abbreviated ERR", "ERR: Abbreviated error", "ERROR"},
		{"Abbreviated WRN", "WRN: Abbreviated warning", "WARN"},
		{"Abbreviated INF", "INF: Abbreviated info", "INFO"},
		{"Abbreviated DBG", "DBG: Abbreviated debug", "DEBUG"},
		{"Abbreviated TRC", "TRC: Abbreviated trace", "TRACE"},

		// Single letter variations
		{"Single letter E", "E: Single letter error", "ERROR"},
		{"Single letter W", "W: Single letter warning", "WARN"},
		{"Single letter I", "I: Single letter info", "INFO"},
		{"Single letter D", "D: Single letter debug", "DEBUG"},
		{"Single letter T", "T: Single letter trace", "TRACE"},

		// Multiple severities (should return the first match)
		{"Multiple severities", "ERROR WARNING INFO DEBUG TRACE", "ERROR"},

		// With timestamps and other log elements
		{"Timestamp ERROR", "2023-03-15T14:30:00Z ERROR: Timestamp error", "ERROR"},
		{"Timestamp WARN", "2023-03-15T14:30:00Z WARN: Timestamp warning", "WARN"},
		{"Timestamp INFO", "2023-03-15T14:30:00Z INFO: Timestamp info", "INFO"},
		{"Complex log ERROR", "[2023-03-15T14:30:00Z] ERROR AppName: Complex log error", "ERROR"},
		{"Complex log WARN", "[2023-03-15T14:30:00Z] WARN AppName: Complex log warning", "WARN"},

		// With timestamps and other log elements
		{"Timestamp ERROR", "2023-03-15T14:30:00Z level=error: Timestamp error", "ERROR"},
		{"Timestamp WARN", "2023-03-15T14:30:00Z level=warn: Timestamp warning", "WARN"},
		{"Timestamp INFO", "2023-03-15T14:30:00Z level=info: Timestamp info", "INFO"},
		{"Complex log ERROR", "[2023-03-15T14:30:00Z] level=error AppName: Complex log error", "ERROR"},
		{"Complex log WARN", "[2023-03-15T14:30:00Z] level=warn AppName: Complex log warning", "WARN"},

		// Edge cases
		{"No severity", "This log has no severity level", "UNKNOWN"},
		{"Empty string", "", "UNKNOWN"},
		{"Only whitespace", "   \t\n", "UNKNOWN"},
		{"Misleading words", "WATER: message", "UNKNOWN"},
		{"Partial match", "IN: This is an INFO log", "INFO"},
		{"Single letter with colon", "E: Error message", "ERROR"},
		{"Single letter with space", "W: Warning message", "WARN"},
		{"Single letter at end", "Log ending with E", "UNKNOWN"},
		{"Word containing severity", "SOFTWARE: Not a warning", "UNKNOWN"},
		{"Simple Metrics Service", "Perfroming Request", "UNKNOWN"},

		// Special cases from the previous conversation
		{"Datadog log example", "2024-10-04T03:02:55.787Z\tinfo\tdatadoglogreceiver@v0.99.0/logs-receiver.go:58\tDatadog Logs receiver", "INFO"},
		{"external-dns example", "time=\"2024-10-17T18:11:17Z\" level=info msg=\"All records are already up to date\"", "INFO"},
		// Additional edge cases
		{"Severity at end", "Log ending with ERROR", "ERROR"},
		{"Severity in middle", "Start INFO middle WARN end", "INFO"},
		{"Incorrect ordering", "ERRORWARNINFODEBUGTRACE", "UNKNOWN"},
		{"Separated severity", "ERR OR: Not an error", "ERROR"},
		{"Repeated severity", "INFO INFO INFO", "INFO"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractSeverity([]byte(tt.input))
			if result != tt.expected {
				t.Errorf("extractSeverity(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// Benchmark to measure performance
func BenchmarkExtractSeverity(b *testing.B) {
	inputs := [][]byte{
		[]byte("ERROR: This is an error message"),
		[]byte("2023-03-15T14:30:00Z WARN: This is a warning"),
		[]byte("INFO: Just an informational message"),
		[]byte("This log has no severity level"),
		[]byte("2024-10-04T03:02:55.787Z\tinfo\tdatadoglogreceiver@v0.99.0/logs-receiver.go:58\tDatadog Logs receiver"),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		extractSeverity(inputs[i%len(inputs)])
	}
}
