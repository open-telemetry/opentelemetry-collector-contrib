// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package journald

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
)

func TestConvertFieldValue(t *testing.T) {
	tests := []struct {
		name     string
		otelKey  string
		input    any
		expected any
	}{
		{
			name:     "numeric field with valid integer string",
			otelKey:  "process.pid",
			input:    "1234",
			expected: int64(1234),
		},
		{
			name:     "numeric field with negative integer",
			otelKey:  "thread.id",
			input:    "-2",
			expected: int64(-2),
		},
		{
			name:     "numeric field with unparseable string falls back to string",
			otelKey:  "code.lineno",
			input:    "not-a-number",
			expected: "not-a-number",
		},
		{
			name:     "numeric field with non-string value passes through",
			otelKey:  "thread.id",
			input:    int64(42),
			expected: int64(42),
		},
		{
			name:     "non-numeric field returns value as-is",
			otelKey:  "code.filepath",
			input:    "/src/main.c",
			expected: "/src/main.c",
		},
		{
			name:     "non-numeric field with numeric string stays as string",
			otelKey:  "host.name",
			input:    "12345",
			expected: "12345",
		},
		{
			name:     "syslog.facility.code numeric conversion",
			otelKey:  "syslog.facility.code",
			input:    "3",
			expected: int64(3),
		},
		{
			name:     "syslog.pid numeric conversion",
			otelKey:  "syslog.pid",
			input:    "9876",
			expected: int64(9876),
		},
		{
			name:     "code.line.number numeric conversion",
			otelKey:  "code.line.number",
			input:    "42",
			expected: int64(42),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertFieldValue(tt.otelKey, tt.input)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestPriorityToSeverityMapping(t *testing.T) {
	tests := []struct {
		priority         string
		expectedSeverity entry.Severity
		expectedText     string
	}{
		{"0", entry.Fatal, "emerg"},
		{"1", entry.Fatal2, "alert"},
		{"2", entry.Fatal3, "crit"},
		{"3", entry.Error, "err"},
		{"4", entry.Warn, "warning"},
		{"5", entry.Info2, "notice"},
		{"6", entry.Info, "info"},
		{"7", entry.Debug, "debug"},
	}

	for _, tt := range tests {
		t.Run("priority_"+tt.priority, func(t *testing.T) {
			sev, ok := priorityToSeverity[tt.priority]
			require.True(t, ok, "priority %q not in map", tt.priority)
			assert.Equal(t, tt.expectedSeverity, sev)

			text, ok := priorityToSeverityText[tt.priority]
			require.True(t, ok, "priority %q not in text map", tt.priority)
			assert.Equal(t, tt.expectedText, text)
		})
	}
}

func TestMapJournalEntryAttributes_Message(t *testing.T) {
	e := entry.New()
	body := map[string]any{
		"MESSAGE": "hello world",
	}
	mapJournalEntryAttributes(e, body)
	assert.Equal(t, "hello world", e.Body)
}

func TestMapJournalEntryAttributes_MissingMessage(t *testing.T) {
	e := entry.New()
	body := map[string]any{
		"PRIORITY": "6",
	}
	mapJournalEntryAttributes(e, body)
	// Body should remain nil when MESSAGE is absent
	assert.Nil(t, e.Body)
	assert.Equal(t, entry.Info, e.Severity)
}

func TestMapJournalEntryAttributes_AllPriorities(t *testing.T) {
	expectations := []struct {
		priority string
		severity entry.Severity
		text     string
	}{
		{"0", entry.Fatal, "emerg"},
		{"1", entry.Fatal2, "alert"},
		{"2", entry.Fatal3, "crit"},
		{"3", entry.Error, "err"},
		{"4", entry.Warn, "warning"},
		{"5", entry.Info2, "notice"},
		{"6", entry.Info, "info"},
		{"7", entry.Debug, "debug"},
	}

	for _, exp := range expectations {
		t.Run("priority_"+exp.priority, func(t *testing.T) {
			e := entry.New()
			mapJournalEntryAttributes(e, map[string]any{"PRIORITY": exp.priority})
			assert.Equal(t, exp.severity, e.Severity)
			assert.Equal(t, exp.text, e.SeverityText)
		})
	}
}

func TestMapJournalEntryAttributes_UnknownPriority(t *testing.T) {
	e := entry.New()
	// PRIORITY "8" is out of range; severity should stay Default
	mapJournalEntryAttributes(e, map[string]any{"PRIORITY": "8"})
	assert.Equal(t, entry.Default, e.Severity)
	assert.Equal(t, "", e.SeverityText)
}

func TestMapJournalEntryAttributes_NonStringPriority(t *testing.T) {
	e := entry.New()
	// PRIORITY as a non-string (e.g. JSON number decoded as float64) should be ignored
	mapJournalEntryAttributes(e, map[string]any{"PRIORITY": float64(6)})
	assert.Equal(t, entry.Default, e.Severity)
}

func TestMapJournalEntryAttributes_KnownLogAttributes(t *testing.T) {
	e := entry.New()
	body := map[string]any{
		"CODE_FILE":         "/src/unit.c",
		"CODE_FUNC":         "unit_log_success",
		"CODE_LINE":         "42",
		"TID":               "100",
		"SYSLOG_FACILITY":   "3",
		"SYSLOG_IDENTIFIER": "myapp",
		"SYSLOG_PID":        "5678",
	}
	mapJournalEntryAttributes(e, body)

	require.NotNil(t, e.Attributes)
	assert.Equal(t, "/src/unit.c", e.Attributes["code.file.path"])
	assert.Equal(t, "unit_log_success", e.Attributes["code.function.name"])
	assert.Equal(t, int64(42), e.Attributes["code.line.number"])
	assert.Equal(t, int64(100), e.Attributes["thread.id"])
	assert.Equal(t, int64(3), e.Attributes["syslog.facility.code"])
	assert.Equal(t, "myapp", e.Attributes["syslog.msg.id"])
	assert.Equal(t, int64(5678), e.Attributes["syslog.pid"])

	// Original journald field names must not appear
	assert.NotContains(t, e.Attributes, "CODE_FILE")
	assert.NotContains(t, e.Attributes, "CODE_FUNC")
	assert.NotContains(t, e.Attributes, "CODE_LINE")
	assert.NotContains(t, e.Attributes, "TID")
	assert.NotContains(t, e.Attributes, "SYSLOG_FACILITY")
	assert.NotContains(t, e.Attributes, "SYSLOG_IDENTIFIER")
	assert.NotContains(t, e.Attributes, "SYSLOG_PID")
}

func TestMapJournalEntryAttributes_KnownResourceAttributes(t *testing.T) {
	e := entry.New()
	body := map[string]any{
		"_HOSTNAME": "myhost",
		"_PID":      "1234",
		"_COMM":     "myapp",
		"_EXE":      "/usr/bin/myapp",
		"_CMDLINE":  "/usr/bin/myapp --config /etc/myapp.conf",
	}
	mapJournalEntryAttributes(e, body)

	require.NotNil(t, e.Resource)
	assert.Equal(t, "myhost", e.Resource["host.name"])
	assert.Equal(t, int64(1234), e.Resource["process.pid"])
	assert.Equal(t, "myapp", e.Resource["process.command"])
	assert.Equal(t, "/usr/bin/myapp", e.Resource["process.executable.path"])
	assert.Equal(t, "/usr/bin/myapp --config /etc/myapp.conf", e.Resource["process.command_line"])

	// Original trusted field names must not appear in resource
	assert.NotContains(t, e.Resource, "_HOSTNAME")
	assert.NotContains(t, e.Resource, "_PID")
	assert.NotContains(t, e.Resource, "_COMM")
	assert.NotContains(t, e.Resource, "_EXE")
	assert.NotContains(t, e.Resource, "_CMDLINE")
}

func TestMapJournalEntryAttributes_UnmappedFieldsGoToAttributes(t *testing.T) {
	e := entry.New()
	body := map[string]any{
		"_BOOT_ID":              "abc123",
		"_TRANSPORT":            "journal",
		"_UID":                  "1000",
		"_GID":                  "1000",
		"_SYSTEMD_UNIT":         "myapp.service",
		"__CURSOR":              "cursor-value",
		"__MONOTONIC_TIMESTAMP": "123456789",
		"USER_UNIT":             "myapp.mount",
		"MESSAGE_ID":            "7ad2d189f7e94e70a38c781354912448",
	}
	mapJournalEntryAttributes(e, body)

	require.NotNil(t, e.Attributes)
	assert.Equal(t, "abc123", e.Attributes["_BOOT_ID"])
	assert.Equal(t, "journal", e.Attributes["_TRANSPORT"])
	assert.Equal(t, "1000", e.Attributes["_UID"])
	assert.Equal(t, "1000", e.Attributes["_GID"])
	assert.Equal(t, "myapp.service", e.Attributes["_SYSTEMD_UNIT"])
	assert.Equal(t, "cursor-value", e.Attributes["__CURSOR"])
	assert.Equal(t, "123456789", e.Attributes["__MONOTONIC_TIMESTAMP"])
	assert.Equal(t, "myapp.mount", e.Attributes["USER_UNIT"])
	assert.Equal(t, "7ad2d189f7e94e70a38c781354912448", e.Attributes["MESSAGE_ID"])
}

func TestMapJournalEntryAttributes_KnownFieldsNotInAttributes(t *testing.T) {
	// Resource fields must not appear in attributes, and vice versa
	e := entry.New()
	body := map[string]any{
		"_PID":      "99",
		"_HOSTNAME": "host1",
		"CODE_FILE": "main.go",
	}
	mapJournalEntryAttributes(e, body)

	assert.NotContains(t, e.Attributes, "_PID")
	assert.NotContains(t, e.Attributes, "_HOSTNAME")
	assert.NotContains(t, e.Resource, "CODE_FILE")
	assert.NotContains(t, e.Resource, "code.file.path")
}

func TestMapJournalEntryAttributes_EmptyBody(t *testing.T) {
	e := entry.New()
	mapJournalEntryAttributes(e, map[string]any{})
	assert.Nil(t, e.Body)
	assert.Equal(t, entry.Default, e.Severity)
	assert.Nil(t, e.Attributes)
	assert.Nil(t, e.Resource)
}

func TestMapJournalEntryAttributes_FullEntry(t *testing.T) {
	// Integration test combining all field categories in one call
	e := entry.New()
	body := map[string]any{
		"MESSAGE":           "test message",
		"PRIORITY":          "3",
		"CODE_FILE":         "main.c",
		"CODE_FUNC":         "main",
		"CODE_LINE":         "100",
		"TID":               "200",
		"SYSLOG_FACILITY":   "1",
		"SYSLOG_IDENTIFIER": "kernel",
		"SYSLOG_PID":        "1",
		"_HOSTNAME":         "box",
		"_PID":              "42",
		"_COMM":             "init",
		"_EXE":              "/sbin/init",
		"_CMDLINE":          "/sbin/init splash",
		"_TRANSPORT":        "kernel",
		"_BOOT_ID":          "deadbeef",
		"__CURSOR":          "s=abc;i=1",
	}
	mapJournalEntryAttributes(e, body)

	// Body
	assert.Equal(t, "test message", e.Body)

	// Severity (PRIORITY "3" → Error)
	assert.Equal(t, entry.Error, e.Severity)
	assert.Equal(t, "err", e.SeverityText)

	// Log attributes with OTel names
	assert.Equal(t, "main.c", e.Attributes["code.file.path"])
	assert.Equal(t, "main", e.Attributes["code.function.name"])
	assert.Equal(t, int64(100), e.Attributes["code.line.number"])
	assert.Equal(t, int64(200), e.Attributes["thread.id"])
	assert.Equal(t, int64(1), e.Attributes["syslog.facility.code"])
	assert.Equal(t, "kernel", e.Attributes["syslog.msg.id"])
	assert.Equal(t, int64(1), e.Attributes["syslog.pid"])

	// Resource attributes with OTel names
	assert.Equal(t, "box", e.Resource["host.name"])
	assert.Equal(t, int64(42), e.Resource["process.pid"])
	assert.Equal(t, "init", e.Resource["process.command"])
	assert.Equal(t, "/sbin/init", e.Resource["process.executable.path"])
	assert.Equal(t, "/sbin/init splash", e.Resource["process.command_line"])

	// Unmapped fields land in attributes with original names
	assert.Equal(t, "kernel", e.Attributes["_TRANSPORT"])
	assert.Equal(t, "deadbeef", e.Attributes["_BOOT_ID"])
	assert.Equal(t, "s=abc;i=1", e.Attributes["__CURSOR"])

	// Consumed fields must not appear
	assert.NotContains(t, e.Attributes, "MESSAGE")
	assert.NotContains(t, e.Attributes, "PRIORITY")
}
