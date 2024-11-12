// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tailsamplingprocessor

import (
	"context"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/cache"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/sampling"
)

func TestSamplingPolicyTypicalPath(t *testing.T) {
	cfg := Config{
		DecisionWait: defaultTestDecisionWait,
		NumTraces:    defaultNumTraces,
	}
	nextConsumer := new(consumertest.TracesSink)
	tel := setupTestTelemetry()
	idb := newSyncIDBatcher()

	mpe1 := &mockPolicyEvaluator{}

	policies := []*policy{
		{name: "mock-policy-1", evaluator: mpe1, attribute: metric.WithAttributes(attribute.String("policy", "mock-policy-1"))},
	}

	p, err := newTracesProcessor(context.Background(), tel.NewSettings(), nextConsumer, cfg, withDecisionBatcher(idb), withPolicies(policies))
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
	require.EqualValues(t, 0, mpe1.EvaluationCount)

	// This will cause policy evaluations on the first span
	tsp.policyTicker.OnTick()

	// Both policies should have been evaluated once
	require.EqualValues(t, 1, mpe1.EvaluationCount)

	// The final decision SHOULD be Sampled.
	require.EqualValues(t, 1, nextConsumer.SpanCount())
	require.NoError(t, tel.Shutdown(context.Background()))
}

func TestSamplingPolicyInvertSampled(t *testing.T) {
	cfg := Config{
		DecisionWait: defaultTestDecisionWait,
		NumTraces:    defaultNumTraces,
	}
	nextConsumer := new(consumertest.TracesSink)
	tel := setupTestTelemetry()
	idb := newSyncIDBatcher()

	mpe1 := &mockPolicyEvaluator{}

	policies := []*policy{
		{name: "mock-policy-1", evaluator: mpe1, attribute: metric.WithAttributes(attribute.String("policy", "mock-policy-1"))},
	}

	p, err := newTracesProcessor(context.Background(), tel.NewSettings(), nextConsumer, cfg, withDecisionBatcher(idb), withPolicies(policies))
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
	require.EqualValues(t, 0, mpe1.EvaluationCount)

	// This will cause policy evaluations on the first span
	tsp.policyTicker.OnTick()

	// Both policies should have been evaluated once
	require.EqualValues(t, 1, mpe1.EvaluationCount)

	// The final decision SHOULD be Sampled.
	require.EqualValues(t, 1, nextConsumer.SpanCount())
	require.NoError(t, tel.Shutdown(context.Background()))
}

func TestSamplingMultiplePolicies(t *testing.T) {
	cfg := Config{
		DecisionWait: defaultTestDecisionWait,
		NumTraces:    defaultNumTraces,
	}
	nextConsumer := new(consumertest.TracesSink)
	tel := setupTestTelemetry()
	idb := newSyncIDBatcher()

	mpe1 := &mockPolicyEvaluator{}
	mpe2 := &mockPolicyEvaluator{}

	policies := []*policy{
		{name: "mock-policy-1", evaluator: mpe1, attribute: metric.WithAttributes(attribute.String("policy", "mock-policy-1"))},
		{name: "mock-policy-2", evaluator: mpe2, attribute: metric.WithAttributes(attribute.String("policy", "mock-policy-2"))},
	}

	p, err := newTracesProcessor(context.Background(), tel.NewSettings(), nextConsumer, cfg, withDecisionBatcher(idb), withPolicies(policies))
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
	require.EqualValues(t, 0, mpe1.EvaluationCount)
	require.EqualValues(t, 0, mpe2.EvaluationCount)

	// This will cause policy evaluations on the first span
	tsp.policyTicker.OnTick()

	// Both policies should have been evaluated once
	require.EqualValues(t, 1, mpe1.EvaluationCount)
	require.EqualValues(t, 1, mpe2.EvaluationCount)

	// The final decision SHOULD be Sampled.
	require.EqualValues(t, 1, nextConsumer.SpanCount())
	require.NoError(t, tel.Shutdown(context.Background()))
}

