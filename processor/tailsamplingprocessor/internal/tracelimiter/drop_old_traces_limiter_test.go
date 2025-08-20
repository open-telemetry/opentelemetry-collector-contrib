// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tracelimiter

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func TestDropOldTracesLimiter(t *testing.T) {
	// This test verifies that DropOldTracesLimiter drops the oldest trace when the buffer is full.
	var dropped []pcommon.TraceID
	dropTrace := func(id pcommon.TraceID, _ time.Time) {
		dropped = append(dropped, id)
	}

	numTraces := uint64(2)
	limiter := NewDropOldTracesLimiter(numTraces, dropTrace)

	id1 := pcommon.TraceID([16]byte{1})
	id2 := pcommon.TraceID([16]byte{2})
	id3 := pcommon.TraceID([16]byte{3})

	// Accept first two traces, should not drop anything
	limiter.AcceptTrace(t.Context(), id1, time.Now())
	limiter.AcceptTrace(t.Context(), id2, time.Now())
	require.Empty(t, dropped, "expected no dropped traces")

	// Accept third trace, should drop the oldest (id1)
	limiter.AcceptTrace(t.Context(), id3, time.Now())
	require.Len(t, dropped, 1, "expected 1 dropped trace")
	require.Equal(t, id1, dropped[0], "expected dropped trace to be id1")

	// Accept another trace, should drop the next oldest (id2)
	id4 := pcommon.TraceID([16]byte{4})
	limiter.AcceptTrace(t.Context(), id4, time.Now())
	require.Len(t, dropped, 2, "expected 2 dropped traces")
	require.Equal(t, id2, dropped[1], "expected dropped trace to be id2")

	// Accept another trace, should drop id3
	id5 := pcommon.TraceID([16]byte{5})
	limiter.AcceptTrace(t.Context(), id5, time.Now())
	require.Len(t, dropped, 3, "expected 3 dropped traces")
	require.Equal(t, id3, dropped[2], "expected dropped trace to be id3")
}
