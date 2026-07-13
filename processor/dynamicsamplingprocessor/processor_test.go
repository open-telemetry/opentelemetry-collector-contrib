// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dynamicsamplingprocessor

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor/processortest"
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
				{Name: "keep-errors", Conditions: []string{"status.code == 2"}},
			},
			wantWarn:  true,
			wantField: "default",
		},
		{
			name: "catch_all_last_no_warn",
			rules: []RuleConfig{
				{Name: "keep-errors", Conditions: []string{"status.code == 2"}},
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
				{Name: "errors", Conditions: []string{"status.code == 2"}},
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
				Conditions: []string{"status.code == 2"},
				Sampler:    SamplerConfig{Type: AlwaysSample},
			},
			{
				Name: "drop-rest",
				Sampler: SamplerConfig{
					Type:          Deterministic,
					Deterministic: DeterministicConfig{SamplingPercentage: 100},
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
					Type:          Deterministic,
					Deterministic: DeterministicConfig{SamplingPercentage: 10}, // 1-in-10
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
				Conditions: []string{"status.code == 99"},
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
					Type:          Deterministic,
					Deterministic: DeterministicConfig{SamplingPercentage: 10}, // rate 10 = keep 10% of population
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
					Type:          Deterministic,
					Deterministic: DeterministicConfig{SamplingPercentage: 50}, // rate 2 = 50% of population
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
