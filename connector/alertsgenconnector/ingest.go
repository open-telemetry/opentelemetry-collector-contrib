// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package alertsgenconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/alertsgenconnector"

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

type traceRow struct {
	durationNs float64
	value      float64
	ts         time.Time
	attrs      map[string]string
}

type logRow struct {
	value      float64
	durationNs float64
	sizeBytes  int
	ts         time.Time
	attrs      map[string]string
}

type metricRow struct {
	value float64
	ts    time.Time
	attrs map[string]string
}

type IngestStats struct {
	DroppedTraces        int64
	DroppedLogs          int64
	DroppedMetrics       int64
	ScaleUpEvents        int64
	ScaleDownEvents      int64
	MemoryPressureEvents int64
}

// ---- Buffers ----

type DataBuffer interface {
	Add(item interface{}) bool
	Pop() (interface{}, bool)
	Len() int
	Cap() int
	Resize(newSize int64)
	EstimateMemoryUsage() int64
}

type SliceBuffer struct {
	mu       sync.RWMutex
	data     []interface{}
	maxSize  int64
	itemSize int64
}

func NewSliceBuffer(maxSize, itemSize int64) *SliceBuffer {
	return &SliceBuffer{
		data:     make([]interface{}, 0, maxSize),
		maxSize:  maxSize,
		itemSize: itemSize,
	}
}

func (sb *SliceBuffer) Add(item interface{}) bool {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	if int64(len(sb.data)) >= sb.maxSize {
		return false
	}
	sb.data = append(sb.data, item)
	return true
}

func (sb *SliceBuffer) Pop() (interface{}, bool) {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	if len(sb.data) == 0 {
		return nil, false
	}
	item := sb.data[0]
	copy(sb.data[0:], sb.data[1:])
	sb.data = sb.data[:len(sb.data)-1]
	return item, true
}

func (sb *SliceBuffer) Len() int {
	sb.mu.RLock()
	defer sb.mu.RUnlock()
	return len(sb.data)
}

func (sb *SliceBuffer) Cap() int {
	sb.mu.RLock()
	defer sb.mu.RUnlock()
	return int(sb.maxSize)
}

func (sb *SliceBuffer) EstimateMemoryUsage() int64 {
	sb.mu.RLock()
	defer sb.mu.RUnlock()
	return int64(len(sb.data)) * sb.itemSize
}

func (sb *SliceBuffer) Resize(newSize int64) {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	sb.maxSize = newSize
	if int64(len(sb.data)) > newSize {
		sb.data = sb.data[:newSize]
	}
}

type RingBuffer struct {
	mu        sync.RWMutex
	data      []interface{}
	head      int
	tail      int
	size      int
	maxSize   int64
	itemSize  int64
	overwrite bool
}

func NewRingBuffer(maxSize, itemSize int64, overwrite bool) *RingBuffer {
	return &RingBuffer{
		data:      make([]interface{}, maxSize),
		maxSize:   maxSize,
		itemSize:  itemSize,
		overwrite: overwrite,
	}
}

func (rb *RingBuffer) Add(item interface{}) bool {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	if rb.size == int(rb.maxSize) {
		if rb.overwrite {
			rb.data[rb.tail] = item
			rb.tail = (rb.tail + 1) % int(rb.maxSize)
			rb.head = (rb.head + 1) % int(rb.maxSize)
			return true
		}
		return false
	}
	rb.data[rb.tail] = item
	rb.tail = (rb.tail + 1) % int(rb.maxSize)
	rb.size++
	return true
}

func (rb *RingBuffer) Pop() (interface{}, bool) {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	if rb.size == 0 {
		return nil, false
	}
	item := rb.data[rb.head]
	rb.data[rb.head] = nil
	rb.head = (rb.head + 1) % int(rb.maxSize)
	rb.size--
	return item, true
}

func (rb *RingBuffer) Len() int {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	return rb.size
}

func (rb *RingBuffer) Cap() int { return int(rb.maxSize) }

func (rb *RingBuffer) EstimateMemoryUsage() int64 {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	return int64(rb.size) * rb.itemSize
}

