// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package groupbytraceprocessor

import (
	"context"
	"errors"
	"fmt"
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
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/metric/metricdata/metricdatatest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/batchpersignal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/groupbytraceprocessor/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/groupbytraceprocessor/internal/metadatatest"
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
		ids := batchSpanIDs(b)
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

	tdB := buildRemoteChildTrace(traceID, "svc-b", rootA, rootB, childB)
	tdB.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).SetFlags(spanFlagsContextHasIsRemoteMask)

	require.NoError(t, p.ConsumeTraces(t.Context(), tdA))
	require.NoError(t, p.ConsumeTraces(t.Context(), tdB))

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
		ids := batchSpanIDs(b)
		assert.True(t, maps.Equal(ids, svcA) || maps.Equal(ids, svcB),
			"batch span IDs %v matched neither svc-a %v nor svc-b %v", ids, svcA, svcB)
	}
}

func TestSubtrace_ParentArrivesLate(t *testing.T) {
	traceID := makeTraceID(2)
	rootID := makeSpanID(1)
	childID := makeSpanID(2)

	const waitDuration = 300 * time.Millisecond

	sink := new(consumertest.TracesSink)
	cfg := Config{
		NumTraces:    100,
		NumWorkers:   1,
		WaitDuration: waitDuration,
		EmitStrategy: EmitStrategyService,
	}
	p := newSubtraceProcessor(t, cfg, sink)
	defer func() { assert.NoError(t, p.Shutdown(t.Context())) }()

	// Send child first (its parent is not buffered yet).
	tdChild := buildServiceTrace(traceID, "svc-a", childID)
	// Override the parent span ID to make child's parent = rootID (not in index yet).
	tdChild.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).SetParentSpanID(rootID)
	require.NoError(t, p.ConsumeTraces(t.Context(), tdChild))

	// Hold the root back so the child's timer is guaranteed to fire first. Sending
	// them back to back would leave it up to the scheduler which release runs
	// first, and the child releasing first is the case worth pinning down.
	time.Sleep(waitDuration / 2)

	// Now send the parent.
	tdRoot := buildServiceTrace(traceID, "svc-a", rootID)
	require.NoError(t, p.ConsumeTraces(t.Context(), tdRoot))

	require.Eventually(t, func() bool {
		return sink.SpanCount() == 2
	}, 5*time.Second, 5*time.Millisecond)

	// The child must neither be emitted on its own nor emitted twice.
	assert.Never(t, func() bool {
		return sink.SpanCount() != 2
	}, 500*time.Millisecond, 10*time.Millisecond)

	batches := sink.AllTraces()
	require.Len(t, batches, 1, "root and child belong to the same service and must arrive in one batch")
	assert.Equal(t, map[pcommon.SpanID]bool{rootID: true, childID: true}, batchSpanIDs(batches[0]))
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
		WaitDuration: 200 * time.Millisecond,
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
		return sink.SpanCount() == 3
	}, 5*time.Second, 5*time.Millisecond)
	assert.Never(t, func() bool {
		return sink.SpanCount() != 3
	}, 500*time.Millisecond, 10*time.Millisecond)

	batches := sink.AllTraces()
	require.Len(t, batches, 1)
	assert.Equal(t, map[pcommon.SpanID]bool{rootID: true, child1: true, child2: true}, batchSpanIDs(batches[0]))
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
		return len(p.subSt.subtraceIDs()) > 0
	}, 2*time.Second, time.Millisecond)

	// Shutdown before the timer fires; drain should emit the span.
	require.NoError(t, p.Shutdown(t.Context()))
	assert.Equal(t, 1, sink.SpanCount())
}

func TestSubtrace_EvictedSubtracesAreReleasedNotDropped(t *testing.T) {
	const (
		capacity = 3
		overflow = 2
	)

	tel := componenttest.NewTelemetry()
	defer func() { assert.NoError(t, tel.Shutdown(t.Context())) }()

	sink := new(consumertest.TracesSink)
	cfg := Config{
		NumTraces:  capacity,
		NumWorkers: 1,
		// Long enough that nothing can be released by its own timer, so anything
		// reaching the sink got there by eviction.
		WaitDuration: 10 * time.Second,
		EmitStrategy: EmitStrategyService,
	}
	p := newSubtraceProcessorWithSettings(t, cfg, sink, metadatatest.NewSettings(tel))

	evicted := map[pcommon.SpanID]bool{}
	for i := byte(1); i <= byte(capacity+overflow); i++ {
		spanID := makeSpanID(i)
		require.NoError(t, p.ConsumeTraces(t.Context(), buildServiceTrace(makeTraceID(i), "svc", spanID)))
		if int(i) <= overflow {
			evicted[spanID] = true // the oldest entries are the ones pushed out
		}
	}

	// The evicted subtraces arrive without waiting for any timer.
	require.Eventually(t, func() bool {
		return sink.SpanCount() == overflow
	}, 5*time.Second, 5*time.Millisecond, "evicted spans never reached the next consumer")
	assert.Equal(t, evicted, spanIDsAcross(sink.AllTraces()))

	// The rest are still buffered, waiting out their wait_duration.
	assert.Len(t, p.subSt.subtraceIDs(), capacity)

	metadatatest.AssertEqualProcessorGroupbytraceTracesEvicted(t, tel,
		[]metricdata.DataPoint[int64]{{Value: overflow}},
		metricdatatest.IgnoreTimestamp())

	// Shutting down flushes the remainder, so nothing submitted is lost.
	require.NoError(t, p.Shutdown(t.Context()))
	assert.Equal(t, capacity+overflow, sink.SpanCount())
}

