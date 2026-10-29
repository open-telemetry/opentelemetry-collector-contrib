// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package clickhouseexporter

import (
	"fmt"
	"testing"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouseexporter/internal"
)

// tracesWithNEventsAndLinks creates a realistic traces payload where each span
// has the given number of events and links, each with attributes. This models
// a real-world scenario better than single-element test data.
func tracesWithNEventsAndLinks(spans, eventsPerSpan, linksPerSpan int) ptrace.Traces {
	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("service.name", "bench-service")
	rs.Resource().Attributes().PutStr("host.name", "bench-host")
	rs.Resource().Attributes().PutStr("deployment.environment", "production")
	ss := rs.ScopeSpans().AppendEmpty()
	ss.Scope().SetName("io.opentelemetry.contrib.clickhouse")
	ss.Scope().SetVersion("1.0.0")
	ss.Scope().Attributes().PutStr("lib", "clickhouse")
	timestamp := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	for i := range spans {
		s := ss.Spans().AppendEmpty()
		s.SetTraceID([16]byte{1, 2, 3, byte(i)})
		s.SetSpanID([8]byte{1, 2, 3, byte(i)})
		s.SetParentSpanID([8]byte{1, 2, 4, byte(i)})
		s.TraceState().FromRaw("key=value")
		s.SetName("db.query")
		s.SetKind(ptrace.SpanKindClient)
		s.SetStartTimestamp(pcommon.NewTimestampFromTime(timestamp))
		s.SetEndTimestamp(pcommon.NewTimestampFromTime(timestamp.Add(50 * time.Millisecond)))
		s.Attributes().PutStr("db.system", "postgresql")
		s.Attributes().PutStr("db.statement", "SELECT * FROM users WHERE id = $1")
		s.Attributes().PutStr("net.peer.name", "db.example.com")
		s.Status().SetCode(ptrace.StatusCodeOk)

		for e := range eventsPerSpan {
			event := s.Events().AppendEmpty()
			event.SetName("log")
			event.SetTimestamp(pcommon.NewTimestampFromTime(timestamp.Add(time.Duration(e) * time.Millisecond)))
			event.Attributes().PutStr("level", "info")
			event.Attributes().PutStr("message", "executing query")
			event.Attributes().PutInt("attempt", int64(e))
		}

		for l := range linksPerSpan {
			link := s.Links().AppendEmpty()
			link.SetTraceID([16]byte{2, 3, 4, byte(l)})
			link.SetSpanID([8]byte{2, 3, 4, byte(l)})
			link.TraceState().FromRaw("linked=true")
			link.Attributes().PutStr("link.type", "parent")
			link.Attributes().PutStr("link.source", "upstream")
		}
	}

	return traces
}

func BenchmarkConvertEvents(b *testing.B) {
	for _, numEvents := range []int{1, 5, 10} {
		traces := tracesWithNEventsAndLinks(1, numEvents, 0)
		events := traces.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Events()
		b.Run(fmt.Sprintf("events=%d", numEvents), func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				convertEvents(events)
			}
		})
	}
}

func BenchmarkConvertLinks(b *testing.B) {
	for _, numLinks := range []int{1, 5, 10} {
		traces := tracesWithNEventsAndLinks(1, 0, numLinks)
		links := traces.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Links()
		b.Run(fmt.Sprintf("links=%d", numLinks), func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				convertLinks(links)
			}
		})
	}
}

// BenchmarkTracesBatchPrep benchmarks the full data preparation path for a
// batch of spans, including AttributesToMap and convert functions. This doesn't
// include the actual batch.Append (which needs a DB), but covers all the
// allocation-heavy work that happens before it.
func BenchmarkTracesBatchPrep(b *testing.B) {
	for _, numSpans := range []int{100, 1000} {
		traces := tracesWithNEventsAndLinks(numSpans, 3, 2)
		b.Run(fmt.Sprintf("spans=%d", numSpans), func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				prepareTracesBatch(traces)
			}
		})
	}
}

// prepareTracesBatch simulates the data preparation loop from pushTraceData
// without the actual DB interaction.
func prepareTracesBatch(td ptrace.Traces) {
	rsSpans := td.ResourceSpans()
	for i := range rsSpans.Len() {
		spans := rsSpans.At(i)
		res := spans.Resource()
		resAttr := res.Attributes()
		_ = internal.GetServiceName(resAttr)
		_ = internal.AttributesToMap(res.Attributes())

		for j := range spans.ScopeSpans().Len() {
			scopeSpanRoot := spans.ScopeSpans().At(j)
			scopeSpans := scopeSpanRoot.Spans()

			for k := range scopeSpans.Len() {
				span := scopeSpans.At(k)
				_ = internal.AttributesToMap(span.Attributes())
				convertEvents(span.Events())
				convertLinks(span.Links())
			}
		}
	}
}
