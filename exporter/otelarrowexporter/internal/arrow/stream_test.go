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

	arrowpb "github.com/open-telemetry/otel-arrow/api/experimental/arrow/v1"
	"github.com/open-telemetry/otel-arrow/collector/netstats"
	arrowRecordMock "github.com/open-telemetry/otel-arrow/pkg/otel/arrow_record/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc"
)

var oneBatch = &arrowpb.BatchArrowRecords{
	BatchId: 1,
}

type streamTestCase struct {
	*commonTestCase
	*commonTestStream

	producer    *arrowRecordMock.MockProducerAPI
	prioritizer streamPrioritizer
	bgctx       context.Context
	doneCancel
	fromTracesCall  *gomock.Call
	fromMetricsCall *gomock.Call
	fromLogsCall    *gomock.Call
	stream          *Stream
	wait            sync.WaitGroup
}

func newStreamTestCase(t *testing.T, pname PrioritizerName) *streamTestCase {
	ctrl := gomock.NewController(t)
	producer := arrowRecordMock.NewMockProducerAPI(ctrl)

	bg, dc := newDoneCancel(context.Background())
	prio, state := newStreamPrioritizer(dc, pname, 1)

	ctc := newCommonTestCase(t, NotNoisy)
	cts := ctc.newMockStream(bg)

	// metadata functionality is tested in exporter_test.go
	ctc.requestMetadataCall.AnyTimes().Return(nil, nil)

	stream := newStream(producer, prio, ctc.telset, netstats.Noop{}, state[0])
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
		doneCancel:       dc,
		stream:           stream,
		fromTracesCall:   fromTracesCall,
		fromMetricsCall:  fromMetricsCall,
		fromLogsCall:     fromLogsCall,
	}
}

// start runs a test stream according to the behavior of testChannel.
func (tc *streamTestCase) start(channel testChannel) {
	tc.traceCall.Times(1).DoAndReturn(tc.connectTestStream(channel))

	tc.wait.Add(1)

	go func() {
		defer tc.wait.Done()
		tc.stream.run(tc.bgctx, tc.doneCancel, tc.traceClient, nil)
	}()
}

// cancelAndWait cancels the context and waits for the runner to return.
func (tc *streamTestCase) cancelAndWaitForShutdown() {
	tc.cancel()
	tc.wait.Wait()
}

// cancel waits for the runner to exit without canceling the context.
func (tc *streamTestCase) waitForShutdown() {
	tc.wait.Wait()
}

// connectTestStream returns the stream under test from the common test's mock ArrowStream().
func (tc *streamTestCase) connectTestStream(h testChannel) func(context.Context, ...grpc.CallOption) (
	arrowpb.ArrowTracesService_ArrowTracesClient,
	error,
) {
	return func(ctx context.Context, _ ...grpc.CallOption) (
		arrowpb.ArrowTracesService_ArrowTracesClient,
		error,
	) {
		if err := h.onConnect(ctx); err != nil {
			return nil, err
		}
		tc.sendCall.AnyTimes().DoAndReturn(h.onSend(ctx))
		tc.recvCall.AnyTimes().DoAndReturn(h.onRecv(ctx))
		tc.closeSendCall.AnyTimes().DoAndReturn(h.onCloseSend())
		return tc.anyStreamClient, nil
	}
}

// get returns the stream via the prioritizer it is registered with.
func (tc *streamTestCase) mustGet() streamWriter {
	stream := tc.prioritizer.nextWriter()
	if stream == nil {
		panic("unexpected nil stream")
	}
	return stream
}

func (tc *streamTestCase) mustSendAndWait() error {
	ctx := context.Background()
	ch := make(chan error, 1)
	wri := writeItem{
		producerCtx: context.Background(),
		records:     twoTraces,
		errCh:       ch,
	}
	return tc.mustGet().sendAndWait(ctx, ch, wri)
}

// TestStreamNoMaxLifetime verifies that configuring
// max_stream_lifetime==0 works and the client never
// calls CloseSend().
func TestStreamNoMaxLifetime(t *testing.T) {
	for _, pname := range AllPrioritizers {
		t.Run(string(pname), func(t *testing.T) {

			tc := newStreamTestCase(t, pname)
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

			err := tc.mustSendAndWait()
			require.NoError(t, err)
		})
	}
}

