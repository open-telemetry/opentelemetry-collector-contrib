// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filelogreceiver

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/filelogreceiver/internal/metadata"
)

// TestReceiverLifecycle tests the complete lifecycle of the receiver
func TestReceiverLifecycle(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	factory := NewFactory()
	cfg := createDefaultConfig()

	// Create a temporary log file
	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "test.log")
	cfg.InputConfig.Include = []string{logFile}
	cfg.InputConfig.StartAt = "beginning"

	sink := new(consumertest.LogsSink)

	// Create receiver
	receiver, err := factory.CreateLogs(ctx, receivertest.NewNopSettings(metadata.Type), cfg, sink)
	require.NoError(t, err)
	require.NotNil(t, receiver)

	// Start receiver
	err = receiver.Start(ctx, componenttest.NewNopHost())
	require.NoError(t, err)

	// Write logs while receiver is running
	f, err := os.Create(logFile)
	require.NoError(t, err)
	_, err = f.WriteString("log line 1\n")
	require.NoError(t, err)
	_, err = f.WriteString("log line 2\n")
	require.NoError(t, err)
	require.NoError(t, f.Close())

	// Wait for logs to be consumed
	require.Eventually(t, func() bool {
		return sink.LogRecordCount() == 2
	}, 2*time.Second, 10*time.Millisecond)

	// Shutdown receiver
	err = receiver.Shutdown(ctx)
	require.NoError(t, err)

	// Verify logs were received
	assert.Equal(t, 2, sink.LogRecordCount())
}

// TestReceiverLifecycleWithContextCancellation tests receiver behavior when context is cancelled
func TestReceiverLifecycleWithContextCancellation(t *testing.T) {
	t.Parallel()

	factory := NewFactory()
	cfg := createDefaultConfig()

	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "test.log")
	cfg.InputConfig.Include = []string{logFile}
	cfg.InputConfig.StartAt = "beginning"

	// Create test file
	require.NoError(t, os.WriteFile(logFile, []byte("test log\n"), 0o600))

	sink := new(consumertest.LogsSink)

	// Create context that we'll cancel
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	receiver, err := factory.CreateLogs(ctx, receivertest.NewNopSettings(metadata.Type), cfg, sink)
	require.NoError(t, err)

	err = receiver.Start(ctx, componenttest.NewNopHost())
	require.NoError(t, err)

	// Cancel context
	cancel()

	// Shutdown should still work
	err = receiver.Shutdown(t.Context())
	require.NoError(t, err)
}

// TestReceiverMultipleStartStop tests multiple start/stop cycles
func TestReceiverMultipleStartStop(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	factory := NewFactory()
	cfg := createDefaultConfig()

	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "test.log")
	cfg.InputConfig.Include = []string{logFile}
	cfg.InputConfig.StartAt = "beginning"
	cfg.InputConfig.PollInterval = 10 * time.Millisecond

	sink := new(consumertest.LogsSink)

	for i := range 3 {
		t.Run(fmt.Sprintf("cycle_%d", i), func(t *testing.T) {
			receiver, err := factory.CreateLogs(ctx, receivertest.NewNopSettings(metadata.Type), cfg, sink)
			require.NoError(t, err)

			err = receiver.Start(ctx, componenttest.NewNopHost())
			require.NoError(t, err)

			// Write a log
			f, err := os.OpenFile(logFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
			require.NoError(t, err)
			_, err = fmt.Fprintf(f, "log cycle %d\n", i)
			require.NoError(t, err)
			require.NoError(t, f.Close())

			// Give it time to read
			time.Sleep(50 * time.Millisecond)

			err = receiver.Shutdown(ctx)
			require.NoError(t, err)
		})
	}

	// Verify we got logs from all cycles
	assert.Positive(t, sink.LogRecordCount())
}

// TestReceiverShutdownWithoutStart tests that shutdown works even if start wasn't called
func TestReceiverShutdownWithoutStart(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	factory := NewFactory()
	cfg := createDefaultConfig()

	tempDir := t.TempDir()
	cfg.InputConfig.Include = []string{filepath.Join(tempDir, "test.log")}

	sink := new(consumertest.LogsSink)

	receiver, err := factory.CreateLogs(ctx, receivertest.NewNopSettings(metadata.Type), cfg, sink)
	require.NoError(t, err)

	// Shutdown without Start should not error
	err = receiver.Shutdown(ctx)
	require.NoError(t, err)
}

// TestReceiverWithEmptyIncludeList tests receiver with empty include list
func TestReceiverWithEmptyIncludeList(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	factory := NewFactory()
	cfg := createDefaultConfig()

	// Empty include list should cause an error
	cfg.InputConfig.Include = []string{}

	sink := new(consumertest.LogsSink)

	_, err := factory.CreateLogs(ctx, receivertest.NewNopSettings(metadata.Type), cfg, sink)
	require.Error(t, err)
}

// TestReceiverInitializationWithValidConfig tests receiver initialization with various valid configs
func TestReceiverInitializationWithValidConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		setupCfg func(*FileLogConfig, string)
	}{
		{
			name: "with single file",
			setupCfg: func(cfg *FileLogConfig, dir string) {
				cfg.InputConfig.Include = []string{filepath.Join(dir, "test.log")}
				cfg.InputConfig.StartAt = "beginning"
			},
		},
		{
			name: "with wildcard pattern",
			setupCfg: func(cfg *FileLogConfig, dir string) {
				cfg.InputConfig.Include = []string{filepath.Join(dir, "*.log")}
				cfg.InputConfig.StartAt = "end"
			},
		},
		{
			name: "with exclude pattern",
			setupCfg: func(cfg *FileLogConfig, dir string) {
				cfg.InputConfig.Include = []string{filepath.Join(dir, "*.log")}
				cfg.InputConfig.Exclude = []string{filepath.Join(dir, "excluded.log")}
			},
		},
		{
			name: "with custom poll interval",
			setupCfg: func(cfg *FileLogConfig, dir string) {
				cfg.InputConfig.Include = []string{filepath.Join(dir, "test.log")}
				cfg.InputConfig.PollInterval = 100 * time.Millisecond
			},
		},
		{
			name: "with file metadata enabled",
			setupCfg: func(cfg *FileLogConfig, dir string) {
				cfg.InputConfig.Include = []string{filepath.Join(dir, "test.log")}
				cfg.InputConfig.IncludeFileName = true
				cfg.InputConfig.IncludeFilePath = true
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := t.Context()
			factory := NewFactory()
			cfg := createDefaultConfig()
			tempDir := t.TempDir()

			tt.setupCfg(cfg, tempDir)

			sink := new(consumertest.LogsSink)

			receiver, err := factory.CreateLogs(ctx, receivertest.NewNopSettings(metadata.Type), cfg, sink)
			require.NoError(t, err)
			require.NotNil(t, receiver)

			err = receiver.Start(ctx, componenttest.NewNopHost())
			require.NoError(t, err)

			err = receiver.Shutdown(ctx)
			require.NoError(t, err)
		})
	}
}
