// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package macosunifiedloggingencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/macosunifiedloggingencodingextension"

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"
)

var (
	_ encoding.LogsUnmarshalerExtension = (*MacosUnifiedLoggingExtension)(nil)
)

// MacosUnifiedLoggingExtension implements the LogsUnmarshalerExtension interface
// for decoding macOS Unified Logging binary data.
type MacosUnifiedLoggingExtension struct {
	config *Config
	codec  *macosUnifiedLoggingCodec
}

// UnmarshalLogs unmarshals binary data into OpenTelemetry log records.
// Currently reads binary data but does not decode it - just creates placeholder records.
func (e *MacosUnifiedLoggingExtension) UnmarshalLogs(buf []byte) (plog.Logs, error) {
	return e.codec.UnmarshalLogs(buf)
}

// Start initializes the extension.
func (e *MacosUnifiedLoggingExtension) Start(_ context.Context, _ component.Host) error {
	e.codec = &macosUnifiedLoggingCodec{
		debugMode: e.config.DebugMode,
	}
	return nil
}

// Shutdown shuts down the extension.
func (*MacosUnifiedLoggingExtension) Shutdown(context.Context) error {
	return nil
}

// macosUnifiedLoggingCodec handles the actual decoding of macOS Unified Logging binary data.
type macosUnifiedLoggingCodec struct {
	debugMode bool
}

// UnmarshalLogs reads binary data and parses tracev3 entries into individual log records.
func (c *macosUnifiedLoggingCodec) UnmarshalLogs(buf []byte) (plog.Logs, error) {
	// Check if we have any data
	if len(buf) == 0 {
		return plog.NewLogs(), nil
	}

	// Parse the tracev3 binary data into individual entries
	entries, err := ParseTraceV3Data(buf)
	if err != nil {
		// If parsing fails, fall back to creating a single entry with error info
		logs := plog.NewLogs()
		resourceLogs := logs.ResourceLogs().AppendEmpty()
		scopeLogs := resourceLogs.ScopeLogs().AppendEmpty()
		logRecord := scopeLogs.LogRecords().AppendEmpty()

		logRecord.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		logRecord.SetObservedTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		logRecord.SetSeverityNumber(plog.SeverityNumberError)
		logRecord.SetSeverityText("ERROR")

		message := fmt.Sprintf("Failed to parse tracev3 data (%d bytes): %v", len(buf), err)
		if c.debugMode && len(buf) >= 16 {
			message += fmt.Sprintf(" (first 16 bytes: %x)", buf[:16])
		}

		logRecord.Body().SetStr(message)
		logRecord.Attributes().PutStr("source", "macos_unified_logging")
		logRecord.Attributes().PutInt("data_size", int64(len(buf)))
		logRecord.Attributes().PutBool("decoded", false)
		logRecord.Attributes().PutStr("error", err.Error())

		return logs, nil
	}

	// Convert parsed entries to OpenTelemetry logs
	logs := ConvertTraceV3EntriesToLogs(entries)

	// Add debug information if requested
	if c.debugMode && len(buf) >= 16 {
		// Add a debug entry showing raw binary data
		resourceLogs := logs.ResourceLogs().AppendEmpty()
		scopeLogs := resourceLogs.ScopeLogs().AppendEmpty()
		logRecord := scopeLogs.LogRecords().AppendEmpty()

		logRecord.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		logRecord.SetObservedTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		logRecord.SetSeverityNumber(plog.SeverityNumberDebug)
		logRecord.SetSeverityText("DEBUG")

		message := fmt.Sprintf("Raw tracev3 data: %d bytes, %d entries parsed (first 16 bytes: %x)",
			len(buf), len(entries), buf[:16])
		logRecord.Body().SetStr(message)
		logRecord.Attributes().PutStr("source", "macos_unified_logging_debug")
		logRecord.Attributes().PutInt("data_size", int64(len(buf)))
		logRecord.Attributes().PutInt("parsed_entries", int64(len(entries)))
		logRecord.Attributes().PutBool("decoded", true)
	}

	return logs, nil
}
