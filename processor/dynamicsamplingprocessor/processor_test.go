// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dynamicsamplingprocessor

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor/processortest"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/metric/metricdata/metricdatatest"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/dynamicsamplingprocessor/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/dynamicsamplingprocessor/internal/metadatatest"
)

func newTestProcessor(t *testing.T, cfg *Config, sink *consumertest.TracesSink) *dynamicSamplingProcessor {
	t.Helper()
	p, err := newProcessor(processortest.NewNopSettings(metadata.Type), cfg, sink)
	require.NoError(t, err)
	require.NoError(t, p.Start(t.Context(), nil))
	t.Cleanup(func() {
		require.NoError(t, p.Shutdown(t.Context()))
	})
	return p
}

func TestWarnUnreachableRules(t *testing.T) {
	tests := []struct {
		name      string
		rules     []RuleConfig
		wantWarn  bool
		wantField string
	}{
		{
			name: "catch_all_first_warns",
			rules: []RuleConfig{
				{Name: "default"},
				{Name: "keep-errors", Conditions: []string{"span.status.code == STATUS_CODE_ERROR"}},
			},
			wantWarn:  true,
			wantField: "default",
		},
		{
			name: "catch_all_last_no_warn",
			rules: []RuleConfig{
				{Name: "keep-errors", Conditions: []string{"span.status.code == STATUS_CODE_ERROR"}},
				{Name: "default"},
			},
		},
		{
			name: "single_catch_all_no_warn",
			rules: []RuleConfig{
				{Name: "default"},
			},
		},
		{
			name: "all_conditional_no_warn",
			rules: []RuleConfig{
				{Name: "errors", Conditions: []string{"span.status.code == STATUS_CODE_ERROR"}},
				{Name: "payment", Conditions: []string{"service.name == payment"}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			core, recorded := observer.New(zap.WarnLevel)
			warnUnreachableRules(zap.New(core), tt.rules)
			if !tt.wantWarn {
				assert.Zero(t, recorded.Len())
				return
			}
			require.Equal(t, 1, recorded.Len())
			entry := recorded.All()[0]
			assert.Equal(t, zap.WarnLevel, entry.Level)
			assert.Contains(t, entry.Message, "catch-all rule")
			assert.Equal(t, tt.wantField, entry.ContextMap()["rule"])
		})
	}
}

// newTrace builds a single-span ptrace.Traces with a non-empty ParentSpanID so
// the span is treated as a child (not a root). Use newRootTrace when the test
// needs to exercise root-span trigger behavior.
func newTrace(traceID pcommon.TraceID, statusCode ptrace.StatusCode) ptrace.Traces {
	return buildTrace(traceID, statusCode, false)
}

func newRootTrace(traceID pcommon.TraceID) ptrace.Traces {
	return buildTrace(traceID, ptrace.StatusCodeUnset, true)
}

func buildTrace(traceID pcommon.TraceID, statusCode ptrace.StatusCode, root bool) ptrace.Traces {
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()
	span := ss.Spans().AppendEmpty()
	span.SetTraceID(traceID)
	span.SetSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})
	if !root {
		span.SetParentSpanID([8]byte{9, 9, 9, 9, 9, 9, 9, 9})
	}
	span.SetName("op")
	span.Status().SetCode(statusCode)
	return td
}

// setTraceState overwrites the raw W3C tracestate on every span in td.
func setTraceState(td ptrace.Traces, raw string) {
	for _, rs := range td.ResourceSpans().All() {
		for _, ss := range rs.ScopeSpans().All() {
			for _, span := range ss.Spans().All() {
				span.TraceState().FromRaw(raw)
			}
		}
	}
}

// firstSpanTValue returns the ot=th value from the first span in td, or "" if
// the tracestate is missing or unparseable.
func firstSpanTValue(t *testing.T, td ptrace.Traces) string {
	t.Helper()
	span := td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
	w3c, err := sampling.NewW3CTraceState(span.TraceState().AsRaw())
	if err != nil {
		return ""
	}
	return w3c.OTelValue().TValue()
}

func TestProcessor_AlwaysSample_ForwardsAllTraces(t *testing.T) {
	sink := &consumertest.TracesSink{}
	cfg := &Config{
		TraceTimeout:  50 * time.Millisecond,
		DecisionDelay: 50 * time.Millisecond,
		NumTraces:     100,
		Rules: []RuleConfig{
			{Name: "keep-all", Sampler: SamplerConfig{Type: AlwaysSample}},
		},
	}
	p := newTestProcessor(t, cfg, sink)

	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4})
	require.NoError(t, p.ConsumeTraces(t.Context(), newTrace(traceID, ptrace.StatusCodeUnset)))

	assert.Eventually(t, func() bool {
		return sink.SpanCount() == 1
	}, time.Second, 10*time.Millisecond)
}

func TestProcessor_FirstMatchRouting(t *testing.T) {
	sink := &consumertest.TracesSink{}
	cfg := &Config{
		TraceTimeout:  50 * time.Millisecond,
		DecisionDelay: 50 * time.Millisecond,
		NumTraces:     100,
		Rules: []RuleConfig{
			{
				Name:       "keep-errors",
				Conditions: []string{"span.status.code == STATUS_CODE_ERROR"},
				Sampler:    SamplerConfig{Type: AlwaysSample},
			},
			{
				Name: "drop-rest",
				Sampler: SamplerConfig{
					Type:               Deterministic,
					SamplingPercentage: 100,
				},
			},
		},
	}
	p := newTestProcessor(t, cfg, sink)

	errTrace := pcommon.TraceID([16]byte{1})
	okTrace := pcommon.TraceID([16]byte{2})
	require.NoError(t, p.ConsumeTraces(t.Context(), newTrace(errTrace, ptrace.StatusCodeError)))
	require.NoError(t, p.ConsumeTraces(t.Context(), newTrace(okTrace, ptrace.StatusCodeOk)))

	assert.Eventually(t, func() bool {
		return sink.SpanCount() == 2
	}, time.Second, 10*time.Millisecond)

	// Both traces should be forwarded but with different rule attributions.
	rules := map[string]int{}
	for _, td := range sink.AllTraces() {
		for _, rs := range td.ResourceSpans().All() {
			for _, ss := range rs.ScopeSpans().All() {
				for _, span := range ss.Spans().All() {
					v, ok := span.Attributes().Get(ruleAttributeKey)
					require.True(t, ok)
					rules[v.AsString()]++
				}
			}
		}
	}
	assert.Equal(t, 1, rules["keep-errors"])
	assert.Equal(t, 1, rules["drop-rest"])
}

func TestProcessor_StampsRuleAndTraceState(t *testing.T) {
	sink := &consumertest.TracesSink{}
	cfg := &Config{
		TraceTimeout:  50 * time.Millisecond,
		DecisionDelay: 50 * time.Millisecond,
		NumTraces:     100,
		Rules: []RuleConfig{
			{Name: "default", Sampler: SamplerConfig{Type: AlwaysSample}},
		},
	}
	p := newTestProcessor(t, cfg, sink)

	traceID := pcommon.TraceID([16]byte{9})
	require.NoError(t, p.ConsumeTraces(t.Context(), newTrace(traceID, ptrace.StatusCodeUnset)))

	assert.Eventually(t, func() bool {
		return sink.SpanCount() == 1
	}, time.Second, 10*time.Millisecond)

	out := sink.AllTraces()[0]
	span := out.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
	rule, ok := span.Attributes().Get(ruleAttributeKey)
	require.True(t, ok)
	assert.Equal(t, "default", rule.AsString())

	ts := span.TraceState().AsRaw()
	assert.Contains(t, ts, "ot=")
	assert.Contains(t, ts, "th:")
}

