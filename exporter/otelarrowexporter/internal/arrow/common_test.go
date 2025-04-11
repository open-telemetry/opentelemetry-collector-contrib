// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package arrow

import (
	"context"
	"errors"
	"io"

	arrowpb "github.com/open-telemetry/otel-arrow/api/experimental/arrow/v1"
	arrowCollectorMock "github.com/open-telemetry/otel-arrow/api/experimental/arrow/v1/mock"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest"
	"go.uber.org/zap/zaptest/observer"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/otelarrowexporter/internal/arrow/grpcmock"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/otelarrow/testdata"
)

var (
	twoTraces  = testdata.GenerateTraces(2)
	twoMetrics = testdata.GenerateMetrics(2)
	twoLogs    = testdata.GenerateLogs(2)
)

type testChannel interface {
	onRecv(context.Context) func() (*arrowpb.BatchStatus, error)
	onSend(context.Context) func(*arrowpb.BatchArrowRecords) error
	onConnect(context.Context) error
	onCloseSend() func() error
}

type commonTestCase struct {
	ctrl                *gomock.Controller
	telset              component.TelemetrySettings
	observedLogs        *observer.ObservedLogs
	traceClient         StreamClientFunc
	traceCall           *gomock.Call
	perRPCCredentials   credentials.PerRPCCredentials
	requestMetadataCall *gomock.Call
}

type noisyTest bool

const (
	Noisy    noisyTest = true
	NotNoisy noisyTest = false
)

func newTestTelemetry(t zaptest.TestingT, noisy noisyTest) (component.TelemetrySettings, *observer.ObservedLogs) {
	telset := componenttest.NewNopTelemetrySettings()
	if noisy {
		return telset, nil
	}
	core, obslogs := observer.New(zapcore.InfoLevel)
	telset.Logger = zap.New(zapcore.NewTee(core, zaptest.NewLogger(t).Core()))
	return telset, obslogs
}

type z2m struct {
	zaptest.TestingT
}

var _ gomock.TestReporter = z2m{}

func (t z2m) Fatalf(format string, args ...any) {
	t.Errorf(format, args...)
	t.Fail()
}

func newCommonTestCase(t zaptest.TestingT, noisy noisyTest) *commonTestCase {
	ctrl := gomock.NewController(z2m{t})
	telset, obslogs := newTestTelemetry(t, noisy)

	creds := grpcmock.NewMockPerRPCCredentials(ctrl)
	creds.EXPECT().RequireTransportSecurity().Times(0) // unused interface method
	requestMetadataCall := creds.EXPECT().GetRequestMetadata(
		gomock.Any(), // context.Context
		gomock.Any(), // ...string (unused `uri` parameter)
	).Times(0)

	traceClient := arrowCollectorMock.NewMockArrowTracesServiceClient(ctrl)

	traceCall := traceClient.EXPECT().ArrowTraces(
		gomock.Any(), // context.Context
		gomock.Any(), // ...grpc.CallOption
	).Times(0)
	return &commonTestCase{
		ctrl:                ctrl,
		telset:              telset,
		observedLogs:        obslogs,
		traceClient:         MakeAnyStreamClient("ArrowTraces", traceClient.ArrowTraces),
		traceCall:           traceCall,
		perRPCCredentials:   creds,
		requestMetadataCall: requestMetadataCall,
	}
}

type commonTestStream struct {
	anyStreamClient AnyStreamClient
	ctxCall         *gomock.Call
	sendCall        *gomock.Call
	recvCall        *gomock.Call
	closeSendCall   *gomock.Call
}

func (ctc *commonTestCase) newMockStream(ctx context.Context) *commonTestStream {
	client := arrowCollectorMock.NewMockArrowTracesService_ArrowTracesClient(ctc.ctrl)

	testStream := &commonTestStream{
		anyStreamClient: client,
		ctxCall:         client.EXPECT().Context().AnyTimes().Return(ctx),
		sendCall: client.EXPECT().Send(
			gomock.Any(), // *arrowpb.BatchArrowRecords
		).Times(0),
		recvCall:      client.EXPECT().Recv().Times(0),
		closeSendCall: client.EXPECT().CloseSend().Times(0),
	}
	return testStream
}

