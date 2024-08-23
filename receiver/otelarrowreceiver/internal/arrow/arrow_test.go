// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package arrow

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	arrowpb "github.com/open-telemetry/otel-arrow/api/experimental/arrow/v1"
	arrowCollectorMock "github.com/open-telemetry/otel-arrow/api/experimental/arrow/v1/mock"
	arrowRecord "github.com/open-telemetry/otel-arrow/pkg/otel/arrow_record"
	arrowRecordMock "github.com/open-telemetry/otel-arrow/pkg/otel/arrow_record/mock"
	otelAssert "github.com/open-telemetry/otel-arrow/pkg/otel/assert"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/extension/auth"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap/zaptest"
	"golang.org/x/net/http2/hpack"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/otelarrow/admission"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/otelarrow/netstats"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/otelarrow/testdata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/otelarrowreceiver/internal/arrow/mock"
)

func defaultBQ() *admission.BoundedQueue {
	return admission.NewBoundedQueue(int64(100000), int64(10))
}

type compareJSONTraces struct{ ptrace.Traces }
type compareJSONMetrics struct{ pmetric.Metrics }
type compareJSONLogs struct{ plog.Logs }

func (c compareJSONTraces) MarshalJSON() ([]byte, error) {
	var m ptrace.JSONMarshaler
	return m.MarshalTraces(c.Traces)
}

func (c compareJSONMetrics) MarshalJSON() ([]byte, error) {
	var m pmetric.JSONMarshaler
	return m.MarshalMetrics(c.Metrics)
}

func (c compareJSONLogs) MarshalJSON() ([]byte, error) {
	var m plog.JSONMarshaler
	return m.MarshalLogs(c.Logs)
}

type consumeResult struct {
	Ctx  context.Context
	Data any
}

type commonTestCase struct {
	*testing.T

	ctrl      *gomock.Controller
	cancel    context.CancelFunc
	telset    component.TelemetrySettings
	consumers mockConsumers
	stream    *arrowCollectorMock.MockArrowTracesService_ArrowTracesServer
	receive   chan recvResult
	consume   chan consumeResult
	streamErr chan error

	// testProducer is for convenience -- not thread safe, see copyBatch().
	testProducer *arrowRecord.Producer

	ctxCall  *gomock.Call
	recvCall *gomock.Call
}

type testChannel interface {
	onConsume(ctx context.Context) error
}

type healthyTestChannel struct {
	t *testing.T
}

func newHealthyTestChannel(t *testing.T) *healthyTestChannel {
	return &healthyTestChannel{t: t}
}

func (h healthyTestChannel) onConsume(ctx context.Context) error {
	select {
	case <-ctx.Done():
		h.t.Error("unexpected consume with canceled request")
		return ctx.Err()
	default:
		return nil
	}
}

type unhealthyTestChannel struct {
	t *testing.T
}

func newUnhealthyTestChannel(t *testing.T) *unhealthyTestChannel {
	return &unhealthyTestChannel{t: t}
}

func (u unhealthyTestChannel) onConsume(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return status.Errorf(codes.Unavailable, "consumer unhealthy")
	}
}

type recvResult struct {
	payload *arrowpb.BatchArrowRecords
	err     error
}

type mockConsumers struct {
	traces  *mock.MockTraces
	logs    *mock.MockLogs
	metrics *mock.MockMetrics

	tracesCall  *gomock.Call
	logsCall    *gomock.Call
	metricsCall *gomock.Call
}

func newTestTelemetry(t *testing.T) component.TelemetrySettings {
	telset := componenttest.NewNopTelemetrySettings()
	telset.Logger = zaptest.NewLogger(t)
	return telset
}

func (ctc *commonTestCase) putBatch(payload *arrowpb.BatchArrowRecords, err error) {
	ctc.receive <- recvResult{
		payload: payload,
		err:     err,
	}
}

func (ctc *commonTestCase) doAndReturnGetBatch(ctx context.Context) func() (*arrowpb.BatchArrowRecords, error) {
	return func() (*arrowpb.BatchArrowRecords, error) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case r, ok := <-ctc.receive:
			if !ok {
				return nil, io.EOF
			}
			return r.payload, r.err
		}
	}
}

func (ctc *commonTestCase) doAndReturnConsumeTraces(tc testChannel) func(ctx context.Context, traces ptrace.Traces) error {
	return func(ctx context.Context, traces ptrace.Traces) error {
		ctc.consume <- consumeResult{
			Ctx:  ctx,
			Data: traces,
		}
		return tc.onConsume(ctx)
	}
}

func (ctc *commonTestCase) doAndReturnConsumeMetrics(tc testChannel) func(ctx context.Context, metrics pmetric.Metrics) error {
	return func(ctx context.Context, metrics pmetric.Metrics) error {
		ctc.consume <- consumeResult{
			Ctx:  ctx,
			Data: metrics,
		}
		return tc.onConsume(ctx)
	}
}

func (ctc *commonTestCase) doAndReturnConsumeLogs(tc testChannel) func(ctx context.Context, logs plog.Logs) error {
	return func(ctx context.Context, logs plog.Logs) error {
		ctc.consume <- consumeResult{
			Ctx:  ctx,
			Data: logs,
		}
		return tc.onConsume(ctx)
	}
}

