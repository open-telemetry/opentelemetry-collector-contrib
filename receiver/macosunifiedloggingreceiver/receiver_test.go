// Copyright observIQ, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build darwin

package macosunifiedloggingreceiver

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
)

func TestBuildLogCommandArgs(t *testing.T) {
	t.Run("with ndjson format", func(t *testing.T) {
		receiver := &unifiedLoggingReceiver{
			config: &Config{
				ArchivePath: "./testdata/system_logs.logarchive",
				StartTime:   "2024-01-01 00:00:00",
				EndTime:     "2024-01-02 00:00:00",
				Predicate:   "subsystem == 'com.apple.systempreferences'",
				Format:      "ndjson",
			},
		}

		args := receiver.buildLogCommandArgs("./testdata/system_logs.logarchive")
		require.Contains(t, args, "--archive")
		require.Contains(t, args, "./testdata/system_logs.logarchive")
		require.Contains(t, args, "--start")
		require.Contains(t, args, "2024-01-01 00:00:00")
		require.Contains(t, args, "--end")
		require.Contains(t, args, "2024-01-02 00:00:00")
		require.Contains(t, args, "--predicate")
		require.Contains(t, args, "subsystem == 'com.apple.systempreferences'")
		require.Contains(t, args, "--style")
		require.Contains(t, args, "ndjson")
	})

	t.Run("with default format", func(t *testing.T) {
		receiver := &unifiedLoggingReceiver{
			config: &Config{
				ArchivePath: "./testdata/system_logs.logarchive",
				StartTime:   "2024-01-01 00:00:00",
				Predicate:   "subsystem == 'com.apple.systempreferences'",
				Format:      "default",
			},
		}

		args := receiver.buildLogCommandArgs("./testdata/system_logs.logarchive")
		require.Contains(t, args, "--archive")
		require.Contains(t, args, "./testdata/system_logs.logarchive")
		require.Contains(t, args, "--start")
		require.Contains(t, args, "2024-01-01 00:00:00")
		require.Contains(t, args, "--predicate")
		require.Contains(t, args, "subsystem == 'com.apple.systempreferences'")
		// Should NOT contain --style when format is default
		require.NotContains(t, args, "--style")
	})

	t.Run("with json format", func(t *testing.T) {
		receiver := &unifiedLoggingReceiver{
			config: &Config{
				Format: "json",
			},
		}

		args := receiver.buildLogCommandArgs("")
		require.Contains(t, args, "--style")
		require.Contains(t, args, "json")
	})
}

