// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package alertsgenconnector

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap/zaptest"
)

func TestSliceBuffer(t *testing.T) {
	t.Run("basic_operations", func(t *testing.T) {
		buf := NewSliceBuffer(10, 100)

		// Test Add
		assert.True(t, buf.Add("item1"))
		assert.True(t, buf.Add("item2"))
		assert.Equal(t, 2, buf.Len())
		assert.Equal(t, 10, buf.Cap())

		// Test Pop
		item, ok := buf.Pop()
		assert.True(t, ok)
		assert.Equal(t, "item1", item)
		assert.Equal(t, 1, buf.Len())

		item, ok = buf.Pop()
		assert.True(t, ok)
		assert.Equal(t, "item2", item)
		assert.Equal(t, 0, buf.Len())

		// Pop from empty
		item, ok = buf.Pop()
		assert.False(t, ok)
		assert.Nil(t, item)
	})

	t.Run("capacity_limit", func(t *testing.T) {
		buf := NewSliceBuffer(3, 100)

		assert.True(t, buf.Add("item1"))
		assert.True(t, buf.Add("item2"))
		assert.True(t, buf.Add("item3"))
		assert.False(t, buf.Add("item4")) // Should fail
		assert.Equal(t, 3, buf.Len())
	})

	t.Run("resize", func(t *testing.T) {
		buf := NewSliceBuffer(5, 100)

		for i := 0; i < 5; i++ {
			buf.Add(i)
		}
		assert.Equal(t, 5, buf.Len())

		// Resize smaller
		buf.Resize(3)
		assert.Equal(t, 3, buf.Len())
		assert.Equal(t, 3, buf.Cap())

		// Resize larger
		buf.Resize(10)
		assert.Equal(t, 3, buf.Len())
		assert.Equal(t, 10, buf.Cap())
		assert.True(t, buf.Add("new"))
	})

	t.Run("memory_estimation", func(t *testing.T) {
		buf := NewSliceBuffer(10, 256)

		assert.Equal(t, int64(0), buf.EstimateMemoryUsage())

		buf.Add("item1")
		buf.Add("item2")
		assert.Equal(t, int64(512), buf.EstimateMemoryUsage())
	})

	t.Run("concurrent_access", func(t *testing.T) {
		buf := NewSliceBuffer(100, 100)
		var wg sync.WaitGroup

		// Concurrent adds
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(val int) {
				defer wg.Done()
				buf.Add(val)
			}(i)
		}

		wg.Wait()
		assert.Equal(t, 10, buf.Len())

		// Concurrent pops
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				buf.Pop()
			}()
		}

		wg.Wait()
		assert.Equal(t, 5, buf.Len())
	})
}

func TestRingBuffer(t *testing.T) {
	t.Run("basic_operations", func(t *testing.T) {
		buf := NewRingBuffer(10, 100, false)

		// Test Add
		assert.True(t, buf.Add("item1"))
		assert.True(t, buf.Add("item2"))
		assert.Equal(t, 2, buf.Len())
		assert.Equal(t, 10, buf.Cap())

		// Test Pop
		item, ok := buf.Pop()
		assert.True(t, ok)
		assert.Equal(t, "item1", item)
		assert.Equal(t, 1, buf.Len())
	})

	t.Run("overwrite_mode", func(t *testing.T) {
		buf := NewRingBuffer(3, 100, true)

		// Fill buffer
		assert.True(t, buf.Add("item1"))
		assert.True(t, buf.Add("item2"))
		assert.True(t, buf.Add("item3"))
		assert.Equal(t, 3, buf.Len())

		// Should overwrite oldest
		assert.True(t, buf.Add("item4"))
		assert.Equal(t, 3, buf.Len())

		// Pop should get item2 (item1 was overwritten)
		item, _ := buf.Pop()
		assert.Equal(t, "item2", item)
	})

	t.Run("no_overwrite_mode", func(t *testing.T) {
		buf := NewRingBuffer(3, 100, false)

		// Fill buffer
		assert.True(t, buf.Add("item1"))
		assert.True(t, buf.Add("item2"))
		assert.True(t, buf.Add("item3"))

		// Should reject when full
		assert.False(t, buf.Add("item4"))
		assert.Equal(t, 3, buf.Len())
	})

	t.Run("resize", func(t *testing.T) {
		buf := NewRingBuffer(5, 100, false)

		for i := 0; i < 5; i++ {
			buf.Add(i)
		}

		// Resize smaller
		buf.Resize(3)
		assert.Equal(t, 3, buf.Len())

		// Resize larger
		buf.Resize(10)
		assert.Equal(t, 3, buf.Len())

		// Can add more items
		assert.True(t, buf.Add("new"))
		assert.Equal(t, 4, buf.Len())
	})
}