func newMockConsumers(ctrl *gomock.Controller) mockConsumers {
	mc := mockConsumers{
		traces:  mock.NewMockTraces(ctrl),
		logs:    mock.NewMockLogs(ctrl),
		metrics: mock.NewMockMetrics(ctrl),
	}
	mc.traces.EXPECT().Capabilities().Times(0)
	mc.tracesCall = mc.traces.EXPECT().ConsumeTraces(
		gomock.Any(),
		gomock.Any(),
	).Times(0)
	mc.logs.EXPECT().Capabilities().Times(0)
	mc.logsCall = mc.logs.EXPECT().ConsumeLogs(
		gomock.Any(),
		gomock.Any(),
	).Times(0)
	mc.metrics.EXPECT().Capabilities().Times(0)
	mc.metricsCall = mc.metrics.EXPECT().ConsumeMetrics(
		gomock.Any(),
		gomock.Any(),
	).Times(0)
	return mc
}

func (m mockConsumers) Traces() consumer.Traces {
	return m.traces
}

func (m mockConsumers) Logs() consumer.Logs {
	return m.logs
}
func (m mockConsumers) Metrics() consumer.Metrics {
	return m.metrics
}

var _ Consumers = mockConsumers{}

func newCommonTestCase(t *testing.T, tc testChannel) *commonTestCase {
	ctrl := gomock.NewController(t)
	stream := arrowCollectorMock.NewMockArrowTracesService_ArrowTracesServer(ctrl)

	ctx, cancel := context.WithCancel(context.Background())
	ctx = metadata.NewIncomingContext(ctx, metadata.MD{
		"stream_ctx": []string{"per-request"},
	})

	ctc := &commonTestCase{
		T:            t,
		ctrl:         ctrl,
		cancel:       cancel,
		telset:       newTestTelemetry(t),
		consumers:    newMockConsumers(ctrl),
		stream:       stream,
		receive:      make(chan recvResult),
		consume:      make(chan consumeResult),
		streamErr:    make(chan error),
		testProducer: arrowRecord.NewProducer(),
		ctxCall:      stream.EXPECT().Context().Times(0),
		recvCall:     stream.EXPECT().Recv().Times(0),
	}

	ctc.ctxCall.AnyTimes().Return(ctx)
	ctc.recvCall.AnyTimes().DoAndReturn(ctc.doAndReturnGetBatch(ctx))
	ctc.consumers.tracesCall.AnyTimes().DoAndReturn(ctc.doAndReturnConsumeTraces(tc))
	ctc.consumers.logsCall.AnyTimes().DoAndReturn(ctc.doAndReturnConsumeLogs(tc))
	ctc.consumers.metricsCall.AnyTimes().DoAndReturn(ctc.doAndReturnConsumeMetrics(tc))
	return ctc
}

func (ctc *commonTestCase) cancelAndWait() error {
	ctc.cancel()
	return ctc.wait()
}

func (ctc *commonTestCase) wait() error {
	return <-ctc.streamErr
}

func statusOKFor(batchID int64) *arrowpb.BatchStatus {
	return &arrowpb.BatchStatus{
		BatchId:    batchID,
		StatusCode: arrowpb.StatusCode_OK,
	}
}

func statusUnavailableFor(batchID int64, msg string) *arrowpb.BatchStatus {
	return &arrowpb.BatchStatus{
		BatchId:       batchID,
		StatusCode:    arrowpb.StatusCode_UNAVAILABLE,
		StatusMessage: msg,
	}
}

func (ctc *commonTestCase) newRealConsumer() arrowRecord.ConsumerAPI {
	mock := arrowRecordMock.NewMockConsumerAPI(ctc.ctrl)
	cons := arrowRecord.NewConsumer()

	mock.EXPECT().Close().Times(1).Return(nil)
	mock.EXPECT().TracesFrom(gomock.Any()).AnyTimes().DoAndReturn(cons.TracesFrom)
	mock.EXPECT().MetricsFrom(gomock.Any()).AnyTimes().DoAndReturn(cons.MetricsFrom)
	mock.EXPECT().LogsFrom(gomock.Any()).AnyTimes().DoAndReturn(cons.LogsFrom)

	return mock
}

func (ctc *commonTestCase) newErrorConsumer() arrowRecord.ConsumerAPI {
	mock := arrowRecordMock.NewMockConsumerAPI(ctc.ctrl)

	mock.EXPECT().Close().Times(1).Return(nil)
	mock.EXPECT().TracesFrom(gomock.Any()).AnyTimes().Return(nil, fmt.Errorf("test invalid error"))
	mock.EXPECT().MetricsFrom(gomock.Any()).AnyTimes().Return(nil, fmt.Errorf("test invalid error"))
	mock.EXPECT().LogsFrom(gomock.Any()).AnyTimes().Return(nil, fmt.Errorf("test invalid error"))

	return mock
}

func (ctc *commonTestCase) newOOMConsumer() arrowRecord.ConsumerAPI {
	mock := arrowRecordMock.NewMockConsumerAPI(ctc.ctrl)

	mock.EXPECT().Close().Times(1).Return(nil)
	mock.EXPECT().TracesFrom(gomock.Any()).AnyTimes().Return(nil, fmt.Errorf("test oom error %w", arrowRecord.ErrConsumerMemoryLimit))
	mock.EXPECT().MetricsFrom(gomock.Any()).AnyTimes().Return(nil, fmt.Errorf("test oom error %w", arrowRecord.ErrConsumerMemoryLimit))
	mock.EXPECT().LogsFrom(gomock.Any()).AnyTimes().Return(nil, fmt.Errorf("test oom error %w", arrowRecord.ErrConsumerMemoryLimit))

	return mock
}

