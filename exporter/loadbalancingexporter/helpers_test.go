// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"
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
	aspans.Resource().Attributes().PutStr("service.name", "service-name-1")
	aspans.ScopeSpans().AppendEmpty().Spans().AppendEmpty().SetTraceID([16]byte{1, 2, 3, 4})
	bspans := expectedTraces.ResourceSpans().AppendEmpty()
	bspans.Resource().Attributes().PutStr("service.name", "service-name-2")
	bspans.ScopeSpans().AppendEmpty().Spans().AppendEmpty().SetTraceID([16]byte{1, 2, 3, 2})
	cspans := expectedTraces.ResourceSpans().AppendEmpty()
	cspans.Resource().Attributes().PutStr("service.name", "service-name-3")
	cspans.ScopeSpans().AppendEmpty().Spans().AppendEmpty().SetTraceID([16]byte{1, 2, 3, 3})

	trace1 := ptrace.NewTraces()
	trace1.ResourceSpans().EnsureCapacity(2)
	t1aspans := trace1.ResourceSpans().AppendEmpty()
	t1aspans.Resource().Attributes().PutStr("service.name", "service-name-1")
	t1aspans.ScopeSpans().AppendEmpty().Spans().AppendEmpty().SetTraceID([16]byte{1, 2, 3, 4})
	t1bspans := trace1.ResourceSpans().AppendEmpty()
	t1bspans.Resource().Attributes().PutStr("service.name", "service-name-2")
	t1bspans.ScopeSpans().AppendEmpty().Spans().AppendEmpty().SetTraceID([16]byte{1, 2, 3, 2})

	trace2 := ptrace.NewTraces()
	trace2.ResourceSpans().EnsureCapacity(1)
	t2cspans := trace2.ResourceSpans().AppendEmpty()
	t2cspans.Resource().Attributes().PutStr("service.name", "service-name-3")
	t2cspans.ScopeSpans().AppendEmpty().Spans().AppendEmpty().SetTraceID([16]byte{1, 2, 3, 3})

	mergedTraces := mergeTraces(trace1, trace2)

	require.Equal(t, expectedTraces, mergedTraces)
}

func benchMergeTraces(b *testing.B, tracesCount int) {
	traces1 := ptrace.NewTraces()
	traces2 := ptrace.NewTraces()

	for range tracesCount {
		appendSimpleTraceWithID(traces2.ResourceSpans().AppendEmpty(), [16]byte{1, 2, 3, 4})
	}

	for b.Loop() {
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

func TestMergeLogsTwoEmpty(t *testing.T) {
	expectedEmpty := plog.NewLogs()
	logs1 := plog.NewLogs()
	logs2 := plog.NewLogs()

	mergedLogs := mergeLogs(logs1, logs2)

	require.Equal(t, expectedEmpty, mergedLogs)
}

func TestMergeLogsSingleEmpty(t *testing.T) {
	expectedLogs := simpleLogs()

	logs1 := plog.NewLogs()
	logs2 := simpleLogs()

	mergedLogs := mergeLogs(logs1, logs2)

	require.Equal(t, expectedLogs, mergedLogs)
}

func TestMergeLogs(t *testing.T) {
	expectedLogs := plog.NewLogs()
	expectedLogs.ResourceLogs().EnsureCapacity(3)
	arl := expectedLogs.ResourceLogs().AppendEmpty()
	arl.Resource().Attributes().PutStr("service.name", "service-name-1")
	arl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("log-1")
	brl := expectedLogs.ResourceLogs().AppendEmpty()
	brl.Resource().Attributes().PutStr("service.name", "service-name-2")
	brl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("log-2")
	crl := expectedLogs.ResourceLogs().AppendEmpty()
	crl.Resource().Attributes().PutStr("service.name", "service-name-3")
	crl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("log-3")

	logs1 := plog.NewLogs()
	logs1.ResourceLogs().EnsureCapacity(2)
	l1arl := logs1.ResourceLogs().AppendEmpty()
	l1arl.Resource().Attributes().PutStr("service.name", "service-name-1")
	l1arl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("log-1")
	l1brl := logs1.ResourceLogs().AppendEmpty()
	l1brl.Resource().Attributes().PutStr("service.name", "service-name-2")
	l1brl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("log-2")

	logs2 := plog.NewLogs()
	logs2.ResourceLogs().EnsureCapacity(1)
	l2crl := logs2.ResourceLogs().AppendEmpty()
	l2crl.Resource().Attributes().PutStr("service.name", "service-name-3")
	l2crl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("log-3")

	mergedLogs := mergeLogs(logs1, logs2)

	require.Equal(t, expectedLogs, mergedLogs)
}

func benchMergeLogs(b *testing.B, logsCount int) {
	logs1 := plog.NewLogs()
	logs2 := plog.NewLogs()

	for range logsCount {
		rl := logs2.ResourceLogs().AppendEmpty()
		rl.Resource().Attributes().PutStr("service.name", "test-service")
		rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr("test log")
	}

	for b.Loop() {
		mergeLogs(logs1, logs2)
	}
}

func BenchmarkMergeLogs_X100(b *testing.B) {
	benchMergeLogs(b, 100)
}

func BenchmarkMergeLogs_X500(b *testing.B) {
	benchMergeLogs(b, 500)
}

func BenchmarkMergeLogs_X1000(b *testing.B) {
	benchMergeLogs(b, 1000)
}
