// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ptracetest // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/ptracetest"

import (
	"bytes"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil"
)

// CompareTracesOption can be used to mutate expected and/or actual traces before comparing.
type CompareTracesOption interface {
	applyOnTraces(expected, actual ptrace.Traces)
}

type compareTracesOptionFunc func(expected, actual ptrace.Traces)

func (f compareTracesOptionFunc) applyOnTraces(expected, actual ptrace.Traces) {
	f(expected, actual)
}

// IgnoreResourceAttributeValue is a CompareTracesOption that removes a resource attribute
// from all resources.
func IgnoreResourceAttributeValue(attributeName string) CompareTracesOption {
	return compareTracesOptionFunc(func(expected, actual ptrace.Traces) {
		maskTracesResourceAttributeValue(expected, attributeName)
		maskTracesResourceAttributeValue(actual, attributeName)
	})
}

func maskTracesResourceAttributeValue(traces ptrace.Traces, attributeName string) {
	rss := traces.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		internal.MaskResourceAttributeValue(rss.At(i).Resource(), attributeName)
	}
}

// IgnoreResourceSpansOrder is a CompareTracesOption that ignores the order of resource traces/metrics/logs.
func IgnoreResourceSpansOrder() CompareTracesOption {
	return compareTracesOptionFunc(func(expected, actual ptrace.Traces) {
		sortResourceSpansSlice(expected.ResourceSpans())
		sortResourceSpansSlice(actual.ResourceSpans())
	})
}

func sortResourceSpansSlice(rms ptrace.ResourceSpansSlice) {
	rms.Sort(func(a, b ptrace.ResourceSpans) bool {
		if a.SchemaUrl() != b.SchemaUrl() {
			return a.SchemaUrl() < b.SchemaUrl()
		}
		aAttrs := pdatautil.MapHash(a.Resource().Attributes())
		bAttrs := pdatautil.MapHash(b.Resource().Attributes())
		return bytes.Compare(aAttrs[:], bAttrs[:]) < 0
	})
}

// IgnoreScopeSpansOrder is a CompareTracesOption that ignores the order of instrumentation scope traces/metrics/logs.
func IgnoreScopeSpansOrder() CompareTracesOption {
	return compareTracesOptionFunc(func(expected, actual ptrace.Traces) {
		sortScopeSpansSlices(expected)
		sortScopeSpansSlices(actual)
	})
}

func sortScopeSpansSlices(ts ptrace.Traces) {
	for i := 0; i < ts.ResourceSpans().Len(); i++ {
		ts.ResourceSpans().At(i).ScopeSpans().Sort(func(a, b ptrace.ScopeSpans) bool {
			if a.SchemaUrl() != b.SchemaUrl() {
				return a.SchemaUrl() < b.SchemaUrl()
			}
			if a.Scope().Name() != b.Scope().Name() {
				return a.Scope().Name() < b.Scope().Name()
			}
			return a.Scope().Version() < b.Scope().Version()
		})
	}
}

// IgnoreSpansOrder is a CompareTracesOption that ignores the order of spans.
func IgnoreSpansOrder() CompareTracesOption {
	return compareTracesOptionFunc(func(expected, actual ptrace.Traces) {
		sortSpanSlices(expected)
		sortSpanSlices(actual)
	})
}

func sortSpanSlices(ts ptrace.Traces) {
	for i := 0; i < ts.ResourceSpans().Len(); i++ {
		for j := 0; j < ts.ResourceSpans().At(i).ScopeSpans().Len(); j++ {
			ts.ResourceSpans().At(i).ScopeSpans().At(j).Spans().Sort(func(a, b ptrace.Span) bool {
				if a.Kind() != b.Kind() {
					return a.Kind() < b.Kind()
				}
				if a.Name() != b.Name() {
					return a.Name() < b.Name()
				}
				at := a.TraceID()
				bt := b.TraceID()
				if !bytes.Equal(at[:], bt[:]) {
					return bytes.Compare(at[:], bt[:]) < 0
				}
				as := a.SpanID()
				bs := b.SpanID()
				if !bytes.Equal(as[:], bs[:]) {
					return bytes.Compare(as[:], bs[:]) < 0
				}
				aps := a.ParentSpanID()
				bps := b.ParentSpanID()
				if !bytes.Equal(aps[:], bps[:]) {
					return bytes.Compare(aps[:], bps[:]) < 0
				}
				aAttrs := pdatautil.MapHash(a.Attributes())
				bAttrs := pdatautil.MapHash(b.Attributes())
				if !bytes.Equal(aAttrs[:], bAttrs[:]) {
					return bytes.Compare(aAttrs[:], bAttrs[:]) < 0
				}
				if a.StartTimestamp() != b.StartTimestamp() {
					return a.StartTimestamp() < b.StartTimestamp()
				}
				return a.EndTimestamp() < b.EndTimestamp()
			})
		}
	}
}

