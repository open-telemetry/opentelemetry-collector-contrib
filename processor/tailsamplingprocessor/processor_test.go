// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tailsamplingprocessor

import (
	"context"
	"encoding/binary"
	"errors"
	"sort"
	"sync"
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
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/idbatcher"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/sampling"
)

const (
	defaultTestDecisionWait = 30 * time.Second
	defaultNumTraces        = 100
)

var (
	testPolicy        = []PolicyCfg{{sharedPolicyCfg: sharedPolicyCfg{Name: "test-policy", Type: AlwaysSample}}}
	testLatencyPolicy = []PolicyCfg{
		{
			sharedPolicyCfg: sharedPolicyCfg{
				Name:       "test-policy",
				Type:       Latency,
				LatencyCfg: LatencyCfg{ThresholdMs: 1},
			},
		},
	}
)

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

type spanInfo struct {
	span     ptrace.Span
	resource pcommon.Resource
	scope    pcommon.InstrumentationScope
}

func TestTraceIntegrity(t *testing.T) {
	const spanCount = 4
	// Generate trace with several spans with different scopes
	traces := ptrace.NewTraces()
	spans := make(map[pcommon.SpanID]spanInfo, 0)

	// Fill resource
	resourceSpans := traces.ResourceSpans().AppendEmpty()
	resource := resourceSpans.Resource()
	resourceSpans.Resource().Attributes().PutStr("key1", "value1")
	resourceSpans.Resource().Attributes().PutInt("key2", 0)

	// Fill scopeSpans 1
	scopeSpans := resourceSpans.ScopeSpans().AppendEmpty()
	scope := scopeSpans.Scope()
	scopeSpans.Scope().SetName("scope1")
	scopeSpans.Scope().Attributes().PutStr("key1", "value1")
	scopeSpans.Scope().Attributes().PutInt("key2", 0)

	// Add spans to scopeSpans 1
	span := scopeSpans.Spans().AppendEmpty()
	spanID := [8]byte{1, 2, 3, 4, 5, 6, 7, 8}
	span.SetSpanID(pcommon.SpanID(spanID))
	span.SetTraceID(pcommon.TraceID([16]byte{1, 2, 3, 4}))
	spans[spanID] = spanInfo{span: span, resource: resource, scope: scope}

	span = scopeSpans.Spans().AppendEmpty()
	spanID = [8]byte{9, 10, 11, 12, 13, 14, 15, 16}
	span.SetSpanID(pcommon.SpanID(spanID))
	span.SetTraceID(pcommon.TraceID([16]byte{5, 6, 7, 8}))
	spans[spanID] = spanInfo{span: span, resource: resource, scope: scope}

	// Fill scopeSpans 2
	scopeSpans = resourceSpans.ScopeSpans().AppendEmpty()
	scope = scopeSpans.Scope()
	scopeSpans.Scope().SetName("scope2")
	scopeSpans.Scope().Attributes().PutStr("key1", "value1")
	scopeSpans.Scope().Attributes().PutInt("key2", 0)

	// Add spans to scopeSpans 2
	span = scopeSpans.Spans().AppendEmpty()
	spanID = [8]byte{17, 18, 19, 20, 21, 22, 23, 24}
	span.SetSpanID(pcommon.SpanID(spanID))
	span.SetTraceID(pcommon.TraceID([16]byte{9, 10, 11, 12}))
	spans[spanID] = spanInfo{span: span, resource: resource, scope: scope}

	span = scopeSpans.Spans().AppendEmpty()
	spanID = [8]byte{25, 26, 27, 28, 29, 30, 31, 32}
	span.SetSpanID(pcommon.SpanID(spanID))
	span.SetTraceID(pcommon.TraceID([16]byte{13, 14, 15, 16}))
	spans[spanID] = spanInfo{span: span, resource: resource, scope: scope}

	require.Len(t, spans, spanCount)

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
	require.NoError(t, p.ConsumeTraces(context.Background(), traces))

	tsp := p.(*tailSamplingSpanProcessor)

	// The first tick won't do anything
	tsp.policyTicker.OnTick()
	require.Equal(t, 0, mpe1.EvaluationCount)

	// This will cause policy evaluations on the first span
	tsp.policyTicker.OnTick()

	// Both policies should have been evaluated once
	require.Equal(t, 4, mpe1.EvaluationCount)

	consumed := nextConsumer.AllTraces()
	require.Len(t, consumed, 4)
	for _, trace := range consumed {
		require.Equal(t, 1, trace.SpanCount())
		require.Equal(t, 1, trace.ResourceSpans().Len())
		require.Equal(t, 1, trace.ResourceSpans().At(0).ScopeSpans().Len())
		require.Equal(t, 1, trace.ResourceSpans().At(0).ScopeSpans().At(0).Spans().Len())

		span := trace.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
		if spanInfo, ok := spans[span.SpanID()]; ok {
			require.Equal(t, spanInfo.span, span)
			require.Equal(t, spanInfo.resource, trace.ResourceSpans().At(0).Resource())
			require.Equal(t, spanInfo.scope, trace.ResourceSpans().At(0).ScopeSpans().At(0).Scope())
		} else {
			require.Fail(t, "Span not found")
		}
	}
}

