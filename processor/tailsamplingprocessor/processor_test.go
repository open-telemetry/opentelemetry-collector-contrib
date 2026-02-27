// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tailsamplingprocessor

import (
	"context"
	"encoding/binary"
	"errors"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor/processortest"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/metric/metricdata/metricdatatest"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/idbatcher"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/pkg/samplingpolicy"
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
	pe            samplingpolicy.Evaluator
}

func (t *TestPolicyEvaluator) Evaluate(ctx context.Context, traceID pcommon.TraceID, trace *samplingpolicy.TraceData) (samplingpolicy.Decision, error) {
	close(t.Started)
	<-t.CouldContinue
	return t.pe.Evaluate(ctx, traceID, trace)
}

// testTSPController is a set of mechanisms to make the TSP do predictable
// things in tests.
type testTSPController struct {
	tickChan chan chan struct{}
}

func newTestTSPController() *testTSPController {
	return &testTSPController{
		tickChan: make(chan chan struct{}),
	}
}

func (t *testTSPController) triggerTicks() chan struct{} {
	// We need a buffer so the ticker can signal completion without
	// blocking.
	tickDone := make(chan struct{}, 1)
	t.tickChan <- tickDone
	return tickDone
}

func (t *testTSPController) concurrentWithTick(f func()) {
	tickDone := t.triggerTicks()
	f()
	<-tickDone
}

func (t *testTSPController) waitForTick() {
	<-t.triggerTicks()
}

func withTestController(t *testTSPController) Option {
	return func(tsp *tailSamplingSpanProcessor) {
		tsp.tickChan = make(chan chan struct{})
		t.tickChan = tsp.tickChan

		// Use an unbuffered work channel so we know that when ConsumeTraces
		// completes the traces will have been ingested by the TSP.
		tsp.workChan = make(chan []traceBatch)

		// use a sync ID batcher to avoid waiting on lots of empty ticks.
		// We need to close the old one before creating a new one.
		tsp.decisionBatcher = newSyncIDBatcher()

		// Use a slow tick frequency to effectively disable automatic ticks.
		// We'll manually trigger ticks as needed via the tickChan.
		tsp.tickerFrequency = time.Hour
	}
}

// withTickerFrequency sets the frequency at which the processor will evaluate
// the sampling policies.
func withTickerFrequency(frequency time.Duration) Option {
	return func(tsp *tailSamplingSpanProcessor) {
		tsp.tickerFrequency = frequency
	}
}

// withPolicies sets the sampling policies to be used by the processor.
func withPolicies(policies []*policy) Option {
	return func(tsp *tailSamplingSpanProcessor) {
		tsp.policies = policies
	}
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

	mpe1 := &mockPolicyEvaluator{}

	policies := []*policy{
		{name: "mock-policy-1", evaluator: mpe1, attribute: metric.WithAttributes(attribute.String("policy", "mock-policy-1"))},
	}

	controller := newTestTSPController()
	cfg := Config{
		DecisionWait: defaultTestDecisionWait,
		NumTraces:    defaultNumTraces,
		Options: []Option{
			withPolicies(policies),
			withTestController(controller),
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
	require.NoError(t, p.ConsumeTraces(t.Context(), traces))

	// The first tick won't do anything
	controller.waitForTick()
	require.Equal(t, 0, mpe1.EvaluationCount)

	// This will cause policy evaluations on the first span
	controller.waitForTick()

	// Both policies should have been evaluated once
	assert.Equal(t, 4, mpe1.EvaluationCount)

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
	controller := newTestTSPController()
	cfg := Config{
		DecisionWait:            defaultTestDecisionWait,
		NumTraces:               uint64(2 * len(traceIDs)),
		ExpectedNewTracesPerSec: 64,
		PolicyCfgs:              testPolicy,
		Options: []Option{
			withTestController(controller),
		},
	}
	telem := setupTestTelemetry()
	telemetrySettings := telem.newSettings()
	nextConsumer := new(consumertest.TracesSink)
	sp, err := newTracesProcessor(t.Context(), telemetrySettings, nextConsumer, cfg)
	require.NoError(t, err)

	err = sp.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		err = sp.Shutdown(t.Context())
		require.NoError(t, err)
	}()

	for _, batch := range batches {
		require.NoError(t, sp.ConsumeTraces(t.Context(), batch))
	}

	// The first tick won't do anything
	controller.waitForTick()
	controller.waitForTick()

	allSampledTraces := nextConsumer.AllTraces()
	sampledTraceIDs := make(map[pcommon.TraceID]struct{})
	for _, trace := range allSampledTraces {
		sampledTraceIDs[trace.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).TraceID()] = struct{}{}
	}
	require.Len(t, sampledTraceIDs, 128)
	for _, expectedTrace := range traceIDs {
		_, ok := sampledTraceIDs[expectedTrace]
		require.True(t, ok, "Expected trace %v to be sampled", expectedTrace)
		delete(sampledTraceIDs, expectedTrace)
	}
	require.Empty(t, sampledTraceIDs, "No extra traces should be sampled")
}

