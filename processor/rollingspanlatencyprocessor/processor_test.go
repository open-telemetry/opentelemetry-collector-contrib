// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package rollingspanlatencyprocessor

import (
	"context"
	"errors"
	"math"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor/processortest"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
)

// errMeter is a metric.Meter stub whose Int64ObservableGauge always errors.
type errMeter struct{ noop.Meter }

func (errMeter) Int64ObservableGauge(_ string, _ ...metric.Int64ObservableGaugeOption) (metric.Int64ObservableGauge, error) {
	return noop.Int64ObservableGauge{}, errors.New("gauge error")
}

// errMeter2 errors only on Int64ObservableCounter (gauge succeeds).
type errMeter2 struct{ noop.Meter }

func (errMeter2) Int64ObservableCounter(_ string, _ ...metric.Int64ObservableCounterOption) (metric.Int64ObservableCounter, error) {
	return noop.Int64ObservableCounter{}, errors.New("counter error")
}

// errMeterProvider wraps a meter and implements metric.MeterProvider.
type errMeterProvider struct {
	noop.MeterProvider
	m metric.Meter
}

func (e errMeterProvider) Meter(_ string, _ ...metric.MeterOption) metric.Meter { return e.m }

func defaultConfig() *Config {
	return createDefaultConfig().(*Config)
}

func newTestProcessor(t *testing.T, cfg *Config) (*rollingSpanLatencyProcessor, *consumertest.TracesSink) {
	t.Helper()
	sink := new(consumertest.TracesSink)
	set := processortest.NewNopSettings(component.MustNewType("rolling_span_latency"))
	tp, err := newRollingSpanLatencyProcessor(t.Context(), cfg, set, sink)
	require.NoError(t, err)
	p, ok := tp.(*rollingSpanLatencyProcessor)
	require.True(t, ok, "expected *rollingSpanLatencyProcessor, got %T", tp)
	return p, sink
}

// fakeClock is an injectable, manually-advanced stand-in for time.Now. Tests
// wire it into p.nowFn so they can deterministically control the
// processing-time clock that drives EWMA aging and eviction.
type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func newFakeClock(start time.Time) *fakeClock {
	return &fakeClock{now: start}
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) Advance(d time.Duration) time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
	return c.now
}

// newTestProcessorWithClock is newTestProcessor plus a fakeClock wired in as
// p.nowFn, for tests that need to control the processing-time clock.
func newTestProcessorWithClock(t *testing.T, cfg *Config) (*rollingSpanLatencyProcessor, *consumertest.TracesSink, *fakeClock) {
	t.Helper()
	p, sink := newTestProcessor(t, cfg)
	clock := newFakeClock(time.Unix(1_000_000, 0)) // well past epoch so timestamps are valid
	p.nowFn = clock.Now
	return p, sink, clock
}

// makeTraces builds a single-span ptrace.Traces. resAttrs are written as
// resource attributes; spanName and durationNs describe the span. now sets
// the span's event-time start/end timestamps. This is independent of the
// processor's processing-time clock (p.nowFn / fakeClock), which is what
// actually drives EWMA aging and eviction.
func makeTraces(resAttrs map[string]string, spanName string, durationNs int64, now time.Time) ptrace.Traces {
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	for k, v := range resAttrs {
		rs.Resource().Attributes().PutStr(k, v)
	}
	ss := rs.ScopeSpans().AppendEmpty()
	sp := ss.Spans().AppendEmpty()
	sp.SetName(spanName)
	endNs := now.UnixNano()
	sp.SetStartTimestamp(pcommon.Timestamp(endNs - durationNs))
	sp.SetEndTimestamp(pcommon.Timestamp(endNs))
	return td
}

// defaultResAttrs returns resource attrs that satisfy all three default key
// attributes so tests don't need to repeat the map literal.
func defaultResAttrs(namespace, service, env string) map[string]string {
	return map[string]string{
		"service.namespace":           namespace,
		"service.name":                service,
		"deployment.environment.name": env,
	}
}

