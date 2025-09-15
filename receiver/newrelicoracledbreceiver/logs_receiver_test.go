// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package newrelicoracledbreceiver

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

func TestLogsConfig_Validate(t *testing.T) {
	tests := []struct {
		name        string
		config      LogsConfig
		expectError bool
	}{
		{
			name: "valid config with file collection",
			config: LogsConfig{
				EnableLogs:       true,
				CollectAlertLogs: true,
				AlertLogPath:     "/opt/oracle/diag/alert.log",
				BatchSize:        100,
			},
			expectError: false,
		},
		{
			name: "valid config with query collection",
			config: LogsConfig{
				EnableLogs:           true,
				QueryBasedCollection: true,
				AlertLogQuery:        "SELECT * FROM v$diag_alert_ext",
				BatchSize:            100,
			},
			expectError: false,
		},
		{
			name: "invalid config - no log sources enabled",
			config: LogsConfig{
				EnableLogs: true,
				BatchSize:  100,
			},
			expectError: true,
		},
		{
			name: "invalid config - alert logs enabled but no path",
			config: LogsConfig{
				EnableLogs:       true,
				CollectAlertLogs: true,
				BatchSize:        100,
			},
			expectError: true,
		},
		{
			name: "invalid config - negative batch size",
			config: LogsConfig{
				EnableLogs:       true,
				CollectAlertLogs: true,
				AlertLogPath:     "/opt/oracle/diag/alert.log",
				BatchSize:        -1,
			},
			expectError: true,
		},
		{
			name: "invalid config - invalid log level",
			config: LogsConfig{
				EnableLogs:       true,
				CollectAlertLogs: true,
				AlertLogPath:     "/opt/oracle/diag/alert.log",
				MinLogLevel:      "INVALID",
				BatchSize:        100,
			},
			expectError: true,
		},
		{
			name: "disabled logs config",
			config: LogsConfig{
				EnableLogs: false,
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				LogsConfig: tt.config,
			}

			err := cfg.validateLogsConfig()
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestNewLogsReceiver(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		expectError bool
	}{
		{
			name: "valid config",
			config: &Config{
				LogsConfig: LogsConfig{
					EnableLogs:       true,
					CollectAlertLogs: true,
					AlertLogPath:     "/tmp/alert.log",
					BatchSize:        100,
				},
			},
			expectError: false,
		},
		{
			name: "logs disabled",
			config: &Config{
				LogsConfig: LogsConfig{
					EnableLogs: false,
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settings := receivertest.NewNopSettings(component.MustNewType("oracledb"))
			consumer := &consumertest.LogsSink{}

			receiver, err := NewLogsReceiver(tt.config, settings, consumer)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, receiver)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, receiver)
			}
		})
	}
}

func TestLogsReceiver_Lifecycle(t *testing.T) {
	cfg := &Config{
		LogsConfig: LogsConfig{
			EnableLogs:       true,
			CollectAlertLogs: true,
			AlertLogPath:     "/tmp/nonexistent.log", // File doesn't exist, but that's OK for lifecycle test
			PollInterval:     "1s",
			BatchSize:        100,
		},
	}

	settings := receivertest.NewNopSettings(component.MustNewType("oracledb"))
	consumer := &consumertest.LogsSink{}

	receiver, err := NewLogsReceiver(cfg, settings, consumer)
	require.NoError(t, err)
	require.NotNil(t, receiver)

	// Test Start
	ctx := context.Background()
	err = receiver.Start(ctx, nil)
	assert.NoError(t, err)

	// Wait a short time to ensure the receiver is running
	time.Sleep(100 * time.Millisecond)

	// Test Shutdown
	err = receiver.Shutdown(ctx)
	assert.NoError(t, err)
}