func TestProcessor_DeterministicDropsAtRate(t *testing.T) {
	sink := &consumertest.TracesSink{}
	const n = 500
	cfg := &Config{
		TraceTimeout:  50 * time.Millisecond,
		DecisionDelay: 50 * time.Millisecond,
		// NumTraces must hold every trace we send, otherwise eviction skews
		// the observed sample count (eviction order is map-iteration order,
		// which differs across platforms).
		NumTraces: n,
		Rules: []RuleConfig{
			{
				Name: "fixed",
				Sampler: SamplerConfig{
					Type:               Deterministic,
					SamplingPercentage: 10, // 1-in-10
				},
			},
		},
	}
	p := newTestProcessor(t, cfg, sink)

	for i := range n {
		// Vary the last 7 bytes; W3C consistent sampling uses these as randomness.
		id := [16]byte{
			0, 0, 0, 0, 0, 0, 0, 0, 0,
			byte(i), byte(i >> 8), byte(i * 31), byte(i * 7), byte(i + 13), byte(i * 17), byte(i*11 + 5),
		}
		require.NoError(t, p.ConsumeTraces(t.Context(), newTrace(pcommon.TraceID(id), ptrace.StatusCodeUnset)))
	}

	// Wait for decisions to drain.
	assert.Eventually(t, func() bool {
		p.mu.Lock()
		defer p.mu.Unlock()
		return len(p.traces) == 0
	}, 2*time.Second, 20*time.Millisecond)

	// Expect roughly 10% sampled (~50 of 500). Allow a wide tolerance.
	count := sink.SpanCount()
	assert.Greater(t, count, 20, "expected some traces sampled, got %d", count)
	assert.Less(t, count, 100, "expected fewer than ~20%%, got %d", count)
}

func TestProcessor_Eviction(t *testing.T) {
	sink := &consumertest.TracesSink{}
	cfg := &Config{
		TraceTimeout:  time.Hour, // long enough we never decide before eviction
		DecisionDelay: time.Hour,
		NumTraces:     2,
		Rules: []RuleConfig{
			{Name: "default", Sampler: SamplerConfig{Type: AlwaysSample}},
		},
	}
	p := newTestProcessor(t, cfg, sink)

	for i := range byte(5) {
		id := pcommon.TraceID([16]byte{i})
		require.NoError(t, p.ConsumeTraces(t.Context(), newTrace(id, ptrace.StatusCodeUnset)))
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	assert.LessOrEqual(t, len(p.traces), 2, "buffer should be capped at NumTraces")
}

func TestProcessor_EvictionEvaluate_DecidesOldest(t *testing.T) {
	sink := &consumertest.TracesSink{}
	cfg := &Config{
		TraceTimeout:  time.Hour, // decisions can only come from eviction
		DecisionDelay: time.Hour,
		NumTraces:     2,
		DecisionCache: DecisionCacheConfig{SampledCacheSize: 10, NonSampledCacheSize: 10},
		Rules: []RuleConfig{
			{Name: "default", Sampler: SamplerConfig{Type: AlwaysSample}},
		},
	}
	p := newTestProcessor(t, cfg, sink)

	oldest := pcommon.TraceID([16]byte{0xA1})
	require.NoError(t, p.ConsumeTraces(t.Context(), newTrace(oldest, ptrace.StatusCodeUnset)))
	require.NoError(t, p.ConsumeTraces(t.Context(), newTrace(pcommon.TraceID([16]byte{0xA2}), ptrace.StatusCodeUnset)))
	// Third trace overflows the buffer and must evict the OLDEST trace, which
	// is decided immediately through the rules (always_sample keeps it).
	require.NoError(t, p.ConsumeTraces(t.Context(), newTrace(pcommon.TraceID([16]byte{0xA3}), ptrace.StatusCodeUnset)))

	require.Equal(t, 1, sink.SpanCount(), "evicted trace should be decided and forwarded synchronously")
	out := sink.AllTraces()[0]
	span := out.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
	assert.Equal(t, oldest, span.TraceID(), "the oldest trace should be the eviction victim")
	rule, ok := span.Attributes().Get(ruleAttributeKey)
	require.True(t, ok)
	assert.Equal(t, "default", rule.AsString())
	assert.Contains(t, span.TraceState().AsRaw(), "th:", "evicted-and-kept trace must carry ot=th")

	// The decision was recorded: a late span for the evicted trace takes the
	// sampled-cache fast path instead of forming a new partial trace.
	require.NoError(t, p.ConsumeTraces(t.Context(), newTrace(oldest, ptrace.StatusCodeUnset)))
	assert.Eventually(t, func() bool {
		return sink.SpanCount() == 2
	}, time.Second, 10*time.Millisecond)
	p.mu.Lock()
	_, buffered := p.traces[oldest]
	p.mu.Unlock()
	assert.False(t, buffered, "late span of evicted trace must not re-open a pending trace")
}

func TestProcessor_EvictionProbabilistic(t *testing.T) {
	sink := &consumertest.TracesSink{}
	cfg := &Config{
		TraceTimeout:  time.Hour,
		DecisionDelay: time.Hour,
		NumTraces:     1,
		DecisionCache: DecisionCacheConfig{SampledCacheSize: 10, NonSampledCacheSize: 10},
		Eviction:      EvictionConfig{Policy: EvictionProbabilistic, SamplingPercentage: 50},
		Rules: []RuleConfig{
			{Name: "default", Sampler: SamplerConfig{Type: AlwaysSample}},
		},
	}
	p := newTestProcessor(t, cfg, sink)

	// Randomness comes from the last 7 bytes of the traceID: all-zero sorts
	// below the 50% threshold (dropped), all-0xFF above it (kept).
	low := pcommon.TraceID([16]byte{0xB1})
	high := pcommon.TraceID([16]byte{0xB2, 0, 0, 0, 0, 0, 0, 0, 0, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF})

	require.NoError(t, p.ConsumeTraces(t.Context(), newTrace(low, ptrace.StatusCodeUnset)))
	require.NoError(t, p.ConsumeTraces(t.Context(), newTrace(high, ptrace.StatusCodeUnset)))                            // evicts low: dropped
	require.NoError(t, p.ConsumeTraces(t.Context(), newTrace(pcommon.TraceID([16]byte{0xB3}), ptrace.StatusCodeUnset))) // evicts high: kept

	require.Equal(t, 1, sink.SpanCount())
	span := sink.AllTraces()[0].ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
	assert.Equal(t, high, span.TraceID())
	rule, ok := span.Attributes().Get(ruleAttributeKey)
	require.True(t, ok)
	assert.Equal(t, evictionRuleLabel, rule.AsString(), "probabilistic evictions carry the sentinel rule label")
	assert.Contains(t, span.TraceState().AsRaw(), "th:8", "50%% keep must encode ot=th:8")

	// The dropped trace's decision is cached: its late spans are discarded.
	require.NoError(t, p.ConsumeTraces(t.Context(), newTrace(low, ptrace.StatusCodeUnset)))
	assert.Equal(t, 1, sink.SpanCount(), "late span of a probabilistically dropped trace must be discarded")
}

func TestProcessor_EvictionWithinSingleBatch(t *testing.T) {
	// Multiple new traces in one ConsumeTraces call with NumTraces: 1 forces
	// evictions before the evicted traces' timers were ever armed. Shutdown
	// hanging (leaked waitgroup slot) is the failure mode this guards.
	sink := &consumertest.TracesSink{}
	cfg := &Config{
		TraceTimeout:  time.Hour,
		DecisionDelay: time.Hour,
		NumTraces:     1,
		Rules: []RuleConfig{
			{Name: "default", Sampler: SamplerConfig{Type: AlwaysSample}},
		},
	}
	p := newTestProcessor(t, cfg, sink)

	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()
	for i := range byte(3) {
		span := ss.Spans().AppendEmpty()
		span.SetTraceID(pcommon.TraceID([16]byte{0xC0 + i}))
		span.SetSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})
		span.SetParentSpanID([8]byte{9, 9, 9, 9, 9, 9, 9, 9})
		span.SetName("op")
	}
	require.NoError(t, p.ConsumeTraces(t.Context(), td))

	assert.Equal(t, 2, sink.SpanCount(), "two of three traces should be evicted and decided")
	p.mu.Lock()
	assert.Len(t, p.traces, 1)
	p.mu.Unlock()
}

