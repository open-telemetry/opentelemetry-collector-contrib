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

// nolint:errcheck
package logs

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
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/batchpersignal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/groupbytraceprocessor/internal/common"
)

const (
	defaultNumTraces  = 1_000_000
	defaultNumWorkers = 1
)

func TestLogIsDispatchedAfterDuration(t *testing.T) {
	// prepare
	logs := simpleLogs()

	wgReceived := &sync.WaitGroup{} // we wait for the next (mock) processor to receive the trace
	config := common.Config{
		WaitDuration: time.Nanosecond,
		NumTraces:    10,
		NumWorkers:   4,
	}
	mockProcessor := &mockProcessor{
		onLogs: func(ctx context.Context, received plog.Logs) error {
			assert.Equal(t, logs, received)
			wgReceived.Done()
			return nil
		},
	}

	wgDeleted := &sync.WaitGroup{} // we wait for the next (mock) processor to receive the trace
	backing := NewMemoryStorage()
	st := &mockStorage{
		onCreateOrAppend: backing.createOrAppend,
		onGet:            backing.get,
		onDelete: func(traceID pcommon.TraceID) ([]plog.ResourceLogs, error) {
			wgDeleted.Done()
			return backing.delete(traceID)
		},
	}

	p := NewGroupByTraceProcessor(zap.NewNop(), st, mockProcessor, config)
	ctx := context.Background()
	assert.NoError(t, p.Start(ctx, nil))
	defer func() {
		assert.NoError(t, p.Shutdown(ctx))
	}()

	// test
	wgReceived.Add(1) // one should be received
	wgDeleted.Add(1)  // one should be deleted
	assert.NoError(t, p.ConsumeLogs(ctx, logs))

	// verify
	wgReceived.Wait()
	wgDeleted.Wait()
}

func TestInternalCacheLimit(t *testing.T) {
	// prepare
	wg := &sync.WaitGroup{} // we wait for the next (mock) processor to receive the trace

	config := common.Config{
		// should be long enough for the test to run without logs being finished, but short enough to not
		// badly influence the testing experience
		WaitDuration: 50 * time.Millisecond,

		// we create 6 logs, only 5 should be at the Storage in the end
		NumTraces: 5,

		NumWorkers: 1,
	}

	wg.Add(5) // 5 logs are expected to be received

	var receivedTraceIDs []pcommon.TraceID
	mockProcessor := &mockProcessor{}
	mockProcessor.onLogs = func(ctx context.Context, received plog.Logs) error {
		traceID := received.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).TraceID()
		receivedTraceIDs = append(receivedTraceIDs, traceID)
		wg.Done()
		return nil
	}

	st := NewMemoryStorage()

	p := NewGroupByTraceProcessor(zap.NewNop(), st, mockProcessor, config)

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
		batch := simpleLogsWithID(pcommon.NewTraceID(traceID))
		assert.NoError(t, p.ConsumeLogs(ctx, batch))
	}

	wg.Wait()

	// verify
	assert.Equal(t, 5, len(receivedTraceIDs))

	for i := 5; i > 0; i-- { // last 5 logs
		traceID := pcommon.NewTraceID(traceIDs[i])
		assert.Contains(t, receivedTraceIDs, traceID)
	}

	// the first trace should have been evicted
	assert.NotContains(t, receivedTraceIDs, traceIDs[0])
}

func TestProcessorCapabilities(t *testing.T) {
	// prepare
	config := common.Config{
		WaitDuration: time.Nanosecond,
		NumTraces:    10,
		NumWorkers:   1,
	}
	st := NewMemoryStorage()
	next := &mockProcessor{}

	// test
	p := NewGroupByTraceProcessor(zap.NewNop(), st, next, config)
	caps := p.Capabilities()

	// verify
	assert.NotNil(t, p)
	assert.Equal(t, true, caps.MutatesData)
}

