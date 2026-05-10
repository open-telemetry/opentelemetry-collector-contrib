// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package cardinalityguardianprocessor implements an OpenTelemetry Collector
// metrics processor that enforces per-metric-per-label cardinality limits using
// HyperLogLog sketches. It is designed to be a drop-in pipeline component with
// zero web-server footprint — the binary exports no HTTP endpoints.
//
// # Problem
//
// In high-throughput observability pipelines, individual label keys (e.g.
// "session_id", "request_id", "user_id") can take on millions of unique values,
// causing cardinality explosions in downstream time-series databases. Each
// unique label combination creates a new time series; at cloud TSDB pricing,
// a single misbehaving label can cost thousands of dollars per month.
//
// # How it works
//
// For every (metric_name, label_key) pair the processor maintains two
// HyperLogLog++ sketches — one for the current epoch and one for the
// previous epoch. On each data point the unique count is estimated as the
// delta between those two sketches. When the delta exceeds the configured
// MaxCardinalityDeltaPerEpoch the processor either removes the offending
// attribute (strip_and_reaggregate, the default) or injects a routing tag so
// that downstream components can shunt the metric to cheap cold storage
// (tag_only).
//
// # Architecture
//
//	ConsumeMetrics (hot path, called concurrently by the Collector)
//	  └─ handleAttributes           (per data point)
//	        └─ shouldDrop           (per label key/value pair)
//	              ├─ xxhash.Sum64String  — zero-alloc uint64 hash of value
//	              ├─ getShard            — maphash routing to 1/256 of key space
//	              ├─ shard.mu RLock      — fast path when tracker already exists
//	              └─ tracker.insert     — HLL InsertHash + lazy cached estimate
//
//	Background goroutine (one per processor lifetime, started in Start)
//	  └─ rotate — every EpochDurationSeconds, advances the sliding window
//	               across all 256 shards, one shard at a time
package cardinalityguardianprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/cardinalityguardianprocessor"

import (
	"context"
	"hash/maphash"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/axiomhq/hyperloglog"
	"github.com/cespare/xxhash/v2"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/cardinalityguardianprocessor/internal/metadata"
)

// staleSweepEpochs is the number of consecutive epochs a tracker must receive
// zero inserts before it is eligible for eviction from the shard map.
const staleSweepEpochs = 2

// numShards is the number of independent shard buckets used to partition the
// trackers map. 256 was chosen because it is a power of two (bitmask AND),
// fits in a uint8, and eliminates visible mutex contention up to ~512
// concurrent goroutines under a uniform metric-name distribution.
const numShards = 256

// sketchPool is a package-level pool of fresh HyperLogLog++ sketches.
// New14() uses precision p=14 (16 384 registers, ~0.81% standard error, ~12 KB dense).
// Dirty sketches are NOT returned to the pool because hyperloglog.Sketch has no
// Reset() method; returning a used sketch would contaminate the next epoch's delta.
var sketchPool = sync.Pool{
	New: func() any { return hyperloglog.New14() },
}

func mustGetSketch() *hyperloglog.Sketch {
	s, ok := sketchPool.Get().(*hyperloglog.Sketch)
	if !ok {
		panic("sketchPool: New returned a non-*hyperloglog.Sketch value")
	}
	return s
}

// trackerKey is a zero-allocation composite key for the trackers map.
type trackerKey struct {
	metricName string
	attrKey    string
}

// estimateInterval controls how often the HLL Estimate() is recomputed in the
// hot path. Refreshing every 64 inserts amortizes the ~5 heap allocs/call from
// mergeSparse() to ≈0.08 allocs/op.
const estimateInterval = 64

// tracker holds two HyperLogLog++ sketches for a single (metric_name, label_key) pair.
type tracker struct {
	mu          sync.Mutex
	current     *hyperloglog.Sketch
	previous    *hyperloglog.Sketch
	cachedCurr  uint64
	cachedPrev  uint64
	insertCount uint64
	idleEpochs  int
}

func (t *tracker) insert(hashVal uint64) (curr, prev uint64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.current.InsertHash(hashVal)
	t.insertCount++
	if t.insertCount <= estimateInterval || t.insertCount&(estimateInterval-1) == 0 {
		t.cachedCurr = t.current.Estimate()
	}
	return t.cachedCurr, t.cachedPrev
}

