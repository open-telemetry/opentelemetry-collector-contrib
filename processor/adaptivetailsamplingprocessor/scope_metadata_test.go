// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package adaptivetailsamplingprocessor

import (
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling"
)

func TestProcessor_PreservesScopeMetadata(t *testing.T) {
	for _, difference := range []string{"attributes", "schema_url", "both", "dropped_attributes_count", "identical"} {
		t.Run(difference, func(t *testing.T) {
			sink := &consumertest.TracesSink{}
			p := newTestProcessor(t, &Config{
				TraceTimeout: time.Hour, DecisionDelay: time.Millisecond, NumTraces: 10,
				DecisionCache: DecisionCacheConfig{SampledCacheSize: 10, NonSampledCacheSize: 10},
				Rules:         []RuleConfig{{Name: "keep-all", Sampler: SamplerConfig{Type: AlwaysSample}}},
			}, sink)

			// Accumulate two batches before triggering either trace's decision.
			expected := ptrace.NewTraces()
			for batch := byte(1); batch <= 2; batch++ {
				input := scopeMetadataBatch(batch, difference)
				if batch == 2 {
					spans := input.ResourceSpans().At(0).ScopeSpans().At(1).Spans()
					spans.At(0).SetParentSpanID(pcommon.SpanID{})
					spans.At(1).SetParentSpanID(pcommon.SpanID{})
				}
				for _, rs := range input.ResourceSpans().All() {
					rs.CopyTo(expected.ResourceSpans().AppendEmpty())
				}
				require.NoError(t, p.ConsumeTraces(t.Context(), input))
			}
			require.Eventually(t, func() bool { return sink.SpanCount() == expected.SpanCount() }, time.Second, time.Millisecond)
			t.Run("pending", func(t *testing.T) {
				assertScopeMetadataPreserved(t, expected, sink.AllTraces())
			})

			// Both decisions have now been cached through the real decision path.
			previousOutputs := len(sink.AllTraces())
			previousSpans := sink.SpanCount()
			expected = ptrace.NewTraces()
			for batch := byte(3); batch <= 4; batch++ {
				input := scopeMetadataBatch(batch, difference)
				for _, rs := range input.ResourceSpans().All() {
					rs.CopyTo(expected.ResourceSpans().AppendEmpty())
				}
				require.NoError(t, p.ConsumeTraces(t.Context(), input))
			}
			require.Equal(t, previousSpans+expected.SpanCount(), sink.SpanCount(), "cached spans must be forwarded synchronously")
			t.Run("sampled_cache", func(t *testing.T) {
				assertScopeMetadataPreserved(t, expected, sink.AllTraces()[previousOutputs:])
				for _, output := range sink.AllTraces()[previousOutputs:] {
					for _, rs := range output.ResourceSpans().All() {
						for _, ss := range rs.ScopeSpans().All() {
							for _, span := range ss.Spans().All() {
								trigger, ok := span.Attributes().Get(triggerAttributeKey)
								require.True(t, ok)
								assert.Equal(t, string(triggerRootSpan), trigger.Str())
							}
						}
					}
				}
			})
			p.mu.Lock()
			assert.Empty(t, p.traces, "late spans must not reopen pending traces")
			p.mu.Unlock()
		})
	}
}

// Each resource has an empty scope followed by two scopes with the same name
// and version. Trace IDs are interleaved within each populated scope.
func scopeMetadataBatch(batch byte, difference string) ptrace.Traces {
	td := ptrace.NewTraces()
	for resource := byte(1); resource <= 2; resource++ {
		rs := td.ResourceSpans().AppendEmpty()
		rs.Resource().Attributes().PutInt("resource", int64(resource))
		rs.Resource().SetDroppedAttributesCount(uint32(resource))
		rs.SetSchemaUrl("resource-schema-" + strconv.Itoa(int(resource)))
		rs.ScopeSpans().AppendEmpty()
		for scope := byte(1); scope <= 2; scope++ {
			ss := rs.ScopeSpans().AppendEmpty()
			ss.Scope().SetName("lib")
			ss.Scope().SetVersion("1")
			ss.Scope().Attributes().PutStr("variant", "a")
			ss.Scope().SetDroppedAttributesCount(3)
			ss.SetSchemaUrl("schema-a")
			if scope == 2 {
				if difference == "attributes" || difference == "both" {
					ss.Scope().Attributes().PutStr("variant", "b")
				}
				if difference == "schema_url" || difference == "both" {
					ss.SetSchemaUrl("schema-b")
				}
				if difference == "dropped_attributes_count" {
					ss.Scope().SetDroppedAttributesCount(7)
				}
			}
			for i, trace := range []byte{1, 2, 1} {
				span := ss.Spans().AppendEmpty()
				span.SetTraceID(pcommon.TraceID{trace})
				span.SetSpanID(pcommon.SpanID{batch, resource, scope, byte(i + 1)})
				span.SetParentSpanID(pcommon.SpanID{255})
				span.SetName("operation")
			}
		}
	}
	return td
}

