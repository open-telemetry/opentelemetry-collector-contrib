// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package agentcomponents // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/agentcomponents"

import (
	"testing"

	tracelog "github.com/DataDog/datadog-agent/pkg/trace/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestZapLogger_Interface(_ *testing.T) {
	// Verify that ZapLogger implements tracelog.Logger interface
	var _ tracelog.Logger = &ZapLogger{}
}

func setupTestLogger() (*ZapLogger, *observer.ObservedLogs) {
	core, recorded := observer.New(zapcore.DebugLevel)
	logger := zap.New(core)
	zapLogger := &ZapLogger{Logger: logger}
	return zapLogger, recorded
}

func TestZapLogger_Trace(t *testing.T) {
	zapLogger, recorded := setupTestLogger()

	// Trace methods should be no-ops
	zapLogger.Trace("test message")
	zapLogger.Tracef("test %s", "formatted")

	// No logs should be recorded for trace methods
	assert.Equal(t, 0, recorded.Len())
}

func TestZapLogger_Debug(t *testing.T) {
	zapLogger, recorded := setupTestLogger()

	t.Run("Debug", func(t *testing.T) {
		zapLogger.Debug("debug message")

		require.Equal(t, 1, recorded.Len())
		entry := recorded.All()[0]
		assert.Equal(t, zapcore.DebugLevel, entry.Level)
		assert.Equal(t, "debug message", entry.Message)
	})

	t.Run("Debug multiple args", func(t *testing.T) {
		recorded.TakeAll() // Clear previous logs
		zapLogger.Debug("debug", " ", "message")

		require.Equal(t, 1, recorded.Len())
		entry := recorded.All()[0]
		assert.Equal(t, zapcore.DebugLevel, entry.Level)
		assert.Equal(t, "debug message", entry.Message)
	})

	t.Run("Debugf", func(t *testing.T) {
		recorded.TakeAll() // Clear previous logs
		zapLogger.Debugf("debug %s %d", "message", 123)

		require.Equal(t, 1, recorded.Len())
		entry := recorded.All()[0]
		assert.Equal(t, zapcore.DebugLevel, entry.Level)
		assert.Equal(t, "debug message 123", entry.Message)
	})
}

func TestZapLogger_Info(t *testing.T) {
	zapLogger, recorded := setupTestLogger()

	t.Run("Info", func(t *testing.T) {
		zapLogger.Info("info message")

		require.Equal(t, 1, recorded.Len())
		entry := recorded.All()[0]
		assert.Equal(t, zapcore.InfoLevel, entry.Level)
		assert.Equal(t, "info message", entry.Message)
	})

	t.Run("Infof", func(t *testing.T) {
		recorded.TakeAll() // Clear previous logs
		zapLogger.Infof("info %s %d", "message", 456)

		require.Equal(t, 1, recorded.Len())
		entry := recorded.All()[0]
		assert.Equal(t, zapcore.InfoLevel, entry.Level)
		assert.Equal(t, "info message 456", entry.Message)
	})
}

func TestZapLogger_Warn(t *testing.T) {
	zapLogger, recorded := setupTestLogger()

	t.Run("Warn", func(t *testing.T) {
		err := zapLogger.Warn("warn message")

		assert.NoError(t, err)
		require.Equal(t, 1, recorded.Len())
		entry := recorded.All()[0]
		assert.Equal(t, zapcore.WarnLevel, entry.Level)
		assert.Equal(t, "warn message", entry.Message)
	})

	t.Run("Warnf", func(t *testing.T) {
		recorded.TakeAll() // Clear previous logs
		err := zapLogger.Warnf("warn %s %d", "message", 789)

		assert.NoError(t, err)
		require.Equal(t, 1, recorded.Len())
		entry := recorded.All()[0]
		assert.Equal(t, zapcore.WarnLevel, entry.Level)
		assert.Equal(t, "warn message 789", entry.Message)
	})
}