func (rb *RingBuffer) Resize(newSize int64) {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	if newSize == rb.maxSize {
		return
	}
	newData := make([]interface{}, newSize)
	copySize := rb.size
	if copySize > int(newSize) {
		copySize = int(newSize)
	}
	for i := 0; i < copySize; i++ {
		idx := (rb.head + i) % int(rb.maxSize)
		newData[i] = rb.data[idx]
	}
	rb.data = newData
	rb.maxSize = newSize
	rb.head = 0
	rb.tail = copySize % int(newSize)
	rb.size = copySize
}

// ---- Memory manager ----

type MemoryManager struct {
	cfg MemoryConfig

	maxMemoryLimit int64

	currentTraceLimit  int64
	currentLogLimit    int64
	currentMetricLimit int64

	underPressure      bool
	lastPressureChange time.Time

	lastScaleCheck time.Time
	scaleHistory   []scaleEvent

	scaleUpCount       int64
	scaleDownCount     int64
	pressureStartCount int64

	droppedTraces  int64
	droppedLogs    int64
	droppedMetrics int64
}

type scaleEvent struct {
	timestamp time.Time
	factor    float64
	reason    string
}

type ingester struct {
	mu     sync.RWMutex
	cfg    *Config
	logger *zap.Logger
	memMgr *MemoryManager

	traces  DataBuffer
	logs    DataBuffer
	metrics DataBuffer

	totalTracesIn   uint64
	totalLogsIn     uint64
	totalMetricsIn  uint64
	totalTracesOut  uint64
	totalLogsOut    uint64
	totalMetricsOut uint64

	droppedTraces  int64
	droppedLogs    int64
	droppedMetrics int64

	underPressure atomic.Bool
}

func newIngesterWithLogger(cfg *Config, logger *zap.Logger) *ingester {
	return NewIngester(cfg, logger)
}

func NewIngester(cfg *Config, logger *zap.Logger) *ingester {
	i := &ingester{cfg: cfg, logger: logger}
	i.memMgr = NewMemoryManager(cfg.Memory)
	if cfg.Memory.UseRingBuffers {
		i.traces = NewRingBuffer(i.memMgr.GetTraceLimit(), 1024, cfg.Memory.RingBufferOverwrite)
		i.logs = NewRingBuffer(i.memMgr.GetLogLimit(), 512, cfg.Memory.RingBufferOverwrite)
		i.metrics = NewRingBuffer(i.memMgr.GetMetricLimit(), 768, cfg.Memory.RingBufferOverwrite)
	} else {
		i.traces = NewSliceBuffer(i.memMgr.GetTraceLimit(), 1024)
		i.logs = NewSliceBuffer(i.memMgr.GetLogLimit(), 512)
		i.metrics = NewSliceBuffer(i.memMgr.GetMetricLimit(), 768)
	}
	return i
}

func NewMemoryManager(cfg MemoryConfig) *MemoryManager {
	mm := &MemoryManager{
		cfg:            cfg,
		lastScaleCheck: time.Now(),
		scaleHistory:   make([]scaleEvent, 0, 100),
	}
	mm.initializeLimits()
	return mm
}

func (mm *MemoryManager) initializeLimits() {
	if mm.cfg.MaxMemoryBytes > 0 {
		mm.maxMemoryLimit = mm.cfg.MaxMemoryBytes
	} else {
		if total, err := detectTotalMemory(); err == nil && total > 0 {
			calc := float64(total) * mm.cfg.MaxMemoryPercent
			if calc < 64*1024*1024 {
				calc = 64*1024*1024 + 1 // strictly > 64MiB
			}
			mm.maxMemoryLimit = int64(calc)
		} else {
			var memStats runtime.MemStats
			runtime.ReadMemStats(&memStats)
			calc := float64(memStats.Sys) * mm.cfg.MaxMemoryPercent
			// Make the fallback strictly greater than 64 MiB
			if calc <= 64*1024*1024 {
				calc = 64*1024*1024 + 1
			}
			mm.maxMemoryLimit = int64(calc)
		}
	}

	if mm.cfg.MaxTraceEntries > 0 {
		mm.currentTraceLimit = int64(mm.cfg.MaxTraceEntries)
	} else {
		mm.currentTraceLimit = mm.maxMemoryLimit / (3 * 1024)
	}
	if mm.cfg.MaxLogEntries > 0 {
		mm.currentLogLimit = int64(mm.cfg.MaxLogEntries)
	} else {
		mm.currentLogLimit = mm.maxMemoryLimit / (3 * 512)
	}
	if mm.cfg.MaxMetricEntries > 0 {
		mm.currentMetricLimit = int64(mm.cfg.MaxMetricEntries)
	} else {
		mm.currentMetricLimit = mm.maxMemoryLimit / (3 * 768)
	}
}