func TestProcessLogLine(t *testing.T) {
	t.Run("default format - sends unparsed line", func(t *testing.T) {
		sink := &consumertest.LogsSink{}
		receiver := &unifiedLoggingReceiver{
			config: &Config{
				Format: "default",
			},
			consumer: sink,
			logger:   zap.NewNop(),
		}

		rawLine := []byte("2024-01-01 12:00:00.123456-0700  localhost kernel[0]: (AppleACPIPlatform) AppleACPICPU: ProcessorId=0 LocalApicId=0 Enabled")
		err := receiver.processLogLine(context.Background(), rawLine)
		require.NoError(t, err)

		// Verify the log was consumed
		require.Len(t, sink.AllLogs(), 1)
		logs := sink.AllLogs()[0]
		require.Equal(t, 1, logs.LogRecordCount())

		// Verify the log record contains the raw line as string body
		logRecord := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
		require.Equal(t, string(rawLine), logRecord.Body().Str())

		// In default format, timestamp should only be observed (not parsed)
		require.NotZero(t, logRecord.ObservedTimestamp())
		require.Zero(t, logRecord.Timestamp())

		// In default format, severity should not be set
		require.Equal(t, "", logRecord.SeverityText())
		require.Equal(t, plog.SeverityNumberUnspecified, logRecord.SeverityNumber())
	})

	t.Run("ndjson format - parses timestamp and severity", func(t *testing.T) {
		sink := &consumertest.LogsSink{}
		receiver := &unifiedLoggingReceiver{
			config: &Config{
				Format: "ndjson",
			},
			consumer: sink,
			logger:   zap.NewNop(),
		}

		jsonLine := []byte(`{"timestamp":"2024-01-01 12:00:00.123456-0700","eventMessage":"Test message","messageType":"Error","subsystem":"com.test"}`)
		err := receiver.processLogLine(context.Background(), jsonLine)
		require.NoError(t, err)

		// Verify the log was consumed
		require.Len(t, sink.AllLogs(), 1)
		logs := sink.AllLogs()[0]
		require.Equal(t, 1, logs.LogRecordCount())

		// Verify the log record contains the entire JSON as body
		logRecord := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
		require.Equal(t, string(jsonLine), logRecord.Body().Str())

		// Verify timestamp was parsed from JSON
		require.NotZero(t, logRecord.Timestamp())
		expectedTime, _ := time.Parse("2006-01-02 15:04:05.000000-0700", "2024-01-01 12:00:00.123456-0700")
		require.Equal(t, expectedTime.UnixNano(), logRecord.Timestamp().AsTime().UnixNano())

		// Verify severity was parsed from JSON
		require.Equal(t, "Error", logRecord.SeverityText())
		require.Equal(t, plog.SeverityNumberError, logRecord.SeverityNumber())
	})

	t.Run("ndjson format - handles invalid json gracefully", func(t *testing.T) {
		sink := &consumertest.LogsSink{}
		receiver := &unifiedLoggingReceiver{
			config: &Config{
				Format: "ndjson",
			},
			consumer: sink,
			logger:   zap.NewNop(),
		}

		invalidJSON := []byte(`{invalid json}`)
		err := receiver.processLogLine(context.Background(), invalidJSON)
		require.NoError(t, err)

		// Verify the log was still consumed (with just the body)
		require.Len(t, sink.AllLogs(), 1)
		logs := sink.AllLogs()[0]
		require.Equal(t, 1, logs.LogRecordCount())

		// Verify the log record contains the invalid JSON as body
		logRecord := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
		require.Equal(t, string(invalidJSON), logRecord.Body().Str())

		// Timestamp should only be observed (not parsed from invalid JSON)
		require.NotZero(t, logRecord.ObservedTimestamp())
		require.Zero(t, logRecord.Timestamp())
	})

	t.Run("json format - parses timestamp and severity", func(t *testing.T) {
		sink := &consumertest.LogsSink{}
		receiver := &unifiedLoggingReceiver{
			config: &Config{
				Format: "json",
			},
			consumer: sink,
			logger:   zap.NewNop(),
		}

		jsonLine := []byte(`{"timestamp":"2024-01-01 12:00:00.123456-0700","eventMessage":"Test message","messageType":"Debug","subsystem":"com.test"}`)
		err := receiver.processLogLine(context.Background(), jsonLine)
		require.NoError(t, err)

		// Verify the log was consumed
		require.Len(t, sink.AllLogs(), 1)
		logs := sink.AllLogs()[0]
		require.Equal(t, 1, logs.LogRecordCount())

		// Verify the log record contains the entire JSON as body
		logRecord := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
		require.Equal(t, string(jsonLine), logRecord.Body().Str())

		// Verify timestamp was parsed from JSON
		require.NotZero(t, logRecord.Timestamp())

		// Verify severity was parsed from JSON
		require.Equal(t, "Debug", logRecord.SeverityText())
		require.Equal(t, plog.SeverityNumberDebug, logRecord.SeverityNumber())
	})

	t.Run("ndjson format - handles json without timestamp or severity", func(t *testing.T) {
		sink := &consumertest.LogsSink{}
		receiver := &unifiedLoggingReceiver{
			config: &Config{
				Format: "ndjson",
			},
			consumer: sink,
			logger:   zap.NewNop(),
		}

		jsonLine := []byte(`{"eventMessage":"Test message","subsystem":"com.test"}`)
		err := receiver.processLogLine(context.Background(), jsonLine)
		require.NoError(t, err)

		// Verify the log was consumed
		require.Len(t, sink.AllLogs(), 1)
		logs := sink.AllLogs()[0]
		require.Equal(t, 1, logs.LogRecordCount())

		// Verify the log record contains the JSON as body
		logRecord := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
		require.Equal(t, string(jsonLine), logRecord.Body().Str())

		// Timestamp should only be observed (no timestamp in JSON)
		require.NotZero(t, logRecord.ObservedTimestamp())
		require.Zero(t, logRecord.Timestamp())

		// Severity should not be set (no messageType in JSON)
		require.Equal(t, "", logRecord.SeverityText())
		require.Equal(t, plog.SeverityNumberUnspecified, logRecord.SeverityNumber())
	})
}

func TestMapMessageTypeToSeverity(t *testing.T) {
	tests := []struct {
		name     string
		msgType  string
		expected plog.SeverityNumber
	}{
		{
			name:     "Error message type",
			msgType:  "Error",
			expected: plog.SeverityNumberError,
		},
		{
			name:     "Fault message type",
			msgType:  "Fault",
			expected: plog.SeverityNumberFatal,
		},
		{
			name:     "Default message type",
			msgType:  "Default",
			expected: plog.SeverityNumberInfo,
		},
		{
			name:     "Info message type",
			msgType:  "Info",
			expected: plog.SeverityNumberInfo,
		},
		{
			name:     "Debug message type",
			msgType:  "Debug",
			expected: plog.SeverityNumberDebug,
		},
		{
			name:     "Unknown message type",
			msgType:  "Unknown",
			expected: plog.SeverityNumberUnspecified,
		},
		{
			name:     "Empty message type",
			msgType:  "",
			expected: plog.SeverityNumberUnspecified,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mapMessageTypeToSeverity(tt.msgType)
			if result != tt.expected {
				t.Errorf("mapMessageTypeToSeverity(%q) = %v, want %v", tt.msgType, result, tt.expected)
			}
		})
	}
}

