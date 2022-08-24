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

package adapter

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
)

func TestLogEmitter(t *testing.T) {
	emitter := NewLogEmitter(
		LogEmitterWithLogger(zaptest.NewLogger(t).Sugar()),
	)

	require.NoError(t, emitter.Start(nil))

	defer func() {
		require.NoError(t, emitter.Stop())
	}()

	in := entry.New()

	go func() {
		require.NoError(t, emitter.Process(context.Background(), in))
	}()

	select {
	case out := <-emitter.logChan:
		require.Equal(t, in, out[0])
	case <-time.After(time.Second):
		require.FailNow(t, "Timed out waiting for output")
	}
}

func TestLogEmitterRespectsMaxBatchSize(t *testing.T) {
	const (
		numEntries   = 1111
		maxBatchSize = 100
		timeout      = time.Second
	)
	emitter := NewLogEmitter(
		LogEmitterWithLogger(zaptest.NewLogger(t).Sugar()),
		LogEmitterWithMaxBatchSize(maxBatchSize),
		LogEmitterWithFlushInterval(100*time.Millisecond),
	)

	require.NoError(t, emitter.Start(nil))
	defer func() {
		require.NoError(t, emitter.Stop())
	}()

	entries := complexEntries(numEntries)

	go func() {
		ctx := context.Background()
		for _, e := range entries {
			require.NoError(t, emitter.Process(ctx, e))
		}
	}()

	entriesReceived := 0
	timeoutChan := time.After(timeout)

	for entriesReceived < numEntries {
		select {
		case recv := <-emitter.logChan:
			entriesReceived += len(recv)
			if len(recv) > maxBatchSize {
				require.FailNow(t, "Expected only %d entries per batch, but got %d", maxBatchSize, entriesReceived)
			}
		case <-timeoutChan:
			require.FailNow(t, "Failed to receive all log entries before timeout")
		}
	}

	require.Equal(t, numEntries, entriesReceived)
}

func TestLogEmitterEmitsOnMaxBatchSize(t *testing.T) {
	const (
		maxBatchSize = 100
		timeout      = time.Second
	)
	emitter := NewLogEmitter(
		LogEmitterWithLogger(zaptest.NewLogger(t).Sugar()),
		LogEmitterWithMaxBatchSize(maxBatchSize),
		LogEmitterWithFlushInterval(time.Hour),
	)

	require.NoError(t, emitter.Start(nil))
	defer func() {
		require.NoError(t, emitter.Stop())
	}()

	entries := complexEntries(maxBatchSize)

	go func() {
		ctx := context.Background()
		for _, e := range entries {
			require.NoError(t, emitter.Process(ctx, e))
		}
	}()

	timeoutChan := time.After(timeout)

	select {
	case recv := <-emitter.logChan:
		require.Equal(t, maxBatchSize, len(recv), "Length of received entries was not the same as max batch size!")
	case <-timeoutChan:
		require.FailNow(t, "Failed to receive log entries before timeout")
	}
}

func TestLogEmitterEmitsOnFlushInterval(t *testing.T) {
	const (
		flushInterval = 100 * time.Millisecond
		timeout       = time.Second
	)
	emitter := NewLogEmitter(
		LogEmitterWithLogger(zaptest.NewLogger(t).Sugar()),
		LogEmitterWithMaxBatchSize(100),
		LogEmitterWithFlushInterval(flushInterval),
	)

	require.NoError(t, emitter.Start(nil))
	defer func() {
		require.NoError(t, emitter.Stop())
	}()

	entry := complexEntry()

	go func() {
		ctx := context.Background()
		require.NoError(t, emitter.Process(ctx, entry))
	}()

	timeoutChan := time.After(timeout)

	select {
	case recv := <-emitter.logChan:
		require.Equal(t, 1, len(recv), "Should have received one entry, got %d instead", len(recv))
	case <-timeoutChan:
		require.FailNow(t, "Failed to receive log entry before timeout")
	}
}
