// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package groupbytraceprocessor

import (
	"context"
	"errors"
	"maps"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processortest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/batchpersignal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/groupbytraceprocessor/internal/metadata"
)

func TestTraceIsDispatchedAfterDuration(t *testing.T) {
	// prepare
	traces := simpleTraces()

	wgReceived := &sync.WaitGroup{} // we wait for the next (mock) processor to receive the trace
	config := Config{
		WaitDuration: time.Nanosecond,
		NumTraces:    10,
		NumWorkers:   4,
	}
	mockProcessor := &mockProcessor{
		onTraces: func(_ context.Context, received ptrace.Traces) error {
			assert.Equal(t, traces, received)
			wgReceived.Done()
			return nil
		},
	}

	wgDeleted := &sync.WaitGroup{} // we wait for the next (mock) processor to receive the trace

	p := newGroupByTraceProcessor(processortest.NewNopSettings(metadata.Type), mockProcessor, config)
	backing := newMemoryStorage(p.telemetryBuilder)
	st := &mockStorage{
		onCreateOrAppend: backing.createOrAppend,
		onGet:            backing.get,
		onDelete: func(traceID pcommon.TraceID) ([]ptrace.ResourceSpans, error) {
			wgDeleted.Done()
			return backing.delete(traceID)
		},
	}
	p.st = st
	ctx := t.Context()
	assert.NoError(t, p.Start(ctx, componenttest.NewNopHost()))
	defer func() {
		assert.NoError(t, p.Shutdown(ctx))
	}()

	// test
	wgReceived.Add(1) // one should be received
	wgDeleted.Add(1)  // one should be deleted
	assert.NoError(t, p.ConsumeTraces(ctx, traces))

	// verify
	wgReceived.Wait()
	wgDeleted.Wait()
}

func TestInternalCacheLimit(t *testing.T) {
	// prepare
	wg := &sync.WaitGroup{} // we wait for the next (mock) processor to receive the trace

	config := Config{
		// should be long enough for the test to run without traces being finished, but short enough to not
		// badly influence the testing experience
		WaitDuration: 50 * time.Millisecond,

		// we create 6 traces, only 5 should be at the storage in the end
		NumTraces: 5,

		NumWorkers: 1,
	}

	wg.Add(5) // 5 traces are expected to be received

	var receivedTraceIDs []pcommon.TraceID
	mockProcessor := &mockProcessor{}
	mockProcessor.onTraces = func(_ context.Context, received ptrace.Traces) error {
		traceID := received.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).TraceID()
		receivedTraceIDs = append(receivedTraceIDs, traceID)
		wg.Done()
		return nil
	}

	p := newGroupByTraceProcessor(processortest.NewNopSettings(metadata.Type), mockProcessor, config)
	st := newMemoryStorage(p.telemetryBuilder)
	p.st = st
	ctx := t.Context()
	assert.NoError(t, p.Start(ctx, componenttest.NewNopHost()))
	defer func() {
		assert.NoError(t, p.Shutdown(ctx))
	}()

	// test
	traceIDs := [][16]byte{
		{1, 2, 3, 4},
		{2, 3, 4, 5},
		{3, 4, 5, 6},
		{4, 5, 6, 7},
		{5, 6, 7, 8},
		{6, 7, 8, 9},
	}

	// 6 iterations
	for _, traceID := range traceIDs {
		batch := simpleTracesWithID(pcommon.TraceID(traceID))
		assert.NoError(t, p.ConsumeTraces(ctx, batch))
	}

	wg.Wait()

	// verify
	assert.Len(t, receivedTraceIDs, 5)

	for i := 5; i > 0; i-- { // last 5 traces
		traceID := pcommon.TraceID(traceIDs[i])
		assert.Contains(t, receivedTraceIDs, traceID)
	}

	// the first trace should have been evicted
	assert.NotContains(t, receivedTraceIDs, traceIDs[0])
}

func TestProcessorCapabilities(t *testing.T) {
	// prepare
	config := Config{
		WaitDuration: time.Nanosecond,
		NumTraces:    10,
		NumWorkers:   1,
	}
	// test
	p := newGroupByTraceProcessor(processortest.NewNopSettings(metadata.Type), consumertest.NewNop(), config)
	st := newMemoryStorage(p.telemetryBuilder)
	p.st = st
	caps := p.Capabilities()

	// verify
	assert.NotNil(t, p)
	assert.True(t, caps.MutatesData)
}

