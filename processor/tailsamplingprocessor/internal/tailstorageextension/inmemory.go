// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tailstorageextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/tailstorageextension"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

var _ TailStorage = (*inMemoryTailStorage)(nil)

type inMemoryTailStorage struct {
	idToSpans map[pcommon.TraceID]ptrace.Traces
}

func NewInMemoryTailStorage() TailStorage {
	return &inMemoryTailStorage{
		idToSpans: make(map[pcommon.TraceID]ptrace.Traces),
	}
}

func (s *inMemoryTailStorage) Append(traceID pcommon.TraceID, traceTd ptrace.Traces) error {
	td, ok := s.idToSpans[traceID]
	if !ok {
		td = ptrace.NewTraces()
		s.idToSpans[traceID] = td
	}
	for _, rss := range traceTd.ResourceSpans().All() {
		rss.MoveTo(td.ResourceSpans().AppendEmpty())
	}
	return nil
}

func (s *inMemoryTailStorage) Take(traceID pcommon.TraceID) (ptrace.Traces, error) {
	td, ok := s.idToSpans[traceID]
	if ok {
		delete(s.idToSpans, traceID)
		return td, nil
	}
	return ptrace.NewTraces(), nil
}

func (s *inMemoryTailStorage) Delete(traceID pcommon.TraceID) error {
	delete(s.idToSpans, traceID)
	return nil
}
