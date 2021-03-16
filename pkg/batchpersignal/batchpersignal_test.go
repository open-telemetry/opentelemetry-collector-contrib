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
	"go.opentelemetry.io/collector/consumer/pdata"
)

func TestSplitDifferentTracesIntoDifferentBatches(t *testing.T) {
	// we have 1 ResourceSpans with 1 ILS and two traceIDs, resulting in two batches
	inBatch := pdata.NewTraces()
	inBatch.ResourceSpans().Resize(1)
	rs := inBatch.ResourceSpans().At(0)
	rs.InstrumentationLibrarySpans().Resize(1)

	// the first ILS has two spans
	ils := rs.InstrumentationLibrarySpans().At(0)
	library := ils.InstrumentationLibrary()
	library.SetName("first-library")
	ils.Spans().Resize(2)
	firstSpan := ils.Spans().At(0)
	firstSpan.SetName("first-batch-first-span")
	firstSpan.SetTraceID(pdata.NewTraceID([16]byte{1, 2, 3, 4}))
	secondSpan := ils.Spans().At(1)
	secondSpan.SetName("first-batch-second-span")
	secondSpan.SetTraceID(pdata.NewTraceID([16]byte{2, 3, 4, 5}))

	// test
	out := SplitTraces(inBatch)

	// verify
	assert.Len(t, out, 2)

	// first batch
	firstOutILS := out[0].ResourceSpans().At(0).InstrumentationLibrarySpans().At(0)
	assert.Equal(t, library.Name(), firstOutILS.InstrumentationLibrary().Name())
	assert.Equal(t, firstSpan.Name(), firstOutILS.Spans().At(0).Name())

	// second batch
	secondOutILS := out[1].ResourceSpans().At(0).InstrumentationLibrarySpans().At(0)
	assert.Equal(t, library.Name(), secondOutILS.InstrumentationLibrary().Name())
	assert.Equal(t, secondSpan.Name(), secondOutILS.Spans().At(0).Name())
}

func TestSplitDifferentLogsIntoDifferentBatches(t *testing.T) {
	// we have 1 ResourceLogs with 1 ILL and three traceIDs (one null) resulting in three batches
	inBatch := pdata.NewLogs()
	inBatch.ResourceLogs().Resize(1)
	rl := inBatch.ResourceLogs().At(0)
	rl.InstrumentationLibraryLogs().Resize(1)

	// the first ILL has three logs
	ill := rl.InstrumentationLibraryLogs().At(0)
	library := ill.InstrumentationLibrary()
	library.SetName("first-library")
	ill.Logs().Resize(3)
	firstLog := ill.Logs().At(0)
	firstLog.SetName("first-batch-first-log")
	firstLog.SetTraceID(pdata.NewTraceID([16]byte{1, 2, 3, 4}))
	secondLog := ill.Logs().At(1)
	secondLog.SetName("first-batch-second-log")
	secondLog.SetTraceID(pdata.NewTraceID([16]byte{2, 3, 4, 5}))
	thirdLog := ill.Logs().At(2)
	thirdLog.SetName("first-batch-third-log")
	// do not set traceID for third log

	// test
	out := SplitLogs(inBatch)

	// verify
	assert.Len(t, out, 3)

	// first batch
	firstOutILL := out[0].ResourceLogs().At(0).InstrumentationLibraryLogs().At(0)
	assert.Equal(t, library.Name(), firstOutILL.InstrumentationLibrary().Name())
	assert.Equal(t, firstLog.Name(), firstOutILL.Logs().At(0).Name())

	// second batch
	secondOutILL := out[1].ResourceLogs().At(0).InstrumentationLibraryLogs().At(0)
	assert.Equal(t, library.Name(), secondOutILL.InstrumentationLibrary().Name())
	assert.Equal(t, secondLog.Name(), secondOutILL.Logs().At(0).Name())

	// third batch
	thirdOutILL := out[2].ResourceLogs().At(0).InstrumentationLibraryLogs().At(0)
	assert.Equal(t, library.Name(), thirdOutILL.InstrumentationLibrary().Name())
	assert.Equal(t, thirdLog.Name(), thirdOutILL.Logs().At(0).Name())
}