func (ctc *commonTestCase) start(newConsumer func() arrowRecord.ConsumerAPI, bq *admission.BoundedQueue, opts ...func(*configgrpc.ServerConfig, *auth.Server)) {
	var authServer auth.Server
	var gsettings configgrpc.ServerConfig
	for _, gf := range opts {
		gf(&gsettings, &authServer)
	}
	rc := receiver.Settings{
		TelemetrySettings: ctc.telset,
		BuildInfo:         component.NewDefaultBuildInfo(),
	}
	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
		ReceiverID:             component.NewID(component.MustNewType("arrowtest")),
		Transport:              "grpc",
		ReceiverCreateSettings: rc,
	})
	require.NoError(ctc.T, err)

	rcvr, err := New(
		ctc.consumers,
		rc,
		obsrecv,
		gsettings,
		authServer,
		newConsumer,
		bq,
		netstats.Noop{},
	)
	require.NoError(ctc.T, err)
	go func() {
		ctc.streamErr <- rcvr.ArrowTraces(ctc.stream)
	}()
}

func requireCanceledStatus(t *testing.T, err error) {
	requireStatus(t, codes.Canceled, err)
}

func requireUnavailableStatus(t *testing.T, err error) {
	requireStatus(t, codes.Unavailable, err)
}

func requireInternalStatus(t *testing.T, err error) {
	requireStatus(t, codes.Internal, err)
}

func requireExhaustedStatus(t *testing.T, err error) {
	requireStatus(t, codes.ResourceExhausted, err)
}

func requireStatus(t *testing.T, code codes.Code, err error) {
	require.Error(t, err)
	status, ok := status.FromError(err)
	require.True(t, ok, "is status-wrapped %v", err)
	require.Equal(t, code, status.Code())
}

func TestBoundedQueueWithPdataHeaders(t *testing.T) {
	var sizer ptrace.ProtoMarshaler
	stdTesting := otelAssert.NewStdUnitTest(t)
	pdataSizeTenTraces := sizer.TracesSize(testdata.GenerateTraces(10))
	defaultBoundedQueueLimit := int64(100000)
	tests := []struct {
		name               string
		numTraces          int
		includePdataHeader bool
		pdataSize          string
		rejected           bool
	}{
		{
			name:      "no header compressed greater than uncompressed",
			numTraces: 10,
		},
		{
			name:      "no header compressed less than uncompressed",
			numTraces: 100,
		},
		{
			name:               "pdata header less than uncompressedSize",
			numTraces:          10,
			pdataSize:          strconv.Itoa(pdataSizeTenTraces / 2),
			includePdataHeader: true,
		},
		{
			name:               "pdata header equal uncompressedSize",
			numTraces:          10,
			pdataSize:          strconv.Itoa(pdataSizeTenTraces),
			includePdataHeader: true,
		},
		{
			name:               "pdata header greater than uncompressedSize",
			numTraces:          10,
			pdataSize:          strconv.Itoa(pdataSizeTenTraces * 2),
			includePdataHeader: true,
		},
		{
			name:      "no header compressed accepted uncompressed rejected",
			numTraces: 100,
			rejected:  true,
		},
		{
			name:               "pdata header accepted uncompressed rejected",
			numTraces:          100,
			rejected:           true,
			pdataSize:          strconv.Itoa(pdataSizeTenTraces),
			includePdataHeader: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := newHealthyTestChannel(t)
			ctc := newCommonTestCase(t, tc)

			td := testdata.GenerateTraces(tt.numTraces)
			batch, err := ctc.testProducer.BatchArrowRecordsFromTraces(td)
			require.NoError(t, err)
			if tt.includePdataHeader {
				var hpb bytes.Buffer
				hpe := hpack.NewEncoder(&hpb)
				err = hpe.WriteField(hpack.HeaderField{
					Name:  "otlp-pdata-size",
					Value: tt.pdataSize,
				})
				assert.NoError(t, err)
				batch.Headers = make([]byte, hpb.Len())
				copy(batch.Headers, hpb.Bytes())
			}

			var bq *admission.BoundedQueue
			if tt.rejected {
				ctc.stream.EXPECT().Send(statusOKFor(batch.BatchId)).Times(0)
				bq = admission.NewBoundedQueue(int64(sizer.TracesSize(td)-100), 10)
			} else {
				ctc.stream.EXPECT().Send(statusOKFor(batch.BatchId)).Times(1).Return(nil)
				bq = admission.NewBoundedQueue(defaultBoundedQueueLimit, 10)
			}

			ctc.start(ctc.newRealConsumer, bq)
			ctc.putBatch(batch, nil)

			if tt.rejected {
				requireExhaustedStatus(t, ctc.wait())
			} else {
				data := <-ctc.consume
				actualTD := data.Data.(ptrace.Traces)
				otelAssert.Equiv(stdTesting, []json.Marshaler{
					compareJSONTraces{td},
				}, []json.Marshaler{
					compareJSONTraces{actualTD},
				})
				requireCanceledStatus(t, ctc.cancelAndWait())
			}
		})
	}
}

