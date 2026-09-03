// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package rollingspanlatencyprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/rollingspanlatencyprocessor"

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/rollingspanlatencyprocessor/internal/metadata"
)

const (
	attributeValueSlow     = "slow"
	attributeValueVerySlow = "very_slow"
)

type rollingSpanLatencyProcessor struct {
	next         consumer.Traces
	logger       *zap.Logger
	telemetry    *metadata.TelemetryBuilder
	statsMap     map[string]*spanStats
	nowFn        func() time.Time
	cancelEvict  context.CancelFunc
	config       *Config
	droppedTotal atomic.Int64
	statsMu      sync.RWMutex
}

// buildKey returns a composite stats-map key from an ordered slice of resource
// attribute values and the span name. \x00 is the separator; it cannot appear
// in OTel attribute values in practice, so collisions are not possible.
func buildKey(resourceVals []string, spanName string) string {
	key := spanName
	for _, v := range resourceVals {
		key = v + "\x00" + key
	}
	return key
}

// newRollingSpanLatencyProcessor builds the rolling_span_latency processor,
// wiring the EWMA baseline tracker and attribute-labeling logic into the
// traces pipeline.
func newRollingSpanLatencyProcessor(
	_ context.Context,
	cfg *Config,
	set processor.Settings,
	nextConsumer consumer.Traces,
) (processor.Traces, error) {
	p := &rollingSpanLatencyProcessor{
		config:   cfg,
		logger:   set.Logger,
		next:     nextConsumer,
		statsMap: make(map[string]*spanStats),
		nowFn:    time.Now,
	}

	tb, err := metadata.NewTelemetryBuilder(set.TelemetrySettings)
	if err != nil {
		return nil, err
	}
	if err := tb.RegisterProcessorRollingSpanLatencyActiveBaselinesCallback(func(_ context.Context, o metric.Int64Observer) error {
		p.statsMu.RLock()
		n := int64(len(p.statsMap))
		p.statsMu.RUnlock()
		o.Observe(n)
		return nil
	}); err != nil {
		return nil, err
	}
	if err := tb.RegisterProcessorRollingSpanLatencyDroppedKeysTotalCallback(func(_ context.Context, o metric.Int64Observer) error {
		o.Observe(p.droppedTotal.Load())
		return nil
	}); err != nil {
		return nil, err
	}
	p.telemetry = tb

	return p, nil
}

func (*rollingSpanLatencyProcessor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}

func (p *rollingSpanLatencyProcessor) Start(ctx context.Context, _ component.Host) error {
	ctx, cancel := context.WithCancel(ctx)
	p.cancelEvict = cancel
	go p.evictLoop(ctx)
	return nil
}

func (p *rollingSpanLatencyProcessor) Shutdown(_ context.Context) error {
	if p.cancelEvict != nil {
		p.cancelEvict()
	}
	if p.telemetry != nil {
		p.telemetry.Shutdown()
	}
	return nil
}

func (p *rollingSpanLatencyProcessor) evictLoop(ctx context.Context) {
	ticker := time.NewTicker(p.config.EvictionInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.evict(p.nowFn())
		}
	}
}

func (p *rollingSpanLatencyProcessor) evict(now time.Time) {
	cutoff := now.Add(-p.config.IdleTimeout)

	p.statsMu.Lock()
	before := len(p.statsMap)
	for key, s := range p.statsMap {
		if s.idleSince().Before(cutoff) {
			delete(p.statsMap, key)
		}
	}
	after := len(p.statsMap)
	p.statsMu.Unlock()

	evicted := before - after
	dropped := p.droppedTotal.Load()

	if evicted > 0 {
		fields := []zap.Field{
			zap.Int("evicted", evicted),
			zap.Int("remaining", after),
			zap.Int64("dropped_since_last_sweep", dropped),
		}
		// Churn warning: evicted count exceeded the configured ratio of the
		// post-eviction map size. This indicates keys are turning over rapidly,
		// which often means span names contain high-cardinality values.
		if after > 0 && float64(evicted)/float64(after) > p.config.ChurnWarningRatio {
			p.logger.Warn("high baseline key churn detected — check for high-cardinality span names",
				fields...,
			)
		} else {
			p.logger.Debug("evicted stale span baselines", fields...)
		}
	}

	// Reset the per-interval drop counter now that we've reported it.
	p.droppedTotal.Store(0)
}

func (p *rollingSpanLatencyProcessor) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		rs := rss.At(i)
		resAttrs := rs.Resource().Attributes()
		resourceVals := make([]string, len(p.config.ResourceKeyAttributes))
		for idx, attrKey := range p.config.ResourceKeyAttributes {
			if v, ok := resAttrs.Get(attrKey); ok {
				resourceVals[idx] = v.Str()
			}
		}
		scopeSpans := rs.ScopeSpans()
		for j := 0; j < scopeSpans.Len(); j++ {
			spans := scopeSpans.At(j).Spans()
			for k := 0; k < spans.Len(); k++ {
				p.processSpan(spans.At(k), resourceVals)
			}
		}
	}
	return p.next.ConsumeTraces(ctx, td)
}

func (p *rollingSpanLatencyProcessor) processSpan(span ptrace.Span, resourceVals []string) {
	key := buildKey(resourceVals, span.Name())
	durationNs := float64(span.EndTimestamp() - span.StartTimestamp())
	if durationNs <= 0 {
		return
	}
	// Use the span's own end timestamp so spans within the same batch each
	// advance the EWMA clock correctly. A batch-shared wall-clock time would
	// give dt=0 for all but the first span, collapsing alpha to 0 and
	// leaving variance near-zero.
	now := time.Unix(0, int64(span.EndTimestamp()))

	stats := p.getOrCreateStats(key)
	if stats == nil {
		// Cap reached; this span has no baseline — skip attribute write.
		return
	}

	// Snapshot the baseline before updating so the current span is scored
	// against historical data only — prevents a single outlier from
	// inflating its own stddev and masking its own anomaly.
	preMean, preStddev, preCount := stats.snapshot()
	stats.update(durationNs, now, p.config.HalfLife)

	if preCount < int64(p.config.WarmupCount) {
		return
	}

	minStddev := float64(p.config.MinStddev.Nanoseconds())
	effectiveStddev := preStddev
	if effectiveStddev < minStddev {
		effectiveStddev = minStddev
	}

	deviations := (durationNs - preMean) / effectiveStddev
	switch {
	case deviations >= p.config.VerySlowThreshold:
		span.Attributes().PutStr(p.config.AttributeKey, attributeValueVerySlow)
	case deviations >= p.config.SlowThreshold:
		span.Attributes().PutStr(p.config.AttributeKey, attributeValueSlow)
	}
}

// getOrCreateStats returns the spanStats for key, creating it if absent.
// Returns nil when the max_baselines cap is reached and the key is new.
func (p *rollingSpanLatencyProcessor) getOrCreateStats(key string) *spanStats {
	p.statsMu.RLock()
	s, ok := p.statsMap[key]
	p.statsMu.RUnlock()
	if ok {
		return s
	}

	p.statsMu.Lock()
	defer p.statsMu.Unlock()
	// double-checked locking
	if s, ok = p.statsMap[key]; ok {
		return s
	}

	if p.config.MaxBaselines > 0 && len(p.statsMap) >= p.config.MaxBaselines {
		p.droppedTotal.Add(1)
		p.logger.Warn("max_baselines cap reached; dropping new baseline key",
			zap.String("key", key),
			zap.Int("max_baselines", p.config.MaxBaselines),
		)
		return nil
	}

	s = &spanStats{}
	p.statsMap[key] = s
	return s
}
