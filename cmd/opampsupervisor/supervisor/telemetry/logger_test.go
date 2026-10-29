// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zapcore"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/config"
)

func TestNewLogger_EncodingConsole(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "opamp-logger-test") //nolint:usetesting // Zap logger holds file locks on Windows, preventing t.TempDir cleanup
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	logFile := filepath.Join(tmpDir, "console.log")

	logger, err := NewLogger(config.Logs{
		Level:       zapcore.InfoLevel,
		OutputPaths: []string{logFile},
		Encoding:    "console",
	})
	require.NoError(t, err)
	defer func() { _ = logger.Sync() }()

	logger.Info("console message")
	require.NoError(t, logger.Sync())

	output, err := os.ReadFile(logFile)
	require.NoError(t, err)
	logLine := strings.TrimSpace(string(output))
	require.Contains(t, logLine, "console message")
	require.NotContains(t, logLine, `"msg":"console message"`)
	require.False(t, strings.HasPrefix(logLine, "{"))
}

func TestNewLogger_DefaultEncodingJSON(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "opamp-logger-test") //nolint:usetesting // Zap logger holds file locks on Windows, preventing t.TempDir cleanup
	require.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	logFile := filepath.Join(tmpDir, "json.log")

	logger, err := NewLogger(config.Logs{
		Level:       zapcore.InfoLevel,
		OutputPaths: []string{logFile},
	})
	require.NoError(t, err)
	defer func() { _ = logger.Sync() }()

	logger.Info("json message")
	require.NoError(t, logger.Sync())

	output, err := os.ReadFile(logFile)
	require.NoError(t, err)
	logLine := strings.TrimSpace(string(output))
	require.Contains(t, logLine, `"msg":"json message"`)
	require.Contains(t, logLine, `"level":"info"`)
	require.True(t, strings.HasPrefix(logLine, "{"))
}

func TestNewLogger_InvalidEncoding(t *testing.T) {
	logger, err := NewLogger(config.Logs{
		Level:    zapcore.InfoLevel,
		Encoding: "random-string",
	})
	require.Error(t, err)
	require.ErrorContains(t, err, `unsupported log encoding "random-string"`)
	require.Nil(t, logger)
}