func TestReceiverTraces(t *testing.T) {
	stdTesting := otelAssert.NewStdUnitTest(t)
	tc := newHealthyTestChannel(t)
	ctc := newCommonTestCase(t, tc)

	td := testdata.GenerateTraces(2)
	batch, err := ctc.testProducer.BatchArrowRecordsFromTraces(td)
	require.NoError(t, err)

	ctc.stream.EXPECT().Send(statusOKFor(batch.BatchId)).Times(1).Return(nil)

	ctc.start(ctc.newRealConsumer, defaultBQ())
	ctc.putBatch(batch, nil)

	otelAssert.Equiv(stdTesting, []json.Marshaler{
		compareJSONTraces{td},
	}, []json.Marshaler{
		compareJSONTraces{(<-ctc.consume).Data.(ptrace.Traces)},
	})

	err = ctc.cancelAndWait()
	requireCanceledStatus(t, err)
}

func TestReceiverLogs(t *testing.T) {
	tc := newHealthyTestChannel(t)
	ctc := newCommonTestCase(t, tc)

	ld := testdata.GenerateLogs(2)
	batch, err := ctc.testProducer.BatchArrowRecordsFromLogs(ld)
	require.NoError(t, err)

	ctc.stream.EXPECT().Send(statusOKFor(batch.BatchId)).Times(1).Return(nil)

	ctc.start(ctc.newRealConsumer, defaultBQ())
	ctc.putBatch(batch, nil)

	assert.EqualValues(t, []json.Marshaler{compareJSONLogs{ld}}, []json.Marshaler{compareJSONLogs{(<-ctc.consume).Data.(plog.Logs)}})

	err = ctc.cancelAndWait()
	requireCanceledStatus(t, err)
}

func TestReceiverMetrics(t *testing.T) {
	tc := newHealthyTestChannel(t)
	ctc := newCommonTestCase(t, tc)
	stdTesting := otelAssert.NewStdUnitTest(t)

	md := testdata.GenerateMetrics(2)
	batch, err := ctc.testProducer.BatchArrowRecordsFromMetrics(md)
	require.NoError(t, err)

	ctc.stream.EXPECT().Send(statusOKFor(batch.BatchId)).Times(1).Return(nil)

	ctc.start(ctc.newRealConsumer, defaultBQ())
	ctc.putBatch(batch, nil)

	otelAssert.Equiv(stdTesting, []json.Marshaler{
		compareJSONMetrics{md},
	}, []json.Marshaler{
		compareJSONMetrics{(<-ctc.consume).Data.(pmetric.Metrics)},
	})

	err = ctc.cancelAndWait()
	requireCanceledStatus(t, err)
}

func TestReceiverRecvError(t *testing.T) {
	tc := newHealthyTestChannel(t)
	ctc := newCommonTestCase(t, tc)

	ctc.start(ctc.newRealConsumer, defaultBQ())

	ctc.putBatch(nil, fmt.Errorf("test recv error"))

	err := ctc.wait()
	require.Error(t, err)
	require.Contains(t, err.Error(), "test recv error")
}

func TestReceiverSendError(t *testing.T) {
	tc := newHealthyTestChannel(t)
	ctc := newCommonTestCase(t, tc)

	ld := testdata.GenerateLogs(2)
	batch, err := ctc.testProducer.BatchArrowRecordsFromLogs(ld)
	require.NoError(t, err)

	ctc.stream.EXPECT().Send(statusOKFor(batch.BatchId)).Times(1).Return(status.Errorf(codes.Unavailable, "test send error"))

	ctc.start(ctc.newRealConsumer, defaultBQ())
	ctc.putBatch(batch, nil)

	assert.EqualValues(t, ld, (<-ctc.consume).Data)

	start := time.Now()
	for time.Since(start) < 10*time.Second {
		if ctc.ctrl.Satisfied() {
			break
		}
		time.Sleep(time.Second)
	}

	// Release the receiver -- the sender has seen an error by
	// now and should return the stream.  (Oddly, gRPC has no way
	// to signal the receive call to fail using context.)
	close(ctc.receive)
	err = ctc.wait()
	requireUnavailableStatus(t, err)
}

func TestReceiverConsumeError(t *testing.T) {
	stdTesting := otelAssert.NewStdUnitTest(t)

	data := []any{
		testdata.GenerateTraces(2),
		testdata.GenerateMetrics(2),
		testdata.GenerateLogs(2),
	}

	for _, item := range data {
		tc := newUnhealthyTestChannel(t)
		ctc := newCommonTestCase(t, tc)

		var batch *arrowpb.BatchArrowRecords
		var err error
		switch input := item.(type) {
		case ptrace.Traces:
			batch, err = ctc.testProducer.BatchArrowRecordsFromTraces(input)
		case plog.Logs:
			batch, err = ctc.testProducer.BatchArrowRecordsFromLogs(input)
		case pmetric.Metrics:
			batch, err = ctc.testProducer.BatchArrowRecordsFromMetrics(input)
		default:
			panic(input)
		}
		require.NoError(t, err)

		batch = copyBatch(batch)

		ctc.stream.EXPECT().Send(statusUnavailableFor(batch.BatchId, "consumer unhealthy")).Times(1).Return(nil)

		ctc.start(ctc.newRealConsumer, defaultBQ())

		ctc.putBatch(batch, nil)

		switch input := item.(type) {
		case ptrace.Traces:
			otelAssert.Equiv(stdTesting, []json.Marshaler{
				compareJSONTraces{input},
			}, []json.Marshaler{
				compareJSONTraces{(<-ctc.consume).Data.(ptrace.Traces)},
			})
		case plog.Logs:
			otelAssert.Equiv(stdTesting, []json.Marshaler{
				compareJSONLogs{input},
			}, []json.Marshaler{
				compareJSONLogs{(<-ctc.consume).Data.(plog.Logs)},
			})
		case pmetric.Metrics:
			otelAssert.Equiv(stdTesting, []json.Marshaler{
				compareJSONMetrics{input},
			}, []json.Marshaler{
				compareJSONMetrics{(<-ctc.consume).Data.(pmetric.Metrics)},
			})
		}

		err = ctc.cancelAndWait()
		requireCanceledStatus(t, err)
	}
}

