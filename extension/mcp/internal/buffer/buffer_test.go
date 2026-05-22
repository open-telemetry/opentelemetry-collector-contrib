// Copyright The OpenTelemetry Authors
// Copyright 2025 Austin Parker
// SPDX-License-Identifier: Apache-2.0

package buffer

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func TestBufferAdd(t *testing.T) {
	b := New(5, 5, 5)

	td := ptrace.NewTraces()
	b.AddTraces(td)

	md := pmetric.NewMetrics()
	b.AddMetrics(md)

	ld := plog.NewLogs()
	b.AddLogs(ld)

	stats := b.GetStats()
	assert.Equal(t, 1, stats.TracesCount)
	assert.Equal(t, 1, stats.MetricsCount)
	assert.Equal(t, 1, stats.LogsCount)
}

func TestBufferGetRecent(t *testing.T) {
	b := New(10, 10, 10)

	// Add 5 items
	for range 5 {
		b.AddTraces(ptrace.NewTraces())
		b.AddMetrics(pmetric.NewMetrics())
		b.AddLogs(plog.NewLogs())
	}

	// Get all items
	traces := b.GetRecentTraces(10, 0)
	assert.Len(t, traces, 5)

	metrics := b.GetRecentMetrics(10, 0)
	assert.Len(t, metrics, 5)

	logs := b.GetRecentLogs(10, 0)
	assert.Len(t, logs, 5)
}

func TestBufferCapacity(t *testing.T) {
	capacity := 3
	b := New(capacity, capacity, capacity)

	// Add more items than capacity
	for range 5 {
		b.AddTraces(ptrace.NewTraces())
	}

	// Should only keep capacity amount
	traces := b.GetRecentTraces(10, 0)
	assert.Len(t, traces, capacity)

	stats := b.GetStats()
	assert.Equal(t, capacity, stats.TracesCount)
	assert.Equal(t, capacity, stats.TracesCapacity)
}

func TestBufferLimitAndOffset(t *testing.T) {
	b := New(10, 10, 10)

	// Add 5 items
	for range 5 {
		b.AddTraces(ptrace.NewTraces())
	}

	// Get with limit
	traces := b.GetRecentTraces(2, 0)
	assert.Len(t, traces, 2)

	// Get with offset
	traces = b.GetRecentTraces(5, 1)
	assert.Len(t, traces, 4)

	// Get with both limit and offset
	traces = b.GetRecentTraces(2, 1)
	assert.Len(t, traces, 2)
}

func TestBufferConcurrentAccess(t *testing.T) {
	b := New(100, 100, 100)

	var wg sync.WaitGroup
	numGoroutines := 10
	itemsPerGoroutine := 10

	// Concurrent writes
	for range numGoroutines {
		wg.Go(func() {
			for range itemsPerGoroutine {
				b.AddTraces(ptrace.NewTraces())
				b.AddMetrics(pmetric.NewMetrics())
				b.AddLogs(plog.NewLogs())
			}
		})
	}

	// Concurrent reads
	for range numGoroutines {
		wg.Go(func() {
			for range itemsPerGoroutine {
				b.GetRecentTraces(10, 0)
				b.GetRecentMetrics(10, 0)
				b.GetRecentLogs(10, 0)
				b.GetStats()
			}
		})
	}

	wg.Wait()

	// Verify final state
	stats := b.GetStats()
	assert.Equal(t, numGoroutines*itemsPerGoroutine, stats.TracesCount)
	assert.Equal(t, numGoroutines*itemsPerGoroutine, stats.MetricsCount)
	assert.Equal(t, numGoroutines*itemsPerGoroutine, stats.LogsCount)
}

func TestRingBufferOrder(t *testing.T) {
	b := New(5, 5, 5)

	// Add items with identifiable data
	for i := range 3 {
		td := ptrace.NewTraces()
		rs := td.ResourceSpans().AppendEmpty()
		rs.Resource().Attributes().PutInt("order", int64(i))
		b.AddTraces(td)
	}

	traces := b.GetRecentTraces(3, 0)
	require.Len(t, traces, 3)

	// Verify order (oldest first, as stored in ring buffer)
	for i, td := range traces {
		rs := td.ResourceSpans().At(0)
		order, ok := rs.Resource().Attributes().Get("order")
		require.True(t, ok)
		assert.Equal(t, int64(i), order.Int())
	}
}

func TestRingBufferWraparound(t *testing.T) {
	capacity := 3
	b := New(capacity, capacity, capacity)

	// Add items beyond capacity
	for i := range 5 {
		td := ptrace.NewTraces()
		rs := td.ResourceSpans().AppendEmpty()
		rs.Resource().Attributes().PutInt("order", int64(i))
		b.AddTraces(td)
	}

	traces := b.GetRecentTraces(10, 0)
	require.Len(t, traces, capacity)

	// Should have items 2, 3, 4 (oldest to newest after wraparound)
	orders := []int64{2, 3, 4}
	for i, td := range traces {
		rs := td.ResourceSpans().At(0)
		order, ok := rs.Resource().Attributes().Get("order")
		require.True(t, ok)
		assert.Equal(t, orders[i], order.Int())
	}
}

func TestBufferEmptyGet(t *testing.T) {
	b := New(5, 5, 5)

	traces := b.GetRecentTraces(10, 0)
	assert.Empty(t, traces)

	metrics := b.GetRecentMetrics(10, 0)
	assert.Empty(t, metrics)

	logs := b.GetRecentLogs(10, 0)
	assert.Empty(t, logs)

	stats := b.GetStats()
	assert.Equal(t, 0, stats.TracesCount)
	assert.Equal(t, 0, stats.MetricsCount)
	assert.Equal(t, 0, stats.LogsCount)
}

func BenchmarkBufferAdd(b *testing.B) {
	buf := New(1000, 1000, 1000)
	td := ptrace.NewTraces()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		buf.AddTraces(td)
	}
}

func BenchmarkBufferGet(b *testing.B) {
	buf := New(1000, 1000, 1000)

	// Pre-fill buffer
	for range 1000 {
		buf.AddTraces(ptrace.NewTraces())
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		buf.GetRecentTraces(100, 0)
	}
}

func BenchmarkBufferConcurrent(b *testing.B) {
	buf := New(1000, 1000, 1000)
	td := ptrace.NewTraces()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			buf.AddTraces(td)
			buf.GetRecentTraces(10, 0)
		}
	})
}