func TestProcessBatchDoesntFail(t *testing.T) {
	// prepare
	config := common.Config{
		WaitDuration: time.Nanosecond,
		NumTraces:    10,
		NumWorkers:   1,
	}
	st := NewMemoryStorage()
	next := &mockProcessor{}

	traceID := pcommon.NewTraceID([16]byte{1, 2, 3, 4})

	log := plog.NewLogs()
	rs := log.ResourceLogs().AppendEmpty()
	ils := rs.ScopeLogs().AppendEmpty()
	span := ils.LogRecords().AppendEmpty()
	span.SetTraceID(traceID)
	span.SetSpanID(pcommon.NewSpanID([8]byte{1, 2, 3, 4}))

	p := NewGroupByTraceProcessor(zap.NewNop(), st, next, config)
	assert.NotNil(t, p)

	// test
	assert.NoError(t, p.onLogReceived(logsWithID{id: traceID, td: log}, p.eventMachine.workers[0]))
}

func TestLogDisappearedFromStorageBeforeReleasing(t *testing.T) {
	// prepare
	config := common.Config{
		WaitDuration: time.Second, // we are not waiting for this whole time
		NumTraces:    8,
		NumWorkers:   4,
	}
	st := &mockStorage{
		onGet: func(pcommon.TraceID) ([]plog.ResourceLogs, error) {
			return nil, nil
		},
	}
	next := &mockProcessor{}

	p := NewGroupByTraceProcessor(zap.NewNop(), st, next, config)
	require.NotNil(t, p)

	traceID := pcommon.NewTraceID([16]byte{1, 2, 3, 4})
	batch := simpleLogsWithID(traceID)

	ctx := context.Background()
	assert.NoError(t, p.Start(ctx, nil))
	defer func() {
		assert.NoError(t, p.Shutdown(ctx))
	}()

	err := p.ConsumeLogs(context.Background(), batch)
	require.NoError(t, err)

	// test
	// we trigger this manually, instead of waiting the whole duration
	err = p.markAsReleased(traceID, p.eventMachine.workers[workerIndexForTraceID(traceID, config.NumWorkers)].fire)

	// verify
	assert.Error(t, err)
}

func TestLogErrorFromStorageWhileReleasing(t *testing.T) {
	// prepare
	config := common.Config{
		WaitDuration: time.Second, // we are not waiting for this whole time
		NumTraces:    8,
		NumWorkers:   4,
	}
	expectedError := errors.New("some unexpected error")
	st := &mockStorage{
		onGet: func(pcommon.TraceID) ([]plog.ResourceLogs, error) {
			return nil, expectedError
		},
	}
	next := &mockProcessor{}

	p := NewGroupByTraceProcessor(zap.NewNop(), st, next, config)
	require.NotNil(t, p)

	traceID := pcommon.NewTraceID([16]byte{1, 2, 3, 4})
	batch := simpleLogsWithID(traceID)

	ctx := context.Background()
	assert.NoError(t, p.Start(ctx, nil))
	defer func() {
		assert.NoError(t, p.Shutdown(ctx))
	}()

	err := p.ConsumeLogs(context.Background(), batch)
	require.NoError(t, err)

	// test
	// we trigger this manually, instead of waiting the whole duration
	err = p.markAsReleased(traceID, p.eventMachine.workers[workerIndexForTraceID(traceID, config.NumWorkers)].fire)

	// verify
	assert.True(t, errors.Is(err, expectedError))
}

func TestLogErrorFromStorageWhileProcessingLog(t *testing.T) {
	// prepare
	config := common.Config{
		WaitDuration: time.Second, // we are not waiting for this whole time
		NumTraces:    5,
		NumWorkers:   1,
	}
	expectedError := errors.New("some unexpected error")
	st := &mockStorage{
		onCreateOrAppend: func(pcommon.TraceID, plog.Logs) error {
			return expectedError
		},
	}
	next := &mockProcessor{}

	p := NewGroupByTraceProcessor(zap.NewNop(), st, next, config)
	require.NotNil(t, p)

	traceID := pcommon.NewTraceID([16]byte{1, 2, 3, 4})

	log := plog.NewLogs()
	rs := log.ResourceLogs().AppendEmpty()
	ils := rs.ScopeLogs().AppendEmpty()
	span := ils.LogRecords().AppendEmpty()
	span.SetTraceID(traceID)
	span.SetSpanID(pcommon.NewSpanID([8]byte{1, 2, 3, 4}))

	batch := batchpersignal.SplitLogs(log)

	// test
	err := p.onLogReceived(logsWithID{id: traceID, td: batch[0]}, p.eventMachine.workers[0])

	// verify
	assert.True(t, errors.Is(err, expectedError))
}

