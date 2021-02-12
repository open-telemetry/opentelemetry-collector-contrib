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

package groupbyauthprocessor

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"
)

var (
	logger, _ = zap.NewDevelopment()
)

func TestTraceIsDispatchedAfterDuration(t *testing.T) {
	// prepare
	traces := simpleTraces()

	wgReceived := &sync.WaitGroup{} // we wait for the next (mock) processor to receive the trace
	config := Config{
		WaitDuration: time.Nanosecond,
		NumTraces:    10,
	}
	mockProcessor := &mockProcessor{
		onTraces: func(ctx context.Context, received pdata.Traces) error {
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
		onDelete: func(token string) (pdata.Traces, bool) {
			wgDeleted.Done()
			return backing.delete(token)
		},
	}

	p := newGroupByAuthProcessor(logger, st, mockProcessor, config)

	ctx := context.Background()
	p.Start(ctx, nil)
	defer p.Shutdown(ctx)

	// test
	wgReceived.Add(1) // one should be received
	wgDeleted.Add(1)  // one should be deleted
	p.ConsumeTraces(tokenContext("abc"), traces)

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
	}

	wg.Add(5) // 5 traces are expected to be received

	var receivedTraceIDs []pdata.TraceID
	mockProcessor := &mockProcessor{}
	mockProcessor.onTraces = func(ctx context.Context, received pdata.Traces) error {
		traceID := received.ResourceSpans().At(0).InstrumentationLibrarySpans().At(0).Spans().At(0).TraceID()
		receivedTraceIDs = append(receivedTraceIDs, traceID)
		wg.Done()
		return nil
	}

	st := newMemoryStorage()

	p := newGroupByAuthProcessor(logger, st, mockProcessor, config)

	ctx := context.Background()
	p.Start(ctx, nil)
	defer p.Shutdown(ctx)

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
		batch := simpleTracesWithID(pdata.NewTraceID(traceID))
		p.ConsumeTraces(tokenContext(pdata.NewTraceID(traceID).HexString()), batch)
	}

	wg.Wait()

	// verify
	assert.Equal(t, 5, len(receivedTraceIDs))

	for i := 5; i > 0; i-- { // last 5 traces
		traceID := pdata.NewTraceID(traceIDs[i])
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
	}
	st := newMemoryStorage()
	next := &mockProcessor{}

	// test
	p := newGroupByAuthProcessor(logger, st, next, config)
	caps := p.GetCapabilities()

	// verify
	assert.NotNil(t, p)
	assert.Equal(t, true, caps.MutatesConsumedData)
}

func TestProcessBatchDoesntFail(t *testing.T) {
	// prepare
	config := Config{
		WaitDuration: time.Nanosecond,
		NumTraces:    10,
	}
	st := newMemoryStorage()
	next := &mockProcessor{}

	traceID := pdata.NewTraceID([16]byte{1, 2, 3, 4})

	batch := pdata.NewTraces()
	batch.ResourceSpans().Resize(1)
	trace := batch.ResourceSpans().At(0)
	trace.InstrumentationLibrarySpans().Resize(1)
	ils := trace.InstrumentationLibrarySpans().At(0)
	ils.Spans().Resize(1)
	span := ils.Spans().At(0)
	span.SetTraceID(traceID)
	span.SetSpanID(pdata.NewSpanID([8]byte{1, 2, 3, 4}))

	p := newGroupByAuthProcessor(logger, st, next, config)

	// test
	err := p.onBatchReceived("a2", batch)

	// verify
	assert.NoError(t, err)
	assert.NotNil(t, p)
}