func TestZapLogger_Error(t *testing.T) {
	zapLogger, recorded := setupTestLogger()

	t.Run("Error", func(t *testing.T) {
		err := zapLogger.Error("error message")

		assert.NoError(t, err)
		require.Equal(t, 1, recorded.Len())
		entry := recorded.All()[0]
		assert.Equal(t, zapcore.ErrorLevel, entry.Level)
		assert.Equal(t, "error message", entry.Message)
	})

	t.Run("Errorf", func(t *testing.T) {
		recorded.TakeAll() // Clear previous logs
		err := zapLogger.Errorf("error %s %d", "message", 101)

		assert.NoError(t, err)
		require.Equal(t, 1, recorded.Len())
		entry := recorded.All()[0]
		assert.Equal(t, zapcore.ErrorLevel, entry.Level)
		assert.Equal(t, "error message 101", entry.Message)
	})
}

func TestZapLogger_Critical(t *testing.T) {
	zapLogger, recorded := setupTestLogger()

	t.Run("Critical", func(t *testing.T) {
		err := zapLogger.Critical("critical message")

		assert.NoError(t, err)
		require.Equal(t, 1, recorded.Len())
		entry := recorded.All()[0]
		assert.Equal(t, zapcore.ErrorLevel, entry.Level)
		assert.Equal(t, "critical message", entry.Message)

		// Check for critical field
		criticalField := findField(entry.Context, "critical")
		require.NotNil(t, criticalField)
		assert.IsType(t, zapcore.BoolType, criticalField.Type)
		assert.Equal(t, int64(1), criticalField.Integer)
	})

	zapLogger, recorded = setupTestLogger()
	t.Run("Criticalf", func(t *testing.T) {
		err := zapLogger.Criticalf("critical %s %d", "message", 202)

		assert.NoError(t, err)
		require.Equal(t, 1, recorded.Len())
		entry := recorded.All()[0]
		assert.Equal(t, zapcore.ErrorLevel, entry.Level)
		assert.Equal(t, "critical message 202", entry.Message)

		// Check for critical field
		criticalField := findField(entry.Context, "critical")
		require.NotNil(t, criticalField)
		assert.IsType(t, zapcore.BoolType, criticalField.Type)
		assert.Equal(t, int64(1), criticalField.Integer)
	})
}

func TestZapLogger_Flush(t *testing.T) {
	// Use a real logger to test Flush functionality
	logger := zap.NewNop() // Use a no-op logger to avoid actual output
	zapLogger := &ZapLogger{Logger: logger}

	// This should not panic and should complete successfully
	assert.NotPanics(t, func() {
		zapLogger.Flush()
	})
}

func TestZapLogger_FlushWithBufferedLogger(t *testing.T) {
	// Create a buffered logger to test actual sync behavior
	config := zap.NewDevelopmentConfig()
	config.OutputPaths = []string{"stdout"}
	logger, err := config.Build()
	require.NoError(t, err)

	zapLogger := &ZapLogger{Logger: logger}

	// Add some logs
	zapLogger.Info("test message")

	// Flush should not panic
	assert.NotPanics(t, func() {
		zapLogger.Flush()
	})
}

// Helper function to find a field in the context
func findField(fields []zapcore.Field, key string) *zapcore.Field {
	for _, field := range fields {
		if field.Key == key {
			return &field
		}
	}
	return nil
}

// Benchmark tests
func BenchmarkZapLogger_Info(b *testing.B) {
	zapLogger, _ := setupTestLogger()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		zapLogger.Info("benchmark message")
	}
}

func BenchmarkZapLogger_Infof(b *testing.B) {
	zapLogger, _ := setupTestLogger()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		zapLogger.Infof("benchmark %s %d", "message", i)
	}
}

func BenchmarkZapLogger_Critical(b *testing.B) {
	zapLogger, _ := setupTestLogger()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = zapLogger.Critical("benchmark critical message")
	}
}