// TestStreamEncodeError verifies that an encoder error in the sender
// yields a permanent error.
func TestStreamEncodeError(t *testing.T) {
	for _, pname := range AllPrioritizers {
		t.Run(string(pname), func(t *testing.T) {
			tc := newStreamTestCase(t, pname)

			testErr := fmt.Errorf("test encode error")
			tc.fromTracesCall.Times(1).Return(nil, testErr)

			tc.start(newHealthyTestChannel())
			defer tc.cancelAndWaitForShutdown()

			// sender should get a permanent testErr
			err := tc.mustSendAndWait()
			require.Error(t, err)
			require.True(t, errors.Is(err, testErr))
			require.True(t, consumererror.IsPermanent(err))
		})
	}
}

// TestStreamUnknownBatchError verifies that the stream reader handles
// a unknown BatchID.
func TestStreamUnknownBatchError(t *testing.T) {
	for _, pname := range AllPrioritizers {
		t.Run(string(pname), func(t *testing.T) {
			tc := newStreamTestCase(t, pname)

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
			err := tc.mustSendAndWait()
			require.Error(t, err)
			require.True(t, errors.Is(err, ErrStreamRestarting))
		})
	}
}

// TestStreamStatusUnavailableInvalid verifies that the stream reader handles
// an unavailable or invalid status w/o breaking the stream.
func TestStreamStatusUnavailableInvalid(t *testing.T) {
	for _, pname := range AllPrioritizers {
		t.Run(string(pname), func(t *testing.T) {
			tc := newStreamTestCase(t, pname)

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
			err := tc.mustSendAndWait()
			require.Error(t, err)
			require.Contains(t, err.Error(), "test unavailable")

			err = tc.mustSendAndWait()
			require.Error(t, err)
			require.Contains(t, err.Error(), "test invalid")

			err = tc.mustSendAndWait()
			require.NoError(t, err)
		})
	}
}

// TestStreamStatusUnrecognized verifies that the stream reader handles
// an unrecognized status by breaking the stream.
func TestStreamStatusUnrecognized(t *testing.T) {
	for _, pname := range AllPrioritizers {
		t.Run(string(pname), func(t *testing.T) {
			tc := newStreamTestCase(t, pname)

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
			err := tc.mustSendAndWait()
			require.Error(t, err)
			require.Contains(t, err.Error(), "test unrecognized")

			// Note: do not cancel the context, the stream should be
			// shutting down due to the error.
			tc.waitForShutdown()
		})
	}
}

// TestStreamUnsupported verifies that the stream signals downgrade
// when an Unsupported code is received, which is how the gRPC client
// responds when the server does not support arrow.
func TestStreamUnsupported(t *testing.T) {
	for _, pname := range AllPrioritizers {
		t.Run(string(pname), func(t *testing.T) {
			tc := newStreamTestCase(t, pname)

			// If the write succeeds before the read, then the FromTraces
			// call will occur.  Otherwise, it will not.

			tc.fromTracesCall.MinTimes(0).MaxTimes(1).Return(oneBatch, nil)

			channel := newArrowUnsupportedTestChannel()
			tc.start(channel)
			defer func() {
				// When the stream returns, the downgrade is needed to
				// cause the request to respond or else it waits for a new
				// stream.
				tc.waitForShutdown()
				tc.cancel()
			}()

			err := tc.mustSendAndWait()
			require.Equal(t, ErrStreamRestarting, err)

			tc.waitForShutdown()

			require.Less(t, 0, len(tc.observedLogs.All()), "should have at least one log: %v", tc.observedLogs.All())
			require.Equal(t, tc.observedLogs.All()[0].Message, "arrow is not supported")
		})
	}
}

// TestStreamSendError verifies that the stream reader handles a
// Send() error.
func TestStreamSendError(t *testing.T) {
	for _, pname := range AllPrioritizers {
		t.Run(string(pname), func(t *testing.T) {
			tc := newStreamTestCase(t, pname)

			tc.fromTracesCall.Times(1).Return(oneBatch, nil)

			channel := newSendErrorTestChannel()
			tc.start(channel)
			defer tc.cancelAndWaitForShutdown()

			go func() {
				time.Sleep(200 * time.Millisecond)
				channel.unblock()
			}()
			// sender should get ErrStreamRestarting
			err := tc.mustSendAndWait()
			require.Error(t, err)
			require.True(t, errors.Is(err, ErrStreamRestarting))
		})
	}
}
