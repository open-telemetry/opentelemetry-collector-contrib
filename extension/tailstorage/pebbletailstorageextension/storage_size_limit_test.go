// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pebbletailstorageextension

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

func newTestStorage(t *testing.T, cfg Config) *storage {
	t.Helper()

	cfg.Directory = t.TempDir()
	require.NoError(t, cfg.Validate())

	s, err := newStorage(t.Context(), &cfg, zap.NewNop())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, s.Close())
	})

	return s
}

func testTraces(traceID pcommon.TraceID, name string) ptrace.Traces {
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	span := rs.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.SetTraceID(traceID)
	span.SetSpanID(pcommon.SpanID([8]byte{traceID[15], 1, 2, 3, 4, 5, 6, 7}))
	span.SetName(name)
	return td
}

func TestStorageSizeLimitRejectsWhenLastObservedSizeExceedsLimit(t *testing.T) {
	s := newTestStorage(t, Config{MaxStorageSizeMiB: 1})
	s.lastObservedSize.Store(s.maxSize + 1)

	err := s.Append(pcommon.TraceID([16]byte{1}), testTraces(pcommon.TraceID([16]byte{1}), "trace-1"))
	require.ErrorContains(t, err, errStorageLimitReached.Error())
	require.Contains(t, err.Error(), "last observed database size")
}

func TestStorageSizeLimitUsesCachedObservation(t *testing.T) {
	s := newTestStorage(t, Config{MaxStorageSizeMiB: 1})
	s.lastObservedSize.Store(0)
	require.NoError(t, s.Append(pcommon.TraceID([16]byte{1}), testTraces(pcommon.TraceID([16]byte{1}), "trace-1")))

	s.lastObservedSize.Store(s.maxSize + 1)
	err := s.Append(pcommon.TraceID([16]byte{2}), testTraces(pcommon.TraceID([16]byte{2}), "trace-2"))
	require.ErrorContains(t, err, errStorageLimitReached.Error())
}
