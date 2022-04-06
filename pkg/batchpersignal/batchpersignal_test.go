// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package batchpersignal

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/model/pdata"
)

func TestSplitDifferentTracesIntoDifferentBatches(t *testing.T) {
	// we have 1 ResourceSpans with 1 ILS and two traceIDs, resulting in two batches
	inBatch := pdata.NewTraces()
	rs := inBatch.ResourceSpans().AppendEmpty()

	// the first ILS has two spans
	ils := rs.ScopeSpans().AppendEmpty()
	library := ils.Scope()
	library.SetName("first-library")
	firstSpan := ils.Spans().AppendEmpty()
	firstSpan.SetName("first-batch-first-span")
	firstSpan.SetTraceID(pdata.NewTraceID([16]byte{1, 2, 3, 4}))
	secondSpan := ils.Spans().AppendEmpty()
	secondSpan.SetName("first-batch-second-span")
	secondSpan.SetTraceID(pdata.NewTraceID([16]byte{2, 3, 4, 5}))

	// test
	out := SplitTraces(inBatch)

	// verify
	assert.Len(t, out, 2)

	// first batch
	firstOutILS := out[0].ResourceSpans().At(0).ScopeSpans().At(0)
	assert.Equal(t, library.Name(), firstOutILS.Scope().Name())
	assert.Equal(t, firstSpan.Name(), firstOutILS.Spans().At(0).Name())

	// second batch
	secondOutILS := out[1].ResourceSpans().At(0).ScopeSpans().At(0)
	assert.Equal(t, library.Name(), secondOutILS.Scope().Name())
	assert.Equal(t, secondSpan.Name(), secondOutILS.Spans().At(0).Name())
}

func TestSplitTracesWithNilTraceID(t *testing.T) {
	// prepare
	inBatch := pdata.NewTraces()
	rs := inBatch.ResourceSpans().AppendEmpty()
	ils := rs.ScopeSpans().AppendEmpty()
	firstSpan := ils.Spans().AppendEmpty()
	firstSpan.SetTraceID(pdata.NewTraceID([16]byte{}))

	// test
	batches := SplitTraces(inBatch)

	// verify
	assert.Len(t, batches, 1)
	assert.Equal(t, pdata.NewTraceID([16]byte{}), batches[0].ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).TraceID())
}

func TestSplitSameTraceIntoDifferentBatches(t *testing.T) {
	// prepare
	inBatch := pdata.NewTraces()
	rs := inBatch.ResourceSpans().AppendEmpty()

	// we have 1 ResourceSpans with 2 ILS, resulting in two batches
	rs.ScopeSpans().EnsureCapacity(2)

	// the first ILS has two spans
	firstILS := rs.ScopeSpans().AppendEmpty()
	firstLibrary := firstILS.Scope()
	firstLibrary.SetName("first-library")
	firstILS.Spans().EnsureCapacity(2)
	firstSpan := firstILS.Spans().AppendEmpty()
	firstSpan.SetName("first-batch-first-span")
	firstSpan.SetTraceID(pdata.NewTraceID([16]byte{1, 2, 3, 4}))
	secondSpan := firstILS.Spans().AppendEmpty()
	secondSpan.SetName("first-batch-second-span")
	secondSpan.SetTraceID(pdata.NewTraceID([16]byte{1, 2, 3, 4}))

	// the second ILS has one span
	secondILS := rs.ScopeSpans().AppendEmpty()
	secondLibrary := secondILS.Scope()
	secondLibrary.SetName("second-library")
	thirdSpan := secondILS.Spans().AppendEmpty()
	thirdSpan.SetName("second-batch-first-span")
	thirdSpan.SetTraceID(pdata.NewTraceID([16]byte{1, 2, 3, 4}))

	// test
	batches := SplitTraces(inBatch)

	// verify
	assert.Len(t, batches, 2)

	// first batch
	assert.Equal(t, pdata.NewTraceID([16]byte{1, 2, 3, 4}), batches[0].ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).TraceID())
	assert.Equal(t, firstLibrary.Name(), batches[0].ResourceSpans().At(0).ScopeSpans().At(0).Scope().Name())
	assert.Equal(t, firstSpan.Name(), batches[0].ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Name())
	assert.Equal(t, secondSpan.Name(), batches[0].ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(1).Name())

	// second batch
	assert.Equal(t, pdata.NewTraceID([16]byte{1, 2, 3, 4}), batches[1].ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).TraceID())
	assert.Equal(t, secondLibrary.Name(), batches[1].ResourceSpans().At(0).ScopeSpans().At(0).Scope().Name())
	assert.Equal(t, thirdSpan.Name(), batches[1].ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Name())
}