func TestLogsReceiver_ShouldIncludeLogEntry(t *testing.T) {
	cfg := &Config{
		LogsConfig: LogsConfig{
			EnableLogs:     true,
			MinLogLevel:    "INFO",
			ExcludeLevels:  []string{"DEBUG"},
			IncludeClasses: []string{}, // Empty - should include all
			ExcludeClasses: []string{"background"},
		},
	}

	receiver := &LogsReceiver{cfg: cfg}

	tests := []struct {
		name     string
		entry    *LogEntry
		expected bool
	}{
		{
			name: "info level - should include",
			entry: &LogEntry{
				Level:   "INFO",
				Message: "Test message",
			},
			expected: true,
		},
		{
			name: "debug level - should exclude (below min level)",
			entry: &LogEntry{
				Level:   "DEBUG",
				Message: "Debug message",
			},
			expected: false,
		},
		{
			name: "debug level - should exclude (in exclude list)",
			entry: &LogEntry{
				Level:   "DEBUG",
				Message: "Debug message",
			},
			expected: false,
		},
		{
			name: "error level - should include",
			entry: &LogEntry{
				Level:   "ERROR",
				Message: "Error message",
			},
			expected: true,
		},
		{
			name: "auth component - should include",
			entry: &LogEntry{
				Level:     "INFO",
				Component: "authentication",
				Message:   "Auth message",
			},
			expected: true,
		},
		{
			name: "background component - should exclude",
			entry: &LogEntry{
				Level:     "INFO",
				Component: "background_process",
				Message:   "Background message",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := receiver.shouldIncludeLogEntry(tt.entry)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseAlertLogEntry(t *testing.T) {
	cfg := &Config{
		LogsConfig: LogsConfig{
			ParseErrors:        true,
			ParseSQLStatements: true,
		},
	}

	receiver := &LogsReceiver{cfg: cfg}

	tests := []struct {
		name     string
		line     string
		expected *LogEntry
	}{
		{
			name: "standard alert log entry with timestamp",
			line: "2023-09-15T10:30:45.123+00:00 Database mounted",
			expected: &LogEntry{
				Level:   "INFO",
				Message: "Database mounted",
				RawLine: "2023-09-15T10:30:45.123+00:00 Database mounted",
			},
		},
		{
			name: "oracle error entry",
			line: "2023-09-15T10:30:45.123+00:00 ORA-00001: unique constraint violated",
			expected: &LogEntry{
				Level:     "ERROR",
				Message:   "unique constraint violated",
				ErrorCode: "ORA-00001",
				RawLine:   "2023-09-15T10:30:45.123+00:00 ORA-00001: unique constraint violated",
			},
		},
		{
			name: "entry with SQL statement",
			line: "2023-09-15T10:30:45.123+00:00 SELECT * FROM employees WHERE id = 1",
			expected: &LogEntry{
				Level:        "INFO",
				Message:      "SELECT * FROM employees WHERE id = 1",
				SQLStatement: "SELECT * FROM employees WHERE id = 1",
				Operation:    "SELECT",
				RawLine:      "2023-09-15T10:30:45.123+00:00 SELECT * FROM employees WHERE id = 1",
			},
		},
		{
			name:     "empty line",
			line:     "",
			expected: nil,
		},
		{
			name:     "whitespace only",
			line:     "   ",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := receiver.parseAlertLogEntry(tt.line)

			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				require.NotNil(t, result)
				assert.Equal(t, tt.expected.Level, result.Level)
				assert.Equal(t, tt.expected.Message, result.Message)
				assert.Equal(t, tt.expected.ErrorCode, result.ErrorCode)
				assert.Equal(t, tt.expected.SQLStatement, result.SQLStatement)
				assert.Equal(t, tt.expected.Operation, result.Operation)
				assert.Equal(t, tt.expected.RawLine, result.RawLine)
			}
		})
	}
}

func TestParseAuditLogEntry(t *testing.T) {
	receiver := &LogsReceiver{}

	tests := []struct {
		name     string
		line     string
		expected *LogEntry
	}{
		{
			name: "XML audit log entry",
			line: `<Audit xmlns="http://xmlns.oracle.com/oraaudit" Timestamp="2023-09-15T10:30:45.123Z" ACTION="100" RETURNCODE="0">`,
			expected: &LogEntry{
				Level:       "INFO",
				Component:   "audit",
				AuditAction: "100",
				AuditResult: "0",
				Message:     `<Audit xmlns="http://xmlns.oracle.com/oraaudit" Timestamp="2023-09-15T10:30:45.123Z" ACTION="100" RETURNCODE="0">`,
			},
		},
		{
			name: "XML audit log entry with error",
			line: `<Audit xmlns="http://xmlns.oracle.com/oraaudit" Timestamp="2023-09-15T10:30:45.123Z" ACTION="103" RETURNCODE="1017">`,
			expected: &LogEntry{
				Level:       "ERROR",
				Component:   "audit",
				AuditAction: "103",
				AuditResult: "1017",
				Message:     `<Audit xmlns="http://xmlns.oracle.com/oraaudit" Timestamp="2023-09-15T10:30:45.123Z" ACTION="103" RETURNCODE="1017">`,
			},
		},
		{
			name: "legacy audit log entry",
			line: "Fri Sep 15 10:30:45 2023 +00:00 : LENGTH : '142' CLIENT USER:[1]:'oracle' CLIENT TERMINAL:[1]:'console'",
			expected: &LogEntry{
				Level:   "INFO",
				Message: "Fri Sep 15 10:30:45 2023 +00:00 : LENGTH : '142' CLIENT USER:[1]:'oracle' CLIENT TERMINAL:[1]:'console'",
			},
		},
		{
			name:     "empty line",
			line:     "",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := receiver.parseAuditLogEntry(tt.line)

			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				require.NotNil(t, result)
				assert.Equal(t, tt.expected.Level, result.Level)
				assert.Equal(t, tt.expected.Component, result.Component)
				assert.Equal(t, tt.expected.AuditAction, result.AuditAction)
				assert.Equal(t, tt.expected.AuditResult, result.AuditResult)
				assert.Equal(t, tt.expected.Message, result.Message)
			}
		})
	}
}