// spanIDsAcross returns the span IDs found in all of the given batches.
func spanIDsAcross(batches []ptrace.Traces) map[pcommon.SpanID]bool {
	ids := map[pcommon.SpanID]bool{}
	for _, b := range batches {
		for id := range batchSpanIDs(b) {
			ids[id] = true
		}
	}
	return ids
}

func TestSubtrace_ShutdownFlushesBufferedSpans(t *testing.T) {
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
		return len(p.subSt.subtraceIDs()) > 0
	}, 2*time.Second, time.Millisecond)

	// Shutdown before the wait_duration expires.
	require.NoError(t, p.Shutdown(t.Context()))

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

// TestSubtrace_Matrix_ThreeServices_MissingEntrySpan exercises a three-service
// trace (svc-a -> svc-b -> svc-c) in which one of the three entry span spans
// never arrives. The span whose parent is the missing root becomes a entry span
// itself, so every submitted span must still be emitted exactly once, grouped
// by service. The matrix covers each of the three missing roots against every
// arrival order of the three service batches.
func TestSubtrace_Matrix_ThreeServices_MissingEntrySpan(t *testing.T) {
	traceID := makeTraceID(20)
	rootA, childA := makeSpanID(1), makeSpanID(2)
	rootB, childB := makeSpanID(3), makeSpanID(4)
	rootC, childC := makeSpanID(5), makeSpanID(6)

	// The complete trace: each service's entry span comes first, and svc-b/svc-c
	// enter through a remote child of the previous service's child span.
	type service struct {
		name  string
		specs []spanSpec
	}
	complete := []service{
		{"svc-a", []spanSpec{{rootA, pcommon.NewSpanIDEmpty(), false}, {childA, rootA, false}}},
		{"svc-b", []spanSpec{{rootB, childA, true}, {childB, rootB, false}}},
		{"svc-c", []spanSpec{{rootC, childB, true}, {childC, rootC, false}}},
	}

	orders := [][]int{{0, 1, 2}, {0, 2, 1}, {1, 0, 2}, {1, 2, 0}, {2, 0, 1}, {2, 1, 0}}

	for missing := range complete {
		for _, order := range orders {
			name := fmt.Sprintf("missing_root_%s/order_%d%d%d",
				complete[missing].name, order[0], order[1], order[2])
			t.Run(name, func(t *testing.T) {
				// Drop the entry span of the "missing" service.
				submitted := make([]service, len(complete))
				for i, svc := range complete {
					submitted[i] = service{name: svc.name, specs: svc.specs}
					if i == missing {
						submitted[i].specs = svc.specs[1:]
					}
				}

				// Every service's submitted spans form exactly one expected batch:
				// the orphaned child is its own entry span once its parent is gone.
				var expected []map[pcommon.SpanID]bool
				expectedSpans := 0
				for _, svc := range submitted {
					ids := map[pcommon.SpanID]bool{}
					for _, spec := range svc.specs {
						ids[spec.id] = true
					}
					expected = append(expected, ids)
					expectedSpans += len(svc.specs)
				}

				sink := new(consumertest.TracesSink)
				cfg := Config{
					NumTraces:    100,
					NumWorkers:   1,
					WaitDuration: 20 * time.Millisecond,
					EmitStrategy: EmitStrategyService,
				}
				p := newSubtraceProcessor(t, cfg, sink)
				defer func() { assert.NoError(t, p.Shutdown(t.Context())) }()

				for _, i := range order {
					svc := submitted[i]
					require.NoError(t, p.ConsumeTraces(t.Context(),
						buildSpecTrace(traceID, svc.name, svc.specs...)))
				}

				require.Eventually(t, func() bool {
					return sink.SpanCount() >= expectedSpans
				}, 5*time.Second, 5*time.Millisecond)

				assert.Equal(t, expectedSpans, sink.SpanCount(), "no span may be emitted twice")

				batches := sink.AllTraces()
				require.Len(t, batches, len(expected))

				// Each emitted batch must match a distinct expected service group.
				remaining := expected
				for _, b := range batches {
					ids := batchSpanIDs(b)
					matched := -1
					for i, want := range remaining {
						if maps.Equal(ids, want) {
							matched = i
							break
						}
					}
					require.NotEqual(t, -1, matched,
						"batch span IDs %v matched none of the remaining expected groups %v", ids, remaining)
					remaining = append(remaining[:matched], remaining[matched+1:]...)
				}
				assert.Empty(t, remaining)
			})
		}
	}
}