func TestMemoryManager(t *testing.T) {
	t.Run("initialization", func(t *testing.T) {
		cfg := MemoryConfig{
			MaxMemoryPercent:      0.1,
			MaxTraceEntries:       1000,
			MaxLogEntries:         2000,
			MaxMetricEntries:      3000,
			EnableAdaptiveScaling: false,
		}

		mm := NewMemoryManager(cfg)
		assert.Equal(t, int64(1000), mm.GetTraceLimit())
		assert.Equal(t, int64(2000), mm.GetLogLimit())
		assert.Equal(t, int64(3000), mm.GetMetricLimit())
	})

	t.Run("auto_calculate_limits", func(t *testing.T) {
		cfg := MemoryConfig{
			MaxMemoryPercent: 0.1,
			MaxTraceEntries:  0, // Auto-calculate
			MaxLogEntries:    0,
			MaxMetricEntries: 0,
		}

		mm := NewMemoryManager(cfg)

		// Should have non-zero limits
		assert.Greater(t, mm.GetTraceLimit(), int64(0))
		assert.Greater(t, mm.GetLogLimit(), int64(0))
		assert.Greater(t, mm.GetMetricLimit(), int64(0))
	})

	t.Run("memory_pressure", func(t *testing.T) {
		cfg := MemoryConfig{
			MaxMemoryBytes:               100 * 1024 * 1024, // 100MB
			EnableMemoryPressureHandling: true,
			MemoryPressureThreshold:      0.8,
			MaxTraceEntries:              1000,
		}

		mm := NewMemoryManager(cfg)

		// Simulate high memory usage
		mm.UpdateMemoryUsage(85 * 1024 * 1024) // 85MB = 85% usage
		assert.True(t, mm.underPressure)

		// Simulate recovery
		mm.UpdateMemoryUsage(70 * 1024 * 1024) // 70MB = 70% usage
		assert.False(t, mm.underPressure)
	})

	t.Run("adaptive_scaling", func(t *testing.T) {
		cfg := MemoryConfig{
			MaxMemoryBytes:        100 * 1024 * 1024,
			EnableAdaptiveScaling: true,
			ScaleUpThreshold:      0.8,
			ScaleDownThreshold:    0.3,
			ScaleCheckInterval:    1 * time.Millisecond,
			MaxScaleFactor:        5.0,
			MaxTraceEntries:       1000,
			MaxLogEntries:         1000,
			MaxMetricEntries:      1000,
		}

		mm := NewMemoryManager(cfg)
		initialLimit := mm.GetTraceLimit()

		// Trigger scale up
		time.Sleep(2 * time.Millisecond)
		mm.UpdateMemoryUsage(85 * 1024 * 1024) // 85% usage
		assert.Greater(t, mm.GetTraceLimit(), initialLimit)

		// Trigger scale down
		time.Sleep(2 * time.Millisecond)
		mm.UpdateMemoryUsage(25 * 1024 * 1024) // 25% usage
		assert.Less(t, mm.GetTraceLimit(), mm.GetTraceLimit()*2)
	})

	t.Run("get_stats", func(t *testing.T) {
		cfg := MemoryConfig{
			MaxMemoryPercent: 0.1,
		}

		mm := NewMemoryManager(cfg)
		stats := mm.GetStats()

		assert.Equal(t, int64(0), stats.DroppedTraces)
		assert.Equal(t, int64(0), stats.DroppedLogs)
		assert.Equal(t, int64(0), stats.DroppedMetrics)
		assert.Equal(t, int64(0), stats.ScaleUpEvents)
		assert.Equal(t, int64(0), stats.ScaleDownEvents)
		assert.Equal(t, int64(0), stats.MemoryPressureEvents)
	})
}