func TestConcurrentTraceArrival(t *testing.T) {
	traceIDs, batches := generateIDsAndBatches(128)
	controller := newTestTSPController()
	var wg sync.WaitGroup
	cfg := Config{
		DecisionWait:            defaultTestDecisionWait,
		NumTraces:               uint64(2 * len(traceIDs)),
		ExpectedNewTracesPerSec: 64,
		PolicyCfgs:              testPolicy,
		Options: []Option{
			withTestController(controller),
		},
	}
	telem := setupTestTelemetry()
	telemetrySettings := telem.newSettings()
	nextConsumer := new(consumertest.TracesSink)
	sp, err := newTracesProcessor(t.Context(), telemetrySettings, nextConsumer, cfg)
	require.NoError(t, err)

	err = sp.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		err = sp.Shutdown(t.Context())
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
			assert.NoError(t, sp.ConsumeTraces(t.Context(), td))
			wg.Done()
			<-concurrencyLimiter
		}(batch)
		concurrencyLimiter <- struct{}{}
		go func(td ptrace.Traces) {
			assert.NoError(t, sp.ConsumeTraces(t.Context(), td))
			wg.Done()
			<-concurrencyLimiter
		}(batch)
	}

	wg.Wait()

	// The first tick won't do anything
	controller.waitForTick()
	controller.waitForTick()

	allSampledTraces := nextConsumer.AllTraces()
	sampledTraceIDs := make(map[pcommon.TraceID]struct{})
	for _, trace := range allSampledTraces {
		sampledTraceIDs[trace.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).TraceID()] = struct{}{}
	}
	require.Len(t, sampledTraceIDs, 128)
	for _, expectedTrace := range traceIDs {
		_, ok := sampledTraceIDs[expectedTrace]
		require.True(t, ok, "Expected trace %v to be sampled", expectedTrace)
		delete(sampledTraceIDs, expectedTrace)
	}
	require.Empty(t, sampledTraceIDs, "No extra traces should be sampled")
}

func TestConcurrentArrivalAndEvaluation(t *testing.T) {
	traceIDs, batches := generateIDsAndBatches(1)
	controller := newTestTSPController()

	var wg sync.WaitGroup
	cfg := Config{
		DecisionWait:            defaultTestDecisionWait,
		NumTraces:               uint64(2 * len(traceIDs)),
		ExpectedNewTracesPerSec: 64,
		PolicyCfgs:              testLatencyPolicy,
		Options: []Option{
			withTickerFrequency(time.Millisecond),
			withTestController(controller),
		},
	}
	sp, err := newTracesProcessor(t.Context(), processortest.NewNopSettings(metadata.Type), consumertest.NewNop(), cfg)
	require.NoError(t, err)

	err = sp.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		err = sp.Shutdown(t.Context())
		require.NoError(t, err)
	}()

	for _, batch := range batches {
		wg.Add(1)
		go func(td ptrace.Traces) {
			for range 10 {
				assert.NoError(t, sp.ConsumeTraces(t.Context(), td))
			}
			controller.concurrentWithTick(func() {
				for range 10 {
					assert.NoError(t, sp.ConsumeTraces(t.Context(), td))
				}
			})
			wg.Done()
		}(batch)
	}

	wg.Wait()
}

