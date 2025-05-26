// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tailsamplingprocessor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor/processortest"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/cache"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/sampling"
)

func TestSamplingPolicyTypicalPath(t *testing.T) {
	nextConsumer := new(consumertest.TracesSink)
	idb := newSyncIDBatcher()

	mpe1 := &mockPolicyEvaluator{}

	policies := []*policy{
		{name: "mock-policy-1", evaluator: mpe1, attribute: metric.WithAttributes(attribute.String("policy", "mock-policy-1"))},
	}

	cfg := Config{
		DecisionWait: defaultTestDecisionWait,
		NumTraces:    defaultNumTraces,
		Options: []Option{
			withDecisionBatcher(idb),
			withPolicies(policies),
		},
	}
	p, err := newTracesProcessor(context.Background(), processortest.NewNopSettings(metadata.Type), nextConsumer, cfg)
	require.NoError(t, err)

	require.NoError(t, p.Start(context.Background(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(t, p.Shutdown(context.Background()))
	}()

	mpe1.NextDecision = sampling.Sampled

	// Generate and deliver first span
	require.NoError(t, p.ConsumeTraces(context.Background(), simpleTraces()))

	tsp := p.(*tailSamplingSpanProcessor)

	// The first tick won't do anything
	tsp.policyTicker.OnTick()
	require.Equal(t, 0, mpe1.EvaluationCount)

	// This will cause policy evaluations on the first span
	tsp.policyTicker.OnTick()

	// Both policies should have been evaluated once
	require.Equal(t, 1, mpe1.EvaluationCount)

	// The final decision SHOULD be Sampled.
	require.Equal(t, 1, nextConsumer.SpanCount())
}

func TestSamplingPolicyInvertSampled(t *testing.T) {
	nextConsumer := new(consumertest.TracesSink)
	idb := newSyncIDBatcher()

	mpe1 := &mockPolicyEvaluator{}

	policies := []*policy{
		{name: "mock-policy-1", evaluator: mpe1, attribute: metric.WithAttributes(attribute.String("policy", "mock-policy-1"))},
	}

	cfg := Config{
		DecisionWait: defaultTestDecisionWait,
		NumTraces:    defaultNumTraces,
		Options: []Option{
			withDecisionBatcher(idb),
			withPolicies(policies),
		},
	}
	p, err := newTracesProcessor(context.Background(), processortest.NewNopSettings(metadata.Type), nextConsumer, cfg)
	require.NoError(t, err)

	require.NoError(t, p.Start(context.Background(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(t, p.Shutdown(context.Background()))
	}()

	mpe1.NextDecision = sampling.InvertSampled

	// Generate and deliver first span
	require.NoError(t, p.ConsumeTraces(context.Background(), simpleTraces()))

	tsp := p.(*tailSamplingSpanProcessor)

	// The first tick won't do anything
	tsp.policyTicker.OnTick()
	require.Equal(t, 0, mpe1.EvaluationCount)

	// This will cause policy evaluations on the first span
	tsp.policyTicker.OnTick()

	// Both policies should have been evaluated once
	require.Equal(t, 1, mpe1.EvaluationCount)

	// The final decision SHOULD be Sampled.
	require.Equal(t, 1, nextConsumer.SpanCount())
}

func TestSamplingMultiplePolicies(t *testing.T) {
	nextConsumer := new(consumertest.TracesSink)
	idb := newSyncIDBatcher()

	mpe1 := &mockPolicyEvaluator{}
	mpe2 := &mockPolicyEvaluator{}

	policies := []*policy{
		{name: "mock-policy-1", evaluator: mpe1, attribute: metric.WithAttributes(attribute.String("policy", "mock-policy-1"))},
		{name: "mock-policy-2", evaluator: mpe2, attribute: metric.WithAttributes(attribute.String("policy", "mock-policy-2"))},
	}

	cfg := Config{
		DecisionWait: defaultTestDecisionWait,
		NumTraces:    defaultNumTraces,
		Options: []Option{
			withDecisionBatcher(idb),
			withPolicies(policies),
		},
	}
	p, err := newTracesProcessor(context.Background(), processortest.NewNopSettings(metadata.Type), nextConsumer, cfg)
	require.NoError(t, err)

	require.NoError(t, p.Start(context.Background(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(t, p.Shutdown(context.Background()))
	}()

	// InvertNotSampled takes precedence
	mpe1.NextDecision = sampling.Sampled
	mpe2.NextDecision = sampling.Sampled

	// Generate and deliver first span
	require.NoError(t, p.ConsumeTraces(context.Background(), simpleTraces()))

	tsp := p.(*tailSamplingSpanProcessor)

	// The first tick won't do anything
	tsp.policyTicker.OnTick()
	require.Equal(t, 0, mpe1.EvaluationCount)
	require.Equal(t, 0, mpe2.EvaluationCount)

	// This will cause policy evaluations on the first span
	tsp.policyTicker.OnTick()

	// Both policies should have been evaluated once
	require.Equal(t, 1, mpe1.EvaluationCount)
	require.Equal(t, 1, mpe2.EvaluationCount)

	// The final decision SHOULD be Sampled.
	require.Equal(t, 1, nextConsumer.SpanCount())
}

func TestSamplingMultiplePolicies_WithRecordPolicy(t *testing.T) {
	nextConsumer := new(consumertest.TracesSink)
	s := setupTestTelemetry()
	ct := s.newSettings()
	idb := newSyncIDBatcher()

	mpe1 := &mockPolicyEvaluator{}
	mpe2 := &mockPolicyEvaluator{}

	policies := []*policy{
		{name: "mock-policy-1", evaluator: mpe1, attribute: metric.WithAttributes(attribute.String("policy", "mock-policy-1"))},
		{name: "mock-policy-2", evaluator: mpe2, attribute: metric.WithAttributes(attribute.String("policy", "mock-policy-2"))},
	}

	cfg := Config{
		DecisionWait: defaultTestDecisionWait,
		NumTraces:    defaultNumTraces,
		Options:      []Option{withDecisionBatcher(idb), withPolicies(policies), withRecordPolicy()},
	}

	p, err := newTracesProcessor(context.Background(), ct, nextConsumer, cfg)
	require.NoError(t, err)

	require.NoError(t, p.Start(context.Background(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(t, p.Shutdown(context.Background()))
	}()

	// First policy takes precedence
	mpe1.NextDecision = sampling.Sampled
	mpe2.NextDecision = sampling.Sampled

	// Generate and deliver first span
	require.NoError(t, p.ConsumeTraces(context.Background(), simpleTraces()))

	tsp := p.(*tailSamplingSpanProcessor)

	// The first tick won't do anything
	tsp.policyTicker.OnTick()
	// This will cause policy evaluations on the first span
	tsp.policyTicker.OnTick()

	// The final decision SHOULD be Sampled.
	require.Equal(t, 1, nextConsumer.SpanCount())

	// First span should have an attribute that records the policy that sampled it
	policy, ok := nextConsumer.AllTraces()[0].ResourceSpans().At(0).ScopeSpans().At(0).Scope().Attributes().Get("tailsampling.policy")
	if !ok {
		assert.FailNow(t, "Did not find expected attribute")
	}
	require.Equal(t, "mock-policy-1", policy.AsString())
}

func TestSamplingPolicyDecisionNotSampled(t *testing.T) {
	nextConsumer := new(consumertest.TracesSink)
	idb := newSyncIDBatcher()

	mpe1 := &mockPolicyEvaluator{}

	policies := []*policy{
		{name: "mock-policy-1", evaluator: mpe1, attribute: metric.WithAttributes(attribute.String("policy", "mock-policy-1"))},
	}

	cfg := Config{
		DecisionWait: defaultTestDecisionWait,
		NumTraces:    defaultNumTraces,
		Options: []Option{
			withDecisionBatcher(idb),
			withPolicies(policies),
		},
	}
	p, err := newTracesProcessor(context.Background(), processortest.NewNopSettings(metadata.Type), nextConsumer, cfg)
	require.NoError(t, err)

	require.NoError(t, p.Start(context.Background(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(t, p.Shutdown(context.Background()))
	}()

	// InvertNotSampled takes precedence
	mpe1.NextDecision = sampling.NotSampled

	// Generate and deliver first span
	require.NoError(t, p.ConsumeTraces(context.Background(), simpleTraces()))

	tsp := p.(*tailSamplingSpanProcessor)

	// The first tick won't do anything
	tsp.policyTicker.OnTick()
	require.Equal(t, 0, mpe1.EvaluationCount)

	// This will cause policy evaluations on the first span
	tsp.policyTicker.OnTick()

	// Both policies should have been evaluated once
	require.Equal(t, 1, mpe1.EvaluationCount)

	// The final decision SHOULD be NotSampled.
	require.Equal(t, 0, nextConsumer.SpanCount())
}

func TestSamplingPolicyDecisionNotSampled_WithRecordPolicy(t *testing.T) {
	nextConsumer := new(consumertest.TracesSink)
	s := setupTestTelemetry()
	ct := s.newSettings()
	idb := newSyncIDBatcher()

	mpe1 := &mockPolicyEvaluator{}

	policies := []*policy{
		{name: "mock-policy-1", evaluator: mpe1, attribute: metric.WithAttributes(attribute.String("policy", "mock-policy-1"))},
	}

	cfg := Config{
		DecisionWait: defaultTestDecisionWait,
		NumTraces:    defaultNumTraces,
		Options:      []Option{withDecisionBatcher(idb), withPolicies(policies), withRecordPolicy()},
	}

	p, err := newTracesProcessor(context.Background(), ct, nextConsumer, cfg)
	require.NoError(t, err)

	require.NoError(t, p.Start(context.Background(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(t, p.Shutdown(context.Background()))
	}()

	// InvertNotSampled takes precedence
	mpe1.NextDecision = sampling.NotSampled

	// Generate and deliver first span
	require.NoError(t, p.ConsumeTraces(context.Background(), simpleTraces()))

	tsp := p.(*tailSamplingSpanProcessor)

	// The first tick won't do anything
	tsp.policyTicker.OnTick()
	// This will cause policy evaluations on the first span
	tsp.policyTicker.OnTick()

	// The final decision SHOULD be NotSampled.
	require.Equal(t, 0, nextConsumer.SpanCount())
}

func TestSamplingPolicyDecisionInvertNotSampled(t *testing.T) {
	nextConsumer := new(consumertest.TracesSink)
	idb := newSyncIDBatcher()

	mpe1 := &mockPolicyEvaluator{}
	mpe2 := &mockPolicyEvaluator{}

	policies := []*policy{
		{name: "mock-policy-1", evaluator: mpe1, attribute: metric.WithAttributes(attribute.String("policy", "mock-policy-1"))},
		{name: "mock-policy-2", evaluator: mpe2, attribute: metric.WithAttributes(attribute.String("policy", "mock-policy-2"))},
	}

	cfg := Config{
		DecisionWait: defaultTestDecisionWait,
		NumTraces:    defaultNumTraces,
		Options: []Option{
			withDecisionBatcher(idb),
			withPolicies(policies),
		},
	}
	p, err := newTracesProcessor(context.Background(), processortest.NewNopSettings(metadata.Type), nextConsumer, cfg)
	require.NoError(t, err)

	require.NoError(t, p.Start(context.Background(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(t, p.Shutdown(context.Background()))
	}()

	// InvertNotSampled takes precedence
	mpe1.NextDecision = sampling.InvertNotSampled
	mpe2.NextDecision = sampling.Sampled

	// Generate and deliver first span
	require.NoError(t, p.ConsumeTraces(context.Background(), simpleTraces()))

	tsp := p.(*tailSamplingSpanProcessor)

	// The first tick won't do anything
	tsp.policyTicker.OnTick()
	require.Equal(t, 0, mpe1.EvaluationCount)
	require.Equal(t, 0, mpe2.EvaluationCount)

	// This will cause policy evaluations on the first span
	tsp.policyTicker.OnTick()

	// Both policies should have been evaluated once
	require.Equal(t, 1, mpe1.EvaluationCount)
	require.Equal(t, 1, mpe2.EvaluationCount)

	// The final decision SHOULD be NotSampled.
	require.Equal(t, 0, nextConsumer.SpanCount())
}

func TestSamplingPolicyDecisionInvertNotSampled_WithRecordPolicy(t *testing.T) {
	nextConsumer := new(consumertest.TracesSink)
	s := setupTestTelemetry()
	ct := s.newSettings()
	idb := newSyncIDBatcher()

	mpe1 := &mockPolicyEvaluator{}
	mpe2 := &mockPolicyEvaluator{}

	policies := []*policy{
		{name: "mock-policy-1", evaluator: mpe1, attribute: metric.WithAttributes(attribute.String("policy", "mock-policy-1"))},
		{name: "mock-policy-2", evaluator: mpe2, attribute: metric.WithAttributes(attribute.String("policy", "mock-policy-2"))},
	}

	cfg := Config{
		DecisionWait: defaultTestDecisionWait,
		NumTraces:    defaultNumTraces,
		Options:      []Option{withDecisionBatcher(idb), withPolicies(policies), withRecordPolicy()},
	}

	p, err := newTracesProcessor(context.Background(), ct, nextConsumer, cfg)
	require.NoError(t, err)

	require.NoError(t, p.Start(context.Background(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(t, p.Shutdown(context.Background()))
	}()

	// InvertNotSampled takes precedence
	mpe1.NextDecision = sampling.InvertNotSampled
	mpe2.NextDecision = sampling.Sampled

	// Generate and deliver first span
	require.NoError(t, p.ConsumeTraces(context.Background(), simpleTraces()))

	tsp := p.(*tailSamplingSpanProcessor)

	// The first tick won't do anything
	tsp.policyTicker.OnTick()
	// This will cause policy evaluations on the first span
	tsp.policyTicker.OnTick()

	// The final decision SHOULD be NotSampled.
	require.Equal(t, 0, nextConsumer.SpanCount())
}

func TestLateArrivingSpansAssignedOriginalDecision(t *testing.T) {
	nextConsumer := new(consumertest.TracesSink)
	idb := newSyncIDBatcher()

	mpe1 := &mockPolicyEvaluator{}
	mpe2 := &mockPolicyEvaluator{}

	policies := []*policy{
		{name: "mock-policy-1", evaluator: mpe1, attribute: metric.WithAttributes(attribute.String("policy", "mock-policy-1"))},
		{name: "mock-policy-2", evaluator: mpe2, attribute: metric.WithAttributes(attribute.String("policy", "mock-policy-2"))},
	}

	cfg := Config{
		DecisionWait: defaultTestDecisionWait,
		NumTraces:    defaultNumTraces,
		Options: []Option{
			withDecisionBatcher(idb),
			withPolicies(policies),
		},
	}
	p, err := newTracesProcessor(context.Background(), processortest.NewNopSettings(metadata.Type), nextConsumer, cfg)
	require.NoError(t, err)

	require.NoError(t, p.Start(context.Background(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(t, p.Shutdown(context.Background()))
	}()

	// We are going to create 2 spans belonging to the same trace
	traceID := uInt64ToTraceID(1)

	// The combined decision from the policies is NotSampled
	mpe1.NextDecision = sampling.InvertSampled
	mpe2.NextDecision = sampling.NotSampled

	// A function that return a ptrace.Traces containing a single span for the single trace we are using.
	spanIndexToTraces := func(spanIndex uint64) ptrace.Traces {
		traces := ptrace.NewTraces()
		span := traces.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
		span.SetTraceID(traceID)
		span.SetSpanID(uInt64ToSpanID(spanIndex))
		return traces
	}

	// Generate and deliver first span
	require.NoError(t, p.ConsumeTraces(context.Background(), spanIndexToTraces(1)))

	tsp := p.(*tailSamplingSpanProcessor)

	// The first tick won't do anything
	tsp.policyTicker.OnTick()
	require.Equal(t, 0, mpe1.EvaluationCount)
	require.Equal(t, 0, mpe2.EvaluationCount)

	// This will cause policy evaluations on the first span
	tsp.policyTicker.OnTick()

	// Both policies should have been evaluated once
	require.Equal(t, 1, mpe1.EvaluationCount)
	require.Equal(t, 1, mpe2.EvaluationCount)

	// The final decision SHOULD be NotSampled.
	require.Equal(t, 0, nextConsumer.SpanCount())

	// Generate and deliver final span for the trace which SHOULD get the same sampling decision as the first span.
	// The policies should NOT be evaluated again.
	require.NoError(t, p.ConsumeTraces(context.Background(), spanIndexToTraces(2)))
	require.Equal(t, 1, mpe1.EvaluationCount)
	require.Equal(t, 1, mpe2.EvaluationCount)
	require.Equal(t, 0, nextConsumer.SpanCount(), "original final decision not honored")
}

func TestLateArrivingSpanUsesDecisionCache(t *testing.T) {
	nextConsumer := new(consumertest.TracesSink)
	idb := newSyncIDBatcher()

	mpe := &mockPolicyEvaluator{}
	policies := []*policy{
		{name: "mock-policy-1", evaluator: mpe, attribute: metric.WithAttributes(attribute.String("policy", "mock-policy-1"))},
	}

	// Use this instead of the default no-op cache
	c, err := cache.NewLRUDecisionCache[bool](200)
	require.NoError(t, err)

	cfg := Config{
		DecisionWait: defaultTestDecisionWait * 10,
		NumTraces:    defaultNumTraces,
		Options: []Option{
			withDecisionBatcher(idb),
			withPolicies(policies),
			WithSampledDecisionCache(c),
		},
	}
	p, err := newTracesProcessor(context.Background(), processortest.NewNopSettings(metadata.Type), nextConsumer, cfg)
	require.NoError(t, err)

	require.NoError(t, p.Start(context.Background(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(t, p.Shutdown(context.Background()))
	}()

	// We are going to create 2 spans belonging to the same trace
	traceID := uInt64ToTraceID(1)

	// The first span will be sampled, this will later be set to not sampled, but the sampling decision will be cached
	mpe.NextDecision = sampling.Sampled

	// A function that return a ptrace.Traces containing a single span for the single trace we are using.
	spanIndexToTraces := func(spanIndex uint64) ptrace.Traces {
		traces := ptrace.NewTraces()
		span := traces.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
		span.SetTraceID(traceID)
		span.SetSpanID(uInt64ToSpanID(spanIndex))
		return traces
	}

	// Generate and deliver first span
	require.NoError(t, p.ConsumeTraces(context.Background(), spanIndexToTraces(1)))

	tsp := p.(*tailSamplingSpanProcessor)

	// The first tick won't do anything
	tsp.policyTicker.OnTick()
	require.Equal(t, 0, mpe.EvaluationCount)

	// This will cause policy evaluations on the first span
	tsp.policyTicker.OnTick()

	// Policy should have been evaluated once
	require.Equal(t, 1, mpe.EvaluationCount)

	// The final decision SHOULD be Sampled.
	require.Equal(t, 1, nextConsumer.SpanCount())

	// The trace should have been dropped after its id was added to the decision cache
	_, ok := tsp.idToTrace.Load(traceID)
	require.False(t, ok)

	// Set next decision to not sampled, ensuring the next decision is determined by the decision cache, not the policy
	mpe.NextDecision = sampling.NotSampled

	// Generate and deliver final span for the trace which SHOULD get the same sampling decision as the first span.
	// The policies should NOT be evaluated again.
	require.NoError(t, p.ConsumeTraces(context.Background(), spanIndexToTraces(2)))
	require.Equal(t, 1, mpe.EvaluationCount)
	require.Equal(t, 2, nextConsumer.SpanCount(), "original final decision not honored")
}

func TestLateSpanUsesNonSampledDecisionCache(t *testing.T) {
	nextConsumer := new(consumertest.TracesSink)
	idb := newSyncIDBatcher()

	mpe := &mockPolicyEvaluator{}
	policies := []*policy{
		{name: "mock-policy-1", evaluator: mpe, attribute: metric.WithAttributes(attribute.String("policy", "mock-policy-1"))},
	}

	// Use this instead of the default no-op cache
	c, err := cache.NewLRUDecisionCache[bool](200)
	require.NoError(t, err)

	cfg := Config{
		DecisionWait: defaultTestDecisionWait * 10,
		NumTraces:    defaultNumTraces,
		Options: []Option{
			withDecisionBatcher(idb),
			withPolicies(policies),
			WithNonSampledDecisionCache(c),
		},
	}
	p, err := newTracesProcessor(context.Background(), processortest.NewNopSettings(metadata.Type), nextConsumer, cfg)
	require.NoError(t, err)

	require.NoError(t, p.Start(context.Background(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(t, p.Shutdown(context.Background()))
	}()

	// We are going to create 2 spans belonging to the same trace
	traceID := uInt64ToTraceID(1)

	// The first span will be NOT sampled, this will later be set to sampled, but the sampling decision will be cached
	mpe.NextDecision = sampling.NotSampled

	// A function that return a ptrace.Traces containing a single span for the single trace we are using.
	spanIndexToTraces := func(spanIndex uint64) ptrace.Traces {
		traces := ptrace.NewTraces()
		span := traces.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
		span.SetTraceID(traceID)
		span.SetSpanID(uInt64ToSpanID(spanIndex))
		return traces
	}

	// Generate and deliver first span
	require.NoError(t, p.ConsumeTraces(context.Background(), spanIndexToTraces(1)))

	tsp := p.(*tailSamplingSpanProcessor)

	// The first tick won't do anything
	tsp.policyTicker.OnTick()
	require.Equal(t, 0, mpe.EvaluationCount)

	// This will cause policy evaluations on the first span
	tsp.policyTicker.OnTick()

	// Policy should have been evaluated once
	require.Equal(t, 1, mpe.EvaluationCount)

	// The final decision SHOULD be NOT Sampled.
	require.Equal(t, 0, nextConsumer.SpanCount())

	// The trace should have been dropped after its id was added to the decision cache
	_, ok := tsp.idToTrace.Load(traceID)
	require.False(t, ok)

	// Set next decision to sampled, ensuring the next decision is determined by the decision cache, not the policy
	mpe.NextDecision = sampling.Sampled

	// Generate and deliver final span for the trace which SHOULD get the same sampling decision as the first span.
	// The policies should NOT be evaluated again.
	require.NoError(t, p.ConsumeTraces(context.Background(), spanIndexToTraces(2)))
	require.Equal(t, 1, mpe.EvaluationCount)
	require.Equal(t, 0, nextConsumer.SpanCount(), "original final decision not honored")
}

func TestSampleOnFirstMatch(t *testing.T) {
	nextConsumer := new(consumertest.TracesSink)
	idb := newSyncIDBatcher()

	mpe1 := &mockPolicyEvaluator{}
	mpe2 := &mockPolicyEvaluator{}
	mpe3 := &mockPolicyEvaluator{}

	policies := []*policy{
		{name: "mock-policy-1", evaluator: mpe1, attribute: metric.WithAttributes(attribute.String("policy", "mock-policy-1"))},
		{name: "mock-policy-2", evaluator: mpe2, attribute: metric.WithAttributes(attribute.String("policy", "mock-policy-2"))},
		{name: "mock-policy-3", evaluator: mpe2, attribute: metric.WithAttributes(attribute.String("policy", "mock-policy-3"))},
	}

	cfg := Config{
		DecisionWait:       defaultTestDecisionWait,
		NumTraces:          defaultNumTraces,
		SampleOnFirstMatch: true,
		Options: []Option{
			withDecisionBatcher(idb),
			withPolicies(policies),
		},
	}
	p, err := newTracesProcessor(context.Background(), processortest.NewNopSettings(metadata.Type), nextConsumer, cfg)
	require.NoError(t, err)

	require.NoError(t, p.Start(context.Background(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(t, p.Shutdown(context.Background()))
	}()

	// Second policy matches, last policy should not be evaluated
	mpe1.NextDecision = sampling.NotSampled
	mpe2.NextDecision = sampling.Sampled

	// Generate and deliver first span
	require.NoError(t, p.ConsumeTraces(context.Background(), simpleTraces()))

	tsp := p.(*tailSamplingSpanProcessor)

	// The first tick won't do anything
	tsp.policyTicker.OnTick()
	require.Equal(t, 0, mpe1.EvaluationCount)
	require.Equal(t, 0, mpe2.EvaluationCount)
	require.Equal(t, 0, mpe3.EvaluationCount)

	// This will cause policy evaluations on the first span
	tsp.policyTicker.OnTick()

	// Only the first policy should have been evaluated
	require.Equal(t, 1, mpe1.EvaluationCount)
	require.Equal(t, 1, mpe2.EvaluationCount)
	require.Equal(t, 0, mpe3.EvaluationCount)

	// The final decision SHOULD be Sampled.
	require.Equal(t, 1, nextConsumer.SpanCount())
}