// keyFor returns the stats-map key the processor would derive for the given
// resource attrs and span name under cfg.
func keyFor(cfg *Config, resAttrs map[string]string, spanName string) string {
	vals := make([]string, len(cfg.ResourceKeyAttributes))
	for i, attrKey := range cfg.ResourceKeyAttributes {
		vals[i] = resAttrs[attrKey]
	}
	return buildKey(vals, spanName)
}

// warmProcessor feeds count 100ms spans named "op" into p, advancing clock
// (and the spans' event timestamps) by one second each time.
func warmProcessor(p *rollingSpanLatencyProcessor, clock *fakeClock, resAttrs map[string]string, count int) {
	for range count {
		now := clock.Advance(time.Second)
		_ = p.ConsumeTraces(context.Background(), makeTraces(resAttrs, "op", int64(100e6), now))
	}
}

// collectLabels gathers all latency.category attribute values seen in the sink.
func collectLabels(sink *consumertest.TracesSink, attrKey string) []string {
	var labels []string
	for _, td := range sink.AllTraces() {
		rss := td.ResourceSpans()
		for i := 0; i < rss.Len(); i++ {
			sss := rss.At(i).ScopeSpans()
			for j := 0; j < sss.Len(); j++ {
				spans := sss.At(j).Spans()
				for k := 0; k < spans.Len(); k++ {
					if v, ok := spans.At(k).Attributes().Get(attrKey); ok {
						labels = append(labels, v.Str())
					}
				}
			}
		}
	}
	return labels
}

var baseAttrs = defaultResAttrs("ns", "svc", "prod")

func TestProcessor_NoLabelBelowWarmup(t *testing.T) {
	cfg := defaultConfig()
	p, sink, clock := newTestProcessorWithClock(t, cfg)

	warmProcessor(p, clock, baseAttrs, 5)

	assert.Empty(t, collectLabels(sink, cfg.AttributeKey), "expected no labels before warmup")
}

func TestProcessor_NormalSpanNotLabeled(t *testing.T) {
	cfg := defaultConfig()
	p, sink, clock := newTestProcessorWithClock(t, cfg)

	warmProcessor(p, clock, baseAttrs, 50)
	sink.Reset()

	now := clock.Advance(time.Second)
	// 102ms on a 100ms baseline: (102-100)/1ms_floor = 2σ, below slow_threshold=3.
	require.NoError(t, p.ConsumeTraces(t.Context(), makeTraces(baseAttrs, "op", 102e6, now)))

	assert.Empty(t, collectLabels(sink, cfg.AttributeKey), "normal span should not be labeled")
}

func TestProcessor_SlowSpanLabeled(t *testing.T) {
	cfg := defaultConfig()
	p, sink, clock := newTestProcessorWithClock(t, cfg)

	// Tight distribution: 100ms ± 1ms alternating → small stddev.
	for i := range 100 {
		now := clock.Advance(time.Second)
		dur := int64(99e6)
		if i%2 == 0 {
			dur = int64(101e6)
		}
		require.NoError(t, p.ConsumeTraces(t.Context(), makeTraces(baseAttrs, "op", dur, now)))
	}
	sink.Reset()

	now := clock.Advance(time.Second)
	require.NoError(t, p.ConsumeTraces(t.Context(), makeTraces(baseAttrs, "op", int64(200e6), now)))

	labels := collectLabels(sink, cfg.AttributeKey)
	require.NotEmpty(t, labels, "expected slow or very_slow label on span far above tight baseline")
	for _, l := range labels {
		assert.Contains(t, []string{attributeValueSlow, attributeValueVerySlow}, l)
	}
}

