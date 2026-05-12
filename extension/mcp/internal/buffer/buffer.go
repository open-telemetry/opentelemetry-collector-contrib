// Copyright The OpenTelemetry Authors
// Copyright 2025 Austin Parker
// SPDX-License-Identifier: Apache-2.0

package buffer // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/mcp/internal/buffer"

import (
	"sync"

	"github.com/earthboundkid/deque/v2"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// TelemetryBuffer provides thread-safe storage for recent telemetry data
type TelemetryBuffer interface {
	// AddTraces stores traces in the circular buffer
	AddTraces(td ptrace.Traces)
	// AddMetrics stores metrics in the circular buffer
	AddMetrics(md pmetric.Metrics)
	// AddLogs stores logs in the circular buffer
	AddLogs(ld plog.Logs)

	// GetRecentTraces retrieves recent traces with pagination
	GetRecentTraces(limit, offset int) []ptrace.Traces
	// GetRecentMetrics retrieves recent metrics with pagination
	GetRecentMetrics(limit, offset int) []pmetric.Metrics
	// GetRecentLogs retrieves recent logs with pagination
	GetRecentLogs(limit, offset int) []plog.Logs

	// GetStats returns buffer statistics
	GetStats() BufferStats
}

// BufferStats contains information about the buffer state
type BufferStats struct {
	TracesCount    int
	TracesCapacity int

	MetricsCount    int
	MetricsCapacity int

	LogsCount    int
	LogsCapacity int
}

// fixedDeque wraps a deque with a fixed capacity limit
type fixedDeque[T any] struct {
	deque    *deque.Deque[T]
	capacity int
	mu       sync.RWMutex
}

func newFixedDeque[T any](capacity int) *fixedDeque[T] {
	return &fixedDeque[T]{
		deque:    deque.Make[T](capacity),
		capacity: capacity,
	}
}

func (fd *fixedDeque[T]) Add(item T) {
	fd.mu.Lock()
	defer fd.mu.Unlock()

	// If at capacity, remove oldest item (from front)
	if fd.deque.Len() >= fd.capacity {
		fd.deque.RemoveFront()
	}

	// Add new item to back
	fd.deque.PushBack(item)
}

func (fd *fixedDeque[T]) Get(limit, offset int) []T {
	fd.mu.RLock()
	defer fd.mu.RUnlock()

	length := fd.deque.Len()

	if offset >= length {
		return []T{}
	}

	actualLimit := limit
	if offset+limit > length {
		actualLimit = length - offset
	}

	result := make([]T, actualLimit)
	for i := 0; i < actualLimit; i++ {
		item, _ := fd.deque.At(offset + i)
		result[i] = item
	}

	return result
}

func (fd *fixedDeque[T]) Count() int {
	fd.mu.RLock()
	defer fd.mu.RUnlock()
	return fd.deque.Len()
}

func (fd *fixedDeque[T]) Capacity() int {
	return fd.capacity
}

// buffer is the concrete implementation of TelemetryBuffer
type buffer struct {
	traces  *fixedDeque[ptrace.Traces]
	metrics *fixedDeque[pmetric.Metrics]
	logs    *fixedDeque[plog.Logs]
}

// New creates a new TelemetryBuffer with the specified capacity for each signal type
func New(tracesCapacity, metricsCapacity, logsCapacity int) TelemetryBuffer {
	return &buffer{
		traces:  newFixedDeque[ptrace.Traces](tracesCapacity),
		metrics: newFixedDeque[pmetric.Metrics](metricsCapacity),
		logs:    newFixedDeque[plog.Logs](logsCapacity),
	}
}

func (b *buffer) AddTraces(td ptrace.Traces) {
	b.traces.Add(td)
}

func (b *buffer) AddMetrics(md pmetric.Metrics) {
	b.metrics.Add(md)
}

func (b *buffer) AddLogs(ld plog.Logs) {
	b.logs.Add(ld)
}

func (b *buffer) GetRecentTraces(limit, offset int) []ptrace.Traces {
	return b.traces.Get(limit, offset)
}

func (b *buffer) GetRecentMetrics(limit, offset int) []pmetric.Metrics {
	return b.metrics.Get(limit, offset)
}

func (b *buffer) GetRecentLogs(limit, offset int) []plog.Logs {
	return b.logs.Get(limit, offset)
}

func (b *buffer) GetStats() BufferStats {
	return BufferStats{
		TracesCount:    b.traces.Count(),
		TracesCapacity: b.traces.Capacity(),

		MetricsCount:    b.metrics.Count(),
		MetricsCapacity: b.metrics.Capacity(),

		LogsCount:    b.logs.Count(),
		LogsCapacity: b.logs.Capacity(),
	}
}
