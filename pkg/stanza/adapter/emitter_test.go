// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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
	emitter := NewLogEmitter(zaptest.NewLogger(t).Sugar())

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

func TestLogEmitterEmitsOnMaxBatchSize(t *testing.T) {
	const (
		maxBatchSize = 100
		timeout      = time.Second
	)
	emitter := NewLogEmitter(zaptest.NewLogger(t).Sugar())

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
	emitter := NewLogEmitter(zaptest.NewLogger(t).Sugar())

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