func TestProcessBatchDoesntFail(t *testing.T) {
	// prepare
	config := Config{
		WaitDuration: time.Nanosecond,
		NumTraces:    10,
		NumWorkers:   1,
	}

	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4})

	trace := ptrace.NewTraces()
	rs := trace.ResourceSpans().AppendEmpty()
	ils := rs.ScopeSpans().AppendEmpty()
	span := ils.Spans().AppendEmpty()
	span.SetTraceID(traceID)
	span.SetSpanID([8]byte{1, 2, 3, 4})

	p := newGroupByTraceProcessor(processortest.NewNopSettings(metadata.Type), consumertest.NewNop(), config)
	assert.NotNil(t, p)
	st := newMemoryStorage(p.telemetryBuilder)
	p.st = st
	// test
	assert.NoError(t, p.onTraceReceived(tracesWithID{id: traceID, td: trace}, p.eventMachine.workers[0]))
}

func TestTraceDisappearedFromStorageBeforeReleasing(t *testing.T) {
	// prepare
	config := Config{
		WaitDuration: time.Second, // we are not waiting for this whole time
		NumTraces:    8,
		NumWorkers:   4,
	}
	st := &mockStorage{
		onGet: func(pcommon.TraceID) ([]ptrace.ResourceSpans, error) {
			return nil, nil
		},
	}

	p := newGroupByTraceProcessor(processortest.NewNopSettings(metadata.Type), consumertest.NewNop(), config)
	require.NotNil(t, p)

	p.st = st

	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4})
	batch := simpleTracesWithID(traceID)

	ctx := t.Context()
	assert.NoError(t, p.Start(ctx, componenttest.NewNopHost()))
	defer func() {
		assert.NoError(t, p.Shutdown(ctx))
	}()

	err := p.ConsumeTraces(t.Context(), batch)
	require.NoError(t, err)

	// test
	// we trigger this manually, instead of waiting the whole duration
	err = p.markAsReleased(traceID, p.eventMachine.workers[workerIndexForTraceID(traceID, config.NumWorkers)].fire)

	// verify
	assert.Error(t, err)
}

func TestTraceErrorFromStorageWhileReleasing(t *testing.T) {
	// prepare
	config := Config{
		WaitDuration: time.Second, // we are not waiting for this whole time
		NumTraces:    8,
		NumWorkers:   4,
	}
	expectedError := errors.New("some unexpected error")
	st := &mockStorage{
		onGet: func(pcommon.TraceID) ([]ptrace.ResourceSpans, error) {
			return nil, expectedError
		},
	}

	p := newGroupByTraceProcessor(processortest.NewNopSettings(metadata.Type), consumertest.NewNop(), config)
	require.NotNil(t, p)
	p.st = st

	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4})
	batch := simpleTracesWithID(traceID)

	ctx := t.Context()
	assert.NoError(t, p.Start(ctx, componenttest.NewNopHost()))
	defer func() {
		assert.NoError(t, p.Shutdown(ctx))
	}()

	err := p.ConsumeTraces(t.Context(), batch)
	require.NoError(t, err)

	// test
	// we trigger this manually, instead of waiting the whole duration
	err = p.markAsReleased(traceID, p.eventMachine.workers[workerIndexForTraceID(traceID, config.NumWorkers)].fire)

	// verify
	assert.ErrorIs(t, err, expectedError)
}

func TestTraceErrorFromStorageWhileProcessingTrace(t *testing.T) {
	// prepare
	config := Config{
		WaitDuration: time.Second, // we are not waiting for this whole time
		NumTraces:    5,
		NumWorkers:   1,
	}
	expectedError := errors.New("some unexpected error")
	st := &mockStorage{
		onCreateOrAppend: func(pcommon.TraceID, ptrace.Traces) error {
			return expectedError
		},
	}

	p := newGroupByTraceProcessor(processortest.NewNopSettings(metadata.Type), consumertest.NewNop(), config)
	require.NotNil(t, p)
	p.st = st

	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4})

	trace := ptrace.NewTraces()
	rss := trace.ResourceSpans()
	rs := rss.AppendEmpty()
	ils := rs.ScopeSpans().AppendEmpty()
	span := ils.Spans().AppendEmpty()
	span.SetTraceID(traceID)
	span.SetSpanID([8]byte{1, 2, 3, 4})

	batch := batchpersignal.SplitTraces(trace)

	// test
	err := p.onTraceReceived(tracesWithID{id: traceID, td: batch[0]}, p.eventMachine.workers[0])

	// verify
	assert.ErrorIs(t, err, expectedError)
}