func TestSequentialTraceMapSize(t *testing.T) {
	controller := newTestTSPController()
	traceIDs, batches := generateIDsAndBatches(210)
	cfg := Config{
		DecisionWait:            defaultTestDecisionWait,
		NumTraces:               defaultNumTraces,
		ExpectedNewTracesPerSec: 64,
		PolicyCfgs:              testPolicy,
		Options: []Option{
			withTestController(controller),
		},
	}
	nextConsumer := new(consumertest.TracesSink)
	sp, err := newTracesProcessor(t.Context(), processortest.NewNopSettings(metadata.Type), nextConsumer, cfg)
	require.NoError(t, err)

	err = sp.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		err = sp.Shutdown(t.Context())
		require.NoError(t, err)
	}()

	for _, batch := range batches {
		err = sp.ConsumeTraces(t.Context(), batch)
		require.NoError(t, err)
	}

	// On sequential insertion it is possible to know exactly which traces
	// should be still on the map. We expect those to be sampled now.
	controller.waitForTick()
	controller.waitForTick()

	allSampledTraces := nextConsumer.AllTraces()
	sampledTraceIDs := make(map[pcommon.TraceID]struct{})
	for _, trace := range allSampledTraces {
		sampledTraceIDs[trace.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).TraceID()] = struct{}{}
	}

	require.Len(t, sampledTraceIDs, int(cfg.NumTraces))
	for _, expectedTrace := range traceIDs[len(traceIDs)-int(cfg.NumTraces):] {
		_, ok := sampledTraceIDs[expectedTrace]
		require.True(t, ok, "Expected trace %v to be sampled", expectedTrace)
		delete(sampledTraceIDs, expectedTrace)
	}

	require.Empty(t, sampledTraceIDs, "No extra traces should be sampled")
}

func TestConsumptionDuringPolicyEvaluation(t *testing.T) {
	// This test was added to reproduce a specific race condition:

	// Each G is a goroutine
	// G1: ConsumeTraces
	// G1: Cache.Get (miss)

	// G2: OnTick
	// G2: makeDecision
	// G2: Cache.Put
	// G2: idToTrace.Delete

	// G1: idToTrace.LoadOrStore
	// G1: AppendToCurrentBatch
	// < — At this point, we have a trace id which is in the batcher (G1 added it), the idToTrace map (G1 added it), and the decision cache (G2 added it).

	// G3: ConsumeTraces
	// G3: Cache.Get (hit)
	// G3: idToTrace.Delete
	// < — At this point, G3 has dropped the data added by G1, and orphaned a trace ID in the batcher.

	// G2: CloseCurrentAndTakeFirstBatch
	// G2: idToTrace.Load (miss) <- this is the droppedTooEarly signal

	// We need a lot of tries to make this happen reliably.
	numBatches := 100
	_, batches := generateIDsAndBatches(numBatches)
	// prepare
	msp := new(consumertest.TracesSink)
	cfg := Config{
		DecisionWait: 10 * time.Millisecond,
		// idToTrace map size is 2x the number of batches, to eliminate "expected"
		// dropped too early errors.
		NumTraces:  uint64(numBatches * 2),
		PolicyCfgs: testPolicy,
		DecisionCache: DecisionCacheConfig{
			// Cache large enough to hold all traces, to eliminate "expected"
			// dropped too early errors.
			SampledCacheSize: numBatches * 2,
		},
		Options: []Option{
			withTickerFrequency(5 * time.Millisecond),
		},
	}
	settings := processortest.NewNopSettings(metadata.Type)
	reader := sdkmetric.NewManualReader()
	settings.MeterProvider = sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	tsp, err := newTracesProcessor(t.Context(), settings, msp, cfg)
	require.NoError(t, err)

	require.NoError(t, tsp.Start(t.Context(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(t, tsp.Shutdown(t.Context()))
	}()

	var expectedSpans atomic.Int64
	wg := sync.WaitGroup{}
	var combinedErr error
	errCh := make(chan error, len(batches))
	errDone := make(chan struct{})

	go func() {
		for err := range errCh {
			combinedErr = errors.Join(combinedErr, err)
		}
		close(errDone)
	}()
	// For each batch, we consume the same trace repeatedly for at least 2x the decision wait time
	// this ensures that batches are being consumed concurrently with policy evaluation.
	for _, batch := range batches {
		wg.Go(func() {
			start := time.Now()
			// The important thing here is that we are writing as close as
			// possible to the moment when the policy is evaluated. We can't
			// know exactly when that will happen, so we just write in a loop
			// until the time must have passed.
			for time.Since(start) < 2*cfg.DecisionWait {
				expectedSpans.Add(int64(batch.SpanCount()))
				err := tsp.ConsumeTraces(t.Context(), batch)
				if err != nil {
					errCh <- err
				}
			}
		})
	}
	wg.Wait()
	close(errCh)
	<-errDone
	require.NoError(t, combinedErr)

	// verify
	// despite all the concurrency above, we should eventually sample all the spans.
	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		received := int64(msp.SpanCount())
		expected := expectedSpans.Load()
		missing := expected - received
		require.Equal(collect, expected, received, "expected %d spans, received %d, missing %d", expected, received, missing)
	}, 1*time.Second, 100*time.Millisecond)
}