func TestProcessor_ArrivalListCompaction(t *testing.T) {
	// During normal operation (buffer never full) eviction never pops the
	// arrival list, so entries for decided traces must be reclaimed by
	// compaction or the list grows forever.
	sink := &consumertest.TracesSink{}
	cfg := &Config{
		TraceTimeout:  5 * time.Millisecond,
		DecisionDelay: 5 * time.Millisecond,
		NumTraces:     10_000,
		Rules: []RuleConfig{
			{Name: "default", Sampler: SamplerConfig{Type: AlwaysSample}},
		},
	}
	p := newTestProcessor(t, cfg, sink)

	const total = 2000
	for i := range total {
		var id pcommon.TraceID
		id[0] = byte(i)
		id[1] = byte(i >> 8)
		id[15] = 0x77
		require.NoError(t, p.ConsumeTraces(t.Context(), newTrace(id, ptrace.StatusCodeUnset)))
	}
	// Wait for the whole population to be decided through the normal timer
	// flow, then run one more batch so the end-of-batch compaction check
	// observes the (now almost entirely stale) arrival list.
	require.Eventually(t, func() bool {
		return sink.SpanCount() == total
	}, 30*time.Second, 10*time.Millisecond)
	require.NoError(t, p.ConsumeTraces(t.Context(), newTrace(pcommon.TraceID([16]byte{0xEE}), ptrace.StatusCodeUnset)))

	p.mu.Lock()
	arrivalLen := len(p.arrival)
	live := len(p.traces)
	p.mu.Unlock()
	assert.LessOrEqual(t, arrivalLen, arrivalCompactionFactor*live+arrivalCompactionFloor,
		"arrival list must be compacted once decided-trace entries dominate")
}

func TestProcessor_EvictionMetrics(t *testing.T) {
	tt := componenttest.NewTelemetry()
	t.Cleanup(func() {
		require.NoError(t, tt.Shutdown(context.Background())) //nolint:usetesting // cleanup after ctx cancel
	})

	sink := &consumertest.TracesSink{}
	cfg := &Config{
		TraceTimeout:  time.Hour,
		DecisionDelay: time.Hour,
		NumTraces:     1,
		Rules: []RuleConfig{
			{Name: "default", Sampler: SamplerConfig{Type: AlwaysSample}},
		},
	}
	p, err := newProcessor(metadatatest.NewSettings(tt), cfg, sink)
	require.NoError(t, err)
	require.NoError(t, p.Start(t.Context(), nil))
	t.Cleanup(func() { require.NoError(t, p.Shutdown(t.Context())) })

	require.NoError(t, p.ConsumeTraces(t.Context(), newTrace(pcommon.TraceID([16]byte{0xD1}), ptrace.StatusCodeUnset)))
	require.NoError(t, p.ConsumeTraces(t.Context(), newTrace(pcommon.TraceID([16]byte{0xD2}), ptrace.StatusCodeUnset)))

	metadatatest.AssertEqualProcessorDynamicSamplingTracesEvicted(t, tt,
		[]metricdata.DataPoint[int64]{{Value: 1}},
		metricdatatest.IgnoreTimestamp(),
		metricdatatest.IgnoreExemplars(),
	)
	metadatatest.AssertEqualProcessorDynamicSamplingDecisionTriggers(t, tt,
		[]metricdata.DataPoint[int64]{{
			Value:      1,
			Attributes: attribute.NewSet(attribute.String("trigger", "eviction")),
		}},
		metricdatatest.IgnoreTimestamp(),
		metricdatatest.IgnoreExemplars(),
	)
}

func TestProcessor_RootSpanTriggersEarlyDecision(t *testing.T) {
	sink := &consumertest.TracesSink{}
	cfg := &Config{
		TraceTimeout:  time.Hour, // never fires
		DecisionDelay: 50 * time.Millisecond,
		NumTraces:     10,
		Rules: []RuleConfig{
			{Name: "default", Sampler: SamplerConfig{Type: AlwaysSample}},
		},
	}
	p := newTestProcessor(t, cfg, sink)

	traceID := pcommon.TraceID([16]byte{0xAA})
	require.NoError(t, p.ConsumeTraces(t.Context(), newRootTrace(traceID)))

	assert.Eventually(t, func() bool {
		return sink.SpanCount() == 1
	}, time.Second, 10*time.Millisecond, "root span should trigger decision before trace_timeout")
}

func TestProcessor_RootSpanTriggerIsIdempotent(t *testing.T) {
	sink := &consumertest.TracesSink{}
	cfg := &Config{
		TraceTimeout:  time.Hour,
		DecisionDelay: 50 * time.Millisecond,
		NumTraces:     10,
		Rules: []RuleConfig{
			{Name: "default", Sampler: SamplerConfig{Type: AlwaysSample}},
		},
	}
	p := newTestProcessor(t, cfg, sink)

	traceID := pcommon.TraceID([16]byte{0xBB})
	require.NoError(t, p.ConsumeTraces(t.Context(), newRootTrace(traceID)))
	require.NoError(t, p.ConsumeTraces(t.Context(), newRootTrace(traceID)))
	require.NoError(t, p.ConsumeTraces(t.Context(), newRootTrace(traceID)))

	assert.Eventually(t, func() bool {
		return sink.SpanCount() == 3
	}, time.Second, 10*time.Millisecond)
	// Only one decision should have fired: total traces forwarded is one batch
	// for the trace, containing all three spans accumulated.
	assert.Len(t, sink.AllTraces(), 1)
}

func TestProcessor_TraceTimeoutFiresWithoutRootSpan(t *testing.T) {
	sink := &consumertest.TracesSink{}
	cfg := &Config{
		TraceTimeout:  50 * time.Millisecond,
		DecisionDelay: 50 * time.Millisecond,
		NumTraces:     10,
		Rules: []RuleConfig{
			{Name: "default", Sampler: SamplerConfig{Type: AlwaysSample}},
		},
	}
	p := newTestProcessor(t, cfg, sink)

	traceID := pcommon.TraceID([16]byte{0xCC})
	require.NoError(t, p.ConsumeTraces(t.Context(), newTrace(traceID, ptrace.StatusCodeUnset)))

	assert.Eventually(t, func() bool {
		return sink.SpanCount() == 1
	}, time.Second, 10*time.Millisecond, "trace_timeout should fire even without a root span")
}

