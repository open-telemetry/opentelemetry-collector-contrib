// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tailsamplingprocessor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/processor/processortest"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/sampling"
)

func TestSkipPolicyIsFirstInPolicyList(t *testing.T) {
	idb := newSyncIDBatcher()
	msp := new(consumertest.TracesSink)

	cfg := Config{
		DecisionWait: defaultTestDecisionWait,
		NumTraces:    defaultNumTraces,
		PolicyCfgs: []PolicyCfg{
			{
				sharedPolicyCfg: sharedPolicyCfg{
					Name: "regular-policy",
					Type: AlwaysSample,
				},
			},
			{
				sharedPolicyCfg: sharedPolicyCfg{
					Name: "skip-policy",
					Type: Skip,
				},
			},
		},
		Options: []Option{
			withDecisionBatcher(idb),
		},
	}
	p, err := newTracesProcessor(t.Context(), processortest.NewNopSettings(metadata.Type), msp, cfg)
	require.NoError(t, err)

	tsp := p.(*tailSamplingSpanProcessor)
	require.GreaterOrEqual(t, len(tsp.policies), 2)
	assert.Equal(t, "skip-policy", tsp.policies[0].name)
}

func TestSkipPolicyEvaluatorSkippedDecision(t *testing.T) {
	nextConsumer := new(consumertest.TracesSink)
	idb := newSyncIDBatcher()

	mpe1 := &mockPolicyEvaluator{}

	policies := []*policy{
		{name: "mock-skip-policy", evaluator: mpe1, attribute: metric.WithAttributes(attribute.String("policy", "mock-skip-policy"))},
	}

	cfg := Config{
		DecisionWait: defaultTestDecisionWait,
		NumTraces:    defaultNumTraces,
		Options: []Option{
			withDecisionBatcher(idb),
			withPolicies(policies),
		},
	}
	p, err := newTracesProcessor(t.Context(), processortest.NewNopSettings(metadata.Type), nextConsumer, cfg)
	require.NoError(t, err)

	require.NoError(t, p.Start(t.Context(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(t, p.Shutdown(t.Context()))
	}()

	mpe1.NextDecision = sampling.Skipped

	// Generate and deliver first span
	require.NoError(t, p.ConsumeTraces(t.Context(), simpleTraces()))

	tsp := p.(*tailSamplingSpanProcessor)

	// The first tick won't do anything
	tsp.policyTicker.OnTick()
	require.Equal(t, 0, mpe1.EvaluationCount)

	// This will cause policy evaluation on the first span
	tsp.policyTicker.OnTick()

	require.Equal(t, 1, mpe1.EvaluationCount)

	// The final decision should be NotSampled (default when all policies are skipped)
	require.Equal(t, 0, nextConsumer.SpanCount())
}

func TestSkipPolicyEvaluatorContinuedDecision(t *testing.T) {
	nextConsumer := new(consumertest.TracesSink)
	idb := newSyncIDBatcher()

	mpe1 := &mockPolicyEvaluator{}

	policies := []*policy{
		{name: "mock-skip-policy", evaluator: mpe1, attribute: metric.WithAttributes(attribute.String("policy", "mock-skip-policy"))},
	}

	cfg := Config{
		DecisionWait: defaultTestDecisionWait,
		NumTraces:    defaultNumTraces,
		Options: []Option{
			withDecisionBatcher(idb),
			withPolicies(policies),
		},
	}
	p, err := newTracesProcessor(t.Context(), processortest.NewNopSettings(metadata.Type), nextConsumer, cfg)
	require.NoError(t, err)

	require.NoError(t, p.Start(t.Context(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(t, p.Shutdown(t.Context()))
	}()

	mpe1.NextDecision = sampling.Continued

	// Generate and deliver first span
	require.NoError(t, p.ConsumeTraces(t.Context(), simpleTraces()))

	tsp := p.(*tailSamplingSpanProcessor)

	// The first tick won't do anything
	tsp.policyTicker.OnTick()
	require.Equal(t, 0, mpe1.EvaluationCount)

	// This will cause policy evaluation on the first span
	tsp.policyTicker.OnTick()

	require.Equal(t, 1, mpe1.EvaluationCount)

	// The final decision should be NotSampled (default when all policies return Continued)
	require.Equal(t, 0, nextConsumer.SpanCount())
}

func TestSkipPolicyWithSubsequentPolicies(t *testing.T) {
	nextConsumer := new(consumertest.TracesSink)
	idb := newSyncIDBatcher()

	mpe1 := &mockPolicyEvaluator{} // Skip policy
	mpe2 := &mockPolicyEvaluator{} // Regular policy

	policies := []*policy{
		{name: "mock-skip-policy", evaluator: mpe1, attribute: metric.WithAttributes(attribute.String("policy", "mock-skip-policy"))},
		{name: "mock-regular-policy", evaluator: mpe2, attribute: metric.WithAttributes(attribute.String("policy", "mock-regular-policy"))},
	}

	cfg := Config{
		DecisionWait: defaultTestDecisionWait,
		NumTraces:    defaultNumTraces,
		Options: []Option{
			withDecisionBatcher(idb),
			withPolicies(policies),
		},
	}
	p, err := newTracesProcessor(t.Context(), processortest.NewNopSettings(metadata.Type), nextConsumer, cfg)
	require.NoError(t, err)

	require.NoError(t, p.Start(t.Context(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(t, p.Shutdown(t.Context()))
	}()

	// Skip policy returns Skipped, regular policy returns Sampled
	mpe1.NextDecision = sampling.Skipped
	mpe2.NextDecision = sampling.Sampled

	// Generate and deliver first span
	require.NoError(t, p.ConsumeTraces(t.Context(), simpleTraces()))

	tsp := p.(*tailSamplingSpanProcessor)

	// The first tick won't do anything
	tsp.policyTicker.OnTick()
	require.Equal(t, 0, mpe1.EvaluationCount)
	require.Equal(t, 0, mpe2.EvaluationCount)

	// This will cause policy evaluations on the first span
	tsp.policyTicker.OnTick()

	require.Equal(t, 1, mpe1.EvaluationCount)
	require.Equal(t, 1, mpe2.EvaluationCount) // Should be evaluated despite first policy being skipped

	// The final decision should be Sampled due to the regular policy
	require.Equal(t, 1, nextConsumer.SpanCount())
}

func TestSkipPolicyWithContinuedAndSampledPolicies(t *testing.T) {
	nextConsumer := new(consumertest.TracesSink)
	idb := newSyncIDBatcher()

	mpe1 := &mockPolicyEvaluator{} // Skip policy returning Continued
	mpe2 := &mockPolicyEvaluator{} // Regular policy returning Sampled

	policies := []*policy{
		{name: "mock-skip-policy", evaluator: mpe1, attribute: metric.WithAttributes(attribute.String("policy", "mock-skip-policy"))},
		{name: "mock-regular-policy", evaluator: mpe2, attribute: metric.WithAttributes(attribute.String("policy", "mock-regular-policy"))},
	}

	cfg := Config{
		DecisionWait: defaultTestDecisionWait,
		NumTraces:    defaultNumTraces,
		Options: []Option{
			withDecisionBatcher(idb),
			withPolicies(policies),
		},
	}
	p, err := newTracesProcessor(t.Context(), processortest.NewNopSettings(metadata.Type), nextConsumer, cfg)
	require.NoError(t, err)

	require.NoError(t, p.Start(t.Context(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(t, p.Shutdown(t.Context()))
	}()

	// Skip policy returns Continued, regular policy returns Sampled
	mpe1.NextDecision = sampling.Continued
	mpe2.NextDecision = sampling.Sampled

	// Generate and deliver first span
	require.NoError(t, p.ConsumeTraces(t.Context(), simpleTraces()))

	tsp := p.(*tailSamplingSpanProcessor)

	// The first tick won't do anything
	tsp.policyTicker.OnTick()
	require.Equal(t, 0, mpe1.EvaluationCount)
	require.Equal(t, 0, mpe2.EvaluationCount)

	// This will cause policy evaluations on the first span
	tsp.policyTicker.OnTick()

	require.Equal(t, 1, mpe1.EvaluationCount)
	require.Equal(t, 1, mpe2.EvaluationCount)

	// The final decision should be Sampled due to the regular policy
	require.Equal(t, 1, nextConsumer.SpanCount())
}

func TestSkipPolicyWithDroppedDecision(t *testing.T) {
	nextConsumer := new(consumertest.TracesSink)
	idb := newSyncIDBatcher()

	mpe1 := &mockPolicyEvaluator{} // Skip policy
	mpe2 := &mockPolicyEvaluator{} // Drop policy

	policies := []*policy{
		{name: "mock-skip-policy", evaluator: mpe1, attribute: metric.WithAttributes(attribute.String("policy", "mock-skip-policy"))},
		{name: "mock-drop-policy", evaluator: mpe2, attribute: metric.WithAttributes(attribute.String("policy", "mock-drop-policy"))},
	}

	cfg := Config{
		DecisionWait: defaultTestDecisionWait,
		NumTraces:    defaultNumTraces,
		Options: []Option{
			withDecisionBatcher(idb),
			withPolicies(policies),
		},
	}
	p, err := newTracesProcessor(t.Context(), processortest.NewNopSettings(metadata.Type), nextConsumer, cfg)
	require.NoError(t, err)

	require.NoError(t, p.Start(t.Context(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(t, p.Shutdown(t.Context()))
	}()

	// Skip policy returns Skipped, drop policy returns Dropped
	mpe1.NextDecision = sampling.Skipped
	mpe2.NextDecision = sampling.Dropped

	// Generate and deliver first span
	require.NoError(t, p.ConsumeTraces(t.Context(), simpleTraces()))

	tsp := p.(*tailSamplingSpanProcessor)

	// The first tick won't do anything
	tsp.policyTicker.OnTick()
	require.Equal(t, 0, mpe1.EvaluationCount)
	require.Equal(t, 0, mpe2.EvaluationCount)

	// This will cause policy evaluations on the first span
	tsp.policyTicker.OnTick()

	require.Equal(t, 1, mpe1.EvaluationCount)
	require.Equal(t, 1, mpe2.EvaluationCount)

	// The final decision should be Dropped (Dropped takes precedence)
	require.Equal(t, 0, nextConsumer.SpanCount())
}

func TestMultipleSkipPoliciesAllSkipped(t *testing.T) {
	nextConsumer := new(consumertest.TracesSink)
	idb := newSyncIDBatcher()

	mpe1 := &mockPolicyEvaluator{} // Skip policy 1
	mpe2 := &mockPolicyEvaluator{} // Skip policy 2
	mpe3 := &mockPolicyEvaluator{} // Skip policy 3

	policies := []*policy{
		{name: "mock-skip-policy-1", evaluator: mpe1, attribute: metric.WithAttributes(attribute.String("policy", "mock-skip-policy-1"))},
		{name: "mock-skip-policy-2", evaluator: mpe2, attribute: metric.WithAttributes(attribute.String("policy", "mock-skip-policy-2"))},
		{name: "mock-skip-policy-3", evaluator: mpe3, attribute: metric.WithAttributes(attribute.String("policy", "mock-skip-policy-3"))},
	}

	cfg := Config{
		DecisionWait: defaultTestDecisionWait,
		NumTraces:    defaultNumTraces,
		Options: []Option{
			withDecisionBatcher(idb),
			withPolicies(policies),
		},
	}
	p, err := newTracesProcessor(t.Context(), processortest.NewNopSettings(metadata.Type), nextConsumer, cfg)
	require.NoError(t, err)

	require.NoError(t, p.Start(t.Context(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(t, p.Shutdown(t.Context()))
	}()

	// All policies return Skipped
	mpe1.NextDecision = sampling.Skipped
	mpe2.NextDecision = sampling.Skipped
	mpe3.NextDecision = sampling.Skipped

	// Generate and deliver first span
	require.NoError(t, p.ConsumeTraces(t.Context(), simpleTraces()))

	tsp := p.(*tailSamplingSpanProcessor)

	// The first tick won't do anything
	tsp.policyTicker.OnTick()
	require.Equal(t, 0, mpe1.EvaluationCount)
	require.Equal(t, 0, mpe2.EvaluationCount)
	require.Equal(t, 0, mpe3.EvaluationCount)

	// This will cause policy evaluations on the first span
	tsp.policyTicker.OnTick()

	require.Equal(t, 1, mpe1.EvaluationCount)
	require.Equal(t, 1, mpe2.EvaluationCount)
	require.Equal(t, 1, mpe3.EvaluationCount)

	// The final decision should be NotSampled (default when all policies are skipped)
	require.Equal(t, 0, nextConsumer.SpanCount())
}

func TestSkipPolicyMixedDecisions(t *testing.T) {
	nextConsumer := new(consumertest.TracesSink)
	idb := newSyncIDBatcher()

	mpe1 := &mockPolicyEvaluator{} // Returns Skipped
	mpe2 := &mockPolicyEvaluator{} // Returns Continued
	mpe3 := &mockPolicyEvaluator{} // Returns NotSampled

	policies := []*policy{
		{name: "mock-skip-policy", evaluator: mpe1, attribute: metric.WithAttributes(attribute.String("policy", "mock-skip-policy"))},
		{name: "mock-continue-policy", evaluator: mpe2, attribute: metric.WithAttributes(attribute.String("policy", "mock-continue-policy"))},
		{name: "mock-notsampled-policy", evaluator: mpe3, attribute: metric.WithAttributes(attribute.String("policy", "mock-notsampled-policy"))},
	}

	cfg := Config{
		DecisionWait: defaultTestDecisionWait,
		NumTraces:    defaultNumTraces,
		Options: []Option{
			withDecisionBatcher(idb),
			withPolicies(policies),
		},
	}
	p, err := newTracesProcessor(t.Context(), processortest.NewNopSettings(metadata.Type), nextConsumer, cfg)
	require.NoError(t, err)

	require.NoError(t, p.Start(t.Context(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(t, p.Shutdown(t.Context()))
	}()

	// Mixed decisions: Skipped, Continued, NotSampled
	mpe1.NextDecision = sampling.Skipped
	mpe2.NextDecision = sampling.Continued
	mpe3.NextDecision = sampling.NotSampled

	// Generate and deliver first span
	require.NoError(t, p.ConsumeTraces(t.Context(), simpleTraces()))

	tsp := p.(*tailSamplingSpanProcessor)

	// The first tick won't do anything
	tsp.policyTicker.OnTick()
	require.Equal(t, 0, mpe1.EvaluationCount)
	require.Equal(t, 0, mpe2.EvaluationCount)
	require.Equal(t, 0, mpe3.EvaluationCount)

	// This will cause policy evaluations on the first span
	tsp.policyTicker.OnTick()

	require.Equal(t, 1, mpe1.EvaluationCount)
	require.Equal(t, 1, mpe2.EvaluationCount)
	require.Equal(t, 1, mpe3.EvaluationCount)

	// The final decision should be NotSampled
	require.Equal(t, 0, nextConsumer.SpanCount())
}
