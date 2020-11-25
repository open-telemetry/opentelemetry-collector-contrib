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

package batchpertrace

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/consumer/pdata"
)

func TestSplitDifferentTracesIntoDifferentBatches(t *testing.T) {
	// we have 1 ResourceSpans with 1 ILS and two traceIDs, resulting in two batches
	rs := pdata.NewResourceSpans()
	rs.InitEmpty()
	rs.InstrumentationLibrarySpans().Resize(1)

	// the first ILS has two spans
	ils := rs.InstrumentationLibrarySpans().At(0)
	library := ils.InstrumentationLibrary()
	library.InitEmpty()
	library.SetName("first-library")
	ils.Spans().Resize(2)
	firstSpan := ils.Spans().At(0)
	firstSpan.SetName("first-batch-first-span")
	firstSpan.SetTraceID(pdata.NewTraceID([16]byte{1, 2, 3, 4}))
	secondSpan := ils.Spans().At(1)
	secondSpan.SetName("first-batch-second-span")
	secondSpan.SetTraceID(pdata.NewTraceID([16]byte{2, 3, 4, 5}))

	inBatch := pdata.NewTraces()
	inBatch.ResourceSpans().Append(rs)

	// test
	out := Split(inBatch)

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