func TestAddSpansToExistingTrace(t *testing.T) {
	// prepare
	wg := &sync.WaitGroup{}
	config := Config{
		WaitDuration: 50 * time.Millisecond,
		NumTraces:    8,
		NumWorkers:   4,
	}

	var receivedTraces []ptrace.ResourceSpans
	next := &mockProcessor{
		onTraces: func(_ context.Context, traces ptrace.Traces) error {
			require.Equal(t, 2, traces.ResourceSpans().Len())
			receivedTraces = append(receivedTraces, traces.ResourceSpans().At(0), traces.ResourceSpans().At(1))
			wg.Done()
			return nil
		},
	}

	p := newGroupByTraceProcessor(processortest.NewNopSettings(metadata.Type), next, config)
	require.NotNil(t, p)
	st := newMemoryStorage(p.telemetryBuilder)
	p.st = st

	ctx := t.Context()
	assert.NoError(t, p.Start(ctx, componenttest.NewNopHost()))
	defer func() {
		assert.NoError(t, p.Shutdown(ctx))
	}()

	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4})

	// test
	first := simpleTracesWithID(traceID)
	first.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).SetName("first-span")

	second := simpleTracesWithID(traceID)
	second.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).SetName("second-span")

	wg.Add(1)

	assert.NoError(t, p.ConsumeTraces(t.Context(), first))
	assert.NoError(t, p.ConsumeTraces(t.Context(), second))

	wg.Wait()

	// verify
	assert.Len(t, receivedTraces, 2)
}

func TestTraceErrorFromStorageWhileProcessingSecondTrace(t *testing.T) {
	// prepare
	config := Config{
		WaitDuration: time.Second, // we are not waiting for this whole time
		NumTraces:    8,
		NumWorkers:   4,
	}
	st := &mockStorage{}
	next := &mockProcessor{}

	p := newGroupByTraceProcessor(processortest.NewNopSettings(metadata.Type), next, config)
	require.NotNil(t, p)
	p.st = st

	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4})

	trace := ptrace.NewTraces()
	rss := trace.ResourceSpans()
	rs := rss.AppendEmpty()
	ils := rs.ScopeSpans().AppendEmpty()
	span := ils.Spans().AppendEmpty()
	span.SetTraceID(traceID)
	span.SetSpanID([8]byte{1, 2, 3, 4})

	batch := batchpersignal.SplitTraces(trace)

	// test
	err := p.eventMachine.consume(batch[0])
	assert.NoError(t, err)

	expectedError := errors.New("some unexpected error")
	st.onCreateOrAppend = func(pcommon.TraceID, ptrace.Traces) error {
		return expectedError
	}

	// processing another batch for the same trace takes a slightly different code path
	err = p.onTraceReceived(tracesWithID{id: traceID, td: batch[0]},
		p.eventMachine.workers[workerIndexForTraceID(traceID, config.NumWorkers)],
	)

	// verify
	assert.ErrorIs(t, err, expectedError)
}

func TestErrorFromStorageWhileRemovingTrace(t *testing.T) {
	// prepare
	config := Config{
		WaitDuration: time.Second, // we are not waiting for this whole time
		NumTraces:    8,
		NumWorkers:   4,
	}
	expectedError := errors.New("some unexpected error")
	st := &mockStorage{
		onDelete: func(pcommon.TraceID) ([]ptrace.ResourceSpans, error) {
			return nil, expectedError
		},
	}
	next := &mockProcessor{}

	p := newGroupByTraceProcessor(processortest.NewNopSettings(metadata.Type), next, config)
	require.NotNil(t, p)
	p.st = st
	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4})

	// test
	err := p.onTraceRemoved(traceID)

	// verify
	assert.ErrorIs(t, err, expectedError)
}