// batchSpanIDs collects the set of span IDs contained in a batch.
func batchSpanIDs(td ptrace.Traces) map[pcommon.SpanID]bool {
	ids := map[pcommon.SpanID]bool{}
	for _, rs := range td.ResourceSpans().All() {
		for _, ss := range rs.ScopeSpans().All() {
			for _, s := range ss.Spans().All() {
				ids[s.SpanID()] = true
			}
		}
	}

	return ids
}

// spanSpec describes one span to build: its ID, its parent, and whether the
// parent context is flagged as remote (i.e. it is a service-entry span).
type spanSpec struct {
	id     pcommon.SpanID
	parent pcommon.SpanID
	remote bool
}

// buildSpecTrace builds a single-resource batch for serviceName holding the
// given spans, all sharing traceID.
func buildSpecTrace(traceID pcommon.TraceID, serviceName string, specs ...spanSpec) ptrace.Traces {
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("service.name", serviceName)
	ss := rs.ScopeSpans().AppendEmpty()
	for _, spec := range specs {
		s := ss.Spans().AppendEmpty()
		s.SetTraceID(traceID)
		s.SetSpanID(spec.id)
		s.SetParentSpanID(spec.parent)
		if spec.remote {
			s.SetFlags(spanFlagsContextHasIsRemoteMask | spanFlagsContextIsRemoteMask)
		}
	}
	return td
}

// newSubtraceProcessor creates a started groupByTraceProcessor wired with the
// given config (EmitStrategy must be EmitStrategyService) and a consumertest.TracesSink.
// The caller is responsible for calling Shutdown.
func newSubtraceProcessor(t *testing.T, cfg Config, sink *consumertest.TracesSink) *groupByTraceProcessor {
	t.Helper()
	return newSubtraceProcessorWithSettings(t, cfg, sink, processortest.NewNopSettings(metadata.Type))
}