func TestSequentialTraceArrival(t *testing.T) {
	traceIDs, batches := generateIDsAndBatches(128)
	cfg := Config{
		DecisionWait:            defaultTestDecisionWait,
		NumTraces:               uint64(2 * len(traceIDs)),
		ExpectedNewTracesPerSec: 64,
		PolicyCfgs:              testPolicy,
		Options: []Option{
			withTickerFrequency(time.Millisecond),
		},
	}
	sp, err := newTracesProcessor(context.Background(), processortest.NewNopSettings(metadata.Type), consumertest.NewNop(), cfg)
	require.NoError(t, err)

	err = sp.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		err = sp.Shutdown(context.Background())
		require.NoError(t, err)
	}()

	for _, batch := range batches {
		require.NoError(t, sp.ConsumeTraces(context.Background(), batch))
	}

	tsp := sp.(*tailSamplingSpanProcessor)
	for i := range traceIDs {
		d, ok := tsp.idToTrace.Load(traceIDs[i])
		require.True(t, ok, "Missing expected traceId")
		v := d.(*sampling.TraceData)
		require.Equal(t, int64(i+1), v.SpanCount.Load(), "Incorrect number of spans for entry %d", i)
	}
}

func TestConcurrentTraceArrival(t *testing.T) {
	traceIDs, batches := generateIDsAndBatches(128)

	var wg sync.WaitGroup
	cfg := Config{
		DecisionWait:            defaultTestDecisionWait,
		NumTraces:               uint64(2 * len(traceIDs)),
		ExpectedNewTracesPerSec: 64,
		PolicyCfgs:              testPolicy,
		Options: []Option{
			withTickerFrequency(time.Millisecond),
		},
	}
	sp, err := newTracesProcessor(context.Background(), processortest.NewNopSettings(metadata.Type), consumertest.NewNop(), cfg)
	require.NoError(t, err)

	err = sp.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		err = sp.Shutdown(context.Background())
		require.NoError(t, err)
	}()

	// Limit the concurrency here to avoid creating too many goroutines and hit
	// https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/9126
	concurrencyLimiter := make(chan struct{}, 128)
	defer close(concurrencyLimiter)
	for _, batch := range batches {
		// Add the same traceId twice.
		wg.Add(2)
		concurrencyLimiter <- struct{}{}
		go func(td ptrace.Traces) {
			assert.NoError(t, sp.ConsumeTraces(context.Background(), td))
			wg.Done()
			<-concurrencyLimiter
		}(batch)
		concurrencyLimiter <- struct{}{}
		go func(td ptrace.Traces) {
			assert.NoError(t, sp.ConsumeTraces(context.Background(), td))
			wg.Done()
			<-concurrencyLimiter
		}(batch)
	}

	wg.Wait()

	tsp := sp.(*tailSamplingSpanProcessor)
	for i := range traceIDs {
		d, ok := tsp.idToTrace.Load(traceIDs[i])
		require.True(t, ok, "Missing expected traceId")
		v := d.(*sampling.TraceData)
		require.Equal(t, int64(i+1)*2, v.SpanCount.Load(), "Incorrect number of spans for entry %d", i)
	}
}