func TestReceiverInvalidData(t *testing.T) {
	data := []any{
		testdata.GenerateTraces(2),
		testdata.GenerateMetrics(2),
		testdata.GenerateLogs(2),
	}

	for _, item := range data {
		tc := newHealthyTestChannel(t)
		ctc := newCommonTestCase(t, tc)

		var batch *arrowpb.BatchArrowRecords
		var err error
		switch input := item.(type) {
		case ptrace.Traces:
			batch, err = ctc.testProducer.BatchArrowRecordsFromTraces(input)
		case plog.Logs:
			batch, err = ctc.testProducer.BatchArrowRecordsFromLogs(input)
		case pmetric.Metrics:
			batch, err = ctc.testProducer.BatchArrowRecordsFromMetrics(input)
		default:
			panic(input)
		}
		require.NoError(t, err)

		batch = copyBatch(batch)

		// newErrorConsumer determines the internal error in decoding above
		ctc.start(ctc.newErrorConsumer, defaultBQ())
		ctc.putBatch(batch, nil)

		err = ctc.wait()
		requireInternalStatus(t, err)
	}
}

func TestReceiverMemoryLimit(t *testing.T) {
	data := []any{
		testdata.GenerateTraces(2),
		testdata.GenerateMetrics(2),
		testdata.GenerateLogs(2),
	}

	for _, item := range data {
		tc := newHealthyTestChannel(t)
		ctc := newCommonTestCase(t, tc)

		var batch *arrowpb.BatchArrowRecords
		var err error
		switch input := item.(type) {
		case ptrace.Traces:
			batch, err = ctc.testProducer.BatchArrowRecordsFromTraces(input)
		case plog.Logs:
			batch, err = ctc.testProducer.BatchArrowRecordsFromLogs(input)
		case pmetric.Metrics:
			batch, err = ctc.testProducer.BatchArrowRecordsFromMetrics(input)
		default:
			panic(input)
		}
		require.NoError(t, err)

		batch = copyBatch(batch)

		// The Recv() returns an error, there are no Send() calls.

		ctc.start(ctc.newOOMConsumer, defaultBQ())
		ctc.putBatch(batch, nil)

		err = ctc.wait()
		requireExhaustedStatus(t, err)
	}
}

func copyBatch(in *arrowpb.BatchArrowRecords) *arrowpb.BatchArrowRecords {
	// Because Arrow-IPC uses zero copy, we have to copy inside the test
	// instead of sharing pointers to BatchArrowRecords.

	hcpy := make([]byte, len(in.Headers))
	copy(hcpy, in.Headers)

	pays := make([]*arrowpb.ArrowPayload, len(in.ArrowPayloads))

	for i, inp := range in.ArrowPayloads {
		rcpy := make([]byte, len(inp.Record))
		copy(rcpy, inp.Record)
		pays[i] = &arrowpb.ArrowPayload{
			SchemaId: inp.SchemaId,
			Type:     inp.Type,
			Record:   rcpy,
		}
	}

	return &arrowpb.BatchArrowRecords{
		BatchId:       in.BatchId,
		Headers:       hcpy,
		ArrowPayloads: pays,
	}
}

func TestReceiverEOF(t *testing.T) {
	tc := newHealthyTestChannel(t)
	ctc := newCommonTestCase(t, tc)
	stdTesting := otelAssert.NewStdUnitTest(t)

	// send a sequence of data then simulate closing the connection.
	const times = 10

	var actualData []ptrace.Traces
	var expectData []ptrace.Traces

	ctc.stream.EXPECT().Send(gomock.Any()).Times(times).Return(nil)

	ctc.start(ctc.newRealConsumer, defaultBQ())

	go func() {
		for i := 0; i < times; i++ {
			td := testdata.GenerateTraces(2)
			expectData = append(expectData, td)

			batch, err := ctc.testProducer.BatchArrowRecordsFromTraces(td)
			require.NoError(t, err)

			batch = copyBatch(batch)

			ctc.putBatch(batch, nil)
		}
		close(ctc.receive)
	}()

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		require.NoError(t, ctc.wait())
		wg.Done()
	}()

	for i := 0; i < times; i++ {
		actualData = append(actualData, (<-ctc.consume).Data.(ptrace.Traces))
	}

	assert.Equal(t, len(expectData), len(actualData))

	for i := 0; i < len(expectData); i++ {
		otelAssert.Equiv(stdTesting, []json.Marshaler{
			compareJSONTraces{expectData[i]},
		}, []json.Marshaler{
			compareJSONTraces{actualData[i]},
		})
	}

	wg.Wait()
}