func TestSamplingMultiplePolicies_WithRecordPolicy(t *testing.T) {
	cfg := Config{
		DecisionWait: defaultTestDecisionWait,
		NumTraces:    defaultNumTraces,
	}
	nextConsumer := new(consumertest.TracesSink)
	s := setupTestTelemetry()
	ct := s.NewSettings()
	idb := newSyncIDBatcher()

	mpe1 := &mockPolicyEvaluator{}
	mpe2 := &mockPolicyEvaluator{}

	policies := []*policy{
		{name: "mock-policy-1", evaluator: mpe1, attribute: metric.WithAttributes(attribute.String("policy", "mock-policy-1"))},
		{name: "mock-policy-2", evaluator: mpe2, attribute: metric.WithAttributes(attribute.String("policy", "mock-policy-2"))},
	}

	p, err := newTracesProcessor(context.Background(), ct, nextConsumer, cfg, withDecisionBatcher(idb), withPolicies(policies), withRecordPolicy())
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
	require.EqualValues(t, 1, nextConsumer.SpanCount())

	// First span should have an attribute that records the policy that sampled it
	policy, ok := nextConsumer.AllTraces()[0].ResourceSpans().At(0).ScopeSpans().At(0).Scope().Attributes().Get("tailsampling.policy")
	if !ok {
		assert.FailNow(t, "Did not find expected attribute")
	}
	require.EqualValues(t, "mock-policy-1", policy.AsString())
}

func TestSamplingPolicyDecisionNotSampled(t *testing.T) {
	cfg := Config{
		DecisionWait: defaultTestDecisionWait,
		NumTraces:    defaultNumTraces,
	}
	nextConsumer := new(consumertest.TracesSink)
	tel := setupTestTelemetry()
	idb := newSyncIDBatcher()

	mpe1 := &mockPolicyEvaluator{}

	policies := []*policy{
		{name: "mock-policy-1", evaluator: mpe1, attribute: metric.WithAttributes(attribute.String("policy", "mock-policy-1"))},
	}

	p, err := newTracesProcessor(context.Background(), tel.NewSettings(), nextConsumer, cfg, withDecisionBatcher(idb), withPolicies(policies))
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
	require.EqualValues(t, 0, mpe1.EvaluationCount)

	// This will cause policy evaluations on the first span
	tsp.policyTicker.OnTick()

	// Both policies should have been evaluated once
	require.EqualValues(t, 1, mpe1.EvaluationCount)

	// The final decision SHOULD be NotSampled.
	require.EqualValues(t, 0, nextConsumer.SpanCount())
	require.NoError(t, tel.Shutdown(context.Background()))
}

func TestSamplingPolicyDecisionNotSampled_WithRecordPolicy(t *testing.T) {
	cfg := Config{
		DecisionWait: defaultTestDecisionWait,
		NumTraces:    defaultNumTraces,
	}
	nextConsumer := new(consumertest.TracesSink)
	s := setupTestTelemetry()
	ct := s.NewSettings()
	idb := newSyncIDBatcher()

	mpe1 := &mockPolicyEvaluator{}

	policies := []*policy{
		{name: "mock-policy-1", evaluator: mpe1, attribute: metric.WithAttributes(attribute.String("policy", "mock-policy-1"))},
	}

	p, err := newTracesProcessor(context.Background(), ct, nextConsumer, cfg, withDecisionBatcher(idb), withPolicies(policies), withRecordPolicy())
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
	require.EqualValues(t, 0, nextConsumer.SpanCount())
}