// newSubtraceProcessorWithSettings is newSubtraceProcessor with the processor
// settings supplied, so that a test can read the recorded telemetry.
func newSubtraceProcessorWithSettings(t *testing.T, cfg Config, sink *consumertest.TracesSink, set processor.Settings) *groupByTraceProcessor {
	t.Helper()
	cfg.EmitStrategy = EmitStrategyService
	p := newGroupByTraceProcessor(set, sink, cfg)
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
// acts as the entry span; subsequent spans are children of the server span.
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

// Parentless spans of one service must be emitted as one batch, not one batch
// per span. This is the shape produced when the service-entry span lives in
// another Collector, or has already been released.
func TestSubtrace_ParentlessSiblingsGroupPerService(t *testing.T) {
	const perService = 5
	traceID := makeTraceID(60)
	missingParent := makeSpanID(99)

	sink := new(consumertest.TracesSink)
	cfg := Config{NumTraces: 1000, NumWorkers: 1, WaitDuration: 80 * time.Millisecond, EmitStrategy: EmitStrategyService}
	p := newSubtraceProcessor(t, cfg, sink)
	defer func() { assert.NoError(t, p.Shutdown(t.Context())) }()

	svcA := map[pcommon.SpanID]bool{}
	svcB := map[pcommon.SpanID]bool{}
	specsA := make([]spanSpec, 0, perService)
	specsB := make([]spanSpec, 0, perService)
	for i := range perService {
		a, b := makeSpanID(byte(1+i)), makeSpanID(byte(20+i))
		specsA = append(specsA, spanSpec{id: a, parent: missingParent})
		specsB = append(specsB, spanSpec{id: b, parent: missingParent})
		svcA[a] = true
		svcB[b] = true
	}
	require.NoError(t, p.ConsumeTraces(t.Context(), buildSpecTrace(traceID, "svc-a", specsA...)))
	require.NoError(t, p.ConsumeTraces(t.Context(), buildSpecTrace(traceID, "svc-b", specsB...)))

	require.Eventually(t, func() bool {
		return sink.SpanCount() == 2*perService
	}, 5*time.Second, 5*time.Millisecond)
	assert.Never(t, func() bool {
		return sink.SpanCount() != 2*perService
	}, 400*time.Millisecond, 10*time.Millisecond)

	batches := sink.AllTraces()
	require.Len(t, batches, 2, "one batch per service, not one per parentless span")
	for _, b := range batches {
		ids := batchSpanIDs(b)
		assert.True(t, maps.Equal(ids, svcA) || maps.Equal(ids, svcB),
			"batch span IDs %v matched neither svc-a %v nor svc-b %v", ids, svcA, svcB)
	}
}

// The service semconv attributes are used to determine service identity;
// other attributes don't influence which service a set of spans is
// associated with.
func TestSubtrace_OneServiceAcrossResources(t *testing.T) {
	traceID := makeTraceID(62)
	rootID := makeSpanID(1)
	childID := makeSpanID(2)

	build := func(pod string, spanID, parent pcommon.SpanID) ptrace.Traces {
		td := ptrace.NewTraces()
		rs := td.ResourceSpans().AppendEmpty()
		rs.Resource().Attributes().PutStr("service.name", "svc-a")
		rs.Resource().Attributes().PutStr("k8s.pod.name", pod)
		s := rs.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
		s.SetTraceID(traceID)
		s.SetSpanID(spanID)
		s.SetParentSpanID(parent)
		return td
	}

	sink := new(consumertest.TracesSink)
	cfg := Config{NumTraces: 1000, NumWorkers: 1, WaitDuration: 150 * time.Millisecond, EmitStrategy: EmitStrategyService}
	p := newSubtraceProcessor(t, cfg, sink)
	defer func() { assert.NoError(t, p.Shutdown(t.Context())) }()

	require.NoError(t, p.ConsumeTraces(t.Context(), build("pod-1", rootID, pcommon.NewSpanIDEmpty())))
	require.NoError(t, p.ConsumeTraces(t.Context(), build("pod-2", childID, rootID)))

	require.Eventually(t, func() bool { return sink.SpanCount() == 2 }, 5*time.Second, 5*time.Millisecond)
	assert.Never(t, func() bool { return sink.SpanCount() != 2 }, 400*time.Millisecond, 10*time.Millisecond)

	batches := sink.AllTraces()
	require.Len(t, batches, 1, "same service.name means one subtrace, whatever else differs on the resource")
	assert.Equal(t, 2, batches[0].ResourceSpans().Len(), "the distinct resources must be preserved")
	assert.Equal(t, map[pcommon.SpanID]bool{rootID: true, childID: true}, batchSpanIDs(batches[0]))
}

// A batch carrying several resources and scopes at once, which is what the
// processor sees before batchpersignal splits it apart.
func TestSubtrace_MultiResourceMultiScopeBatch(t *testing.T) {
	traceID := makeTraceID(63)
	rootA, rootB := makeSpanID(1), makeSpanID(20)

	td := ptrace.NewTraces()
	for _, svc := range []struct {
		name  string
		base  byte
		entry pcommon.SpanID
	}{{"svc-a", 1, rootA}, {"svc-b", 20, rootB}} {
		rs := td.ResourceSpans().AppendEmpty()
		rs.Resource().Attributes().PutStr("service.name", svc.name)
		for scope := range 2 {
			ss := rs.ScopeSpans().AppendEmpty()
			ss.Scope().SetName(fmt.Sprintf("lib-%d", scope))
			for k := range 2 {
				s := ss.Spans().AppendEmpty()
				s.SetTraceID(traceID)
				id := makeSpanID(svc.base + byte(scope*2+k))
				s.SetSpanID(id)
				switch id {
				case rootA:
					s.SetParentSpanID(pcommon.NewSpanIDEmpty())
				case rootB:
					s.SetParentSpanID(rootA)
					s.SetFlags(spanFlagsContextHasIsRemoteMask | spanFlagsContextIsRemoteMask)
				default:
					s.SetParentSpanID(svc.entry)
				}
			}
		}
	}

	sink := new(consumertest.TracesSink)
	cfg := Config{NumTraces: 1000, NumWorkers: 1, WaitDuration: 100 * time.Millisecond, EmitStrategy: EmitStrategyService}
	p := newSubtraceProcessor(t, cfg, sink)
	defer func() { assert.NoError(t, p.Shutdown(t.Context())) }()

	require.NoError(t, p.ConsumeTraces(t.Context(), td))
	require.Eventually(t, func() bool { return sink.SpanCount() == 8 }, 5*time.Second, 5*time.Millisecond)
	assert.Never(t, func() bool { return sink.SpanCount() != 8 }, 400*time.Millisecond, 10*time.Millisecond)

	batches := sink.AllTraces()
	require.Len(t, batches, 2, "one batch per service")
	for _, b := range batches {
		require.Equal(t, 1, b.ResourceSpans().Len())
		assert.Equal(t, 2, b.ResourceSpans().At(0).ScopeSpans().Len(), "both scopes preserved within the service")
		assert.Equal(t, 4, b.SpanCount())
	}
}

// benchSpanID returns a span ID distinct across a wide range of indices.
func spanIDAt(i int) pcommon.SpanID {
	var id pcommon.SpanID
	id[0], id[1], id[2] = byte(i), byte(i>>8), 0xC0
	return id
}

// spanIDsIn returns how many times each span ID appears across all batches, so
// that both loss and duplication are visible.
func spanIDCounts(batches []ptrace.Traces) map[pcommon.SpanID]int {
	counts := map[pcommon.SpanID]int{}
	for _, b := range batches {
		for _, rs := range b.ResourceSpans().All() {
			for _, ss := range rs.ScopeSpans().All() {
				for _, s := range ss.Spans().All() {
					counts[s.SpanID()]++
				}
			}
		}
	}
	return counts
}

// One service's spans arriving one per submission must still come out as a
// single subtrace.
func TestSubtrace_ManySubmissionsOneSubtrace(t *testing.T) {
	const spans = 200
	traceID := makeTraceID(64)
	rootID := spanIDAt(1)

	sink := new(consumertest.TracesSink)
	cfg := Config{NumTraces: 1000, NumWorkers: 1, WaitDuration: 2 * time.Second, EmitStrategy: EmitStrategyService}
	p := newSubtraceProcessor(t, cfg, sink)
	defer func() { assert.NoError(t, p.Shutdown(t.Context())) }()

	require.NoError(t, p.ConsumeTraces(t.Context(), buildSpecTrace(traceID, "svc-a",
		spanSpec{id: rootID, parent: pcommon.NewSpanIDEmpty()})))
	for i := 2; i <= spans; i++ {
		require.NoError(t, p.ConsumeTraces(t.Context(), buildSpecTrace(traceID, "svc-a",
			spanSpec{id: spanIDAt(i), parent: rootID})))
	}

	require.Eventually(t, func() bool {
		return sink.SpanCount() == spans
	}, 10*time.Second, 10*time.Millisecond)

	batches := sink.AllTraces()
	require.Len(t, batches, 1, "%d submissions of one service should still be one subtrace", spans)
	counts := spanIDCounts(batches)
	assert.Len(t, counts, spans)
	for id, n := range counts {
		assert.Equal(t, 1, n, "span %v emitted %d times", id, n)
	}
}

// A trace that stays alive across many wait_durations, receiving spans in waves
// long after its entry span was released. Every span must be emitted exactly
// once, and each wave must stay together rather than fragmenting per span.
func TestSubtrace_LongRunningTraceGroupsEachWave(t *testing.T) {
	const (
		waitDuration = 80 * time.Millisecond
		waves        = 6
		perWave      = 8
	)
	traceID := makeTraceID(65)
	rootID := spanIDAt(1)

	sink := new(consumertest.TracesSink)
	cfg := Config{NumTraces: 1000, NumWorkers: 1, WaitDuration: waitDuration, EmitStrategy: EmitStrategyService}
	p := newSubtraceProcessor(t, cfg, sink)

	require.NoError(t, p.ConsumeTraces(t.Context(), buildSpecTrace(traceID, "svc-a",
		spanSpec{id: rootID, parent: pcommon.NewSpanIDEmpty()})))
	require.Eventually(t, func() bool { return sink.SpanCount() == 1 }, 5*time.Second, 5*time.Millisecond)

	// Each wave lands after the previous release, so its spans are parentless:
	// the root they hang off has already left the buffer.
	for w := range waves {
		specs := make([]spanSpec, 0, perWave)
		for i := range perWave {
			specs = append(specs, spanSpec{id: spanIDAt(100 + w*perWave + i), parent: rootID})
		}
		require.NoError(t, p.ConsumeTraces(t.Context(), buildSpecTrace(traceID, "svc-a", specs...)))
		require.Eventually(t, func() bool {
			return sink.SpanCount() == 1+(w+1)*perWave
		}, 5*time.Second, 5*time.Millisecond)
	}

	require.NoError(t, p.Shutdown(t.Context()))

	total := 1 + waves*perWave
	counts := spanIDCounts(sink.AllTraces())
	require.Len(t, counts, total, "every submitted span must be emitted")
	for id, n := range counts {
		require.Equal(t, 1, n, "span %v emitted %d times", id, n)
	}
	// One batch for the root, then one per wave: the wave's spans are all
	// parentless in the same service, so they group instead of fragmenting.
	assert.Len(t, sink.AllTraces(), 1+waves,
		"a long-running trace must not emit one batch per late span")
}

// The strategy across several workers: traces are sharded by trace ID, so each
// must still be assembled completely.
func TestSubtrace_MultipleWorkers(t *testing.T) {
	const (
		traces  = 60
		workers = 4
	)
	sink := new(consumertest.TracesSink)
	cfg := Config{NumTraces: 1000, NumWorkers: workers, WaitDuration: 60 * time.Millisecond, EmitStrategy: EmitStrategyService}
	p := newSubtraceProcessor(t, cfg, sink)

	rootA, childA, rootB := makeSpanID(1), makeSpanID(2), makeSpanID(3)
	for i := 1; i <= traces; i++ {
		tid := makeTraceID(byte(i))
		require.NoError(t, p.ConsumeTraces(t.Context(), buildSpecTrace(tid, "svc-a",
			spanSpec{id: rootA, parent: pcommon.NewSpanIDEmpty()})))
		require.NoError(t, p.ConsumeTraces(t.Context(), buildSpecTrace(tid, "svc-a",
			spanSpec{id: childA, parent: rootA})))
		require.NoError(t, p.ConsumeTraces(t.Context(), buildSpecTrace(tid, "svc-b",
			spanSpec{id: rootB, parent: childA, remote: true})))
	}

	require.Eventually(t, func() bool {
		return sink.SpanCount() == 3*traces
	}, 10*time.Second, 10*time.Millisecond)
	assert.Never(t, func() bool {
		return sink.SpanCount() != 3*traces
	}, 300*time.Millisecond, 10*time.Millisecond)
	require.NoError(t, p.Shutdown(t.Context()))

	// Every trace must be complete, and no span may cross a trace boundary.
	perTrace := map[pcommon.TraceID]int{}
	for _, b := range sink.AllTraces() {
		for _, rs := range b.ResourceSpans().All() {
			for _, ss := range rs.ScopeSpans().All() {
				for _, s := range ss.Spans().All() {
					perTrace[s.TraceID()]++
				}
			}
		}
	}
	require.Len(t, perTrace, traces)
	for tid, n := range perTrace {
		assert.Equal(t, 3, n, "trace %v emitted %d spans", tid, n)
	}
}

// A span resubmitted under a different parent must move to the subtrace its new
// parent belongs to, not stay attached to the old one.
func TestSubtrace_ResubmittedSpanIsReparented(t *testing.T) {
	traceID := makeTraceID(66)
	oldParent, newParent := makeSpanID(1), makeSpanID(2)
	childID := makeSpanID(3)

	sink := new(consumertest.TracesSink)
	cfg := Config{NumTraces: 1000, NumWorkers: 1, WaitDuration: 200 * time.Millisecond, EmitStrategy: EmitStrategyService}
	p := newSubtraceProcessor(t, cfg, sink)
	defer func() { assert.NoError(t, p.Shutdown(t.Context())) }()

	// Two separate services, each a entry span of its own.
	require.NoError(t, p.ConsumeTraces(t.Context(), buildSpecTrace(traceID, "svc-a",
		spanSpec{id: oldParent, parent: pcommon.NewSpanIDEmpty()})))
	require.NoError(t, p.ConsumeTraces(t.Context(), buildSpecTrace(traceID, "svc-b",
		spanSpec{id: newParent, parent: oldParent, remote: true})))
	// The child first appears under svc-a, then is resubmitted under svc-b.
	require.NoError(t, p.ConsumeTraces(t.Context(), buildSpecTrace(traceID, "svc-a",
		spanSpec{id: childID, parent: oldParent})))
	require.NoError(t, p.ConsumeTraces(t.Context(), buildSpecTrace(traceID, "svc-b",
		spanSpec{id: childID, parent: newParent})))

	require.Eventually(t, func() bool { return sink.SpanCount() == 3 }, 5*time.Second, 5*time.Millisecond)
	assert.Never(t, func() bool { return sink.SpanCount() != 3 }, 400*time.Millisecond, 10*time.Millisecond)

	// The child must travel with svc-b, which is where its parent now is.
	for _, b := range sink.AllTraces() {
		ids := batchSpanIDs(b)
		if ids[newParent] {
			assert.True(t, ids[childID], "the resubmitted span should be released with its new parent")
		}
		if ids[oldParent] {
			assert.False(t, ids[childID], "the resubmitted span should have left its old parent's subtrace")
		}
	}
}

// A service entered twice in one trace must be emitted as two batches, so that
// a consumer never sees one batch standing for two calls.
func TestSubtrace_ServiceEnteredTwiceEmitsSeparateBatches(t *testing.T) {
	traceID := makeTraceID(70)
	rootA := makeSpanID(1)
	entryB1, entryB2 := makeSpanID(2), makeSpanID(4)
	viaC := makeSpanID(3)

	sink := new(consumertest.TracesSink)
	cfg := Config{NumTraces: 1000, NumWorkers: 1, WaitDuration: 100 * time.Millisecond, EmitStrategy: EmitStrategyService}
	p := newSubtraceProcessor(t, cfg, sink)
	defer func() { assert.NoError(t, p.Shutdown(t.Context())) }()

	// A -> B -> C -> B
	require.NoError(t, p.ConsumeTraces(t.Context(), buildSpecTrace(traceID, "svc-a",
		spanSpec{id: rootA, parent: pcommon.NewSpanIDEmpty()})))
	require.NoError(t, p.ConsumeTraces(t.Context(), buildSpecTrace(traceID, "svc-b",
		spanSpec{id: entryB1, parent: rootA, remote: true})))
	require.NoError(t, p.ConsumeTraces(t.Context(), buildSpecTrace(traceID, "svc-c",
		spanSpec{id: viaC, parent: entryB1, remote: true})))
	require.NoError(t, p.ConsumeTraces(t.Context(), buildSpecTrace(traceID, "svc-b",
		spanSpec{id: entryB2, parent: viaC, remote: true})))

	require.Eventually(t, func() bool { return sink.SpanCount() == 4 }, 5*time.Second, 5*time.Millisecond)
	assert.Never(t, func() bool { return sink.SpanCount() != 4 }, 400*time.Millisecond, 10*time.Millisecond)

	batches := sink.AllTraces()
	require.Len(t, batches, 4, "svc-b's two entries must not share a batch")
	for _, b := range batches {
		assert.Equal(t, 1, b.SpanCount())
	}
}

// The two calls each keep their own descendants, even when the descendants
// arrive before the entry spans that explain them.
func TestSubtrace_ReEntryKeepsCallsSeparate(t *testing.T) {
	traceID := makeTraceID(71)
	rootA := makeSpanID(1)
	entryB1, childB1 := makeSpanID(2), makeSpanID(3)
	viaC := makeSpanID(4)
	entryB2, childB2 := makeSpanID(5), makeSpanID(6)

	sink := new(consumertest.TracesSink)
	cfg := Config{NumTraces: 1000, NumWorkers: 1, WaitDuration: 250 * time.Millisecond, EmitStrategy: EmitStrategyService}
	p := newSubtraceProcessor(t, cfg, sink)
	defer func() { assert.NoError(t, p.Shutdown(t.Context())) }()

	require.NoError(t, p.ConsumeTraces(t.Context(), buildSpecTrace(traceID, "svc-a",
		spanSpec{id: rootA, parent: pcommon.NewSpanIDEmpty()})))
	// Both services' children arrive first, before anything explains them.
	require.NoError(t, p.ConsumeTraces(t.Context(), buildSpecTrace(traceID, "svc-b",
		spanSpec{id: childB1, parent: entryB1},
		spanSpec{id: childB2, parent: entryB2})))
	require.NoError(t, p.ConsumeTraces(t.Context(), buildSpecTrace(traceID, "svc-b",
		spanSpec{id: entryB1, parent: rootA, remote: true})))
	require.NoError(t, p.ConsumeTraces(t.Context(), buildSpecTrace(traceID, "svc-c",
		spanSpec{id: viaC, parent: childB1, remote: true})))
	require.NoError(t, p.ConsumeTraces(t.Context(), buildSpecTrace(traceID, "svc-b",
		spanSpec{id: entryB2, parent: viaC, remote: true})))

	require.Eventually(t, func() bool { return sink.SpanCount() == 6 }, 5*time.Second, 5*time.Millisecond)
	assert.Never(t, func() bool { return sink.SpanCount() != 6 }, 500*time.Millisecond, 10*time.Millisecond)

	// svc-a and svc-c one batch each; svc-b two, each with its own child.
	batches := sink.AllTraces()
	require.Len(t, batches, 4)
	var svcBCalls []map[pcommon.SpanID]bool
	for _, b := range batches {
		svc, ok := b.ResourceSpans().At(0).Resource().Attributes().Get("service.name")
		require.True(t, ok)
		if svc.AsString() == "svc-b" {
			svcBCalls = append(svcBCalls, batchSpanIDs(b))
		}
	}
	require.Len(t, svcBCalls, 2)
	for _, ids := range svcBCalls {
		assert.True(t,
			maps.Equal(ids, map[pcommon.SpanID]bool{entryB1: true, childB1: true}) ||
				maps.Equal(ids, map[pcommon.SpanID]bool{entryB2: true, childB2: true}),
			"svc-b call %v matched neither expected call", ids)
	}
}

// Malformed input where a span is its own ancestor leaves nothing to head the
// ring. It must still be released on the ordinary timer rather than held until
// eviction or shutdown.
func TestSubtrace_CycleReleasedOnTimer(t *testing.T) {
	traceID := makeTraceID(72)
	x, y := makeSpanID(1), makeSpanID(2)

	sink := new(consumertest.TracesSink)
	cfg := Config{NumTraces: 1000, NumWorkers: 1, WaitDuration: 80 * time.Millisecond, EmitStrategy: EmitStrategyService}
	p := newSubtraceProcessor(t, cfg, sink)

	require.NoError(t, p.ConsumeTraces(t.Context(), buildSpecTrace(traceID, "svc-a",
		spanSpec{id: x, parent: y},
		spanSpec{id: y, parent: x})))

	require.Eventually(t, func() bool { return sink.SpanCount() == 2 }, 5*time.Second, 5*time.Millisecond)
	batches := sink.AllTraces()
	require.Len(t, batches, 1)
	assert.Equal(t, map[pcommon.SpanID]bool{x: true, y: true}, batchSpanIDs(batches[0]))

	// Released, so storage is empty and shutdown has nothing left to drain.
	assert.Empty(t, p.subSt.subtraceIDs())
	require.NoError(t, p.Shutdown(t.Context()))
	assert.Equal(t, 2, sink.SpanCount())
}

// Once a parent turns up, the spans it accounts for stop being treated as
// parentless and are placed by it, while the rest keep their best-effort
// grouping.
func TestSubtrace_ParentArrivesForSomeParentlessSpans(t *testing.T) {
	traceID := makeTraceID(73)
	caller := makeSpanID(1)
	placedA, placedB := makeSpanID(2), makeSpanID(3)
	stillParentless := makeSpanID(4)

	sink := new(consumertest.TracesSink)
	cfg := Config{NumTraces: 1000, NumWorkers: 1, WaitDuration: 250 * time.Millisecond, EmitStrategy: EmitStrategyService}
	p := newSubtraceProcessor(t, cfg, sink)
	defer func() { assert.NoError(t, p.Shutdown(t.Context())) }()

	// Three svc-b spans with no parent in the buffer yet.
	require.NoError(t, p.ConsumeTraces(t.Context(), buildSpecTrace(traceID, "svc-b",
		spanSpec{id: placedA, parent: caller},
		spanSpec{id: placedB, parent: caller},
		spanSpec{id: stillParentless, parent: makeSpanID(99)})))
	// The caller arrives, from another service.
	require.NoError(t, p.ConsumeTraces(t.Context(), buildSpecTrace(traceID, "svc-a",
		spanSpec{id: caller, parent: pcommon.NewSpanIDEmpty()})))

	require.Eventually(t, func() bool { return sink.SpanCount() == 4 }, 5*time.Second, 5*time.Millisecond)
	assert.Never(t, func() bool { return sink.SpanCount() != 4 }, 500*time.Millisecond, 10*time.Millisecond)

	// svc-a alone; placedA and placedB are now separate entry spans into svc-b;
	// the span whose parent never arrived goes out on its own.
	got := map[pcommon.SpanID]bool{}
	for _, b := range sink.AllTraces() {
		ids := batchSpanIDs(b)
		require.Len(t, ids, 1, "each of these belongs to a different call")
		for id := range ids {
			got[id] = true
		}
	}
	assert.Equal(t, map[pcommon.SpanID]bool{
		caller: true, placedA: true, placedB: true, stillParentless: true,
	}, got)
}

// A trace can come back to a service long after it first passed through. The
// later call must wait out its own wait_duration rather than inheriting the
// deadline of the earlier one, or its spans would be cut off partway and split
// across batches.
func TestSubtrace_LaterCallGetsItsOwnWindow(t *testing.T) {
	const waitDuration = 300 * time.Millisecond
	traceID := makeTraceID(74)
	rootA := makeSpanID(1)
	entryB1, childB1 := makeSpanID(2), makeSpanID(3)
	viaC := makeSpanID(4)
	entryB2, childB2 := makeSpanID(5), makeSpanID(6)

	sink := new(consumertest.TracesSink)
	cfg := Config{NumTraces: 1000, NumWorkers: 1, WaitDuration: waitDuration, EmitStrategy: EmitStrategyService}
	p := newSubtraceProcessor(t, cfg, sink)
	defer func() { assert.NoError(t, p.Shutdown(t.Context())) }()

	// svc-b's first call, which starts its timer.
	require.NoError(t, p.ConsumeTraces(t.Context(), buildSpecTrace(traceID, "svc-a",
		spanSpec{id: rootA, parent: pcommon.NewSpanIDEmpty()})))
	require.NoError(t, p.ConsumeTraces(t.Context(), buildSpecTrace(traceID, "svc-b",
		spanSpec{id: entryB1, parent: rootA, remote: true},
		spanSpec{id: childB1, parent: entryB1})))
	require.NoError(t, p.ConsumeTraces(t.Context(), buildSpecTrace(traceID, "svc-c",
		spanSpec{id: viaC, parent: childB1, remote: true})))

	// The trace returns to svc-b late in the first call's window.
	time.Sleep(waitDuration * 2 / 3)
	require.NoError(t, p.ConsumeTraces(t.Context(), buildSpecTrace(traceID, "svc-b",
		spanSpec{id: entryB2, parent: viaC, remote: true})))

	// return the span ID sets of the batches emitted for one service, in
	// the order they were emitted.
	svcSpanIDs := func() []map[pcommon.SpanID]bool {
		var out []map[pcommon.SpanID]bool
		for _, b := range sink.AllTraces() {
			svc, ok := b.ResourceSpans().At(0).Resource().Attributes().Get("service.name")
			if ok && svc.AsString() == "svc-b" {
				out = append(out, batchSpanIDs(b))
			}
		}
		return out
	}

	// The first call goes out on schedule, without the second.
	require.Eventually(t, func() bool {
		return svcSpanIDs() != nil && len(svcSpanIDs()) == 1
	}, 5*time.Second, 5*time.Millisecond)
	assert.Equal(t, map[pcommon.SpanID]bool{entryB1: true, childB1: true}, svcSpanIDs()[0])

	// A span belonging to the second call arrives after the first was released.
	// It must still be waited for, because the second call's own window is open.
	require.NoError(t, p.ConsumeTraces(t.Context(), buildSpecTrace(traceID, "svc-b",
		spanSpec{id: childB2, parent: entryB2})))

	require.Eventually(t, func() bool {
		return len(svcSpanIDs()) == 2
	}, 5*time.Second, 5*time.Millisecond)
	assert.Never(t, func() bool {
		return len(svcSpanIDs()) != 2
	}, 2*waitDuration, 10*time.Millisecond)

	calls := svcSpanIDs()
	assert.Equal(t, map[pcommon.SpanID]bool{entryB2: true, childB2: true}, calls[1],
		"the second call must be released whole, not cut off at the first call's deadline")
}