func TestReceiverHeadersNoAuth(t *testing.T) {
	t.Run("include", func(t *testing.T) { testReceiverHeaders(t, true) })
	t.Run("noinclude", func(t *testing.T) { testReceiverHeaders(t, false) })
}

func testReceiverHeaders(t *testing.T, includeMeta bool) {
	tc := newHealthyTestChannel(t)
	ctc := newCommonTestCase(t, tc)

	expectData := []map[string][]string{
		{"k1": []string{"v1"}},
		nil,
		{"k2": []string{"v2"}, "k3": []string{"v3"}},
		nil,
		{"k1": []string{"v5"}},
		{"k1": []string{"v1"}, "k3": []string{"v2", "v3", "v4"}},
		nil,
	}

	ctc.stream.EXPECT().Send(gomock.Any()).Times(len(expectData)).Return(nil)

	ctc.start(ctc.newRealConsumer, defaultBQ(), func(gsettings *configgrpc.ServerConfig, _ *auth.Server) {
		gsettings.IncludeMetadata = includeMeta
	})

	go func() {
		var hpb bytes.Buffer
		hpe := hpack.NewEncoder(&hpb)

		for _, md := range expectData {
			td := testdata.GenerateTraces(2)

			batch, err := ctc.testProducer.BatchArrowRecordsFromTraces(td)
			require.NoError(t, err)

			batch = copyBatch(batch)

			if len(md) != 0 {
				hpb.Reset()
				for key, vals := range md {
					for _, val := range vals {
						err := hpe.WriteField(hpack.HeaderField{
							Name:  key,
							Value: val,
						})
						require.NoError(t, err)
					}
				}

				batch.Headers = make([]byte, hpb.Len())
				copy(batch.Headers, hpb.Bytes())
			}
			ctc.putBatch(batch, nil)
		}
		close(ctc.receive)
	}()

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		require.NoError(t, ctc.wait())
		wg.Done()
	}()

	for _, expect := range expectData {
		info := client.FromContext((<-ctc.consume).Ctx)

		// The static stream context contains one extra variable.
		if expect == nil {
			expect = map[string][]string{}
		}
		expect["stream_ctx"] = []string{"per-request"}

		for key, vals := range expect {
			if includeMeta {
				require.Equal(t, vals, info.Metadata.Get(key))
			} else {
				require.Equal(t, []string(nil), info.Metadata.Get(key))
			}
		}
	}

	wg.Wait()
}

func TestReceiverCancel(t *testing.T) {
	tc := newHealthyTestChannel(t)
	ctc := newCommonTestCase(t, tc)

	ctc.cancel()
	ctc.start(ctc.newRealConsumer, defaultBQ())

	err := ctc.wait()
	requireCanceledStatus(t, err)
}

func requireContainsAll(t *testing.T, md client.Metadata, exp map[string][]string) {
	for key, vals := range exp {
		require.Equal(t, vals, md.Get(key))
	}
}

func requireContainsNone(t *testing.T, md client.Metadata, exp map[string][]string) {
	for key := range exp {
		require.Equal(t, []string(nil), md.Get(key))
	}
}

func TestHeaderReceiverStreamContextOnly(t *testing.T) {
	expect := map[string][]string{
		"K": {"k1", "k2"},
		"L": {"l1"},
	}

	ctx := metadata.NewIncomingContext(context.Background(), metadata.MD(expect))

	h := newHeaderReceiver(ctx, nil, true)

	for i := 0; i < 3; i++ {
		cc, _, err := h.combineHeaders(ctx, nil)

		require.NoError(t, err)
		requireContainsAll(t, client.FromContext(cc).Metadata, expect)
	}
}

func TestHeaderReceiverNoIncludeMetadata(t *testing.T) {
	noExpect := map[string][]string{
		"K": {"k1", "k2"},
		"L": {"l1"},
	}

	ctx := metadata.NewIncomingContext(context.Background(), metadata.MD(noExpect))

	h := newHeaderReceiver(ctx, nil, false)

	for i := 0; i < 3; i++ {
		cc, _, err := h.combineHeaders(ctx, nil)

		require.NoError(t, err)
		requireContainsNone(t, client.FromContext(cc).Metadata, noExpect)
	}
}

func TestHeaderReceiverAuthServerNoIncludeMetadata(t *testing.T) {
	expectForAuth := map[string][]string{
		"L": {"k1", "k2"},
		"K": {"l1"},
	}

	ctx := metadata.NewIncomingContext(context.Background(), metadata.MD(expectForAuth))

	ctrl := gomock.NewController(t)
	as := mock.NewMockServer(ctrl)

	// The auth server is not called, it just needs to be non-nil.
	as.EXPECT().Authenticate(gomock.Any(), gomock.Any()).Times(0)

	h := newHeaderReceiver(ctx, as, false)

	for i := 0; i < 3; i++ {
		cc, hdrs, err := h.combineHeaders(ctx, nil)

		// The incoming metadata keys are not in the context.
		require.NoError(t, err)
		requireContainsNone(t, client.FromContext(cc).Metadata, expectForAuth)

		// Headers are returned for the auth server, though
		// names have been forced to lower case.
		require.Equal(t, len(hdrs), len(expectForAuth))
		for k, v := range expectForAuth {
			require.Equal(t, hdrs[strings.ToLower(k)], v)
		}
	}
}