func TestIngester(t *testing.T) {
	t.Run("initialization", func(t *testing.T) {
		cfg := &Config{
			Memory: MemoryConfig{
				MaxMemoryPercent: 0.1,
				UseRingBuffers:   false,
			},
		}

		logger := zaptest.NewLogger(t)
		ing := NewIngester(cfg, logger)

		assert.NotNil(t, ing)
		assert.NotNil(t, ing.traces)
		assert.NotNil(t, ing.logs)
		assert.NotNil(t, ing.metrics)
		assert.NotNil(t, ing.memMgr)
	})

	t.Run("ingest_traces", func(t *testing.T) {
		cfg := &Config{
			Memory: MemoryConfig{
				MaxMemoryPercent:          0.1,
				SamplingRateUnderPressure: 1.0,
			},
		}

		logger := zaptest.NewLogger(t)
		ing := NewIngester(cfg, logger)

		// Create test traces
		td := ptrace.NewTraces()
		rs := td.ResourceSpans().AppendEmpty()
		ss := rs.ScopeSpans().AppendEmpty()
		for i := 0; i < 5; i++ {
			span := ss.Spans().AppendEmpty()
			span.SetName("test-span")
		}

		ing.IngestTraces(td)
		assert.Equal(t, uint64(5), ing.totalTracesIn)
		assert.Equal(t, 1, ing.traces.Len())
	})

	t.Run("ingest_logs", func(t *testing.T) {
		cfg := &Config{
			Memory: MemoryConfig{
				MaxMemoryPercent:          0.1,
				SamplingRateUnderPressure: 1.0,
			},
		}

		logger := zaptest.NewLogger(t)
		ing := NewIngester(cfg, logger)

		// Create test logs
		ld := plog.NewLogs()
		rl := ld.ResourceLogs().AppendEmpty()
		sl := rl.ScopeLogs().AppendEmpty()
		for i := 0; i < 3; i++ {
			lr := sl.LogRecords().AppendEmpty()
			lr.Body().SetStr("test log")
		}

		ing.IngestLogs(ld)
		assert.Equal(t, uint64(3), ing.totalLogsIn)
		assert.Equal(t, 1, ing.logs.Len())
	})

	t.Run("ingest_metrics", func(t *testing.T) {
		cfg := &Config{
			Memory: MemoryConfig{
				MaxMemoryPercent:          0.1,
				SamplingRateUnderPressure: 1.0,
			},
		}

		logger := zaptest.NewLogger(t)
		ing := NewIngester(cfg, logger)

		// Create test metrics
		md := pmetric.NewMetrics()
		rm := md.ResourceMetrics().AppendEmpty()
		sm := rm.ScopeMetrics().AppendEmpty()
		for i := 0; i < 4; i++ {
			m := sm.Metrics().AppendEmpty()
			m.SetName("test.metric")
			m.SetEmptyGauge()
		}

		ing.IngestMetrics(md)
		assert.Equal(t, uint64(4), ing.totalMetricsIn)
		assert.Equal(t, 1, ing.metrics.Len())
	})

	t.Run("memory_pressure_sampling", func(t *testing.T) {
		cfg := &Config{
			Memory: MemoryConfig{
				MaxMemoryBytes:               100 * 1024 * 1024,
				EnableMemoryPressureHandling: true,
				MemoryPressureThreshold:      0.8,
				SamplingRateUnderPressure:    0.0, // Drop all under pressure
			},
		}

		logger := zaptest.NewLogger(t)
		ing := NewIngester(cfg, logger)

		// Simulate memory pressure
		ing.memMgr.underPressure = true

		// Create test data
		td := ptrace.NewTraces()
		td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()

		ing.IngestTraces(td)

		// Should be dropped
		assert.Equal(t, int64(1), ing.droppedTraces)
		assert.Equal(t, 0, ing.traces.Len())
	})

	t.Run("drain", func(t *testing.T) {
		cfg := &Config{
			Memory: MemoryConfig{
				MaxMemoryPercent: 0.1,
			},
		}

		logger := zaptest.NewLogger(t)
		ing := NewIngester(cfg, logger)

		// Add test data
		ing.traces.Add(traceRow{durationNs: 100})
		ing.traces.Add(traceRow{durationNs: 200})
		ing.logs.Add(logRow{value: 1})
		ing.metrics.Add(metricRow{value: 0.5})

		// Drain
		traces, logs, metrics := ing.drain()

		assert.Equal(t, 2, len(traces))
		assert.Equal(t, 1, len(logs))
		assert.Equal(t, 1, len(metrics))

		// Buffers should be empty
		assert.Equal(t, 0, ing.traces.Len())
		assert.Equal(t, 0, ing.logs.Len())
		assert.Equal(t, 0, ing.metrics.Len())
	})

	t.Run("concurrent_ingestion", func(t *testing.T) {
		cfg := &Config{
			Memory: MemoryConfig{
				MaxMemoryPercent:          0.1,
				SamplingRateUnderPressure: 1.0,
			},
		}

		logger := zaptest.NewLogger(t)
		ing := NewIngester(cfg, logger)

		var wg sync.WaitGroup

		// Concurrent trace ingestion
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				td := ptrace.NewTraces()
				td.ResourceSpans().AppendEmpty()
				ing.IngestTraces(td)
			}()
		}

		// Concurrent log ingestion
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				ld := plog.NewLogs()
				ld.ResourceLogs().AppendEmpty()
				ing.IngestLogs(ld)
			}()
		}

		// Concurrent metric ingestion
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				md := pmetric.NewMetrics()
				md.ResourceMetrics().AppendEmpty()
				ing.IngestMetrics(md)
			}()
		}

		wg.Wait()

		// Check that data was ingested
		assert.Greater(t, ing.totalTracesIn, uint64(0))
		assert.Greater(t, ing.totalLogsIn, uint64(0))
		assert.Greater(t, ing.totalMetricsIn, uint64(0))
	})
}

