// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage/storagetest"
)

func TestStorage(t *testing.T) {
	t.Parallel()

	const baseLog = "This is a simple log line with the number %3d"

	ctx := context.Background()

	logsDir := newTempDir(t)
	storageDir := newTempDir(t)

	f := NewFactory()

	cfg := testdataRotateTestYamlAsMap(logsDir)
	cfg.Converter.MaxFlushCount = 1
	cfg.Converter.FlushInterval = time.Millisecond
	cfg.Operators = nil // not testing processing, just read the lines

	logger := newRecallLogger(t, logsDir)

	host := storagetest.NewStorageHost(t, storageDir, "test")
	sink := new(consumertest.LogsSink)
	rcvr, err := f.CreateLogsReceiver(ctx, componenttest.NewNopReceiverCreateSettings(), cfg, sink)
	require.NoError(t, err, "failed to create receiver")
	require.NoError(t, rcvr.Start(ctx, host))

	// Write 2 logs
	logger.log(fmt.Sprintf(baseLog, 0))
	logger.log(fmt.Sprintf(baseLog, 1))

	// Expect them now, since the receiver is running
	require.Eventually(t,
		expectLogs(sink, logger.recall()),
		time.Second,
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
	host = storagetest.NewStorageHost(t, storageDir, "test")
	rcvr, err = f.CreateLogsReceiver(ctx, componenttest.NewNopReceiverCreateSettings(), cfg, sink)
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
	host = storagetest.NewStorageHost(t, storageDir, "test")
	rcvr, err = f.CreateLogsReceiver(ctx, componenttest.NewNopReceiverCreateSettings(), cfg, sink)
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
}

type recallLogger struct {
	*log.Logger
	written []string
}

func newRecallLogger(t *testing.T, tempDir string) *recallLogger {
	path := filepath.Join(tempDir, "test.log")
	logFile, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	require.NoError(t, err)

	return &recallLogger{
		Logger:  log.New(logFile, "", 0),
		written: []string{},
	}
}

func (l *recallLogger) log(s string) {
	l.written = append(l.written, s)
	l.Logger.Println(s)
}

func (l *recallLogger) recall() []string {
	defer func() { l.written = []string{} }()
	return l.written
}

// TODO use stateless Convert() from #3125 to generate exact pdata.Logs
// for now, just validate body
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
			body := logs.ResourceLogs().
				At(0).InstrumentationLibraryLogs().
				At(0).Logs().
				At(0).Body().
				StringVal()

			found[body] = true
		}

		for _, v := range found {
			if !v {
				return false
			}
		}

		return true
	}
}