func TestHeaderReceiverRequestNoStreamMetadata(t *testing.T) {
	expect := map[string][]string{
		"K": {"k1", "k2"},
		"L": {"l1"},
	}

	var hpb bytes.Buffer

	hpe := hpack.NewEncoder(&hpb)

	ctx := context.Background()

	h := newHeaderReceiver(ctx, nil, true)

	for i := 0; i < 3; i++ {
		hpb.Reset()

		for key, vals := range expect {
			for _, val := range vals {
				err := hpe.WriteField(hpack.HeaderField{
					Name:  strings.ToLower(key),
					Value: val,
				})
				require.NoError(t, err)
			}
		}

		cc, _, err := h.combineHeaders(ctx, hpb.Bytes())

		require.NoError(t, err)
		requireContainsAll(t, client.FromContext(cc).Metadata, expect)
	}
}

func TestHeaderReceiverAuthServerIsSetNoIncludeMetadata(t *testing.T) {
	expect := map[string][]string{
		"K": {"k1", "k2"},
		"L": {"l1"},
	}

	var hpb bytes.Buffer

	hpe := hpack.NewEncoder(&hpb)

	ctx := context.Background()

	ctrl := gomock.NewController(t)
	as := mock.NewMockServer(ctrl)

	// The auth server is not called, it just needs to be non-nil.
	as.EXPECT().Authenticate(gomock.Any(), gomock.Any()).Times(0)

	h := newHeaderReceiver(ctx, as, true)

	for i := 0; i < 3; i++ {
		hpb.Reset()

		for key, vals := range expect {
			for _, val := range vals {
				err := hpe.WriteField(hpack.HeaderField{
					Name:  strings.ToLower(key),
					Value: val,
				})
				require.NoError(t, err)
			}
		}

		cc, hdrs, err := h.combineHeaders(ctx, hpb.Bytes())

		require.NoError(t, err)

		// Note: The call to client.Metadata.Get() inside
		// requireContainsAll() actually modifies the metadata
		// map (this is weird, but true and possibly
		// valid). In cases where the input has incorrect
		// case.  It's not safe to check that the map sizes
		// are equal after calling Get() below, so we assert
		// same size first.
		require.Equal(t, len(hdrs), len(expect))

		requireContainsAll(t, client.FromContext(cc).Metadata, expect)

		// Headers passed to the auth server are equivalent w/
		// with names forced to lower case.

		for k, v := range expect {
			require.Equal(t, hdrs[strings.ToLower(k)], v, "for %v", k)
		}
	}
}

func TestHeaderReceiverBothMetadata(t *testing.T) {
	expectK := map[string][]string{
		"K": {"k1", "k2"},
	}
	expectL := map[string][]string{
		"L": {"l1"},
		"M": {"m1", "m2"},
	}
	expect := map[string][]string{
		"K": {"k1", "k2"},
		"L": {"l1"},
		"M": {"m1", "m2"},
	}

	var hpb bytes.Buffer

	hpe := hpack.NewEncoder(&hpb)

	ctx := metadata.NewIncomingContext(context.Background(), metadata.MD(expectK))

	h := newHeaderReceiver(ctx, nil, true)

	for i := 0; i < 3; i++ {
		hpb.Reset()

		for key, vals := range expectL {
			for _, val := range vals {
				err := hpe.WriteField(hpack.HeaderField{
					Name:  strings.ToLower(key),
					Value: val,
				})
				require.NoError(t, err)
			}
		}

		cc, _, err := h.combineHeaders(ctx, hpb.Bytes())

		require.NoError(t, err)
		requireContainsAll(t, client.FromContext(cc).Metadata, expect)
	}
}

func TestHeaderReceiverDuplicateMetadata(t *testing.T) {
	expectStream := map[string][]string{
		"K": {"k1", "k2"},

		// "M" value does not appear b/c the same header
		// appears in per-request metadata.
		"M": {""},
	}
	expectRequest := map[string][]string{
		"L": {"l1"},
		"M": {"m1", "m2"},
	}
	expectCombined := map[string][]string{
		"K": {"k1", "k2"},
		"L": {"l1"},
		"M": {"m1", "m2"},
	}

	var hpb bytes.Buffer

	hpe := hpack.NewEncoder(&hpb)

	ctx := metadata.NewIncomingContext(context.Background(), metadata.MD(expectStream))

	h := newHeaderReceiver(ctx, nil, true)

	for i := 0; i < 3; i++ {
		hpb.Reset()

		for key, vals := range expectRequest {
			for _, val := range vals {
				err := hpe.WriteField(hpack.HeaderField{
					Name:  strings.ToLower(key),
					Value: val,
				})
				require.NoError(t, err)
			}
		}

		cc, _, err := h.combineHeaders(ctx, hpb.Bytes())

		require.NoError(t, err)
		requireContainsAll(t, client.FromContext(cc).Metadata, expectCombined)
	}
}

func TestReceiverAuthHeadersStream(t *testing.T) {
	t.Run("no-metadata", func(t *testing.T) { testReceiverAuthHeaders(t, false, false) })
	t.Run("per-stream", func(t *testing.T) { testReceiverAuthHeaders(t, true, false) })
	t.Run("per-data", func(t *testing.T) { testReceiverAuthHeaders(t, true, true) })
}