func TestProcessor_LateSpansForSampledTraceForwarded(t *testing.T) {
	sink := &consumertest.TracesSink{}
	cfg := &Config{
		TraceTimeout:  50 * time.Millisecond,
		DecisionDelay: 50 * time.Millisecond,
		NumTraces:     10,
		DecisionCache: DecisionCacheConfig{SampledCacheSize: 100, NonSampledCacheSize: 100},
		Rules: []RuleConfig{
			{Name: "default", Sampler: SamplerConfig{Type: AlwaysSample}},
		},
	}
	p := newTestProcessor(t, cfg, sink)

	traceID := pcommon.TraceID([16]byte{0xDD})
	require.NoError(t, p.ConsumeTraces(t.Context(), newTrace(traceID, ptrace.StatusCodeUnset)))
	assert.Eventually(t, func() bool {
		return sink.SpanCount() == 1
	}, time.Second, 10*time.Millisecond)

	// Late span: same traceID, decision already cached as sampled.
	require.NoError(t, p.ConsumeTraces(t.Context(), newTrace(traceID, ptrace.StatusCodeUnset)))
	assert.Eventually(t, func() bool {
		return sink.SpanCount() == 2
	}, time.Second, 10*time.Millisecond, "late span on sampled trace should be forwarded immediately")

	late := sink.AllTraces()[1]
	span := late.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
	rule, ok := span.Attributes().Get(ruleAttributeKey)
	require.True(t, ok)
	assert.Equal(t, "default", rule.AsString())
	assert.Contains(t, span.TraceState().AsRaw(), "ot=")
}

func TestProcessor_LateSpansForDroppedTraceDropped(t *testing.T) {
	sink := &consumertest.TracesSink{}
	cfg := &Config{
		TraceTimeout:  50 * time.Millisecond,
		DecisionDelay: 50 * time.Millisecond,
		NumTraces:     10,
		DecisionCache: DecisionCacheConfig{SampledCacheSize: 100, NonSampledCacheSize: 100},
		Rules: []RuleConfig{
			// A rule that doesn't match anything, so all traces fall through
			// and end up dropped as unmatched.
			{
				Name:       "never",
				Conditions: []string{"span.status.code == 999"},
				Sampler:    SamplerConfig{Type: AlwaysSample},
			},
		},
	}
	p := newTestProcessor(t, cfg, sink)

	traceID := pcommon.TraceID([16]byte{0xEE})
	require.NoError(t, p.ConsumeTraces(t.Context(), newTrace(traceID, ptrace.StatusCodeUnset)))

	// Wait for the original decision to drain (trace removed from pending map).
	assert.Eventually(t, func() bool {
		p.mu.Lock()
		defer p.mu.Unlock()
		return len(p.traces) == 0
	}, time.Second, 10*time.Millisecond)
	require.Equal(t, 0, sink.SpanCount())

	// Late span: should be dropped via the non-sampled cache.
	require.NoError(t, p.ConsumeTraces(t.Context(), newTrace(traceID, ptrace.StatusCodeUnset)))
	// Verify nothing arrives and no new pending trace is created.
	time.Sleep(150 * time.Millisecond)
	assert.Equal(t, 0, sink.SpanCount())
	p.mu.Lock()
	assert.Empty(t, p.traces)
	p.mu.Unlock()
}

func TestEffectiveThreshold(t *testing.T) {
	th50pct, err := sampling.ProbabilityToThreshold(0.50)
	require.NoError(t, err)
	th20pct, err := sampling.ProbabilityToThreshold(0.20)
	require.NoError(t, err)
	th10pct, err := sampling.ProbabilityToThreshold(0.10)
	require.NoError(t, err)

	tests := []struct {
		name     string
		upstream sampling.Threshold
		rate     int
		want     sampling.Threshold
	}{
		{"rate_1_returns_upstream", th20pct, 1, th20pct},
		{"rate_0_returns_upstream", th20pct, 0, th20pct},
		{"no_upstream_ours_wins", sampling.AlwaysSampleThreshold, 10, th10pct},
		{"upstream_looser_ours_wins", th50pct, 10, th10pct},
		{"upstream_stricter_upstream_wins", th10pct, 5, th10pct},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := effectiveThreshold(tt.upstream, tt.rate)
			require.NoError(t, err)
			assert.InDelta(t, tt.want.AdjustedCount(), got.AdjustedCount(), 0.01,
				"got %v want %v", got.AdjustedCount(), tt.want.AdjustedCount())
		})
	}
}

func TestProcessor_HonoursIncomingThreshold(t *testing.T) {
	sink := &consumertest.TracesSink{}
	cfg := &Config{
		TraceTimeout:  50 * time.Millisecond,
		DecisionDelay: 50 * time.Millisecond,
		NumTraces:     10,
		Rules: []RuleConfig{
			{
				Name: "fixed",
				Sampler: SamplerConfig{
					Type:               Deterministic,
					SamplingPercentage: 10, // rate 10 = keep 10% of population
				},
			},
		},
	}
	p := newTestProcessor(t, cfg, sink)

	// Upstream at 50% keep. Under equalizing composition, our rate 10 (10% of
	// population) is stricter than upstream, so the emitted threshold should
	// represent 10% keep of the population, i.e. adjusted count 10.
	upstream, err := sampling.ProbabilityToThreshold(0.50)
	require.NoError(t, err)
	upstreamTValue := upstream.TValue()

	// TraceID randomness in the top of the space guarantees the span passes
	// both upstream (T=0.5M) and our threshold (T=0.9M).
	traceID := pcommon.TraceID([16]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff})
	td := newTrace(traceID, ptrace.StatusCodeUnset)
	setTraceState(td, "ot=th:"+upstreamTValue)

	require.NoError(t, p.ConsumeTraces(t.Context(), td))
	assert.Eventually(t, func() bool { return sink.SpanCount() == 1 }, time.Second, 10*time.Millisecond)

	out := sink.AllTraces()[0]
	tv := firstSpanTValue(t, out)
	th, err := sampling.TValueToThreshold(tv)
	require.NoError(t, err)
	// 10% keep of population ⇒ adjusted count 10.
	assert.InDelta(t, 10.0, th.AdjustedCount(), 0.01,
		"emitted adjusted count should reflect the equalizing rate")
}

func TestProcessor_UpstreamStricterThanRate_UpstreamWins(t *testing.T) {
	// Under equalizing, if upstream is already stricter than our configured
	// rate, upstream's threshold is preserved (we do not lower it).
	sink := &consumertest.TracesSink{}
	cfg := &Config{
		TraceTimeout:  50 * time.Millisecond,
		DecisionDelay: 50 * time.Millisecond,
		NumTraces:     10,
		Rules: []RuleConfig{
			{
				Name: "loose",
				Sampler: SamplerConfig{
					Type:               Deterministic,
					SamplingPercentage: 50, // rate 2 = 50% of population
				},
			},
		},
	}
	p := newTestProcessor(t, cfg, sink)

	// Upstream at 10% keep, stricter than our 50%.
	upstream, err := sampling.ProbabilityToThreshold(0.10)
	require.NoError(t, err)
	upstreamTValue := upstream.TValue()

	traceID := pcommon.TraceID([16]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff})
	td := newTrace(traceID, ptrace.StatusCodeUnset)
	setTraceState(td, "ot=th:"+upstreamTValue)

	require.NoError(t, p.ConsumeTraces(t.Context(), td))
	assert.Eventually(t, func() bool { return sink.SpanCount() == 1 }, time.Second, 10*time.Millisecond)

	out := sink.AllTraces()[0]
	tv := firstSpanTValue(t, out)
	th, err := sampling.TValueToThreshold(tv)
	require.NoError(t, err)
	// 10% keep (upstream) ⇒ adjusted count 10, not 2.
	assert.InDelta(t, 10.0, th.AdjustedCount(), 0.01,
		"upstream threshold should be preserved when stricter than the rule's rate")
}