// returnNewStream applies the list of test channels in order to
// construct new streams.  The final entry is re-used for new streams
// when it is reached.
func (ctc *commonTestCase) returnNewStream(hs ...testChannel) func(context.Context, ...grpc.CallOption) (
	arrowpb.ArrowTracesService_ArrowTracesClient,
	error,
) {
	var pos int
	return func(ctx context.Context, _ ...grpc.CallOption) (
		arrowpb.ArrowTracesService_ArrowTracesClient,
		error,
	) {
		h := hs[pos]
		if pos < len(hs) {
			pos++
		}
		if err := h.onConnect(ctx); err != nil {
			return nil, err
		}
		str := ctc.newMockStream(ctx)
		str.sendCall.AnyTimes().DoAndReturn(h.onSend(ctx))
		str.recvCall.AnyTimes().DoAndReturn(h.onRecv(ctx))
		str.closeSendCall.AnyTimes().DoAndReturn(h.onCloseSend())
		return str.anyStreamClient, nil
	}
}

// repeatedNewStream returns a stream configured with a new test
// channel on every ArrowStream() request.
func (ctc *commonTestCase) repeatedNewStream(nc func() testChannel) func(context.Context, ...grpc.CallOption) (
	arrowpb.ArrowTracesService_ArrowTracesClient,
	error,
) {
	return func(ctx context.Context, _ ...grpc.CallOption) (
		arrowpb.ArrowTracesService_ArrowTracesClient,
		error,
	) {
		h := nc()
		if err := h.onConnect(ctx); err != nil {
			return nil, err
		}
		str := ctc.newMockStream(ctx)
		str.sendCall.AnyTimes().DoAndReturn(h.onSend(ctx))
		str.recvCall.AnyTimes().DoAndReturn(h.onRecv(ctx))
		str.closeSendCall.AnyTimes().DoAndReturn(h.onCloseSend())
		return str.anyStreamClient, nil
	}
}

// healthyTestChannel accepts the connection and returns an OK status immediately.
type healthyTestChannel struct {
	sent chan *arrowpb.BatchArrowRecords
	recv chan *arrowpb.BatchStatus
}

func newHealthyTestChannel() *healthyTestChannel {
	return &healthyTestChannel{
		sent: make(chan *arrowpb.BatchArrowRecords),
		recv: make(chan *arrowpb.BatchStatus),
	}
}

func (tc *healthyTestChannel) sendChannel() chan *arrowpb.BatchArrowRecords {
	return tc.sent
}

func (tc *healthyTestChannel) onConnect(_ context.Context) error {
	return nil
}

func (tc *healthyTestChannel) onCloseSend() func() error {
	return func() error {
		close(tc.sent)
		return nil
	}
}

