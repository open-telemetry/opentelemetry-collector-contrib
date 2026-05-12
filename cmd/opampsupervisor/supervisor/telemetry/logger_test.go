package telemetry

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/config"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zapcore"
)

func TestNewLogger_LogFormatConsole(t *testing.T) {
	logFile := filepath.Join(t.TempDir(), "console.log")

	logger, err := NewLogger(config.Logs{
		Level:       zapcore.InfoLevel,
		OutputPaths: []string{logFile},
		LogFormat:   "console",
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

func TestNewLogger_DefaultLogFormatJSON(t *testing.T) {
	logFile := filepath.Join(t.TempDir(), "json.log")

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