func (t *tracker) rotate(fresh *hyperloglog.Sketch) (idle bool) {
	t.mu.Lock()
	idle = t.insertCount == 0
	if idle {
		t.idleEpochs++
	} else {
		t.idleEpochs = 0
	}
	t.cachedPrev = t.cachedCurr
	t.cachedCurr = 0
	t.insertCount = 0
	t.previous = t.current
	t.current = fresh
	t.mu.Unlock()
	return idle
}

func newTracker() *tracker {
	return &tracker{
		current:  mustGetSketch(),
		previous: mustGetSketch(),
	}
}

// offenderEntry holds a snapshot of a single high-delta tracker for telemetry reporting.
type offenderEntry struct {
	metricName string
	labelKey   string
	delta      uint64
}

// trackerEntry pairs a trackerKey with its tracker pointer.
type trackerEntry struct {
	key trackerKey
	t   *tracker
}

// trackerShard is an independently-locked partition of the global trackers map.
type trackerShard struct {
	mu       sync.RWMutex
	trackers map[trackerKey]*tracker
}

// cardinalityProcessor is the internal implementation of the processor.Metrics interface.
type cardinalityProcessor struct {
	config                      *Config
	logger                      *zap.Logger
	next                        consumer.Metrics
	protectedLabels             map[string]struct{}
	seed                        maphash.Seed
	shards                      [numShards]*trackerShard
	ctx                         context.Context
	cancel                      context.CancelFunc
	startOnce                   sync.Once
	enforcementMode             EnforcementMode
	estimatedCostPerMetricMonth float64
	labelsStripped              atomic.Int64
	trackerCount                atomic.Int64
	trackersRejected            atomic.Int64
	rejectionWarned             atomic.Bool
	dropLogCount                atomic.Int64
	dropsThisEpoch              atomic.Int64
	telemetry                   *metadata.TelemetryBuilder
	topOffenders                []offenderEntry
	topOffendersMu              sync.RWMutex
}

func newCardinalityProcessor(ctx context.Context, cfg *Config, set processor.Settings, next consumer.Metrics) (processor.Metrics, error) {
	protected := make(map[string]struct{}, len(cfg.NeverDropLabels))
	for _, l := range cfg.NeverDropLabels {
		protected[l] = struct{}{}
	}

	childCtx, cancel := context.WithCancel(ctx)

	p := &cardinalityProcessor{
		config:                      cfg,
		logger:                      set.TelemetrySettings.Logger,
		next:                        next,
		protectedLabels:             protected,
		seed:                        maphash.MakeSeed(),
		ctx:                         childCtx,
		cancel:                      cancel,
		enforcementMode:             cfg.ResolvedEnforcementMode(),
		estimatedCostPerMetricMonth: cfg.EstimatedCostPerMetricMonth,
	}

	for i := range p.shards {
		p.shards[i] = &trackerShard{
			trackers: make(map[trackerKey]*tracker),
		}
	}

	builder, err := metadata.NewTelemetryBuilder(set.TelemetrySettings)
	if err != nil {
		return nil, err
	}
	p.telemetry = builder

	err = builder.RegisterProcessorCardinalityTrackersActiveCallback(func(_ context.Context, o metric.Int64Observer) error {
		var totalActive int64
		for i := range p.shards {
			p.shards[i].mu.RLock()
			totalActive += int64(len(p.shards[i].trackers))
			p.shards[i].mu.RUnlock()
		}
		o.Observe(totalActive)
		return nil
	})
	if err != nil {
		return nil, err
	}

	err = builder.RegisterProcessorCardinalityLabelsStrippedCallback(func(_ context.Context, o metric.Int64Observer) error {
		o.Observe(p.labelsStripped.Load())
		return nil
	})
	if err != nil {
		return nil, err
	}

	err = builder.RegisterProcessorCardinalitySavingsEstimatedCallback(func(_ context.Context, o metric.Float64Observer) error {
		drops := p.labelsStripped.Load()
		savings := float64(drops) * p.estimatedCostPerMetricMonth
		roundedSavings := math.Round(savings*100) / 100
		o.Observe(roundedSavings)
		return nil
	})
	if err != nil {
		return nil, err
	}

	err = builder.RegisterProcessorCardinalityTrackersRejectedCallback(func(_ context.Context, o metric.Int64Observer) error {
		o.Observe(p.trackersRejected.Load())
		return nil
	})
	if err != nil {
		return nil, err
	}

	err = builder.RegisterProcessorCardinalityTopOffendersCallback(func(_ context.Context, o metric.Int64Observer) error {
		p.topOffendersMu.RLock()
		for _, entry := range p.topOffenders {
			o.Observe(int64(entry.delta),
				metric.WithAttributes(
					attribute.String("metric_name", entry.metricName),
					attribute.String("label_key", entry.labelKey),
				),
			)
		}
		p.topOffendersMu.RUnlock()
		return nil
	})
	if err != nil {
		return nil, err
	}

	return p, nil
}

