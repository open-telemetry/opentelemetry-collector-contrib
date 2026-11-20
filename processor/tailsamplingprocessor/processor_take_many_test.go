// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tailsamplingprocessor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// singleTakeStorage implements TailStorage but not BatchTailStorage.
type singleTakeStorage struct {
	spans     map[pcommon.TraceID]ptrace.Traces
	takeCalls int
}

func (s *singleTakeStorage) Append(id pcommon.TraceID, td ptrace.Traces) error {
	s.spans[id] = td
	return nil
}

func (s *singleTakeStorage) Take(id pcommon.TraceID) (ptrace.Traces, error) {
	s.takeCalls++
	td, ok := s.spans[id]
	if !ok {
		return ptrace.NewTraces(), nil
	}
	delete(s.spans, id)
	return td, nil
}

func (s *singleTakeStorage) Delete(id pcommon.TraceID) error {
	delete(s.spans, id)
	return nil
}

func TestTakeAllFallsBackToTake(t *testing.T) {
	st := &singleTakeStorage{spans: map[pcommon.TraceID]ptrace.Traces{}}
	idA, idB := pcommon.TraceID{1}, pcommon.TraceID{2}
	for _, id := range []pcommon.TraceID{idA, idB} {
		td := ptrace.NewTraces()
		td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
		require.NoError(t, st.Append(id, td))
	}
	tsp := &tailSamplingSpanProcessor{tailStorage: st}

	out, errs := tsp.takeAll([]pcommon.TraceID{idB, idA})
	require.Len(t, out, 2)
	require.Len(t, errs, 2)
	assert.NoError(t, errs[0])
	assert.NoError(t, errs[1])
	assert.Equal(t, 1, out[0].ResourceSpans().Len())
	assert.Equal(t, 1, out[1].ResourceSpans().Len())
	assert.Equal(t, 2, st.takeCalls)
}