func TestProcessor_UpstreamStricterThanOurs_LatePathHonoursUpstream(t *testing.T) {
	// Verify late-arriving spans keep a stricter incoming ot=th rather than
	// having it lowered by our cached decision. UpdateTValueWithSampling
	// refuses to weaken a stricter threshold, so the emitted th should equal
	// the incoming th on the late span.
	sink := &consumertest.TracesSink{}
	cfg := &Config{
		TraceTimeout:  50 * time.Millisecond,
		DecisionDelay: 50 * time.Millisecond,
		NumTraces:     10,
		DecisionCache: DecisionCacheConfig{SampledCacheSize: 100, NonSampledCacheSize: 100},
		Rules: []RuleConfig{
			{Name: "default", Sampler: SamplerConfig{Type: AlwaysSample}},
		},
	}
	p := newTestProcessor(t, cfg, sink)

	traceID := pcommon.TraceID([16]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff})
	require.NoError(t, p.ConsumeTraces(t.Context(), newTrace(traceID, ptrace.StatusCodeUnset)))
	assert.Eventually(t, func() bool { return sink.SpanCount() == 1 }, time.Second, 10*time.Millisecond)

	// Late span comes in with a stricter upstream th (1% keep). Cached decision
	// is AlwaysSample (100% keep). Emitted should keep the stricter value.
	strict, err := sampling.ProbabilityToThreshold(0.01)
	require.NoError(t, err)
	late := newTrace(traceID, ptrace.StatusCodeUnset)
	setTraceState(late, "ot=th:"+strict.TValue())
	require.NoError(t, p.ConsumeTraces(t.Context(), late))
	assert.Eventually(t, func() bool { return sink.SpanCount() == 2 }, time.Second, 10*time.Millisecond)

	tv := firstSpanTValue(t, sink.AllTraces()[1])
	th, err := sampling.TValueToThreshold(tv)
	require.NoError(t, err)
	assert.Equal(t, strict.Unsigned(), th.Unsigned(),
		"late span should retain the stricter incoming threshold, not be lowered by the cached decision")
}

func TestProcessor_UnparseableTracestateCounter(t *testing.T) {
	// Use a real componenttest.Telemetry so the counter actually records and
	// we can assert on it via the generated metadatatest helper.
	tt := componenttest.NewTelemetry()
	t.Cleanup(func() {
		// Cleanup runs after the test's context is canceled; use a fresh
		// background context so Shutdown can drain.
		require.NoError(t, tt.Shutdown(context.Background())) //nolint:usetesting // see comment
	})

	sink := &consumertest.TracesSink{}
	cfg := &Config{
		TraceTimeout:  50 * time.Millisecond,
		DecisionDelay: 50 * time.Millisecond,
		NumTraces:     10,
		Rules: []RuleConfig{
			{Name: "default", Sampler: SamplerConfig{Type: AlwaysSample}},
		},
	}
	p, err := newProcessor(metadatatest.NewSettings(tt), cfg, sink)
	require.NoError(t, err)
	require.NoError(t, p.Start(t.Context(), nil))
	t.Cleanup(func() { require.NoError(t, p.Shutdown(t.Context())) })

	traceID := pcommon.TraceID([16]byte{0xAB})
	td := newTrace(traceID, ptrace.StatusCodeUnset)
	// Malformed tracestate value that fails parse (invalid key char in vendor).
	setTraceState(td, "!!not-a-tracestate!!")
	require.NoError(t, p.ConsumeTraces(t.Context(), td))
	assert.Eventually(t, func() bool { return sink.SpanCount() == 1 }, time.Second, 10*time.Millisecond)

	metadatatest.AssertEqualProcessorDynamicSamplingIncomingTracestateUnparseable(t, tt,
		[]metricdata.DataPoint[int64]{{Value: 1}},
		metricdatatest.IgnoreTimestamp(),
		metricdatatest.IgnoreExemplars(),
	)
}

func TestProcessor_DivergentRVLogsWarning(t *testing.T) {
	// Two spans on the same trace carrying different ot=rv values is a
	// producer bug; the processor keeps the first one for the decision and
	// logs a warning.
	sink := &consumertest.TracesSink{}
	cfg := &Config{
		TraceTimeout:  50 * time.Millisecond,
		DecisionDelay: 50 * time.Millisecond,
		NumTraces:     10,
		Rules: []RuleConfig{
			{Name: "default", Sampler: SamplerConfig{Type: AlwaysSample}},
		},
	}
	core, recorded := observer.New(zap.WarnLevel)
	set := processortest.NewNopSettings(metadata.Type)
	set.Logger = zap.New(core)
	p, err := newProcessor(set, cfg, sink)
	require.NoError(t, err)
	require.NoError(t, p.Start(t.Context(), nil))
	t.Cleanup(func() { require.NoError(t, p.Shutdown(t.Context())) })

	traceID := pcommon.TraceID([16]byte{0xCD})
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()

	// Two arbitrary but distinct rv values.
	rvA, err := sampling.RValueToRandomness("aaaaaaaaaaaaaa")
	require.NoError(t, err)
	rvB, err := sampling.RValueToRandomness("bbbbbbbbbbbbbb")
	require.NoError(t, err)

	for i, rv := range []sampling.Randomness{rvA, rvB} {
		span := ss.Spans().AppendEmpty()
		span.SetTraceID(traceID)
		span.SetSpanID([8]byte{byte(i + 1)})
		span.SetParentSpanID([8]byte{9, 9, 9, 9, 9, 9, 9, 9})
		span.SetName("op")
		span.TraceState().FromRaw("ot=rv:" + rv.RValue())
	}

	require.NoError(t, p.ConsumeTraces(t.Context(), td))
	assert.Eventually(t, func() bool { return sink.SpanCount() == 2 }, time.Second, 10*time.Millisecond)

	warnings := recorded.FilterMessageSnippet("divergent ot=rv").All()
	require.Len(t, warnings, 1, "expected exactly one divergent-rv warning")
	assert.Equal(t, zap.WarnLevel, warnings[0].Level)
}

// TestProcessor_RootSpanCondition_CustomExpression verifies that operators
// can point the trigger at spans whose ParentSpanID is non-empty by providing
// an OTTL expression. The trace has no true root span, so the default
// IsRootSpan() would let it wait until trace_timeout; with a custom hint
// attribute the decision must fire before trace_timeout.
func TestProcessor_RootSpanCondition_CustomExpression(t *testing.T) {
	sink := &consumertest.TracesSink{}
	cfg := &Config{
		TraceTimeout:      time.Hour,
		DecisionDelay:     50 * time.Millisecond,
		NumTraces:         10,
		RootSpanCondition: `span.attributes["otelcol.dynamic_sampling.root_span"] == true`,
		Rules: []RuleConfig{
			{Name: "default", Sampler: SamplerConfig{Type: AlwaysSample}},
		},
	}
	p := newTestProcessor(t, cfg, sink)

	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()
	span := ss.Spans().AppendEmpty()
	span.SetTraceID(pcommon.TraceID([16]byte{0xDE, 0xAD, 0xBE, 0xEF}))
	span.SetSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})
	span.SetParentSpanID([8]byte{9, 9, 9, 9, 9, 9, 9, 9})
	span.SetName("consume")
	span.Attributes().PutBool("otelcol.dynamic_sampling.root_span", true)

	require.NoError(t, p.ConsumeTraces(t.Context(), td))

	assert.Eventually(t, func() bool {
		return sink.SpanCount() == 1
	}, time.Second, 10*time.Millisecond, "hint attribute should trigger decision before trace_timeout")
}

