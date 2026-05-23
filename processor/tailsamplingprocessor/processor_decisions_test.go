// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tailsamplingprocessor

import (
	"context"
	"sync"
	"testing"
	"time"

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

type batchSizeTrackingEvaluator struct {
	mu                sync.Mutex
	decisions         []samplingpolicy.Decision
	evaluationBatches []int
}

func (e *batchSizeTrackingEvaluator) Evaluate(_ context.Context, _ pcommon.TraceID, trace *samplingpolicy.TraceData) (samplingpolicy.Decision, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.evaluationBatches = append(e.evaluationBatches, trace.ReceivedBatches.SpanCount())
	if len(e.decisions) == 0 {
		return samplingpolicy.NotSampled, nil
	}
	decision := e.decisions[0]
	e.decisions = e.decisions[1:]
	return decision, nil
}

func (*batchSizeTrackingEvaluator) IsStateful() bool {
	return false
}

func (e *batchSizeTrackingEvaluator) EvaluationCount() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.evaluationBatches)
}

func (e *batchSizeTrackingEvaluator) EvaluationBatches() []int {
	e.mu.Lock()
	defer e.mu.Unlock()
	batches := make([]int, len(e.evaluationBatches))
	copy(batches, e.evaluationBatches)
	return batches
}

func TestSamplingPolicyTypicalPath(t *testing.T) {
	nextConsumer := new(consumertest.TracesSink)

	mpe1 := &mockPolicyEvaluator{}

	policies := []*policy{
		{name: "mock-policy-1", evaluator: mpe1, attribute: metric.WithAttributes(attribute.String("policy", "mock-policy-1"))},
	}

	controller := newTestTSPController()
	cfg := Config{
		SamplingStrategy: samplingStrategyTraceComplete,
		DecisionWait:     defaultTestDecisionWait,
		NumTraces:        defaultNumTraces,
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

	mpe1.SetDecision(samplingpolicy.Sampled)

	// Generate and deliver first span
	require.NoError(t, p.ConsumeTraces(t.Context(), simpleTraces()))

	// The first tick won't do anything
	controller.waitForTick()
	require.Equal(t, 0, mpe1.EvaluationCount())

	// This will cause policy evaluations on the first span
	controller.waitForTick()

	// Both policies should have been evaluated once
	require.Equal(t, 1, mpe1.EvaluationCount())

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
		SamplingStrategy: samplingStrategyTraceComplete,
		DecisionWait:     defaultTestDecisionWait,
		NumTraces:        defaultNumTraces,
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

	//nolint:staticcheck // SA1019: Use of inverted decisions until they are fully removed.
	mpe1.SetDecision(samplingpolicy.InvertSampled)

	// Generate and deliver first span
	require.NoError(t, p.ConsumeTraces(t.Context(), simpleTraces()))

	// The first tick won't do anything
	controller.waitForTick()
	require.Equal(t, 0, mpe1.EvaluationCount())

	// This will cause policy evaluations on the first span
	controller.waitForTick()

	// Both policies should have been evaluated once
	require.Equal(t, 1, mpe1.EvaluationCount())

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
		SamplingStrategy: samplingStrategyTraceComplete,
		DecisionWait:     defaultTestDecisionWait,
		NumTraces:        defaultNumTraces,
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
	mpe1.SetDecision(samplingpolicy.Sampled)
	mpe2.SetDecision(samplingpolicy.Sampled)

	// Generate and deliver first span
	require.NoError(t, p.ConsumeTraces(t.Context(), simpleTraces()))

	// The first tick won't do anything
	controller.waitForTick()
	require.Equal(t, 0, mpe1.EvaluationCount())
	require.Equal(t, 0, mpe2.EvaluationCount())

	// This will cause policy evaluations on the first span
	controller.waitForTick()

	// Both policies should have been evaluated once
	require.Equal(t, 1, mpe1.EvaluationCount())
	require.Equal(t, 1, mpe2.EvaluationCount())

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
		SamplingStrategy: samplingStrategyTraceComplete,
		DecisionWait:     defaultTestDecisionWait,
		NumTraces:        defaultNumTraces,
		Options:          []Option{withTestController(controller), withPolicies(policies), withRecordPolicy()},
	}

	p, err := newTracesProcessor(t.Context(), ct, nextConsumer, cfg)
	require.NoError(t, err)

	require.NoError(t, p.Start(t.Context(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(t, p.Shutdown(t.Context()))
	}()

	// First policy takes precedence
	mpe1.SetDecision(samplingpolicy.Sampled)
	mpe2.SetDecision(samplingpolicy.Sampled)

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
		SamplingStrategy: samplingStrategyTraceComplete,
		DecisionWait:     defaultTestDecisionWait,
		NumTraces:        defaultNumTraces,
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
	mpe1.SetDecision(samplingpolicy.NotSampled)

	// Generate and deliver first span
	require.NoError(t, p.ConsumeTraces(t.Context(), simpleTraces()))

	// The first tick won't do anything
	controller.waitForTick()
	require.Equal(t, 0, mpe1.EvaluationCount())

	// This will cause policy evaluations on the first span
	controller.waitForTick()

	// Both policies should have been evaluated once
	require.Equal(t, 1, mpe1.EvaluationCount())

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
		SamplingStrategy: samplingStrategyTraceComplete,
		DecisionWait:     defaultTestDecisionWait,
		NumTraces:        defaultNumTraces,
		Options:          []Option{withTestController(controller), withPolicies(policies), withRecordPolicy()},
	}

	p, err := newTracesProcessor(t.Context(), ct, nextConsumer, cfg)
	require.NoError(t, err)

	require.NoError(t, p.Start(t.Context(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(t, p.Shutdown(t.Context()))
	}()

	// InvertNotSampled takes precedence
	mpe1.SetDecision(samplingpolicy.NotSampled)

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
		SamplingStrategy: samplingStrategyTraceComplete,
		DecisionWait:     defaultTestDecisionWait,
		NumTraces:        defaultNumTraces,
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
	//nolint:staticcheck // SA1019: Use of inverted decisions until they are fully removed.
	mpe1.SetDecision(samplingpolicy.InvertNotSampled)
	mpe2.SetDecision(samplingpolicy.Sampled)

	// Generate and deliver first span
	require.NoError(t, p.ConsumeTraces(t.Context(), simpleTraces()))

	// The first tick won't do anything
	controller.waitForTick()
	require.Equal(t, 0, mpe1.EvaluationCount())
	require.Equal(t, 0, mpe2.EvaluationCount())

	// This will cause policy evaluations on the first span
	controller.waitForTick()

	// Both policies should have been evaluated once
	require.Equal(t, 1, mpe1.EvaluationCount())
	require.Equal(t, 1, mpe2.EvaluationCount())

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
		SamplingStrategy: samplingStrategyTraceComplete,
		DecisionWait:     defaultTestDecisionWait,
		NumTraces:        defaultNumTraces,
		Options:          []Option{withTestController(controller), withPolicies(policies), withRecordPolicy()},
	}

	p, err := newTracesProcessor(t.Context(), ct, nextConsumer, cfg)
	require.NoError(t, err)

	require.NoError(t, p.Start(t.Context(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(t, p.Shutdown(t.Context()))
	}()

	// InvertNotSampled takes precedence
	//nolint:staticcheck // SA1019: Use of inverted decisions until they are fully removed.
	mpe1.SetDecision(samplingpolicy.InvertNotSampled)
	mpe2.SetDecision(samplingpolicy.Sampled)

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
		SamplingStrategy: samplingStrategyTraceComplete,
		DecisionWait:     defaultTestDecisionWait,
		NumTraces:        defaultNumTraces,
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
	//nolint:staticcheck // SA1019: Use of inverted decisions until they are fully removed.
	mpe1.SetDecision(samplingpolicy.InvertSampled)
	mpe2.SetDecision(samplingpolicy.NotSampled)

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
	require.Equal(t, 0, mpe1.EvaluationCount())
	require.Equal(t, 0, mpe2.EvaluationCount())

	// This will cause policy evaluations on the first span
	controller.waitForTick()

	// Both policies should have been evaluated once
	require.Equal(t, 1, mpe1.EvaluationCount())
	require.Equal(t, 1, mpe2.EvaluationCount())

	// The final decision SHOULD be NotSampled.
	require.Equal(t, 0, nextConsumer.SpanCount())

	// Generate and deliver final span for the trace which SHOULD get the same sampling decision as the first span.
	// The policies should NOT be evaluated again.
	require.NoError(t, p.ConsumeTraces(t.Context(), spanIndexToTraces(2)))
	require.Equal(t, 1, mpe1.EvaluationCount())
	require.Equal(t, 1, mpe2.EvaluationCount())
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
		SamplingStrategy: samplingStrategyTraceComplete,
		DecisionWait:     defaultTestDecisionWait * 10,
		NumTraces:        defaultNumTraces,
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
	mpe.SetDecision(samplingpolicy.Sampled)

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
	require.Equal(t, 0, mpe.EvaluationCount())

	// This will cause policy evaluations on the first span
	controller.waitForTick()

	// Policy should have been evaluated once
	require.Equal(t, 1, mpe.EvaluationCount())

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
	mpe.SetDecision(samplingpolicy.NotSampled)

	// Generate and deliver final span for the trace which SHOULD get the same sampling decision as the first span.
	// The policies should NOT be evaluated again.
	require.NoError(t, p.ConsumeTraces(t.Context(), spanIndexToTraces(2)))
	controller.waitForTick()

	// Policy should still have been evaluated only once
	require.Equal(t, 1, mpe.EvaluationCount())
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
		SamplingStrategy: samplingStrategyTraceComplete,
		DecisionWait:     defaultTestDecisionWait * 10,
		NumTraces:        defaultNumTraces,
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
	mpe.SetDecision(samplingpolicy.Dropped)

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
	require.Equal(t, 0, mpe.EvaluationCount())

	// This will cause policy evaluations on the first span
	controller.waitForTick()

	// Policy should have been evaluated once
	require.Equal(t, 1, mpe.EvaluationCount())

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
	mpe.SetDecision(samplingpolicy.NotSampled)

	// Generate and deliver final span for the trace which SHOULD get the same sampling decision as the first span.
	// The policies should NOT be evaluated again.
	require.NoError(t, p.ConsumeTraces(t.Context(), spanIndexToTraces(2)))
	controller.waitForTick()

	// Policy should still have been evaluated only once
	require.Equal(t, 1, mpe.EvaluationCount())
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
		SamplingStrategy: samplingStrategyTraceComplete,
		DecisionWait:     defaultTestDecisionWait * 10,
		NumTraces:        defaultNumTraces,
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
	mpe.SetDecision(samplingpolicy.Sampled)

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
	mpe.SetDecision(samplingpolicy.NotSampled)

	// Generate and deliver final span for the trace which SHOULD get the same sampling decision as the first span.
	// The policies should NOT be evaluated again.
	require.NoError(t, p.ConsumeTraces(t.Context(), spanIndexToTraces(2)))
	controller.waitForTick()

	// Policy should never have been evaluated
	require.Equal(t, 0, mpe.EvaluationCount())
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
		SamplingStrategy: samplingStrategyTraceComplete,
		DecisionWait:     defaultTestDecisionWait * 10,
		NumTraces:        defaultNumTraces,
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
	mpe.SetDecision(samplingpolicy.NotSampled)

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
	require.Equal(t, 0, mpe.EvaluationCount())

	// This will cause policy evaluations on the first span
	controller.waitForTick()

	// Policy should have been evaluated once
	require.Equal(t, 1, mpe.EvaluationCount())

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
	mpe.SetDecision(samplingpolicy.Sampled)

	// Generate and deliver final span for the trace which SHOULD get the same sampling decision as the first span.
	// The policies should NOT be evaluated again.
	require.NoError(t, p.ConsumeTraces(t.Context(), spanIndexToTraces(2)))
	controller.waitForTick()

	// Policy should still have been evaluated only once
	require.Equal(t, 1, mpe.EvaluationCount())
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
		SamplingStrategy:   samplingStrategyTraceComplete,
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
	mpe1.SetDecision(samplingpolicy.NotSampled)
	mpe2.SetDecision(samplingpolicy.Sampled)

	// Generate and deliver first span
	require.NoError(t, p.ConsumeTraces(t.Context(), simpleTraces()))

	// The first tick won't do anything
	controller.waitForTick()
	require.Equal(t, 0, mpe1.EvaluationCount())
	require.Equal(t, 0, mpe2.EvaluationCount())
	require.Equal(t, 0, mpe3.EvaluationCount())

	// This will cause policy evaluations on the first span
	controller.waitForTick()

	// Only the first policy should have been evaluated
	require.Equal(t, 1, mpe1.EvaluationCount())
	require.Equal(t, 1, mpe2.EvaluationCount())
	require.Equal(t, 0, mpe3.EvaluationCount())

	// The final decision SHOULD be Sampled.
	require.Equal(t, 1, nextConsumer.SpanCount())
}

func TestSpanIngestEvaluatesOnIngestAndFinalizesOnTerminalDecision(t *testing.T) {
	nextConsumer := new(consumertest.TracesSink)
	controller := newTestTSPController()

	mpe := &mockPolicyEvaluator{}
	policies := []*policy{
		{name: "mock-policy-1", evaluator: mpe, attribute: metric.WithAttributes(attribute.String("policy", "mock-policy-1"))},
	}

	cfg := Config{
		DecisionWait:     defaultTestDecisionWait,
		NumTraces:        defaultNumTraces,
		SamplingStrategy: samplingStrategySpanIngest,
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

	traceID := uInt64ToTraceID(50)
	rootSpanID := uInt64ToSpanID(1)

	// First ingest path evaluation is non-terminal in span-ingest mode.
	mpe.SetDecision(samplingpolicy.NotSampled)
	require.NoError(t, p.ConsumeTraces(t.Context(), singleSpanTrace(traceID, uInt64ToSpanID(2), rootSpanID)))
	require.Eventually(t, func() bool { return mpe.EvaluationCount() == 1 }, time.Second, 10*time.Millisecond)
	require.Equal(t, 0, nextConsumer.SpanCount())

	// Second ingest path evaluation returns terminal Sampled and releases both spans.
	mpe.SetDecision(samplingpolicy.Sampled)
	require.NoError(t, p.ConsumeTraces(t.Context(), singleSpanTrace(traceID, rootSpanID, pcommon.SpanID{})))
	require.Eventually(t, func() bool {
		return mpe.EvaluationCount() == 2 && nextConsumer.SpanCount() == 2
	}, time.Second, 10*time.Millisecond)
}

func TestSpanIngestFinalizesOnDroppedDecision(t *testing.T) {
	nextConsumer := new(consumertest.TracesSink)
	controller := newTestTSPController()

	mpe := &mockPolicyEvaluator{}
	policies := []*policy{
		{name: "mock-policy-1", evaluator: mpe, attribute: metric.WithAttributes(attribute.String("policy", "mock-policy-1"))},
	}

	cfg := Config{
		DecisionWait:     defaultTestDecisionWait,
		NumTraces:        defaultNumTraces,
		SamplingStrategy: samplingStrategySpanIngest,
		DecisionCache: DecisionCacheConfig{
			NonSampledCacheSize: 64,
		},
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

	traceID := uInt64ToTraceID(51)
	rootSpanID := uInt64ToSpanID(1)

	// First ingest path evaluation is non-terminal in span-ingest mode.
	mpe.SetDecision(samplingpolicy.NotSampled)
	require.NoError(t, p.ConsumeTraces(t.Context(), singleSpanTrace(traceID, uInt64ToSpanID(2), rootSpanID)))
	require.Eventually(t, func() bool { return mpe.EvaluationCount() == 1 }, time.Second, 10*time.Millisecond)
	require.Equal(t, 0, nextConsumer.SpanCount())

	// Second ingest path evaluation returns terminal Dropped and finalizes trace.
	mpe.SetDecision(samplingpolicy.Dropped)
	require.NoError(t, p.ConsumeTraces(t.Context(), singleSpanTrace(traceID, rootSpanID, pcommon.SpanID{})))
	require.Eventually(t, func() bool {
		return mpe.EvaluationCount() == 2 && nextConsumer.SpanCount() == 0
	}, time.Second, 10*time.Millisecond)

	// Late spans should use cached non-sampled decision and skip re-evaluation.
	mpe.SetDecision(samplingpolicy.Sampled)
	require.NoError(t, p.ConsumeTraces(t.Context(), singleSpanTrace(traceID, uInt64ToSpanID(3), rootSpanID)))
	require.Eventually(t, func() bool {
		return mpe.EvaluationCount() == 2 && nextConsumer.SpanCount() == 0
	}, time.Second, 10*time.Millisecond)
}

func TestSpanIngestTickCleansUpPendingWithoutPolicyEvaluation(t *testing.T) {
	nextConsumer := new(consumertest.TracesSink)
	controller := newTestTSPController()

	mpe := &mockPolicyEvaluator{}
	policies := []*policy{
		{name: "mock-policy-1", evaluator: mpe, attribute: metric.WithAttributes(attribute.String("policy", "mock-policy-1"))},
	}

	cfg := Config{
		DecisionWait:     defaultTestDecisionWait,
		NumTraces:        defaultNumTraces,
		SamplingStrategy: samplingStrategySpanIngest,
		DecisionCache: DecisionCacheConfig{
			NonSampledCacheSize: 64,
		},
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

	// Non-terminal in span-ingest mode: pending in memory.
	mpe.SetDecision(samplingpolicy.NotSampled)
	require.NoError(t, p.ConsumeTraces(t.Context(), simpleTraces()))
	require.Eventually(t, func() bool { return mpe.EvaluationCount() == 1 }, time.Second, 10*time.Millisecond)
	require.Equal(t, 0, nextConsumer.SpanCount())

	// Tick path cleans pending traces without evaluating policies in span-ingest mode.
	controller.waitForTick()
	controller.waitForTick()
	require.Equal(t, 1, mpe.EvaluationCount())
	require.Equal(t, 0, nextConsumer.SpanCount())
	require.Eventually(t, func() bool {
		return len(p.(*tailSamplingSpanProcessor).idToTrace) == 0
	}, time.Second, 10*time.Millisecond)
}

func TestSpanIngestEvaluatesOnlyIncomingBatch(t *testing.T) {
	nextConsumer := new(consumertest.TracesSink)
	controller := newTestTSPController()
	eval := &batchSizeTrackingEvaluator{
		decisions: []samplingpolicy.Decision{samplingpolicy.NotSampled, samplingpolicy.Sampled},
	}

	policies := []*policy{
		{name: "tracking-policy", evaluator: eval, attribute: metric.WithAttributes(attribute.String("policy", "tracking-policy"))},
	}

	cfg := Config{
		DecisionWait:     defaultTestDecisionWait,
		NumTraces:        defaultNumTraces,
		SamplingStrategy: samplingStrategySpanIngest,
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

	traceID := uInt64ToTraceID(55)
	rootSpanID := uInt64ToSpanID(1)

	require.NoError(t, p.ConsumeTraces(t.Context(), singleSpanTrace(traceID, uInt64ToSpanID(2), rootSpanID)))
	require.NoError(t, p.ConsumeTraces(t.Context(), singleSpanTrace(traceID, rootSpanID, pcommon.SpanID{})))

	require.Eventually(t, func() bool {
		return eval.EvaluationCount() == 2
	}, time.Second, 10*time.Millisecond)
	require.Equal(t, []int{1, 1}, eval.EvaluationBatches())
	require.Eventually(t, func() bool {
		return nextConsumer.SpanCount() == 2
	}, time.Second, 10*time.Millisecond)
}

func TestSpanIngestRootFirstThenChild(t *testing.T) {
	nextConsumer := new(consumertest.TracesSink)
	controller := newTestTSPController()

	mpe := &mockPolicyEvaluator{}
	policies := []*policy{
		{name: "mock-policy-1", evaluator: mpe, attribute: metric.WithAttributes(attribute.String("policy", "mock-policy-1"))},
	}

	cfg := Config{
		DecisionWait:     defaultTestDecisionWait,
		NumTraces:        defaultNumTraces,
		SamplingStrategy: samplingStrategySpanIngest,
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

	traceID := uInt64ToTraceID(60)
	rootSpanID := uInt64ToSpanID(1)

	// Root first, non-terminal.
	mpe.SetDecision(samplingpolicy.NotSampled)
	require.NoError(t, p.ConsumeTraces(t.Context(), singleSpanTrace(traceID, rootSpanID, pcommon.SpanID{})))
	require.Eventually(t, func() bool { return mpe.EvaluationCount() == 1 }, time.Second, 10*time.Millisecond)
	require.Equal(t, 0, nextConsumer.SpanCount())

	// Child later, terminal sampled -> both spans released.
	mpe.SetDecision(samplingpolicy.Sampled)
	require.NoError(t, p.ConsumeTraces(t.Context(), singleSpanTrace(traceID, uInt64ToSpanID(2), rootSpanID)))
	require.Eventually(t, func() bool {
		return mpe.EvaluationCount() == 2 && nextConsumer.SpanCount() == 2
	}, time.Second, 10*time.Millisecond)
}

func TestSpanIngestChildFirstThenRootPendingCleanup(t *testing.T) {
	nextConsumer := new(consumertest.TracesSink)
	controller := newTestTSPController()

	mpe := &mockPolicyEvaluator{}
	policies := []*policy{
		{name: "mock-policy-1", evaluator: mpe, attribute: metric.WithAttributes(attribute.String("policy", "mock-policy-1"))},
	}

	cfg := Config{
		DecisionWait:     defaultTestDecisionWait,
		NumTraces:        defaultNumTraces,
		SamplingStrategy: samplingStrategySpanIngest,
		DecisionCache: DecisionCacheConfig{
			NonSampledCacheSize: 64,
		},
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

	traceID := uInt64ToTraceID(61)
	rootSpanID := uInt64ToSpanID(1)

	// Child then root, both non-terminal.
	mpe.SetDecision(samplingpolicy.NotSampled)
	require.NoError(t, p.ConsumeTraces(t.Context(), singleSpanTrace(traceID, uInt64ToSpanID(2), rootSpanID)))
	require.Eventually(t, func() bool { return mpe.EvaluationCount() == 1 }, time.Second, 10*time.Millisecond)
	require.NoError(t, p.ConsumeTraces(t.Context(), singleSpanTrace(traceID, rootSpanID, pcommon.SpanID{})))
	require.Eventually(t, func() bool { return mpe.EvaluationCount() == 2 }, time.Second, 10*time.Millisecond)
	require.Equal(t, 0, nextConsumer.SpanCount())

	// Cleanup tick finalizes pending as not sampled without more evaluations.
	controller.waitForTick()
	controller.waitForTick()
	require.Equal(t, 2, mpe.EvaluationCount())
	require.Equal(t, 0, nextConsumer.SpanCount())
	require.Eventually(t, func() bool {
		return len(p.(*tailSamplingSpanProcessor).idToTrace) == 0
	}, time.Second, 10*time.Millisecond)
}

func TestRateLimiter(t *testing.T) {
	nextConsumer := new(consumertest.TracesSink)
	controller := newTestTSPController()

	cfg := Config{
		SamplingStrategy:   samplingStrategyTraceComplete,
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

func singleSpanTrace(traceID pcommon.TraceID, spanID, parentID pcommon.SpanID) ptrace.Traces {
	traces := ptrace.NewTraces()
	span := traces.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.SetTraceID(traceID)
	span.SetSpanID(spanID)
	if !parentID.IsEmpty() {
		span.SetParentSpanID(parentID)
	}
	return traces
}
