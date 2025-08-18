// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package macosunifiedloggingencodingextension

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
)

func TestUnmarshalLogs_RealTracev3Data(t *testing.T) {
	// Skip if test data doesn't exist
	testDataPath := "../../../../opentelemetry-collector-contrib/local/macOSUnifiedLogging/testdata/test_sample.tracev3"
	absPath, err := filepath.Abs(testDataPath)
	require.NoError(t, err)

	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		t.Skip("Test data not available, skipping integration test")
	}

	// Read the first 1024 bytes of real tracev3 data
	data, err := os.ReadFile(absPath)
	require.NoError(t, err)

	// Limit to first 1024 bytes for this basic test
	if len(data) > 1024 {
		data = data[:1024]
	}

	t.Logf("Read %d bytes of real tracev3 data", len(data))

	// Create extension with debug mode
	ext := &MacosUnifiedLoggingExtension{
		config: &Config{DebugMode: true},
	}

	err = ext.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer ext.Shutdown(context.Background())

	// Unmarshal the logs
	logs, err := ext.UnmarshalLogs(data)
	require.NoError(t, err)

	// Verify we get log records - now expecting multiple individual entries
	assert.GreaterOrEqual(t, logs.ResourceLogs().Len(), 1, "Should have at least one resource log")

	// Check the first resource log
	resourceLogs := logs.ResourceLogs().At(0)
	assert.Equal(t, 1, resourceLogs.ScopeLogs().Len())

	scopeLogs := resourceLogs.ScopeLogs().At(0)
	logRecordCount := scopeLogs.LogRecords().Len()
	assert.GreaterOrEqual(t, logRecordCount, 1, "Should have at least one log record")

	t.Logf("Successfully parsed %d individual log entries from tracev3 data", logRecordCount)

	// Check the first log record (header info)
	firstLogRecord := scopeLogs.LogRecords().At(0)
	firstMessage := firstLogRecord.Body().AsString()

	// Should contain header information
	assert.Contains(t, firstMessage, "TraceV3 Header", "First entry should be header info")
	assert.Contains(t, firstMessage, "chunk_tag=", "Should contain chunk tag info")

	// Check attributes of first record
	attrs := firstLogRecord.Attributes()
	source, exists := attrs.Get("source")
	assert.True(t, exists)
	assert.Equal(t, "macos_unified_logging", source.AsString())

	// If we have multiple log records, check that we have individual entries
	if logRecordCount > 1 {
		// Check a subsequent log record for individual entry characteristics
		for i := 1; i < logRecordCount; i++ {
			logRecord := scopeLogs.LogRecords().At(i)
			message := logRecord.Body().AsString()

			// Individual entries should have different characteristics
			attrs := logRecord.Attributes()

			// Should have chunk type
			chunkType, exists := attrs.Get("chunk.type")
			assert.True(t, exists, "Individual entries should have chunk type")
			t.Logf("Entry %d: chunk_type=%s, message=%q", i, chunkType.AsString(), message)

			// Should be marked as decoded
			decoded, exists := attrs.Get("decoded")
			assert.True(t, exists)
			assert.True(t, decoded.Bool(), "Individual entries should be marked as decoded")
		}
	}

	t.Logf("Successfully processed real tracev3 data with %d individual log entries", logRecordCount)
}
