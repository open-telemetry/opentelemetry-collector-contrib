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
	// insertSpan deep-copies and indexes one span under the given context, which
	// the caller builds once per (resource, scope) pair and reuses for its spans.
	insertSpan(pcommon.TraceID, spanContext, ptrace.Span) error

	// localRoots returns which of the given candidate span IDs are local root
	// spans, given everything currently buffered for the trace. Candidates that
	// are no longer buffered are skipped.
	localRoots(pcommon.TraceID, []pcommon.SpanID) []pcommon.SpanID

	// deleteSubtrace removes the spans that belong to rootID's subtrace and
	// returns them. It is a no-op returning no spans if rootID is no longer a
	// local root, since those spans belong to another subtrace.
	deleteSubtrace(pcommon.TraceID, pcommon.SpanID) ([]*bufferedSpan, error)

	// traceIDs returns all trace IDs currently held in storage.
	traceIDs() []pcommon.TraceID

	// deleteTrace removes every span still buffered for a trace and returns them,
	// so that spans no subtrace claimed can be flushed rather than lost.
	deleteTrace(pcommon.TraceID) ([]*bufferedSpan, error)

	start() error
	shutdown() error
}

var _ subtraceStorage = (*subtraceMemoryStorage)(nil)

type subtraceMemoryStorage struct {
	sync.RWMutex
	// traces maps traceID → the spans buffered for that trace
	traces    map[pcommon.TraceID]*traceIndex
	telemetry *metadata.TelemetryBuilder
}

var _ subtraceStorage = (*subtraceMemoryStorage)(nil)

func newSubtraceMemoryStorage(telemetry *metadata.TelemetryBuilder) *subtraceMemoryStorage {
	return &subtraceMemoryStorage{
		traces:    make(map[pcommon.TraceID]*traceIndex),
		telemetry: telemetry,
	}
}

func (s *subtraceMemoryStorage) insertSpan(traceID pcommon.TraceID, ctx spanContext, span ptrace.Span) error {
	bs := newBufferedSpan(ctx, span)

	s.Lock()
	defer s.Unlock()

	idx, ok := s.traces[traceID]
	if !ok {
		idx = newTraceIndex()
		s.traces[traceID] = idx
	}
	idx.insert(bs)
	return nil
}

func (s *subtraceMemoryStorage) localRoots(traceID pcommon.TraceID, candidates []pcommon.SpanID) []pcommon.SpanID {
	s.RLock()
	defer s.RUnlock()

	idx, ok := s.traces[traceID]
	if !ok {
		return nil
	}

	var roots []pcommon.SpanID
	for _, spanID := range candidates {
		bs, ok := idx.spans[spanID]
		if ok && isLocalRoot(bs, idx) {
			roots = append(roots, spanID)
		}
	}
	return roots
}

// getSubtrace returns all bufferedSpans beneath rootID that are not separated
// from it by another local root. Read-only: it neither deletes nor checks that
// rootID is still a local root, so it reports what rootID would claim rather
// than what it is entitled to.
func (s *subtraceMemoryStorage) getSubtrace(traceID pcommon.TraceID, rootID pcommon.SpanID) ([]*bufferedSpan, error) {
	s.RLock()
	defer s.RUnlock()

	idx, ok := s.traces[traceID]
	if !ok {
		return nil, nil
	}
	return subtraceMembers(rootID, idx), nil
}

func (s *subtraceMemoryStorage) deleteSubtrace(traceID pcommon.TraceID, rootID pcommon.SpanID) ([]*bufferedSpan, error) {
	// Collect and remove under a single write lock: if the read and the delete
	// were separate, two concurrent releases of overlapping subtraces could both
	// observe, and therefore both emit, the same spans.
	s.Lock()
	defer s.Unlock()

	idx, ok := s.traces[traceID]
	if !ok {
		return nil, nil
	}

	// The span may have stopped being a local root since its timer was scheduled,
	// e.g. because its parent arrived in a later batch. Leave its spans in place
	// so they are released with the subtrace they actually belong to.
	bs, ok := idx.spans[rootID]
	if !ok || !isLocalRoot(bs, idx) {
		return nil, nil
	}

	members := subtraceMembers(rootID, idx)
	idx.remove(members)
	if idx.len() == 0 {
		delete(s.traces, traceID)
	}
	return members, nil
}

// getRemainder returns all spans still buffered for a trace, i.e. those not yet
// claimed by a subtrace.
func (s *subtraceMemoryStorage) getRemainder(traceID pcommon.TraceID) ([]*bufferedSpan, error) {
	s.RLock()
	defer s.RUnlock()

	idx, ok := s.traces[traceID]
	if !ok {
		return nil, nil
	}

	members := make([]*bufferedSpan, 0, idx.len())
	for _, bs := range idx.spans {
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

func (s *subtraceMemoryStorage) deleteTrace(traceID pcommon.TraceID) ([]*bufferedSpan, error) {
	// Collecting and removing under one write lock keeps a span inserted alongside
	// this call from being deleted without ever being returned to anyone.
	s.Lock()
	defer s.Unlock()

	idx, ok := s.traces[traceID]
	if !ok {
		return nil, nil
	}
	members := make([]*bufferedSpan, 0, idx.len())
	for _, bs := range idx.spans {
		members = append(members, bs)
	}
	delete(s.traces, traceID)
	return members, nil
}

func (*subtraceMemoryStorage) start() error    { return nil }
func (*subtraceMemoryStorage) shutdown() error { return nil }