// Compare by span identity, independently of trace map iteration and payload
// boundaries. Each (original scope group, trace ID) must produce one group.
func assertScopeMetadataPreserved(t *testing.T, expected ptrace.Traces, outputs []ptrace.Traces) {
	t.Helper()
	type source struct {
		rs    ptrace.ResourceSpans
		ss    ptrace.ScopeSpans
		span  ptrace.Span
		group int
	}
	type groupKey struct {
		group int
		trace pcommon.TraceID
	}
	want := make(map[pcommon.SpanID]source)
	group := 0
	for _, rs := range expected.ResourceSpans().All() {
		for _, ss := range rs.ScopeSpans().All() {
			for _, span := range ss.Spans().All() {
				want[span.SpanID()] = source{rs: rs, ss: ss, span: span, group: group}
			}
			group++
		}
	}
	seenGroups := make(map[groupKey]bool)
	for _, output := range outputs {
		for _, rs := range output.ResourceSpans().All() {
			for _, ss := range rs.ScopeSpans().All() {
				require.Positive(t, ss.Spans().Len(), "empty source scopes should not produce output groups")
				first, ok := want[ss.Spans().At(0).SpanID()]
				require.True(t, ok, "unexpected or duplicate span")
				key := groupKey{group: first.group, trace: first.span.TraceID()}
				require.False(t, seenGroups[key], "one source group must not be fragmented for the same trace")
				seenGroups[key] = true
				for _, span := range ss.Spans().All() {
					src, found := want[span.SpanID()]
					require.True(t, found, "unexpected or duplicate span")
					expectedGroup := groupKey{group: src.group, trace: src.span.TraceID()}
					assert.Equal(t, expectedGroup, key, "distinct source groups must not merge")
					assert.Equal(t, src.span.TraceID(), span.TraceID())
					assert.Equal(t, src.span.ParentSpanID(), span.ParentSpanID())
					assert.Equal(t, src.span.Name(), span.Name())
					assert.Equal(t, src.rs.Resource().Attributes().AsRaw(), rs.Resource().Attributes().AsRaw())
					assert.Equal(t, src.rs.Resource().DroppedAttributesCount(), rs.Resource().DroppedAttributesCount())
					assert.Equal(t, src.rs.SchemaUrl(), rs.SchemaUrl())
					assert.Equal(t, src.ss.Scope().Name(), ss.Scope().Name())
					assert.Equal(t, src.ss.Scope().Version(), ss.Scope().Version())
					assert.Equal(t, src.ss.Scope().Attributes().AsRaw(), ss.Scope().Attributes().AsRaw())
					assert.Equal(t, src.ss.Scope().DroppedAttributesCount(), ss.Scope().DroppedAttributesCount())
					assert.Equal(t, src.ss.SchemaUrl(), ss.SchemaUrl())
					rule, hasRule := span.Attributes().Get(ruleAttributeKey)
					require.True(t, hasRule)
					assert.Equal(t, "keep-all", rule.Str())
					assert.Contains(t, span.TraceState().AsRaw(), "ot=th:")
					delete(want, span.SpanID())
				}
			}
		}
	}
	assert.Empty(t, want, "all expected spans must be forwarded")
}

func TestProcessor_PreservesScopeMetadataWithMixedDecisions(t *testing.T) {
	sink := &consumertest.TracesSink{}
	p := newTestProcessor(t, &Config{
		TraceTimeout: time.Hour, DecisionDelay: time.Millisecond, NumTraces: 10,
		DecisionCache: DecisionCacheConfig{SampledCacheSize: 10, NonSampledCacheSize: 10},
		Rules:         []RuleConfig{{Name: "keep-all", Sampler: SamplerConfig{Type: AlwaysSample}}},
	}, sink)
	p.cache.recordSampled(pcommon.TraceID{2}, cachedDecision{ruleName: "keep-all", threshold: sampling.AlwaysSampleThreshold})
	p.cache.recordNotSampled(pcommon.TraceID{3})
	input := scopeMetadataBatch(1, "both")
	input.ResourceSpans().At(0).ScopeSpans().At(1).Spans().At(0).SetParentSpanID(pcommon.SpanID{})
	expected := ptrace.NewTraces()
	input.CopyTo(expected)
	for _, rs := range input.ResourceSpans().All() {
		for _, ss := range rs.ScopeSpans().All() {
			span := ss.Spans().AppendEmpty()
			span.SetTraceID(pcommon.TraceID{3})
			span.SetSpanID(pcommon.SpanID{255})
		}
	}
	require.NoError(t, p.ConsumeTraces(t.Context(), input))
	require.Eventually(t, func() bool { return sink.SpanCount() == expected.SpanCount() }, time.Second, time.Millisecond)
	assertScopeMetadataPreserved(t, expected, sink.AllTraces())
	p.mu.Lock()
	assert.Empty(t, p.traces)
	p.mu.Unlock()
}