func TestMemoryDetection(t *testing.T) {
	t.Run("detect_total_memory", func(t *testing.T) {
		// Save original function
		origDetect := detectTotalMemory
		defer func() { detectTotalMemory = origDetect }()

		// Mock memory detection
		detectTotalMemory = func() (uint64, error) {
			return 8 * 1024 * 1024 * 1024, nil // 8GB
		}

		cfg := MemoryConfig{
			MaxMemoryPercent: 0.1,
		}

		mm := NewMemoryManager(cfg)
		assert.Greater(t, mm.maxMemoryLimit, int64(0))
		assert.LessOrEqual(t, mm.maxMemoryLimit, int64(8*1024*1024*1024))
	})

	t.Run("fallback_to_runtime", func(t *testing.T) {
		// Save original function
		origDetect := detectTotalMemory
		defer func() { detectTotalMemory = origDetect }()

		// Mock failure
		detectTotalMemory = func() (uint64, error) {
			return 0, fmt.Errorf("unable to detect")
		}

		cfg := MemoryConfig{
			MaxMemoryPercent: 0.1,
		}

		mm := NewMemoryManager(cfg)

		// Should fall back to runtime.MemStats
		assert.Greater(t, mm.maxMemoryLimit, int64(64*1024*1024)) // At least 64MB
	})
}

func TestBufferSelection(t *testing.T) {
	t.Run("use_slice_buffers", func(t *testing.T) {
		cfg := &Config{
			Memory: MemoryConfig{
				MaxMemoryPercent: 0.1,
				UseRingBuffers:   false,
			},
		}

		logger := zaptest.NewLogger(t)
		ing := NewIngester(cfg, logger)

		// Check buffer types
		_, isSlice := ing.traces.(*SliceBuffer)
		assert.True(t, isSlice)
	})

	t.Run("use_ring_buffers", func(t *testing.T) {
		cfg := &Config{
			Memory: MemoryConfig{
				MaxMemoryPercent:    0.1,
				UseRingBuffers:      true,
				RingBufferOverwrite: true,
			},
		}

		logger := zaptest.NewLogger(t)
		ing := NewIngester(cfg, logger)

		// Check buffer types
		_, isRing := ing.traces.(*RingBuffer)
		assert.True(t, isRing)
	})
}
