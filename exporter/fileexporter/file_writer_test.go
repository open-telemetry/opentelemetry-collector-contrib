// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fileexporter

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

type mockFlushableWriteCloser struct {
	mu         sync.Mutex
	flushErr   error
	flushCalls int
}

func (m *mockFlushableWriteCloser) Write(p []byte) (int, error) {
	return len(p), nil
}

func (m *mockFlushableWriteCloser) Close() error {
	return nil
}

func (m *mockFlushableWriteCloser) flush() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.flushCalls++
	return m.flushErr
}

func (m *mockFlushableWriteCloser) setFlushErr(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.flushErr = err
}

func (m *mockFlushableWriteCloser) getFlushCalls() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.flushCalls
}

func TestFileWriter_FlushErrorLoggedOnce(t *testing.T) {
	core, logs := observer.New(zap.ErrorLevel)
	logger := zap.New(core)

	mockFile := &mockFlushableWriteCloser{flushErr: errors.New("disk full")}

	w := &fileWriter{
		path:          "/tmp/test.log",
		file:          mockFile,
		flushInterval: 10 * time.Millisecond,
		logger:        logger,
	}

	w.start()

	// Wait for several ticks to occur
	require.Eventually(t, func() bool {
		return mockFile.getFlushCalls() >= 3
	}, 1*time.Second, 10*time.Millisecond)

	require.NoError(t, w.shutdown())

	// Assert only ONE error was logged
	assert.Equal(t, 1, logs.Len(), "Flush error should only be logged once")
	logRecord := logs.All()[0]
	assert.Equal(t, "Failed to flush file", logRecord.Message)
	assert.Equal(t, zap.ErrorLevel, logRecord.Level)

	fields := logRecord.ContextMap()
	assert.Equal(t, "/tmp/test.log", fields["path"])
	assert.Equal(t, "disk full", fields["error"])
}

func TestFileWriter_FlushRecovery(t *testing.T) {
	core, logs := observer.New(zap.ErrorLevel)
	logger := zap.New(core)

	mockFile := &mockFlushableWriteCloser{flushErr: errors.New("initial failure")}

	w := &fileWriter{
		path:          "/tmp/recovery.log",
		file:          mockFile,
		flushInterval: 10 * time.Millisecond,
		logger:        logger,
	}

	w.start()

	// Wait for initial failure to be logged
	require.Eventually(t, func() bool {
		return logs.Len() == 1
	}, 1*time.Second, 10*time.Millisecond)

	// Now simulate recovery: flush succeeds
	mockFile.setFlushErr(nil)

	// Wait for successful flush to happen
	callsBeforeRecovery := mockFile.getFlushCalls()
	require.Eventually(t, func() bool {
		return mockFile.getFlushCalls() > callsBeforeRecovery+2
	}, 1*time.Second, 10*time.Millisecond)

	// Still only 1 error logged
	assert.Equal(t, 1, logs.Len())

	// Now fail again: it should log a second time
	mockFile.setFlushErr(errors.New("second failure"))

	require.Eventually(t, func() bool {
		return logs.Len() == 2
	}, 1*time.Second, 10*time.Millisecond)

	require.NoError(t, w.shutdown())

	assert.Equal(t, 2, logs.Len(), "Should log again after recovery")
	assert.Equal(t, "second failure", logs.All()[1].ContextMap()["error"])
}

func TestFileWriter_NilLoggerSafe(t *testing.T) {
	mockFile := &mockFlushableWriteCloser{flushErr: errors.New("disk full")}

	w := &fileWriter{
		path:          "/tmp/nillogger.log",
		file:          mockFile,
		flushInterval: 10 * time.Millisecond,
		logger:        nil, // explicitly nil
	}

	assert.NotPanics(t, func() {
		w.start()
		require.Eventually(t, func() bool {
			return mockFile.getFlushCalls() >= 2
		}, 1*time.Second, 10*time.Millisecond)
		require.NoError(t, w.shutdown())
	})
}
