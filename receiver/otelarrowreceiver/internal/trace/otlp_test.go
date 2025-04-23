// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package trace

import (
	"context"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/otelarrow/admission2"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/otelarrow/testdata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/otelarrowreceiver/internal/metadata"
)

const (
	maxBytes = 250
)

type testSink struct {
	consumertest.TracesSink
	context.Context
	context.CancelFunc
}

func newTestSink() *testSink {
	ctx, cancel := context.WithCancel(context.Background())
	return &testSink{
		Context:    ctx,
		CancelFunc: cancel,
	}
}

func (ts *testSink) unblock() {
	time.Sleep(10 * time.Millisecond)
	ts.CancelFunc()
}

func (ts *testSink) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	<-ts.Done()
	return ts.TracesSink.ConsumeTraces(ctx, td)
}

func TestExport_Success(t *testing.T) {
	td := testdata.GenerateTraces(1)
	req := ptraceotlp.NewExportRequestFromTraces(td)

	traceSink := newTestSink()
	traceClient, selfExp, selfProv := makeTraceServiceClient(t, traceSink)

	go traceSink.unblock()
	resp, err := traceClient.Export(context.Background(), req)
	require.NoError(t, err, "Failed to export trace: %v", err)
	require.NotNil(t, resp, "The response is missing")

	require.Len(t, traceSink.AllTraces(), 1)
	assert.Equal(t, td, traceSink.AllTraces()[0])

	// One self-tracing spans is issued.
	require.NoError(t, selfProv.ForceFlush(context.Background()))
	require.Len(t, selfExp.GetSpans(), 1)
}

func TestExport_EmptyRequest(t *testing.T) {
	traceSink := newTestSink()
	traceClient, selfExp, selfProv := makeTraceServiceClient(t, traceSink)
	empty := ptraceotlp.NewExportRequest()

	go traceSink.unblock()
	resp, err := traceClient.Export(context.Background(), empty)
	assert.NoError(t, err, "Failed to export trace: %v", err)
	assert.NotNil(t, resp, "The response is missing")

	require.Empty(t, traceSink.AllTraces())

	// No self-tracing spans are issued.
	require.NoError(t, selfProv.ForceFlush(context.Background()))
	require.Empty(t, selfExp.GetSpans())
}

func TestExport_ErrorConsumer(t *testing.T) {
	td := testdata.GenerateTraces(1)
	req := ptraceotlp.NewExportRequestFromTraces(td)

	traceClient, selfExp, selfProv := makeTraceServiceClient(t, consumertest.NewErr(errors.New("my error")))
	resp, err := traceClient.Export(context.Background(), req)
	assert.EqualError(t, err, "rpc error: code = Unknown desc = my error")
	assert.Equal(t, ptraceotlp.ExportResponse{}, resp)

	// One self-tracing spans is issued.
	require.NoError(t, selfProv.ForceFlush(context.Background()))
	require.Len(t, selfExp.GetSpans(), 1)
}

func TestExport_AdmissionRequestTooLarge(t *testing.T) {
	td := testdata.GenerateTraces(10)
	traceSink := newTestSink()
	req := ptraceotlp.NewExportRequestFromTraces(td)
	traceClient, selfExp, selfProv := makeTraceServiceClient(t, traceSink)

	go traceSink.unblock()
	resp, err := traceClient.Export(context.Background(), req)
	assert.EqualError(t, err, "rpc error: code = InvalidArgument desc = rejecting request, request is too large")
	assert.Equal(t, ptraceotlp.ExportResponse{}, resp)

	// One self-tracing spans is issued.
	require.NoError(t, selfProv.ForceFlush(context.Background()))
	require.Len(t, selfExp.GetSpans(), 1)
}

func TestExport_AdmissionLimitExceeded(t *testing.T) {
	td := testdata.GenerateTraces(1)
	traceSink := newTestSink()
	req := ptraceotlp.NewExportRequestFromTraces(td)

	traceClient, selfExp, selfProv := makeTraceServiceClient(t, traceSink)

	var wait sync.WaitGroup
	wait.Add(10)

	var expectSuccess atomic.Int32

	for i := 0; i < 10; i++ {
		go func() {
			defer wait.Done()
			_, err := traceClient.Export(context.Background(), req)
			if err == nil {
				// some succeed!
				expectSuccess.Add(1)
				return
			}
			assert.EqualError(t, err, "rpc error: code = ResourceExhausted desc = rejecting request, too much pending data")
		}()
	}

	traceSink.unblock()
	wait.Wait()

	// 10 self-tracing spans are issued
	require.NoError(t, selfProv.ForceFlush(context.Background()))
	require.Len(t, selfExp.GetSpans(), 10)

	// Expect the correct number of success and failure.
	testSuccess := 0
	for _, span := range selfExp.GetSpans() {
		switch span.Status.Code {
		case codes.Ok, codes.Unset:
			testSuccess++
		}
	}
	require.Equal(t, int(expectSuccess.Load()), testSuccess)
}

func makeTraceServiceClient(t *testing.T, tc consumer.Traces) (ptraceotlp.GRPCClient, *tracetest.InMemoryExporter, *trace.TracerProvider) {
	addr, exp, tp := otlpReceiverOnGRPCServer(t, tc)
	cc, err := grpc.NewClient(addr.String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err, "Failed to create the TraceServiceClient: %v", err)
	t.Cleanup(func() {
		require.NoError(t, cc.Close())
	})

	return ptraceotlp.NewGRPCClient(cc), exp, tp
}

func otlpReceiverOnGRPCServer(t *testing.T, tc consumer.Traces) (net.Addr, *tracetest.InMemoryExporter, *trace.TracerProvider) {
	ln, err := net.Listen("tcp", "localhost:")
	require.NoError(t, err, "Failed to find an available address to run the gRPC server: %v", err)

	t.Cleanup(func() {
		require.NoError(t, ln.Close())
	})

	exp := tracetest.NewInMemoryExporter()

	tp := trace.NewTracerProvider(trace.WithSyncer(exp))
	telset := componenttest.NewNopTelemetrySettings()
	telset.TracerProvider = tp

	set := receivertest.NewNopSettings(metadata.Type)
	set.TelemetrySettings = telset

	set.ID = component.NewIDWithName(component.MustNewType("otlp"), "trace")
	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
		ReceiverID:             set.ID,
		Transport:              "grpc",
		ReceiverCreateSettings: set,
	})
	require.NoError(t, err)
	bq, err := admission2.NewBoundedQueue(set.ID, telset, maxBytes, 0)
	require.NoError(t, err)
	r := New(zap.NewNop(), tc, obsrecv, bq)
	// Now run it as a gRPC server
	srv := grpc.NewServer()
	ptraceotlp.RegisterGRPCServer(srv, r)
	go func() {
		_ = srv.Serve(ln)
	}()

	return ln.Addr(), exp, tp
}