func TestAddLogRecordsToExistingLog(t *testing.T) {
	// prepare
	wg := &sync.WaitGroup{}
	config := common.Config{
		WaitDuration: 50 * time.Millisecond,
		NumTraces:    8,
		NumWorkers:   4,
	}
	st := NewMemoryStorage()

	var receivedLogs []plog.ResourceLogs
	next := &mockProcessor{
		onLogs: func(ctx context.Context, logs plog.Logs) error {
			require.Equal(t, 2, logs.ResourceLogs().Len())
			receivedLogs = append(receivedLogs, logs.ResourceLogs().At(0))
			receivedLogs = append(receivedLogs, logs.ResourceLogs().At(1))
			wg.Done()
			return nil
		},
	}

	p := NewGroupByTraceProcessor(zap.NewNop(), st, next, config)
	require.NotNil(t, p)

	ctx := context.Background()
	assert.NoError(t, p.Start(ctx, nil))
	defer func() {
		assert.NoError(t, p.Shutdown(ctx))
	}()

	traceID := pcommon.NewTraceID([16]byte{1, 2, 3, 4})

	// test
	first := simpleLogsWithID(traceID)
	first.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().SetStringVal("first-log")

	second := simpleLogsWithID(traceID)
	second.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().SetStringVal("second-log")

	wg.Add(1)

	assert.NoError(t, p.ConsumeLogs(context.Background(), first))
	assert.NoError(t, p.ConsumeLogs(context.Background(), second))

	wg.Wait()

	// verify
	assert.Len(t, receivedLogs, 2)
}

func TestLogeErrorFromStorageWhileProcessingSecondLog(t *testing.T) {
	// prepare
	config := common.Config{
		WaitDuration: time.Second, // we are not waiting for this whole time
		NumTraces:    8,
		NumWorkers:   4,
	}
	st := &mockStorage{}
	next := &mockProcessor{}

	p := NewGroupByTraceProcessor(zap.NewNop(), st, next, config)
	require.NotNil(t, p)

	traceID := pcommon.NewTraceID([16]byte{1, 2, 3, 4})

	log := plog.NewLogs()
	rss := log.ResourceLogs()
	rs := rss.AppendEmpty()
	ils := rs.ScopeLogs().AppendEmpty()
	span := ils.LogRecords().AppendEmpty()
	span.SetTraceID(traceID)
	span.SetSpanID(pcommon.NewSpanID([8]byte{1, 2, 3, 4}))

	batch := batchpersignal.SplitLogs(log)

	// test
	err := p.eventMachine.consume(batch[0])
	assert.NoError(t, err)

	expectedError := errors.New("some unexpected error")
	st.onCreateOrAppend = func(pcommon.TraceID, plog.Logs) error {
		return expectedError
	}

	// processing another batch for the same log takes a slightly different code path
	err = p.onLogReceived(logsWithID{id: traceID, td: batch[0]},
		p.eventMachine.workers[workerIndexForTraceID(traceID, config.NumWorkers)],
	)

	// verify
	assert.True(t, errors.Is(err, expectedError))
}

