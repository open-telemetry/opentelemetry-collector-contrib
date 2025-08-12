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

	// Verify we get a log record
	assert.Equal(t, 1, logs.ResourceLogs().Len())

	resourceLogs := logs.ResourceLogs().At(0)
	assert.Equal(t, 1, resourceLogs.ScopeLogs().Len())

	scopeLogs := resourceLogs.ScopeLogs().At(0)
	assert.Equal(t, 1, scopeLogs.LogRecords().Len())

	logRecord := scopeLogs.LogRecords().At(0)
	message := logRecord.Body().AsString()

	// Should contain information about the binary data read
	assert.Contains(t, message, "macOS Unified Logging binary data")
	assert.Contains(t, message, "1024 bytes")

	// Check attributes
	attrs := logRecord.Attributes()
	source, exists := attrs.Get("source")
	assert.True(t, exists)
	assert.Equal(t, "macos_unified_logging", source.AsString())

	dataSize, exists := attrs.Get("data_size")
	assert.True(t, exists)
	assert.Equal(t, int64(1024), dataSize.Int())

	decoded, exists := attrs.Get("decoded")
	assert.True(t, exists)
	assert.False(t, decoded.Bool()) // Should be false since we're not decoding yet

	t.Logf("Successfully processed real tracev3 data: %s", message)
}