func (p *cardinalityProcessor) getShard(metricName string) *trackerShard {
	return p.shards[maphash.String(p.seed, metricName)&(numShards-1)]
}

func (p *cardinalityProcessor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}

func (p *cardinalityProcessor) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	rm := md.ResourceMetrics()
	for i := 0; i < rm.Len(); i++ {
		sm := rm.At(i).ScopeMetrics()
		for j := 0; j < sm.Len(); j++ {
			ms := sm.At(j).Metrics()
			for k := 0; k < ms.Len(); k++ {
				p.processMetric(ms.At(k))
			}
		}
	}
	return p.next.ConsumeMetrics(ctx, md)
}

// processMetric dispatches a single metric to the appropriate handler.
// When enforcement mode is strip_and_reaggregate, inline spatial reaggregation
// is performed for Delta Sum and Gauge. Unsupported types (Cumulative Sum,
// Histogram, ExponentialHistogram, Summary) fall back to tag_only.
func (p *cardinalityProcessor) processMetric(m pmetric.Metric) {
	switch m.Type() {
	case pmetric.MetricTypeGauge:
		p.processNumberDataPoints(m.Name(), m.Gauge().DataPoints())
		if p.enforcementMode == EnforcementStripAndReaggregate {
			reaggregateNumberDataPoints(m.Gauge().DataPoints(), pmetric.MetricTypeGauge, false)
		}
	case pmetric.MetricTypeSum:
		isDelta := m.Sum().AggregationTemporality() == pmetric.AggregationTemporalityDelta
		if p.enforcementMode == EnforcementStripAndReaggregate && !isDelta {
			p.processNumberDataPointsWithMode(m.Name(), m.Sum().DataPoints(), EnforcementTagOnly)
		} else {
			p.processNumberDataPoints(m.Name(), m.Sum().DataPoints())
			if p.enforcementMode == EnforcementStripAndReaggregate && isDelta {
				reaggregateNumberDataPoints(m.Sum().DataPoints(), pmetric.MetricTypeSum, true)
			}
		}
	case pmetric.MetricTypeHistogram:
		if p.enforcementMode == EnforcementStripAndReaggregate {
			p.processHistogramDataPointsWithMode(m.Name(), m.Histogram().DataPoints(), EnforcementTagOnly)
		} else {
			p.processHistogramDataPoints(m.Name(), m.Histogram().DataPoints())
		}
	case pmetric.MetricTypeExponentialHistogram:
		if p.enforcementMode == EnforcementStripAndReaggregate {
			p.processExponentialHistogramDataPointsWithMode(m.Name(), m.ExponentialHistogram().DataPoints(), EnforcementTagOnly)
		} else {
			p.processExponentialHistogramDataPoints(m.Name(), m.ExponentialHistogram().DataPoints())
		}
	case pmetric.MetricTypeSummary:
		if p.enforcementMode == EnforcementStripAndReaggregate {
			p.processSummaryDataPointsWithMode(m.Name(), m.Summary().DataPoints(), EnforcementTagOnly)
		} else {
			p.processSummaryDataPoints(m.Name(), m.Summary().DataPoints())
		}
	}
}

func (p *cardinalityProcessor) processNumberDataPoints(metricName string, dps pmetric.NumberDataPointSlice) {
	for i := 0; i < dps.Len(); i++ {
		p.handleAttributes(metricName, dps.At(i).Attributes())
	}
}

func (p *cardinalityProcessor) processHistogramDataPoints(metricName string, dps pmetric.HistogramDataPointSlice) {
	for i := 0; i < dps.Len(); i++ {
		p.handleAttributes(metricName, dps.At(i).Attributes())
	}
}

func (p *cardinalityProcessor) processExponentialHistogramDataPoints(metricName string, dps pmetric.ExponentialHistogramDataPointSlice) {
	for i := 0; i < dps.Len(); i++ {
		p.handleAttributes(metricName, dps.At(i).Attributes())
	}
}

func (p *cardinalityProcessor) processSummaryDataPoints(metricName string, dps pmetric.SummaryDataPointSlice) {
	for i := 0; i < dps.Len(); i++ {
		p.handleAttributes(metricName, dps.At(i).Attributes())
	}
}

