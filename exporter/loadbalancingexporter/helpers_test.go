// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/otel/semconv/v1.27.0"
)

func TestMergeTracesTwoEmpty(t *testing.T) {
	expectedEmpty := ptrace.NewTraces()
	trace1 := ptrace.NewTraces()
	trace2 := ptrace.NewTraces()

	mergedTraces := mergeTraces(trace1, trace2)

	require.Equal(t, expectedEmpty, mergedTraces)
}

func TestMergeTracesSingleEmpty(t *testing.T) {
	expectedTraces := simpleTraces()

	trace1 := ptrace.NewTraces()
	trace2 := simpleTraces()

	mergedTraces := mergeTraces(trace1, trace2)

	require.Equal(t, expectedTraces, mergedTraces)
}

func TestMergeTraces(t *testing.T) {
	expectedTraces := ptrace.NewTraces()
	expectedTraces.ResourceSpans().EnsureCapacity(3)
	aspans := expectedTraces.ResourceSpans().AppendEmpty()
	aspans.Resource().Attributes().PutStr(string(conventions.ServiceNameKey), "service-name-1")
	aspans.ScopeSpans().AppendEmpty().Spans().AppendEmpty().SetTraceID([16]byte{1, 2, 3, 4})
	bspans := expectedTraces.ResourceSpans().AppendEmpty()
	bspans.Resource().Attributes().PutStr(string(conventions.ServiceNameKey), "service-name-2")
	bspans.ScopeSpans().AppendEmpty().Spans().AppendEmpty().SetTraceID([16]byte{1, 2, 3, 2})
	cspans := expectedTraces.ResourceSpans().AppendEmpty()
	cspans.Resource().Attributes().PutStr(string(conventions.ServiceNameKey), "service-name-3")
	cspans.ScopeSpans().AppendEmpty().Spans().AppendEmpty().SetTraceID([16]byte{1, 2, 3, 3})

	trace1 := ptrace.NewTraces()
	trace1.ResourceSpans().EnsureCapacity(2)
	t1aspans := trace1.ResourceSpans().AppendEmpty()
	t1aspans.Resource().Attributes().PutStr(string(conventions.ServiceNameKey), "service-name-1")
	t1aspans.ScopeSpans().AppendEmpty().Spans().AppendEmpty().SetTraceID([16]byte{1, 2, 3, 4})
	t1bspans := trace1.ResourceSpans().AppendEmpty()
	t1bspans.Resource().Attributes().PutStr(string(conventions.ServiceNameKey), "service-name-2")
	t1bspans.ScopeSpans().AppendEmpty().Spans().AppendEmpty().SetTraceID([16]byte{1, 2, 3, 2})

	trace2 := ptrace.NewTraces()
	trace2.ResourceSpans().EnsureCapacity(1)
	t2cspans := trace2.ResourceSpans().AppendEmpty()
	t2cspans.Resource().Attributes().PutStr(string(conventions.ServiceNameKey), "service-name-3")
	t2cspans.ScopeSpans().AppendEmpty().Spans().AppendEmpty().SetTraceID([16]byte{1, 2, 3, 3})

	mergedTraces := mergeTraces(trace1, trace2)

	require.Equal(t, expectedTraces, mergedTraces)
}

func benchMergeTraces(b *testing.B, tracesCount int) {
	traces1 := ptrace.NewTraces()
	traces2 := ptrace.NewTraces()

	for i := 0; i < tracesCount; i++ {
		appendSimpleTraceWithID(traces2.ResourceSpans().AppendEmpty(), [16]byte{1, 2, 3, 4})
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		mergeTraces(traces1, traces2)
	}
}

func BenchmarkMergeTraces_X100(b *testing.B) {
	benchMergeTraces(b, 100)
}

func BenchmarkMergeTraces_X500(b *testing.B) {
	benchMergeTraces(b, 500)
}

func BenchmarkMergeTraces_X1000(b *testing.B) {
	benchMergeTraces(b, 1000)
}