func TestConcurrentArrivalAndEvaluation(t *testing.T) {
	traceIDs, batches := generateIDsAndBatches(1)
	evalStarted := make(chan struct{})
	continueEvaluation := make(chan struct{})

	var wg sync.WaitGroup
	cfg := Config{
		DecisionWait:            defaultTestDecisionWait,
		NumTraces:               uint64(2 * len(traceIDs)),
		ExpectedNewTracesPerSec: 64,
		PolicyCfgs:              testLatencyPolicy,
		Options: []Option{
			withTickerFrequency(time.Millisecond),
		},
	}
	sp, err := newTracesProcessor(context.Background(), processortest.NewNopSettings(metadata.Type), consumertest.NewNop(), cfg)
	require.NoError(t, err)

	err = sp.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		err = sp.Shutdown(context.Background())
		require.NoError(t, err)
	}()

	tsp := sp.(*tailSamplingSpanProcessor)
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
				assert.NoError(t, tsp.ConsumeTraces(context.Background(), td))
			}
			<-evalStarted
			close(continueEvaluation)
			for i := 0; i < 10; i++ {
				assert.NoError(t, tsp.ConsumeTraces(context.Background(), td))
			}
			wg.Done()
		}(batch)
	}

	wg.Wait()
}

func TestSequentialTraceMapSize(t *testing.T) {
	traceIDs, batches := generateIDsAndBatches(210)
	cfg := Config{
		DecisionWait:            defaultTestDecisionWait,
		NumTraces:               defaultNumTraces,
		ExpectedNewTracesPerSec: 64,
		PolicyCfgs:              testPolicy,
		Options: []Option{
			withTickerFrequency(100 * time.Millisecond),
		},
	}
	sp, err := newTracesProcessor(context.Background(), processortest.NewNopSettings(metadata.Type), consumertest.NewNop(), cfg)
	require.NoError(t, err)

	err = sp.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		err = sp.Shutdown(context.Background())
		require.NoError(t, err)
	}()

	for _, batch := range batches {
		err = sp.ConsumeTraces(context.Background(), batch)
		require.NoError(t, err)
	}

	// On sequential insertion it is possible to know exactly which traces should be still on the map.
	tsp := sp.(*tailSamplingSpanProcessor)
	for i := 0; i < len(traceIDs)-int(cfg.NumTraces); i++ {
		_, ok := tsp.idToTrace.Load(traceIDs[i])
		require.False(t, ok, "Found unexpected traceId[%d] still on map (id: %v)", i, traceIDs[i])
	}
}

