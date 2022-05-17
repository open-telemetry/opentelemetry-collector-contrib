// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cascadingfilterprocessor

import (
	"context"
	"errors"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/cascadingfilterprocessor/bigendianconverter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/cascadingfilterprocessor/config"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/cascadingfilterprocessor/idbatcher"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/cascadingfilterprocessor/sampling"
)

const (
	defaultTestDecisionWait = 30 * time.Second
)

//nolint:unused
var testPolicy = []config.TraceAcceptCfg{{
	Name:           "test-policy",
	SpansPerSecond: 1000,
}}

func TestSequentialTraceArrival(t *testing.T) {
	traceIds, batches := generateIdsAndBatches(128)
	cfg := config.Config{
		DecisionWait:            defaultTestDecisionWait,
		NumTraces:               uint64(2 * len(traceIds)),
		ExpectedNewTracesPerSec: 64,
		PolicyCfgs:              testPolicy,
	}
	sp, err := newTraceProcessor(zap.NewNop(), consumertest.NewNop(), cfg)
	require.NoError(t, err)
	tsp := sp.(*cascadingFilterSpanProcessor)
	for _, batch := range batches {
		assert.NoError(t, tsp.ConsumeTraces(context.Background(), batch))
	}

	for i := range traceIds {
		d, ok := tsp.idToTrace.Load(traceKey(traceIds[i].Bytes()))
		require.True(t, ok, "Missing expected traceId")
		v := d.(*sampling.TraceData)
		require.Equal(t, int32(i+1), v.SpanCount, "Incorrect number of spans for entry %d", i)
	}
}

func TestConcurrentTraceArrival(t *testing.T) {
	traceIds, batches := generateIdsAndBatches(128)

	var wg sync.WaitGroup
	cfg := config.Config{
		DecisionWait:            defaultTestDecisionWait,
		NumTraces:               uint64(2 * len(traceIds)),
		ExpectedNewTracesPerSec: 64,
		PolicyCfgs:              testPolicy,
	}
	sp, err := newTraceProcessor(zap.NewNop(), consumertest.NewNop(), cfg)
	require.NoError(t, err)
	tsp := sp.(*cascadingFilterSpanProcessor)
	for _, batch := range batches {
		// Add the same traceId twice.
		wg.Add(2)
		go func(td pdata.Traces) {
			if err := tsp.ConsumeTraces(context.Background(), td); err != nil {
				t.Errorf("Failed consuming traces: %v", err)
			}
			wg.Done()
		}(batch)
		go func(td pdata.Traces) {
			if err := tsp.ConsumeTraces(context.Background(), td); err != nil {
				t.Errorf("Failed consuming traces: %v", err)
			}
			wg.Done()
		}(batch)
	}

	wg.Wait()

	for i := range traceIds {
		d, ok := tsp.idToTrace.Load(traceKey(traceIds[i].Bytes()))
		require.True(t, ok, "Missing expected traceId")
		v := d.(*sampling.TraceData)
		require.Equal(t, int32(i+1)*2, v.SpanCount, "Incorrect number of spans for entry %d", i)
	}
}

func TestSequentialTraceMapSize(t *testing.T) {
	traceIds, batches := generateIdsAndBatches(210)
	const maxSize = 100
	cfg := config.Config{
		DecisionWait:            defaultTestDecisionWait,
		NumTraces:               uint64(maxSize),
		ExpectedNewTracesPerSec: 64,
		PolicyCfgs:              testPolicy,
	}
	sp, err := newTraceProcessor(zap.NewNop(), consumertest.NewNop(), cfg)
	require.NoError(t, err)
	tsp := sp.(*cascadingFilterSpanProcessor)
	for _, batch := range batches {
		if err := tsp.ConsumeTraces(context.Background(), batch); err != nil {
			t.Errorf("Failed consuming traces: %v", err)
		}
	}

	// On sequential insertion it is possible to know exactly which traces should be still on the map.
	for i := 0; i < len(traceIds)-maxSize; i++ {
		_, ok := tsp.idToTrace.Load(traceKey(traceIds[i].Bytes()))
		require.False(t, ok, "Found unexpected traceId[%d] still on map (id: %v)", i, traceIds[i])
	}
}