func TestMultipleBatchesAreCombinedIntoOne(t *testing.T) {
	controller := newTestTSPController()
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
			withTestController(controller),
		},
	}
	p, err := newTracesProcessor(t.Context(), processortest.NewNopSettings(metadata.Type), msp, cfg)
	require.NoError(t, err)

	require.NoError(t, p.Start(t.Context(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(t, p.Shutdown(t.Context()))
	}()

	traceIDs, batches := generateIDsAndBatches(3)
	for _, batch := range batches {
		require.NoError(t, p.ConsumeTraces(t.Context(), batch))
	}

	controller.waitForTick() // the first tick always gets an empty batch
	controller.waitForTick()

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
	controller := newTestTSPController()
	msp := new(consumertest.TracesSink)
	telem := setupTestTelemetry()

	cfg := Config{
		DecisionWait: defaultTestDecisionWait,
		NumTraces:    defaultNumTraces,
		PolicyCfgs: []PolicyCfg{
			{
				sharedPolicyCfg: sharedPolicyCfg{
					Name: "only-metrics",
					Type: StringAttribute,
					StringAttributeCfg: StringAttributeCfg{
						Key:    "url.path",
						Values: []string{"/metrics"},
					},
				},
			},
		},
		Options: []Option{
			withTestController(controller),
		},
	}
	p, err := newTracesProcessor(t.Context(), telem.newSettings(), msp, cfg)
	require.NoError(t, err)

	require.NoError(t, p.Start(t.Context(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(t, p.Shutdown(t.Context()))
	}()

	// Send some metrics traces and confirm they are sampled
	metricsTrace := simpleTracesWithID(uInt64ToTraceID(1))
	metricsTrace.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("url.path", "/metrics")
	healthTrace := simpleTracesWithID(uInt64ToTraceID(2))
	healthTrace.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("url.path", "/health")

	require.NoError(t, p.ConsumeTraces(t.Context(), metricsTrace))
	require.NoError(t, p.ConsumeTraces(t.Context(), healthTrace))

	controller.waitForTick()
	controller.waitForTick()

	assert.Len(t, msp.AllTraces(), 1)
	assert.Equal(t, uInt64ToTraceID(1), msp.AllTraces()[0].ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).TraceID())

	msp.Reset()

	cfgs := []PolicyCfg{
		{
			sharedPolicyCfg: sharedPolicyCfg{
				Name: "only-health",
				Type: StringAttribute,
				StringAttributeCfg: StringAttributeCfg{
					Key:    "url.path",
					Values: []string{"/health"},
				},
			},
		},
	}
	p.(*tailSamplingSpanProcessor).SetSamplingPolicy(cfgs)

	controller.waitForTick()

	metricsTrace = simpleTracesWithID(uInt64ToTraceID(3))
	metricsTrace.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("url.path", "/metrics")
	healthTrace = simpleTracesWithID(uInt64ToTraceID(4))
	healthTrace.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("url.path", "/health")

	require.NoError(t, p.ConsumeTraces(t.Context(), metricsTrace))
	require.NoError(t, p.ConsumeTraces(t.Context(), healthTrace))

	controller.waitForTick()
	controller.waitForTick()

	assert.Len(t, msp.AllTraces(), 1)
	assert.Equal(t, uInt64ToTraceID(4), msp.AllTraces()[0].ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).TraceID())
}

func TestSubSecondDecisionTime(t *testing.T) {
	// prepare
	msp := new(consumertest.TracesSink)
	tsp, err := newTracesProcessor(t.Context(), processortest.NewNopSettings(metadata.Type), msp, Config{
		DecisionWait: 500 * time.Millisecond,
		NumTraces:    defaultNumTraces,
		PolicyCfgs:   testPolicy,
		Options: []Option{
			withTickerFrequency(10 * time.Millisecond),
		},
	})
	require.NoError(t, err)

	require.NoError(t, tsp.Start(t.Context(), componenttest.NewNopHost()))
	defer func() {
		require.NoError(t, tsp.Shutdown(t.Context()))
	}()

	// test
	require.NoError(t, tsp.ConsumeTraces(t.Context(), simpleTraces()))

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

	evaluator, err := getSharedPolicyEvaluator(set, cfg, nil)
	require.NoError(t, err)

	// test
	_, err = evaluator.Evaluate(t.Context(), pcommon.TraceID{}, nil)
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

	p, err := newTracesProcessor(t.Context(), processortest.NewNopSettings(metadata.Type), msp, Config{
		DecisionWait: defaultTestDecisionWait,
		NumTraces:    defaultNumTraces,
		PolicyCfgs: []PolicyCfg{
			{sharedPolicyCfg: alwaysSample},
			{sharedPolicyCfg: alwaysSample},
		},
	})
	require.NoError(t, err)
	err = p.Start(t.Context(), componenttest.NewNopHost())
	defer func() {
		err = p.Shutdown(t.Context())
		require.NoError(t, err)
	}()

	// verify
	assert.Equal(t, err, errors.New(`duplicate policy name "always_sample"`))
}

func TestDropPolicyIsFirstInPolicyList(t *testing.T) {
	controller := newTestTSPController()
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
					Name: "drop-metrics-policy",
					Type: Drop,
				},
				DropCfg: DropCfg{
					SubPolicyCfg: []AndSubPolicyCfg{
						{
							sharedPolicyCfg: sharedPolicyCfg{
								Name: "drop-metrics-policy",
								Type: StringAttribute,
								StringAttributeCfg: StringAttributeCfg{
									Key:    "url.path",
									Values: []string{"/metrics"},
								},
							},
						},
					},
				},
			},
		},
		Options: []Option{
			withTestController(controller),
		},
	}
	p, err := newTracesProcessor(t.Context(), processortest.NewNopSettings(metadata.Type), msp, cfg)
	require.NoError(t, err)
	err = p.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		err := p.Shutdown(t.Context())
		require.NoError(t, err)
	}()

	metricsTrace := simpleTracesWithID(uInt64ToTraceID(1))
	metricsTrace.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("url.path", "/metrics")

	require.NoError(t, p.ConsumeTraces(t.Context(), metricsTrace))

	healthTrace := simpleTracesWithID(uInt64ToTraceID(2))
	healthTrace.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Attributes().PutStr("url.path", "/health")

	require.NoError(t, p.ConsumeTraces(t.Context(), healthTrace))

	controller.waitForTick()
	controller.waitForTick()

	assert.Len(t, msp.AllTraces(), 1, "Health trace should be sampled")
	sampledTraceIDs := make(map[pcommon.TraceID]struct{})
	for _, trace := range msp.AllTraces() {
		sampledTraceIDs[trace.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).TraceID()] = struct{}{}
	}
	require.Len(t, sampledTraceIDs, 1)
	assert.Contains(t, sampledTraceIDs, uInt64ToTraceID(2))
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
	for i := range numIDs {
		traceIDs[i] = uInt64ToTraceID(uint64(i))
		// Send each span in a separate batch
		for j := 0; j <= i; j++ {
			td := simpleTraces()
			span := td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
			span.SetTraceID(traceIDs[i])

			if j != 0 {
				span.SetParentSpanID(uInt64ToSpanID(uint64(spanID)))
			}
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
	NextDecision    samplingpolicy.Decision
	NextError       error
	EvaluationCount int
}

var _ samplingpolicy.Evaluator = (*mockPolicyEvaluator)(nil)

func (m *mockPolicyEvaluator) Evaluate(context.Context, pcommon.TraceID, *samplingpolicy.TraceData) (samplingpolicy.Decision, error) {
	m.EvaluationCount++
	return m.NextDecision, m.NextError
}

type syncIDBatcher struct {
	sync.Mutex
	openBatch idbatcher.Batch
	batchPipe chan idbatcher.Batch
	stopped   bool
	currentID uint64
}

var _ idbatcher.Batcher = (*syncIDBatcher)(nil)

func newSyncIDBatcher() idbatcher.Batcher {
	batches := make(chan idbatcher.Batch, 1)
	batches <- nil
	return &syncIDBatcher{
		batchPipe: batches,
		openBatch: idbatcher.Batch{},
	}
}

func (s *syncIDBatcher) AddToCurrentBatch(id pcommon.TraceID) uint64 {
	s.Lock()
	defer s.Unlock()
	if s.stopped {
		panic("cannot add to stopped batcher!")
	}
	s.openBatch[id] = struct{}{}
	return s.currentID
}

// MoveToEarlierBatch is a noop for a sync batcher as there is only one pending batch.
func (s *syncIDBatcher) MoveToEarlierBatch(_ pcommon.TraceID, currentBatch, _ uint64) uint64 {
	s.Lock()
	defer s.Unlock()
	return currentBatch
}

func (s *syncIDBatcher) RemoveFromBatch(id pcommon.TraceID, batch uint64) {
	s.Lock()
	defer s.Unlock()
	if batch == s.currentID {
		delete(s.openBatch, id)
	}
}

func (s *syncIDBatcher) CloseCurrentAndTakeFirstBatch() (idbatcher.Batch, bool) {
	s.Lock()
	defer s.Unlock()
	firstBatch, ok := <-s.batchPipe
	// When batchPipe is closed it means we have stopped and just need to return the openBatch as the last entries.
	if !ok {
		batch := s.openBatch
		s.openBatch = nil
		return batch, false
	}
	// Do not move the open batch to the channel if we are stopped. It will panic, we return it once the channel is closed instead.
	if !s.stopped {
		s.batchPipe <- s.openBatch
		s.openBatch = idbatcher.Batch{}
		s.currentID++
	}
	return firstBatch, true
}

func (s *syncIDBatcher) Stop() {
	s.Lock()
	defer s.Unlock()
	s.stopped = true
	close(s.batchPipe)
}

func simpleTraces() ptrace.Traces {
	return simpleTracesWithID(pcommon.TraceID([16]byte{1, 2, 3, 4}))
}

func simpleTracesWithID(traceID pcommon.TraceID) ptrace.Traces {
	traces := ptrace.NewTraces()
	traces.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty().SetTraceID(traceID)
	return traces
}

// TestNumericAttributeCases tests cases for the numeric attribute filter
func TestNumericAttributeCases(t *testing.T) {
	tests := []struct {
		name           string
		minValue       int64
		maxValue       int64
		testValue      int64
		expectedResult samplingpolicy.Decision
		description    string
	}{
		{
			name:           "Only min_value set (positive)",
			minValue:       400,
			maxValue:       0, // not set (default)
			testValue:      500,
			expectedResult: samplingpolicy.Sampled,
			description:    "Should sample when value >= min_value and max_value not set",
		},
		{
			name:           "Only min_value set (negative value)",
			minValue:       -100,
			maxValue:       0, // not set (default)
			testValue:      50,
			expectedResult: samplingpolicy.Sampled,
			description:    "Should sample when value >= min_value (negative) and max_value not set",
		},
		{
			name:           "Only max_value set (positive)",
			minValue:       0, // not set (default)
			maxValue:       1000,
			testValue:      500,
			expectedResult: samplingpolicy.Sampled,
			description:    "Should sample when value <= max_value and min_value not set",
		},
		{
			name:           "Both min and max set",
			minValue:       100,
			maxValue:       200,
			testValue:      150,
			expectedResult: samplingpolicy.Sampled,
			description:    "Should sample when min_value <= value <= max_value",
		},
		{
			name:           "Value below min_value",
			minValue:       400,
			maxValue:       0, // not set (default)
			testValue:      300,
			expectedResult: samplingpolicy.NotSampled,
			description:    "Should not sample when value < min_value",
		},
		{
			name:           "Value above max_value",
			minValue:       0, // not set (default)
			maxValue:       100,
			testValue:      200,
			expectedResult: samplingpolicy.NotSampled,
			description:    "Should not sample when value > max_value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := NumericAttributeCfg{
				Key:      "test_attribute",
				MinValue: tt.minValue,
				MaxValue: tt.maxValue,
			}

			settings := componenttest.NewNopTelemetrySettings()

			evaluator, err := getSharedPolicyEvaluator(settings, &sharedPolicyCfg{
				Name:                "test-policy",
				Type:                NumericAttribute,
				NumericAttributeCfg: cfg,
			}, nil)
			require.NoError(t, err)
			require.NotNil(t, evaluator)

			// Create test trace data
			trace := &samplingpolicy.TraceData{}
			trace.ReceivedBatches = ptrace.NewTraces()

			rs := trace.ReceivedBatches.ResourceSpans().AppendEmpty()
			ils := rs.ScopeSpans().AppendEmpty()
			span := ils.Spans().AppendEmpty()
			span.Attributes().PutInt("test_attribute", tt.testValue)

			decision, err := evaluator.Evaluate(t.Context(), pcommon.TraceID([16]byte{1, 2, 3, 4}), trace)
			require.NoError(t, err)

			assert.Equal(t, tt.expectedResult, decision, tt.description)
		})
	}
}