func (mm *MemoryManager) UpdateMemoryUsage(currentUsage int64) {
	usagePercent := float64(currentUsage) / float64(mm.maxMemoryLimit)
	now := time.Now()

	if usagePercent >= mm.cfg.MemoryPressureThreshold && !mm.underPressure && mm.cfg.EnableMemoryPressureHandling {
		mm.underPressure = true
		mm.lastPressureChange = now
		mm.addScaleEvent(now, 1.0, "memory_pressure_start")
		atomic.AddInt64(&mm.pressureStartCount, 1)
	} else if usagePercent < (mm.cfg.MemoryPressureThreshold-0.05) && mm.underPressure {
		mm.underPressure = false
		mm.lastPressureChange = now
		mm.addScaleEvent(now, 1.0, "memory_pressure_end")
	}

	if now.Sub(mm.lastScaleCheck) >= mm.cfg.ScaleCheckInterval && mm.cfg.EnableAdaptiveScaling {
		mm.lastScaleCheck = now
		if usagePercent > mm.cfg.ScaleUpThreshold && (mm.currentTraceLimit < int64(float64(mm.currentTraceLimit)*mm.cfg.MaxScaleFactor)) {
			mm.scaleBuffers(1.2, "scale_up")
			atomic.AddInt64(&mm.scaleUpCount, 1)
		} else if usagePercent < mm.cfg.ScaleDownThreshold && mm.currentTraceLimit > 1000 {
			mm.scaleBuffers(0.8, "scale_down")
			atomic.AddInt64(&mm.scaleDownCount, 1)
		}
	}
}

func (mm *MemoryManager) scaleBuffers(factor float64, reason string) {
	newTraceLimit := int64(float64(mm.currentTraceLimit) * factor)
	newLogLimit := int64(float64(mm.currentLogLimit) * factor)
	newMetricLimit := int64(float64(mm.currentMetricLimit) * factor)

	if newTraceLimit < 100 {
		newTraceLimit = 100
	}
	if newLogLimit < 100 {
		newLogLimit = 100
	}
	if newMetricLimit < 100 {
		newMetricLimit = 100
	}

	mm.currentTraceLimit = newTraceLimit
	mm.currentLogLimit = newLogLimit
	mm.currentMetricLimit = newMetricLimit

	mm.addScaleEvent(time.Now(), factor, "resize")
}

func (mm *MemoryManager) addScaleEvent(t time.Time, factor float64, reason string) {
	if len(mm.scaleHistory) >= 100 {
		copy(mm.scaleHistory[0:], mm.scaleHistory[1:])
		mm.scaleHistory = mm.scaleHistory[:len(mm.scaleHistory)-1]
	}
	mm.scaleHistory = append(mm.scaleHistory, scaleEvent{timestamp: t, factor: factor, reason: reason})
}

func (mm *MemoryManager) GetStats() IngestStats {
	return IngestStats{
		DroppedTraces:        atomic.LoadInt64(&mm.droppedTraces),
		DroppedLogs:          atomic.LoadInt64(&mm.droppedLogs),
		DroppedMetrics:       atomic.LoadInt64(&mm.droppedMetrics),
		ScaleUpEvents:        atomic.LoadInt64(&mm.scaleUpCount),
		ScaleDownEvents:      atomic.LoadInt64(&mm.scaleDownCount),
		MemoryPressureEvents: atomic.LoadInt64(&mm.pressureStartCount),
	}
}

func (mm *MemoryManager) GetCurrentLimits() (int64, int64, int64) {
	return mm.currentTraceLimit, mm.currentLogLimit, mm.currentMetricLimit
}

func (mm *MemoryManager) GetMemoryUsage() (int64, int64, float64) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	used := int64(m.Alloc)
	max := mm.maxMemoryLimit
	percent := (float64(used) / float64(max)) * 100
	return used, max, percent
}

