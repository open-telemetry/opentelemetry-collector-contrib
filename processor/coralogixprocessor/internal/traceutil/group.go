// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package traceutil // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/coralogixprocessor/internal/traceutil"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/otel/semconv/v1.40.0"
)

// GroupSpansByTraceID collects spans from a traces payload by trace ID.
func GroupSpansByTraceID(td ptrace.Traces) map[pcommon.TraceID][]ptrace.Span {
	traceSpanMap := make(map[pcommon.TraceID][]ptrace.Span)
	for i := 0; i < td.ResourceSpans().Len(); i++ {
		rs := td.ResourceSpans().At(i)
		for j := 0; j < rs.ScopeSpans().Len(); j++ {
			scopeSpans := rs.ScopeSpans().At(j)
			for k := 0; k < scopeSpans.Spans().Len(); k++ {
				span := scopeSpans.Spans().At(k)
				traceID := span.TraceID()
				traceSpanMap[traceID] = append(traceSpanMap[traceID], span)
			}
		}
	}
	return traceSpanMap
}

// ServiceNameIndex resolves each span's resource `service.name` cheaply:
// each resource block's name is read and stored once, and every span ID
// maps to a 4-byte ordinal into that table rather than duplicating the
// string per span.
type ServiceNameIndex struct {
	names    []string
	bySpanID map[pcommon.SpanID]uint32
}

// ServiceName returns the span's resource service.name
func (idx ServiceNameIndex) ServiceName(id pcommon.SpanID) (string, bool) {
	ord, ok := idx.bySpanID[id]
	if !ok {
		return "", false
	}
	return idx.names[ord], true
}

// ServiceNamesBySpanID resolves each span's resource `service.name`
func ServiceNamesBySpanID(td ptrace.Traces) ServiceNameIndex {
	idx := ServiceNameIndex{bySpanID: make(map[pcommon.SpanID]uint32, td.SpanCount())}
	for i := 0; i < td.ResourceSpans().Len(); i++ {
		rs := td.ResourceSpans().At(i)
		nameVal, ok := rs.Resource().Attributes().Get(string(conventions.ServiceNameKey))
		if !ok {
			continue
		}
		ord := uint32(len(idx.names))
		idx.names = append(idx.names, nameVal.Str())
		for j := 0; j < rs.ScopeSpans().Len(); j++ {
			scopeSpans := rs.ScopeSpans().At(j)
			for k := 0; k < scopeSpans.Spans().Len(); k++ {
				idx.bySpanID[scopeSpans.Spans().At(k).SpanID()] = ord
			}
		}
	}
	return idx
}

// GroupSpansByTraceIDWithServiceNames does the combined work of
// GroupSpansByTraceID and ServiceNamesBySpanID in a single pass over td,
// for callers that need both (avoids walking the ResourceSpans/ScopeSpans
// tree twice).
func GroupSpansByTraceIDWithServiceNames(td ptrace.Traces) (map[pcommon.TraceID][]ptrace.Span, ServiceNameIndex) {
	traceSpanMap := make(map[pcommon.TraceID][]ptrace.Span)
	idx := ServiceNameIndex{bySpanID: make(map[pcommon.SpanID]uint32, td.SpanCount())}
	for i := 0; i < td.ResourceSpans().Len(); i++ {
		rs := td.ResourceSpans().At(i)
		nameVal, hasServiceName := rs.Resource().Attributes().Get(string(conventions.ServiceNameKey))
		var ord uint32
		if hasServiceName {
			ord = uint32(len(idx.names))
			idx.names = append(idx.names, nameVal.Str())
		}
		for j := 0; j < rs.ScopeSpans().Len(); j++ {
			scopeSpans := rs.ScopeSpans().At(j)
			for k := 0; k < scopeSpans.Spans().Len(); k++ {
				span := scopeSpans.Spans().At(k)
				traceSpanMap[span.TraceID()] = append(traceSpanMap[span.TraceID()], span)
				if hasServiceName {
					idx.bySpanID[span.SpanID()] = ord
				}
			}
		}
	}
	return traceSpanMap, idx
}