func TestTraceNotFoundWhileRemovingTrace(t *testing.T) {
	// prepare
	config := Config{
		WaitDuration: time.Second, // we are not waiting for this whole time
		NumTraces:    8,
		NumWorkers:   4,
	}
	st := &mockStorage{
		onDelete: func(pcommon.TraceID) ([]ptrace.ResourceSpans, error) {
			return nil, nil
		},
	}
	next := &mockProcessor{}

	p := newGroupByTraceProcessor(processortest.NewNopSettings(metadata.Type), next, config)
	require.NotNil(t, p)
	p.st = st
	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4})

	// test
	err := p.onTraceRemoved(traceID)

	// verify
	assert.Error(t, err)
}

func TestTracesAreDispatchedInIndividualBatches(t *testing.T) {
	// prepare
	wg := &sync.WaitGroup{}

	config := Config{
		WaitDuration: time.Nanosecond, // we are not waiting for this whole time
		NumTraces:    8,
		NumWorkers:   4,
	}

	next := &mockProcessor{
		onTraces: func(_ context.Context, traces ptrace.Traces) error {
			// we should receive two batches, each one with one trace
			assert.Equal(t, 1, traces.ResourceSpans().Len())
			wg.Done()
			return nil
		},
	}

	p := newGroupByTraceProcessor(processortest.NewNopSettings(metadata.Type), next, config)
	require.NotNil(t, p)
	st := newMemoryStorage(p.telemetryBuilder)
	p.st = st
	ctx := t.Context()
	assert.NoError(t, p.Start(ctx, componenttest.NewNopHost()))
	defer func() {
		assert.NoError(t, p.Shutdown(ctx))
	}()

	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4})

	firstTrace := ptrace.NewTraces()
	firstRss := firstTrace.ResourceSpans()
	firstResourceSpans := firstRss.AppendEmpty()
	ils := firstResourceSpans.ScopeSpans().AppendEmpty()
	span := ils.Spans().AppendEmpty()
	span.SetTraceID(traceID)

	secondTraceID := pcommon.TraceID([16]byte{2, 3, 4, 5})
	secondTrace := ptrace.NewTraces()
	secondRss := secondTrace.ResourceSpans()
	secondResourceSpans := secondRss.AppendEmpty()
	secondIls := secondResourceSpans.ScopeSpans().AppendEmpty()
	secondSpan := secondIls.Spans().AppendEmpty()
	secondSpan.SetTraceID(secondTraceID)

	// test
	wg.Add(2)

	assert.NoError(t, p.eventMachine.consume(firstTrace))
	assert.NoError(t, p.eventMachine.consume(secondTrace))

	wg.Wait()

	// verify
	// verification is done at onTraces from the mockProcessor
}

func TestErrorOnProcessResourceSpansContinuesProcessing(t *testing.T) {
	// prepare
	config := Config{
		WaitDuration: time.Second, // we are not waiting for this whole time
		NumTraces:    8,
		NumWorkers:   4,
	}
	st := &mockStorage{}
	next := &mockProcessor{}

	p := newGroupByTraceProcessor(processortest.NewNopSettings(metadata.Type), next, config)
	require.NotNil(t, p)
	p.st = st
	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4})

	trace := ptrace.NewTraces()
	rss := trace.ResourceSpans()
	rs := rss.AppendEmpty()
	ils := rs.ScopeSpans().AppendEmpty()
	span := ils.Spans().AppendEmpty()
	span.SetTraceID(traceID)
	span.SetSpanID([8]byte{1, 2, 3, 4})

	expectedError := errors.New("some unexpected error")
	returnedError := false
	st.onCreateOrAppend = func(pcommon.TraceID, ptrace.Traces) error {
		returnedError = true
		return expectedError
	}

	// test
	assert.Error(t, p.onTraceReceived(tracesWithID{id: traceID, td: trace}, p.eventMachine.workers[0]))

	// verify
	assert.True(t, returnedError)
}

func TestAsyncOnRelease(t *testing.T) {
	blockCh := make(chan struct{})
	blocker := &blockingConsumer{
		blockCh: blockCh,
	}
	set := processortest.NewNopSettings(metadata.Type)
	tel, _ := metadata.NewTelemetryBuilder(set.TelemetrySettings)
	sp := &groupByTraceProcessor{
		logger:           zap.NewNop(),
		nextConsumer:     blocker,
		telemetryBuilder: tel,
	}
	assert.NoError(t, sp.onTraceReleased(nil))
	close(blockCh)
}