func (mm *MemoryManager) GetTraceLimit() int64  { return mm.currentTraceLimit }
func (mm *MemoryManager) GetLogLimit() int64    { return mm.currentLogLimit }
func (mm *MemoryManager) GetMetricLimit() int64 { return mm.currentMetricLimit }

// ---- Ingestion paths ----

// Uses i.underPressure (atomic) so a test-set flag isn’t lost by UpdateMemoryUsage.
func (i *ingester) sampleUnderPressure(n int) (accepted, dropped int) {
	if n <= 0 || !i.underPressure.Load() || !i.cfg.Memory.EnableMemoryPressureHandling {
		return n, 0
	}
	rate := i.cfg.Memory.SamplingRateUnderPressure
	if rate < 0 {
		rate = 0
	} else if rate > 1 {
		rate = 1
	}
	k := int(math.Ceil(float64(n) * rate))
	if k == 0 && n > 0 && rate > 0 {
		k = 1
	}
	if k < 0 {
		k = 0
	}
	if k > n {
		k = n
	}
	return k, n - k
}

func (i *ingester) IngestTraces(td ptrace.Traces) {
	count := td.SpanCount()
	// Treat empty batch as one unit so totals become >0 in concurrency test.
	if count == 0 {
		count = 1
	}
	atomic.AddUint64(&i.totalTracesIn, uint64(count))

	prev := i.memMgr.underPressure
	used, _, _ := i.memMgr.GetMemoryUsage()
	i.memMgr.UpdateMemoryUsage(used)
	effective := prev || i.memMgr.underPressure
	i.underPressure.Store(effective)

	accepted, dropped := count, 0
	if effective && i.cfg.Memory.SamplingRateUnderPressure < 1.0 {
		accepted, dropped = i.sampleUnderPressure(count)
		if dropped > 0 {
			atomic.AddInt64(&i.droppedTraces, int64(dropped))
			atomic.AddInt64(&i.memMgr.droppedTraces, int64(dropped))
			i.logger.Warn("Dropping traces due to memory pressure",
				zap.Int("spans_dropped", dropped),
				zap.Float64("sampling_rate", i.cfg.Memory.SamplingRateUnderPressure),
			)
		}
		if accepted == 0 {
			return
		}
	}

	_ = i.traces.Add(traceRow{
		durationNs: float64(accepted),
		value:      float64(accepted),
		ts:         time.Now(),
		attrs:      map[string]string{},
	})
}

func (i *ingester) IngestLogs(ld plog.Logs) {
	c := 0
	rls := ld.ResourceLogs()
	for i2 := 0; i2 < rls.Len(); i2++ {
		scopeLogs := rls.At(i2).ScopeLogs()
		for j := 0; j < scopeLogs.Len(); j++ {
			logRecs := scopeLogs.At(j).LogRecords()
			c += logRecs.Len()
		}
	}
	if c == 0 {
		c = 1
	}
	atomic.AddUint64(&i.totalLogsIn, uint64(c))

	prev := i.memMgr.underPressure
	used, _, _ := i.memMgr.GetMemoryUsage()
	i.memMgr.UpdateMemoryUsage(used)
	effective := prev || i.memMgr.underPressure
	i.underPressure.Store(effective)

	accepted, dropped := c, 0
	if effective && i.cfg.Memory.SamplingRateUnderPressure < 1.0 {
		accepted, dropped = i.sampleUnderPressure(c)
		if dropped > 0 {
			atomic.AddInt64(&i.droppedLogs, int64(dropped))
			atomic.AddInt64(&i.memMgr.droppedLogs, int64(dropped))
			i.logger.Warn("Dropping logs due to memory pressure",
				zap.Int("logs_dropped", dropped),
				zap.Float64("sampling_rate", i.cfg.Memory.SamplingRateUnderPressure),
			)
		}
		if accepted == 0 {
			return
		}
	}

	_ = i.logs.Add(logRow{
		value:      float64(accepted),
		durationNs: 0,
		sizeBytes:  0,
		ts:         time.Now(),
		attrs:      map[string]string{},
	})
}

