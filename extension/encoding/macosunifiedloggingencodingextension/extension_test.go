// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package macosunifiedloggingencodingextension

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/extension/extensiontest"
)

func TestFactory(t *testing.T) {
	factory := NewFactory()
	assert.NotNil(t, factory)
	assert.Equal(t, "macos_unified_logging_encoding", factory.Type().String())
}

func TestDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	config, ok := cfg.(*Config)
	require.True(t, ok)
	assert.False(t, config.DebugMode)
}

func TestCreateExtension(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	extension, err := factory.Create(
		context.Background(),
		extensiontest.NewNopSettings(factory.Type()),
		cfg,
	)

	require.NoError(t, err)
	assert.NotNil(t, extension)

	// Start the extension
	err = extension.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	// Shutdown the extension
	err = extension.Shutdown(context.Background())
	require.NoError(t, err)
}

func TestUnmarshalLogs_EmptyData(t *testing.T) {
	ext := &MacosUnifiedLoggingExtension{
		config: &Config{DebugMode: false},
	}

	err := ext.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	// Test empty data
	logs, err := ext.UnmarshalLogs([]byte{})
	require.NoError(t, err)
	assert.Equal(t, 0, logs.ResourceLogs().Len())
}

func TestUnmarshalLogs_BinaryData(t *testing.T) {
	ext := &MacosUnifiedLoggingExtension{
		config: &Config{DebugMode: true},
	}

	err := ext.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	// Test with some binary data
	testData := []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F}

	logs, err := ext.UnmarshalLogs(testData)
	require.NoError(t, err)
	assert.Equal(t, 1, logs.ResourceLogs().Len())

	resourceLogs := logs.ResourceLogs().At(0)
	assert.Equal(t, 1, resourceLogs.ScopeLogs().Len())

	scopeLogs := resourceLogs.ScopeLogs().At(0)
	assert.Equal(t, 1, scopeLogs.LogRecords().Len())

	logRecord := scopeLogs.LogRecords().At(0)

	// Check the message contains information about the binary data
	message := logRecord.Body().AsString()
	assert.Contains(t, message, "Read 16 bytes of macOS Unified Logging binary data")
	assert.Contains(t, message, "000102030405060708090a0b0c0d0e0f")

	// Check attributes
	attrs := logRecord.Attributes()
	source, exists := attrs.Get("source")
	assert.True(t, exists)
	assert.Equal(t, "macos_unified_logging", source.AsString())

	dataSize, exists := attrs.Get("data_size")
	assert.True(t, exists)
	assert.Equal(t, int64(16), dataSize.Int())

	decoded, exists := attrs.Get("decoded")
	assert.True(t, exists)
	assert.False(t, decoded.Bool())
}

func TestUnmarshalLogs_DebugModeOff(t *testing.T) {
	ext := &MacosUnifiedLoggingExtension{
		config: &Config{DebugMode: false},
	}

	err := ext.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	// Test with some binary data but debug mode off
	testData := []byte{0x00, 0x01, 0x02, 0x03}

	logs, err := ext.UnmarshalLogs(testData)
	require.NoError(t, err)

	logRecord := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	message := logRecord.Body().AsString()

	// Should not contain hex dump when debug mode is off
	assert.Contains(t, message, "Read 4 bytes of macOS Unified Logging binary data")
	assert.NotContains(t, message, "00010203")
}
