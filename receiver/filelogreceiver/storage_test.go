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
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.uber.org/zap/zaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage/storagetest"
)

func TestStorage(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	logsDir := newTempDir(t)
	storageDir := newTempDir(t)

	f := NewFactory()
	params := component.ReceiverCreateParams{Logger: zaptest.NewLogger(t)}

	cfg := testdataRotateTestYamlAsMap(logsDir)
	cfg.Converter.MaxFlushCount = 1
	cfg.Converter.FlushInterval = time.Millisecond
	cfg.Operators = nil // not testing processing, just read the lines

	logger := newRotatingLogger(t, logsDir, 1000, 1, false, false)

	writeLog := func(nums ...int) {
		for _, i := range nums {
			logger.Println(fmt.Sprintf("This is a simple log line with the number %3d", i))
		}
	}

	host := storagetest.NewStorageHost(t, storageDir, "test")
	sink := new(consumertest.LogsSink)
	rcvr, err := f.CreateLogsReceiver(ctx, params, cfg, sink)
	require.NoError(t, err, "failed to create receiver")
	require.NoError(t, rcvr.Start(ctx, host))

	// Write 2 logs
	writeLog(0, 1)

	// Expect them now, since the receiver is running
	require.Eventually(t,
		expectNLogs(sink, 2),
		time.Second,
		10*time.Millisecond,
		"expected 2 but got %d logs",
		sink.LogRecordsCount(),
	)

	// Shut down the components
	require.NoError(t, rcvr.Shutdown(ctx))
	for _, e := range host.GetExtensions() {
		require.NoError(t, e.Shutdown(ctx))
	}

	// Write 3 more logs while the collector is not running
	writeLog(2, 3, 4)

	// Start the components again
	host = storagetest.NewStorageHost(t, storageDir, "test")
	rcvr, err = f.CreateLogsReceiver(ctx, params, cfg, sink)
	require.NoError(t, err, "failed to create receiver")
	require.NoError(t, rcvr.Start(ctx, host))
	sink.Reset()

	// Expect only the new 3
	require.Eventually(t,
		expectNLogs(sink, 3),
		time.Second,
		10*time.Millisecond,
		"expected 3 but got %d logs",
		sink.LogRecordsCount(),
	)
	sink.Reset()

	// Write 100 more, to ensure we're past the fingerprint size
	for i := 100; i < 200; i++ {
		writeLog(i)
	}

	// Expect the new 100
	require.Eventually(t,
		expectNLogs(sink, 100),
		time.Second,
		10*time.Millisecond,
		"expected 100 but got %d logs",
		sink.LogRecordsCount(),
	)

	// Shut down the components
	require.NoError(t, rcvr.Shutdown(ctx))
	for _, e := range host.GetExtensions() {
		require.NoError(t, e.Shutdown(ctx))
	}

	// Write 5 more logs while the collector is not running
	writeLog(5, 6, 7, 8, 9)

	// Start the components again
	host = storagetest.NewStorageHost(t, storageDir, "test")
	rcvr, err = f.CreateLogsReceiver(ctx, params, cfg, sink)
	require.NoError(t, err, "failed to create receiver")
	require.NoError(t, rcvr.Start(ctx, host))
	sink.Reset()

	// Expect only the new 5
	require.Eventually(t,
		expectNLogs(sink, 5),
		time.Second,
		10*time.Millisecond,
		"expected 5 but got %d logs",
		sink.LogRecordsCount(),
	)

	// Shut down the components
	require.NoError(t, rcvr.Shutdown(ctx))
	for _, e := range host.GetExtensions() {
		require.NoError(t, e.Shutdown(ctx))
	}
}