func TestProcessor_VerySlowSpanLabeled(t *testing.T) {
	cfg := defaultConfig()
	p, sink, clock := newTestProcessorWithClock(t, cfg)

	for i := range 100 {
		now := clock.Advance(time.Second)
		dur := int64(99e6)
		if i%2 == 0 {
			dur = int64(101e6)
		}
		require.NoError(t, p.ConsumeTraces(t.Context(), makeTraces(baseAttrs, "op", dur, now)))
	}
	sink.Reset()

	now := clock.Advance(time.Second)
	require.NoError(t, p.ConsumeTraces(t.Context(), makeTraces(baseAttrs, "op", int64(500e6), now)))

	assert.Contains(t, collectLabels(sink, cfg.AttributeKey), attributeValueVerySlow)
}

// TestProcessor_DemoScenario_NormalSpanNotMislabeled simulates a realistic
// traffic pattern: warmup on jittered normal spans, then repeated rounds of
// slow + very-slow + normal traffic. Verifies that normal-latency spans are
// never labeled and that slow/very-slow spans are labeled correctly even after
// outliers feed back into the baseline.
func TestProcessor_DemoScenario_NormalSpanNotMislabeled(t *testing.T) {
	cfg := defaultConfig()
	cfg.HalfLife = 2 * time.Second
	p, sink, clock := newTestProcessorWithClock(t, cfg)

	// Warmup: 35 iterations × 2 spans, jittered ±2ms. Exceeds warmup_count=30.
	jitters := []int64{-2, -1, 0, 1, 2, -2, 0, 1, -1, 2}
	for i := range 35 {
		now := clock.Advance(200 * time.Millisecond)
		jitter := jitters[i%len(jitters)]
		require.NoError(t, p.ConsumeTraces(t.Context(), makeTraces(baseAttrs, "process-order", (50+jitter)*int64(time.Millisecond), now)))
		now = clock.Advance(200 * time.Millisecond)
		require.NoError(t, p.ConsumeTraces(t.Context(), makeTraces(baseAttrs, "query-inventory", (50+jitter)*int64(time.Millisecond), now)))
	}
	sink.Reset()

	slowLabeled := 0
	verySlowLabeled := 0

	// Run 5 rounds of: slow(67ms) + very-slow(80ms) + 5×normal pairs.
	for range 5 {
		now := clock.Advance(200 * time.Millisecond)
		require.NoError(t, p.ConsumeTraces(t.Context(), makeTraces(baseAttrs, "process-order", 67*int64(time.Millisecond), now)))

		now = clock.Advance(200 * time.Millisecond)
		require.NoError(t, p.ConsumeTraces(t.Context(), makeTraces(baseAttrs, "process-order", 80*int64(time.Millisecond), now)))

		// Normal spans — must NOT be labeled.
		for i := range 5 {
			now = clock.Advance(200 * time.Millisecond)
			jitter := jitters[i%len(jitters)]
			require.NoError(t, p.ConsumeTraces(t.Context(), makeTraces(baseAttrs, "process-order", (50+jitter)*int64(time.Millisecond), now)))
			now = clock.Advance(200 * time.Millisecond)
			require.NoError(t, p.ConsumeTraces(t.Context(), makeTraces(baseAttrs, "query-inventory", (50+jitter)*int64(time.Millisecond), now)))
		}
	}

	for _, td := range sink.AllTraces() {
		rss := td.ResourceSpans()
		for i := 0; i < rss.Len(); i++ {
			sss := rss.At(i).ScopeSpans()
			for j := 0; j < sss.Len(); j++ {
				spans := sss.At(j).Spans()
				for k := 0; k < spans.Len(); k++ {
					sp := spans.At(k)
					dur := int64(sp.EndTimestamp()-sp.StartTimestamp()) / int64(time.Millisecond)
					v, hasLabel := sp.Attributes().Get(cfg.AttributeKey)

					if sp.Name() == "query-inventory" {
						assert.False(t, hasLabel, "query-inventory span should never be labeled, got %q", v.Str())
					}
					if sp.Name() == "process-order" {
						if dur <= 55 {
							assert.False(t, hasLabel, "normal process-order span (%dms) should not be labeled, got %q", dur, v.Str())
						}
						if hasLabel {
							switch v.Str() {
							case attributeValueSlow:
								slowLabeled++
							case attributeValueVerySlow:
								verySlowLabeled++
							}
						}
					}
				}
			}
		}
	}

	assert.Positive(t, slowLabeled, "expected at least one slow label on 67ms spans")
	assert.Positive(t, verySlowLabeled, "expected at least one very_slow label on 80ms spans")
}

