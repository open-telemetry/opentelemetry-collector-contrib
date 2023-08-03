// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package batchpersignal

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func TestSplitDifferentTracesIntoDifferentBatches(t *testing.T) {
	// we have 1 ResourceSpans with 1 ILS and two traceIDs, resulting in two batches
	inBatch := ptrace.NewTraces()
	rs := inBatch.ResourceSpans().AppendEmpty()
	rs.SetSchemaUrl("https://opentelemetry.io/schemas/1.6.1")
	// the first ILS has two spans
	ils := rs.ScopeSpans().AppendEmpty()
	ils.SetSchemaUrl("https://opentelemetry.io/schemas/1.6.1")
	library := ils.Scope()
	library.SetName("first-library")
	firstSpan := ils.Spans().AppendEmpty()
	firstSpan.SetName("first-batch-first-span")
	firstSpan.SetTraceID([16]byte{1, 2, 3, 4})
	secondSpan := ils.Spans().AppendEmpty()
	secondSpan.SetName("first-batch-second-span")
	secondSpan.SetTraceID([16]byte{2, 3, 4, 5})

	// test
	out := SplitTraces(inBatch)

	// verify
	assert.Len(t, out, 2)

	// first batch
	firstOutRS := out[0].ResourceSpans().At(0)
	assert.Equal(t, rs.SchemaUrl(), firstOutRS.SchemaUrl())

	firstOutILS := out[0].ResourceSpans().At(0).ScopeSpans().At(0)
	assert.Equal(t, library.Name(), firstOutILS.Scope().Name())
	assert.Equal(t, firstSpan.Name(), firstOutILS.Spans().At(0).Name())
	assert.Equal(t, ils.SchemaUrl(), firstOutILS.SchemaUrl())

	// second batch
	secondOutRS := out[1].ResourceSpans().At(0)
	assert.Equal(t, rs.SchemaUrl(), secondOutRS.SchemaUrl())

	secondOutILS := out[1].ResourceSpans().At(0).ScopeSpans().At(0)
	assert.Equal(t, library.Name(), secondOutILS.Scope().Name())
	assert.Equal(t, secondSpan.Name(), secondOutILS.Spans().At(0).Name())
	assert.Equal(t, ils.SchemaUrl(), secondOutILS.SchemaUrl())

}

func TestSplitTracesWithNilTraceID(t *testing.T) {
	// prepare
	inBatch := ptrace.NewTraces()
	rs := inBatch.ResourceSpans().AppendEmpty()
	rs.SetSchemaUrl("https://opentelemetry.io/schemas/1.6.1")
	ils := rs.ScopeSpans().AppendEmpty()
	ils.SetSchemaUrl("https://opentelemetry.io/schemas/1.6.1")
	firstSpan := ils.Spans().AppendEmpty()
	firstSpan.SetTraceID([16]byte{})

	// test
	batches := SplitTraces(inBatch)

	// verify
	assert.Len(t, batches, 1)
	assert.Equal(t, pcommon.TraceID([16]byte{}), batches[0].ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).TraceID())
	assert.Equal(t, rs.SchemaUrl(), batches[0].ResourceSpans().At(0).SchemaUrl())
	assert.Equal(t, ils.SchemaUrl(), batches[0].ResourceSpans().At(0).ScopeSpans().At(0).SchemaUrl())
}