func TestSamplingPolicyDecisionInvertNotSampled(t *testing.T) {
	cfg := Config{
		DecisionWait: defaultTestDecisionWait,
		NumTraces:    defaultNumTraces,
	}
	nextConsumer := new(consumertest.TracesSink)
	tel := setupTestTelemetry()
	idb := newSyncIDBatcher()

	mpe1 := &mockPolicyEvaluator{}
	mpe2 := &mockPolicyEvaluator{}

	policies := []*policy{
		{name: "mock-policy-1", evaluator: mpe1, attribute: metric.WithAttributes(attribute.String("policy", "mock-policy-1"))},
		{name: "mock-policy-2", evaluator: mpe2, attribute: metric.WithAttributes(attribute.String("policy", "mock-policy-2"))},
	}

	p, err := newTracesProcessor(context.Background(), tel.NewSettings(), nextConsumer, cfg, withDecisionBatcher(idb), withPolicies(policies))
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
	require.EqualValues(t, 0, mpe1.EvaluationCount)
	require.EqualValues(t, 0, mpe2.EvaluationCount)

	// This will cause policy evaluations on the first span
	tsp.policyTicker.OnTick()

	// Both policies should have been evaluated once
	require.EqualValues(t, 1, mpe1.EvaluationCount)
	require.EqualValues(t, 1, mpe2.EvaluationCount)

	// The final decision SHOULD be NotSampled.
	require.EqualValues(t, 0, nextConsumer.SpanCount())
	require.NoError(t, tel.Shutdown(context.Background()))
}

func TestSamplingPolicyDecisionInvertNotSampled_WithRecordPolicy(t *testing.T) {
	cfg := Config{
		DecisionWait: defaultTestDecisionWait,
		NumTraces:    defaultNumTraces,
	}
	nextConsumer := new(consumertest.TracesSink)
	s := setupTestTelemetry()
	ct := s.NewSettings()
	idb := newSyncIDBatcher()

	mpe1 := &mockPolicyEvaluator{}
	mpe2 := &mockPolicyEvaluator{}

	policies := []*policy{
		{name: "mock-policy-1", evaluator: mpe1, attribute: metric.WithAttributes(attribute.String("policy", "mock-policy-1"))},
		{name: "mock-policy-2", evaluator: mpe2, attribute: metric.WithAttributes(attribute.String("policy", "mock-policy-2"))},
	}

	p, err := newTracesProcessor(context.Background(), ct, nextConsumer, cfg, withDecisionBatcher(idb), withPolicies(policies), withRecordPolicy())
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
	require.EqualValues(t, 0, nextConsumer.SpanCount())
}

func TestLateArrivingSpansAssignedOriginalDecision(t *testing.T) {
	cfg := Config{
		DecisionWait: defaultTestDecisionWait,
		NumTraces:    defaultNumTraces,
	}
	nextConsumer := new(consumertest.TracesSink)
	tel := setupTestTelemetry()
	idb := newSyncIDBatcher()

	mpe1 := &mockPolicyEvaluator{}
	mpe2 := &mockPolicyEvaluator{}

	policies := []*policy{
		{name: "mock-policy-1", evaluator: mpe1, attribute: metric.WithAttributes(attribute.String("policy", "mock-policy-1"))},
		{name: "mock-policy-2", evaluator: mpe2, attribute: metric.WithAttributes(attribute.String("policy", "mock-policy-2"))},
	}

	p, err := newTracesProcessor(context.Background(), tel.NewSettings(), nextConsumer, cfg, withDecisionBatcher(idb), withPolicies(policies))
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
	require.EqualValues(t, 0, mpe1.EvaluationCount)
	require.EqualValues(t, 0, mpe2.EvaluationCount)

	// This will cause policy evaluations on the first span
	tsp.policyTicker.OnTick()

	// Both policies should have been evaluated once
	require.EqualValues(t, 1, mpe1.EvaluationCount)
	require.EqualValues(t, 1, mpe2.EvaluationCount)

	// The final decision SHOULD be NotSampled.
	require.EqualValues(t, 0, nextConsumer.SpanCount())

	// Generate and deliver final span for the trace which SHOULD get the same sampling decision as the first span.
	// The policies should NOT be evaluated again.
	require.NoError(t, p.ConsumeTraces(context.Background(), spanIndexToTraces(2)))
	require.EqualValues(t, 1, mpe1.EvaluationCount)
	require.EqualValues(t, 1, mpe2.EvaluationCount)
	require.EqualValues(t, 0, nextConsumer.SpanCount(), "original final decision not honored")
	require.NoError(t, tel.Shutdown(context.Background()))
}