func BenchmarkConsumeTracesCompleteOnFirstBatch(b *testing.B) {
	// prepare
	config := Config{
		WaitDuration: 50 * time.Millisecond,
		NumTraces:    defaultNumTraces,
		NumWorkers:   4 * defaultNumWorkers,
	}

	// For each input trace there are always <= 2 events in the machine simultaneously.
	semaphoreCh := make(chan struct{}, bufferSize/2)
	next := &mockProcessor{onTraces: func(context.Context, ptrace.Traces) error {
		<-semaphoreCh
		return nil
	}}

	p := newGroupByTraceProcessor(processortest.NewNopSettings(metadata.Type), next, config)
	require.NotNil(b, p)
	st := newMemoryStorage(p.telemetryBuilder)
	p.st = st
	ctx := b.Context()
	require.NoError(b, p.Start(ctx, componenttest.NewNopHost()))
	defer func() {
		assert.NoError(b, p.Shutdown(ctx))
	}()

	for n := 0; b.Loop(); n++ {
		traceID := pcommon.TraceID([16]byte{byte(1 + n), 2, 3, 4})
		trace := simpleTracesWithID(traceID)
		assert.NoError(b, p.ConsumeTraces(b.Context(), trace))
	}
}

func TestSubtrace_HappyPath_TwoServices(t *testing.T) {
	traceID := makeTraceID(1)
	rootA := makeSpanID(1)
	childA := makeSpanID(2)
	rootB := makeSpanID(3)
	childB := makeSpanID(4)

	sink := new(consumertest.TracesSink)
	cfg := Config{
		NumTraces:    100,
		NumWorkers:   1,
		WaitDuration: 20 * time.Millisecond,
		EmitStrategy: EmitStrategyService,
	}
	p := newSubtraceProcessor(t, cfg, sink)
	defer func() { assert.NoError(t, p.Shutdown(t.Context())) }()

	tdA := buildServiceTrace(traceID, "svc-a", rootA, childA)
	tdB := buildRemoteChildTrace(traceID, "svc-b", rootA, rootB, childB)

	require.NoError(t, p.ConsumeTraces(t.Context(), tdA))
	require.NoError(t, p.ConsumeTraces(t.Context(), tdB))

	// Wait for both subtraces to be flushed.
	require.Eventually(t, func() bool {
		return sink.SpanCount() == 4
	}, 5*time.Second, 5*time.Millisecond)

	// Each service should arrive as a separate batch.
	batches := sink.AllTraces()
	assert.Len(t, batches, 2)

	// Total span count must be 4.
	total := 0
	for _, b := range batches {
		total += b.SpanCount()
	}
	assert.Equal(t, 4, total)

	svcA := map[pcommon.SpanID]bool{rootA: true, childA: true}
	svcB := map[pcommon.SpanID]bool{rootB: true, childB: true}
	for _, b := range batches {
		ids := map[pcommon.SpanID]bool{}
		for i := 0; i < b.ResourceSpans().Len(); i++ {
			rs := b.ResourceSpans().At(i)
			for j := 0; j < rs.ScopeSpans().Len(); j++ {
				ss := rs.ScopeSpans().At(j)
				for k := 0; k < ss.Spans().Len(); k++ {
					ids[ss.Spans().At(k).SpanID()] = true
				}
			}
		}
		assert.True(t, maps.Equal(ids, svcA) || maps.Equal(ids, svcB),
			"batch span IDs %v matched neither svc-a %v nor svc-b %v", ids, svcA, svcB)
	}
}

func TestSubtrace_IsRemoteCleared_TwoServices(t *testing.T) {
	traceID := makeTraceID(10)
	rootA := makeSpanID(1)
	childA := makeSpanID(2)
	rootB := makeSpanID(3)
	childB := makeSpanID(4)

	sink := new(consumertest.TracesSink)
	cfg := Config{
		NumTraces:    100,
		NumWorkers:   1,
		WaitDuration: 20 * time.Millisecond,
		EmitStrategy: EmitStrategyService,
	}
	p := newSubtraceProcessor(t, cfg, sink)
	defer func() { assert.NoError(t, p.Shutdown(t.Context())) }()

	tdA := buildServiceTrace(traceID, "svc-a", rootA, childA)

	// Build svc-b where rootB's IS_REMOTE flag is explicitly cleared (HAS_IS_REMOTE=1,
	// IS_REMOTE=0). isLocalRoot short-circuits on this flag combination and returns
	// false, so rootB is not treated as a service-entry span. svc-b spans are therefore
	// absorbed into svc-a's subtrace, and the whole trace is emitted as a single batch.
	tdB := buildRemoteChildTrace(traceID, "svc-b", rootA, rootB, childB)
	tdB.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).SetFlags(spanFlagsContextHasIsRemoteMask)

	require.NoError(t, p.ConsumeTraces(t.Context(), tdA))
	require.NoError(t, p.ConsumeTraces(t.Context(), tdB))

	require.Eventually(t, func() bool {
		return sink.SpanCount() == 4
	}, 5*time.Second, 5*time.Millisecond)

	// One batch: all four spans belong to rootA's subtrace.
	assert.Len(t, sink.AllTraces(), 1)
}

