// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package groupbytraceprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/groupbytraceprocessor"

import (
	"sync"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/groupbytraceprocessor/internal/metadata"
)

// subtraceStorage persists spans at individual-span granularity, keyed by
// (traceID, spanID). It is used exclusively when EmitStrategy == EmitStrategyService.
type subtraceStorage interface {
	// insertSpan deep-copies and indexes one span together with its resource and scope.
	insertSpan(pcommon.TraceID, pcommon.Resource, pcommon.InstrumentationScope, ptrace.Span) error

	// localRoots returns which of the given candidate span IDs are local root
	// spans, given everything currently buffered for the trace. Candidates that
	// are no longer buffered are skipped.
	localRoots(pcommon.TraceID, []pcommon.SpanID) []pcommon.SpanID

	// deleteSubtrace removes the spans that belong to rootID's subtrace and
	// returns them. It is a no-op returning no spans if rootID is no longer a
	// local root, since those spans belong to another subtrace.
	deleteSubtrace(pcommon.TraceID, pcommon.SpanID) ([]bufferedSpan, error)

	// traceIDs returns all trace IDs currently held in storage.
	traceIDs() []pcommon.TraceID

	// deleteTrace removes every span still buffered for a trace and returns them,
	// so that spans no subtrace claimed can be flushed rather than lost.
	deleteTrace(pcommon.TraceID) ([]bufferedSpan, error)

	start() error
	shutdown() error
}

var _ subtraceStorage = (*subtraceMemoryStorage)(nil)

type subtraceMemoryStorage struct {
	sync.RWMutex
	// traces maps traceID → (spanID → bufferedSpan)
	traces    map[pcommon.TraceID]map[pcommon.SpanID]bufferedSpan
	telemetry *metadata.TelemetryBuilder
}

var _ subtraceStorage = (*subtraceMemoryStorage)(nil)

func newSubtraceMemoryStorage(telemetry *metadata.TelemetryBuilder) *subtraceMemoryStorage {
	return &subtraceMemoryStorage{
		traces:    make(map[pcommon.TraceID]map[pcommon.SpanID]bufferedSpan),
		telemetry: telemetry,
	}
}

func (s *subtraceMemoryStorage) insertSpan(
	traceID pcommon.TraceID,
	resource pcommon.Resource,
	scope pcommon.InstrumentationScope,
	span ptrace.Span,
) error {
	bs := newBufferedSpan(resource, scope, span)

	s.Lock()
	defer s.Unlock()

	if _, ok := s.traces[traceID]; !ok {
		s.traces[traceID] = make(map[pcommon.SpanID]bufferedSpan)
	}
	s.traces[traceID][bs.span.SpanID()] = bs
	return nil
}

func (s *subtraceMemoryStorage) localRoots(traceID pcommon.TraceID, candidates []pcommon.SpanID) []pcommon.SpanID {
	s.RLock()
	defer s.RUnlock()

	index, ok := s.traces[traceID]
	if !ok {
		return nil
	}

	var roots []pcommon.SpanID
	for _, spanID := range candidates {
		bs, ok := index[spanID]
		if ok && isLocalRoot(bs, index) {
			roots = append(roots, spanID)
		}
	}
	return roots
}

// getSubtrace returns all bufferedSpans whose ancestor chain reaches rootID
// without crossing another local-root boundary. Read-only: it neither deletes
// nor checks that rootID is still a local root, so it reports what rootID would
// claim rather than what it is entitled to.
func (s *subtraceMemoryStorage) getSubtrace(traceID pcommon.TraceID, rootID pcommon.SpanID) ([]bufferedSpan, error) {
	s.RLock()
	defer s.RUnlock()

	index, ok := s.traces[traceID]
	if !ok {
		return nil, nil
	}
	return subtraceMembers(rootID, index), nil
}

func (s *subtraceMemoryStorage) deleteSubtrace(traceID pcommon.TraceID, rootID pcommon.SpanID) ([]bufferedSpan, error) {
	// Collect and remove under a single write lock: if the read and the delete
	// were separate, two concurrent releases of overlapping subtraces could both
	// observe, and therefore both emit, the same spans.
	s.Lock()
	defer s.Unlock()

	index, ok := s.traces[traceID]
	if !ok {
		return nil, nil
	}

	// The span may have stopped being a local root since its timer was scheduled,
	// e.g. because its parent arrived in a later batch. Leave its spans in place
	// so they are released with the subtrace they actually belong to.
	bs, ok := index[rootID]
	if !ok || !isLocalRoot(bs, index) {
		return nil, nil
	}

	members := subtraceMembers(rootID, index)
	for _, m := range members {
		delete(index, m.span.SpanID())
	}
	if len(index) == 0 {
		delete(s.traces, traceID)
	}
	return members, nil
}

// getRemainder returns all spans still buffered for a trace, i.e. those not yet
// claimed by a subtrace.
func (s *subtraceMemoryStorage) getRemainder(traceID pcommon.TraceID) ([]bufferedSpan, error) {
	s.RLock()
	defer s.RUnlock()

	index, ok := s.traces[traceID]
	if !ok {
		return nil, nil
	}

	members := make([]bufferedSpan, 0, len(index))
	for _, bs := range index {
		members = append(members, bs)
	}
	return members, nil
}

func (s *subtraceMemoryStorage) traceIDs() []pcommon.TraceID {
	s.RLock()
	defer s.RUnlock()
	ids := make([]pcommon.TraceID, 0, len(s.traces))
	for id := range s.traces {
		ids = append(ids, id)
	}
	return ids
}

func (s *subtraceMemoryStorage) deleteTrace(traceID pcommon.TraceID) ([]bufferedSpan, error) {
	// Collecting and removing under one write lock keeps a span inserted alongside
	// this call from being deleted without ever being returned to anyone.
	s.Lock()
	defer s.Unlock()

	index, ok := s.traces[traceID]
	if !ok {
		return nil, nil
	}
	members := make([]bufferedSpan, 0, len(index))
	for _, bs := range index {
		members = append(members, bs)
	}
	delete(s.traces, traceID)
	return members, nil
}

func (*subtraceMemoryStorage) start() error    { return nil }
func (*subtraceMemoryStorage) shutdown() error { return nil }
