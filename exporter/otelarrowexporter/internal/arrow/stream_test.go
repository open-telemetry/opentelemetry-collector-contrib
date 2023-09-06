// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package arrow

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	arrowpb "github.com/open-telemetry/otel-arrow/api/experimental/arrow/v1"
	arrowRecordMock "github.com/open-telemetry/otel-arrow/pkg/otel/arrow_record/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"go.opentelemetry.io/collector/consumer/consumererror"
)

var oneBatch = &arrowpb.BatchArrowRecords{
	BatchId: 1,
}

type streamTestCase struct {
	*commonTestCase
	*commonTestStream

	producer        *arrowRecordMock.MockProducerAPI
	prioritizer     *streamPrioritizer
	bgctx           context.Context
	bgcancel        context.CancelFunc
	fromTracesCall  *gomock.Call
	fromMetricsCall *gomock.Call
	fromLogsCall    *gomock.Call
	stream          *Stream
	wait            sync.WaitGroup
}

func newStreamTestCase(t *testing.T) *streamTestCase {
	ctrl := gomock.NewController(t)
	producer := arrowRecordMock.NewMockProducerAPI(ctrl)

	bg, cancel := context.WithCancel(context.Background())
	prio := newStreamPrioritizer(bg, 1)

	ctc := newCommonTestCase(t, NotNoisy)
	cts := ctc.newMockStream(bg)

	// metadata functionality is tested in exporter_test.go
	ctc.requestMetadataCall.AnyTimes().Return(nil, nil)

	stream := newStream(producer, prio, ctc.telset, ctc.perRPCCredentials)
	stream.maxStreamLifetime = 10 * time.Second

	fromTracesCall := producer.EXPECT().BatchArrowRecordsFromTraces(gomock.Any()).Times(0)
	fromMetricsCall := producer.EXPECT().BatchArrowRecordsFromMetrics(gomock.Any()).Times(0)
	fromLogsCall := producer.EXPECT().BatchArrowRecordsFromLogs(gomock.Any()).Times(0)

	return &streamTestCase{
		commonTestCase:   ctc,
		commonTestStream: cts,
		producer:         producer,
		prioritizer:      prio,
		bgctx:            bg,
		bgcancel:         cancel,
		stream:           stream,
		fromTracesCall:   fromTracesCall,
		fromMetricsCall:  fromMetricsCall,
		fromLogsCall:     fromLogsCall,
	}
}

// start runs a test stream according to the behavior of testChannel.
func (tc *streamTestCase) start(channel testChannel) {
	tc.streamCall.Times(1).DoAndReturn(tc.connectTestStream(channel))

	tc.wait.Add(1)

	go func() {
		defer tc.wait.Done()
		tc.stream.run(tc.bgctx, MakeAnyStreamClient(tc.streamClient), nil)
	}()
}

// cancelAndWait cancels the context and waits for the runner to return.
func (tc *streamTestCase) cancelAndWaitForShutdown() {
	tc.bgcancel()
	tc.wait.Wait()
}

// cancel waits for the runner to exit without canceling the context.
func (tc *streamTestCase) waitForShutdown() {
	tc.wait.Wait()
}

// connectTestStream returns the stream under test from the common test's mock ArrowStream().
func (tc *streamTestCase) connectTestStream(h testChannel) func(context.Context, ...grpc.CallOption) (
	arrowpb.ArrowStreamService_ArrowStreamClient,
	error,
) {
	return func(ctx context.Context, _ ...grpc.CallOption) (
		arrowpb.ArrowStreamService_ArrowStreamClient,
		error,
	) {
		if err := h.onConnect(ctx); err != nil {
			return nil, err
		}
		tc.sendCall.AnyTimes().DoAndReturn(h.onSend(ctx))
		tc.recvCall.AnyTimes().DoAndReturn(h.onRecv(ctx))
		return tc.anyStreamClient, nil
	}
}

// get returns the stream via the prioritizer it is registered with.
func (tc *streamTestCase) get() *Stream {
	return <-tc.prioritizer.readyChannel()
}

// TestStreamEncodeError verifies that exceeding the
// max_stream_lifetime results in shutdown that
// simply restarts the stream.
func TestStreamGracefulShutdown(t *testing.T) {
	tc := newStreamTestCase(t)
	maxStreamLifetime := 1 * time.Second
	tc.stream.maxStreamLifetime = maxStreamLifetime

	tc.fromTracesCall.Times(1).Return(oneBatch, nil)
	tc.closeSendCall.Times(1).Return(nil)

	channel := newHealthyTestChannel()
	tc.start(channel)
	defer tc.cancelAndWaitForShutdown()
	var wg sync.WaitGroup
	wg.Add(1)
	defer wg.Wait()
	go func() {
		defer wg.Done()
		batch := <-channel.sent
		channel.recv <- statusOKFor(batch.BatchId)

		// mimick the server which will send a batchID
		// of 0 after max_stream_lifetime elapses.
		time.Sleep(maxStreamLifetime)
		channel.recv <- statusCanceledFor(0)
	}()

	err := tc.get().SendAndWait(tc.bgctx, twoTraces)
	require.NoError(t, err)

	// need to sleep so CloseSend will be called.
	time.Sleep(maxStreamLifetime)
	err = tc.get().SendAndWait(tc.bgctx, twoTraces)
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrStreamRestarting))
}

