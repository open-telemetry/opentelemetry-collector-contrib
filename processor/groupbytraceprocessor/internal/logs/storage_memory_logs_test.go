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

// nolint:errcheck
package logs

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

func TestMemoryCreateAndGetLog(t *testing.T) {
	// prepare
	st := NewMemoryStorage()

	traceIDs := []pcommon.TraceID{
		pcommon.NewTraceID([16]byte{1, 2, 3, 4}),
		pcommon.NewTraceID([16]byte{2, 3, 4, 5}),
	}

	baseLog := plog.NewLogs()
	rss := baseLog.ResourceLogs()
	rs := rss.AppendEmpty()
	ils := rs.ScopeLogs().AppendEmpty()
	span := ils.LogRecords().AppendEmpty()

	// test
	for _, traceID := range traceIDs {
		span.SetTraceID(traceID)
		assert.NoError(t, st.createOrAppend(traceID, baseLog))
	}

	// verify
	assert.Equal(t, 2, st.count())
	for _, traceID := range traceIDs {
		expected := []plog.ResourceLogs{baseLog.ResourceLogs().At(0)}
		expected[0].ScopeLogs().At(0).LogRecords().At(0).SetTraceID(traceID)

		retrieved, err := st.get(traceID)
		assert.NoError(t, st.createOrAppend(traceID, baseLog))

		require.NoError(t, err)
		assert.Equal(t, expected, retrieved)
	}
}

func TestMemoryDeleteLog(t *testing.T) {
	// prepare
	st := NewMemoryStorage()

	traceID := pcommon.NewTraceID([16]byte{1, 2, 3, 4})

	log := plog.NewLogs()
	rss := log.ResourceLogs()
	rs := rss.AppendEmpty()
	ils := rs.ScopeLogs().AppendEmpty()
	span := ils.LogRecords().AppendEmpty()
	span.SetTraceID(traceID)

	assert.NoError(t, st.createOrAppend(traceID, log))

	// test
	deleted, err := st.delete(traceID)

	// verify
	require.NoError(t, err)
	assert.Equal(t, []plog.ResourceLogs{log.ResourceLogs().At(0)}, deleted)

	retrieved, err := st.get(traceID)
	require.NoError(t, err)
	assert.Nil(t, retrieved)
}

func TestMemoryAppendLogRecords(t *testing.T) {
	// prepare
	st := NewMemoryStorage()

	traceID := pcommon.NewTraceID([16]byte{1, 2, 3, 4})

	log := plog.NewLogs()
	rss := log.ResourceLogs()
	rs := rss.AppendEmpty()
	ils := rs.ScopeLogs().AppendEmpty()
	span := ils.LogRecords().AppendEmpty()
	span.SetTraceID(traceID)
	span.SetSpanID(pcommon.NewSpanID([8]byte{1, 2, 3, 4}))

	assert.NoError(t, st.createOrAppend(traceID, log))

	secondLog := plog.NewLogs()
	secondRss := secondLog.ResourceLogs()
	secondRs := secondRss.AppendEmpty()
	secondIls := secondRs.ScopeLogs().AppendEmpty()
	secondLogRecord := secondIls.LogRecords().AppendEmpty()
	secondLogRecord.Body().SetStringVal("second-name")
	secondLogRecord.SetTraceID(traceID)
	secondLogRecord.SetSpanID(pcommon.NewSpanID([8]byte{5, 6, 7, 8}))

	expected := []plog.ResourceLogs{
		plog.NewResourceLogs(),
		plog.NewResourceLogs(),
	}
	ils.CopyTo(expected[0].ScopeLogs().AppendEmpty())
	secondIls.CopyTo(expected[1].ScopeLogs().AppendEmpty())

	// test
	err := st.createOrAppend(traceID, secondLog)
	require.NoError(t, err)

	// override something in the second span, to make sure we are storing a copy
	secondLogRecord.Body().SetStringVal("changed-second-name")

	// verify
	retrieved, err := st.get(traceID)
	require.NoError(t, err)
	require.Len(t, retrieved, 2)
	assert.Equal(t, "second-name", retrieved[1].ScopeLogs().At(0).LogRecords().At(0).Body().StringVal())

	// now that we checked that the secondLogRecord change here didn't have an effect, revert
	// so that we can compare the that everything else has the same value
	secondLogRecord.Body().SetStringVal("second-name")
	assert.Equal(t, expected, retrieved)
}

func TestMemoryLogIsBeingCloned(t *testing.T) {
	// prepare
	st := NewMemoryStorage()
	traceID := pcommon.NewTraceID([16]byte{1, 2, 3, 4})

	log := plog.NewLogs()
	rss := log.ResourceLogs()
	rs := rss.AppendEmpty()
	ils := rs.ScopeLogs().AppendEmpty()
	span := ils.LogRecords().AppendEmpty()
	span.SetTraceID(traceID)
	span.SetSpanID(pcommon.NewSpanID([8]byte{1, 2, 3, 4}))
	span.Body().SetStringVal("should-not-be-changed")

	// test
	err := st.createOrAppend(traceID, log)
	require.NoError(t, err)
	span.Body().SetStringVal("changed-log")

	// verify
	retrieved, err := st.get(traceID)
	require.NoError(t, err)
	assert.Equal(t, "should-not-be-changed", retrieved[0].ScopeLogs().At(0).LogRecords().At(0).Body().StringVal())
}
