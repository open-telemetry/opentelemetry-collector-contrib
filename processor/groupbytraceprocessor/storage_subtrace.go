// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package groupbytraceprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/groupbytraceprocessor"

import (
	"sync"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/groupbytraceprocessor/internal/metadata"
)

// subtraceStorage buffers spans per (trace, service). It is used exclusively
// when EmitStrategy == EmitStrategyService.
type subtraceStorage interface {
	// insertScopeSpans deep-copies all spans in ss and buffers them under the
	// given subtrace, stamping each with arrivedAt, the time the batch carrying
	// them was received. The copy is made before the lock is held so the caller
	// can recycle its batch. Every span is kept independently; duplicate span
	// IDs, including the empty span ID, are all emitted as separate spans.
	insertScopeSpans(id subtraceID, ctx *spanContext, ss ptrace.ScopeSpans, arrivedAt time.Time) error

	// releaseDue removes and returns the service's calls whose first span arrived
	// at or before cutoff, each as its own slice, together with the first arrival
	// among the calls left behind. That time is zero when nothing is left, and is
	// otherwise what the next release should be scheduled from.
	releaseDue(subtraceID, time.Time) ([][]*bufferedSpan, time.Time, error)

	// deleteSubtrace removes a service's buffered spans however recently they
	// arrived, and returns them divided into separate calls. It is used where
	// waiting any longer isn't an option: eviction and shutdown.
	deleteSubtrace(subtraceID) ([][]*bufferedSpan, error)

	// subtraceIDs returns every subtrace currently held.
	subtraceIDs() []subtraceID

	// count returns the number of (trace, service) pairs currently buffered.
	count() int

	start() error
	shutdown() error
}

var _ subtraceStorage = (*subtraceMemoryStorage)(nil)

// traceBuffer holds everything buffered for one trace: the spans grouped by
// service, and every span ID the trace has carried.
//
// spanIDs records which span IDs this trace has been seen to contain, which is
// what lets a service-entry span be told apart from a span whose parent never
// arrived. Its values name one service that has held each span (the last seen).
//
// Entries deliberately outlive the spans themselves. Services are released one
// at a time, usually the caller before the callee, and a span whose parent has
// already gone would otherwise look parentless and lose its place.
//
// They do not outlive their usefulness, though: see forgetUnreferencedSpanIDs.
type traceBuffer struct {
	services map[[16]byte]map[pcommon.SpanID][]*bufferedSpan
	spanIDs  map[pcommon.SpanID][16]byte
}

// liveSpans counts the spans the trace still holds, across every service.
func (tb *traceBuffer) liveSpans() int {
	n := 0
	for _, spans := range tb.services {
		for _, bsList := range spans {
			n += len(bsList)
		}
	}
	return n
}

// forgetUnreferencedSpanIDs drops the record of span IDs that are neither
// buffered any more nor named as a parent by something that is.
//
// A span's ID cannot simply be forgotten as the span is emitted: services are
// released one at a time, usually the caller before the callee, and the callee's
// entry spans are recognizable only because they point at spans the caller has
// already taken with it. What can be forgotten is an ID nothing points at any
// more, because the span is then far enough in the past that anything arriving
// for it now is better treated as parentless.
//
// Without this a trace that always has some service buffered would accumulate
// every span ID it ever carried.
func (tb *traceBuffer) forgetUnreferencedSpanIDs() {
	kept := make(map[pcommon.SpanID][16]byte, len(tb.spanIDs))
	for service, spans := range tb.services {
		for spanID := range spans {
			kept[spanID] = service
		}
	}
	for _, spans := range tb.services {
		for _, bsList := range spans {
			for _, bs := range bsList {
				parent := bs.span.ParentSpanID()
				if _, alreadyKept := kept[parent]; alreadyKept {
					continue
				}
				if service, known := tb.spanIDs[parent]; known {
					kept[parent] = service
				}
			}
		}
	}
	tb.spanIDs = kept
}

type subtraceMemoryStorage struct {
	sync.RWMutex
	traces    map[pcommon.TraceID]*traceBuffer
	telemetry *metadata.TelemetryBuilder
	n         int // number of (trace, service) pairs currently buffered
}

func newSubtraceMemoryStorage(telemetry *metadata.TelemetryBuilder) *subtraceMemoryStorage {
	return &subtraceMemoryStorage{
		traces:    make(map[pcommon.TraceID]*traceBuffer),
		telemetry: telemetry,
	}
}

func (s *subtraceMemoryStorage) insertScopeSpans(id subtraceID, ctx *spanContext, ss ptrace.ScopeSpans, arrivedAt time.Time) error {
	if ss.Spans().Len() == 0 {
		return nil
	}

	ssCopy := ptrace.NewScopeSpans()
	ss.Spans().CopyTo(ssCopy.Spans())

	s.Lock()
	defer s.Unlock()

	tb, ok := s.traces[id.traceID]
	if !ok {
		tb = &traceBuffer{
			services: make(map[[16]byte]map[pcommon.SpanID][]*bufferedSpan),
			spanIDs:  make(map[pcommon.SpanID][16]byte),
		}
		s.traces[id.traceID] = tb
	}

	spans, ok := tb.services[id.serviceID]
	if !ok {
		spans = make(map[pcommon.SpanID][]*bufferedSpan)
		tb.services[id.serviceID] = spans
		s.n++
	}

	for i := range ssCopy.Spans().Len() {
		span := ssCopy.Spans().At(i)
		spanID := span.SpanID()
		bs := &bufferedSpan{spanContext: ctx, span: span, arrivedAt: arrivedAt}
		spans[spanID] = append(spans[spanID], bs)
		tb.spanIDs[spanID] = id.serviceID
	}
	return nil
}

