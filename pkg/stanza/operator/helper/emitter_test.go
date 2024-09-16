// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
)

func TestLogEmitter(t *testing.T) {
	emitter := NewLogEmitter(componenttest.NewNopTelemetrySettings())

	require.NoError(t, emitter.Start(nil))

	defer func() {
		require.NoError(t, emitter.Stop())
	}()

	in := entry.New()

	go func() {
		assert.NoError(t, emitter.Process(context.Background(), in))
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
	emitter := NewLogEmitter(componenttest.NewNopTelemetrySettings())

	require.NoError(t, emitter.Start(nil))
	defer func() {
		require.NoError(t, emitter.Stop())
	}()

	entries := complexEntries(maxBatchSize)

	go func() {
		ctx := context.Background()
		for _, e := range entries {
			assert.NoError(t, emitter.Process(ctx, e))
		}
	}()

	timeoutChan := time.After(timeout)

	select {
	case recv := <-emitter.logChan:
		require.Len(t, recv, maxBatchSize, "Length of received entries was not the same as max batch size!")
	case <-timeoutChan:
		require.FailNow(t, "Failed to receive log entries before timeout")
	}
}

func TestLogEmitterEmitsOnFlushInterval(t *testing.T) {
	const (
		flushInterval = 100 * time.Millisecond
		timeout       = time.Second
	)
	emitter := NewLogEmitter(componenttest.NewNopTelemetrySettings())

	require.NoError(t, emitter.Start(nil))
	defer func() {
		require.NoError(t, emitter.Stop())
	}()

	entry := complexEntry()

	go func() {
		ctx := context.Background()
		assert.NoError(t, emitter.Process(ctx, entry))
	}()

	timeoutChan := time.After(timeout)

	select {
	case recv := <-emitter.logChan:
		require.Len(t, recv, 1, "Should have received one entry, got %d instead", len(recv))
	case <-timeoutChan:
		require.FailNow(t, "Failed to receive log entry before timeout")
	}
}

func complexEntries(count int) []*entry.Entry {
	return complexEntriesForNDifferentHosts(count, 1)
}

func complexEntry() *entry.Entry {
	e := entry.New()
	e.Severity = entry.Error
	e.Resource = map[string]any{
		"bool":   true,
		"int":    123,
		"double": 12.34,
		"string": "hello",
		"object": map[string]any{
			"bool":   true,
			"int":    123,
			"double": 12.34,
			"string": "hello",
		},
	}
	e.Attributes = map[string]any{
		"bool":   true,
		"int":    123,
		"double": 12.34,
		"string": "hello",
		"object": map[string]any{
			"bool":   true,
			"int":    123,
			"double": 12.34,
			"string": "hello",
		},
	}
	e.Body = map[string]any{
		"bool":   true,
		"int":    123,
		"double": 12.34,
		"string": "hello",
		// "bytes":  []byte("asdf"),
		"object": map[string]any{
			"bool":   true,
			"int":    123,
			"double": 12.34,
			"string": "hello",
			// "bytes":  []byte("asdf"),
			"object": map[string]any{
				"bool": true,
				"int":  123,
				// "double": 12.34,
				"string": "hello",
				// "bytes":  []byte("asdf"),
			},
		},
	}
	return e
}

func complexEntriesForNDifferentHosts(count int, n int) []*entry.Entry {
	ret := make([]*entry.Entry, count)
	for i := 0; i < count; i++ {
		e := entry.New()
		e.Severity = entry.Error
		e.Resource = map[string]any{
			"host":   fmt.Sprintf("host-%d", i%n),
			"bool":   true,
			"int":    123,
			"double": 12.34,
			"string": "hello",
			"object": map[string]any{
				"bool":   true,
				"int":    123,
				"double": 12.34,
				"string": "hello",
			},
		}
		e.Body = map[string]any{
			"bool":   true,
			"int":    123,
			"double": 12.34,
			"string": "hello",
			"bytes":  []byte("asdf"),
			"object": map[string]any{
				"bool":   true,
				"int":    123,
				"double": 12.34,
				"string": "hello",
				"bytes":  []byte("asdf"),
				"object": map[string]any{
					"bool":   true,
					"int":    123,
					"double": 12.34,
					"string": "hello",
					"bytes":  []byte("asdf"),
				},
			},
		}
		ret[i] = e
	}
	return ret
}
