// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package groupbytraceprocessor

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/batchpersignal"
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
		onTraces: func(ctx context.Context, received ptrace.Traces) error {
			assert.Equal(t, traces, received)
			wgReceived.Done()
			return nil
		},
	}

	wgDeleted := &sync.WaitGroup{} // we wait for the next (mock) processor to receive the trace
	backing := newMemoryStorage()
	st := &mockStorage{
		onCreateOrAppend: backing.createOrAppend,
		onGet:            backing.get,
		onDelete: func(traceID pcommon.TraceID) ([]ptrace.ResourceSpans, error) {
			wgDeleted.Done()
			return backing.delete(traceID)
		},
	}

	p := newGroupByTraceProcessor(zap.NewNop(), st, mockProcessor, config)
	ctx := context.Background()
	assert.NoError(t, p.Start(ctx, nil))
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
	mockProcessor.onTraces = func(ctx context.Context, received ptrace.Traces) error {
		traceID := received.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).TraceID()
		receivedTraceIDs = append(receivedTraceIDs, traceID)
		wg.Done()
		return nil
	}

	st := newMemoryStorage()

	p := newGroupByTraceProcessor(zap.NewNop(), st, mockProcessor, config)

	ctx := context.Background()
	assert.NoError(t, p.Start(ctx, nil))
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
	assert.Equal(t, 5, len(receivedTraceIDs))

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
	st := newMemoryStorage()
	next := &mockProcessor{}

	// test
	p := newGroupByTraceProcessor(zap.NewNop(), st, next, config)
	caps := p.Capabilities()

	// verify
	assert.NotNil(t, p)
	assert.Equal(t, true, caps.MutatesData)
}

func TestProcessBatchDoesntFail(t *testing.T) {
	// prepare
	config := Config{
		WaitDuration: time.Nanosecond,
		NumTraces:    10,
		NumWorkers:   1,
	}
	st := newMemoryStorage()
	next := &mockProcessor{}

	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4})

	trace := ptrace.NewTraces()
	rs := trace.ResourceSpans().AppendEmpty()
	ils := rs.ScopeSpans().AppendEmpty()
	span := ils.Spans().AppendEmpty()
	span.SetTraceID(traceID)
	span.SetSpanID([8]byte{1, 2, 3, 4})

	p := newGroupByTraceProcessor(zap.NewNop(), st, next, config)
	assert.NotNil(t, p)

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
	next := &mockProcessor{}

	p := newGroupByTraceProcessor(zap.NewNop(), st, next, config)
	require.NotNil(t, p)

	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4})
	batch := simpleTracesWithID(traceID)

	ctx := context.Background()
	assert.NoError(t, p.Start(ctx, nil))
	defer func() {
		assert.NoError(t, p.Shutdown(ctx))
	}()

	err := p.ConsumeTraces(context.Background(), batch)
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
	next := &mockProcessor{}

	p := newGroupByTraceProcessor(zap.NewNop(), st, next, config)
	require.NotNil(t, p)

	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4})
	batch := simpleTracesWithID(traceID)

	ctx := context.Background()
	assert.NoError(t, p.Start(ctx, nil))
	defer func() {
		assert.NoError(t, p.Shutdown(ctx))
	}()

	err := p.ConsumeTraces(context.Background(), batch)
	require.NoError(t, err)

	// test
	// we trigger this manually, instead of waiting the whole duration
	err = p.markAsReleased(traceID, p.eventMachine.workers[workerIndexForTraceID(traceID, config.NumWorkers)].fire)

	// verify
	assert.True(t, errors.Is(err, expectedError))
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
	next := &mockProcessor{}

	p := newGroupByTraceProcessor(zap.NewNop(), st, next, config)
	require.NotNil(t, p)

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
	assert.True(t, errors.Is(err, expectedError))
}