func TestSplitSameTraceIntoDifferentBatches(t *testing.T) {
	// prepare
	inBatch := ptrace.NewTraces()
	rs := inBatch.ResourceSpans().AppendEmpty()
	rs.SetSchemaUrl("https://opentelemetry.io/schemas/1.6.1")

	// we have 1 ResourceSpans with 2 ILS, resulting in two batches
	rs.ScopeSpans().EnsureCapacity(2)

	// the first ILS has two spans
	firstILS := rs.ScopeSpans().AppendEmpty()
	firstILS.SetSchemaUrl("https://opentelemetry.io/schemas/1.6.1")

	firstLibrary := firstILS.Scope()
	firstLibrary.SetName("first-library")
	firstILS.Spans().EnsureCapacity(2)
	firstSpan := firstILS.Spans().AppendEmpty()
	firstSpan.SetName("first-batch-first-span")
	firstSpan.SetTraceID([16]byte{1, 2, 3, 4})
	secondSpan := firstILS.Spans().AppendEmpty()
	secondSpan.SetName("first-batch-second-span")
	secondSpan.SetTraceID([16]byte{1, 2, 3, 4})

	// the second ILS has one span
	secondILS := rs.ScopeSpans().AppendEmpty()
	secondILS.SetSchemaUrl("https://opentelemetry.io/schemas/1.6.1")

	secondLibrary := secondILS.Scope()
	secondLibrary.SetName("second-library")
	thirdSpan := secondILS.Spans().AppendEmpty()
	thirdSpan.SetName("second-batch-first-span")
	thirdSpan.SetTraceID([16]byte{1, 2, 3, 4})

	// test
	batches := SplitTraces(inBatch)

	// verify
	assert.Len(t, batches, 2)

	// first batch
	assert.Equal(t, rs.SchemaUrl(), batches[0].ResourceSpans().At(0).SchemaUrl())
	assert.Equal(t, firstILS.SchemaUrl(), batches[0].ResourceSpans().At(0).ScopeSpans().At(0).SchemaUrl())
	assert.Equal(t, pcommon.TraceID([16]byte{1, 2, 3, 4}), batches[0].ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).TraceID())
	assert.Equal(t, firstLibrary.Name(), batches[0].ResourceSpans().At(0).ScopeSpans().At(0).Scope().Name())
	assert.Equal(t, firstSpan.Name(), batches[0].ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Name())
	assert.Equal(t, secondSpan.Name(), batches[0].ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(1).Name())

	// second batch
	assert.Equal(t, rs.SchemaUrl(), batches[1].ResourceSpans().At(0).SchemaUrl())
	assert.Equal(t, secondILS.SchemaUrl(), batches[1].ResourceSpans().At(0).ScopeSpans().At(0).SchemaUrl())
	assert.Equal(t, pcommon.TraceID([16]byte{1, 2, 3, 4}), batches[1].ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).TraceID())
	assert.Equal(t, secondLibrary.Name(), batches[1].ResourceSpans().At(0).ScopeSpans().At(0).Scope().Name())
	assert.Equal(t, thirdSpan.Name(), batches[1].ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Name())
}

func TestSplitDifferentLogsIntoDifferentBatches(t *testing.T) {
	// we have 1 ResourceLogs with 1 ILL and three traceIDs (one null) resulting in three batches
	inBatch := plog.NewLogs()
	rl := inBatch.ResourceLogs().AppendEmpty()
	rl.SetSchemaUrl("https://opentelemetry.io/schemas/1.6.1")

	// the first ILL has three logs
	sl := rl.ScopeLogs().AppendEmpty()
	sl.SetSchemaUrl("https://opentelemetry.io/schemas/1.6.1")
	library := sl.Scope()
	library.SetName("first-library")
	sl.LogRecords().EnsureCapacity(3)
	firstLog := sl.LogRecords().AppendEmpty()
	firstLog.Body().SetStr("first-batch-first-log")
	firstLog.SetTraceID([16]byte{1, 2, 3, 4})
	secondLog := sl.LogRecords().AppendEmpty()
	secondLog.Body().SetStr("first-batch-second-log")
	secondLog.SetTraceID([16]byte{2, 3, 4, 5})
	thirdLog := sl.LogRecords().AppendEmpty()
	thirdLog.Body().SetStr("first-batch-third-log")
	// do not set traceID for third log

	// test
	out := SplitLogs(inBatch)

	// verify
	assert.Len(t, out, 3)

	// first batch
	assert.Equal(t, rl.SchemaUrl(), out[0].ResourceLogs().At(0).SchemaUrl())
	firstOutILL := out[0].ResourceLogs().At(0).ScopeLogs().At(0)
	assert.Equal(t, sl.SchemaUrl(), firstOutILL.SchemaUrl())
	assert.Equal(t, library.Name(), firstOutILL.Scope().Name())
	assert.Equal(t, firstLog.Body().Str(), firstOutILL.LogRecords().At(0).Body().Str())

	// second batch
	assert.Equal(t, rl.SchemaUrl(), out[1].ResourceLogs().At(0).SchemaUrl())
	secondOutILL := out[1].ResourceLogs().At(0).ScopeLogs().At(0)
	assert.Equal(t, sl.SchemaUrl(), secondOutILL.SchemaUrl())
	assert.Equal(t, library.Name(), secondOutILL.Scope().Name())
	assert.Equal(t, secondLog.Body().Str(), secondOutILL.LogRecords().At(0).Body().Str())

	// third batch
	assert.Equal(t, rl.SchemaUrl(), out[2].ResourceLogs().At(0).SchemaUrl())
	thirdOutILL := out[2].ResourceLogs().At(0).ScopeLogs().At(0)
	assert.Equal(t, sl.SchemaUrl(), thirdOutILL.SchemaUrl())
	assert.Equal(t, library.Name(), thirdOutILL.Scope().Name())
	assert.Equal(t, thirdLog.Body().Str(), thirdOutILL.LogRecords().At(0).Body().Str())
}