func TestErrorFromStorageWhileRemovingLog(t *testing.T) {
	// prepare
	config := common.Config{
		WaitDuration: time.Second, // we are not waiting for this whole time
		NumTraces:    8,
		NumWorkers:   4,
	}
	expectedError := errors.New("some unexpected error")
	st := &mockStorage{
		onDelete: func(pcommon.TraceID) ([]plog.ResourceLogs, error) {
			return nil, expectedError
		},
	}
	next := &mockProcessor{}

	p := NewGroupByTraceProcessor(zap.NewNop(), st, next, config)
	require.NotNil(t, p)

	traceID := pcommon.NewTraceID([16]byte{1, 2, 3, 4})

	// test
	err := p.onLogRemoved(traceID)

	// verify
	assert.True(t, errors.Is(err, expectedError))
}

func TestLogNotFoundWhileRemovingLog(t *testing.T) {
	// prepare
	config := common.Config{
		WaitDuration: time.Second, // we are not waiting for this whole time
		NumTraces:    8,
		NumWorkers:   4,
	}
	st := &mockStorage{
		onDelete: func(pcommon.TraceID) ([]plog.ResourceLogs, error) {
			return nil, nil
		},
	}
	next := &mockProcessor{}

	p := NewGroupByTraceProcessor(zap.NewNop(), st, next, config)
	require.NotNil(t, p)

	traceID := pcommon.NewTraceID([16]byte{1, 2, 3, 4})

	// test
	err := p.onLogRemoved(traceID)

	// verify
	assert.Error(t, err)
}

func TestLogsAreDispatchedInIndividualBatches(t *testing.T) {
	// prepare
	wg := &sync.WaitGroup{}

	config := common.Config{
		WaitDuration: time.Nanosecond, // we are not waiting for this whole time
		NumTraces:    8,
		NumWorkers:   4,
	}
	st := NewMemoryStorage()
	next := &mockProcessor{
		onLogs: func(_ context.Context, logs plog.Logs) error {
			// we should receive two batches, each one with one trace
			assert.Equal(t, 1, logs.ResourceLogs().Len())
			wg.Done()
			return nil
		},
	}

	p := NewGroupByTraceProcessor(zap.NewNop(), st, next, config)
	require.NotNil(t, p)

	ctx := context.Background()
	assert.NoError(t, p.Start(ctx, nil))
	defer func() {
		assert.NoError(t, p.Shutdown(ctx))
	}()

	traceID := pcommon.NewTraceID([16]byte{1, 2, 3, 4})

	firstLog := plog.NewLogs()
	firstRss := firstLog.ResourceLogs()
	firstResourceLogs := firstRss.AppendEmpty()
	ils := firstResourceLogs.ScopeLogs().AppendEmpty()
	span := ils.LogRecords().AppendEmpty()
	span.SetTraceID(traceID)

	secondTraceID := pcommon.NewTraceID([16]byte{2, 3, 4, 5})
	secondLog := plog.NewLogs()
	secondRss := secondLog.ResourceLogs()
	secondResourceLogs := secondRss.AppendEmpty()
	secondIls := secondResourceLogs.ScopeLogs().AppendEmpty()
	secondSpan := secondIls.LogRecords().AppendEmpty()
	secondSpan.SetTraceID(secondTraceID)

	// test
	wg.Add(2)

	assert.NoError(t, p.eventMachine.consume(firstLog))
	assert.NoError(t, p.eventMachine.consume(secondLog))

	wg.Wait()

	// verify
	// verification is done at onLogs from the mockProcessor
}

func TestErrorOnProcessResourceLogsContinuesProcessing(t *testing.T) {
	// prepare
	config := common.Config{
		WaitDuration: time.Second, // we are not waiting for this whole time
		NumTraces:    8,
		NumWorkers:   4,
	}
	st := &mockStorage{}
	next := &mockProcessor{}

	p := NewGroupByTraceProcessor(zap.NewNop(), st, next, config)
	require.NotNil(t, p)

	traceID := pcommon.NewTraceID([16]byte{1, 2, 3, 4})

	log := plog.NewLogs()
	rss := log.ResourceLogs()
	rs := rss.AppendEmpty()
	ils := rs.ScopeLogs().AppendEmpty()
	span := ils.LogRecords().AppendEmpty()
	span.SetTraceID(traceID)
	span.SetSpanID(pcommon.NewSpanID([8]byte{1, 2, 3, 4}))

	expectedError := errors.New("some unexpected error")
	returnedError := false
	st.onCreateOrAppend = func(pcommon.TraceID, plog.Logs) error {
		returnedError = true
		return expectedError
	}

	// test
	assert.Error(t, p.onLogReceived(logsWithID{id: traceID, td: log}, p.eventMachine.workers[0]))

	// verify
	assert.True(t, returnedError)
}

