// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pebbletailstorageextension

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/extension/extensiontest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func newStartedTailStorage(t *testing.T) TailStorage {
	t.Helper()

	f := NewFactory()
	cfg := f.CreateDefaultConfig().(*Config)
	cfg.Directory = t.TempDir()

	ext, err := f.Create(
		t.Context(),
		extensiontest.NewNopSettings(f.Type()),
		cfg,
	)
	require.NoError(t, err)
	require.NotNil(t, ext)

	require.NoError(t, ext.Start(t.Context(), componenttest.NewNopHost()))
	t.Cleanup(func() {
		require.NoError(t, ext.Shutdown(t.Context()))
	})

	storage, ok := ext.(TailStorage)
	require.True(t, ok)
	return storage
}

func appendTraceSpan(storage TailStorage, traceID pcommon.TraceID, spanID pcommon.SpanID, name string) {
	td := ptrace.NewTraces()
	rss := td.ResourceSpans().AppendEmpty()
	span := rss.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.SetTraceID(traceID)
	if spanID != (pcommon.SpanID{}) {
		span.SetSpanID(spanID)
	}
	if name != "" {
		span.SetName(name)
	}
	storage.Append(traceID, rss)
}

func TestAppendThenTake(t *testing.T) {
	storage := newStartedTailStorage(t)

	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4})
	appendTraceSpan(storage, traceID, pcommon.SpanID{}, "")

	out, found := storage.Take(traceID)
	require.True(t, found)
	require.Equal(t, 1, out.SpanCount())
}

func TestDeleteRemovesOnlyTargetTrace(t *testing.T) {
	storage := newStartedTailStorage(t)

	traceID1 := pcommon.TraceID([16]byte{1, 2, 3, 4})
	traceID2 := pcommon.TraceID([16]byte{1, 2, 3, 5})

	// Append multiple entries for traceID1 to exercise range deletion.
	for i := range 3 {
		appendTraceSpan(storage, traceID1, pcommon.SpanID([8]byte{byte(i + 1)}), "trace1-span")
	}

	appendTraceSpan(storage, traceID2, pcommon.SpanID{}, "")

	storage.Delete(traceID1)

	_, found := storage.Take(traceID1)
	require.False(t, found)

	out2, found := storage.Take(traceID2)
	require.True(t, found)
	require.Equal(t, 1, out2.SpanCount())
}

func TestTakeRemovesOnlyTargetTrace(t *testing.T) {
	storage := newStartedTailStorage(t)

	traceID1 := pcommon.TraceID([16]byte{9, 9, 9, 1})
	traceID2 := pcommon.TraceID([16]byte{9, 9, 9, 2})

	for i := range 3 {
		appendTraceSpan(storage, traceID1, pcommon.SpanID([8]byte{byte(i + 1)}), "")
	}

	appendTraceSpan(storage, traceID2, pcommon.SpanID{}, "")

	out1, found := storage.Take(traceID1)
	require.True(t, found)
	require.Equal(t, 3, out1.SpanCount())

	_, found = storage.Take(traceID1)
	require.False(t, found)

	out2, found := storage.Take(traceID2)
	require.True(t, found)
	require.Equal(t, 1, out2.SpanCount())
}

// TestStartErrorsIfDBExists guards the ErrorIfExists Pebble option set in pebble.go,
// which prevents users from relying on persistence across restarts while the
// on-disk schema is still in development.
func TestStartErrorsIfDBExists(t *testing.T) {
	f := NewFactory()
	cfg := f.CreateDefaultConfig().(*Config)
	cfg.Directory = t.TempDir()

	first, err := f.Create(t.Context(), extensiontest.NewNopSettings(f.Type()), cfg)
	require.NoError(t, err)
	require.NoError(t, first.Start(t.Context(), componenttest.NewNopHost()))
	require.NoError(t, first.Shutdown(t.Context()))

	second, err := f.Create(t.Context(), extensiontest.NewNopSettings(f.Type()), cfg)
	require.NoError(t, err)
	require.Error(t, second.Start(t.Context(), componenttest.NewNopHost()))
}
