// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

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
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
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
	md := testdata.GenerateMetrics(1)
	req := pmetricotlp.NewExportRequestFromMetrics(md)

	metricSink := new(consumertest.MetricsSink)
	metricsClient := makeMetricsServiceClient(t, metricSink)
	resp, err := metricsClient.Export(context.Background(), req)

	require.NoError(t, err, "Failed to export metrics: %v", err)
	require.NotNil(t, resp, "The response is missing")

	mds := metricSink.AllMetrics()
	require.Len(t, mds, 1)
	assert.EqualValues(t, md, mds[0])
}

func TestExport_EmptyRequest(t *testing.T) {
	metricSink := new(consumertest.MetricsSink)
	metricsClient := makeMetricsServiceClient(t, metricSink)
	resp, err := metricsClient.Export(context.Background(), pmetricotlp.NewExportRequest())
	require.NoError(t, err)
	require.NotNil(t, resp)
}

func TestExport_ErrorConsumer(t *testing.T) {
	md := testdata.GenerateMetrics(1)
	req := pmetricotlp.NewExportRequestFromMetrics(md)

	metricsClient := makeMetricsServiceClient(t, consumertest.NewErr(errors.New("my error")))
	resp, err := metricsClient.Export(context.Background(), req)
	assert.EqualError(t, err, "rpc error: code = Unknown desc = my error")
	assert.Equal(t, pmetricotlp.ExportResponse{}, resp)
}

func TestExport_AdmissionLimitBytesExceeded(t *testing.T) {
	md := testdata.GenerateMetrics(10)
	metricSink := new(consumertest.MetricsSink)
	req := pmetricotlp.NewExportRequestFromMetrics(md)

	metricsClient := makeMetricsServiceClient(t, metricSink)
	resp, err := metricsClient.Export(context.Background(), req)
	assert.EqualError(t, err, "rpc error: code = Unknown desc = rejecting request, request size larger than configured limit")
	assert.Equal(t, pmetricotlp.ExportResponse{}, resp)
}

func TestExport_TooManyWaiters(t *testing.T) {
	bc := testconsumer.NewBlockingConsumer()

	metricsClient := makeMetricsServiceClient(t, bc)
	bg := context.Background()
	var errs, err error
	md := testdata.GenerateMetrics(1)
	req := pmetricotlp.NewExportRequestFromMetrics(md)
	var mtx sync.Mutex
	numResponses := 0
	// Send request that will acquire all of the semaphores bytes and block.
	go func() {
		_, err = metricsClient.Export(bg, req)
		mtx.Lock()
		errs = multierr.Append(errs, err)
		numResponses++
		mtx.Unlock()
	}()

	for i := 0; i < maxWaiters+1; i++ {
		go func() {
			_, err := metricsClient.Export(bg, req)
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

func makeMetricsServiceClient(t *testing.T, mc consumer.Metrics) pmetricotlp.GRPCClient {
	addr := otlpReceiverOnGRPCServer(t, mc)

	cc, err := grpc.NewClient(addr.String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err, "Failed to create the MetricsServiceClient: %v", err)
	t.Cleanup(func() {
		require.NoError(t, cc.Close())
	})

	return pmetricotlp.NewGRPCClient(cc)
}

func otlpReceiverOnGRPCServer(t *testing.T, mc consumer.Metrics) net.Addr {
	ln, err := net.Listen("tcp", "localhost:")
	require.NoError(t, err, "Failed to find an available address to run the gRPC server: %v", err)

	t.Cleanup(func() {
		require.NoError(t, ln.Close())
	})

	set := receivertest.NewNopSettings()
	set.ID = component.NewIDWithName(component.MustNewType("otlp"), "metrics")
	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
		ReceiverID:             set.ID,
		Transport:              "grpc",
		ReceiverCreateSettings: set,
	})
	require.NoError(t, err)

	bq := admission.NewBoundedQueue(noop.NewTracerProvider(), maxBytes, maxWaiters)
	r := New(zap.NewNop(), mc, obsrecv, bq)
	// Now run it as a gRPC server
	srv := grpc.NewServer()
	pmetricotlp.RegisterGRPCServer(srv, r)
	go func() {
		_ = srv.Serve(ln)
	}()

	return ln.Addr()
}