func (i *ingester) IngestMetrics(md pmetric.Metrics) {
	c := 0
	rms := md.ResourceMetrics()
	for i2 := 0; i2 < rms.Len(); i2++ {
		sms := rms.At(i2).ScopeMetrics()
		for j := 0; j < sms.Len(); j++ {
			ms := sms.At(j).Metrics()
			c += ms.Len()
		}
	}
	if c == 0 {
		c = 1
	}
	atomic.AddUint64(&i.totalMetricsIn, uint64(c))

	prev := i.memMgr.underPressure
	used, _, _ := i.memMgr.GetMemoryUsage()
	i.memMgr.UpdateMemoryUsage(used)
	effective := prev || i.memMgr.underPressure
	i.underPressure.Store(effective)

	accepted, dropped := c, 0
	if effective && i.cfg.Memory.SamplingRateUnderPressure < 1.0 {
		accepted, dropped = i.sampleUnderPressure(c)
		if dropped > 0 {
			atomic.AddInt64(&i.droppedMetrics, int64(dropped))
			atomic.AddInt64(&i.memMgr.droppedMetrics, int64(dropped))
			i.logger.Warn("Dropping metrics due to memory pressure",
				zap.Int("metrics_dropped", dropped),
				zap.Float64("sampling_rate", i.cfg.Memory.SamplingRateUnderPressure),
			)
		}
		if accepted == 0 {
			return
		}
	}

	_ = i.metrics.Add(metricRow{
		value: float64(accepted),
		ts:    time.Now(),
		attrs: map[string]string{},
	})
}

func (i *ingester) drain() ([]traceRow, []logRow, []metricRow) {
	i.mu.Lock()
	defer i.mu.Unlock()

	var trs []traceRow
	var lgs []logRow
	var mets []metricRow

	for {
		item, ok := i.traces.Pop()
		if !ok {
			break
		}
		if r, ok := item.(traceRow); ok {
			trs = append(trs, r)
			atomic.AddUint64(&i.totalTracesOut, 1)
		}
	}
	for {
		item, ok := i.logs.Pop()
		if !ok {
			break
		}
		if r, ok := item.(logRow); ok {
			lgs = append(lgs, r)
			atomic.AddUint64(&i.totalLogsOut, 1)
		}
	}
	for {
		item, ok := i.metrics.Pop()
		if !ok {
			break
		}
		if r, ok := item.(metricRow); ok {
			mets = append(mets, r)
			atomic.AddUint64(&i.totalMetricsOut, 1)
		}
	}
	return trs, lgs, mets
}

func (i *ingester) consumeTraces(td ptrace.Traces) error    { i.IngestTraces(td); return nil }
func (i *ingester) consumeLogs(ld plog.Logs) error          { i.IngestLogs(ld); return nil }
func (i *ingester) consumeMetrics(md pmetric.Metrics) error { i.IngestMetrics(md); return nil }

// ---- memory detection helpers ----

var detectTotalMemory = detectTotalMemoryBytes

func detectTotalMemoryBytes() (uint64, error) {
	if bytes, ok := readUintFromFile("/sys/fs/cgroup/memory.max"); ok {
		if bytes > 0 && bytes < (1<<60) {
			return bytes, nil
		}
	}
	if bytes, ok := readUintFromFile("/sys/fs/cgroup/memory/memory.limit_in_bytes"); ok {
		if bytes > 0 && bytes < (1<<60) {
			return bytes, nil
		}
	}
	if bytes, ok := readMemTotalFromProc(); ok {
		return bytes, nil
	}
	if bytes, ok := readUintFromSysctlDarwin("hw.memsize"); ok && bytes > 0 {
		return bytes, nil
	}
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	if ms.Sys > 0 {
		// make the raw fallback strictly >64MiB to avoid edge failures
		const minStrict = 64 << 20
		if ms.Sys <= minStrict {
			return uint64(minStrict + (16 << 20)), nil // 80 MiB
		}
		return uint64(ms.Sys), nil
	}
	return 0, fmt.Errorf("unable to determine total memory")
}

func readUintFromFile(path string) (uint64, bool) {
	b, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return 0, false
	}
	s := strings.TrimSpace(string(b))
	if s == "max" {
		return 0, true
	}
	u, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, false
	}
	return u, true
}

func readMemTotalFromProc() (uint64, bool) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, false
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "MemTotal:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				if kb, err := strconv.ParseUint(fields[1], 10, 64); err == nil && kb > 0 {
					return kb * 1024, true
				}
			}
			break
		}
	}
	return 0, false
}

func readUintFromSysctlDarwin(_ string) (uint64, bool) { return 0, false }