func (p *cardinalityProcessor) handleAttributes(metricName string, attrs pcommon.Map) {
	p.handleAttributesWithMode(metricName, attrs, p.enforcementMode)
}

// handleAttributesWithMode is the mode-parameterized enforcement loop.
// Post-iteration mutations are deferred to avoid mutating the map during RemoveIf.
func (p *cardinalityProcessor) handleAttributesWithMode(metricName string, attrs pcommon.Map, mode EnforcementMode) {
	shouldTag := false
	var overflowKeys []string

	attrs.RemoveIf(func(k string, v pcommon.Value) bool {
		if p.isProtected(k) {
			return false
		}
		if p.shouldDrop(metricName, k, v.AsString()) {
			p.labelsStripped.Add(1)
			switch mode {
			case EnforcementTagOnly:
				shouldTag = true
				return false
			case EnforcementOverflowAttribute:
				overflowKeys = append(overflowKeys, k)
				return false
			case EnforcementStripAndReaggregate:
				p.dropsThisEpoch.Add(1)
				if maxLog := p.config.DropLogMaxPerEpoch; maxLog == 0 || p.dropLogCount.Add(1) <= int64(maxLog) {
					p.logger.Warn("Dropping high-cardinality attribute",
						zap.String("metric", metricName),
						zap.String("key", k))
				}
				return true
			}
		}
		return false
	})

	if shouldTag {
		attrs.PutBool("otel.metric.overflow", true)
	}
	for _, k := range overflowKeys {
		attrs.PutStr(k, overflowSentinel)
	}
}

func (p *cardinalityProcessor) processNumberDataPointsWithMode(metricName string, dps pmetric.NumberDataPointSlice, mode EnforcementMode) {
	for i := 0; i < dps.Len(); i++ {
		p.handleAttributesWithMode(metricName, dps.At(i).Attributes(), mode)
	}
}

func (p *cardinalityProcessor) processHistogramDataPointsWithMode(metricName string, dps pmetric.HistogramDataPointSlice, mode EnforcementMode) {
	for i := 0; i < dps.Len(); i++ {
		p.handleAttributesWithMode(metricName, dps.At(i).Attributes(), mode)
	}
}

func (p *cardinalityProcessor) processExponentialHistogramDataPointsWithMode(metricName string, dps pmetric.ExponentialHistogramDataPointSlice, mode EnforcementMode) {
	for i := 0; i < dps.Len(); i++ {
		p.handleAttributesWithMode(metricName, dps.At(i).Attributes(), mode)
	}
}

func (p *cardinalityProcessor) processSummaryDataPointsWithMode(metricName string, dps pmetric.SummaryDataPointSlice, mode EnforcementMode) {
	for i := 0; i < dps.Len(); i++ {
		p.handleAttributesWithMode(metricName, dps.At(i).Attributes(), mode)
	}
}

func (p *cardinalityProcessor) isProtected(key string) bool {
	_, ok := p.protectedLabels[key]
	return ok
}

func (p *cardinalityProcessor) rotate() {
	p.logger.Debug("Rotating cardinality sketches")

	topN := p.config.TopOffendersCount
	var topBuf []offenderEntry
	if topN > 0 {
		topBuf = make([]offenderEntry, 0, topN)
	}

	for _, shard := range p.shards {
		shard.mu.RLock()
		entries := make([]trackerEntry, 0, len(shard.trackers))
		for k, t := range shard.trackers {
			entries = append(entries, trackerEntry{key: k, t: t})
		}
		shard.mu.RUnlock()

		topBuf = collectShardDeltas(entries, topBuf, topN)

		fresh := make([]*hyperloglog.Sketch, len(entries))
		for i := range entries {
			fresh[i] = mustGetSketch()
		}

		var staleKeys []trackerKey
		for i, e := range entries {
			idle := e.t.rotate(fresh[i])
			if idle && e.t.idleEpochs >= staleSweepEpochs {
				staleKeys = append(staleKeys, e.key)
			}
		}

		if len(staleKeys) > 0 {
			shard.mu.Lock()
			for _, k := range staleKeys {
				delete(shard.trackers, k)
				p.trackerCount.Add(-1)
			}
			shard.mu.Unlock()
			p.logger.Debug("Evicted stale trackers", zap.Int("count", len(staleKeys)))
		}
	}

	p.rejectionWarned.Store(false)

	totalDrops := p.dropsThisEpoch.Swap(0)
	logged := p.dropLogCount.Swap(0)
	suppressed := totalDrops - logged
	if suppressed > 0 {
		p.logger.Info("Suppressed drop log entries this epoch",
			zap.Int64("suppressed", suppressed),
			zap.Int64("total_drops", totalDrops))
	}

	p.publishTopOffenders(topBuf)
}