func TestConcurrentTraceMapSize(t *testing.T) {
	_, batches := generateIdsAndBatches(210)
	const maxSize = 100
	var wg sync.WaitGroup
	cfg := config.Config{
		DecisionWait:            defaultTestDecisionWait,
		NumTraces:               uint64(maxSize),
		ExpectedNewTracesPerSec: 64,
		PolicyCfgs:              testPolicy,
	}
	sp, err := newTraceProcessor(zap.NewNop(), consumertest.NewNop(), cfg)
	require.NoError(t, err)
	tsp := sp.(*cascadingFilterSpanProcessor)
	for _, batch := range batches {
		wg.Add(1)
		go func(td pdata.Traces) {
			if err := tsp.ConsumeTraces(context.Background(), td); err != nil {
				t.Errorf("Failed consuming traces: %v", err)
			}
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
	tsp := &cascadingFilterSpanProcessor{
		ctx:               context.Background(),
		nextConsumer:      msp,
		maxNumTraces:      maxSize,
		logger:            zap.NewNop(),
		decisionBatcher:   newSyncIDBatcher(decisionWaitSeconds),
		traceAcceptRules:  []*TraceAcceptEvaluator{{Name: "mock-policy", Evaluator: mpe, ctx: context.TODO()}},
		deleteChan:        make(chan traceKey, maxSize),
		policyTicker:      mtt,
		maxSpansPerSecond: 10000,
		filteringEnabled:  true,
	}

	_, batches := generateIdsAndBatches(210)
	currItem := 0
	numSpansPerBatchWindow := 10
	// First evaluations shouldn't have anything to evaluate, until decision wait time passed.
	for evalNum := 0; evalNum < decisionWaitSeconds; evalNum++ {
		for ; currItem < numSpansPerBatchWindow*(evalNum+1); currItem++ {
			if err := tsp.ConsumeTraces(context.Background(), batches[currItem]); err != nil {
				t.Errorf("Failed consuming traces: %v", err)
			}
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
	if err := tsp.ConsumeTraces(context.Background(), batches[0]); err != nil {
		t.Errorf("Failed consuming traces: %v", err)
	}
	expectedNumWithLateSpan := numSpansPerBatchWindow + 1
	require.Equal(t, expectedNumWithLateSpan, msp.SpanCount(), "late span was not accounted for")
}

func TestSamplingPolicyNoFiltering(t *testing.T) {
	const maxSize = 100
	const decisionWaitSeconds = 5
	// For this test explicitly control the timer calls and batcher, and set a mock
	// sampling policy evaluator.
	msp := new(consumertest.TracesSink)
	mtt := &manualTTicker{}
	tsp := &cascadingFilterSpanProcessor{
		ctx:               context.Background(),
		nextConsumer:      msp,
		maxNumTraces:      maxSize,
		logger:            zap.NewNop(),
		decisionBatcher:   newSyncIDBatcher(decisionWaitSeconds),
		deleteChan:        make(chan traceKey, maxSize),
		policyTicker:      mtt,
		maxSpansPerSecond: 10000,
		filteringEnabled:  false,
	}

	_, batches := generateIdsAndBatches(1)
	if err := tsp.ConsumeTraces(context.Background(), batches[0]); err != nil {
		t.Errorf("Failed consuming traces: %v", err)
	}
	require.False(t, mtt.Started, "Time ticker was expected to have not started since filtering is not enabled")
	require.Equal(t, 1, msp.SpanCount(), "all spans were accounted for")
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
	tsp := &cascadingFilterSpanProcessor{
		ctx:             context.Background(),
		nextConsumer:    msp,
		maxNumTraces:    maxSize,
		logger:          zap.NewNop(),
		decisionBatcher: newSyncIDBatcher(decisionWaitSeconds),
		traceAcceptRules: []*TraceAcceptEvaluator{
			{
				Name: "policy-1", Evaluator: mpe1, ctx: context.TODO(),
			},
			{
				Name: "policy-2", Evaluator: mpe2, ctx: context.TODO(),
			}},
		deleteChan:        make(chan traceKey, maxSize),
		policyTicker:      mtt,
		maxSpansPerSecond: 10000,
		filteringEnabled:  true,
	}

	_, batches := generateIdsAndBatches(210)
	currItem := 0
	numSpansPerBatchWindow := 10
	// First evaluations shouldn't have anything to evaluate, until decision wait time passed.
	for evalNum := 0; evalNum < decisionWaitSeconds; evalNum++ {
		for ; currItem < numSpansPerBatchWindow*(evalNum+1); currItem++ {
			if err := tsp.ConsumeTraces(context.Background(), batches[currItem]); err != nil {
				t.Errorf("Failed consuming traces: %v", err)
			}

			require.True(t, mtt.Started, "Time ticker was expected to have started")
		}
		tsp.samplingPolicyOnTick()
		require.False(
			t,
			msp.SpanCount() != 0 || mpe1.EvaluationCount != 0 || mpe2.EvaluationCount != 0,
			"policy for initial items was evaluated before decision wait period",
		)
	}

	// Single traceAcceptRules will decide to sample
	mpe1.NextDecision = sampling.Sampled
	mpe2.NextDecision = sampling.Unspecified
	tsp.samplingPolicyOnTick()
	require.False(
		t,
		msp.SpanCount() == 0 || mpe1.EvaluationCount == 0,
		"policy should have been evaluated totalspans == %d and evaluationcount(1) == %d and evaluationcount(2) == %d",
		msp.SpanCount(),
		mpe1.EvaluationCount,
		mpe2.EvaluationCount,
	)

	require.Equal(t, numSpansPerBatchWindow, msp.SpanCount(), "nextConsumer should've been called with exactly 1 batch of spans")

	// Late span of a sampled trace should be sent directly down the pipeline exporter
	if err := tsp.ConsumeTraces(context.Background(), batches[0]); err != nil {
		t.Errorf("Failed consuming traces: %v", err)
	}

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
	tsp := &cascadingFilterSpanProcessor{
		ctx:               context.Background(),
		nextConsumer:      msp,
		maxNumTraces:      maxSize,
		logger:            zap.NewNop(),
		decisionBatcher:   newSyncIDBatcher(decisionWaitSeconds),
		traceAcceptRules:  []*TraceAcceptEvaluator{{Name: "mock-policy", Evaluator: mpe, ctx: context.TODO()}},
		deleteChan:        make(chan traceKey, maxSize),
		policyTicker:      mtt,
		maxSpansPerSecond: 10000,
		filteringEnabled:  true,
	}

	_, batches := generateIdsAndBatches(210)
	currItem := 0
	numSpansPerBatchWindow := 10
	// First evaluations shouldn't have anything to evaluate, until decision wait time passed.
	for evalNum := 0; evalNum < decisionWaitSeconds; evalNum++ {
		for ; currItem < numSpansPerBatchWindow*(evalNum+1); currItem++ {
			if err := tsp.ConsumeTraces(context.Background(), batches[currItem]); err != nil {
				t.Errorf("Failed consuming traces: %v", err)
			}
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

	if err := tsp.ConsumeTraces(context.Background(), batches[0]); err != nil {
		t.Errorf("Failed consuming traces: %v", err)
	}
	require.Equal(t, 0, msp.SpanCount())

	mpe.NextDecision = sampling.Unspecified
	mpe.NextError = errors.New("mock policy error")
	tsp.samplingPolicyOnTick()
	require.EqualValues(t, 0, msp.SpanCount(), "exporter should have received zero spans")
	require.EqualValues(t, 6, mpe.EvaluationCount, "policy should have been evaluated 6 times")

	// Late span of a non-sampled trace should be ignored
	if err := tsp.ConsumeTraces(context.Background(), batches[0]); err != nil {
		t.Errorf("Failed consuming traces: %v", err)
	}
	require.Equal(t, 0, msp.SpanCount())
}

func TestSamplingPolicyDecisionDrop(t *testing.T) {
	const maxSize = 100
	const decisionWaitSeconds = 5
	// For this test explicitly control the timer calls and batcher, and set a mock
	// sampling policy evaluator.
	msp := new(consumertest.TracesSink)
	mpe := &mockPolicyEvaluator{}
	mde := &mockDropEvaluator{}
	mtt := &manualTTicker{}
	tsp := &cascadingFilterSpanProcessor{
		ctx:               context.Background(),
		nextConsumer:      msp,
		maxNumTraces:      maxSize,
		logger:            zap.NewNop(),
		decisionBatcher:   newSyncIDBatcher(decisionWaitSeconds),
		traceAcceptRules:  []*TraceAcceptEvaluator{{Name: "mock-policy", Evaluator: mpe, ctx: context.TODO()}},
		traceRejectRules:  []*TraceRejectEvaluator{{Name: "mock-drop-eval", Evaluator: mde, ctx: context.TODO()}},
		deleteChan:        make(chan traceKey, maxSize),
		policyTicker:      mtt,
		maxSpansPerSecond: 10000,
		filteringEnabled:  true,
	}

	_, batches := generateIdsAndBatches(210)
	currItem := 0
	numSpansPerBatchWindow := 10
	// First evaluations shouldn't have anything to evaluate, until decision wait time passed.
	for evalNum := 0; evalNum < decisionWaitSeconds; evalNum++ {
		for ; currItem < numSpansPerBatchWindow*(evalNum+1); currItem++ {
			if err := tsp.ConsumeTraces(context.Background(), batches[currItem]); err != nil {
				t.Errorf("Failed consuming traces: %v", err)
			}
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
	tsp.samplingPolicyOnTick()
	require.EqualValues(t, 0, msp.SpanCount(), "exporter should have received zero spans since they were dropped")
	require.EqualValues(t, 0, mpe.EvaluationCount, "policy should have been evaluated 0 times since it was dropped")
}

func TestSamplingPolicyDecisionNoLimitSet(t *testing.T) {
	const maxSize = 100
	const decisionWaitSeconds = 2
	// For this test explicitly control the timer calls and batcher, and set a mock
	// sampling policy evaluator.
	msp := new(consumertest.TracesSink)
	mtt := &manualTTicker{}
	tsp := &cascadingFilterSpanProcessor{
		ctx:               context.Background(),
		nextConsumer:      msp,
		maxNumTraces:      maxSize,
		logger:            zap.NewNop(),
		decisionBatcher:   newSyncIDBatcher(decisionWaitSeconds),
		deleteChan:        make(chan traceKey, maxSize),
		policyTicker:      mtt,
		maxSpansPerSecond: 0,
		filteringEnabled:  true,
	}

	_, batches := generateIdsAndBatches(210)
	currItem := 0
	numSpansPerBatchWindow := 10

	// First evaluations shouldn't have anything to evaluate, until decision wait time passed.
	for evalNum := 0; evalNum < decisionWaitSeconds; evalNum++ {
		for ; currItem < numSpansPerBatchWindow*(evalNum+1); currItem++ {
			if err := tsp.ConsumeTraces(context.Background(), batches[currItem]); err != nil {
				t.Errorf("Failed consuming traces: %v", err)
			}
			require.True(t, mtt.Started, "Time ticker was expected to have started")
		}
		tsp.samplingPolicyOnTick()
	}

	// Now the first batch that waited the decision period.
	tsp.samplingPolicyOnTick()
	//require.EqualValues(t, 210, msp.SpanCount(), "exporter should have received all spans since no rate limiting was applied")
	require.EqualValues(t, 10, msp.SpanCount())
}

func TestMultipleBatchesAreCombinedIntoOne(t *testing.T) {
	const maxSize = 100
	const decisionWaitSeconds = 1
	// For this test explicitly control the timer calls and batcher, and set a mock
	// sampling policy evaluator.
	msp := new(consumertest.TracesSink)
	mpe := &mockPolicyEvaluator{}
	mtt := &manualTTicker{}
	tsp := &cascadingFilterSpanProcessor{
		ctx:               context.Background(),
		nextConsumer:      msp,
		maxNumTraces:      maxSize,
		logger:            zap.NewNop(),
		decisionBatcher:   newSyncIDBatcher(decisionWaitSeconds),
		traceAcceptRules:  []*TraceAcceptEvaluator{{Name: "mock-policy", Evaluator: mpe, ctx: context.TODO()}},
		deleteChan:        make(chan traceKey, maxSize),
		policyTicker:      mtt,
		maxSpansPerSecond: 10000,
		filteringEnabled:  true,
	}

	mpe.NextDecision = sampling.Sampled

	traceIds, batches := generateIdsAndBatches(3)
	for _, batch := range batches {
		require.NoError(t, tsp.ConsumeTraces(context.Background(), batch))
	}

	tsp.samplingPolicyOnTick()
	tsp.samplingPolicyOnTick()

	require.EqualValues(t, 3, len(msp.AllTraces()), "There should be three batches, one for each trace")

	expectedSpanIds := make(map[int][]pdata.SpanID)
	expectedSpanIds[0] = []pdata.SpanID{
		bigendianconverter.UInt64ToSpanID(uint64(1)),
	}
	expectedSpanIds[1] = []pdata.SpanID{
		bigendianconverter.UInt64ToSpanID(uint64(2)),
		bigendianconverter.UInt64ToSpanID(uint64(3)),
	}
	expectedSpanIds[2] = []pdata.SpanID{
		bigendianconverter.UInt64ToSpanID(uint64(4)),
		bigendianconverter.UInt64ToSpanID(uint64(5)),
		bigendianconverter.UInt64ToSpanID(uint64(6)),
	}

	receivedTraces := msp.AllTraces()
	for i, traceID := range traceIds {
		trace := findTrace(receivedTraces, traceID)
		require.NotNil(t, trace, "Trace was not received. TraceId %s", traceID.HexString())
		require.EqualValues(t, i+1, trace.SpanCount(), "The trace should have all of its spans in a single batch")

		expected := expectedSpanIds[i]
		got := collectSpanIds(trace)

		// might have received out of order, sort for comparison
		sort.Slice(got, func(i, j int) bool {
			a := bigendianconverter.SpanIDToUInt64(got[i])
			b := bigendianconverter.SpanIDToUInt64(got[j])
			return a < b
		})

		require.EqualValues(t, expected, got)
	}
}

func TestSamplingPolicyOnlyReject(t *testing.T) {
	const maxSize = 100
	const decisionWaitSeconds = 5
	// For this test explicitly control the timer calls and batcher, and set a mock
	// sampling policy evaluator.
	msp := new(consumertest.TracesSink)
	mtt := &manualTTicker{}
	mde := &mockDropFalseEvaluator{}
	tsp := &cascadingFilterSpanProcessor{
		ctx:               context.Background(),
		nextConsumer:      msp,
		maxNumTraces:      maxSize,
		logger:            zap.NewNop(),
		decisionBatcher:   newSyncIDBatcher(decisionWaitSeconds),
		traceRejectRules:  []*TraceRejectEvaluator{{Name: "mock-drop-eval", Evaluator: mde, ctx: context.TODO()}},
		deleteChan:        make(chan traceKey, maxSize),
		policyTicker:      mtt,
		maxSpansPerSecond: 10000,
		filteringEnabled:  true,
	}

	_, batches := generateIdsAndBatches(1)
	if err := tsp.ConsumeTraces(context.Background(), batches[0]); err != nil {
		t.Errorf("Failed consuming traces: %v", err)
	}

	// Count "decision wait" times
	for i := 0; i < decisionWaitSeconds; i++ {
		tsp.samplingPolicyOnTick()
	}

	tsp.samplingPolicyOnTick()

	require.Equal(t, 1, msp.SpanCount(), "all spans were accounted for")
	for _, trace := range msp.AllTraces() {
		for i := 0; i < trace.ResourceSpans().Len(); i++ {
			sss := trace.ResourceSpans().At(i).InstrumentationLibrarySpans()
			for j := 0; j < sss.Len(); j++ {
				spans := sss.At(j).Spans()
				for k := 0; k < spans.Len(); k++ {
					attrs := spans.At(k).Attributes()
					println(attrs.Len())
					_, found := attrs.Get("sampling.rule")
					require.False(t, found, "sampling.rule value should not be set when only reject rules are applied")
				}
			}
		}
	}
}

//nolint:unused
func collectSpanIds(trace *pdata.Traces) []pdata.SpanID {
	spanIDs := make([]pdata.SpanID, 0)

	for i := 0; i < trace.ResourceSpans().Len(); i++ {
		ilss := trace.ResourceSpans().At(i).InstrumentationLibrarySpans()

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

//nolint:unused
func findTrace(a []pdata.Traces, traceID pdata.TraceID) *pdata.Traces {
	for _, batch := range a {
		id := batch.ResourceSpans().At(0).InstrumentationLibrarySpans().At(0).Spans().At(0).TraceID()
		if traceID.Bytes() == id.Bytes() {
			return &batch
		}
	}
	return nil
}

func generateIdsAndBatches(numIds int) ([]pdata.TraceID, []pdata.Traces) {
	traceIds := make([]pdata.TraceID, numIds)
	spanID := 0
	var tds []pdata.Traces
	for i := 0; i < numIds; i++ {
		traceIds[i] = bigendianconverter.UInt64ToTraceID(1, uint64(i+1))
		// Send each span in a separate batch
		for j := 0; j <= i; j++ {
			td := simpleTraces()
			span := td.ResourceSpans().At(0).InstrumentationLibrarySpans().At(0).Spans().At(0)
			span.SetTraceID(traceIds[i])

			spanID++
			span.SetSpanID(bigendianconverter.UInt64ToSpanID(uint64(spanID)))
			tds = append(tds, td)
		}
	}

	return traceIds, tds
}

//nolint:unused
func simpleTraces() pdata.Traces {
	return simpleTracesWithID(pdata.NewTraceID([16]byte{1, 2, 3, 4}))
}

//nolint:unused
func simpleTracesWithID(traceID pdata.TraceID) pdata.Traces {
	traces := pdata.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()

	ils := rs.InstrumentationLibrarySpans().AppendEmpty()
	span := ils.Spans().AppendEmpty()
	span.SetTraceID(traceID)

	return traces
}

type mockPolicyEvaluator struct {
	NextDecision       sampling.Decision
	NextError          error
	EvaluationCount    int
	OnDroppedSpanCount int
}

type mockDropEvaluator struct{}
type mockDropFalseEvaluator struct{}

var _ sampling.PolicyEvaluator = (*mockPolicyEvaluator)(nil)
var _ sampling.DropTraceEvaluator = (*mockDropEvaluator)(nil)

func (d *mockDropFalseEvaluator) ShouldDrop(_ pdata.TraceID, _ *sampling.TraceData) bool {
	return false
}

func (m *mockPolicyEvaluator) Evaluate(_ pdata.TraceID, _ *sampling.TraceData) sampling.Decision {
	m.EvaluationCount++
	return m.NextDecision
}

func (d *mockDropEvaluator) ShouldDrop(_ pdata.TraceID, _ *sampling.TraceData) bool {
	return true
}

type manualTTicker struct {
	Started bool
}

var _ tTicker = (*manualTTicker)(nil)

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

func (s *syncIDBatcher) AddToCurrentBatch(id pdata.TraceID) {
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
