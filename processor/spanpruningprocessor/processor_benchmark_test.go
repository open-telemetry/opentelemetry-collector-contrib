// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package spanpruningprocessor

import (
	"context"
	"fmt"
	"testing"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/spanpruningprocessor/internal/metadata"
)

// BenchmarkProcessTrace_SmallTrace benchmarks processing a small trace (10 spans)
func BenchmarkProcessTrace_SmallTrace(b *testing.B) {
	benchmarkProcessTrace(b, 10, 5)
}

// BenchmarkProcessTrace_MediumTrace benchmarks processing a medium trace (100 spans)
func BenchmarkProcessTrace_MediumTrace(b *testing.B) {
	benchmarkProcessTrace(b, 100, 20)
}

// BenchmarkProcessTrace_LargeTrace benchmarks processing a large trace (1000 spans)
func BenchmarkProcessTrace_LargeTrace(b *testing.B) {
	benchmarkProcessTrace(b, 1000, 50)
}

// benchmarkProcessTrace is a helper for benchmarking trace processing with different sizes
func benchmarkProcessTrace(b *testing.B, numSpans, minSpans int) {
	cfg := createDefaultConfig().(*Config)
	cfg.MinSpansToAggregate = minSpans
	cfg.GroupByAttributes = []string{"http.*"}

	proc, err := newSpanPruningProcessor(processortest.NewNopSettings(metadata.Type), cfg)
	if err != nil {
		b.Fatal(err)
	}

	// Generate test trace
	td := generateTestTrace(numSpans, minSpans)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Clone the trace for each iteration
		cloned := ptrace.NewTraces()
		td.CopyTo(cloned)
		_, err := proc.processTraces(context.Background(), cloned)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// generateTestTrace creates a test trace with the specified number of spans
func generateTestTrace(numSpans, leafSpansPerParent int) ptrace.Traces {
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()

	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})

	// Create root span
	rootSpan := ss.Spans().AppendEmpty()
	rootSpan.SetTraceID(traceID)
	rootSpan.SetSpanID(pcommon.SpanID([8]byte{0, 0, 0, 0, 0, 0, 0, 1}))
	rootSpan.SetName("root")
	rootSpan.SetKind(ptrace.SpanKindServer)
	rootSpan.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	rootSpan.SetEndTimestamp(pcommon.NewTimestampFromTime(time.Now().Add(time.Second)))
	rootSpan.Status().SetCode(ptrace.StatusCodeOk)

	spanID := uint64(2)
	numParents := (numSpans - 1) / leafSpansPerParent
	if numParents < 1 {
		numParents = 1
	}

	// Create parent spans
	parentSpanIDs := make([]pcommon.SpanID, numParents)
	for i := 0; i < numParents && spanID < uint64(numSpans); i++ {
		parentSpan := ss.Spans().AppendEmpty()
		parentSpan.SetTraceID(traceID)
		id := pcommon.SpanID([8]byte{0, 0, 0, 0, 0, 0, byte(spanID >> 8), byte(spanID)})
		parentSpan.SetSpanID(id)
		parentSpan.SetParentSpanID(rootSpan.SpanID())
		parentSpan.SetName(fmt.Sprintf("parent-%d", i))
		parentSpan.SetKind(ptrace.SpanKindInternal)
		parentSpan.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		parentSpan.SetEndTimestamp(pcommon.NewTimestampFromTime(time.Now().Add(100 * time.Millisecond)))
		parentSpan.Status().SetCode(ptrace.StatusCodeOk)
		parentSpanIDs[i] = id
		spanID++
	}

	// Create leaf spans
	for i := 0; spanID < uint64(numSpans+1); i++ {
		parentIdx := i % len(parentSpanIDs)
		leafSpan := ss.Spans().AppendEmpty()
		leafSpan.SetTraceID(traceID)
		id := pcommon.SpanID([8]byte{0, 0, 0, 0, 0, 0, byte(spanID >> 8), byte(spanID)})
		leafSpan.SetSpanID(id)
		leafSpan.SetParentSpanID(parentSpanIDs[parentIdx])
		leafSpan.SetName("leaf-operation")
		leafSpan.SetKind(ptrace.SpanKindClient)
		leafSpan.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		leafSpan.SetEndTimestamp(pcommon.NewTimestampFromTime(time.Now().Add(10 * time.Millisecond)))
		leafSpan.Status().SetCode(ptrace.StatusCodeOk)

		// Add some attributes
		leafSpan.Attributes().PutStr("http.method", "GET")
		leafSpan.Attributes().PutStr("http.url", "/api/data")

		spanID++
	}

	return td
}

// generateTestSpans creates test span infos for benchmarking
func generateTestSpans(numSpans, leafSpansPerParent int) []spanInfo {
	td := generateTestTrace(numSpans, leafSpansPerParent)
	spans := make([]spanInfo, 0, numSpans)

	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		ilss := rss.At(i).ScopeSpans()
		for j := 0; j < ilss.Len(); j++ {
			ss := ilss.At(j)
			for k := 0; k < ss.Spans().Len(); k++ {
				spans = append(spans, spanInfo{
					span:       ss.Spans().At(k),
					scopeSpans: ss,
				})
			}
		}
	}

	return spans
}
