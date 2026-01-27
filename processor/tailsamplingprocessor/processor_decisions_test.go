// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tailsamplingprocessor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processortest"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/cache"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/pkg/samplingpolicy"
)

func TestSamplingPolicyTypicalPath(t *testing.T) {
	nextConsumer := new(consumertest.TracesSink)

	mpe1 := &mockPolicyEvaluator{}

	policies := []*policy{
		{name: "mock-policy-1", evaluator: mpe1, attribute: metric.WithAttributes(attribute.String("policy", "mock-policy-1"))},
	}

	controller := newTestTSPController()
	cfg := Config{
		DecisionWait: defaultTestDecisionWait,
		NumTraces:    defaultNumTraces,
		Options: []Option{
			withTestController(controller),
			withPolicies(policies),
		},
	}
	p, err := newTracesProcessor(t.Context(), processortest.NewNopSettings(metadata.Type), nextConsumer, cfg)
	require.NoError(t, err)

	require.NoError(t, p.Start(t.Context(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(t, p.Shutdown(t.Context()))
	}()

	mpe1.NextDecision = samplingpolicy.Sampled

	// Generate and deliver first span
	require.NoError(t, p.ConsumeTraces(t.Context(), simpleTraces()))

	// The first tick won't do anything
	controller.waitForTick()
	require.Equal(t, 0, mpe1.EvaluationCount)

	// This will cause policy evaluations on the first span
	controller.waitForTick()

	// Both policies should have been evaluated once
	require.Equal(t, 1, mpe1.EvaluationCount)

	// The final decision SHOULD be Sampled.
	require.Equal(t, 1, nextConsumer.SpanCount())
}

func TestSamplingPolicyInvertSampled(t *testing.T) {
	nextConsumer := new(consumertest.TracesSink)
	controller := newTestTSPController()

	mpe1 := &mockPolicyEvaluator{}

	policies := []*policy{
		{name: "mock-policy-1", evaluator: mpe1, attribute: metric.WithAttributes(attribute.String("policy", "mock-policy-1"))},
	}

	cfg := Config{
		DecisionWait: defaultTestDecisionWait,
		NumTraces:    defaultNumTraces,
		Options: []Option{
			withTestController(controller),
			withPolicies(policies),
		},
	}
	p, err := newTracesProcessor(t.Context(), processortest.NewNopSettings(metadata.Type), nextConsumer, cfg)
	require.NoError(t, err)

	require.NoError(t, p.Start(t.Context(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(t, p.Shutdown(t.Context()))
	}()

	mpe1.NextDecision = samplingpolicy.InvertSampled

	// Generate and deliver first span
	require.NoError(t, p.ConsumeTraces(t.Context(), simpleTraces()))

	// The first tick won't do anything
	controller.waitForTick()
	require.Equal(t, 0, mpe1.EvaluationCount)

	// This will cause policy evaluations on the first span
	controller.waitForTick()

	// Both policies should have been evaluated once
	require.Equal(t, 1, mpe1.EvaluationCount)

	// The final decision SHOULD be Sampled.
	require.Equal(t, 1, nextConsumer.SpanCount())
}

func TestSamplingMultiplePolicies(t *testing.T) {
	nextConsumer := new(consumertest.TracesSink)
	controller := newTestTSPController()

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
			withTestController(controller),
			withPolicies(policies),
		},
	}
	p, err := newTracesProcessor(t.Context(), processortest.NewNopSettings(metadata.Type), nextConsumer, cfg)
	require.NoError(t, err)

	require.NoError(t, p.Start(t.Context(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(t, p.Shutdown(t.Context()))
	}()

	// InvertNotSampled takes precedence
	mpe1.NextDecision = samplingpolicy.Sampled
	mpe2.NextDecision = samplingpolicy.Sampled

	// Generate and deliver first span
	require.NoError(t, p.ConsumeTraces(t.Context(), simpleTraces()))

	// The first tick won't do anything
	controller.waitForTick()
	require.Equal(t, 0, mpe1.EvaluationCount)
	require.Equal(t, 0, mpe2.EvaluationCount)

	// This will cause policy evaluations on the first span
	controller.waitForTick()

	// Both policies should have been evaluated once
	require.Equal(t, 1, mpe1.EvaluationCount)
	require.Equal(t, 1, mpe2.EvaluationCount)

	// The final decision SHOULD be Sampled.
	require.Equal(t, 1, nextConsumer.SpanCount())
}

func TestSamplingMultiplePolicies_WithRecordPolicy(t *testing.T) {
	controller := newTestTSPController()
	nextConsumer := new(consumertest.TracesSink)
	s := setupTestTelemetry()
	ct := s.newSettings()

	mpe1 := &mockPolicyEvaluator{}
	mpe2 := &mockPolicyEvaluator{}

	policies := []*policy{
		{name: "mock-policy-1", evaluator: mpe1, attribute: metric.WithAttributes(attribute.String("policy", "mock-policy-1"))},
		{name: "mock-policy-2", evaluator: mpe2, attribute: metric.WithAttributes(attribute.String("policy", "mock-policy-2"))},
	}

	cfg := Config{
		DecisionWait: defaultTestDecisionWait,
		NumTraces:    defaultNumTraces,
		Options:      []Option{withTestController(controller), withPolicies(policies), withRecordPolicy()},
	}

	p, err := newTracesProcessor(t.Context(), ct, nextConsumer, cfg)
	require.NoError(t, err)

	require.NoError(t, p.Start(t.Context(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(t, p.Shutdown(t.Context()))
	}()

	// First policy takes precedence
	mpe1.NextDecision = samplingpolicy.Sampled
	mpe2.NextDecision = samplingpolicy.Sampled

	// Generate and deliver first span
	require.NoError(t, p.ConsumeTraces(t.Context(), simpleTraces()))

	// The first tick won't do anything
	controller.waitForTick()
	// This will cause policy evaluations on the first span
	controller.waitForTick()

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
	controller := newTestTSPController()

	mpe1 := &mockPolicyEvaluator{}

	policies := []*policy{
		{name: "mock-policy-1", evaluator: mpe1, attribute: metric.WithAttributes(attribute.String("policy", "mock-policy-1"))},
	}

	cfg := Config{
		DecisionWait: defaultTestDecisionWait,
		NumTraces:    defaultNumTraces,
		Options: []Option{
			withTestController(controller),
			withPolicies(policies),
		},
	}
	p, err := newTracesProcessor(t.Context(), processortest.NewNopSettings(metadata.Type), nextConsumer, cfg)
	require.NoError(t, err)

	require.NoError(t, p.Start(t.Context(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(t, p.Shutdown(t.Context()))
	}()

	// InvertNotSampled takes precedence
	mpe1.NextDecision = samplingpolicy.NotSampled

	// Generate and deliver first span
	require.NoError(t, p.ConsumeTraces(t.Context(), simpleTraces()))

	// The first tick won't do anything
	controller.waitForTick()
	require.Equal(t, 0, mpe1.EvaluationCount)

	// This will cause policy evaluations on the first span
	controller.waitForTick()

	// Both policies should have been evaluated once
	require.Equal(t, 1, mpe1.EvaluationCount)

	// The final decision SHOULD be NotSampled.
	require.Equal(t, 0, nextConsumer.SpanCount())
}

func TestSamplingPolicyDecisionNotSampled_WithRecordPolicy(t *testing.T) {
	controller := newTestTSPController()
	nextConsumer := new(consumertest.TracesSink)
	s := setupTestTelemetry()
	ct := s.newSettings()

	mpe1 := &mockPolicyEvaluator{}

	policies := []*policy{
		{name: "mock-policy-1", evaluator: mpe1, attribute: metric.WithAttributes(attribute.String("policy", "mock-policy-1"))},
	}

	cfg := Config{
		DecisionWait: defaultTestDecisionWait,
		NumTraces:    defaultNumTraces,
		Options:      []Option{withTestController(controller), withPolicies(policies), withRecordPolicy()},
	}

	p, err := newTracesProcessor(t.Context(), ct, nextConsumer, cfg)
	require.NoError(t, err)

	require.NoError(t, p.Start(t.Context(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(t, p.Shutdown(t.Context()))
	}()

	// InvertNotSampled takes precedence
	mpe1.NextDecision = samplingpolicy.NotSampled

	// Generate and deliver first span
	require.NoError(t, p.ConsumeTraces(t.Context(), simpleTraces()))

	// The first tick won't do anything
	controller.waitForTick()
	// This will cause policy evaluations on the first span
	controller.waitForTick()

	// The final decision SHOULD be NotSampled.
	require.Equal(t, 0, nextConsumer.SpanCount())
}

func TestSamplingPolicyDecisionInvertNotSampled(t *testing.T) {
	nextConsumer := new(consumertest.TracesSink)
	controller := newTestTSPController()

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
			withTestController(controller),
			withPolicies(policies),
		},
	}
	p, err := newTracesProcessor(t.Context(), processortest.NewNopSettings(metadata.Type), nextConsumer, cfg)
	require.NoError(t, err)

	require.NoError(t, p.Start(t.Context(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(t, p.Shutdown(t.Context()))
	}()

	// InvertNotSampled takes precedence
	mpe1.NextDecision = samplingpolicy.InvertNotSampled
	mpe2.NextDecision = samplingpolicy.Sampled

	// Generate and deliver first span
	require.NoError(t, p.ConsumeTraces(t.Context(), simpleTraces()))

	// The first tick won't do anything
	controller.waitForTick()
	require.Equal(t, 0, mpe1.EvaluationCount)
	require.Equal(t, 0, mpe2.EvaluationCount)

	// This will cause policy evaluations on the first span
	controller.waitForTick()

	// Both policies should have been evaluated once
	require.Equal(t, 1, mpe1.EvaluationCount)
	require.Equal(t, 1, mpe2.EvaluationCount)

	// The final decision SHOULD be NotSampled.
	require.Equal(t, 0, nextConsumer.SpanCount())
}

func TestSamplingPolicyDecisionInvertNotSampled_WithRecordPolicy(t *testing.T) {
	nextConsumer := new(consumertest.TracesSink)
	controller := newTestTSPController()
	s := setupTestTelemetry()
	ct := s.newSettings()

	mpe1 := &mockPolicyEvaluator{}
	mpe2 := &mockPolicyEvaluator{}

	policies := []*policy{
		{name: "mock-policy-1", evaluator: mpe1, attribute: metric.WithAttributes(attribute.String("policy", "mock-policy-1"))},
		{name: "mock-policy-2", evaluator: mpe2, attribute: metric.WithAttributes(attribute.String("policy", "mock-policy-2"))},
	}

	cfg := Config{
		DecisionWait: defaultTestDecisionWait,
		NumTraces:    defaultNumTraces,
		Options:      []Option{withTestController(controller), withPolicies(policies), withRecordPolicy()},
	}

	p, err := newTracesProcessor(t.Context(), ct, nextConsumer, cfg)
	require.NoError(t, err)

	require.NoError(t, p.Start(t.Context(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(t, p.Shutdown(t.Context()))
	}()

	// InvertNotSampled takes precedence
	mpe1.NextDecision = samplingpolicy.InvertNotSampled
	mpe2.NextDecision = samplingpolicy.Sampled

	// Generate and deliver first span
	require.NoError(t, p.ConsumeTraces(t.Context(), simpleTraces()))

	// The first tick won't do anything
	controller.waitForTick()
	// This will cause policy evaluations on the first span
	controller.waitForTick()

	// The final decision SHOULD be NotSampled.
	require.Equal(t, 0, nextConsumer.SpanCount())
}

func TestLateArrivingSpansAssignedOriginalDecision(t *testing.T) {
	nextConsumer := new(consumertest.TracesSink)
	controller := newTestTSPController()

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
			withTestController(controller),
			withPolicies(policies),
		},
	}
	p, err := newTracesProcessor(t.Context(), processortest.NewNopSettings(metadata.Type), nextConsumer, cfg)
	require.NoError(t, err)

	require.NoError(t, p.Start(t.Context(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(t, p.Shutdown(t.Context()))
	}()

	// We are going to create 2 spans belonging to the same trace
	traceID := uInt64ToTraceID(1)

	// The combined decision from the policies is NotSampled
	mpe1.NextDecision = samplingpolicy.InvertSampled
	mpe2.NextDecision = samplingpolicy.NotSampled

	// A function that return a ptrace.Traces containing a single span for the single trace we are using.
	spanIndexToTraces := func(spanIndex uint64) ptrace.Traces {
		traces := ptrace.NewTraces()
		span := traces.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
		span.SetTraceID(traceID)
		span.SetSpanID(uInt64ToSpanID(spanIndex))
		return traces
	}

	// Generate and deliver first span
	require.NoError(t, p.ConsumeTraces(t.Context(), spanIndexToTraces(1)))

	// The first tick won't do anything
	controller.waitForTick()
	require.Equal(t, 0, mpe1.EvaluationCount)
	require.Equal(t, 0, mpe2.EvaluationCount)

	// This will cause policy evaluations on the first span
	controller.waitForTick()

	// Both policies should have been evaluated once
	require.Equal(t, 1, mpe1.EvaluationCount)
	require.Equal(t, 1, mpe2.EvaluationCount)

	// The final decision SHOULD be NotSampled.
	require.Equal(t, 0, nextConsumer.SpanCount())

	// Generate and deliver final span for the trace which SHOULD get the same sampling decision as the first span.
	// The policies should NOT be evaluated again.
	require.NoError(t, p.ConsumeTraces(t.Context(), spanIndexToTraces(2)))
	require.Equal(t, 1, mpe1.EvaluationCount)
	require.Equal(t, 1, mpe2.EvaluationCount)
	require.Equal(t, 0, nextConsumer.SpanCount(), "original final decision not honored")
}

func TestLateArrivingSpanUsesDecisionCache(t *testing.T) {
	nextConsumer := new(consumertest.TracesSink)
	controller := newTestTSPController()

	mpe := &mockPolicyEvaluator{}
	policies := []*policy{
		{name: "mock-policy-1", evaluator: mpe, attribute: metric.WithAttributes(attribute.String("policy", "mock-policy-1"))},
	}

	// Use this instead of the default no-op cache
	c, err := cache.NewLRUDecisionCache(200)
	require.NoError(t, err)

	cfg := Config{
		DecisionWait: defaultTestDecisionWait * 10,
		NumTraces:    defaultNumTraces,
		Options: []Option{
			withTestController(controller),
			withPolicies(policies),
			WithSampledDecisionCache(c),
			withRecordPolicy(),
		},
	}
	p, err := newTracesProcessor(t.Context(), processortest.NewNopSettings(metadata.Type), nextConsumer, cfg)
	require.NoError(t, err)

	require.NoError(t, p.Start(t.Context(), componenttest.NewNopHost()))
	defer func(p processor.Traces) {
		require.NoError(t, p.Shutdown(t.Context()))
	}(p)

	// We are going to create 2 spans belonging to the same trace
	traceID := uInt64ToTraceID(1)

	// The first span will be sampled, this will later be set to not sampled, but the sampling decision will be cached
	mpe.NextDecision = samplingpolicy.Sampled

	// A function that return a ptrace.Traces containing a single span for the single trace we are using.
	spanIndexToTraces := func(spanIndex uint64) ptrace.Traces {
		traces := ptrace.NewTraces()
		span := traces.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
		span.SetTraceID(traceID)
		span.SetSpanID(uInt64ToSpanID(spanIndex))
		return traces
	}

	// Generate and deliver first span
	require.NoError(t, p.ConsumeTraces(t.Context(), spanIndexToTraces(1)))

	// The first tick won't do anything
	controller.waitForTick()
	require.Equal(t, 0, mpe.EvaluationCount)

	// This will cause policy evaluations on the first span
	controller.waitForTick()

	// Policy should have been evaluated once
	require.Equal(t, 1, mpe.EvaluationCount)

	// The final decision SHOULD be Sampled.
	require.Equal(t, 1, nextConsumer.SpanCount())

	// Now we create a brand new tailsampling span processor with the same decision cache
	p, err = newTracesProcessor(t.Context(), processortest.NewNopSettings(metadata.Type), nextConsumer, cfg)
	require.NoError(t, err)
	require.NoError(t, p.Start(t.Context(), componenttest.NewNopHost()))
	defer func(p processor.Traces) {
		require.NoError(t, p.Shutdown(t.Context()))
	}(p)

	// Set next decision to not sampled, ensuring the next decision is determined by the decision cache, not the policy
	mpe.NextDecision = samplingpolicy.NotSampled

	// Generate and deliver final span for the trace which SHOULD get the same sampling decision as the first span.
	// The policies should NOT be evaluated again.
	require.NoError(t, p.ConsumeTraces(t.Context(), spanIndexToTraces(2)))
	controller.waitForTick()

	// Policy should still have been evaluated only once
	require.Equal(t, 1, mpe.EvaluationCount)
	require.Equal(t, 2, nextConsumer.SpanCount(), "original final decision not honored")
	allTraces := nextConsumer.AllTraces()
	require.Len(t, allTraces, 2)

	// Second trace should have the cached decision attribute
	policyAttr, ok := allTraces[1].ResourceSpans().At(0).ScopeSpans().At(0).Scope().Attributes().Get("tailsampling.policy")
	if !ok {
		assert.FailNow(t, "Did not find expected attribute")
	}
	require.Equal(t, "mock-policy-1", policyAttr.AsString())
	cacheAttr, ok := allTraces[1].ResourceSpans().At(0).ScopeSpans().At(0).Scope().Attributes().Get("tailsampling.cached_decision")
	if !ok {
		assert.FailNow(t, "Did not find expected attribute")
	}
	require.True(t, cacheAttr.Bool())
}

func TestLateArrivingSpanUsesDecisionCacheWhenDropped(t *testing.T) {
	nextConsumer := new(consumertest.TracesSink)
	controller := newTestTSPController()

	mpe := &mockPolicyEvaluator{}
	policies := []*policy{
		{name: "mock-policy-1", evaluator: mpe, attribute: metric.WithAttributes(attribute.String("policy", "mock-policy-1"))},
	}

	// Use this instead of the default no-op cache
	c, err := cache.NewLRUDecisionCache(200)
	require.NoError(t, err)

	cfg := Config{
		DecisionWait: defaultTestDecisionWait * 10,
		NumTraces:    defaultNumTraces,
		Options: []Option{
			withTestController(controller),
			withPolicies(policies),
			WithNonSampledDecisionCache(c),
			withRecordPolicy(),
		},
	}
	p, err := newTracesProcessor(t.Context(), processortest.NewNopSettings(metadata.Type), nextConsumer, cfg)
	require.NoError(t, err)

	require.NoError(t, p.Start(t.Context(), componenttest.NewNopHost()))
	defer func(p processor.Traces) {
		require.NoError(t, p.Shutdown(t.Context()))
	}(p)

	// We are going to create 2 spans belonging to the same trace
	traceID := uInt64ToTraceID(1)

	// The first span will be sampled, this will later be set to not sampled, but the sampling decision will be cached
	mpe.NextDecision = samplingpolicy.Dropped

	// A function that return a ptrace.Traces containing a single span for the single trace we are using.
	spanIndexToTraces := func(spanIndex uint64) ptrace.Traces {
		traces := ptrace.NewTraces()
		span := traces.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
		span.SetTraceID(traceID)
		span.SetSpanID(uInt64ToSpanID(spanIndex))
		return traces
	}

	// Generate and deliver first span
	require.NoError(t, p.ConsumeTraces(t.Context(), spanIndexToTraces(1)))

	// The first tick won't do anything
	controller.waitForTick()
	require.Equal(t, 0, mpe.EvaluationCount)

	// This will cause policy evaluations on the first span
	controller.waitForTick()

	// Policy should have been evaluated once
	require.Equal(t, 1, mpe.EvaluationCount)

	// The final decision SHOULD be Dropped.
	require.Equal(t, 0, nextConsumer.SpanCount())

	// Now we create a brand new tailsampling span processor with the same decision cache
	p, err = newTracesProcessor(t.Context(), processortest.NewNopSettings(metadata.Type), nextConsumer, cfg)
	require.NoError(t, err)
	require.NoError(t, p.Start(t.Context(), componenttest.NewNopHost()))
	defer func(p processor.Traces) {
		require.NoError(t, p.Shutdown(t.Context()))
	}(p)

	// Set next decision to not sampled, ensuring the next decision is determined by the decision cache, not the policy
	mpe.NextDecision = samplingpolicy.NotSampled

	// Generate and deliver final span for the trace which SHOULD get the same sampling decision as the first span.
	// The policies should NOT be evaluated again.
	require.NoError(t, p.ConsumeTraces(t.Context(), spanIndexToTraces(2)))
	controller.waitForTick()

	// Policy should still have been evaluated only once
	require.Equal(t, 1, mpe.EvaluationCount)
	require.Equal(t, 0, nextConsumer.SpanCount(), "original final decision not honored")
	allTraces := nextConsumer.AllTraces()
	require.Empty(t, allTraces)

	metadata, ok := c.Get(traceID)
	if !ok {
		assert.FailNow(t, "Did not find expected metadata")
	}
	require.Equal(t, "mock-policy-1", metadata.PolicyName)
}

func TestLateArrivingSpanWithoutCacheMetadata(t *testing.T) {
	nextConsumer := new(consumertest.TracesSink)
	controller := newTestTSPController()

	mpe := &mockPolicyEvaluator{}
	policies := []*policy{
		{name: "mock-policy-1", evaluator: mpe, attribute: metric.WithAttributes(attribute.String("policy", "mock-policy-1"))},
	}

	// Use this instead of the default no-op cache
	c, err := cache.NewLRUDecisionCache(200)
	require.NoError(t, err)

	cfg := Config{
		DecisionWait: defaultTestDecisionWait * 10,
		NumTraces:    defaultNumTraces,
		Options: []Option{
			withTestController(controller),
			withPolicies(policies),
			WithSampledDecisionCache(c),
			withRecordPolicy(),
		},
	}

	// We are going to create 2 spans belonging to the same trace
	traceID := uInt64ToTraceID(1)

	// The first span will be sampled, this will later be set to not sampled, but the sampling decision will be cached
	mpe.NextDecision = samplingpolicy.Sampled

	// A function that return a ptrace.Traces containing a single span for the single trace we are using.
	spanIndexToTraces := func(spanIndex uint64) ptrace.Traces {
		traces := ptrace.NewTraces()
		span := traces.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
		span.SetTraceID(traceID)
		span.SetSpanID(uInt64ToSpanID(spanIndex))
		return traces
	}

	// Simulate a decision cached without any metadata.
	c.Put(traceID, cache.DecisionMetadata{})

	// Now we create a brand new tailsampling span processor with the same decision cache
	p, err := newTracesProcessor(t.Context(), processortest.NewNopSettings(metadata.Type), nextConsumer, cfg)
	require.NoError(t, err)
	require.NoError(t, p.Start(t.Context(), componenttest.NewNopHost()))
	defer func(p processor.Traces) {
		require.NoError(t, p.Shutdown(t.Context()))
	}(p)

	// Set next decision to not sampled, ensuring the next decision is determined by the decision cache, not the policy
	mpe.NextDecision = samplingpolicy.NotSampled

	// Generate and deliver final span for the trace which SHOULD get the same sampling decision as the first span.
	// The policies should NOT be evaluated again.
	require.NoError(t, p.ConsumeTraces(t.Context(), spanIndexToTraces(2)))
	controller.waitForTick()

	// Policy should never have been evaluated
	require.Equal(t, 0, mpe.EvaluationCount)
	require.Equal(t, 1, nextConsumer.SpanCount(), "original final decision not honored")
	allTraces := nextConsumer.AllTraces()
	require.Len(t, allTraces, 1)

	// Second trace should have the cached decision attribute
	cacheAttr, ok := allTraces[0].ResourceSpans().At(0).ScopeSpans().At(0).Scope().Attributes().Get("tailsampling.cached_decision")
	if !ok {
		assert.FailNow(t, "Did not find expected attribute")
	}
	require.True(t, cacheAttr.Bool())
}

func TestLateSpanUsesNonSampledDecisionCache(t *testing.T) {
	nextConsumer := new(consumertest.TracesSink)
	controller := newTestTSPController()

	mpe := &mockPolicyEvaluator{}
	policies := []*policy{
		{name: "mock-policy-1", evaluator: mpe, attribute: metric.WithAttributes(attribute.String("policy", "mock-policy-1"))},
	}

	// Use this instead of the default no-op cache
	c, err := cache.NewLRUDecisionCache(200)
	require.NoError(t, err)

	cfg := Config{
		DecisionWait: defaultTestDecisionWait * 10,
		NumTraces:    defaultNumTraces,
		Options: []Option{
			withTestController(controller),
			withPolicies(policies),
			WithNonSampledDecisionCache(c),
		},
	}
	p, err := newTracesProcessor(t.Context(), processortest.NewNopSettings(metadata.Type), nextConsumer, cfg)
	require.NoError(t, err)

	require.NoError(t, p.Start(t.Context(), componenttest.NewNopHost()))
	defer func(p processor.Traces) {
		require.NoError(t, p.Shutdown(t.Context()))
	}(p)

	// We are going to create 2 spans belonging to the same trace
	traceID := uInt64ToTraceID(1)

	// The first span will be NOT sampled, this will later be set to sampled, but the sampling decision will be cached
	mpe.NextDecision = samplingpolicy.NotSampled

	// A function that return a ptrace.Traces containing a single span for the single trace we are using.
	spanIndexToTraces := func(spanIndex uint64) ptrace.Traces {
		traces := ptrace.NewTraces()
		span := traces.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
		span.SetTraceID(traceID)
		span.SetSpanID(uInt64ToSpanID(spanIndex))
		return traces
	}

	// Generate and deliver first span
	require.NoError(t, p.ConsumeTraces(t.Context(), spanIndexToTraces(1)))

	// The first tick won't do anything
	controller.waitForTick()
	require.Equal(t, 0, mpe.EvaluationCount)

	// This will cause policy evaluations on the first span
	controller.waitForTick()

	// Policy should have been evaluated once
	require.Equal(t, 1, mpe.EvaluationCount)

	// The final decision SHOULD be NOT Sampled.
	require.Equal(t, 0, nextConsumer.SpanCount())

	// Now we create a brand new tailsampling span processor with the same decision cache
	p, err = newTracesProcessor(t.Context(), processortest.NewNopSettings(metadata.Type), nextConsumer, cfg)
	require.NoError(t, err)
	require.NoError(t, p.Start(t.Context(), componenttest.NewNopHost()))
	defer func(p processor.Traces) {
		require.NoError(t, p.Shutdown(t.Context()))
	}(p)

	// Set next decision to sampled, ensuring the next decision is determined by the decision cache, not the policy
	mpe.NextDecision = samplingpolicy.Sampled

	// Generate and deliver final span for the trace which SHOULD get the same sampling decision as the first span.
	// The policies should NOT be evaluated again.
	require.NoError(t, p.ConsumeTraces(t.Context(), spanIndexToTraces(2)))
	controller.waitForTick()

	// Policy should still have been evaluated only once
	require.Equal(t, 1, mpe.EvaluationCount)
	require.Equal(t, 0, nextConsumer.SpanCount(), "original final decision not honored")
}

func TestSampleOnFirstMatch(t *testing.T) {
	nextConsumer := new(consumertest.TracesSink)
	controller := newTestTSPController()

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
			withTestController(controller),
			withPolicies(policies),
		},
	}
	p, err := newTracesProcessor(t.Context(), processortest.NewNopSettings(metadata.Type), nextConsumer, cfg)
	require.NoError(t, err)

	require.NoError(t, p.Start(t.Context(), componenttest.NewNopHost()))
	defer func(p processor.Traces) {
		require.NoError(t, p.Shutdown(t.Context()))
	}(p)

	// Second policy matches, last policy should not be evaluated
	mpe1.NextDecision = samplingpolicy.NotSampled
	mpe2.NextDecision = samplingpolicy.Sampled

	// Generate and deliver first span
	require.NoError(t, p.ConsumeTraces(t.Context(), simpleTraces()))

	// The first tick won't do anything
	controller.waitForTick()
	require.Equal(t, 0, mpe1.EvaluationCount)
	require.Equal(t, 0, mpe2.EvaluationCount)
	require.Equal(t, 0, mpe3.EvaluationCount)

	// This will cause policy evaluations on the first span
	controller.waitForTick()

	// Only the first policy should have been evaluated
	require.Equal(t, 1, mpe1.EvaluationCount)
	require.Equal(t, 1, mpe2.EvaluationCount)
	require.Equal(t, 0, mpe3.EvaluationCount)

	// The final decision SHOULD be Sampled.
	require.Equal(t, 1, nextConsumer.SpanCount())
}

func TestRateLimiter(t *testing.T) {
	nextConsumer := new(consumertest.TracesSink)
	controller := newTestTSPController()

	cfg := Config{
		DecisionWait:       defaultTestDecisionWait,
		NumTraces:          defaultNumTraces,
		SampleOnFirstMatch: true,
		PolicyCfgs: []PolicyCfg{
			{
				sharedPolicyCfg: sharedPolicyCfg{
					Name: "test-policy-1",
					Type: "rate_limiting",
					RateLimitingCfg: RateLimitingCfg{
						SpansPerSecond: 2,
					},
				},
			},
		},
		Options: []Option{
			withTestController(controller),
		},
	}
	p, err := newTracesProcessor(t.Context(), processortest.NewNopSettings(metadata.Type), nextConsumer, cfg)
	require.NoError(t, err)

	require.NoError(t, p.Start(t.Context(), componenttest.NewNopHost()))
	defer func(p processor.Traces) {
		require.NoError(t, p.Shutdown(t.Context()))
	}(p)

	for i := range 11 {
		require.NoError(t, p.ConsumeTraces(t.Context(), simpleTracesWithID(uInt64ToTraceID(uint64(i)))))
	}
	controller.waitForTick()
	controller.waitForTick()

	// The rate limiter resets every time time.Now().Unix() changes, so
	// depending on whether this test runs close to the second boundary, the
	// number of spans sampled will be 1 or 2.
	require.LessOrEqual(t, nextConsumer.SpanCount(), 2)
	require.GreaterOrEqual(t, nextConsumer.SpanCount(), 1)

	allTraces := nextConsumer.AllTraces()
	sampledTraceIDs := make(map[pcommon.TraceID]struct{})
	for _, trace := range allTraces {
		sampledTraceIDs[trace.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).TraceID()] = struct{}{}
	}

	require.LessOrEqual(t, len(sampledTraceIDs), 2)
	require.GreaterOrEqual(t, len(sampledTraceIDs), 1)
}