func collectShardDeltas(entries []trackerEntry, topBuf []offenderEntry, topN int) []offenderEntry {
	if topN <= 0 {
		return topBuf
	}
	for _, e := range entries {
		e.t.mu.Lock()
		curr, prev := e.t.cachedCurr, e.t.cachedPrev
		e.t.mu.Unlock()
		if curr <= prev {
			continue
		}
		delta := curr - prev

		if len(topBuf) < topN {
			topBuf = append(topBuf, offenderEntry{
				metricName: e.key.metricName,
				labelKey:   e.key.attrKey,
				delta:      delta,
			})
			continue
		}

		minIdx := 0
		for i := 1; i < len(topBuf); i++ {
			if topBuf[i].delta < topBuf[minIdx].delta {
				minIdx = i
			}
		}
		if delta > topBuf[minIdx].delta {
			topBuf[minIdx] = offenderEntry{
				metricName: e.key.metricName,
				labelKey:   e.key.attrKey,
				delta:      delta,
			}
		}
	}
	return topBuf
}

func (p *cardinalityProcessor) publishTopOffenders(topBuf []offenderEntry) {
	if len(topBuf) == 0 {
		return
	}
	sortOffenders(topBuf)
	p.topOffendersMu.Lock()
	p.topOffenders = topBuf
	p.topOffendersMu.Unlock()

	p.logger.Info("Top cardinality offender",
		zap.String("metric", topBuf[0].metricName),
		zap.String("label", topBuf[0].labelKey),
		zap.Uint64("delta", topBuf[0].delta))
}

func sortOffenders(s []offenderEntry) {
	for i := 1; i < len(s); i++ {
		key := s[i]
		j := i - 1
		for j >= 0 && s[j].delta < key.delta {
			s[j+1] = s[j]
			j--
		}
		s[j+1] = key
	}
}

// Start launches the background epoch-rotation goroutine exactly once.
func (p *cardinalityProcessor) Start(_ context.Context, _ component.Host) error {
	p.startOnce.Do(func() {
		p.logger.Info("Starting cardinality rotation ticker",
			zap.Int("interval_seconds", p.config.EpochDurationSeconds))

		go func() {
			ticker := time.NewTicker(time.Duration(p.config.EpochDurationSeconds) * time.Second)
			defer ticker.Stop()

			for {
				select {
				case <-ticker.C:
					p.rotate()
				case <-p.ctx.Done():
					return
				}
			}
		}()
	})
	return nil
}

// Shutdown unregisters telemetry callbacks and stops the background goroutine.
func (p *cardinalityProcessor) Shutdown(_ context.Context) error {
	p.logger.Info("Shutting down cardinality processor")
	if p.telemetry != nil {
		p.telemetry.Shutdown()
	}
	p.cancel()
	return nil
}

func (p *cardinalityProcessor) shouldDrop(metricName string, attrKey string, attrVal string) bool {
	key := trackerKey{metricName: metricName, attrKey: attrKey}
	hashVal := xxhash.Sum64String(attrVal)
	shard := p.getShard(metricName)

	shard.mu.RLock()
	t, ok := shard.trackers[key]
	shard.mu.RUnlock()

	if !ok {
		shard.mu.Lock()
		t, ok = shard.trackers[key]
		if !ok {
			if maxT := p.config.MaxTrackerCount; maxT > 0 && p.trackerCount.Load() >= int64(maxT) {
				shard.mu.Unlock()
				p.trackersRejected.Add(1)
				if p.rejectionWarned.CompareAndSwap(false, true) {
					p.logger.Warn("MaxTrackerCount reached; ignoring new metric attributes until next epoch rotation",
						zap.Int("limit", maxT))
				}
				return false
			}
			t = newTracker()
			shard.trackers[key] = t
			p.trackerCount.Add(1)
		}
		shard.mu.Unlock()
	}

	currCount, prevCount := t.insert(hashVal)
	if currCount <= prevCount {
		return false
	}
	return (currCount - prevCount) > p.getLimit(metricName)
}

func (p *cardinalityProcessor) getLimit(metricName string) uint64 {
	if v, ok := p.config.MetricOverrides[metricName]; ok {
		return uint64(v)
	}
	return uint64(p.config.MaxCardinalityDeltaPerEpoch)
}