func (s *subtraceMemoryStorage) releaseDue(id subtraceID, cutoff time.Time) ([][]*bufferedSpan, time.Time, error) {
	s.Lock()
	defer s.Unlock()
	return s.takeLocked(id, cutoff)
}

func (s *subtraceMemoryStorage) deleteSubtrace(id subtraceID) ([][]*bufferedSpan, error) {
	s.Lock()
	defer s.Unlock()
	// A cutoff no arrival can be after takes everything.
	calls, _, err := s.takeLocked(id, time.Now().Add(time.Hour*1_000_000))
	return calls, err
}

// takeLocked divides a service's spans into calls and removes the ones due at
// cutoff, returning them along with the first arrival among those left.
//
// Dividing into calls under the same write lock as the removal is what keeps a
// concurrent release from observing, and so emitting, the same spans twice.
func (s *subtraceMemoryStorage) takeLocked(id subtraceID, cutoff time.Time) ([][]*bufferedSpan, time.Time, error) {
	tb, ok := s.traces[id.traceID]
	if !ok {
		return nil, time.Time{}, nil
	}
	spans, ok := tb.services[id.serviceID]
	if !ok {
		return nil, time.Time{}, nil
	}

	var due [][]*bufferedSpan
	var nextArrival time.Time

	if countCallHeads(spans) <= 1 {
		// Fast path: at most one call head means all spans belong to one call.
		// Skip the splitCalls walk (and its callOf map allocation) entirely.
		first := firstArrivalInMap(spans)
		if first.After(cutoff) {
			nextArrival = first
		} else {
			var all []*bufferedSpan
			for _, bsList := range spans {
				all = append(all, bsList...)
			}
			due = append(due, all)
			for spanID := range spans {
				delete(spans, spanID)
			}
		}
	} else {
		for _, call := range splitCalls(spans, tb.spanIDs) {
			first := firstArrival(call)
			if first.After(cutoff) {
				// This call started later than the one whose timer just fired, so it
				// has time left on its own.
				if nextArrival.IsZero() || first.Before(nextArrival) {
					nextArrival = first
				}
				continue
			}
			due = append(due, call)
			for _, bs := range call {
				delete(spans, bs.span.SpanID())
			}
		}
	}

	if len(spans) == 0 {
		delete(tb.services, id.serviceID)
		s.n--
		if len(tb.services) == 0 {
			delete(s.traces, id.traceID)
			return due, nextArrival, nil
		}
	}

	// Rebuilding costs a pass over everything the trace still holds, so only do it
	// once at least half the record is spans that have come and gone. Running it
	// on every release would scale that pass with the number of services a trace
	// passes through. Retention is bounded either way, at twice what is buffered.
	if live := tb.liveSpans(); len(tb.spanIDs) > 2*live {
		tb.forgetUnreferencedSpanIDs()
	}

	return due, nextArrival, nil
}

// countCallHeads returns the number of spans in `spans` that head a call: spans
// whose parent is not among the service's own spans, or that report a remote
// parent context. This mirrors the entry-detection logic in splitCalls.
func countCallHeads(spans map[pcommon.SpanID][]*bufferedSpan) int {
	n := 0
	for _, bsList := range spans {
		bs := bsList[len(bsList)-1]
		parent := bs.span.ParentSpanID()
		if hasRemoteParent(bs) || parent.IsEmpty() {
			n++
			continue
		}
		if _, inService := spans[parent]; !inService {
			n++
		}
	}
	return n
}

// firstArrivalInMap returns the earliest arrivedAt across all buffered spans in
// the map. Used by the single-call fast path in takeLocked.
func firstArrivalInMap(spans map[pcommon.SpanID][]*bufferedSpan) time.Time {
	var first time.Time
	for _, bsList := range spans {
		for _, bs := range bsList {
			if first.IsZero() || bs.arrivedAt.Before(first) {
				first = bs.arrivedAt
			}
		}
	}
	return first
}

func (s *subtraceMemoryStorage) subtraceIDs() []subtraceID {
	s.RLock()
	defer s.RUnlock()

	var ids []subtraceID
	for traceID, tb := range s.traces {
		for serviceID := range tb.services {
			ids = append(ids, subtraceID{traceID: traceID, serviceID: serviceID})
		}
	}
	return ids
}

// count returns the number of (trace, service) pairs currently buffered.
func (s *subtraceMemoryStorage) count() int {
	s.RLock()
	defer s.RUnlock()
	return s.n
}

func (*subtraceMemoryStorage) start() error    { return nil }
func (*subtraceMemoryStorage) shutdown() error { return nil }