// IgnoreSpanID is a CompareTracesOption that clears SpanID fields on all spans.
func IgnoreSpanID() CompareTracesOption {
	return compareTracesOptionFunc(func(expected, actual ptrace.Traces) {
		spanID := pcommon.NewSpanIDEmpty()
		maskSpanID(expected, spanID)
		maskSpanID(actual, spanID)
	})
}

func maskSpanID(traces ptrace.Traces, spanID pcommon.SpanID) {
	for i := 0; i < traces.ResourceSpans().Len(); i++ {
		rs := traces.ResourceSpans().At(i)
		for j := 0; j < rs.ScopeSpans().Len(); j++ {
			ss := rs.ScopeSpans().At(j)
			for k := 0; k < ss.Spans().Len(); k++ {
				span := ss.Spans().At(k)
				span.SetSpanID(spanID)
			}
		}
	}
}

// IgnoreSpanAttributeValue is a CompareTracesOption that clears value of the span attribute.
func IgnoreSpanAttributeValue(attributeName string) CompareTracesOption {
	return compareTracesOptionFunc(func(expected, actual ptrace.Traces) {
		maskSpanAttributeValue(expected, attributeName)
		maskSpanAttributeValue(actual, attributeName)
	})
}

func maskSpanAttributeValue(traces ptrace.Traces, attributeName string) {
	for i := 0; i < traces.ResourceSpans().Len(); i++ {
		rs := traces.ResourceSpans().At(i)
		for j := 0; j < rs.ScopeSpans().Len(); j++ {
			ss := rs.ScopeSpans().At(j)
			for k := 0; k < ss.Spans().Len(); k++ {
				span := ss.Spans().At(k)
				if _, ok := span.Attributes().Get(attributeName); ok {
					span.Attributes().PutStr(attributeName, "*")
				}
			}
		}
	}
}

// IgnoreStartTimestamp is a CompareTracesOption that clears StartTimestamp fields on all spans.
func IgnoreStartTimestamp() CompareTracesOption {
	return compareTracesOptionFunc(func(expected, actual ptrace.Traces) {
		now := pcommon.NewTimestampFromTime(time.Now())
		maskStartTimestamp(expected, now)
		maskStartTimestamp(actual, now)
	})
}

func maskStartTimestamp(traces ptrace.Traces, ts pcommon.Timestamp) {
	for i := 0; i < traces.ResourceSpans().Len(); i++ {
		rs := traces.ResourceSpans().At(i)
		for j := 0; j < rs.ScopeSpans().Len(); j++ {
			ss := rs.ScopeSpans().At(j)
			for k := 0; k < ss.Spans().Len(); k++ {
				span := ss.Spans().At(k)
				span.SetStartTimestamp(ts)
			}
		}
	}
}

// IgnoreEndTimestamp is a CompareTracesOption that clears EndTimestamp fields on all spans.
func IgnoreEndTimestamp() CompareTracesOption {
	return compareTracesOptionFunc(func(expected, actual ptrace.Traces) {
		now := pcommon.NewTimestampFromTime(time.Now())
		maskEndTimestamp(expected, now)
		maskEndTimestamp(actual, now)
	})
}

func maskEndTimestamp(traces ptrace.Traces, ts pcommon.Timestamp) {
	for i := 0; i < traces.ResourceSpans().Len(); i++ {
		rs := traces.ResourceSpans().At(i)
		for j := 0; j < rs.ScopeSpans().Len(); j++ {
			ss := rs.ScopeSpans().At(j)
			for k := 0; k < ss.Spans().Len(); k++ {
				span := ss.Spans().At(k)
				span.SetEndTimestamp(ts)
			}
		}
	}
}

// IgnoreTraceID is a CompareTracesOption that clears TraceID fields on all spans.
func IgnoreTraceID() CompareTracesOption {
	return compareTracesOptionFunc(func(expected, actual ptrace.Traces) {
		traceID := pcommon.NewTraceIDEmpty()
		maskTraceID(expected, traceID)
		maskTraceID(actual, traceID)
	})
}

func maskTraceID(traces ptrace.Traces, traceID pcommon.TraceID) {
	for i := 0; i < traces.ResourceSpans().Len(); i++ {
		rs := traces.ResourceSpans().At(i)
		for j := 0; j < rs.ScopeSpans().Len(); j++ {
			ss := rs.ScopeSpans().At(j)
			for k := 0; k < ss.Spans().Len(); k++ {
				span := ss.Spans().At(k)
				span.SetTraceID(traceID)
			}
		}
	}
}
