// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package arrow

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/golang/mock/gomock"
	arrowpb "github.com/open-telemetry/otel-arrow/api/experimental/arrow/v1"
	arrowCollectorMock "github.com/open-telemetry/otel-arrow/api/experimental/arrow/v1/mock"
	arrowRecord "github.com/open-telemetry/otel-arrow/pkg/otel/arrow_record"
	arrowRecordMock "github.com/open-telemetry/otel-arrow/pkg/otel/arrow_record/mock"
	otelAssert "github.com/open-telemetry/otel-arrow/pkg/otel/assert"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"golang.org/x/net/http2/hpack"
	"google.golang.org/grpc/metadata"

	"github.com/open-telemetry/otel-arrow/collector/receiver/otelarrowreceiver/internal/arrow/mock"
	"github.com/open-telemetry/otel-arrow/collector/testdata"
	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/extension/auth"
	"go.opentelemetry.io/collector/obsreport"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/receiver"
)

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
	Data interface{}
}

type commonTestCase struct {
	*testing.T

	ctrl      *gomock.Controller
	cancel    context.CancelFunc
	telset    component.TelemetrySettings
	consumers mockConsumers
	stream    *arrowCollectorMock.MockArrowStreamService_ArrowStreamServer
	receive   chan recvResult
	consume   chan consumeResult
	streamErr chan error

	// testProducer is for convenience -- not thread safe, see copyBatch().
	testProducer *arrowRecord.Producer

	ctxCall  *gomock.Call
	recvCall *gomock.Call
}

type testChannel interface {
	onConsume() error
}

type healthyTestChannel struct{}

func (healthyTestChannel) onConsume() error {
	return nil
}

type unhealthyTestChannel struct{}

func (unhealthyTestChannel) onConsume() error {
	return fmt.Errorf("consumer unhealthy")
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
		return tc.onConsume()
	}
}

func (ctc *commonTestCase) doAndReturnConsumeMetrics(tc testChannel) func(ctx context.Context, metrics pmetric.Metrics) error {
	return func(ctx context.Context, metrics pmetric.Metrics) error {
		ctc.consume <- consumeResult{
			Ctx:  ctx,
			Data: metrics,
		}
		return tc.onConsume()
	}
}