func TestDropLargeTraces(t *testing.T) {
	controller := newTestTSPController()

	traces := ptrace.NewTraces()
	// Create a large trace (clearly more than 1024 bytes).
	largeTrace := traces.ResourceSpans().AppendEmpty()
	ss := largeTrace.ScopeSpans().AppendEmpty()
	largeValue := strings.Repeat("bar", 1024)
	sp := ss.Spans().AppendEmpty()
	sp.SetTraceID(pcommon.TraceID([16]byte{1, 2, 3, 4}))
	sp.Attributes().PutStr("foo", largeValue)

	// Small trace with just one attribute.
	smallTrace := traces.ResourceSpans().AppendEmpty()
	ss = smallTrace.ScopeSpans().AppendEmpty()
	sp = ss.Spans().AppendEmpty()
	sp.SetTraceID(pcommon.TraceID([16]byte{1, 2, 3, 5}))
	sp.Attributes().PutStr("foo", "short")

	cfg := Config{
		DecisionWait:            defaultTestDecisionWait,
		NumTraces:               uint64(4),
		ExpectedNewTracesPerSec: 64,
		MaximumTraceSizeBytes:   1024,
		PolicyCfgs:              testPolicy,
		Options: []Option{
			withTestController(controller),
		},
	}
	telem := setupTestTelemetry()
	telemetrySettings := telem.newSettings()
	nextConsumer := new(consumertest.TracesSink)
	processor, err := newTracesProcessor(t.Context(), telemetrySettings, nextConsumer, cfg)
	require.NoError(t, err)

	err = processor.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		err = processor.Shutdown(t.Context())
		require.NoError(t, err)
	}()

	require.NoError(t, processor.ConsumeTraces(t.Context(), traces))
	controller.waitForTick()
	controller.waitForTick()

	allSampledTraces := nextConsumer.AllTraces()
	assert.Len(t, allSampledTraces, 1)

	// Verify that the config can be changed without restarting the processor.
	processor.(*tailSamplingSpanProcessor).SetMaximumTraceSizeBytes(1 << 20)
	controller.waitForTick()

	largeOnly := ptrace.NewTraces()
	// Create a another large trace as ConsumeTraces is not guaranteed to preserve the trace.
	largeTrace = largeOnly.ResourceSpans().AppendEmpty()
	ss = largeTrace.ScopeSpans().AppendEmpty()
	sp = ss.Spans().AppendEmpty()
	sp.SetTraceID(pcommon.TraceID([16]byte{1, 2, 3, 6}))
	sp.Attributes().PutStr("foo", largeValue)

	require.NoError(t, processor.ConsumeTraces(t.Context(), largeOnly))
	controller.waitForTick()
	controller.waitForTick()

	allSampledTraces = nextConsumer.AllTraces()
	// The sink will still contain the original trace.
	assert.Len(t, allSampledTraces, 2)

	// These traces should not count as dropped too early as we record a separate metric.
	var md metricdata.ResourceMetrics
	require.NoError(t, telem.reader.Collect(t.Context(), &md))

	expectedTooEarly := metricdata.Metrics{
		Name:        "otelcol_processor_tail_sampling_sampling_trace_dropped_too_early",
		Description: "Count of traces that needed to be dropped before the configured wait time [Development]",
		Unit:        "{traces}",
		Data: metricdata.Sum[int64]{
			IsMonotonic: true,
			Temporality: metricdata.CumulativeTemporality,
			DataPoints: []metricdata.DataPoint[int64]{
				{
					Value: 0,
				},
			},
		},
	}
	tooEarly := telem.getMetric(expectedTooEarly.Name, md)
	metricdatatest.AssertEqual(t, expectedTooEarly, tooEarly, metricdatatest.IgnoreTimestamp())

	expectedTooLarge := metricdata.Metrics{
		Name:        "otelcol_processor_tail_sampling_traces_dropped_too_large",
		Description: "Count of traces that were dropped because they were too large [Development]",
		Unit:        "{traces}",
		Data: metricdata.Sum[int64]{
			IsMonotonic: true,
			Temporality: metricdata.CumulativeTemporality,
			DataPoints: []metricdata.DataPoint[int64]{
				{
					Value: 1,
				},
			},
		},
	}
	tooLarge := telem.getMetric(expectedTooLarge.Name, md)
	metricdatatest.AssertEqual(t, expectedTooLarge, tooLarge, metricdatatest.IgnoreTimestamp())
}