func TestTraceDisappearedFromStorageBeforeReleasing(t *testing.T) {
	// prepare
	config := Config{
		WaitDuration: time.Second, // we are not waiting for this whole time
		NumTraces:    5,
	}
	st := &mockStorage{
		onGet: func(string) (pdata.Traces, bool) {
			return pdata.Traces{}, false
		},
	}
	next := &mockProcessor{}

	p := newGroupByAuthProcessor(logger, st, next, config)
	require.NotNil(t, p)

	traceID := pdata.NewTraceID([16]byte{1, 2, 3, 4})
	batch := simpleTracesWithID(traceID)

	ctx := context.Background()
	p.Start(ctx, nil)
	defer p.Shutdown(ctx)

	err := p.ConsumeTraces(tokenContext("a1"), batch)
	require.NoError(t, err)

	// test
	// we trigger this manually, instead of waiting the whole duration
	err = p.markAsReleased("a1")

	// verify
	assert.Error(t, err)
}

func TestTraceErrorFromStorageWhileReleasing(t *testing.T) {
	// prepare
	config := Config{
		WaitDuration: time.Second, // we are not waiting for this whole time
		NumTraces:    5,
	}
	st := &mockStorage{
		onGet: func(string) (pdata.Traces, bool) {
			return pdata.Traces{}, false
		},
	}
	next := &mockProcessor{}

	p := newGroupByAuthProcessor(logger, st, next, config)
	require.NotNil(t, p)

	traceID := pdata.NewTraceID([16]byte{1, 2, 3, 4})
	batch := simpleTracesWithID(traceID)

	ctx := context.Background()
	p.Start(ctx, nil)
	defer p.Shutdown(ctx)

	tokenCtx := client.NewContext(context.Background(), &client.Client{
		Token: "token-123",
	})
	err := p.ConsumeTraces(tokenCtx, batch)
	require.NoError(t, err)

	// test
	// we trigger this manually, instead of waiting the whole duration
	err = p.markAsReleased("token-123")

	// verify
	assert.Error(t, err)
}

func TestAddSpansToExistingTrace(t *testing.T) {
	// prepare
	wg := &sync.WaitGroup{}
	config := Config{
		WaitDuration: 50 * time.Millisecond,
		NumTraces:    5,
	}
	st := newMemoryStorage()

	var receivedTraces []pdata.ResourceSpans
	next := &mockProcessor{
		onTraces: func(ctx context.Context, traces pdata.Traces) error {
			require.Equal(t, 2, traces.ResourceSpans().Len())
			receivedTraces = append(receivedTraces, traces.ResourceSpans().At(0))
			receivedTraces = append(receivedTraces, traces.ResourceSpans().At(1))
			wg.Done()
			return nil
		},
	}

	p := newGroupByAuthProcessor(logger, st, next, config)
	require.NotNil(t, p)

	ctx := context.Background()
	p.Start(ctx, nil)
	defer p.Shutdown(ctx)

	traceID := pdata.NewTraceID([16]byte{1, 2, 3, 4})

	// test
	first := simpleTracesWithID(traceID)
	first.ResourceSpans().At(0).InstrumentationLibrarySpans().At(0).Spans().At(0).SetName("first-span")

	second := simpleTracesWithID(traceID)
	second.ResourceSpans().At(0).InstrumentationLibrarySpans().At(0).Spans().At(0).SetName("second-span")

	wg.Add(1)

	p.ConsumeTraces(tokenContext("abc"), first)
	p.ConsumeTraces(tokenContext("abc"), second)

	wg.Wait()

	// verify
	assert.Len(t, receivedTraces, 2)
}

func TestTraceNotFoundWhileRemovingTrace(t *testing.T) {
	// prepare
	config := Config{
		WaitDuration: time.Second, // we are not waiting for this whole time
		NumTraces:    5,
	}
	st := &mockStorage{
		onDelete: func(string) (pdata.Traces, bool) {
			return pdata.Traces{}, false
		},
	}
	next := &mockProcessor{}

	p := newGroupByAuthProcessor(logger, st, next, config)
	require.NotNil(t, p)

	// test
	err := p.onTokenRemoved("no-here")

	// verify
	assert.Error(t, err)
}

