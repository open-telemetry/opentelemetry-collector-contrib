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

package groupbytraceprocessor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/pdata"
)

func TestMemoryCreateAndGetTrace(t *testing.T) {
	// prepare
	st := newMemoryStorage()

	traceIDs := []pdata.TraceID{
		pdata.NewTraceID([16]byte{1, 2, 3, 4}),
		pdata.NewTraceID([16]byte{2, 3, 4, 5}),
	}

	baseTrace := pdata.NewResourceSpans()
	ils := baseTrace.InstrumentationLibrarySpans().AppendEmpty()
	span := ils.Spans().AppendEmpty()

	// test
	for _, traceID := range traceIDs {
		span.SetTraceID(traceID)
		st.createOrAppend(traceID, baseTrace)
	}

	// verify
	assert.Equal(t, 2, st.count())
	for _, traceID := range traceIDs {
		expected := []pdata.ResourceSpans{baseTrace}
		expected[0].InstrumentationLibrarySpans().At(0).Spans().At(0).SetTraceID(traceID)

		retrieved, err := st.get(traceID)
		st.createOrAppend(traceID, expected[0])

		require.NoError(t, err)
		assert.Equal(t, expected, retrieved)
	}
}

func TestMemoryDeleteTrace(t *testing.T) {
	// prepare
	st := newMemoryStorage()

	traceID := pdata.NewTraceID([16]byte{1, 2, 3, 4})

	trace := pdata.NewResourceSpans()
	ils := trace.InstrumentationLibrarySpans().AppendEmpty()
	span := ils.Spans().AppendEmpty()
	span.SetTraceID(traceID)

	st.createOrAppend(traceID, trace)

	// test
	deleted, err := st.delete(traceID)

	// verify
	require.NoError(t, err)
	assert.Equal(t, []pdata.ResourceSpans{trace}, deleted)

	retrieved, err := st.get(traceID)
	require.NoError(t, err)
	assert.Nil(t, retrieved)
}

func TestMemoryAppendSpans(t *testing.T) {
	// prepare
	st := newMemoryStorage()

	traceID := pdata.NewTraceID([16]byte{1, 2, 3, 4})

	batch := pdata.NewResourceSpans()
	ils := batch.InstrumentationLibrarySpans().AppendEmpty()
	span := ils.Spans().AppendEmpty()
	span.SetTraceID(traceID)
	span.SetSpanID(pdata.NewSpanID([8]byte{1, 2, 3, 4}))

	st.createOrAppend(traceID, batch)

	secondBatch := pdata.NewResourceSpans()
	secondIls := secondBatch.InstrumentationLibrarySpans().AppendEmpty()
	secondSpan := secondIls.Spans().AppendEmpty()
	secondSpan.SetName("second-name")
	secondSpan.SetTraceID(traceID)
	secondSpan.SetSpanID(pdata.NewSpanID([8]byte{5, 6, 7, 8}))

	expected := []pdata.ResourceSpans{
		pdata.NewResourceSpans(),
		pdata.NewResourceSpans(),
	}
	ils.CopyTo(expected[0].InstrumentationLibrarySpans().AppendEmpty())
	secondIls.CopyTo(expected[1].InstrumentationLibrarySpans().AppendEmpty())

	// test
	err := st.createOrAppend(traceID, secondBatch)
	require.NoError(t, err)

	// override something in the second span, to make sure we are storing a copy
	secondSpan.SetName("changed-second-name")

	// verify
	retrieved, err := st.get(traceID)
	require.NoError(t, err)
	assert.Equal(t, "second-name", retrieved[1].InstrumentationLibrarySpans().At(0).Spans().At(0).Name())

	// now that we checked that the secondSpan change here didn't have an effect, revert
	// so that we can compare the that everything else has the same value
	secondSpan.SetName("second-name")
	assert.Equal(t, expected, retrieved)
}

func TestMemoryTraceIsBeingCloned(t *testing.T) {
	// prepare
	st := newMemoryStorage()
	traceID := pdata.NewTraceID([16]byte{1, 2, 3, 4})

	batch := pdata.NewResourceSpans()
	ils := batch.InstrumentationLibrarySpans().AppendEmpty()
	span := ils.Spans().AppendEmpty()
	span.SetTraceID(traceID)
	span.SetSpanID(pdata.NewSpanID([8]byte{1, 2, 3, 4}))
	span.SetName("should-not-be-changed")

	// test
	err := st.createOrAppend(traceID, batch)
	require.NoError(t, err)
	span.SetName("changed-trace")

	// verify
	retrieved, err := st.get(traceID)
	require.NoError(t, err)
	assert.Equal(t, "should-not-be-changed", retrieved[0].InstrumentationLibrarySpans().At(0).Spans().At(0).Name())
}