func TestSplitLogsWithNilTraceID(t *testing.T) {
	// prepare
	inBatch := plog.NewLogs()
	rl := inBatch.ResourceLogs().AppendEmpty()
	rl.SetSchemaUrl("https://opentelemetry.io/schemas/1.6.1")

	sl := rl.ScopeLogs().AppendEmpty()
	sl.SetSchemaUrl("https://opentelemetry.io/schemas/1.6.1")

	firstLog := sl.LogRecords().AppendEmpty()
	firstLog.SetTraceID([16]byte{})

	// test
	batches := SplitLogs(inBatch)

	// verify
	assert.Len(t, batches, 1)
	assert.Equal(t, pcommon.TraceID([16]byte{}), batches[0].ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).TraceID())
	assert.Equal(t, rl.SchemaUrl(), batches[0].ResourceLogs().At(0).SchemaUrl())
}

func TestSplitLogsSameTraceIntoDifferentBatches(t *testing.T) {
	// prepare
	inBatch := plog.NewLogs()
	rl := inBatch.ResourceLogs().AppendEmpty()
	rl.SetSchemaUrl("https://opentelemetry.io/schemas/1.6.1")

	// we have 1 ResourceLogs with 2 ILL, resulting in two batches
	rl.ScopeLogs().EnsureCapacity(2)

	// the first ILL has two logs
	firstILS := rl.ScopeLogs().AppendEmpty()
	firstILS.SetSchemaUrl("https://opentelemetry.io/schemas/1.6.1")
	firstLibrary := firstILS.Scope()
	firstLibrary.SetName("first-library")
	firstILS.LogRecords().EnsureCapacity(2)
	firstLog := firstILS.LogRecords().AppendEmpty()
	firstLog.Body().SetStr("first-batch-first-log")
	firstLog.SetTraceID([16]byte{1, 2, 3, 4})
	secondLog := firstILS.LogRecords().AppendEmpty()
	secondLog.Body().SetStr("first-batch-second-log")
	secondLog.SetTraceID([16]byte{1, 2, 3, 4})

	// the second ILL has one log
	secondILS := rl.ScopeLogs().AppendEmpty()
	secondILS.SetSchemaUrl("https://opentelemetry.io/schemas/1.6.1")
	secondLibrary := secondILS.Scope()
	secondLibrary.SetName("second-library")
	thirdLog := secondILS.LogRecords().AppendEmpty()
	thirdLog.Body().SetStr("second-batch-first-log")
	thirdLog.SetTraceID([16]byte{1, 2, 3, 4})

	// test
	batches := SplitLogs(inBatch)

	// verify
	assert.Len(t, batches, 2)

	// first batch
	assert.Equal(t, rl.SchemaUrl(), batches[0].ResourceLogs().At(0).SchemaUrl())
	assert.Equal(t, firstILS.SchemaUrl(), batches[0].ResourceLogs().At(0).ScopeLogs().At(0).SchemaUrl())
	assert.Equal(t, pcommon.TraceID([16]byte{1, 2, 3, 4}), batches[0].ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).TraceID())
	assert.Equal(t, firstLibrary.Name(), batches[0].ResourceLogs().At(0).ScopeLogs().At(0).Scope().Name())
	assert.Equal(t, firstLog.Body().Str(), batches[0].ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().Str())
	assert.Equal(t, secondLog.Body().Str(), batches[0].ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(1).Body().Str())

	// second batch
	assert.Equal(t, rl.SchemaUrl(), batches[1].ResourceLogs().At(0).SchemaUrl())
	assert.Equal(t, secondILS.SchemaUrl(), batches[1].ResourceLogs().At(0).ScopeLogs().At(0).SchemaUrl())
	assert.Equal(t, pcommon.TraceID([16]byte{1, 2, 3, 4}), batches[1].ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).TraceID())
	assert.Equal(t, secondLibrary.Name(), batches[1].ResourceLogs().At(0).ScopeLogs().At(0).Scope().Name())
	assert.Equal(t, thirdLog.Body().Str(), batches[1].ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().Str())
}
