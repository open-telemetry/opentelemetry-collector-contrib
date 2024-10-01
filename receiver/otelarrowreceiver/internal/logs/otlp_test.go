// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logs

import (
	"context"
	"errors"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog/plogotlp"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/otel/trace/noop"
	"go.uber.org/multierr"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/otelarrow/admission"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/otelarrow/testdata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/otelarrowreceiver/internal/testconsumer"
)

const (
	maxWaiters = 10
	maxBytes   = int64(250)
)

func TestExport(t *testing.T) {
	ld := testdata.GenerateLogs(1)
	req := plogotlp.NewExportRequestFromLogs(ld)

	logSink := new(consumertest.LogsSink)
	logClient := makeLogsServiceClient(t, logSink)
	resp, err := logClient.Export(context.Background(), req)
	require.NoError(t, err, "Failed to export trace: %v", err)
	require.NotNil(t, resp, "The response is missing")

	lds := logSink.AllLogs()
	require.Len(t, lds, 1)
	assert.EqualValues(t, ld, lds[0])
}

func TestExport_EmptyRequest(t *testing.T) {
	logSink := new(consumertest.LogsSink)

	logClient := makeLogsServiceClient(t, logSink)
	resp, err := logClient.Export(context.Background(), plogotlp.NewExportRequest())
	assert.NoError(t, err, "Failed to export trace: %v", err)
	assert.NotNil(t, resp, "The response is missing")
}

func TestExport_ErrorConsumer(t *testing.T) {
	ld := testdata.GenerateLogs(1)
	req := plogotlp.NewExportRequestFromLogs(ld)

	logClient := makeLogsServiceClient(t, consumertest.NewErr(errors.New("my error")))
	resp, err := logClient.Export(context.Background(), req)
	assert.EqualError(t, err, "rpc error: code = Unknown desc = my error")
	assert.Equal(t, plogotlp.ExportResponse{}, resp)
}

func TestExport_AdmissionLimitBytesExceeded(t *testing.T) {
	ld := testdata.GenerateLogs(10)
	logSink := new(consumertest.LogsSink)
	req := plogotlp.NewExportRequestFromLogs(ld)

	logClient := makeLogsServiceClient(t, logSink)
	resp, err := logClient.Export(context.Background(), req)
	assert.EqualError(t, err, "rpc error: code = Unknown desc = rejecting request, request size larger than configured limit")
	assert.Equal(t, plogotlp.ExportResponse{}, resp)
}

func TestExport_TooManyWaiters(t *testing.T) {
	bc := testconsumer.NewBlockingConsumer()

	logsClient := makeLogsServiceClient(t, bc)
	bg := context.Background()
	var errs, err error
	ld := testdata.GenerateLogs(1)
	req := plogotlp.NewExportRequestFromLogs(ld)
	var mtx sync.Mutex
	numResponses := 0
	// Send request that will acquire all of the semaphores bytes and block.
	go func() {
		_, err = logsClient.Export(bg, req)
		mtx.Lock()
		errs = multierr.Append(errs, err)
		numResponses++
		mtx.Unlock()
	}()

	for i := 0; i < maxWaiters+1; i++ {
		go func() {
			_, err := logsClient.Export(bg, req)
			mtx.Lock()
			errs = multierr.Append(errs, err)
			numResponses++
			mtx.Unlock()
		}()
	}

	// sleep so all async requests are blocked on semaphore Acquire.
	time.Sleep(1 * time.Second)

	// unblock and wait for errors to be returned and written.
	bc.Unblock()
	assert.Eventually(t, func() bool {
		mtx.Lock()
		defer mtx.Unlock()
		errSlice := multierr.Errors(errs)
		return numResponses == maxWaiters+2 && len(errSlice) == 1
	}, 3*time.Second, 10*time.Millisecond)

	assert.ErrorContains(t, errs, "too many waiters")
}

func makeLogsServiceClient(t *testing.T, lc consumer.Logs) plogotlp.GRPCClient {
	addr := otlpReceiverOnGRPCServer(t, lc)
	cc, err := grpc.NewClient(addr.String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err, "Failed to create the TraceServiceClient: %v", err)
	t.Cleanup(func() {
		require.NoError(t, cc.Close())
	})

	return plogotlp.NewGRPCClient(cc)
}

func otlpReceiverOnGRPCServer(t *testing.T, lc consumer.Logs) net.Addr {
	ln, err := net.Listen("tcp", "localhost:")
	require.NoError(t, err, "Failed to find an available address to run the gRPC server: %v", err)

	t.Cleanup(func() {
		require.NoError(t, ln.Close())
	})

	set := receivertest.NewNopSettings()
	set.ID = component.NewIDWithName(component.MustNewType("otlp"), "log")
	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
		ReceiverID:             set.ID,
		Transport:              "grpc",
		ReceiverCreateSettings: set,
	})
	require.NoError(t, err)

	bq := admission.NewBoundedQueue(noop.NewTracerProvider(), maxBytes, maxWaiters)
	r := New(zap.NewNop(), lc, obsrecv, bq)
	// Now run it as a gRPC server
	srv := grpc.NewServer()
	plogotlp.RegisterGRPCServer(srv, r)
	go func() {
		_ = srv.Serve(ln)
	}()

	return ln.Addr()
}
