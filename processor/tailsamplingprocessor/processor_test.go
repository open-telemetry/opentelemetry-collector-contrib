// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tailsamplingprocessor

import (
	"context"
	"encoding/binary"
	"errors"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/timeutils"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/idbatcher"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/sampling"
)

const (
	defaultTestDecisionWait = 30 * time.Second
)

var testPolicy = []PolicyCfg{{sharedPolicyCfg: sharedPolicyCfg{Name: "test-policy", Type: AlwaysSample}}}
var testLatencyPolicy = []PolicyCfg{
	{
		sharedPolicyCfg: sharedPolicyCfg{
			Name:       "test-policy",
			Type:       Latency,
			LatencyCfg: LatencyCfg{ThresholdMs: 1},
		},
	},
}

type TestPolicyEvaluator struct {
	Started       chan struct{}
	CouldContinue chan struct{}
	pe            sampling.PolicyEvaluator
}

func (t *TestPolicyEvaluator) Evaluate(ctx context.Context, traceID pcommon.TraceID, trace *sampling.TraceData) (sampling.Decision, error) {
	close(t.Started)
	<-t.CouldContinue
	return t.pe.Evaluate(ctx, traceID, trace)
}

func TestSequentialTraceArrival(t *testing.T) {
	traceIds, batches := generateIdsAndBatches(128)
	cfg := Config{
		DecisionWait:            defaultTestDecisionWait,
		NumTraces:               uint64(2 * len(traceIds)),
		ExpectedNewTracesPerSec: 64,
		PolicyCfgs:              testPolicy,
	}

	sp, _ := newTracesProcessor(context.Background(), componenttest.NewNopTelemetrySettings(), consumertest.NewNop(), cfg)
	tsp := sp.(*tailSamplingSpanProcessor)
	tsp.tickerFrequency = 100 * time.Millisecond
	require.NoError(t, tsp.Start(context.Background(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(t, tsp.Shutdown(context.Background()))
	}()

	for _, batch := range batches {
		require.NoError(t, tsp.ConsumeTraces(context.Background(), batch))
	}

	for i := range traceIds {
		d, ok := tsp.idToTrace.Load(traceIds[i])
		require.True(t, ok, "Missing expected traceId")
		v := d.(*sampling.TraceData)
		require.Equal(t, int64(i+1), v.SpanCount.Load(), "Incorrect number of spans for entry %d", i)
	}
}

func TestConcurrentTraceArrival(t *testing.T) {
	traceIds, batches := generateIdsAndBatches(128)

	var wg sync.WaitGroup
	cfg := Config{
		DecisionWait:            defaultTestDecisionWait,
		NumTraces:               uint64(2 * len(traceIds)),
		ExpectedNewTracesPerSec: 64,
		PolicyCfgs:              testPolicy,
	}
	sp, _ := newTracesProcessor(context.Background(), componenttest.NewNopTelemetrySettings(), consumertest.NewNop(), cfg)
	tsp := sp.(*tailSamplingSpanProcessor)
	tsp.tickerFrequency = 100 * time.Millisecond
	require.NoError(t, tsp.Start(context.Background(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(t, tsp.Shutdown(context.Background()))
	}()

	for _, batch := range batches {
		// Add the same traceId twice.
		wg.Add(2)
		go func(td ptrace.Traces) {
			require.NoError(t, tsp.ConsumeTraces(context.Background(), td))
			wg.Done()
		}(batch)
		go func(td ptrace.Traces) {
			require.NoError(t, tsp.ConsumeTraces(context.Background(), td))
			wg.Done()
		}(batch)
	}

	wg.Wait()

	for i := range traceIds {
		d, ok := tsp.idToTrace.Load(traceIds[i])
		require.True(t, ok, "Missing expected traceId")
		v := d.(*sampling.TraceData)
		require.Equal(t, int64(i+1)*2, v.SpanCount.Load(), "Incorrect number of spans for entry %d", i)
	}
}

func TestConcurrentArrivalAndEvaluation(t *testing.T) {
	traceIds, batches := generateIdsAndBatches(1)
	evalStarted := make(chan struct{})
	continueEvaluation := make(chan struct{})

	var wg sync.WaitGroup
	cfg := Config{
		DecisionWait:            defaultTestDecisionWait,
		NumTraces:               uint64(2 * len(traceIds)),
		ExpectedNewTracesPerSec: 64,
		PolicyCfgs:              testLatencyPolicy,
	}
	sp, _ := newTracesProcessor(context.Background(), componenttest.NewNopTelemetrySettings(), consumertest.NewNop(), cfg)
	tsp := sp.(*tailSamplingSpanProcessor)
	tsp.tickerFrequency = 1 * time.Millisecond
	require.NoError(t, tsp.Start(context.Background(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(t, tsp.Shutdown(context.Background()))
	}()

	tpe := &TestPolicyEvaluator{
		Started:       evalStarted,
		CouldContinue: continueEvaluation,
		pe:            tsp.policies[0].evaluator,
	}
	tsp.policies[0].evaluator = tpe

	for _, batch := range batches {
		wg.Add(1)
		go func(td ptrace.Traces) {
			for i := 0; i < 10; i++ {
				require.NoError(t, tsp.ConsumeTraces(context.Background(), td))
			}
			<-evalStarted
			close(continueEvaluation)
			for i := 0; i < 10; i++ {
				require.NoError(t, tsp.ConsumeTraces(context.Background(), td))
			}
			wg.Done()
		}(batch)
	}

	wg.Wait()
}

func TestSequentialTraceMapSize(t *testing.T) {
	traceIds, batches := generateIdsAndBatches(210)
	const maxSize = 100
	cfg := Config{
		DecisionWait:            defaultTestDecisionWait,
		NumTraces:               uint64(maxSize),
		ExpectedNewTracesPerSec: 64,
		PolicyCfgs:              testPolicy,
	}
	sp, _ := newTracesProcessor(context.Background(), componenttest.NewNopTelemetrySettings(), consumertest.NewNop(), cfg)
	tsp := sp.(*tailSamplingSpanProcessor)
	tsp.tickerFrequency = 100 * time.Millisecond
	require.NoError(t, tsp.Start(context.Background(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(t, tsp.Shutdown(context.Background()))
	}()

	for _, batch := range batches {
		require.NoError(t, tsp.ConsumeTraces(context.Background(), batch))
	}

	// On sequential insertion it is possible to know exactly which traces should be still on the map.
	for i := 0; i < len(traceIds)-maxSize; i++ {
		_, ok := tsp.idToTrace.Load(traceIds[i])
		require.False(t, ok, "Found unexpected traceId[%d] still on map (id: %v)", i, traceIds[i])
	}
}

func TestConcurrentTraceMapSize(t *testing.T) {
	t.Skip("Flaky test, see https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/9126")
	_, batches := generateIdsAndBatches(210)
	const maxSize = 100
	var wg sync.WaitGroup
	cfg := Config{
		DecisionWait:            defaultTestDecisionWait,
		NumTraces:               uint64(maxSize),
		ExpectedNewTracesPerSec: 64,
		PolicyCfgs:              testPolicy,
	}
	sp, _ := newTracesProcessor(context.Background(), componenttest.NewNopTelemetrySettings(), consumertest.NewNop(), cfg)
	tsp := sp.(*tailSamplingSpanProcessor)
	tsp.tickerFrequency = 100 * time.Millisecond
	require.NoError(t, tsp.Start(context.Background(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(t, tsp.Shutdown(context.Background()))
	}()

	for _, batch := range batches {
		wg.Add(1)
		go func(td ptrace.Traces) {
			require.NoError(t, tsp.ConsumeTraces(context.Background(), td))
			wg.Done()
		}(batch)
	}

	wg.Wait()

	// Since we can't guarantee the order of insertion the only thing that can be checked is
	// if the number of traces on the map matches the expected value.
	cnt := 0
	tsp.idToTrace.Range(func(_ interface{}, _ interface{}) bool {
		cnt++
		return true
	})
	require.Equal(t, maxSize, cnt, "Incorrect traces count on idToTrace")
}

func TestSamplingPolicyTypicalPath(t *testing.T) {
	const maxSize = 100
	const decisionWaitSeconds = 5
	// For this test explicitly control the timer calls and batcher, and set a mock
	// sampling policy evaluator.
	msp := new(consumertest.TracesSink)
	mpe := &mockPolicyEvaluator{}
	mtt := &manualTTicker{}
	tsp := &tailSamplingSpanProcessor{
		ctx:             context.Background(),
		nextConsumer:    msp,
		maxNumTraces:    maxSize,
		logger:          zap.NewNop(),
		decisionBatcher: newSyncIDBatcher(decisionWaitSeconds),
		policies:        []*policy{{name: "mock-policy", evaluator: mpe, ctx: context.TODO()}},
		deleteChan:      make(chan pcommon.TraceID, maxSize),
		policyTicker:    mtt,
		tickerFrequency: 100 * time.Millisecond,
		numTracesOnMap:  &atomic.Uint64{},
	}
	require.NoError(t, tsp.Start(context.Background(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(t, tsp.Shutdown(context.Background()))
	}()

	_, batches := generateIdsAndBatches(210)
	currItem := 0
	numSpansPerBatchWindow := 10
	// First evaluations shouldn't have anything to evaluate, until decision wait time passed.
	for evalNum := 0; evalNum < decisionWaitSeconds; evalNum++ {
		for ; currItem < numSpansPerBatchWindow*(evalNum+1); currItem++ {
			require.NoError(t, tsp.ConsumeTraces(context.Background(), batches[currItem]))
			require.True(t, mtt.Started, "Time ticker was expected to have started")
		}
		tsp.samplingPolicyOnTick()
		require.False(
			t,
			msp.SpanCount() != 0 || mpe.EvaluationCount != 0,
			"policy for initial items was evaluated before decision wait period",
		)
	}

	// Now the first batch that waited the decision period.
	mpe.NextDecision = sampling.Sampled
	tsp.samplingPolicyOnTick()
	require.False(
		t,
		msp.SpanCount() == 0 || mpe.EvaluationCount == 0,
		"policy should have been evaluated totalspans == %d and evaluationcount == %d",
		msp.SpanCount(),
		mpe.EvaluationCount,
	)

	require.Equal(t, numSpansPerBatchWindow, msp.SpanCount(), "not all spans of first window were accounted for")

	// Late span of a sampled trace should be sent directly down the pipeline exporter
	require.NoError(t, tsp.ConsumeTraces(context.Background(), batches[0]))
	expectedNumWithLateSpan := numSpansPerBatchWindow + 1
	require.Equal(t, expectedNumWithLateSpan, msp.SpanCount(), "late span was not accounted for")
}

func TestSamplingPolicyInvertSampled(t *testing.T) {
	const maxSize = 100
	const decisionWaitSeconds = 5
	// For this test explicitly control the timer calls and batcher, and set a mock
	// sampling policy evaluator.
	msp := new(consumertest.TracesSink)
	mpe := &mockPolicyEvaluator{}
	mtt := &manualTTicker{}
	tsp := &tailSamplingSpanProcessor{
		ctx:             context.Background(),
		nextConsumer:    msp,
		maxNumTraces:    maxSize,
		logger:          zap.NewNop(),
		decisionBatcher: newSyncIDBatcher(decisionWaitSeconds),
		policies:        []*policy{{name: "mock-policy", evaluator: mpe, ctx: context.TODO()}},
		deleteChan:      make(chan pcommon.TraceID, maxSize),
		policyTicker:    mtt,
		tickerFrequency: 100 * time.Millisecond,
		numTracesOnMap:  &atomic.Uint64{},
	}
	require.NoError(t, tsp.Start(context.Background(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(t, tsp.Shutdown(context.Background()))
	}()

	_, batches := generateIdsAndBatches(210)
	currItem := 0
	numSpansPerBatchWindow := 10
	// First evaluations shouldn't have anything to evaluate, until decision wait time passed.
	for evalNum := 0; evalNum < decisionWaitSeconds; evalNum++ {
		for ; currItem < numSpansPerBatchWindow*(evalNum+1); currItem++ {
			require.NoError(t, tsp.ConsumeTraces(context.Background(), batches[currItem]))
			require.True(t, mtt.Started, "Time ticker was expected to have started")
		}
		tsp.samplingPolicyOnTick()
		require.False(
			t,
			msp.SpanCount() != 0 || mpe.EvaluationCount != 0,
			"policy for initial items was evaluated before decision wait period",
		)
	}

	// Now the first batch that waited the decision period.
	mpe.NextDecision = sampling.InvertSampled
	tsp.samplingPolicyOnTick()
	require.False(
		t,
		msp.SpanCount() == 0 || mpe.EvaluationCount == 0,
		"policy should have been evaluated totalspans == %d and evaluationcount == %d",
		msp.SpanCount(),
		mpe.EvaluationCount,
	)

	require.Equal(t, numSpansPerBatchWindow, msp.SpanCount(), "not all spans of first window were accounted for")

	// Late span of a sampled trace should be sent directly down the pipeline exporter
	require.NoError(t, tsp.ConsumeTraces(context.Background(), batches[0]))
	expectedNumWithLateSpan := numSpansPerBatchWindow + 1
	require.Equal(t, expectedNumWithLateSpan, msp.SpanCount(), "late span was not accounted for")
}

func TestSamplingMultiplePolicies(t *testing.T) {
	const maxSize = 100
	const decisionWaitSeconds = 5
	// For this test explicitly control the timer calls and batcher, and set a mock
	// sampling policy evaluator.
	msp := new(consumertest.TracesSink)
	mpe1 := &mockPolicyEvaluator{}
	mpe2 := &mockPolicyEvaluator{}
	mtt := &manualTTicker{}
	tsp := &tailSamplingSpanProcessor{
		ctx:             context.Background(),
		nextConsumer:    msp,
		maxNumTraces:    maxSize,
		logger:          zap.NewNop(),
		decisionBatcher: newSyncIDBatcher(decisionWaitSeconds),
		policies: []*policy{
			{
				name: "policy-1", evaluator: mpe1, ctx: context.TODO(),
			},
			{
				name: "policy-2", evaluator: mpe2, ctx: context.TODO(),
			}},
		deleteChan:      make(chan pcommon.TraceID, maxSize),
		policyTicker:    mtt,
		tickerFrequency: 100 * time.Millisecond,
		numTracesOnMap:  &atomic.Uint64{},
	}
	require.NoError(t, tsp.Start(context.Background(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(t, tsp.Shutdown(context.Background()))
	}()

	_, batches := generateIdsAndBatches(210)
	currItem := 0
	numSpansPerBatchWindow := 10
	// First evaluations shouldn't have anything to evaluate, until decision wait time passed.
	for evalNum := 0; evalNum < decisionWaitSeconds; evalNum++ {
		for ; currItem < numSpansPerBatchWindow*(evalNum+1); currItem++ {
			require.NoError(t, tsp.ConsumeTraces(context.Background(), batches[currItem]))
			require.True(t, mtt.Started, "Time ticker was expected to have started")
		}
		tsp.samplingPolicyOnTick()
		require.False(
			t,
			msp.SpanCount() != 0 || mpe1.EvaluationCount != 0 || mpe2.EvaluationCount != 0,
			"policy for initial items was evaluated before decision wait period",
		)
	}

	// Both policies will decide to sample
	mpe1.NextDecision = sampling.Sampled
	mpe2.NextDecision = sampling.Sampled
	tsp.samplingPolicyOnTick()
	require.False(
		t,
		msp.SpanCount() == 0 || mpe1.EvaluationCount == 0 || mpe2.EvaluationCount == 0,
		"policy should have been evaluated totalspans == %d and evaluationcount(1) == %d and evaluationcount(2) == %d",
		msp.SpanCount(),
		mpe1.EvaluationCount,
		mpe2.EvaluationCount,
	)

	require.Equal(t, numSpansPerBatchWindow, msp.SpanCount(), "nextConsumer should've been called with exactly 1 batch of spans")

	// Late span of a sampled trace should be sent directly down the pipeline exporter
	require.NoError(t, tsp.ConsumeTraces(context.Background(), batches[0]))
	expectedNumWithLateSpan := numSpansPerBatchWindow + 1
	require.Equal(t, expectedNumWithLateSpan, msp.SpanCount(), "late span was not accounted for")
}

func TestSamplingPolicyDecisionNotSampled(t *testing.T) {
	const maxSize = 100
	const decisionWaitSeconds = 5
	// For this test explicitly control the timer calls and batcher, and set a mock
	// sampling policy evaluator.
	msp := new(consumertest.TracesSink)
	mpe := &mockPolicyEvaluator{}
	mtt := &manualTTicker{}
	tsp := &tailSamplingSpanProcessor{
		ctx:             context.Background(),
		nextConsumer:    msp,
		maxNumTraces:    maxSize,
		logger:          zap.NewNop(),
		decisionBatcher: newSyncIDBatcher(decisionWaitSeconds),
		policies:        []*policy{{name: "mock-policy", evaluator: mpe, ctx: context.TODO()}},
		deleteChan:      make(chan pcommon.TraceID, maxSize),
		policyTicker:    mtt,
		tickerFrequency: 100 * time.Millisecond,
		numTracesOnMap:  &atomic.Uint64{},
	}
	require.NoError(t, tsp.Start(context.Background(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(t, tsp.Shutdown(context.Background()))
	}()

	_, batches := generateIdsAndBatches(210)
	currItem := 0
	numSpansPerBatchWindow := 10
	// First evaluations shouldn't have anything to evaluate, until decision wait time passed.
	for evalNum := 0; evalNum < decisionWaitSeconds; evalNum++ {
		for ; currItem < numSpansPerBatchWindow*(evalNum+1); currItem++ {
			require.NoError(t, tsp.ConsumeTraces(context.Background(), batches[currItem]))
			require.True(t, mtt.Started, "Time ticker was expected to have started")
		}
		tsp.samplingPolicyOnTick()
		require.False(
			t,
			msp.SpanCount() != 0 || mpe.EvaluationCount != 0,
			"policy for initial items was evaluated before decision wait period",
		)
	}

	// Now the first batch that waited the decision period.
	mpe.NextDecision = sampling.NotSampled
	tsp.samplingPolicyOnTick()
	require.EqualValues(t, 0, msp.SpanCount(), "exporter should have received zero spans")
	require.EqualValues(t, 4, mpe.EvaluationCount, "policy should have been evaluated 4 times")

	// Late span of a non-sampled trace should be ignored
	require.NoError(t, tsp.ConsumeTraces(context.Background(), batches[0]))
	require.Equal(t, 0, msp.SpanCount())

	mpe.NextDecision = sampling.Unspecified
	mpe.NextError = errors.New("mock policy error")
	tsp.samplingPolicyOnTick()
	require.EqualValues(t, 0, msp.SpanCount(), "exporter should have received zero spans")
	require.EqualValues(t, 6, mpe.EvaluationCount, "policy should have been evaluated 6 times")

	// Late span of a non-sampled trace should be ignored
	require.NoError(t, tsp.ConsumeTraces(context.Background(), batches[0]))
	require.Equal(t, 0, msp.SpanCount())
}

func TestSamplingPolicyDecisionInvertNotSampled(t *testing.T) {
	const maxSize = 100
	const decisionWaitSeconds = 5
	// For this test explicitly control the timer calls and batcher, and set a mock
	// sampling policy evaluator.
	msp := new(consumertest.TracesSink)
	mpe := &mockPolicyEvaluator{}
	mtt := &manualTTicker{}
	tsp := &tailSamplingSpanProcessor{
		ctx:             context.Background(),
		nextConsumer:    msp,
		maxNumTraces:    maxSize,
		logger:          zap.NewNop(),
		decisionBatcher: newSyncIDBatcher(decisionWaitSeconds),
		policies:        []*policy{{name: "mock-policy", evaluator: mpe, ctx: context.TODO()}},
		deleteChan:      make(chan pcommon.TraceID, maxSize),
		policyTicker:    mtt,
		tickerFrequency: 100 * time.Millisecond,
		numTracesOnMap:  &atomic.Uint64{},
	}
	require.NoError(t, tsp.Start(context.Background(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(t, tsp.Shutdown(context.Background()))
	}()

	_, batches := generateIdsAndBatches(210)
	currItem := 0
	numSpansPerBatchWindow := 10
	// First evaluations shouldn't have anything to evaluate, until decision wait time passed.
	for evalNum := 0; evalNum < decisionWaitSeconds; evalNum++ {
		for ; currItem < numSpansPerBatchWindow*(evalNum+1); currItem++ {
			require.NoError(t, tsp.ConsumeTraces(context.Background(), batches[currItem]))
			require.True(t, mtt.Started, "Time ticker was expected to have started")
		}
		tsp.samplingPolicyOnTick()
		require.False(
			t,
			msp.SpanCount() != 0 || mpe.EvaluationCount != 0,
			"policy for initial items was evaluated before decision wait period",
		)
	}

	// Now the first batch that waited the decision period.
	mpe.NextDecision = sampling.InvertNotSampled
	tsp.samplingPolicyOnTick()
	require.EqualValues(t, 0, msp.SpanCount(), "exporter should have received zero spans")
	require.EqualValues(t, 4, mpe.EvaluationCount, "policy should have been evaluated 4 times")

	// Late span of a non-sampled trace should be ignored
	require.NoError(t, tsp.ConsumeTraces(context.Background(), batches[0]))
	require.Equal(t, 0, msp.SpanCount())

	mpe.NextDecision = sampling.Unspecified
	mpe.NextError = errors.New("mock policy error")
	tsp.samplingPolicyOnTick()
	require.EqualValues(t, 0, msp.SpanCount(), "exporter should have received zero spans")
	require.EqualValues(t, 6, mpe.EvaluationCount, "policy should have been evaluated 6 times")

	// Late span of a non-sampled trace should be ignored
	require.NoError(t, tsp.ConsumeTraces(context.Background(), batches[0]))
	require.Equal(t, 0, msp.SpanCount())
}

func TestLateArrivingSpansAssignedOriginalDecision(t *testing.T) {
	const maxSize = 100
	nextConsumer := new(consumertest.TracesSink)
	mpe1 := &mockPolicyEvaluator{}
	mpe2 := &mockPolicyEvaluator{}
	tsp := &tailSamplingSpanProcessor{
		ctx:             context.Background(),
		nextConsumer:    nextConsumer,
		maxNumTraces:    maxSize,
		logger:          zap.NewNop(),
		decisionBatcher: newSyncIDBatcher(1),
		policies: []*policy{
			{name: "mock-policy-1", evaluator: mpe1, ctx: context.TODO()},
			{name: "mock-policy-2", evaluator: mpe2, ctx: context.TODO()},
		},
		deleteChan:      make(chan pcommon.TraceID, maxSize),
		policyTicker:    &manualTTicker{},
		tickerFrequency: 100 * time.Millisecond,
		numTracesOnMap:  &atomic.Uint64{},
	}
	require.NoError(t, tsp.Start(context.Background(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(t, tsp.Shutdown(context.Background()))
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
	require.NoError(t, tsp.ConsumeTraces(context.Background(), spanIndexToTraces(1)))

	// The first tick won't do anything
	tsp.samplingPolicyOnTick()
	require.EqualValues(t, 0, mpe1.EvaluationCount)
	require.EqualValues(t, 0, mpe2.EvaluationCount)

	// This will cause policy evaluations on the first span
	tsp.samplingPolicyOnTick()

	// Both policies should have been evaluated once
	require.EqualValues(t, 1, mpe1.EvaluationCount)
	require.EqualValues(t, 1, mpe2.EvaluationCount)

	// The final decision SHOULD be NotSampled.
	require.EqualValues(t, 0, nextConsumer.SpanCount())

	// Generate and deliver final span for the trace which SHOULD get the same sampling decision as the first span.
	// The policies should NOT be evaluated again.
	require.NoError(t, tsp.ConsumeTraces(context.Background(), spanIndexToTraces(2)))
	require.EqualValues(t, 1, mpe1.EvaluationCount)
	require.EqualValues(t, 1, mpe2.EvaluationCount)
	require.EqualValues(t, 0, nextConsumer.SpanCount(), "original final decision not honored")
}

func TestMultipleBatchesAreCombinedIntoOne(t *testing.T) {
	const maxSize = 100
	const decisionWaitSeconds = 1
	// For this test explicitly control the timer calls and batcher, and set a mock
	// sampling policy evaluator.
	msp := new(consumertest.TracesSink)
	mpe := &mockPolicyEvaluator{}
	mtt := &manualTTicker{}
	tsp := &tailSamplingSpanProcessor{
		ctx:             context.Background(),
		nextConsumer:    msp,
		maxNumTraces:    maxSize,
		logger:          zap.NewNop(),
		decisionBatcher: newSyncIDBatcher(decisionWaitSeconds),
		policies:        []*policy{{name: "mock-policy", evaluator: mpe, ctx: context.TODO()}},
		deleteChan:      make(chan pcommon.TraceID, maxSize),
		policyTicker:    mtt,
		tickerFrequency: 100 * time.Millisecond,
		numTracesOnMap:  &atomic.Uint64{},
	}
	require.NoError(t, tsp.Start(context.Background(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(t, tsp.Shutdown(context.Background()))
	}()

	mpe.NextDecision = sampling.Sampled

	traceIds, batches := generateIdsAndBatches(3)
	for _, batch := range batches {
		require.NoError(t, tsp.ConsumeTraces(context.Background(), batch))
	}

	tsp.samplingPolicyOnTick()
	tsp.samplingPolicyOnTick()

	require.EqualValues(t, 3, len(msp.AllTraces()), "There should be three batches, one for each trace")

	expectedSpanIds := make(map[int][]pcommon.SpanID)
	expectedSpanIds[0] = []pcommon.SpanID{
		uInt64ToSpanID(uint64(1)),
	}
	expectedSpanIds[1] = []pcommon.SpanID{
		uInt64ToSpanID(uint64(2)),
		uInt64ToSpanID(uint64(3)),
	}
	expectedSpanIds[2] = []pcommon.SpanID{
		uInt64ToSpanID(uint64(4)),
		uInt64ToSpanID(uint64(5)),
		uInt64ToSpanID(uint64(6)),
	}

	receivedTraces := msp.AllTraces()
	for i, traceID := range traceIds {
		trace := findTrace(t, receivedTraces, traceID)
		require.EqualValues(t, i+1, trace.SpanCount(), "The trace should have all of its spans in a single batch")

		expected := expectedSpanIds[i]
		got := collectSpanIds(trace)

		// might have received out of order, sort for comparison
		sort.Slice(got, func(i, j int) bool {
			bytesA := got[i]
			a := binary.BigEndian.Uint64(bytesA[:])
			bytesB := got[j]
			b := binary.BigEndian.Uint64(bytesB[:])
			return a < b
		})

		require.EqualValues(t, expected, got)
	}
}

func collectSpanIds(trace ptrace.Traces) []pcommon.SpanID {
	var spanIDs []pcommon.SpanID

	for i := 0; i < trace.ResourceSpans().Len(); i++ {
		ilss := trace.ResourceSpans().At(i).ScopeSpans()

		for j := 0; j < ilss.Len(); j++ {
			ils := ilss.At(j)

			for k := 0; k < ils.Spans().Len(); k++ {
				span := ils.Spans().At(k)
				spanIDs = append(spanIDs, span.SpanID())
			}
		}
	}

	return spanIDs
}

func findTrace(t *testing.T, a []ptrace.Traces, traceID pcommon.TraceID) ptrace.Traces {
	for _, batch := range a {
		id := batch.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).TraceID()
		if traceID == id {
			return batch
		}
	}
	t.Fatalf("Trace was not received. TraceId %s", traceID)
	return ptrace.Traces{}
}

func generateIdsAndBatches(numIds int) ([]pcommon.TraceID, []ptrace.Traces) {
	traceIds := make([]pcommon.TraceID, numIds)
	spanID := 0
	var tds []ptrace.Traces
	for i := 0; i < numIds; i++ {
		traceIds[i] = uInt64ToTraceID(uint64(i))
		// Send each span in a separate batch
		for j := 0; j <= i; j++ {
			td := simpleTraces()
			span := td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
			span.SetTraceID(traceIds[i])

			spanID++
			span.SetSpanID(uInt64ToSpanID(uint64(spanID)))
			tds = append(tds, td)
		}
	}

	return traceIds, tds
}

func uInt64ToTraceID(id uint64) pcommon.TraceID {
	traceID := [16]byte{}
	binary.BigEndian.PutUint64(traceID[:8], id)
	binary.BigEndian.PutUint64(traceID[8:], id+1)
	return pcommon.TraceID(traceID)
}

// uInt64ToSpanID converts the uint64 representation of a SpanID to pcommon.SpanID.
func uInt64ToSpanID(id uint64) pcommon.SpanID {
	spanID := [8]byte{}
	binary.BigEndian.PutUint64(spanID[:], id)
	return pcommon.SpanID(spanID)
}

type mockPolicyEvaluator struct {
	NextDecision    sampling.Decision
	NextError       error
	EvaluationCount int
}

var _ sampling.PolicyEvaluator = (*mockPolicyEvaluator)(nil)

func (m *mockPolicyEvaluator) Evaluate(context.Context, pcommon.TraceID, *sampling.TraceData) (sampling.Decision, error) {
	m.EvaluationCount++
	return m.NextDecision, m.NextError
}

type manualTTicker struct {
	Started bool
}

var _ timeutils.TTicker = (*manualTTicker)(nil)

func (t *manualTTicker) Start(time.Duration) {
	t.Started = true
}

func (t *manualTTicker) OnTick() {
}

func (t *manualTTicker) Stop() {
}

type syncIDBatcher struct {
	sync.Mutex
	openBatch idbatcher.Batch
	batchPipe chan idbatcher.Batch
}

var _ idbatcher.Batcher = (*syncIDBatcher)(nil)

func newSyncIDBatcher(numBatches uint64) idbatcher.Batcher {
	batches := make(chan idbatcher.Batch, numBatches)
	for i := uint64(0); i < numBatches; i++ {
		batches <- nil
	}
	return &syncIDBatcher{
		batchPipe: batches,
	}
}

func (s *syncIDBatcher) AddToCurrentBatch(id pcommon.TraceID) {
	s.Lock()
	s.openBatch = append(s.openBatch, id)
	s.Unlock()
}

func (s *syncIDBatcher) CloseCurrentAndTakeFirstBatch() (idbatcher.Batch, bool) {
	s.Lock()
	defer s.Unlock()
	firstBatch := <-s.batchPipe
	s.batchPipe <- s.openBatch
	s.openBatch = nil
	return firstBatch, true
}

func (s *syncIDBatcher) Stop() {
}

func simpleTraces() ptrace.Traces {
	return simpleTracesWithID(pcommon.TraceID([16]byte{1, 2, 3, 4}))
}

func simpleTracesWithID(traceID pcommon.TraceID) ptrace.Traces {
	traces := ptrace.NewTraces()
	traces.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty().SetTraceID(traceID)
	return traces
}