func TestIsCompletionLine(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		expected bool
	}{
		{
			name:     "JSON completion format",
			line:     `{"count":540659,"finished":1}`,
			expected: true,
		},
		{
			name:     "JSON completion format with whitespace",
			line:     `  {"count":100,"finished":1}  `,
			expected: true,
		},
		{
			name:     "completion line with asterisks",
			line:     "** Processed 574 entries, done. **",
			expected: true,
		},
		{
			name:     "completion line with whitespace",
			line:     "  ** Finished processing **  ",
			expected: true,
		},
		{
			name:     "completion line with Processed and entries",
			line:     "Processed 100 entries successfully",
			expected: true,
		},
		{
			name:     "completion line with Processed and done",
			line:     "Processed all logs, done",
			expected: true,
		},
		{
			name:     "normal log line",
			line:     "2024-01-01 12:00:00.123456-0700  localhost kernel[0]: System initialized",
			expected: false,
		},
		{
			name:     "log line containing Processed word only",
			line:     "2024-01-01 12:00:00.123456-0700  localhost app[123]: Processed user request",
			expected: false,
		},
		{
			name:     "JSON without count and finished",
			line:     `{"timestamp":"2024-01-01","message":"test"}`,
			expected: false,
		},
		{
			name:     "empty line",
			line:     "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isCompletionLine([]byte(tt.line))
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestReadFromMultipleArchives(t *testing.T) {
	// Create test archive directories
	testdataDir := filepath.Join(".", "testdata", "multi_archive_test")
	archive1 := filepath.Join(testdataDir, "archive1.logarchive")
	archive2 := filepath.Join(testdataDir, "archive2.logarchive")
	archive3 := filepath.Join(testdataDir, "logs", "archive3.logarchive")

	_ = os.MkdirAll(archive1, 0755)
	_ = os.MkdirAll(archive2, 0755)
	_ = os.MkdirAll(archive3, 0755)

	defer func() {
		_ = os.RemoveAll(testdataDir)
	}()

	t.Run("receiver processes multiple archives with glob pattern", func(t *testing.T) {
		// Create config with glob pattern
		cfg := &Config{
			ArchivePath: filepath.Join(testdataDir, "*.logarchive"),
			Format:      "ndjson",
		}

		// Validate config to resolve glob paths
		err := cfg.Validate()
		require.NoError(t, err)

		// Verify that multiple archives were resolved
		resolvedPaths := cfg.getResolvedArchivePaths()
		require.Len(t, resolvedPaths, 2, "Should resolve 2 archives matching *.logarchive pattern")
		require.Contains(t, resolvedPaths, archive1)
		require.Contains(t, resolvedPaths, archive2)
		require.NotContains(t, resolvedPaths, archive3, "Should not include archive3 (in subdirectory)")

		// Create receiver
		sink := &consumertest.LogsSink{}
		receiver := newReceiver(cfg, zap.NewNop(), sink)
		require.NotNil(t, receiver)
		require.Equal(t, 2, len(receiver.config.getResolvedArchivePaths()))
	})

	t.Run("receiver processes multiple archives with doublestar glob pattern", func(t *testing.T) {
		// Create config with doublestar glob pattern
		cfg := &Config{
			ArchivePath: filepath.Join(testdataDir, "**", "*.logarchive"),
			Format:      "ndjson",
		}

		// Validate config to resolve glob paths
		err := cfg.Validate()
		require.NoError(t, err)

		// Verify that all archives were resolved (including subdirectories)
		resolvedPaths := cfg.getResolvedArchivePaths()
		require.Len(t, resolvedPaths, 3, "Should resolve all 3 archives with ** pattern")
		require.Contains(t, resolvedPaths, archive1)
		require.Contains(t, resolvedPaths, archive2)
		require.Contains(t, resolvedPaths, archive3)

		// Create receiver
		sink := &consumertest.LogsSink{}
		receiver := newReceiver(cfg, zap.NewNop(), sink)
		require.NotNil(t, receiver)
		require.Equal(t, 3, len(receiver.config.getResolvedArchivePaths()))
	})

	t.Run("receiver handles single archive path", func(t *testing.T) {
		// Create config with direct path (no glob)
		cfg := &Config{
			ArchivePath: archive1,
			Format:      "ndjson",
		}

		// Validate config
		err := cfg.Validate()
		require.NoError(t, err)

		// Verify that single archive was resolved
		resolvedPaths := cfg.getResolvedArchivePaths()
		require.Len(t, resolvedPaths, 1, "Should resolve to single archive")
		require.Contains(t, resolvedPaths, archive1)

		// Create receiver
		sink := &consumertest.LogsSink{}
		receiver := newReceiver(cfg, zap.NewNop(), sink)
		require.NotNil(t, receiver)
		require.Equal(t, 1, len(receiver.config.getResolvedArchivePaths()))
	})
}