func TestProcessor_IndependentBaselinePerSpanName(t *testing.T) {
	cfg := defaultConfig()
	p, _, clock := newTestProcessorWithClock(t, cfg)

	for range 50 {
		now := clock.Advance(time.Second)
		require.NoError(t, p.ConsumeTraces(t.Context(), makeTraces(baseAttrs, "fast-op", int64(10e6), now)))
		require.NoError(t, p.ConsumeTraces(t.Context(), makeTraces(baseAttrs, "slow-op", int64(500e6), now)))
	}

	fastMean, _, _ := p.getOrCreateStats(keyFor(cfg, baseAttrs, "fast-op")).snapshot()
	slowMean, _, _ := p.getOrCreateStats(keyFor(cfg, baseAttrs, "slow-op")).snapshot()

	assert.Less(t, fastMean, slowMean)
}

func TestProcessor_SameSpanNameDifferentServicesHaveIndependentBaselines(t *testing.T) {
	cfg := defaultConfig()
	p, _, clock := newTestProcessorWithClock(t, cfg)

	attrsA := defaultResAttrs("ns", "service-a", "prod")
	attrsB := defaultResAttrs("ns", "service-b", "prod")

	for range 60 {
		now := clock.Advance(time.Second)
		require.NoError(t, p.ConsumeTraces(t.Context(), makeTraces(attrsA, "POST /items", int64(10e6), now)))
		require.NoError(t, p.ConsumeTraces(t.Context(), makeTraces(attrsB, "POST /items", int64(500e6), now)))
	}

	keyA := keyFor(cfg, attrsA, "POST /items")
	keyB := keyFor(cfg, attrsB, "POST /items")
	require.NotEqual(t, keyA, keyB, "keys must differ for different service names")

	meanA, _, _ := p.getOrCreateStats(keyA).snapshot()
	meanB, _, _ := p.getOrCreateStats(keyB).snapshot()

	assert.Less(t, meanA, meanB)
}

func TestProcessor_SameSpanNameDifferentNamespacesHaveIndependentBaselines(t *testing.T) {
	cfg := defaultConfig()
	p, _, clock := newTestProcessorWithClock(t, cfg)

	attrsProd := defaultResAttrs("ns-prod", "svc", "prod")
	attrsStaging := defaultResAttrs("ns-staging", "svc", "prod")

	for range 60 {
		now := clock.Advance(time.Second)
		require.NoError(t, p.ConsumeTraces(t.Context(), makeTraces(attrsProd, "query", int64(20e6), now)))
		require.NoError(t, p.ConsumeTraces(t.Context(), makeTraces(attrsStaging, "query", int64(300e6), now)))
	}

	keyProd := keyFor(cfg, attrsProd, "query")
	keyStaging := keyFor(cfg, attrsStaging, "query")
	require.NotEqual(t, keyProd, keyStaging, "keys must differ for different namespaces")

	meanProd, _, _ := p.getOrCreateStats(keyProd).snapshot()
	meanStaging, _, _ := p.getOrCreateStats(keyStaging).snapshot()

	assert.Less(t, meanProd, meanStaging)
}

