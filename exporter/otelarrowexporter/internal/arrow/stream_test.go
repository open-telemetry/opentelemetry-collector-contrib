// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package arrow

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	arrowpb "github.com/open-telemetry/otel-arrow/go/api/experimental/arrow/v1"
	arrowRecordMock "github.com/open-telemetry/otel-arrow/go/pkg/otel/arrow_record/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/otelarrow/netstats"
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

	bg, dc := newDoneCancel(t.Context())
	prio, state := newStreamPrioritizer(dc, pname, 1, 10*time.Second)

	ctc := newCommonTestCase(t, NotNoisy)
	cts := ctc.newMockStream(bg)

	// metadata functionality is tested in exporter_test.go
	ctc.requestMetadataCall.AnyTimes().Return(nil, nil)

	stream := newStream(producer, prio, ctc.telset, netstats.Noop{}, state[0])

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

	tc.wait.Go(func() {
		tc.stream.run(tc.bgctx, tc.doneCancel, tc.traceClient, nil)
	})
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

			testErr := errors.New("test encode error")
			tc.fromTracesCall.Times(1).Return(nil, testErr)

			tc.start(newHealthyTestChannel())
			defer tc.cancelAndWaitForShutdown()

			// sender should get a permanent testErr
			err := tc.mustSendAndWait()
			require.Error(t, err)

			stat, is := status.FromError(err)
			require.True(t, is, "is a gRPC status error: %v", err)
			require.Equal(t, codes.Internal, stat.Code())

			require.Contains(t, stat.Message(), testErr.Error())
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
			require.ErrorIs(t, err, ErrStreamRestarting)
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
			require.ErrorContains(t, err, "test unavailable")

			err = tc.mustSendAndWait()
			require.ErrorContains(t, err, "test invalid")

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
			require.ErrorContains(t, err, "test unrecognized")

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

			require.NotEmpty(t, tc.observedLogs.All(), "should have at least one log: %v", tc.observedLogs.All())
			require.Equal(t, "arrow is not supported", tc.observedLogs.All()[0].Message)
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
			require.ErrorIs(t, err, ErrStreamRestarting)
		})
	}
}
