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

	// localRoots returns the span IDs of all local root spans currently buffered
	// for the given trace.
	localRoots(pcommon.TraceID) []pcommon.SpanID

	// getSubtrace returns all bufferedSpans whose ancestor chain reaches rootID
	// without crossing another local-root boundary. Read-only; does not delete.
	getSubtrace(pcommon.TraceID, pcommon.SpanID) ([]bufferedSpan, error)

	// deleteSubtrace removes the spans that belong to rootID's subtrace and
	// returns them.
	deleteSubtrace(pcommon.TraceID, pcommon.SpanID) ([]bufferedSpan, error)

	// getRemainder returns all spans still buffered for a trace (those not yet
	// claimed by a subtrace timer). Used by the shutdown drain.
	getRemainder(pcommon.TraceID) ([]bufferedSpan, error)

	// traceIDs returns all trace IDs currently held in storage.
	traceIDs() []pcommon.TraceID

	// deleteTrace removes all remaining spans for a trace.
	deleteTrace(pcommon.TraceID) error

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
	// Deep-copy all three fields so each bufferedSpan is self-contained and the
	// caller can recycle its pdata objects. Because spans are indexed flat by
	// SpanID rather than grouped by resource/scope, spans that share a resource
	// or scope each get their own independent copy.
	rCopy := pcommon.NewResource()
	resource.CopyTo(rCopy)

	sCopy := pcommon.NewInstrumentationScope()
	scope.CopyTo(sCopy)

	spCopy := ptrace.NewSpan()
	span.CopyTo(spCopy)

	bs := bufferedSpan{resource: rCopy, scope: sCopy, span: spCopy}

	s.Lock()
	defer s.Unlock()

	if _, ok := s.traces[traceID]; !ok {
		s.traces[traceID] = make(map[pcommon.SpanID]bufferedSpan)
	}
	s.traces[traceID][spCopy.SpanID()] = bs
	return nil
}

func (s *subtraceMemoryStorage) localRoots(traceID pcommon.TraceID) []pcommon.SpanID {
	s.RLock()
	defer s.RUnlock()

	index, ok := s.traces[traceID]
	if !ok {
		return nil
	}

	var roots []pcommon.SpanID
	for spanID, bs := range index {
		if isLocalRoot(bs, index) {
			roots = append(roots, spanID)
		}
	}
	return roots
}

func (s *subtraceMemoryStorage) getSubtrace(traceID pcommon.TraceID, rootID pcommon.SpanID) ([]bufferedSpan, error) {
	s.RLock()
	defer s.RUnlock()

	index, ok := s.traces[traceID]
	if !ok {
		return nil, nil
	}

	var members []bufferedSpan
	for spanID, bs := range index {
		if spanID == rootID || reaches(spanID, rootID, index) {
			members = append(members, bs)
		}
	}
	return members, nil
}

func (s *subtraceMemoryStorage) deleteSubtrace(traceID pcommon.TraceID, rootID pcommon.SpanID) ([]bufferedSpan, error) {
	members, err := s.getSubtrace(traceID, rootID)
	if err != nil || len(members) == 0 {
		return members, err
	}

	s.Lock()
	defer s.Unlock()

	index, ok := s.traces[traceID]
	if !ok {
		return members, nil
	}
	for _, bs := range members {
		delete(index, bs.span.SpanID())
	}
	if len(index) == 0 {
		delete(s.traces, traceID)
	}
	return members, nil
}

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

func (s *subtraceMemoryStorage) deleteTrace(traceID pcommon.TraceID) error {
	s.Lock()
	defer s.Unlock()
	delete(s.traces, traceID)
	return nil
}

func (*subtraceMemoryStorage) start() error    { return nil }
func (*subtraceMemoryStorage) shutdown() error { return nil }
