// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filelogreceiver

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage/storagetest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/filelogreceiver/internal/metadata"
)

func TestStorage(t *testing.T) {
	t.Parallel()

	const baseLog = "This is a simple log line with the number %3d"

	ctx := context.Background()

	logsDir := t.TempDir()
	storageDir := t.TempDir()
	extID := storagetest.NewFileBackedStorageExtension("test", storageDir).ID

	f := NewFactory()

	cfg := rotationTestConfig(logsDir)
	cfg.Operators = nil // not testing processing, just read the lines
	cfg.StorageID = &extID

	logger := newRecallLogger(t, logsDir)

	ext := storagetest.NewFileBackedStorageExtension("test", storageDir)
	host := storagetest.NewStorageHost().WithExtension(ext.ID, ext)
	sink := new(consumertest.LogsSink)
	set := receivertest.NewNopSettings(metadata.Type)
	rcvr, err := f.CreateLogs(ctx, set, cfg, sink)
	require.NoError(t, err, "failed to create receiver")
	require.NoError(t, rcvr.Start(ctx, host))

	// Write 2 logs
	logger.log(fmt.Sprintf(baseLog, 0))
	logger.log(fmt.Sprintf(baseLog, 1))

	// Expect them now, since the receiver is running
	require.Eventually(t,
		expectLogs(sink, logger.recall()),
		5*time.Second,
		10*time.Millisecond,
		"expected 2 but got %d logs",
		sink.LogRecordCount(),
	)

	// Shut down the components
	require.NoError(t, rcvr.Shutdown(ctx))
	for _, e := range host.GetExtensions() {
		require.NoError(t, e.Shutdown(ctx))
	}

	// Write 3 more logs while the collector is not running
	logger.log(fmt.Sprintf(baseLog, 2))
	logger.log(fmt.Sprintf(baseLog, 3))
	logger.log(fmt.Sprintf(baseLog, 4))

	// Start the components again
	ext = storagetest.NewFileBackedStorageExtension("test", storageDir)
	host = storagetest.NewStorageHost().WithExtension(ext.ID, ext)
	rcvr, err = f.CreateLogs(ctx, set, cfg, sink)
	require.NoError(t, err, "failed to create receiver")
	require.NoError(t, rcvr.Start(ctx, host))
	sink.Reset()

	// Expect only the new 3
	require.Eventually(t,
		expectLogs(sink, logger.recall()),
		time.Second,
		10*time.Millisecond,
		"expected 3 but got %d logs",
		sink.LogRecordCount(),
	)
	sink.Reset()

	// Write 100 more, to ensure we're past the fingerprint size
	for i := 100; i < 200; i++ {
		logger.log(fmt.Sprintf(baseLog, i))
	}

	// Expect the new 100
	require.Eventually(t,
		expectLogs(sink, logger.recall()),
		time.Second,
		10*time.Millisecond,
		"expected 100 but got %d logs",
		sink.LogRecordCount(),
	)

	// Shut down the components
	require.NoError(t, rcvr.Shutdown(ctx))
	for _, e := range host.GetExtensions() {
		require.NoError(t, e.Shutdown(ctx))
	}

	// Write 5 more logs while the collector is not running
	logger.log(fmt.Sprintf(baseLog, 5))
	logger.log(fmt.Sprintf(baseLog, 6))
	logger.log(fmt.Sprintf(baseLog, 7))
	logger.log(fmt.Sprintf(baseLog, 8))
	logger.log(fmt.Sprintf(baseLog, 9))

	// Start the components again
	ext = storagetest.NewFileBackedStorageExtension("test", storageDir)
	host = storagetest.NewStorageHost().WithExtension(ext.ID, ext)
	rcvr, err = f.CreateLogs(ctx, set, cfg, sink)
	require.NoError(t, err, "failed to create receiver")
	require.NoError(t, rcvr.Start(ctx, host))
	sink.Reset()

	// Expect only the new 5
	require.Eventually(t,
		expectLogs(sink, logger.recall()),
		time.Second,
		10*time.Millisecond,
		"expected 5 but got %d logs",
		sink.LogRecordCount(),
	)

	// Shut down the components
	require.NoError(t, rcvr.Shutdown(ctx))
	for _, e := range host.GetExtensions() {
		require.NoError(t, e.Shutdown(ctx))
	}
	require.NoError(t, logger.close())
}

type recallLogger struct {
	logFile *os.File
	*log.Logger
	written []string
}

func newRecallLogger(t *testing.T, tempDir string) *recallLogger {
	path := filepath.Join(tempDir, "test.log")
	logFile, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	require.NoError(t, err)

	return &recallLogger{
		logFile: logFile,
		Logger:  log.New(logFile, "", 0),
		written: []string{},
	}
}

func (l *recallLogger) log(s string) {
	l.written = append(l.written, s)
	l.Println(s)
}

func (l *recallLogger) recall() []string {
	defer func() { l.written = []string{} }()
	return l.written
}

func (l *recallLogger) close() error {
	return l.logFile.Close()
}

func expectLogs(sink *consumertest.LogsSink, expected []string) func() bool {
	return func() bool {
		if sink.LogRecordCount() != len(expected) {
			return false
		}

		found := make(map[string]bool)
		for _, e := range expected {
			found[e] = false
		}

		for _, logs := range sink.AllLogs() {
			rl := logs.ResourceLogs()
			for i := 0; i < rl.Len(); i++ {
				sl := rl.At(i).ScopeLogs()
				for j := 0; j < sl.Len(); j++ {
					lrs := sl.At(j).LogRecords()
					for k := 0; k < lrs.Len(); k++ {
						body := lrs.At(k).Body().Str()
						found[body] = true
					}
				}
			}
		}

		for _, v := range found {
			if !v {
				return false
			}
		}

		return true
	}
}