func TestConcurrentTraceMapSize(t *testing.T) {
	t.Skip("Flaky test, see https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/9126")
	_, batches := generateIDsAndBatches(210)
	const maxSize = 100
	var wg sync.WaitGroup
	cfg := Config{
		DecisionWait:            defaultTestDecisionWait,
		NumTraces:               uint64(maxSize),
		ExpectedNewTracesPerSec: 64,
		PolicyCfgs:              testPolicy,
		Options: []Option{
			withTickerFrequency(100 * time.Millisecond),
		},
	}
	sp, _ := newTracesProcessor(context.Background(), processortest.NewNopSettings(metadata.Type), consumertest.NewNop(), cfg)
	require.NoError(t, sp.Start(context.Background(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(t, sp.Shutdown(context.Background()))
	}()

	for _, batch := range batches {
		wg.Add(1)
		go func(td ptrace.Traces) {
			assert.NoError(t, sp.ConsumeTraces(context.Background(), td))
			wg.Done()
		}(batch)
	}

	wg.Wait()

	// Since we can't guarantee the order of insertion the only thing that can be checked is
	// if the number of traces on the map matches the expected value.
	cnt := 0
	tsp := sp.(*tailSamplingSpanProcessor)
	tsp.idToTrace.Range(func(_ any, _ any) bool {
		cnt++
		return true
	})
	require.Equal(t, maxSize, cnt, "Incorrect traces count on idToTrace")
}

func TestMultipleBatchesAreCombinedIntoOne(t *testing.T) {
	idb := newSyncIDBatcher()
	msp := new(consumertest.TracesSink)

	cfg := Config{
		DecisionWait: defaultTestDecisionWait,
		NumTraces:    defaultNumTraces,
		PolicyCfgs: []PolicyCfg{
			{
				sharedPolicyCfg: sharedPolicyCfg{
					Name: "always",
					Type: AlwaysSample,
				},
			},
		},
		Options: []Option{
			withDecisionBatcher(idb),
		},
	}
	p, err := newTracesProcessor(context.Background(), processortest.NewNopSettings(metadata.Type), msp, cfg)
	require.NoError(t, err)

	require.NoError(t, p.Start(context.Background(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(t, p.Shutdown(context.Background()))
	}()

	traceIDs, batches := generateIDsAndBatches(3)
	for _, batch := range batches {
		require.NoError(t, p.ConsumeTraces(context.Background(), batch))
	}

	tsp := p.(*tailSamplingSpanProcessor)
	tsp.policyTicker.OnTick() // the first tick always gets an empty batch
	tsp.policyTicker.OnTick()

	require.Len(t, msp.AllTraces(), 3, "There should be three batches, one for each trace")

	expectedSpanIDs := make(map[int][]pcommon.SpanID)
	expectedSpanIDs[0] = []pcommon.SpanID{
		uInt64ToSpanID(uint64(1)),
	}
	expectedSpanIDs[1] = []pcommon.SpanID{
		uInt64ToSpanID(uint64(2)),
		uInt64ToSpanID(uint64(3)),
	}
	expectedSpanIDs[2] = []pcommon.SpanID{
		uInt64ToSpanID(uint64(4)),
		uInt64ToSpanID(uint64(5)),
		uInt64ToSpanID(uint64(6)),
	}

	receivedTraces := msp.AllTraces()
	for i, traceID := range traceIDs {
		trace := findTrace(t, receivedTraces, traceID)
		require.Equal(t, i+1, trace.SpanCount(), "The trace should have all of its spans in a single batch")

		expected := expectedSpanIDs[i]
		got := collectSpanIDs(trace)

		// might have received out of order, sort for comparison
		sort.Slice(got, func(i, j int) bool {
			bytesA := got[i]
			a := binary.BigEndian.Uint64(bytesA[:])
			bytesB := got[j]
			b := binary.BigEndian.Uint64(bytesB[:])
			return a < b
		})

		require.Equal(t, expected, got)
	}
}

func TestSetSamplingPolicy(t *testing.T) {
	idb := newSyncIDBatcher()
	msp := new(consumertest.TracesSink)

	cfg := Config{
		DecisionWait: defaultTestDecisionWait,
		NumTraces:    defaultNumTraces,
		PolicyCfgs: []PolicyCfg{
			{
				sharedPolicyCfg: sharedPolicyCfg{
					Name: "always",
					Type: AlwaysSample,
				},
			},
		},
		Options: []Option{
			withDecisionBatcher(idb),
		},
	}
	p, err := newTracesProcessor(context.Background(), processortest.NewNopSettings(metadata.Type), msp, cfg)
	require.NoError(t, err)

	require.NoError(t, p.Start(context.Background(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(t, p.Shutdown(context.Background()))
	}()

	tsp := p.(*tailSamplingSpanProcessor)

	assert.Len(t, tsp.policies, 1)

	tsp.policyTicker.OnTick()

	assert.Len(t, tsp.policies, 1)

	cfgs := []PolicyCfg{
		{
			sharedPolicyCfg: sharedPolicyCfg{
				Name: "always",
				Type: AlwaysSample,
			},
		},
		{
			sharedPolicyCfg: sharedPolicyCfg{
				Name: "everything",
				Type: AlwaysSample,
			},
		},
	}
	tsp.SetSamplingPolicy(cfgs)

	assert.Len(t, tsp.policies, 1)

	tsp.policyTicker.OnTick()

	assert.Len(t, tsp.policies, 2)

	// Duplicate policy name.
	cfgs = []PolicyCfg{
		{
			sharedPolicyCfg: sharedPolicyCfg{
				Name: "always",
				Type: AlwaysSample,
			},
		},
		{
			sharedPolicyCfg: sharedPolicyCfg{
				Name: "everything",
				Type: AlwaysSample,
			},
		},
		{
			sharedPolicyCfg: sharedPolicyCfg{
				Name: "everything",
				Type: AlwaysSample,
			},
		},
	}
	tsp.SetSamplingPolicy(cfgs)

	assert.Len(t, tsp.policies, 2)

	tsp.policyTicker.OnTick()

	// Should revert sampling policy.
	assert.Len(t, tsp.policies, 2)
}

func TestSubSecondDecisionTime(t *testing.T) {
	// prepare
	msp := new(consumertest.TracesSink)
	tsp, err := newTracesProcessor(context.Background(), processortest.NewNopSettings(metadata.Type), msp, Config{
		DecisionWait: 500 * time.Millisecond,
		NumTraces:    defaultNumTraces,
		PolicyCfgs:   testPolicy,
	})
	require.NoError(t, err)

	// speed up the test a bit
	tsp.(*tailSamplingSpanProcessor).tickerFrequency = 10 * time.Millisecond

	require.NoError(t, tsp.Start(context.Background(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(t, tsp.Shutdown(context.Background()))
	}()

	// test
	require.NoError(t, tsp.ConsumeTraces(context.Background(), simpleTraces()))

	// verify
	require.Eventually(t, func() bool {
		return len(msp.AllTraces()) == 1
	}, time.Second, 10*time.Millisecond)
}

func TestPolicyLoggerAddsPolicyName(t *testing.T) {
	// prepare
	zc, logs := observer.New(zap.DebugLevel)
	logger := zap.New(zc)

	set := componenttest.NewNopTelemetrySettings()
	set.Logger = logger

	cfg := &sharedPolicyCfg{
		Type: AlwaysSample, // we test only one evaluator
	}

	evaluator, err := getSharedPolicyEvaluator(set, cfg)
	require.NoError(t, err)

	// test
	_, err = evaluator.Evaluate(context.Background(), pcommon.TraceID{}, nil)
	require.NoError(t, err)

	// verify
	assert.Len(t, logs.All(), 1)
	assert.Equal(t, AlwaysSample, logs.All()[0].ContextMap()["policy"])
}

func TestDuplicatePolicyName(t *testing.T) {
	// prepare
	msp := new(consumertest.TracesSink)

	alwaysSample := sharedPolicyCfg{
		Name: "always_sample",
		Type: AlwaysSample,
	}

	_, err := newTracesProcessor(context.Background(), processortest.NewNopSettings(metadata.Type), msp, Config{
		DecisionWait: defaultTestDecisionWait,
		NumTraces:    defaultNumTraces,
		PolicyCfgs: []PolicyCfg{
			{sharedPolicyCfg: alwaysSample},
			{sharedPolicyCfg: alwaysSample},
		},
	})

	// verify
	assert.Equal(t, err, errors.New(`duplicate policy name "always_sample"`))
}

func TestDecisionPolicyMetrics(t *testing.T) {
	traceIDs, batches := generateIDsAndBatches(10)
	policy := []PolicyCfg{
		{
			sharedPolicyCfg: sharedPolicyCfg{
				Name:             "test-policy",
				Type:             Probabilistic,
				ProbabilisticCfg: ProbabilisticCfg{SamplingPercentage: 50},
			},
		},
	}
	cfg := Config{
		DecisionWait:            defaultTestDecisionWait,
		NumTraces:               uint64(2 * len(traceIDs)),
		ExpectedNewTracesPerSec: 64,
		PolicyCfgs:              policy,
	}
	sp, _ := newTracesProcessor(context.Background(), processortest.NewNopSettings(metadata.Type), consumertest.NewNop(), cfg)
	tsp := sp.(*tailSamplingSpanProcessor)
	require.NoError(t, tsp.Start(context.Background(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(t, tsp.Shutdown(context.Background()))
	}()
	metrics := &policyMetrics{}

	for i, id := range traceIDs {
		sb := &sampling.TraceData{
			ArrivalTime:     time.Now(),
			ReceivedBatches: batches[i],
		}

		_ = tsp.makeDecision(id, sb, metrics)
	}

	assert.EqualValues(t, 5, metrics.decisionSampled)
	assert.EqualValues(t, 5, metrics.decisionNotSampled)
	assert.EqualValues(t, 0, metrics.idNotFoundOnMapCount)
	assert.EqualValues(t, 0, metrics.evaluateErrorCount)
}

func collectSpanIDs(trace ptrace.Traces) []pcommon.SpanID {
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

func generateIDsAndBatches(numIDs int) ([]pcommon.TraceID, []ptrace.Traces) {
	traceIDs := make([]pcommon.TraceID, numIDs)
	spanID := 0
	var tds []ptrace.Traces
	for i := 0; i < numIDs; i++ {
		traceIDs[i] = uInt64ToTraceID(uint64(i))
		// Send each span in a separate batch
		for j := 0; j <= i; j++ {
			td := simpleTraces()
			span := td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
			span.SetTraceID(traceIDs[i])

			spanID++
			span.SetSpanID(uInt64ToSpanID(uint64(spanID)))
			tds = append(tds, td)
		}
	}

	return traceIDs, tds
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

type syncIDBatcher struct {
	sync.Mutex
	openBatch idbatcher.Batch
	batchPipe chan idbatcher.Batch
}

var _ idbatcher.Batcher = (*syncIDBatcher)(nil)

func newSyncIDBatcher() idbatcher.Batcher {
	batches := make(chan idbatcher.Batch, 1)
	batches <- nil
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
