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
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/dynamicsamplingprocessor/internal/metadata"
)

func newTestProcessor(t *testing.T, cfg *Config, sink *consumertest.TracesSink) *dynamicSamplingProcessor {
	t.Helper()
	p, err := newProcessor(processortest.NewNopSettings(metadata.Type), cfg, sink)
	require.NoError(t, err)
	require.NoError(t, p.Start(context.Background(), nil))
	t.Cleanup(func() {
		require.NoError(t, p.Shutdown(context.Background()))
	})
	return p
}

func newTrace(traceID pcommon.TraceID, statusCode ptrace.StatusCode, attrs map[string]string) ptrace.Traces {
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	for k, v := range attrs {
		if rest, ok := strings.CutPrefix(k, "resource."); ok {
			rs.Resource().Attributes().PutStr(rest, v)
		}
	}
	ss := rs.ScopeSpans().AppendEmpty()
	span := ss.Spans().AppendEmpty()
	span.SetTraceID(traceID)
	span.SetSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})
	span.SetName("op")
	span.Status().SetCode(statusCode)
	for k, v := range attrs {
		if !strings.HasPrefix(k, "resource.") {
			span.Attributes().PutStr(k, v)
		}
	}
	return td
}

func TestProcessor_AlwaysSample_ForwardsAllTraces(t *testing.T) {
	sink := &consumertest.TracesSink{}
	cfg := &Config{
		DecisionWait: 50 * time.Millisecond,
		NumTraces:    100,
		Rules: []RuleConfig{
			{Name: "keep-all", Sampler: SamplerConfig{Type: AlwaysSample}},
		},
	}
	p := newTestProcessor(t, cfg, sink)

	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4})
	require.NoError(t, p.ConsumeTraces(context.Background(), newTrace(traceID, ptrace.StatusCodeUnset, nil)))

	assert.Eventually(t, func() bool {
		return sink.SpanCount() == 1
	}, time.Second, 10*time.Millisecond)
}

func TestProcessor_FirstMatchRouting(t *testing.T) {
	sink := &consumertest.TracesSink{}
	cfg := &Config{
		DecisionWait: 50 * time.Millisecond,
		NumTraces:    100,
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
	require.NoError(t, p.ConsumeTraces(context.Background(), newTrace(errTrace, ptrace.StatusCodeError, nil)))
	require.NoError(t, p.ConsumeTraces(context.Background(), newTrace(okTrace, ptrace.StatusCodeOk, nil)))

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
		DecisionWait: 50 * time.Millisecond,
		NumTraces:    100,
		Rules: []RuleConfig{
			{Name: "default", Sampler: SamplerConfig{Type: AlwaysSample}},
		},
	}
	p := newTestProcessor(t, cfg, sink)

	traceID := pcommon.TraceID([16]byte{9})
	require.NoError(t, p.ConsumeTraces(context.Background(), newTrace(traceID, ptrace.StatusCodeUnset, nil)))

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
	cfg := &Config{
		DecisionWait: 50 * time.Millisecond,
		NumTraces:    100,
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

	const n = 500
	for i := range n {
		// Vary the last 7 bytes; W3C consistent sampling uses these as randomness.
		var id [16]byte
		id[9] = byte(i)
		id[10] = byte(i >> 8)
		id[11] = byte(i * 31)
		id[12] = byte(i * 7)
		id[13] = byte(i + 13)
		id[14] = byte(i * 17)
		id[15] = byte(i*11 + 5)
		require.NoError(t, p.ConsumeTraces(context.Background(), newTrace(pcommon.TraceID(id), ptrace.StatusCodeUnset, nil)))
	}

	// Wait for decisions to drain.
	assert.Eventually(t, func() bool {
		p.mu.Lock()
		defer p.mu.Unlock()
		return len(p.traces) == 0
	}, 2*time.Second, 20*time.Millisecond)

	// Expect roughly 10% sampled. Allow a wide tolerance for the small sample.
	count := sink.SpanCount()
	assert.Greater(t, count, 10, "expected some traces sampled, got %d", count)
	assert.Less(t, count, 150, "expected fewer than ~30%, got %d", count)
}

func TestProcessor_Eviction(t *testing.T) {
	sink := &consumertest.TracesSink{}
	cfg := &Config{
		DecisionWait: time.Hour, // long enough we never decide before eviction
		NumTraces:    2,
		Rules: []RuleConfig{
			{Name: "default", Sampler: SamplerConfig{Type: AlwaysSample}},
		},
	}
	p := newTestProcessor(t, cfg, sink)

	for i := range byte(5) {
		id := pcommon.TraceID([16]byte{i})
		require.NoError(t, p.ConsumeTraces(context.Background(), newTrace(id, ptrace.StatusCodeUnset, nil)))
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	assert.LessOrEqual(t, len(p.traces), 2, "buffer should be capped at NumTraces")
}
