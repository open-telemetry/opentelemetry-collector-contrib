// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !aix && !solaris

package pebbletailstorageextension

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

type takeManyStorage interface {
	TailStorage
	TakeMany(traceIDs []pcommon.TraceID) ([]ptrace.Traces, []error)
}

func TestTakeMany(t *testing.T) {
	st, ok := newStartedTailStorage(t).(takeManyStorage)
	require.True(t, ok)

	idA := pcommon.TraceID([16]byte{1})
	idB := pcommon.TraceID([16]byte{2})
	idC := pcommon.TraceID([16]byte{3})
	appendTraceSpan(st, idA, pcommon.SpanID([8]byte{1}), "a1")
	appendTraceSpan(st, idA, pcommon.SpanID([8]byte{2}), "a2")
	appendTraceSpan(st, idB, pcommon.SpanID([8]byte{3}), "b1")

	out, errs := st.TakeMany([]pcommon.TraceID{idC, idA, idB})
	require.Len(t, out, 3)
	require.Len(t, errs, 3)
	for _, err := range errs {
		require.NoError(t, err)
	}
	assert.Equal(t, 0, out[0].ResourceSpans().Len())
	assert.Equal(t, 2, out[1].ResourceSpans().Len())
	assert.Equal(t, 1, out[2].ResourceSpans().Len())
	assert.Equal(t, 3, out[1].SpanCount()+out[2].SpanCount())

	out, errs = st.TakeMany([]pcommon.TraceID{idA, idB})
	require.Len(t, out, 2)
	for i := range out {
		require.NoError(t, errs[i])
		assert.Equal(t, 0, out[i].ResourceSpans().Len())
	}

	td, err := st.Take(idA)
	require.NoError(t, err)
	assert.Equal(t, 0, td.ResourceSpans().Len())
}

func TestTakeManyEmpty(t *testing.T) {
	st, ok := newStartedTailStorage(t).(takeManyStorage)
	require.True(t, ok)

	out, errs := st.TakeMany(nil)
	assert.Empty(t, out)
	assert.Empty(t, errs)
}

func TestTakeManyReadErrorPolicy(t *testing.T) {
	t.Run("drop_trace", func(t *testing.T) {
		traceID := pcommon.TraceID([16]byte{7})
		s, tel := newStorageWithReadFailure(t, ReadErrorPolicyDropTrace, traceID)

		out, errs := s.TakeMany([]pcommon.TraceID{traceID})
		require.Len(t, out, 1)
		require.ErrorIs(t, errs[0], errReadErrorDropTrace)
		assert.Equal(t, 0, out[0].ResourceSpans().Len())
		assertCounter(t, tel, "otelcol_extension_pebble_tail_storage_read_error_trace_drops", 1)
		assertCounter(t, tel, "otelcol_extension_pebble_tail_storage_read_error_partial_returns", 0)

		s.newIter = func() (storageIter, error) { return s.db.NewIter(nil) }
		td, err := s.Take(traceID)
		require.NoError(t, err)
		assert.Equal(t, 0, td.ResourceSpans().Len())
	})

	t.Run("return_partial", func(t *testing.T) {
		traceID := pcommon.TraceID([16]byte{8})
		s, tel := newStorageWithReadFailure(t, ReadErrorPolicyReturnPartial, traceID)

		out, errs := s.TakeMany([]pcommon.TraceID{traceID})
		require.Len(t, out, 1)
		require.NoError(t, errs[0])
		assert.Equal(t, 1, out[0].ResourceSpans().Len())
		assertCounter(t, tel, "otelcol_extension_pebble_tail_storage_read_error_partial_returns", 1)
		assertCounter(t, tel, "otelcol_extension_pebble_tail_storage_read_error_trace_drops", 0)
	})
}