// TestProcessor_RootSpanCondition_NonMatchingHoldsForTimeout verifies that
// when the custom expression does not match any span, the trace waits for
// trace_timeout and is then decided normally. This guards against a broken
// override silently accepting every span.
func TestProcessor_RootSpanCondition_NonMatchingHoldsForTimeout(t *testing.T) {
	sink := &consumertest.TracesSink{}
	cfg := &Config{
		TraceTimeout:      50 * time.Millisecond,
		DecisionDelay:     50 * time.Millisecond,
		NumTraces:         10,
		RootSpanCondition: `span.attributes["never_present"] == true`,
		Rules: []RuleConfig{
			{Name: "default", Sampler: SamplerConfig{Type: AlwaysSample}},
		},
	}
	p := newTestProcessor(t, cfg, sink)

	traceID := pcommon.TraceID([16]byte{0x01, 0x02})
	// newRootTrace has an empty parent, but our condition ignores that fact.
	require.NoError(t, p.ConsumeTraces(t.Context(), newRootTrace(traceID)))

	assert.Eventually(t, func() bool {
		return sink.SpanCount() == 1
	}, time.Second, 10*time.Millisecond, "trace_timeout should fire since the custom condition never matches")
}

func TestProcessor_DecisionCacheDisabled(t *testing.T) {
	sink := &consumertest.TracesSink{}
	cfg := &Config{
		TraceTimeout:  50 * time.Millisecond,
		DecisionDelay: 50 * time.Millisecond,
		NumTraces:     10,
		DecisionCache: DecisionCacheConfig{SampledCacheSize: 0, NonSampledCacheSize: 0},
		Rules: []RuleConfig{
			{Name: "default", Sampler: SamplerConfig{Type: AlwaysSample}},
		},
	}
	p := newTestProcessor(t, cfg, sink)

	traceID := pcommon.TraceID([16]byte{0xFF})
	require.NoError(t, p.ConsumeTraces(t.Context(), newTrace(traceID, ptrace.StatusCodeUnset)))
	assert.Eventually(t, func() bool {
		return sink.SpanCount() == 1
	}, time.Second, 10*time.Millisecond)

	// Late span should fall through to the pending-trace path (no cache hit),
	// producing a fresh decision and a second forwarded span.
	require.NoError(t, p.ConsumeTraces(t.Context(), newTrace(traceID, ptrace.StatusCodeUnset)))
	assert.Eventually(t, func() bool {
		return sink.SpanCount() == 2
	}, time.Second, 10*time.Millisecond)
}

// TestSerializedEmptyTraceState_MatchesFullSerialize guards the fast path in
// updateTraceState: for a span with no incoming tracestate, the precomputed
// string must be byte-identical to what the parse/update/serialize round trip
// produces.
func TestSerializedEmptyTraceState_MatchesFullSerialize(t *testing.T) {
	for _, prob := range []float64{1.0, 0.5, 0.1, 0.01, 1.0 / 3.0} {
		th, err := sampling.ProbabilityToThreshold(prob)
		require.NoError(t, err)

		w3c, err := sampling.NewW3CTraceState("")
		require.NoError(t, err)
		require.NoError(t, w3c.OTelValue().UpdateTValueWithSampling(th))
		var sb strings.Builder
		require.NoError(t, w3c.Serialize(&sb))

		assert.Equal(t, sb.String(), serializedEmptyTraceState(th), "prob %v", prob)
	}
}

func TestProcessor_ShutdownDrainsPendingTraces(t *testing.T) {
	sink := &consumertest.TracesSink{}
	cfg := &Config{
		TraceTimeout:  time.Hour, // decisions can only come from the drain
		DecisionDelay: time.Hour,
		NumTraces:     10,
		DecisionCache: DecisionCacheConfig{SampledCacheSize: 10, NonSampledCacheSize: 10},
		Rules: []RuleConfig{
			{Name: "default", Sampler: SamplerConfig{Type: AlwaysSample}},
		},
	}
	p := newTestProcessor(t, cfg, sink)

	first := pcommon.TraceID([16]byte{0xE1})
	second := pcommon.TraceID([16]byte{0xE2})
	require.NoError(t, p.ConsumeTraces(t.Context(), newTrace(first, ptrace.StatusCodeUnset)))
	require.NoError(t, p.ConsumeTraces(t.Context(), newTrace(second, ptrace.StatusCodeUnset)))

	require.NoError(t, p.Shutdown(t.Context()))

	require.Equal(t, 2, sink.SpanCount(), "shutdown must decide and forward buffered traces, not drop them")
	out := sink.AllTraces()
	firstSpan := out[0].ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
	assert.Equal(t, first, firstSpan.TraceID(), "drain should decide in arrival order")
	rule, ok := firstSpan.Attributes().Get(ruleAttributeKey)
	require.True(t, ok)
	assert.Equal(t, "default", rule.AsString())
	assert.Contains(t, firstSpan.TraceState().AsRaw(), "th:", "drained-and-kept trace must carry ot=th")

	// Second Shutdown (the test-helper cleanup also triggers one) must be a
	// no-op: nothing new forwarded, no error.
	require.NoError(t, p.Shutdown(t.Context()))
	assert.Equal(t, 2, sink.SpanCount())
}

func TestProcessor_ShutdownDrain_DropsUnmatched(t *testing.T) {
	sink := &consumertest.TracesSink{}
	cfg := &Config{
		TraceTimeout:  time.Hour,
		DecisionDelay: time.Hour,
		NumTraces:     10,
		DecisionCache: DecisionCacheConfig{SampledCacheSize: 10, NonSampledCacheSize: 10},
		Rules: []RuleConfig{
			{Name: "never-matches", Conditions: []string{`span.attributes["nonexistent"] == "x"`}, Sampler: SamplerConfig{Type: AlwaysSample}},
		},
	}
	p := newTestProcessor(t, cfg, sink)

	require.NoError(t, p.ConsumeTraces(t.Context(), newTrace(pcommon.TraceID([16]byte{0xE3}), ptrace.StatusCodeUnset)))
	require.NoError(t, p.Shutdown(t.Context()))

	assert.Equal(t, 0, sink.SpanCount(), "unmatched drained trace is dropped, same as the timer path")
}