func (ctc *commonTestCase) doAndReturnConsumeLogs(tc testChannel) func(ctx context.Context, logs plog.Logs) error {
	return func(ctx context.Context, logs plog.Logs) error {
		ctc.consume <- consumeResult{
			Ctx:  ctx,
			Data: logs,
		}
		return tc.onConsume()
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
	stream := arrowCollectorMock.NewMockArrowStreamService_ArrowStreamServer(ctrl)

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

func statusInvalidFor(batchID int64, msg string) *arrowpb.BatchStatus {
	return &arrowpb.BatchStatus{
		BatchId:       batchID,
		StatusCode:    arrowpb.StatusCode_INVALID_ARGUMENT,
		StatusMessage: msg,
	}
}

func statusExhaustedFor(batchID int64, msg string) *arrowpb.BatchStatus {
	return &arrowpb.BatchStatus{
		BatchId:       batchID,
		StatusCode:    arrowpb.StatusCode_RESOURCE_EXHAUSTED,
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

func (ctc *commonTestCase) start(newConsumer func() arrowRecord.ConsumerAPI, opts ...func(*configgrpc.GRPCServerSettings, *auth.Server)) {
	var authServer auth.Server
	gsettings := &configgrpc.GRPCServerSettings{}
	for _, gf := range opts {
		gf(gsettings, &authServer)
	}
	rc := receiver.CreateSettings{
		TelemetrySettings: ctc.telset,
		BuildInfo:         component.NewDefaultBuildInfo(),
	}
	obsrecv, err := obsreport.NewReceiver(obsreport.ReceiverSettings{
		ReceiverID:             component.NewID("arrowtest"),
		Transport:              "grpc",
		ReceiverCreateSettings: rc,
	})
	require.NoError(ctc.T, err)

	rcvr := New(
		ctc.consumers,
		rc,
		obsrecv,
		gsettings,
		authServer,
		newConsumer,
	)
	go func() {
		ctc.streamErr <- rcvr.ArrowStream(ctc.stream)
	}()
}

func TestReceiverTraces(t *testing.T) {
	tc := healthyTestChannel{}
	ctc := newCommonTestCase(t, tc)

	td := testdata.GenerateTraces(2)
	batch, err := ctc.testProducer.BatchArrowRecordsFromTraces(td)
	require.NoError(t, err)

	ctc.stream.EXPECT().Send(statusOKFor(batch.BatchId)).Times(1).Return(nil)

	ctc.start(ctc.newRealConsumer)
	ctc.putBatch(batch, nil)

	assert.EqualValues(t, td, (<-ctc.consume).Data)

	err = ctc.cancelAndWait()
	require.Error(t, err)
	require.True(t, errors.Is(err, context.Canceled))
}

func TestReceiverLogs(t *testing.T) {
	tc := healthyTestChannel{}
	ctc := newCommonTestCase(t, tc)

	ld := testdata.GenerateLogs(2)
	batch, err := ctc.testProducer.BatchArrowRecordsFromLogs(ld)
	require.NoError(t, err)

	ctc.stream.EXPECT().Send(statusOKFor(batch.BatchId)).Times(1).Return(nil)

	ctc.start(ctc.newRealConsumer)
	ctc.putBatch(batch, nil)

	assert.EqualValues(t, []json.Marshaler{compareJSONLogs{ld}}, []json.Marshaler{compareJSONLogs{(<-ctc.consume).Data.(plog.Logs)}})

	err = ctc.cancelAndWait()
	require.Error(t, err)
	require.True(t, errors.Is(err, context.Canceled), "for %v", err)
}

func TestReceiverMetrics(t *testing.T) {
	tc := healthyTestChannel{}
	ctc := newCommonTestCase(t, tc)
	stdTesting := otelAssert.NewStdUnitTest(t)

	md := testdata.GenerateMetrics(2)
	batch, err := ctc.testProducer.BatchArrowRecordsFromMetrics(md)
	require.NoError(t, err)

	ctc.stream.EXPECT().Send(statusOKFor(batch.BatchId)).Times(1).Return(nil)

	ctc.start(ctc.newRealConsumer)
	ctc.putBatch(batch, nil)

	otelAssert.Equiv(stdTesting, []json.Marshaler{
		compareJSONMetrics{md},
	}, []json.Marshaler{
		compareJSONMetrics{(<-ctc.consume).Data.(pmetric.Metrics)},
	})

	err = ctc.cancelAndWait()
	require.Error(t, err)
	require.True(t, errors.Is(err, context.Canceled), "for %v", err)
}

func TestReceiverRecvError(t *testing.T) {
	tc := healthyTestChannel{}
	ctc := newCommonTestCase(t, tc)

	ctc.start(ctc.newRealConsumer)

	ctc.putBatch(nil, fmt.Errorf("test recv error"))

	err := ctc.wait()
	require.Error(t, err)
	require.Contains(t, err.Error(), "test recv error")
}

func TestReceiverSendError(t *testing.T) {
	tc := healthyTestChannel{}
	ctc := newCommonTestCase(t, tc)

	ld := testdata.GenerateLogs(2)
	batch, err := ctc.testProducer.BatchArrowRecordsFromLogs(ld)
	require.NoError(t, err)

	ctc.stream.EXPECT().Send(statusOKFor(batch.BatchId)).Times(1).Return(fmt.Errorf("test send error"))

	ctc.start(ctc.newRealConsumer)
	ctc.putBatch(batch, nil)

	assert.EqualValues(t, ld, (<-ctc.consume).Data)

	err = ctc.wait()
	require.Error(t, err)
	require.Contains(t, err.Error(), "test send error")
}

func TestReceiverConsumeError(t *testing.T) {
	stdTesting := otelAssert.NewStdUnitTest(t)

	data := []interface{}{
		testdata.GenerateTraces(2),
		testdata.GenerateMetrics(2),
		testdata.GenerateLogs(2),
	}

	for _, item := range data {
		tc := unhealthyTestChannel{}
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

		ctc.start(ctc.newRealConsumer)

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
		require.Error(t, err)
		require.True(t, errors.Is(err, context.Canceled), "for %v", err)
	}
}

func TestReceiverInvalidData(t *testing.T) {
	data := []interface{}{
		testdata.GenerateTraces(2),
		testdata.GenerateMetrics(2),
		testdata.GenerateLogs(2),
	}

	for _, item := range data {
		tc := unhealthyTestChannel{}
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

		ctc.stream.EXPECT().Send(statusInvalidFor(batch.BatchId, "Permanent error: test invalid error")).Times(1).Return(nil)

		ctc.start(ctc.newErrorConsumer)
		ctc.putBatch(batch, nil)

		err = ctc.cancelAndWait()
		require.Error(t, err)
		require.True(t, errors.Is(err, context.Canceled), "for %v", err)
	}
}

func TestReceiverMemoryLimit(t *testing.T) {
	data := []interface{}{
		testdata.GenerateTraces(2),
		testdata.GenerateMetrics(2),
		testdata.GenerateLogs(2),
	}

	for _, item := range data {
		tc := healthyTestChannel{}
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

		ctc.stream.EXPECT().Send(statusExhaustedFor(batch.BatchId, "Permanent error: test oom error "+arrowRecord.ErrConsumerMemoryLimit.Error())).Times(1).Return(nil)

		ctc.start(ctc.newOOMConsumer)
		ctc.putBatch(batch, nil)

		err = ctc.cancelAndWait()
		require.Error(t, err)
		require.True(t, errors.Is(err, context.Canceled), "for %v", err)
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
	tc := healthyTestChannel{}
	ctc := newCommonTestCase(t, tc)

	// send a sequence of data then simulate closing the connection.
	const times = 10

	var actualData []ptrace.Traces
	var expectData []ptrace.Traces

	ctc.stream.EXPECT().Send(gomock.Any()).Times(times + 1).Return(nil)

	ctc.start(ctc.newRealConsumer)

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
		err := ctc.wait()
		// Once receiver receives EOF it will call Send() to signal shutdown for the client.
		// Then the receiver returns nil, so we expect no error in this case
		require.NoError(t, err)
		wg.Done()
	}()

	for i := 0; i < times; i++ {
		actualData = append(actualData, (<-ctc.consume).Data.(ptrace.Traces))
	}

	assert.EqualValues(t, expectData, actualData)

	wg.Wait()
}

func TestReceiverHeadersNoAuth(t *testing.T) {
	t.Run("include", func(t *testing.T) { testReceiverHeaders(t, true) })
	t.Run("noinclude", func(t *testing.T) { testReceiverHeaders(t, false) })
}

func testReceiverHeaders(t *testing.T, includeMeta bool) {
	tc := healthyTestChannel{}
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

	ctc.stream.EXPECT().Send(gomock.Any()).Times(len(expectData) + 1).Return(nil)

	ctc.start(ctc.newRealConsumer, func(gsettings *configgrpc.GRPCServerSettings, _ *auth.Server) {
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
		err := ctc.wait()
		require.NoError(t, err)
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
	tc := healthyTestChannel{}
	ctc := newCommonTestCase(t, tc)

	ctc.cancel()
	ctc.start(ctc.newRealConsumer)

	err := ctc.wait()
	require.Error(t, err)
	require.True(t, errors.Is(err, context.Canceled))
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
	tc := healthyTestChannel{}
	ctc := newCommonTestCase(t, tc)

	expectData := []map[string][]string{
		{"auth": []string{"true"}},
		nil,
		{"auth": []string{"false"}},
		nil,
	}

	var recvBatches []*arrowpb.BatchStatus

	ctc.stream.EXPECT().Send(gomock.Any()).Times(len(expectData) + 1).DoAndReturn(func(batch *arrowpb.BatchStatus) error {
		recvBatches = append(recvBatches, batch)
		return nil
	})

	var authCall *gomock.Call
	ctc.start(ctc.newRealConsumer, func(gsettings *configgrpc.GRPCServerSettings, authPtr *auth.Server) {
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

	var expectErrs []bool

	for _, testInput := range expectData {
		// The static stream context contains one extra variable.
		cpy := map[string][]string{}
		cpy["stream_ctx"] = []string{"per-request"}

		for k, v := range testInput {
			cpy[k] = v
		}

		expectErr := false
		if dataAuth {
			hasAuth := false
			for _, val := range cpy["auth"] {
				hasAuth = hasAuth || (val == "true")
			}
			if hasAuth {
				cpy["has_auth"] = []string{":+1:", ":100:"}
			} else {
				expectErr = true
			}
		}

		expectErrs = append(expectErrs, expectErr)

		if expectErr {
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

	err := ctc.wait()
	require.NoError(t, err)
	// Add in expectErrs for when receiver sees EOF,
	// the status code will not be arrowpb.StatusCode_OK.
	expectErrs = append(expectErrs, true)

	require.Equal(t, len(expectData), dataCount)

	// recvBatches will be 1 more than dataCount, because of the extra
	// Send() call the receiver makes when it encounters EOF.
	require.Equal(t, len(recvBatches), dataCount+1)

	for idx, batch := range recvBatches {
		if expectErrs[idx] {
			require.NotEqual(t, arrowpb.StatusCode_OK, batch.StatusCode)
		} else {
			require.Equal(t, arrowpb.StatusCode_OK, batch.StatusCode)
		}
	}
}
