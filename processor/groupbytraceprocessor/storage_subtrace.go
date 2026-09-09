// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package groupbytraceprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/groupbytraceprocessor"

import (
	"sync"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/groupbytraceprocessor/internal/metadata"
)

// subtraceStorage buffers spans per (trace, service). It is used exclusively
// when EmitStrategy == EmitStrategyService.
type subtraceStorage interface {
	// insertSpan deep-copies and buffers one span under the given subtrace. A
	// span ID already held replaces the earlier copy, wherever it was held.
	insertSpan(subtraceID, spanContext, ptrace.Span) error

	// deleteSubtrace removes a service's buffered spans and returns them divided
	// into separate calls, so that one batch never stands for two calls to the
	// service that could be told apart.
	deleteSubtrace(subtraceID) ([][]*bufferedSpan, error)

	// subtraceIDs returns every subtrace currently held.
	subtraceIDs() []subtraceID

	start() error
	shutdown() error
}

var _ subtraceStorage = (*subtraceMemoryStorage)(nil)

// traceBuffer holds everything buffered for one trace: the spans grouped by
// service, and every span ID the trace has carried.
//
// spanIDs serves two purposes. Its keys record which span IDs this trace has
// been seen to contain, which is what lets a service-entry span be told apart
// from a span whose parent never arrived. Its values name the service currently
// holding each span, which keeps a span from being buffered under two services
// at once if it is resubmitted under a different resource.
//
// Entries deliberately outlive the spans themselves. Services are released one
// at a time, usually the caller before the callee, and a span whose parent has
// already gone would otherwise look parentless and lose its place. The whole
// buffer is discarded once no service holds anything, so the set lives no longer
// than the trace stays active.
type traceBuffer struct {
	services map[string]map[pcommon.SpanID]*bufferedSpan
	spanIDs  map[pcommon.SpanID]string
}

type subtraceMemoryStorage struct {
	sync.RWMutex
	traces    map[pcommon.TraceID]*traceBuffer
	telemetry *metadata.TelemetryBuilder
}

func newSubtraceMemoryStorage(telemetry *metadata.TelemetryBuilder) *subtraceMemoryStorage {
	return &subtraceMemoryStorage{
		traces:    make(map[pcommon.TraceID]*traceBuffer),
		telemetry: telemetry,
	}
}

func (s *subtraceMemoryStorage) insertSpan(id subtraceID, ctx spanContext, span ptrace.Span) error {
	bs := newBufferedSpan(ctx, span)

	s.Lock()
	defer s.Unlock()

	tb, ok := s.traces[id.traceID]
	if !ok {
		tb = &traceBuffer{
			services: make(map[string]map[pcommon.SpanID]*bufferedSpan),
			spanIDs:  make(map[pcommon.SpanID]string),
		}
		s.traces[id.traceID] = tb
	}

	spanID := bs.span.SpanID()
	// A resubmission naming a different service would otherwise leave the span
	// buffered under both, and so emitted twice.
	if previous, held := tb.spanIDs[spanID]; held && previous != id.serviceID {
		delete(tb.services[previous], spanID)
		if len(tb.services[previous]) == 0 {
			delete(tb.services, previous)
		}
	}

	spans, ok := tb.services[id.serviceID]
	if !ok {
		spans = make(map[pcommon.SpanID]*bufferedSpan)
		tb.services[id.serviceID] = spans
	}
	spans[spanID] = bs
	tb.spanIDs[spanID] = id.serviceID
	return nil
}

func (s *subtraceMemoryStorage) deleteSubtrace(id subtraceID) ([][]*bufferedSpan, error) {
	// Dividing into calls under the same write lock as the removal is what keeps a
	// concurrent release from observing, and so emitting, the same spans twice.
	s.Lock()
	defer s.Unlock()

	tb, ok := s.traces[id.traceID]
	if !ok {
		return nil, nil
	}
	spans, ok := tb.services[id.serviceID]
	if !ok {
		return nil, nil
	}

	calls := splitCalls(spans, tb.spanIDs)

	delete(tb.services, id.serviceID)
	if len(tb.services) == 0 {
		delete(s.traces, id.traceID)
	}
	return calls, nil
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

func (*subtraceMemoryStorage) start() error    { return nil }
func (*subtraceMemoryStorage) shutdown() error { return nil }