func TestTokensAreDispatchedInIndividualBatches(t *testing.T) {
	// prepare
	wg := &sync.WaitGroup{}

	config := Config{
		WaitDuration: time.Nanosecond, // we are not waiting for this whole time
		NumTraces:    5,
	}
	st := newMemoryStorage()
	next := &mockProcessor{
		onTraces: func(_ context.Context, traces pdata.Traces) error {
			// we should receive two batches, each one with one trace
			assert.Equal(t, 1, traces.ResourceSpans().Len())
			wg.Done()
			return nil
		},
	}

	p := newGroupByAuthProcessor(logger, st, next, config)
	require.NotNil(t, p)

	ctx := context.Background()
	p.Start(ctx, nil)
	defer p.Shutdown(ctx)

	// test
	wg.Add(2)

	err := p.onBatchReceived("a1", simpleTraces())
	require.NoError(t, err)

	err = p.onBatchReceived("a2", simpleTraces())
	require.NoError(t, err)

	wg.Wait()

	// verify
	// verification is done at onTraces from the mockProcessor
}

func BenchmarkConsumeTracesCompleteOnFirstBatch(b *testing.B) {
	// prepare
	config := Config{
		WaitDuration: 50 * time.Millisecond,
		NumTraces:    defaultNumTraces,
	}
	st := newMemoryStorage()
	next := &mockProcessor{}

	p := newGroupByAuthProcessor(zap.NewNop(), st, next, config)
	require.NotNil(b, p)

	ctx := context.Background()
	p.Start(ctx, nil)
	defer p.Shutdown(ctx)

	for n := 0; n < b.N; n++ {
		traceID := pdata.NewTraceID([16]byte{byte(1 + n), 2, 3, 4})
		trace := simpleTracesWithID(traceID)
		token := fmt.Sprintf("abc-%d", n%100)
		p.ConsumeTraces(tokenContext(token), trace)
	}
}

type mockProcessor struct {
	onTraces func(context.Context, pdata.Traces) error
}

var _ component.TracesProcessor = (*mockProcessor)(nil)

func tokenContext(token string) context.Context {
	return client.NewContext(context.Background(), &client.Client{
		Token: token,
	})
}

func (m *mockProcessor) ConsumeTraces(ctx context.Context, td pdata.Traces) error {
	if m.onTraces != nil {
		return m.onTraces(ctx, td)
	}
	return nil
}
func (m *mockProcessor) GetCapabilities() component.ProcessorCapabilities {
	return component.ProcessorCapabilities{MutatesConsumedData: true}
}
func (m *mockProcessor) Shutdown(context.Context) error {
	return nil
}
func (m *mockProcessor) Start(_ context.Context, _ component.Host) error {
	return nil
}

type mockStorage struct {
	onCreateOrAppend func(string, pdata.Traces) error
	onGet            func(string) (pdata.Traces, bool)
	onDelete         func(string) (pdata.Traces, bool)
	onStart          func() error
	onShutdown       func() error
}

var _ storage = (*mockStorage)(nil)

func (st *mockStorage) createOrAppend(token string, traces pdata.Traces) error {
	if st.onCreateOrAppend != nil {
		return st.onCreateOrAppend(token, traces)
	}
	return nil
}
func (st *mockStorage) get(token string) (pdata.Traces, bool) {
	if st.onGet != nil {
		return st.onGet(token)
	}
	return pdata.Traces{}, false
}
func (st *mockStorage) delete(token string) (pdata.Traces, bool) {
	if st.onDelete != nil {
		return st.onDelete(token)
	}
	return pdata.Traces{}, false
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

func simpleTraces() pdata.Traces {
	return simpleTracesWithID(pdata.NewTraceID([16]byte{1, 2, 3, 4}))
}

func simpleTracesWithID(traceID pdata.TraceID) pdata.Traces {
	traces := pdata.NewTraces()
	traces.ResourceSpans().Resize(1)
	rs := traces.ResourceSpans().At(0)
	rs.InstrumentationLibrarySpans().Resize(1)
	ils := rs.InstrumentationLibrarySpans().At(0)
	ils.Spans().Resize(1)
	ils.Spans().At(0).SetTraceID(traceID)
	return traces
}