func TestLateArrivingSpanUsesDecisionCache(t *testing.T) {
	cfg := Config{
		DecisionWait: defaultTestDecisionWait * 10,
		NumTraces:    defaultNumTraces,
	}
	nextConsumer := new(consumertest.TracesSink)
	tel := setupTestTelemetry()
	idb := newSyncIDBatcher()

	mpe := &mockPolicyEvaluator{}
	policies := []*policy{
		{name: "mock-policy-1", evaluator: mpe, attribute: metric.WithAttributes(attribute.String("policy", "mock-policy-1"))},
	}

	// Use this instead of the default no-op cache
	c, err := cache.NewLRUDecisionCache[bool](200)
	require.NoError(t, err)
	p, err := newTracesProcessor(context.Background(), tel.NewSettings(), nextConsumer, cfg, withDecisionBatcher(idb), withPolicies(policies), withSampledDecisionCache(c))
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
	require.EqualValues(t, 0, mpe.EvaluationCount)

	// This will cause policy evaluations on the first span
	tsp.policyTicker.OnTick()

	// Policy should have been evaluated once
	require.EqualValues(t, 1, mpe.EvaluationCount)

	// The final decision SHOULD be Sampled.
	require.EqualValues(t, 1, nextConsumer.SpanCount())

	// Drop the trace to force cache to make decision
	tsp.dropTrace(traceID, time.Now())
	_, ok := tsp.idToTrace.Load(traceID)
	require.False(t, ok)

	// Set next decision to not sampled, ensuring the next decision is determined by the decision cache, not the policy
	mpe.NextDecision = sampling.NotSampled

	// Generate and deliver final span for the trace which SHOULD get the same sampling decision as the first span.
	// The policies should NOT be evaluated again.
	require.NoError(t, p.ConsumeTraces(context.Background(), spanIndexToTraces(2)))
	require.EqualValues(t, 1, mpe.EvaluationCount)
	require.EqualValues(t, 2, nextConsumer.SpanCount(), "original final decision not honored")
	require.NoError(t, tel.Shutdown(context.Background()))
}

func TestLateSpanUsesNonSampledDecisionCache(t *testing.T) {
	cfg := Config{
		DecisionWait: defaultTestDecisionWait * 10,
		NumTraces:    defaultNumTraces,
	}
	nextConsumer := new(consumertest.TracesSink)
	s := setupTestTelemetry()
	ct := s.NewSettings()
	idb := newSyncIDBatcher()

	mpe := &mockPolicyEvaluator{}
	policies := []*policy{
		{name: "mock-policy-1", evaluator: mpe, attribute: metric.WithAttributes(attribute.String("policy", "mock-policy-1"))},
	}

	// Use this instead of the default no-op cache
	c, err := cache.NewLRUDecisionCache[bool](200)
	require.NoError(t, err)
	p, err := newTracesProcessor(context.Background(), ct, nextConsumer, cfg, withDecisionBatcher(idb), withPolicies(policies), withNonSampledDecisionCache(c))
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
	require.EqualValues(t, 0, mpe.EvaluationCount)

	// This will cause policy evaluations on the first span
	tsp.policyTicker.OnTick()

	// Policy should have been evaluated once
	require.EqualValues(t, 1, mpe.EvaluationCount)

	// The final decision SHOULD be NOT Sampled.
	require.EqualValues(t, 0, nextConsumer.SpanCount())

	// Drop the trace to force cache to make decision
	tsp.dropTrace(traceID, time.Now())
	_, ok := tsp.idToTrace.Load(traceID)
	require.False(t, ok)

	// Set next decision to sampled, ensuring the next decision is determined by the decision cache, not the policy
	mpe.NextDecision = sampling.Sampled

	// Generate and deliver final span for the trace which SHOULD get the same sampling decision as the first span.
	// The policies should NOT be evaluated again.
	require.NoError(t, p.ConsumeTraces(context.Background(), spanIndexToTraces(2)))
	require.EqualValues(t, 1, mpe.EvaluationCount)
	require.EqualValues(t, 0, nextConsumer.SpanCount(), "original final decision not honored")
}