func testReceiverAuthHeaders(t *testing.T, includeMeta bool, dataAuth bool) {
	tc := newHealthyTestChannel(t)
	ctc := newCommonTestCase(t, tc)

	expectData := []map[string][]string{
		{"auth": []string{"true"}},
		nil,
		{"auth": []string{"false"}},
		nil,
	}

	recvBatches := make([]*arrowpb.BatchStatus, len(expectData))

	ctc.stream.EXPECT().Send(gomock.Any()).Times(len(expectData)).DoAndReturn(func(batch *arrowpb.BatchStatus) error {
		require.Nil(t, recvBatches[batch.BatchId])
		recvBatches[batch.BatchId] = batch
		return nil
	})

	var authCall *gomock.Call
	ctc.start(ctc.newRealConsumer, defaultBQ(), func(gsettings *configgrpc.ServerConfig, authPtr *auth.Server) {
		gsettings.IncludeMetadata = includeMeta

		as := mock.NewMockServer(ctc.ctrl)
		*authPtr = as

		authCall = as.EXPECT().Authenticate(gomock.Any(), gomock.Any()).AnyTimes()
	})

	dataCount := 0

	authCall.DoAndReturn(func(ctx context.Context, hdrs map[string][]string) (context.Context, error) {
		dataCount++
		if !dataAuth {
			return ctx, nil
		}

		ok := false
		for _, val := range hdrs["auth"] {
			ok = ok || (val == "true")
		}

		if ok {
			newmd := map[string][]string{}
			for k, v := range hdrs {
				newmd[k] = v
			}
			newmd["has_auth"] = []string{":+1:", ":100:"}
			return client.NewContext(ctx, client.Info{
				Metadata: client.NewMetadata(newmd),
			}), nil
		}
		return ctx, fmt.Errorf("not authorized")
	})

	go func() {
		var hpb bytes.Buffer
		hpe := hpack.NewEncoder(&hpb)

		for _, md := range expectData {
			td := testdata.GenerateTraces(2)

			batch, err := ctc.testProducer.BatchArrowRecordsFromTraces(td)
			require.NoError(t, err)

			batch = copyBatch(batch)

			if len(md) != 0 {

				hpb.Reset()
				for key, vals := range md {
					for _, val := range vals {
						err := hpe.WriteField(hpack.HeaderField{
							Name:  strings.ToLower(key),
							Value: val,
						})
						require.NoError(t, err)
					}
				}

				batch.Headers = make([]byte, hpb.Len())
				copy(batch.Headers, hpb.Bytes())
			}
			ctc.putBatch(batch, nil)
		}
		close(ctc.receive)
	}()

	var expectCodes []arrowpb.StatusCode

	for _, testInput := range expectData {
		// The static stream context contains one extra variable.
		cpy := map[string][]string{}
		cpy["stream_ctx"] = []string{"per-request"}

		for k, v := range testInput {
			cpy[k] = v
		}

		expectCode := arrowpb.StatusCode_OK
		if dataAuth {
			hasAuth := false
			for _, val := range cpy["auth"] {
				hasAuth = hasAuth || (val == "true")
			}
			if hasAuth {
				cpy["has_auth"] = []string{":+1:", ":100:"}
			} else {
				expectCode = arrowpb.StatusCode_UNAUTHENTICATED
			}
		}

		expectCodes = append(expectCodes, expectCode)

		if expectCode != arrowpb.StatusCode_OK {
			continue
		}

		info := client.FromContext((<-ctc.consume).Ctx)

		for key, vals := range cpy {
			if includeMeta {
				require.Equal(t, vals, info.Metadata.Get(key))
			} else {
				require.Equal(t, []string(nil), info.Metadata.Get(key))
			}
		}
	}

	require.NoError(t, ctc.wait())

	require.Equal(t, len(expectCodes), dataCount)
	require.Equal(t, len(expectData), dataCount)
	require.Equal(t, len(recvBatches), dataCount)

	for idx, batch := range recvBatches {
		require.Equal(t, expectCodes[idx], batch.StatusCode)
	}
}

func TestHeaderReceiverIsTraced(t *testing.T) {
	streamHeaders := map[string][]string{
		"K": {"k1", "k2"},
	}
	requestHeaders := map[string][]string{
		"L":           {"l1"},
		"traceparent": {"00-00112233445566778899aabbccddeeff-0011223344556677-01"},
	}
	expectCombined := map[string][]string{
		"K": {"k1", "k2"},
		"L": {"l1"},
	}

	var hpb bytes.Buffer

	otel.SetTextMapPropagator(propagation.TraceContext{})

	hpe := hpack.NewEncoder(&hpb)

	ctx := metadata.NewIncomingContext(context.Background(), metadata.MD(streamHeaders))

	h := newHeaderReceiver(ctx, nil, true)

	for i := 0; i < 3; i++ {
		hpb.Reset()

		for key, vals := range requestHeaders {
			for _, val := range vals {
				err := hpe.WriteField(hpack.HeaderField{
					Name:  strings.ToLower(key),
					Value: val,
				})
				require.NoError(t, err)
			}
		}

		newCtx, _, err := h.combineHeaders(ctx, hpb.Bytes())

		require.NoError(t, err)
		requireContainsAll(t, client.FromContext(newCtx).Metadata, expectCombined)

		// Check for hard-coded trace and span IDs from `traceparent` header above.
		spanCtx := trace.SpanContextFromContext(newCtx)
		require.Equal(
			t,
			trace.TraceID{0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88, 0x99, 0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff},
			spanCtx.TraceID())
		require.Equal(
			t,
			trace.SpanID{0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77},
			spanCtx.SpanID())
	}
}