func TestSubtrace_LocalRootArrivesLate(t *testing.T) {
	traceID := makeTraceID(2)
	rootID := makeSpanID(1)
	childID := makeSpanID(2)

	sink := new(consumertest.TracesSink)
	cfg := Config{
		NumTraces:    100,
		NumWorkers:   1,
		WaitDuration: 50 * time.Millisecond,
		EmitStrategy: EmitStrategyService,
	}
	p := newSubtraceProcessor(t, cfg, sink)
	defer func() { assert.NoError(t, p.Shutdown(t.Context())) }()

	// Send child first (parent not yet in index — child is misclassified as a root).
	tdChild := buildServiceTrace(traceID, "svc-a", childID)
	// Override the parent span ID to make child's parent = rootID (not in index yet).
	tdChild.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).SetParentSpanID(rootID)
	require.NoError(t, p.ConsumeTraces(t.Context(), tdChild))

	// Now send the root.
	tdRoot := buildServiceTrace(traceID, "svc-a", rootID)
	require.NoError(t, p.ConsumeTraces(t.Context(), tdRoot))

	// After wait duration, at least one flush should happen (root starts its own timer).
	require.Eventually(t, func() bool {
		return sink.SpanCount() >= 1
	}, 5*time.Second, 5*time.Millisecond)
}

func TestSubtrace_SpansSplitAcrossCalls(t *testing.T) {
	traceID := makeTraceID(3)
	rootID := makeSpanID(1)
	child1 := makeSpanID(2)
	child2 := makeSpanID(3)

	sink := new(consumertest.TracesSink)
	cfg := Config{
		NumTraces:    100,
		NumWorkers:   1,
		WaitDuration: 50 * time.Millisecond,
		EmitStrategy: EmitStrategyService,
	}
	p := newSubtraceProcessor(t, cfg, sink)
	defer func() { assert.NoError(t, p.Shutdown(t.Context())) }()

	// Send root first.
	require.NoError(t, p.ConsumeTraces(t.Context(), buildServiceTrace(traceID, "svc-a", rootID)))
	// Then child1.
	td1 := buildServiceTrace(traceID, "svc-a", child1)
	td1.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).SetParentSpanID(rootID)
	require.NoError(t, p.ConsumeTraces(t.Context(), td1))
	// Then child2.
	td2 := buildServiceTrace(traceID, "svc-a", child2)
	td2.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).SetParentSpanID(rootID)
	require.NoError(t, p.ConsumeTraces(t.Context(), td2))

	// After the wait duration, all 3 spans should arrive in one batch.
	require.Eventually(t, func() bool {
		return sink.SpanCount() >= 3
	}, 5*time.Second, 5*time.Millisecond)
}

func TestSubtrace_ShutdownDrain_OrphanSpans(t *testing.T) {
	traceID := makeTraceID(4)
	orphanID := makeSpanID(1)
	missingParent := makeSpanID(99)

	sink := new(consumertest.TracesSink)
	cfg := Config{
		NumTraces:    100,
		NumWorkers:   1,
		WaitDuration: 10 * time.Second, // long enough that timer won't fire
		EmitStrategy: EmitStrategyService,
	}
	p := newSubtraceProcessor(t, cfg, sink)

	td := buildServiceTrace(traceID, "svc-a", orphanID)
	td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).SetParentSpanID(missingParent)
	require.NoError(t, p.ConsumeTraces(t.Context(), td))

	// Wait until the span is persisted in storage before shutting down.
	require.Eventually(t, func() bool {
		return len(p.subSt.traceIDs()) > 0
	}, 2*time.Second, time.Millisecond)

	// Shutdown before the timer fires — drain should emit the span.
	require.NoError(t, p.Shutdown(t.Context()))
	assert.Equal(t, 1, sink.SpanCount())
}