// TestDeleteQueueCleared verifies that all in memory traces are removed from
// both the idToTrace map as well as the deleteTraceQueue.
func TestDeleteQueueCleared(t *testing.T) {
	controller := newTestTSPController()

	traceIDs, batches := generateIDsAndBatches(128)
	cfg := Config{
		DecisionWait:            defaultTestDecisionWait,
		NumTraces:               uint64(2 * len(traceIDs)),
		ExpectedNewTracesPerSec: 64,
		// Use cache so that traces are cleared from memory.
		DecisionCache: DecisionCacheConfig{
			SampledCacheSize:    128,
			NonSampledCacheSize: 128,
		},
		PolicyCfgs: testPolicy,
		Options: []Option{
			withTestController(controller),
		},
	}
	telem := setupTestTelemetry()
	telemetrySettings := telem.newSettings()
	nextConsumer := new(consumertest.TracesSink)
	sp, err := newTracesProcessor(t.Context(), telemetrySettings, nextConsumer, cfg)
	require.NoError(t, err)

	err = sp.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		err = sp.Shutdown(t.Context())
		require.NoError(t, err)
	}()

	for _, batch := range batches {
		require.NoError(t, sp.ConsumeTraces(t.Context(), batch))
	}
	controller.waitForTick()
	controller.waitForTick()

	allSampledTraces := nextConsumer.AllTraces()
	assert.Len(t, allSampledTraces, 128)
	// All traces should be flushed from the map.
	assert.Empty(t, sp.(*tailSamplingSpanProcessor).idToTrace)
	// All traces should be removed from the delete queue.
	assert.Zero(t, sp.(*tailSamplingSpanProcessor).deleteTraceQueue.Len())
}