func TestSplitDifferentLogsIntoDifferentBatches(t *testing.T) {
	// we have 1 ResourceLogs with 1 ILL and three traceIDs (one null) resulting in three batches
	inBatch := pdata.NewLogs()
	rl := inBatch.ResourceLogs().AppendEmpty()

	// the first ILL has three logs
	sl := rl.ScopeLogs().AppendEmpty()
	library := sl.Scope()
	library.SetName("first-library")
	sl.LogRecords().EnsureCapacity(3)
	firstLog := sl.LogRecords().AppendEmpty()
	firstLog.SetName("first-batch-first-log")
	firstLog.SetTraceID(pdata.NewTraceID([16]byte{1, 2, 3, 4}))
	secondLog := sl.LogRecords().AppendEmpty()
	secondLog.SetName("first-batch-second-log")
	secondLog.SetTraceID(pdata.NewTraceID([16]byte{2, 3, 4, 5}))
	thirdLog := sl.LogRecords().AppendEmpty()
	thirdLog.SetName("first-batch-third-log")
	// do not set traceID for third log

	// test
	out := SplitLogs(inBatch)

	// verify
	assert.Len(t, out, 3)

	// first batch
	firstOutILL := out[0].ResourceLogs().At(0).ScopeLogs().At(0)
	assert.Equal(t, library.Name(), firstOutILL.Scope().Name())
	assert.Equal(t, firstLog.Name(), firstOutILL.LogRecords().At(0).Name())

	// second batch
	secondOutILL := out[1].ResourceLogs().At(0).ScopeLogs().At(0)
	assert.Equal(t, library.Name(), secondOutILL.Scope().Name())
	assert.Equal(t, secondLog.Name(), secondOutILL.LogRecords().At(0).Name())

	// third batch
	thirdOutILL := out[2].ResourceLogs().At(0).ScopeLogs().At(0)
	assert.Equal(t, library.Name(), thirdOutILL.Scope().Name())
	assert.Equal(t, thirdLog.Name(), thirdOutILL.LogRecords().At(0).Name())
}

func TestSplitLogsWithNilTraceID(t *testing.T) {
	// prepare
	inBatch := pdata.NewLogs()
	rl := inBatch.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()
	firstLog := sl.LogRecords().AppendEmpty()
	firstLog.SetTraceID(pdata.NewTraceID([16]byte{}))

	// test
	batches := SplitLogs(inBatch)

	// verify
	assert.Len(t, batches, 1)
	assert.Equal(t, pdata.NewTraceID([16]byte{}), batches[0].ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).TraceID())
}

func TestSplitLogsSameTraceIntoDifferentBatches(t *testing.T) {
	// prepare
	inBatch := pdata.NewLogs()
	rl := inBatch.ResourceLogs().AppendEmpty()

	// we have 1 ResourceLogs with 2 ILL, resulting in two batches
	rl.ScopeLogs().EnsureCapacity(2)

	// the first ILL has two logs
	firstILS := rl.ScopeLogs().AppendEmpty()
	firstLibrary := firstILS.Scope()
	firstLibrary.SetName("first-library")
	firstILS.LogRecords().EnsureCapacity(2)
	firstLog := firstILS.LogRecords().AppendEmpty()
	firstLog.SetName("first-batch-first-log")
	firstLog.SetTraceID(pdata.NewTraceID([16]byte{1, 2, 3, 4}))
	secondLog := firstILS.LogRecords().AppendEmpty()
	secondLog.SetName("first-batch-second-log")
	secondLog.SetTraceID(pdata.NewTraceID([16]byte{1, 2, 3, 4}))

	// the second ILL has one log
	secondILS := rl.ScopeLogs().AppendEmpty()
	secondLibrary := secondILS.Scope()
	secondLibrary.SetName("second-library")
	thirdLog := secondILS.LogRecords().AppendEmpty()
	thirdLog.SetName("second-batch-first-log")
	thirdLog.SetTraceID(pdata.NewTraceID([16]byte{1, 2, 3, 4}))

	// test
	batches := SplitLogs(inBatch)

	// verify
	assert.Len(t, batches, 2)

	// first batch
	assert.Equal(t, pdata.NewTraceID([16]byte{1, 2, 3, 4}), batches[0].ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).TraceID())
	assert.Equal(t, firstLibrary.Name(), batches[0].ResourceLogs().At(0).ScopeLogs().At(0).Scope().Name())
	assert.Equal(t, firstLog.Name(), batches[0].ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Name())
	assert.Equal(t, secondLog.Name(), batches[0].ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(1).Name())

	// second batch
	assert.Equal(t, pdata.NewTraceID([16]byte{1, 2, 3, 4}), batches[1].ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).TraceID())
	assert.Equal(t, secondLibrary.Name(), batches[1].ResourceLogs().At(0).ScopeLogs().At(0).Scope().Name())
	assert.Equal(t, thirdLog.Name(), batches[1].ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Name())
}