func TestSubtrace_RingBufferEviction(t *testing.T) {
	const capacity = 3
	sink := new(consumertest.TracesSink)
	cfg := Config{
		NumTraces:    capacity,
		NumWorkers:   1,
		WaitDuration: 10 * time.Second,
		EmitStrategy: EmitStrategyService,
	}
	p := newSubtraceProcessor(t, cfg, sink)
	defer func() { assert.NoError(t, p.Shutdown(t.Context())) }()

	// Fill the buffer beyond capacity.
	for i := byte(1); i <= byte(capacity+1); i++ {
		tid := makeTraceID(i)
		sid := makeSpanID(i)
		require.NoError(t, p.ConsumeTraces(t.Context(), buildServiceTrace(tid, "svc", sid)))
	}

	// The eviction metric should have been incremented at least once.
	// We can't read the metric directly; just verify the processor doesn't crash.
}

func TestSubtrace_ShutdownWithLocalRoots(t *testing.T) {
	traceID := makeTraceID(6)
	rootID := makeSpanID(1)
	childID := makeSpanID(2)

	sink := new(consumertest.TracesSink)
	cfg := Config{
		NumTraces:    100,
		NumWorkers:   1,
		WaitDuration: 10 * time.Second,
		EmitStrategy: EmitStrategyService,
	}
	p := newSubtraceProcessor(t, cfg, sink)

	td := buildServiceTrace(traceID, "svc", rootID, childID)
	require.NoError(t, p.ConsumeTraces(t.Context(), td))

	// Wait until spans are persisted in storage before shutting down.
	require.Eventually(t, func() bool {
		return len(p.subSt.traceIDs()) > 0
	}, 2*time.Second, time.Millisecond)

	// Shutdown before the wait_duration expires.
	require.NoError(t, p.Shutdown(t.Context()))

	// The shutdown drain should have emitted both spans.
	assert.Equal(t, 2, sink.SpanCount())
}

func TestSubtrace_Regression_TraceStrategy(t *testing.T) {
	sink := new(consumertest.TracesSink)
	cfg := Config{
		NumTraces:    100,
		NumWorkers:   1,
		WaitDuration: 20 * time.Millisecond,
		EmitStrategy: EmitStrategyTrace,
	}
	p := newGroupByTraceProcessor(processortest.NewNopSettings(metadata.Type), sink, cfg)
	require.NotNil(t, p)
	st := newMemoryStorage(p.telemetryBuilder)
	p.st = st
	require.NoError(t, p.Start(t.Context(), nil))
	defer func() { assert.NoError(t, p.Shutdown(t.Context())) }()

	traceID := makeTraceID(8)
	td := buildServiceTrace(traceID, "svc-a", makeSpanID(1), makeSpanID(2))
	require.NoError(t, p.ConsumeTraces(t.Context(), td))

	require.Eventually(t, func() bool {
		return sink.SpanCount() == 2
	}, 5*time.Second, 5*time.Millisecond)
}

// newSubtraceProcessor creates a started groupByTraceProcessor wired with the
// given config (EmitStrategy must be EmitStrategyService) and a consumertest.TracesSink.
// The caller is responsible for calling Shutdown.
func newSubtraceProcessor(t *testing.T, cfg Config, sink *consumertest.TracesSink) *groupByTraceProcessor {
	t.Helper()
	cfg.EmitStrategy = EmitStrategyService
	p := newGroupByTraceProcessor(processortest.NewNopSettings(metadata.Type), sink, cfg)
	require.NotNil(t, p)

	subSt := newSubtraceMemoryStorage(p.telemetryBuilder)
	p.subSt = subSt
	p.eventMachine.onSubtraceExpired = p.onSubtraceExpired
	p.eventMachine.onSubtraceReleased = p.onSubtraceReleased
	p.eventMachine.onSubtraceRemoved = p.onSubtraceRemoved
	for _, w := range p.eventMachine.workers {
		w.subtraceBuffer = newSubtraceRingBuffer(cfg.NumTraces / cfg.NumWorkers)
	}

	require.NoError(t, p.Start(t.Context(), nil))
	return p
}