func TestRootReceivedBatcher(t *testing.T) {
	traceIDs, batches := generateIDsAndBatches(128)
	cfg := Config{
		DecisionWait:            time.Minute,
		NumTraces:               uint64(2 * len(traceIDs)),
		ExpectedNewTracesPerSec: 64,
		DecisionCache: DecisionCacheConfig{
			SampledCacheSize:    128,
			NonSampledCacheSize: 128,
		},
		PolicyCfgs: []PolicyCfg{
			{sharedPolicyCfg: sharedPolicyCfg{
				Name: "test-policy",
				Type: Probabilistic,
				ProbabilisticCfg: ProbabilisticCfg{
					SamplingPercentage: 50,
				},
			}},
		},
		DecisionWaitAfterRootReceived: time.Second,
		DropPendingTracesOnShutdown:   true,
	}
	telem := setupTestTelemetry()
	telemetrySettings := telem.newSettings()
	nextConsumer := new(consumertest.TracesSink)
	sp, err := newTracesProcessor(t.Context(), telemetrySettings, nextConsumer, cfg)
	require.NoError(t, err)

	err = sp.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		err = sp.Shutdown(t.Context())
		require.NoError(t, err)
	}()

	for _, batch := range batches {
		require.NoError(t, sp.ConsumeTraces(t.Context(), batch))
	}
	// Wait long enough that we pass the decision wait after a root is received,
	// but no where near the base decision wait.
	time.Sleep(2 * time.Second)

	// Make sure about half of traces are sampled before a tick is called.
	allSampledTraces := nextConsumer.AllTraces()
	assert.Less(t, len(allSampledTraces), len(traceIDs)*6/10)
	assert.Greater(t, len(allSampledTraces), len(traceIDs)*4/10)
}