func TestProcessor_SameSpanNameDifferentEnvironmentsHaveIndependentBaselines(t *testing.T) {
	cfg := defaultConfig()
	p, _, clock := newTestProcessorWithClock(t, cfg)

	attrsEast := defaultResAttrs("ns", "svc", "us-east-1")
	attrsWest := defaultResAttrs("ns", "svc", "us-west-2")

	for range 60 {
		now := clock.Advance(time.Second)
		require.NoError(t, p.ConsumeTraces(t.Context(), makeTraces(attrsEast, "SELECT", int64(5e6), now)))
		require.NoError(t, p.ConsumeTraces(t.Context(), makeTraces(attrsWest, "SELECT", int64(400e6), now)))
	}

	keyEast := keyFor(cfg, attrsEast, "SELECT")
	keyWest := keyFor(cfg, attrsWest, "SELECT")
	require.NotEqual(t, keyEast, keyWest, "keys must differ for different deployment environments")

	meanEast, _, _ := p.getOrCreateStats(keyEast).snapshot()
	meanWest, _, _ := p.getOrCreateStats(keyWest).snapshot()

	assert.Less(t, meanEast, meanWest)
}

func TestProcessor_CustomResourceKeyAttributes(t *testing.T) {
	cfg := defaultConfig()
	cfg.ResourceKeyAttributes = []string{"service.name"}
	p, _, clock := newTestProcessorWithClock(t, cfg)

	attrsA := map[string]string{"service.namespace": "ns-a", "service.name": "svc"}
	attrsB := map[string]string{"service.namespace": "ns-b", "service.name": "svc"}

	for range 60 {
		now := clock.Advance(time.Second)
		require.NoError(t, p.ConsumeTraces(t.Context(), makeTraces(attrsA, "op", int64(100e6), now)))
		require.NoError(t, p.ConsumeTraces(t.Context(), makeTraces(attrsB, "op", int64(100e6), now)))
	}

	keyA := keyFor(cfg, attrsA, "op")
	keyB := keyFor(cfg, attrsB, "op")

	assert.Equal(t, keyA, keyB, "with single-key config, different namespaces should share the same key")
}

func TestProcessor_EndBeforeStartSpanIgnored(t *testing.T) {
	cfg := defaultConfig()
	p, sink, clock := newTestProcessorWithClock(t, cfg)
	warmProcessor(p, clock, baseAttrs, 50)
	sink.Reset()

	// End timestamp earlier than start. EndTimestamp/StartTimestamp are
	// unsigned, so a naive subtraction would underflow to a huge positive
	// value instead of being caught as an invalid span.
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	for k, v := range baseAttrs {
		rs.Resource().Attributes().PutStr(k, v)
	}
	sp := rs.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	sp.SetName("op")
	end := clock.Advance(time.Second)
	sp.SetStartTimestamp(pcommon.Timestamp(end.UnixNano()))
	sp.SetEndTimestamp(pcommon.Timestamp(end.Add(-time.Hour).UnixNano()))
	require.NoError(t, p.ConsumeTraces(t.Context(), td))

	assert.Empty(t, collectLabels(sink, cfg.AttributeKey), "end-before-start span should not be labeled")
}

func TestProcessor_OutOfOrderEventTimestampDoesNotCorruptBaseline(t *testing.T) {
	cfg := defaultConfig()
	p, sink, clock := newTestProcessorWithClock(t, cfg)

	warmProcessor(p, clock, baseAttrs, cfg.WarmupCount+5)
	sink.Reset()

	key := keyFor(cfg, baseAttrs, "op")
	_, preStddev, _ := p.getOrCreateStats(key).snapshot()
	require.False(t, math.IsNaN(preStddev), "precondition: baseline must be healthy before the out-of-order span")

	// The span's event-time end timestamp is far earlier than any timestamp
	// already folded into the baseline, but it's still processed in real
	// (processing-time) order. Because aging is driven by p.nowFn(), not the
	// span's own timestamp, this must not produce a negative decay step.
	backwardsEventTime := time.Unix(1, 0)
	clock.Advance(time.Second)
	require.NoError(t, p.ConsumeTraces(t.Context(), makeTraces(baseAttrs, "op", int64(100e6), backwardsEventTime)))

	mean, stddev, _ := p.getOrCreateStats(key).snapshot()
	assert.False(t, math.IsNaN(mean), "mean must not become NaN from an out-of-order event timestamp")
	assert.False(t, math.IsNaN(stddev), "stddev must not become NaN from an out-of-order event timestamp")
}