// TestStreamNoMaxLifetime verifies that configuring
// max_stream_lifetime==0 works and the client never
// calls CloseSend().
func TestStreamNoMaxLifetime(t *testing.T) {
	tc := newStreamTestCase(t)
	tc.stream.maxStreamLifetime = 0

	tc.fromTracesCall.Times(1).Return(oneBatch, nil)
	tc.closeSendCall.Times(0)

	channel := newHealthyTestChannel()
	tc.start(channel)
	defer tc.cancelAndWaitForShutdown()
	var wg sync.WaitGroup
	wg.Add(1)
	defer wg.Wait()
	go func() {
		defer wg.Done()
		batch := <-channel.sent
		channel.recv <- statusOKFor(batch.BatchId)
	}()

	err := tc.get().SendAndWait(tc.bgctx, twoTraces)
	require.NoError(t, err)
}

// TestStreamEncodeError verifies that an encoder error in the sender
// yields a permanent error.
func TestStreamEncodeError(t *testing.T) {
	tc := newStreamTestCase(t)

	testErr := fmt.Errorf("test encode error")
	tc.fromTracesCall.Times(1).Return(nil, testErr)

	tc.start(newHealthyTestChannel())
	defer tc.cancelAndWaitForShutdown()

	// sender should get a permanent testErr
	err := (<-tc.prioritizer.readyChannel()).SendAndWait(tc.bgctx, twoTraces)
	require.Error(t, err)
	require.True(t, errors.Is(err, testErr))
	require.True(t, consumererror.IsPermanent(err))
}

// TestStreamUnknownBatchError verifies that the stream reader handles
// a unknown BatchID.
func TestStreamUnknownBatchError(t *testing.T) {
	tc := newStreamTestCase(t)

	tc.fromTracesCall.Times(1).Return(oneBatch, nil)

	channel := newHealthyTestChannel()
	tc.start(channel)
	defer tc.cancelAndWaitForShutdown()

	var wg sync.WaitGroup
	wg.Add(1)
	defer wg.Wait()
	go func() {
		defer wg.Done()
		<-channel.sent
		channel.recv <- statusOKFor(-1 /*unknown*/)
	}()
	// sender should get ErrStreamRestarting
	err := tc.get().SendAndWait(tc.bgctx, twoTraces)
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrStreamRestarting))
}

// TestStreamStatusUnavailableInvalid verifies that the stream reader handles
// an unavailable or invalid status w/o breaking the stream.
func TestStreamStatusUnavailableInvalid(t *testing.T) {
	tc := newStreamTestCase(t)

	tc.fromTracesCall.Times(3).Return(oneBatch, nil)

	channel := newHealthyTestChannel()
	tc.start(channel)
	defer tc.cancelAndWaitForShutdown()

	var wg sync.WaitGroup
	wg.Add(1)
	defer wg.Wait()
	go func() {
		defer wg.Done()
		batch := <-channel.sent
		channel.recv <- statusUnavailableFor(batch.BatchId)
		batch = <-channel.sent
		channel.recv <- statusInvalidFor(batch.BatchId)
		batch = <-channel.sent
		channel.recv <- statusOKFor(batch.BatchId)
	}()
	// sender should get "test unavailable" once, success second time.
	err := tc.get().SendAndWait(tc.bgctx, twoTraces)
	require.Error(t, err)
	require.Contains(t, err.Error(), "test unavailable")

	err = tc.get().SendAndWait(tc.bgctx, twoTraces)
	require.Error(t, err)
	require.Contains(t, err.Error(), "test invalid")

	err = tc.get().SendAndWait(tc.bgctx, twoTraces)
	require.NoError(t, err)
}

// TestStreamStatusUnrecognized verifies that the stream reader handles
// an unrecognized status by breaking the stream.
func TestStreamStatusUnrecognized(t *testing.T) {
	tc := newStreamTestCase(t)

	tc.fromTracesCall.Times(1).Return(oneBatch, nil)

	channel := newHealthyTestChannel()
	tc.start(channel)
	defer tc.cancelAndWaitForShutdown()

	var wg sync.WaitGroup
	wg.Add(1)
	defer wg.Wait()
	go func() {
		defer wg.Done()
		batch := <-channel.sent
		channel.recv <- statusUnrecognizedFor(batch.BatchId)
	}()
	err := tc.get().SendAndWait(tc.bgctx, twoTraces)
	require.Error(t, err)
	require.Contains(t, err.Error(), "test unrecognized")

	// Note: do not cancel the context, the stream should be
	// shutting down due to the error.
	tc.waitForShutdown()
}

// TestStreamUnsupported verifies that the stream signals downgrade
// when an Unsupported code is received, which is how the gRPC client
// responds when the server does not support arrow.
func TestStreamUnsupported(t *testing.T) {
	tc := newStreamTestCase(t)

	channel := newArrowUnsupportedTestChannel()
	tc.start(channel)
	defer tc.cancelAndWaitForShutdown()

	err := tc.get().SendAndWait(tc.bgctx, twoTraces)
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrStreamRestarting))

	tc.waitForShutdown()

	require.Less(t, 0, len(tc.observedLogs.All()), "should have at least one log: %v", tc.observedLogs.All())
	require.Equal(t, tc.observedLogs.All()[0].Message, "arrow is not supported")
}

// TestStreamSendError verifies that the stream reader handles a
// Send() error.
func TestStreamSendError(t *testing.T) {
	tc := newStreamTestCase(t)

	tc.fromTracesCall.Times(1).Return(oneBatch, nil)

	channel := newSendErrorTestChannel()
	tc.start(channel)
	defer tc.cancelAndWaitForShutdown()

	go func() {
		time.Sleep(200 * time.Millisecond)
		channel.unblock()
	}()
	// sender should get ErrStreamRestarting
	err := tc.get().SendAndWait(tc.bgctx, twoTraces)
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrStreamRestarting))
}
