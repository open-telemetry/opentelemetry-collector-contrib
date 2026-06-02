// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package coralogixprocessor

import (
	"testing"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

func BenchmarkProcessTraces_BothFeatures_DeepChain(b *testing.B) {
	benchmarkProcessTraces(b, benchmarkTraceDeepChain(2000))
}

func BenchmarkProcessTraces_BothFeatures_BranchyTree(b *testing.B) {
	benchmarkProcessTraces(b, benchmarkTraceBranchyTree(7, 4))
}

func benchmarkProcessTraces(b *testing.B, traces ptrace.Traces) {
	processor := &coralogixProcessor{
		config: &Config{
			TransactionsConfig: TransactionsConfig{Enabled: true},
			CriticalPathConfig: CriticalPathConfig{Enabled: true},
		},
		logger: zap.NewNop(),
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		td := ptrace.NewTraces()
		traces.CopyTo(td)
		_, err := processor.processTraces(b.Context(), td)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func benchmarkTraceDeepChain(depth int) ptrace.Traces {
	traces := ptrace.NewTraces()
	spans := traces.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans()
	traceID := pcommon.TraceID([16]byte{42, 42, 42, 42, 42, 42, 42, 42, 42, 42, 42, 42, 42, 42, 42, 42})

	for i := range depth {
		span := spans.AppendEmpty()
		span.SetTraceID(traceID)
		span.SetSpanID(benchmarkSpanID(uint64(i + 1)))
		if i > 0 {
			span.SetParentSpanID(benchmarkSpanID(uint64(i)))
		}
		span.SetName("span-" + string(rune('a'+(i%26))))
		span.SetStartTimestamp(pcommon.Timestamp(i))
		span.SetEndTimestamp(pcommon.Timestamp(depth))
	}

	return traces
}

func benchmarkTraceBranchyTree(levels, fanout int) ptrace.Traces {
	traces := ptrace.NewTraces()
	spans := traces.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans()
	traceID := pcommon.TraceID([16]byte{24, 24, 24, 24, 24, 24, 24, 24, 24, 24, 24, 24, 24, 24, 24, 24})

	type queueItem struct {
		id       uint64
		level    int
		startNS  int64
		endNS    int64
		parentID uint64
	}

	nextID := uint64(1)
	queue := []queueItem{{id: nextID, level: 0, startNS: 0, endNS: int64(levels*fanout*20 + 100)}}
	nextID++

	for len(queue) > 0 {
		item := queue[0]
		queue = queue[1:]

		span := spans.AppendEmpty()
		span.SetTraceID(traceID)
		span.SetSpanID(benchmarkSpanID(item.id))
		if item.parentID != 0 {
			span.SetParentSpanID(benchmarkSpanID(item.parentID))
		}
		span.SetName("node")
		span.SetStartTimestamp(pcommon.Timestamp(item.startNS))
		span.SetEndTimestamp(pcommon.Timestamp(item.endNS))

		if item.level == levels-1 {
			continue
		}

		childWindow := (item.endNS - item.startNS) / int64(fanout+1)
		for childIdx := range fanout {
			startNS := item.startNS + int64(childIdx+1)*childWindow/2
			endNS := startNS + childWindow
			queue = append(queue, queueItem{
				id:       nextID,
				level:    item.level + 1,
				startNS:  startNS,
				endNS:    endNS,
				parentID: item.id,
			})
			nextID++
		}
	}

	return traces
}

func benchmarkSpanID(id uint64) pcommon.SpanID {
	return pcommon.SpanID([8]byte{
		byte(id >> 56),
		byte(id >> 48),
		byte(id >> 40),
		byte(id >> 32),
		byte(id >> 24),
		byte(id >> 16),
		byte(id >> 8),
		byte(id),
	})
}