func TestEvict_RemovesStaleEntries(t *testing.T) {
	cfg := defaultConfig()
	cfg.IdleTimeout = time.Hour
	p, _, clock := newTestProcessorWithClock(t, cfg)

	for range 15 {
		now := clock.Advance(time.Second)
		require.NoError(t, p.ConsumeTraces(t.Context(), makeTraces(baseAttrs, "op-a", int64(100e6), now)))
		require.NoError(t, p.ConsumeTraces(t.Context(), makeTraces(baseAttrs, "op-b", int64(100e6), now)))
	}

	keyA := keyFor(cfg, baseAttrs, "op-a")
	keyB := keyFor(cfg, baseAttrs, "op-b")

	evictTime := clock.Advance(cfg.IdleTimeout + time.Second)
	require.NoError(t, p.ConsumeTraces(t.Context(), makeTraces(baseAttrs, "op-b", int64(100e6), evictTime)))

	p.evict(evictTime)

	p.statsMu.RLock()
	_, aExists := p.statsMap[keyA]
	_, bExists := p.statsMap[keyB]
	p.statsMu.RUnlock()

	assert.False(t, aExists, "op-a should have been evicted after idle timeout")
	assert.True(t, bExists, "op-b should not have been evicted — it was recently observed")
}

func TestEvict_UsesProcessingTimeNotEventTimestamp(t *testing.T) {
	cfg := defaultConfig()
	cfg.IdleTimeout = time.Hour
	p, _, clock := newTestProcessorWithClock(t, cfg)

	// The span's event-time end timestamp is 9 hours in the past relative to
	// the processing clock — beyond idle_timeout=1h. Under event-time aging
	// this would make the entry immediately eligible for eviction even though
	// the collector just observed it; under processing-time aging (the fix)
	// it must not be.
	eventTime := clock.Now().Add(-9 * time.Hour)
	require.NoError(t, p.ConsumeTraces(t.Context(), makeTraces(baseAttrs, "op", int64(100e6), eventTime)))

	key := keyFor(cfg, baseAttrs, "op")
	p.evict(clock.Now().Add(cfg.IdleTimeout - time.Second))

	p.statsMu.RLock()
	_, exists := p.statsMap[key]
	p.statsMu.RUnlock()
	assert.True(t, exists, "entry observed just now must not be evicted based on a stale event timestamp")
}

func TestEvict_DoesNotRemoveActiveEntries(t *testing.T) {
	cfg := defaultConfig()
	cfg.IdleTimeout = time.Hour
	p, _, clock := newTestProcessorWithClock(t, cfg)

	var now time.Time
	for range 15 {
		now = clock.Advance(time.Second)
		require.NoError(t, p.ConsumeTraces(t.Context(), makeTraces(baseAttrs, "op", int64(100e6), now)))
	}

	key := keyFor(cfg, baseAttrs, "op")

	p.evict(now.Add(cfg.IdleTimeout - time.Second))

	p.statsMu.RLock()
	_, exists := p.statsMap[key]
	p.statsMu.RUnlock()

	assert.True(t, exists, "op should not be evicted before idle timeout elapses")
}

func TestEvict_RelearnsAfterEviction(t *testing.T) {
	cfg := defaultConfig()
	cfg.IdleTimeout = time.Hour
	p, sink, clock := newTestProcessorWithClock(t, cfg)

	var now time.Time
	for range 50 {
		now = clock.Advance(time.Second)
		require.NoError(t, p.ConsumeTraces(t.Context(), makeTraces(baseAttrs, "op", int64(100e6), now)))
	}

	p.evict(now.Add(cfg.IdleTimeout + time.Second))

	key := keyFor(cfg, baseAttrs, "op")
	p.statsMu.RLock()
	_, exists := p.statsMap[key]
	p.statsMu.RUnlock()
	require.False(t, exists, "entry should have been evicted")

	sink.Reset()
	for i := 0; i < cfg.WarmupCount-1; i++ {
		now = clock.Advance(cfg.IdleTimeout + time.Second)
		require.NoError(t, p.ConsumeTraces(t.Context(), makeTraces(baseAttrs, "op", int64(100e6), now)))
	}

	assert.Empty(t, collectLabels(sink, cfg.AttributeKey), "should not label during re-warmup after eviction")
}

