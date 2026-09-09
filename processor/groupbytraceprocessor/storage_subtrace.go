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

	// childrenOf returns the buffered spans whose parent is one of the given span
	// IDs.
	childrenOf(pcommon.TraceID, []pcommon.SpanID) []pcommon.SpanID

	// subtracesFor returns the subtraces headed by the given candidate span IDs,
	// given everything currently buffered for the trace. Candidates that are no
	// longer buffered, or that head no subtrace, are skipped.
	subtracesFor(pcommon.TraceID, []pcommon.SpanID) []subtraceID

	// deleteSubtrace removes the spans belonging to the given subtrace and returns
	// them. It is a no-op returning no spans if nothing belongs to it any more,
	// which happens when its root has since acquired a parent and its spans have
	// moved to another subtrace.
	deleteSubtrace(subtraceID) ([]*bufferedSpan, error)

	// deleteUnclaimed removes and returns the spans of a trace that no local root
	// can collect, along with how many spans are still buffered for it afterwards.
	deleteUnclaimed(pcommon.TraceID) ([]*bufferedSpan, int, error)

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

func (s *subtraceMemoryStorage) childrenOf(traceID pcommon.TraceID, parents []pcommon.SpanID) []pcommon.SpanID {
	s.RLock()
	defer s.RUnlock()

	idx, ok := s.traces[traceID]
	if !ok {
		return nil
	}

	var children []pcommon.SpanID
	for _, parent := range parents {
		children = append(children, idx.children[parent]...)
	}
	return children
}

func (s *subtraceMemoryStorage) subtracesFor(traceID pcommon.TraceID, candidates []pcommon.SpanID) []subtraceID {
	s.RLock()
	defer s.RUnlock()

	idx, ok := s.traces[traceID]
	if !ok {
		return nil
	}

	var ids []subtraceID
	seen := map[subtraceID]struct{}{}
	for _, spanID := range candidates {
		bs, ok := idx.spans[spanID]
		if !ok {
			continue
		}
		id, heads := subtraceIDFor(traceID, bs, idx)
		if !heads {
			continue
		}
		// Several parentless spans of one service share a single group ID.
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids
}

// getSubtrace returns all bufferedSpans beneath rootID that are not separated
// from it by another local root. Read-only: it neither deletes nor checks that
// rootID is still a local root, so it reports what rootID would claim rather
// than what it is entitled to.
func (s *subtraceMemoryStorage) getSubtrace(id subtraceID) ([]*bufferedSpan, error) {
	s.RLock()
	defer s.RUnlock()

	idx, ok := s.traces[id.traceID]
	if !ok {
		return nil, nil
	}
	return subtraceMembers(id, idx), nil
}

func (s *subtraceMemoryStorage) deleteSubtrace(id subtraceID) ([]*bufferedSpan, error) {
	// Collect and remove under a single write lock: if the read and the delete
	// were separate, two concurrent releases of overlapping subtraces could both
	// observe, and therefore both emit, the same spans.
	s.Lock()
	defer s.Unlock()

	idx, ok := s.traces[id.traceID]
	if !ok {
		return nil, nil
	}

	// A span may have stopped being a local root since its timer was scheduled,
	// e.g. because its parent arrived in a later batch. Its spans then belong to
	// another subtrace, and subtraceMembers leaves them alone.
	if !id.isOrphanGroup() {
		bs, ok := idx.spans[id.spanID]
		if !ok || classifyRoot(bs, idx) != ownRoot {
			return nil, nil
		}
	}

	members := subtraceMembers(id, idx)
	if len(members) == 0 {
		return nil, nil
	}
	idx.remove(members)
	if idx.len() == 0 {
		delete(s.traces, id.traceID)
	}
	return members, nil
}

func (s *subtraceMemoryStorage) deleteUnclaimed(traceID pcommon.TraceID) ([]*bufferedSpan, int, error) {
	// Collecting and removing under one write lock, as deleteSubtrace does, so a
	// span cannot be classified as unclaimed and then be collected by a subtrace
	// that runs before the removal lands.
	s.Lock()
	defer s.Unlock()

	idx, ok := s.traces[traceID]
	if !ok {
		return nil, 0, nil
	}

	unclaimed := idx.unclaimed()
	idx.remove(unclaimed)
	remaining := idx.len()
	if remaining == 0 {
		delete(s.traces, traceID)
	}
	return unclaimed, remaining, nil
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
