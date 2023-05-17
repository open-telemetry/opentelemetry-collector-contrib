// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package groupbytraceprocessor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func TestMemoryCreateAndGetTrace(t *testing.T) {
	// prepare
	st := newMemoryStorage()

	traceIDs := []pcommon.TraceID{
		pcommon.TraceID([16]byte{1, 2, 3, 4}),
		pcommon.TraceID([16]byte{2, 3, 4, 5}),
	}

	baseTrace := ptrace.NewTraces()
	rss := baseTrace.ResourceSpans()
	rs := rss.AppendEmpty()
	ils := rs.ScopeSpans().AppendEmpty()
	span := ils.Spans().AppendEmpty()

	// test
	for _, traceID := range traceIDs {
		span.SetTraceID(traceID)
		assert.NoError(t, st.createOrAppend(traceID, baseTrace))
	}

	// verify
	assert.Equal(t, 2, st.count())
	for _, traceID := range traceIDs {
		expected := []ptrace.ResourceSpans{baseTrace.ResourceSpans().At(0)}
		expected[0].ScopeSpans().At(0).Spans().At(0).SetTraceID(traceID)

		retrieved, err := st.get(traceID)
		assert.NoError(t, st.createOrAppend(traceID, baseTrace))

		require.NoError(t, err)
		assert.Equal(t, expected, retrieved)
	}
}

func TestMemoryDeleteTrace(t *testing.T) {
	// prepare
	st := newMemoryStorage()

	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4})

	trace := ptrace.NewTraces()
	rss := trace.ResourceSpans()
	rs := rss.AppendEmpty()
	ils := rs.ScopeSpans().AppendEmpty()
	span := ils.Spans().AppendEmpty()
	span.SetTraceID(traceID)

	assert.NoError(t, st.createOrAppend(traceID, trace))

	// test
	deleted, err := st.delete(traceID)

	// verify
	require.NoError(t, err)
	assert.Equal(t, []ptrace.ResourceSpans{trace.ResourceSpans().At(0)}, deleted)

	retrieved, err := st.get(traceID)
	require.NoError(t, err)
	assert.Nil(t, retrieved)
}

func TestMemoryAppendSpans(t *testing.T) {
	// prepare
	st := newMemoryStorage()

	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4})

	trace := ptrace.NewTraces()
	rss := trace.ResourceSpans()
	rs := rss.AppendEmpty()
	ils := rs.ScopeSpans().AppendEmpty()
	span := ils.Spans().AppendEmpty()
	span.SetTraceID(traceID)
	span.SetSpanID([8]byte{1, 2, 3, 4})

	assert.NoError(t, st.createOrAppend(traceID, trace))

	secondTrace := ptrace.NewTraces()
	secondRss := secondTrace.ResourceSpans()
	secondRs := secondRss.AppendEmpty()
	secondIls := secondRs.ScopeSpans().AppendEmpty()
	secondSpan := secondIls.Spans().AppendEmpty()
	secondSpan.SetName("second-name")
	secondSpan.SetTraceID(traceID)
	secondSpan.SetSpanID([8]byte{5, 6, 7, 8})

	expected := []ptrace.ResourceSpans{
		ptrace.NewResourceSpans(),
		ptrace.NewResourceSpans(),
	}
	ils.CopyTo(expected[0].ScopeSpans().AppendEmpty())
	secondIls.CopyTo(expected[1].ScopeSpans().AppendEmpty())

	// test
	err := st.createOrAppend(traceID, secondTrace)
	require.NoError(t, err)

	// override something in the second span, to make sure we are storing a copy
	secondSpan.SetName("changed-second-name")

	// verify
	retrieved, err := st.get(traceID)
	require.NoError(t, err)
	require.Len(t, retrieved, 2)
	assert.Equal(t, "second-name", retrieved[1].ScopeSpans().At(0).Spans().At(0).Name())

	// now that we checked that the secondSpan change here didn't have an effect, revert
	// so that we can compare the that everything else has the same value
	secondSpan.SetName("second-name")
	assert.Equal(t, expected, retrieved)
}

func TestMemoryTraceIsBeingCloned(t *testing.T) {
	// prepare
	st := newMemoryStorage()
	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4})

	trace := ptrace.NewTraces()
	rss := trace.ResourceSpans()
	rs := rss.AppendEmpty()
	ils := rs.ScopeSpans().AppendEmpty()
	span := ils.Spans().AppendEmpty()
	span.SetTraceID(traceID)
	span.SetSpanID([8]byte{1, 2, 3, 4})
	span.SetName("should-not-be-changed")

	// test
	err := st.createOrAppend(traceID, trace)
	require.NoError(t, err)
	span.SetName("changed-trace")

	// verify
	retrieved, err := st.get(traceID)
	require.NoError(t, err)
	assert.Equal(t, "should-not-be-changed", retrieved[0].ScopeSpans().At(0).Spans().At(0).Name())
}