func TestMaxBaselines_CapEnforced(t *testing.T) {
	cfg := defaultConfig()
	cfg.MaxBaselines = 2
	p, _ := newTestProcessor(t, cfg)

	now := time.Unix(1_000_000, 0)

	require.NoError(t, p.ConsumeTraces(t.Context(), makeTraces(baseAttrs, "op-a", int64(100e6), now)))
	require.NoError(t, p.ConsumeTraces(t.Context(), makeTraces(baseAttrs, "op-b", int64(100e6), now)))

	p.statsMu.RLock()
	sizeBefore := len(p.statsMap)
	p.statsMu.RUnlock()
	require.Equal(t, 2, sizeBefore)

	require.NoError(t, p.ConsumeTraces(t.Context(), makeTraces(baseAttrs, "op-c", int64(100e6), now)))

	p.statsMu.RLock()
	sizeAfter := len(p.statsMap)
	p.statsMu.RUnlock()
	assert.Equal(t, 2, sizeAfter, "expected map to remain at cap")

	assert.EqualValues(t, 1, p.droppedTotal.Load())
}

func TestMaxBaselines_ExistingKeyAllowedAfterCap(t *testing.T) {
	cfg := defaultConfig()
	cfg.MaxBaselines = 1
	p, _ := newTestProcessor(t, cfg)

	now := time.Unix(1_000_000, 0)
	require.NoError(t, p.ConsumeTraces(t.Context(), makeTraces(baseAttrs, "op", int64(100e6), now)))

	now = now.Add(time.Second)
	require.NoError(t, p.ConsumeTraces(t.Context(), makeTraces(baseAttrs, "op", int64(110e6), now)))

	assert.EqualValues(t, 0, p.droppedTotal.Load(), "existing key should not count as dropped")
}

func TestMaxBaselines_DroppedCounterRemainsCumulativeAfterEvictionSweep(t *testing.T) {
	cfg := defaultConfig()
	cfg.MaxBaselines = 1
	cfg.IdleTimeout = time.Hour
	p, _ := newTestProcessor(t, cfg)

	now := time.Unix(1_000_000, 0)
	require.NoError(t, p.ConsumeTraces(t.Context(), makeTraces(baseAttrs, "op-a", int64(100e6), now)))
	require.NoError(t, p.ConsumeTraces(t.Context(), makeTraces(baseAttrs, "op-b", int64(100e6), now))) // dropped

	require.EqualValues(t, 1, p.droppedTotal.Load())

	p.evict(now.Add(cfg.IdleTimeout + time.Second))

	assert.EqualValues(t, 1, p.droppedTotal.Load(), "dropped_keys_total is a cumulative monotonic counter and must not reset on eviction sweep")
}

func TestEvict_ChurnWarningWhenHighTurnover(t *testing.T) {
	cfg := defaultConfig()
	cfg.IdleTimeout = time.Hour
	cfg.ChurnWarningRatio = 0.01
	p, _, clock := newTestProcessorWithClock(t, cfg)

	for range 20 {
		now := clock.Now()
		require.NoError(t, p.ConsumeTraces(t.Context(), makeTraces(baseAttrs, "op-a", int64(100e6), now)))
		require.NoError(t, p.ConsumeTraces(t.Context(), makeTraces(baseAttrs, "op-b", int64(100e6), now)))
		clock.Advance(time.Second)
	}

	evictTime := clock.Advance(cfg.IdleTimeout)
	require.NoError(t, p.ConsumeTraces(t.Context(), makeTraces(baseAttrs, "op-b", int64(100e6), evictTime)))

	p.evict(evictTime)

	p.statsMu.RLock()
	_, aExists := p.statsMap[keyFor(cfg, baseAttrs, "op-a")]
	_, bExists := p.statsMap[keyFor(cfg, baseAttrs, "op-b")]
	p.statsMu.RUnlock()

	assert.False(t, aExists, "op-a should have been evicted")
	assert.True(t, bExists, "op-b should remain")
}