func TestSetResourceAttributes(t *testing.T) {
	cfg := &Config{
		Endpoint: "localhost:1521",
		Service:  "ORCL",
	}

	receiver := &LogsReceiver{cfg: cfg}

	// Create a mock resource
	logs := plog.NewLogs()
	resourceLogs := logs.ResourceLogs().AppendEmpty()

	receiver.setResourceAttributes(resourceLogs.Resource(), "/opt/oracle/alert.log", "alert")

	attrs := resourceLogs.Resource().Attributes()

	// Check resource attributes
	hostName, exists := attrs.Get("host.name")
	assert.True(t, exists)
	assert.Equal(t, "localhost", hostName.Str())

	logSource, exists := attrs.Get("oracledb.log.source")
	assert.True(t, exists)
	assert.Equal(t, "alert", logSource.Str())

	fileName, exists := attrs.Get("oracledb.log.file.name")
	assert.True(t, exists)
	assert.Equal(t, "alert.log", fileName.Str())

	filePath, exists := attrs.Get("oracledb.log.file.path")
	assert.True(t, exists)
	assert.Equal(t, "/opt/oracle/alert.log", filePath.Str())
}

func TestPopulateLogRecord(t *testing.T) {
	receiver := &LogsReceiver{}

	entry := &LogEntry{
		Timestamp:    time.Date(2023, 9, 15, 10, 30, 45, 0, time.UTC),
		Level:        "ERROR",
		Message:      "Test error message",
		Component:    "database",
		SessionID:    "12345",
		ErrorCode:    "ORA-00001",
		SQLStatement: "INSERT INTO test VALUES (1)",
		Operation:    "INSERT",
	}

	// Create a mock log record
	logs := plog.NewLogs()
	resourceLogs := logs.ResourceLogs().AppendEmpty()
	logRecord := resourceLogs.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()

	receiver.populateLogRecord(logRecord, entry, "/opt/oracle/alert.log", "alert")

	// Check log record properties
	assert.Equal(t, "Test error message", logRecord.Body().Str())
	assert.Equal(t, plog.SeverityNumberError, logRecord.SeverityNumber())
	assert.Equal(t, "ERROR", logRecord.SeverityText())

	// Check attributes
	attrs := logRecord.Attributes()

	severity, exists := attrs.Get("log.severity")
	assert.True(t, exists)
	assert.Equal(t, "ERROR", severity.Str())

	component, exists := attrs.Get("log.component")
	assert.True(t, exists)
	assert.Equal(t, "database", component.Str())

	sessionID, exists := attrs.Get("log.session_id")
	assert.True(t, exists)
	assert.Equal(t, "12345", sessionID.Str())

	errorCode, exists := attrs.Get("log.error_code")
	assert.True(t, exists)
	assert.Equal(t, "ORA-00001", errorCode.Str())

	sqlStatement, exists := attrs.Get("log.sql_statement")
	assert.True(t, exists)
	assert.Equal(t, "INSERT INTO test VALUES (1)", sqlStatement.Str())

	operation, exists := attrs.Get("log.operation")
	assert.True(t, exists)
	assert.Equal(t, "INSERT", operation.Str())
}

func TestDefaultLogsConfig(t *testing.T) {
	cfg := createDefaultConfig().(*Config)

	// Check that logs are disabled by default
	assert.False(t, cfg.LogsConfig.EnableLogs)
	assert.True(t, cfg.LogsConfig.CollectAlertLogs)
	assert.False(t, cfg.LogsConfig.CollectAuditLogs)
	assert.False(t, cfg.LogsConfig.CollectTraceFiles)
	assert.False(t, cfg.LogsConfig.CollectArchiveLogs)
	assert.Equal(t, "30s", cfg.LogsConfig.PollInterval)
	assert.Equal(t, int64(100), cfg.LogsConfig.MaxLogFileSize)
	assert.Equal(t, 100, cfg.LogsConfig.BatchSize)
}