func (tc *healthyTestChannel) onSend(ctx context.Context) func(*arrowpb.BatchArrowRecords) error {
	return func(req *arrowpb.BatchArrowRecords) error {
		select {
		case tc.sendChannel() <- req:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (tc *healthyTestChannel) onRecv(ctx context.Context) func() (*arrowpb.BatchStatus, error) {
	return func() (*arrowpb.BatchStatus, error) {
		select {
		case recv, ok := <-tc.recv:
			if !ok {
				return nil, io.EOF
			}

			return recv, nil
		case <-ctx.Done():
			return &arrowpb.BatchStatus{}, ctx.Err()
		}
	}
}

// unresponsiveTestChannel accepts the connection and receives data,
// but never responds with status OK.
type unresponsiveTestChannel struct {
	ch chan struct{}
}

func newUnresponsiveTestChannel() *unresponsiveTestChannel {
	return &unresponsiveTestChannel{
		ch: make(chan struct{}),
	}
}

func (tc *unresponsiveTestChannel) onConnect(_ context.Context) error {
	return nil
}

func (tc *unresponsiveTestChannel) onCloseSend() func() error {
	return func() error {
		return nil
	}
}

func (tc *unresponsiveTestChannel) onSend(ctx context.Context) func(*arrowpb.BatchArrowRecords) error {
	return func(_ *arrowpb.BatchArrowRecords) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			return nil
		}
	}
}

func (tc *unresponsiveTestChannel) onRecv(ctx context.Context) func() (*arrowpb.BatchStatus, error) {
	return func() (*arrowpb.BatchStatus, error) {
		select {
		case <-tc.ch:
			return nil, io.EOF
		case <-ctx.Done():
			return &arrowpb.BatchStatus{}, ctx.Err()
		}
	}
}

func (tc *unresponsiveTestChannel) unblock() {
	close(tc.ch)
}

// unsupportedTestChannel mimics gRPC's behavior when there is no
// arrow stream service registered with the server.
type arrowUnsupportedTestChannel struct{}

func newArrowUnsupportedTestChannel() *arrowUnsupportedTestChannel {
	return &arrowUnsupportedTestChannel{}
}

func (tc *arrowUnsupportedTestChannel) onConnect(_ context.Context) error {
	// Note: this matches gRPC's apparent behavior. the stream
	// connection succeeds and the unsupported code is returned to
	// the Recv() call.
	return nil
}

func (tc *arrowUnsupportedTestChannel) onCloseSend() func() error {
	return func() error {
		return nil
	}
}

func (tc *arrowUnsupportedTestChannel) onSend(ctx context.Context) func(*arrowpb.BatchArrowRecords) error {
	return func(_ *arrowpb.BatchArrowRecords) error {
		<-ctx.Done()
		return ctx.Err()
	}
}

func (tc *arrowUnsupportedTestChannel) onRecv(_ context.Context) func() (*arrowpb.BatchStatus, error) {
	return func() (*arrowpb.BatchStatus, error) {
		err := status.Error(codes.Unimplemented, "arrow will not be served")
		return &arrowpb.BatchStatus{}, err
	}
}

// disconnectedTestChannel allows the connection to time out.
type disconnectedTestChannel struct{}

func newDisconnectedTestChannel() *disconnectedTestChannel {
	return &disconnectedTestChannel{}
}

func (tc *disconnectedTestChannel) onConnect(ctx context.Context) error {
	<-ctx.Done()
	return ctx.Err()
}

func (tc *disconnectedTestChannel) onCloseSend() func() error {
	return func() error {
		panic("unreachable")
	}
}

func (tc *disconnectedTestChannel) onSend(_ context.Context) func(*arrowpb.BatchArrowRecords) error {
	return func(_ *arrowpb.BatchArrowRecords) error {
		panic("unreachable")
	}
}

func (tc *disconnectedTestChannel) onRecv(_ context.Context) func() (*arrowpb.BatchStatus, error) {
	return func() (*arrowpb.BatchStatus, error) {
		panic("unreachable")
	}
}

// sendErrorTestChannel returns an error in Send()
type sendErrorTestChannel struct {
	release chan struct{}
}

func newSendErrorTestChannel() *sendErrorTestChannel {
	return &sendErrorTestChannel{
		release: make(chan struct{}),
	}
}

func (tc *sendErrorTestChannel) onConnect(_ context.Context) error {
	return nil
}

func (tc *sendErrorTestChannel) onCloseSend() func() error {
	return func() error {
		return nil
	}
}

func (tc *sendErrorTestChannel) onSend(_ context.Context) func(*arrowpb.BatchArrowRecords) error {
	return func(*arrowpb.BatchArrowRecords) error {
		return io.EOF
	}
}

func (tc *sendErrorTestChannel) unblock() {
	close(tc.release)
}

func (tc *sendErrorTestChannel) onRecv(_ context.Context) func() (*arrowpb.BatchStatus, error) {
	return func() (*arrowpb.BatchStatus, error) {
		<-tc.release
		return &arrowpb.BatchStatus{}, io.EOF
	}
}

// connectErrorTestChannel returns an error from the ArrowTraces() call
type connectErrorTestChannel struct{}

func newConnectErrorTestChannel() *connectErrorTestChannel {
	return &connectErrorTestChannel{}
}

func (tc *connectErrorTestChannel) onConnect(_ context.Context) error {
	return errors.New("test connect error")
}

func (tc *connectErrorTestChannel) onCloseSend() func() error {
	return func() error {
		panic("unreachable")
	}
}

func (tc *connectErrorTestChannel) onSend(_ context.Context) func(*arrowpb.BatchArrowRecords) error {
	return func(*arrowpb.BatchArrowRecords) error {
		panic("not reached")
	}
}

func (tc *connectErrorTestChannel) onRecv(_ context.Context) func() (*arrowpb.BatchStatus, error) {
	return func() (*arrowpb.BatchStatus, error) {
		panic("not reached")
	}
}