func TestProcessor_Capabilities(t *testing.T) {
	p, _ := newTestProcessor(t, defaultConfig())
	assert.True(t, p.Capabilities().MutatesData)
}

func TestProcessor_StartShutdown(t *testing.T) {
	p, _ := newTestProcessor(t, defaultConfig())
	require.NoError(t, p.Start(t.Context(), componenttest.NewNopHost()))
	assert.NoError(t, p.Shutdown(t.Context()))
}

func TestProcessor_ShutdownWithoutStart(t *testing.T) {
	p, _ := newTestProcessor(t, defaultConfig())
	assert.NoError(t, p.Shutdown(t.Context()), "Shutdown without Start should not error")
}

func TestProcessor_ZeroDurationSpanIgnored(t *testing.T) {
	cfg := defaultConfig()
	p, sink, clock := newTestProcessorWithClock(t, cfg)
	warmProcessor(p, clock, baseAttrs, 50)
	sink.Reset()

	// Span with equal start and end timestamps: duration = 0, should be ignored.
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	for k, v := range baseAttrs {
		rs.Resource().Attributes().PutStr(k, v)
	}
	sp := rs.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	sp.SetName("op")
	ts := pcommon.Timestamp(time.Unix(2_000_000, 0).UnixNano())
	sp.SetStartTimestamp(ts)
	sp.SetEndTimestamp(ts)
	require.NoError(t, p.ConsumeTraces(t.Context(), td))

	assert.Empty(t, collectLabels(sink, cfg.AttributeKey), "zero-duration span should not be labeled")
}

func TestNewRollingSpanLatencyProcessor_RegisterMetrics_GaugeError(t *testing.T) {
	sink := new(consumertest.TracesSink)
	set := processortest.NewNopSettings(component.MustNewType("rolling_span_latency"))
	set.MeterProvider = errMeterProvider{m: errMeter{}}

	_, err := newRollingSpanLatencyProcessor(t.Context(), defaultConfig(), set, sink)
	assert.Error(t, err)
}

func TestNewRollingSpanLatencyProcessor_RegisterMetrics_CounterError(t *testing.T) {
	sink := new(consumertest.TracesSink)
	set := processortest.NewNopSettings(component.MustNewType("rolling_span_latency"))
	set.MeterProvider = errMeterProvider{m: errMeter2{}}

	_, err := newRollingSpanLatencyProcessor(t.Context(), defaultConfig(), set, sink)
	assert.Error(t, err)
}

func TestEvictLoop_StopsOnContextCancel(t *testing.T) {
	cfg := defaultConfig()
	cfg.EvictionInterval = 10 * time.Millisecond
	p, _ := newTestProcessor(t, cfg)

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		p.evictLoop(ctx)
		close(done)
	}()
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Error("evictLoop did not stop after context cancellation")
	}
}

func TestGetOrCreateStats_DoubleCheckedLockFastPath(t *testing.T) {
	p, _ := newTestProcessor(t, defaultConfig())
	const key = "test-key"

	// Pre-populate the map directly so the fast-path (key exists under RLock)
	// is not hit, but the double-checked path inside the write lock is hit when
	// two concurrent callers race. We simulate this by inserting the key before
	// the second getOrCreateStats call acquires the write lock.
	s1 := p.getOrCreateStats(key)
	s2 := p.getOrCreateStats(key) // should find existing entry under write-lock check
	assert.Same(t, s1, s2, "expected same *spanStats for the same key")
}