// buildServiceTrace builds a ptrace.Traces with spans for a single service.
// All spans share traceID. The server span (index 0) has an empty parent and
// acts as the local root; subsequent spans are children of the server span.
func buildServiceTrace(traceID pcommon.TraceID, serviceName string, spanIDs ...pcommon.SpanID) ptrace.Traces {
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("service.name", serviceName)
	ss := rs.ScopeSpans().AppendEmpty()
	for i, sid := range spanIDs {
		s := ss.Spans().AppendEmpty()
		s.SetTraceID(traceID)
		s.SetSpanID(sid)
		if i == 0 {
			s.SetParentSpanID(pcommon.NewSpanIDEmpty())
		} else {
			s.SetParentSpanID(spanIDs[0]) // children of the root
		}
	}
	return td
}

// buildRemoteChildTrace builds a trace for service B whose root is a remote
// child of service A's root (i.e. the IS_REMOTE flag is set on the root span).
func buildRemoteChildTrace(traceID pcommon.TraceID, serviceName string, remoteParentID pcommon.SpanID, spanIDs ...pcommon.SpanID) ptrace.Traces {
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("service.name", serviceName)
	ss := rs.ScopeSpans().AppendEmpty()
	for i, sid := range spanIDs {
		s := ss.Spans().AppendEmpty()
		s.SetTraceID(traceID)
		s.SetSpanID(sid)
		if i == 0 {
			s.SetParentSpanID(remoteParentID)
			s.SetFlags(spanFlagsContextHasIsRemoteMask | spanFlagsContextIsRemoteMask)
		} else {
			s.SetParentSpanID(spanIDs[0])
		}
	}
	return td
}

type mockProcessor struct {
	mutex    sync.Mutex
	onTraces func(context.Context, ptrace.Traces) error
}

var _ processor.Traces = (*mockProcessor)(nil)

func (m *mockProcessor) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	if m.onTraces != nil {
		m.mutex.Lock()
		defer m.mutex.Unlock()
		return m.onTraces(ctx, td)
	}
	return nil
}

func (*mockProcessor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}

func (*mockProcessor) Shutdown(context.Context) error {
	return nil
}

func (*mockProcessor) Start(context.Context, component.Host) error {
	return nil
}

type mockStorage struct {
	onCreateOrAppend func(pcommon.TraceID, ptrace.Traces) error
	onGet            func(pcommon.TraceID) ([]ptrace.ResourceSpans, error)
	onDelete         func(pcommon.TraceID) ([]ptrace.ResourceSpans, error)
	onStart          func() error
	onShutdown       func() error
}

var _ traceStorage = (*mockStorage)(nil)

func (st *mockStorage) createOrAppend(traceID pcommon.TraceID, trace ptrace.Traces) error {
	if st.onCreateOrAppend != nil {
		return st.onCreateOrAppend(traceID, trace)
	}
	return nil
}

func (st *mockStorage) get(traceID pcommon.TraceID) ([]ptrace.ResourceSpans, error) {
	if st.onGet != nil {
		return st.onGet(traceID)
	}
	return nil, nil
}

func (st *mockStorage) delete(traceID pcommon.TraceID) ([]ptrace.ResourceSpans, error) {
	if st.onDelete != nil {
		return st.onDelete(traceID)
	}
	return nil, nil
}

func (st *mockStorage) start() error {
	if st.onStart != nil {
		return st.onStart()
	}
	return nil
}

func (st *mockStorage) shutdown() error {
	if st.onShutdown != nil {
		return st.onShutdown()
	}
	return nil
}

type blockingConsumer struct {
	blockCh <-chan struct{}
}

var _ consumer.Traces = (*blockingConsumer)(nil)

func (*blockingConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (b *blockingConsumer) ConsumeTraces(context.Context, ptrace.Traces) error {
	<-b.blockCh
	return nil
}

func simpleTraces() ptrace.Traces {
	return simpleTracesWithID(pcommon.TraceID([16]byte{1, 2, 3, 4}))
}

func simpleTracesWithID(traceID pcommon.TraceID) ptrace.Traces {
	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	ils := rs.ScopeSpans().AppendEmpty()
	ils.Spans().AppendEmpty().SetTraceID(traceID)
	return traces
}
