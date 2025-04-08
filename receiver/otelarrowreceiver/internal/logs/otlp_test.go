// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logs

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
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/plog/plogotlp"
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
	consumertest.LogsSink
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

func (ts *testSink) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	<-ts.Done()
	return ts.LogsSink.ConsumeLogs(ctx, ld)
}

func TestExport_Success(t *testing.T) {
	ld := testdata.GenerateLogs(1)
	req := plogotlp.NewExportRequestFromLogs(ld)

	logSink := newTestSink()
	logsClient, selfExp, selfProv := makeTraceServiceClient(t, logSink)

	go logSink.unblock()
	resp, err := logsClient.Export(context.Background(), req)
	require.NoError(t, err, "Failed to export trace: %v", err)
	require.NotNil(t, resp, "The response is missing")

	require.Len(t, logSink.AllLogs(), 1)
	assert.EqualValues(t, ld, logSink.AllLogs()[0])

	// One self-tracing spans is issued.
	require.NoError(t, selfProv.ForceFlush(context.Background()))
	require.Len(t, selfExp.GetSpans(), 1)
}

func TestExport_EmptyRequest(t *testing.T) {
	logSink := newTestSink()
	logsClient, selfExp, selfProv := makeTraceServiceClient(t, logSink)
	empty := plogotlp.NewExportRequest()

	go logSink.unblock()
	resp, err := logsClient.Export(context.Background(), empty)
	assert.NoError(t, err, "Failed to export trace: %v", err)
	assert.NotNil(t, resp, "The response is missing")

	require.Empty(t, logSink.AllLogs())

	// No self-tracing spans are issued.
	require.NoError(t, selfProv.ForceFlush(context.Background()))
	require.Empty(t, selfExp.GetSpans())
}

func TestExport_ErrorConsumer(t *testing.T) {
	ld := testdata.GenerateLogs(1)
	req := plogotlp.NewExportRequestFromLogs(ld)

	logsClient, selfExp, selfProv := makeTraceServiceClient(t, consumertest.NewErr(errors.New("my error")))
	resp, err := logsClient.Export(context.Background(), req)
	assert.EqualError(t, err, "rpc error: code = Unknown desc = my error")
	assert.Equal(t, plogotlp.ExportResponse{}, resp)

	// One self-tracing spans is issued.
	require.NoError(t, selfProv.ForceFlush(context.Background()))
	require.Len(t, selfExp.GetSpans(), 1)
}

func TestExport_AdmissionRequestTooLarge(t *testing.T) {
	ld := testdata.GenerateLogs(10)
	logSink := newTestSink()
	req := plogotlp.NewExportRequestFromLogs(ld)
	logsClient, selfExp, selfProv := makeTraceServiceClient(t, logSink)

	go logSink.unblock()
	resp, err := logsClient.Export(context.Background(), req)
	assert.EqualError(t, err, "rpc error: code = InvalidArgument desc = rejecting request, request is too large")
	assert.Equal(t, plogotlp.ExportResponse{}, resp)

	// One self-tracing spans is issued.
	require.NoError(t, selfProv.ForceFlush(context.Background()))
	require.Len(t, selfExp.GetSpans(), 1)
}

func TestExport_AdmissionLimitExceeded(t *testing.T) {
	ld := testdata.GenerateLogs(1)
	logSink := newTestSink()
	req := plogotlp.NewExportRequestFromLogs(ld)

	logsClient, selfExp, selfProv := makeTraceServiceClient(t, logSink)

	var wait sync.WaitGroup
	wait.Add(10)

	var expectSuccess atomic.Int32

	for i := 0; i < 10; i++ {
		go func() {
			defer wait.Done()
			_, err := logsClient.Export(context.Background(), req)
			if err == nil {
				// some succeed!
				expectSuccess.Add(1)
				return
			}
			assert.EqualError(t, err, "rpc error: code = ResourceExhausted desc = rejecting request, too much pending data")
		}()
	}

	logSink.unblock()
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

func makeTraceServiceClient(t *testing.T, lc consumer.Logs) (plogotlp.GRPCClient, *tracetest.InMemoryExporter, *trace.TracerProvider) {
	addr, exp, tp := otlpReceiverOnGRPCServer(t, lc)
	cc, err := grpc.NewClient(addr.String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err, "Failed to create the TraceServiceClient: %v", err)
	t.Cleanup(func() {
		require.NoError(t, cc.Close())
	})

	return plogotlp.NewGRPCClient(cc), exp, tp
}

func otlpReceiverOnGRPCServer(t *testing.T, lc consumer.Logs) (net.Addr, *tracetest.InMemoryExporter, *trace.TracerProvider) {
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

	set.ID = component.NewIDWithName(component.MustNewType("otlp"), "logs")
	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
		ReceiverID:             set.ID,
		Transport:              "grpc",
		ReceiverCreateSettings: set,
	})
	require.NoError(t, err)
	bq, err := admission2.NewBoundedQueue(set.ID, telset, maxBytes, 0)
	require.NoError(t, err)
	r := New(zap.NewNop(), lc, obsrecv, bq)
	// Now run it as a gRPC server
	srv := grpc.NewServer()
	plogotlp.RegisterGRPCServer(srv, r)
	go func() {
		_ = srv.Serve(ln)
	}()

	return ln.Addr(), exp, tp
}