func TestAddSpansToExistingTrace(t *testing.T) {
	// prepare
	wg := &sync.WaitGroup{}
	config := Config{
		WaitDuration: 50 * time.Millisecond,
		NumTraces:    8,
		NumWorkers:   4,
	}
	st := newMemoryStorage()

	var receivedTraces []ptrace.ResourceSpans
	next := &mockProcessor{
		onTraces: func(ctx context.Context, traces ptrace.Traces) error {
			require.Equal(t, 2, traces.ResourceSpans().Len())
			receivedTraces = append(receivedTraces, traces.ResourceSpans().At(0))
			receivedTraces = append(receivedTraces, traces.ResourceSpans().At(1))
			wg.Done()
			return nil
		},
	}

	p := newGroupByTraceProcessor(zap.NewNop(), st, next, config)
	require.NotNil(t, p)

	ctx := context.Background()
	assert.NoError(t, p.Start(ctx, nil))
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

	assert.NoError(t, p.ConsumeTraces(context.Background(), first))
	assert.NoError(t, p.ConsumeTraces(context.Background(), second))

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

	p := newGroupByTraceProcessor(zap.NewNop(), st, next, config)
	require.NotNil(t, p)

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
	assert.True(t, errors.Is(err, expectedError))
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

	p := newGroupByTraceProcessor(zap.NewNop(), st, next, config)
	require.NotNil(t, p)

	traceID := pcommon.TraceID([16]byte{1, 2, 3, 4})

	// test
	err := p.onTraceRemoved(traceID)

	// verify
	assert.True(t, errors.Is(err, expectedError))
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

	p := newGroupByTraceProcessor(zap.NewNop(), st, next, config)
	require.NotNil(t, p)

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
	st := newMemoryStorage()
	next := &mockProcessor{
		onTraces: func(_ context.Context, traces ptrace.Traces) error {
			// we should receive two batches, each one with one trace
			assert.Equal(t, 1, traces.ResourceSpans().Len())
			wg.Done()
			return nil
		},
	}

	p := newGroupByTraceProcessor(zap.NewNop(), st, next, config)
	require.NotNil(t, p)

	ctx := context.Background()
	assert.NoError(t, p.Start(ctx, nil))
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

	p := newGroupByTraceProcessor(zap.NewNop(), st, next, config)
	require.NotNil(t, p)

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

	sp := &groupByTraceProcessor{
		logger:       zap.NewNop(),
		nextConsumer: blocker,
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
	st := newMemoryStorage()

	// For each input trace there are always <= 2 events in the machine simultaneously.
	semaphoreCh := make(chan struct{}, bufferSize/2)
	next := &mockProcessor{onTraces: func(context.Context, ptrace.Traces) error {
		<-semaphoreCh
		return nil
	}}

	p := newGroupByTraceProcessor(zap.NewNop(), st, next, config)
	require.NotNil(b, p)

	ctx := context.Background()
	require.NoError(b, p.Start(ctx, nil))
	defer func() {
		assert.NoError(b, p.Shutdown(ctx))
	}()

	for n := 0; n < b.N; n++ {
		traceID := pcommon.TraceID([16]byte{byte(1 + n), 2, 3, 4})
		trace := simpleTracesWithID(traceID)
		assert.NoError(b, p.ConsumeTraces(context.Background(), trace))
	}
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
func (m *mockProcessor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}
func (m *mockProcessor) Shutdown(context.Context) error {
	return nil
}
func (m *mockProcessor) Start(_ context.Context, _ component.Host) error {
	return nil
}

type mockStorage struct {
	onCreateOrAppend func(pcommon.TraceID, ptrace.Traces) error
	onGet            func(pcommon.TraceID) ([]ptrace.ResourceSpans, error)
	onDelete         func(pcommon.TraceID) ([]ptrace.ResourceSpans, error)
	onStart          func() error
	onShutdown       func() error
}

var _ storage = (*mockStorage)(nil)

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

func (b *blockingConsumer) Capabilities() consumer.Capabilities {
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