func TestProcessor_ShutdownDrain_Metrics(t *testing.T) {
	tt := componenttest.NewTelemetry()
	t.Cleanup(func() {
		require.NoError(t, tt.Shutdown(context.Background())) //nolint:usetesting // cleanup after ctx cancel
	})

	sink := &consumertest.TracesSink{}
	cfg := &Config{
		TraceTimeout:  time.Hour,
		DecisionDelay: time.Hour,
		NumTraces:     10,
		Rules: []RuleConfig{
			{Name: "default", Sampler: SamplerConfig{Type: AlwaysSample}},
		},
	}
	p, err := newProcessor(metadatatest.NewSettings(tt), cfg, sink)
	require.NoError(t, err)
	require.NoError(t, p.Start(t.Context(), nil))

	require.NoError(t, p.ConsumeTraces(t.Context(), newTrace(pcommon.TraceID([16]byte{0xE4}), ptrace.StatusCodeUnset)))
	require.NoError(t, p.ConsumeTraces(t.Context(), newTrace(pcommon.TraceID([16]byte{0xE5}), ptrace.StatusCodeUnset)))
	require.NoError(t, p.Shutdown(t.Context()))

	metadatatest.AssertEqualProcessorDynamicSamplingDecisionTriggers(t, tt,
		[]metricdata.DataPoint[int64]{{
			Value:      2,
			Attributes: attribute.NewSet(attribute.String("trigger", "shutdown")),
		}},
		metricdatatest.IgnoreTimestamp(),
		metricdatatest.IgnoreExemplars(),
	)
}

func TestProcessor_ShutdownDrain_TriggeredTraceNotDoubleCounted(t *testing.T) {
	tt := componenttest.NewTelemetry()
	t.Cleanup(func() {
		require.NoError(t, tt.Shutdown(context.Background())) //nolint:usetesting // cleanup after ctx cancel
	})

	sink := &consumertest.TracesSink{}
	cfg := &Config{
		TraceTimeout:  time.Hour,
		DecisionDelay: time.Hour, // triggered traces sit in the delay window until shutdown
		NumTraces:     10,
		Rules: []RuleConfig{
			{Name: "default", Sampler: SamplerConfig{Type: AlwaysSample}},
		},
	}
	p, err := newProcessor(metadatatest.NewSettings(tt), cfg, sink)
	require.NoError(t, err)
	require.NoError(t, p.Start(t.Context(), nil))

	// Root span triggers the decision (trigger=root_span) but the decision
	// itself waits on decision_delay, so the trace is still pending at
	// shutdown.
	require.NoError(t, p.ConsumeTraces(t.Context(), newRootTrace(pcommon.TraceID([16]byte{0xE6}))))
	require.NoError(t, p.Shutdown(t.Context()))

	require.Equal(t, 1, sink.SpanCount(), "triggered trace must still be drained")
	metadatatest.AssertEqualProcessorDynamicSamplingDecisionTriggers(t, tt,
		[]metricdata.DataPoint[int64]{{
			Value:      1,
			Attributes: attribute.NewSet(attribute.String("trigger", "root_span")),
		}},
		metricdatatest.IgnoreTimestamp(),
		metricdatatest.IgnoreExemplars(),
	)
}

func TestProcessor_Eviction_TriggeredTraceNotDoubleCounted(t *testing.T) {
	tt := componenttest.NewTelemetry()
	t.Cleanup(func() {
		require.NoError(t, tt.Shutdown(context.Background())) //nolint:usetesting // cleanup after ctx cancel
	})

	sink := &consumertest.TracesSink{}
	cfg := &Config{
		TraceTimeout:  time.Hour,
		DecisionDelay: time.Hour,
		NumTraces:     1,
		Rules: []RuleConfig{
			{Name: "default", Sampler: SamplerConfig{Type: AlwaysSample}},
		},
	}
	p, err := newProcessor(metadatatest.NewSettings(tt), cfg, sink)
	require.NoError(t, err)
	require.NoError(t, p.Start(t.Context(), nil))
	t.Cleanup(func() { require.NoError(t, p.Shutdown(t.Context())) })

	// First trace is a root trace: counted under trigger=root_span, then held
	// in its decision_delay window. The second trace overflows the buffer and
	// evicts it mid-delay; that decision must not be counted again.
	require.NoError(t, p.ConsumeTraces(t.Context(), newRootTrace(pcommon.TraceID([16]byte{0xE7}))))
	require.NoError(t, p.ConsumeTraces(t.Context(), newTrace(pcommon.TraceID([16]byte{0xE8}), ptrace.StatusCodeUnset)))

	require.Equal(t, 1, sink.SpanCount(), "evicted trace is still decided and forwarded")
	metadatatest.AssertEqualProcessorDynamicSamplingDecisionTriggers(t, tt,
		[]metricdata.DataPoint[int64]{{
			Value:      1,
			Attributes: attribute.NewSet(attribute.String("trigger", "root_span")),
		}},
		metricdatatest.IgnoreTimestamp(),
		metricdatatest.IgnoreExemplars(),
	)
	metadatatest.AssertEqualProcessorDynamicSamplingTracesEvicted(t, tt,
		[]metricdata.DataPoint[int64]{{Value: 1}},
		metricdatatest.IgnoreTimestamp(),
		metricdatatest.IgnoreExemplars(),
	)
}

func TestProcessor_RuleConditionRuntimeEvalError(t *testing.T) {
	tt := componenttest.NewTelemetry()
	t.Cleanup(func() {
		require.NoError(t, tt.Shutdown(context.Background())) //nolint:usetesting // cleanup after ctx cancel
	})

	sink := &consumertest.TracesSink{}
	cfg := &Config{
		TraceTimeout:  10 * time.Millisecond,
		DecisionDelay: time.Millisecond,
		NumTraces:     10,
		DecisionCache: DecisionCacheConfig{SampledCacheSize: 10, NonSampledCacheSize: 10},
		Rules: []RuleConfig{
			// Compiles, but errors at eval time: Double() cannot convert the
			// non-numeric string attribute. No catch-all, so an errored (and
			// therefore non-matching) condition means the trace is dropped.
			{Name: "bad-eval", Conditions: []string{`Double(span.attributes["s"]) > 0.5`}, Sampler: SamplerConfig{Type: AlwaysSample}},
		},
	}
	p, err := newProcessor(metadatatest.NewSettings(tt), cfg, sink)
	require.NoError(t, err)
	require.NoError(t, p.Start(t.Context(), nil))
	t.Cleanup(func() { require.NoError(t, p.Shutdown(t.Context())) })

	td := newTrace(pcommon.TraceID([16]byte{0xF1}), ptrace.StatusCodeUnset)
	td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("s", "not-a-number")
	require.NoError(t, p.ConsumeTraces(t.Context(), td))

	// The trace times out, the condition errors, the rule does not match, and
	// the trace is dropped rather than wedged.
	assert.Eventually(t, func() bool {
		p.mu.Lock()
		defer p.mu.Unlock()
		return len(p.traces) == 0
	}, time.Second, 10*time.Millisecond)
	assert.Equal(t, 0, sink.SpanCount(), "errored condition must be treated as non-match")
	metadatatest.AssertEqualProcessorDynamicSamplingOttlEvalErrors(t, tt,
		[]metricdata.DataPoint[int64]{{
			Value:      1,
			Attributes: attribute.NewSet(attribute.String("rule", "bad-eval")),
		}},
		metricdatatest.IgnoreTimestamp(),
		metricdatatest.IgnoreExemplars(),
	)
}