func TestExtension(t *testing.T) {
	controller := newTestTSPController()
	msp := new(consumertest.TracesSink)

	cfg := Config{
		DecisionWait: defaultTestDecisionWait,
		NumTraces:    defaultNumTraces,
		PolicyCfgs: []PolicyCfg{
			{
				sharedPolicyCfg: sharedPolicyCfg{
					Name: "extension",
					Type: "my_extension",
					ExtensionCfg: map[string]map[string]any{
						"my_extension": {
							"foo": "bar",
						},
					},
				},
			},
		},
		Options: []Option{
			withTestController(controller),
		},
	}
	p, err := newTracesProcessor(t.Context(), processortest.NewNopSettings(metadata.Type), msp, cfg)
	require.NoError(t, err)

	host := &extensionHost{}
	require.NoError(t, p.Start(t.Context(), host))
	defer func() {
		require.NoError(t, p.Shutdown(t.Context()))
	}()

	assert.Equal(t, "extension", host.extension.policyName)
	assert.Equal(t, map[string]any{"foo": "bar"}, host.extension.cfg)
}

type extensionHost struct {
	extension *extension
}

func (h *extensionHost) GetExtensions() map[component.ID]component.Component {
	if h.extension == nil {
		h.extension = &extension{}
	}
	return map[component.ID]component.Component{
		component.MustNewID("my_extension"): h.extension,
	}
}

type extension struct {
	policyName string
	cfg        map[string]any
}

var _ samplingpolicy.Extension = &extension{}

// NewEvaluator implements samplingpolicy.Extension.
func (e *extension) NewEvaluator(policyName string, cfg map[string]any) (samplingpolicy.Evaluator, error) {
	e.policyName = policyName
	e.cfg = cfg
	return nil, nil
}

// Start implements component.Component.
func (*extension) Start(_ context.Context, _ component.Host) error {
	return nil
}

// Shutdown implements component.Component.
func (*extension) Shutdown(_ context.Context) error {
	return nil
}