func TestAsyncOnRelease(t *testing.T) {
	blockCh := make(chan struct{})
	blocker := &blockingConsumer{
		blockCh: blockCh,
	}

	sp := &GroupByTraceProcessor{
		logger:       zap.NewNop(),
		nextConsumer: blocker,
	}
	assert.NoError(t, sp.onLogReleased(nil))
	close(blockCh)
}

func BenchmarkConsumeLogsCompleteOnFirstBatch(b *testing.B) {
	// prepare
	config := common.Config{
		WaitDuration: 50 * time.Millisecond,
		NumTraces:    defaultNumTraces,
		NumWorkers:   4 * defaultNumWorkers,
	}
	st := NewMemoryStorage()

	// For each input log there are always <= 2 events in the machine simultaneously.
	semaphoreCh := make(chan struct{}, bufferSize/2)
	next := &mockProcessor{onLogs: func(context.Context, plog.Logs) error {
		<-semaphoreCh
		return nil
	}}

	p := NewGroupByTraceProcessor(zap.NewNop(), st, next, config)
	require.NotNil(b, p)

	ctx := context.Background()
	require.NoError(b, p.Start(ctx, nil))
	defer func() {
		assert.NoError(b, p.Shutdown(ctx))
	}()

	for n := 0; n < b.N; n++ {
		traceID := pcommon.NewTraceID([16]byte{byte(1 + n), 2, 3, 4})
		log := simpleLogsWithID(traceID)
		assert.NoError(b, p.ConsumeLogs(context.Background(), log))
	}
}

type mockProcessor struct {
	mutex  sync.Mutex
	onLogs func(context.Context, plog.Logs) error
}

var _ component.LogsProcessor = (*mockProcessor)(nil)

func (m *mockProcessor) ConsumeLogs(ctx context.Context, td plog.Logs) error {
	if m.onLogs != nil {
		m.mutex.Lock()
		defer m.mutex.Unlock()
		return m.onLogs(ctx, td)
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
	onCreateOrAppend func(pcommon.TraceID, plog.Logs) error
	onGet            func(pcommon.TraceID) ([]plog.ResourceLogs, error)
	onDelete         func(pcommon.TraceID) ([]plog.ResourceLogs, error)
	onStart          func() error
	onShutdown       func() error
}

var _ Storage = (*mockStorage)(nil)

func (st *mockStorage) createOrAppend(traceID pcommon.TraceID, trace plog.Logs) error {
	if st.onCreateOrAppend != nil {
		return st.onCreateOrAppend(traceID, trace)
	}
	return nil
}
func (st *mockStorage) get(traceID pcommon.TraceID) ([]plog.ResourceLogs, error) {
	if st.onGet != nil {
		return st.onGet(traceID)
	}
	return nil, nil
}
func (st *mockStorage) delete(traceID pcommon.TraceID) ([]plog.ResourceLogs, error) {
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

var _ consumer.Logs = (*blockingConsumer)(nil)

func (b *blockingConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}
func (b *blockingConsumer) ConsumeLogs(context.Context, plog.Logs) error {
	<-b.blockCh
	return nil
}

func simpleLogs() plog.Logs {
	return simpleLogsWithID(pcommon.NewTraceID([16]byte{1, 2, 3, 4}))
}

func simpleLogsWithID(traceID pcommon.TraceID) plog.Logs {
	logs := plog.NewLogs()
	rs := logs.ResourceLogs().AppendEmpty()
	ils := rs.ScopeLogs().AppendEmpty()
	ils.LogRecords().AppendEmpty().SetTraceID(traceID)
	return logs
}