func TestProcessor_RootSpanConditionRuntimeEvalError(t *testing.T) {
	tt := componenttest.NewTelemetry()
	t.Cleanup(func() {
		require.NoError(t, tt.Shutdown(context.Background())) //nolint:usetesting // cleanup after ctx cancel
	})

	sink := &consumertest.TracesSink{}
	cfg := &Config{
		TraceTimeout:      10 * time.Millisecond,
		DecisionDelay:     time.Millisecond,
		NumTraces:         10,
		DecisionCache:     DecisionCacheConfig{SampledCacheSize: 10, NonSampledCacheSize: 10},
		RootSpanCondition: `Double(span.attributes["s"]) > 0.5`,
		Rules: []RuleConfig{
			{Name: "default", Sampler: SamplerConfig{Type: AlwaysSample}},
		},
	}
	p, err := newProcessor(metadatatest.NewSettings(tt), cfg, sink)
	require.NoError(t, err)
	require.NoError(t, p.Start(t.Context(), nil))
	t.Cleanup(func() { require.NoError(t, p.Shutdown(t.Context())) })

	td := newRootTrace(pcommon.TraceID([16]byte{0xF2}))
	td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("s", "not-a-number")
	require.NoError(t, p.ConsumeTraces(t.Context(), td))

	// The erroring condition is a non-match, so the trace is not root-span
	// triggered; it still gets decided by trace_timeout and forwarded.
	assert.Eventually(t, func() bool {
		return sink.SpanCount() == 1
	}, time.Second, 10*time.Millisecond)
	metadatatest.AssertEqualProcessorDynamicSamplingOttlEvalErrors(t, tt,
		[]metricdata.DataPoint[int64]{{
			Value:      1,
			Attributes: attribute.NewSet(attribute.String("rule", "_root_span_condition")),
		}},
		metricdatatest.IgnoreTimestamp(),
		metricdatatest.IgnoreExemplars(),
	)
	metadatatest.AssertEqualProcessorDynamicSamplingDecisionTriggers(t, tt,
		[]metricdata.DataPoint[int64]{{
			Value:      1,
			Attributes: attribute.NewSet(attribute.String("trigger", "trace_timeout")),
		}},
		metricdatatest.IgnoreTimestamp(),
		metricdatatest.IgnoreExemplars(),
	)
}

func TestProcessor_MultiResourceTraceInOneBatch(t *testing.T) {
	sink := &consumertest.TracesSink{}
	cfg := &Config{
		TraceTimeout:  time.Hour,
		DecisionDelay: time.Millisecond,
		NumTraces:     10,
		DecisionCache: DecisionCacheConfig{SampledCacheSize: 10, NonSampledCacheSize: 10},
		Rules: []RuleConfig{
			{Name: "default", Sampler: SamplerConfig{Type: AlwaysSample}},
		},
	}
	p := newTestProcessor(t, cfg, sink)

	id := pcommon.TraceID([16]byte{0xF3})
	td := ptrace.NewTraces()
	// Resource A: the root span.
	rsA := td.ResourceSpans().AppendEmpty()
	rsA.Resource().Attributes().PutStr("service.name", "frontend")
	spanA := rsA.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	spanA.SetTraceID(id)
	spanA.SetSpanID([8]byte{1})
	spanA.SetName("root")
	// Resource B: a child span of the same trace in the same batch.
	rsB := td.ResourceSpans().AppendEmpty()
	rsB.Resource().Attributes().PutStr("service.name", "backend")
	spanB := rsB.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	spanB.SetTraceID(id)
	spanB.SetSpanID([8]byte{2})
	spanB.SetParentSpanID([8]byte{1})
	spanB.SetName("child")

	require.NoError(t, p.ConsumeTraces(t.Context(), td))

	assert.Eventually(t, func() bool {
		return sink.SpanCount() == 2
	}, time.Second, 10*time.Millisecond)

	out := sink.AllTraces()[0]
	require.Equal(t, 2, out.ResourceSpans().Len(), "both resources must survive into the decided trace")
	services := map[string]bool{}
	for i := 0; i < out.ResourceSpans().Len(); i++ {
		rs := out.ResourceSpans().At(i)
		v, ok := rs.Resource().Attributes().Get("service.name")
		require.True(t, ok)
		services[v.AsString()] = true
		span := rs.ScopeSpans().At(0).Spans().At(0)
		rule, ok := span.Attributes().Get(ruleAttributeKey)
		require.True(t, ok, "every span must carry the rule attribute")
		assert.Equal(t, "default", rule.AsString())
		assert.Contains(t, span.TraceState().AsRaw(), "th:")
	}
	assert.True(t, services["frontend"] && services["backend"], "distinct resource attributes must be preserved")

	// Late batch for the same (sampled) trace, again split across two
	// resources: both parts must be stamped and forwarded via the cache path.
	late := ptrace.NewTraces()
	for i, svc := range []string{"frontend", "backend"} {
		rs := late.ResourceSpans().AppendEmpty()
		rs.Resource().Attributes().PutStr("service.name", svc)
		sp := rs.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
		sp.SetTraceID(id)
		sp.SetSpanID([8]byte{byte(10 + i)})
		sp.SetParentSpanID([8]byte{1})
		sp.SetName("late")
	}
	require.NoError(t, p.ConsumeTraces(t.Context(), late))
	assert.Eventually(t, func() bool {
		return sink.SpanCount() == 4
	}, time.Second, 10*time.Millisecond)
	// Late spans forward one Traces per (trace, resource) bucket rather than
	// one merged payload; what must hold is that every late span is stamped
	// and its resource preserved.
	lateServices := map[string]bool{}
	lateSpans := 0
	for _, out := range sink.AllTraces()[1:] {
		for i := 0; i < out.ResourceSpans().Len(); i++ {
			rs := out.ResourceSpans().At(i)
			v, ok := rs.Resource().Attributes().Get("service.name")
			require.True(t, ok)
			lateServices[v.AsString()] = true
			span := rs.ScopeSpans().At(0).Spans().At(0)
			assert.Contains(t, span.TraceState().AsRaw(), "th:", "late spans must be stamped with the original threshold")
			lateSpans++
		}
	}
	assert.Equal(t, 2, lateSpans)
	assert.True(t, lateServices["frontend"] && lateServices["backend"], "late spans must keep their own resource attributes")
}

func TestProcessor_ThroughputSamplersEndToEnd(t *testing.T) {
	for _, typ := range []SamplerType{EMAThroughput, WindowedThroughput} {
		t.Run(string(typ), func(t *testing.T) {
			sink := &consumertest.TracesSink{}
			cfg := &Config{
				TraceTimeout:  time.Hour,
				DecisionDelay: time.Millisecond,
				NumTraces:     10,
				DecisionCache: DecisionCacheConfig{SampledCacheSize: 10, NonSampledCacheSize: 10},
				Rules: []RuleConfig{
					{Name: "default", Sampler: SamplerConfig{
						Type:                 typ,
						GoalThroughputPerSec: 1000,
						KeyAttributes:        []string{"service.name"},
					}},
				},
			}
			p := newTestProcessor(t, cfg, sink)

			// All traces forward regardless of the sampler's initial rate
			// (the trace IDs carry maximal randomness), each with a valid
			// ot=th. This pins the end-to-end wiring (keying, GetSampleRate,
			// threshold composition) for both types.
			for i := 0; i < 3; i++ {
				// Max out the randomness bytes so the traces are kept at any
				// initial rate the sampler chooses; the test pins the wiring,
				// not the rate.
				id := [16]byte{0xF4, byte(i)}
				for j := 9; j < 16; j++ {
					id[j] = 0xFF
				}
				td := newRootTrace(pcommon.TraceID(id))
				td.ResourceSpans().At(0).Resource().Attributes().PutStr("service.name", "svc")
				require.NoError(t, p.ConsumeTraces(t.Context(), td))
			}
			assert.Eventually(t, func() bool {
				return sink.SpanCount() == 3
			}, time.Second, 10*time.Millisecond)
			span := sink.AllTraces()[0].ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
			assert.Contains(t, span.TraceState().AsRaw(), "ot=th:", "kept spans must carry a threshold")
		})
	}
}
