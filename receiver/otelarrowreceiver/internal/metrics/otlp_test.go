// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/otelarrow/testdata"
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
	r := New(mc, obsrecv)
	// Now run it as a gRPC server
	srv := grpc.NewServer()
	pmetricotlp.RegisterGRPCServer(srv, r)
	go func() {
		_ = srv.Serve(ln)
	}()

	return ln.Addr()
}
