// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package octrace

import (
	"context"
	"errors"
	"io"
	"runtime"
	"testing"
	"time"

	commonpb "github.com/census-instrumentation/opencensus-proto/gen-go/agent/common/v1"
	agenttracepb "github.com/census-instrumentation/opencensus-proto/gen-go/agent/trace/v1"
	tracepb "github.com/census-instrumentation/opencensus-proto/gen-go/trace/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/obsreport/obsreporttest"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	nooptrace "go.opentelemetry.io/otel/trace/noop"
	"google.golang.org/grpc/status"
)

const (
	traceSvcTimeout = 10 * time.Second
)

var receiverID = component.NewID("opencensus")

// Ensure that if we add a metrics exporter that our target metrics
// will be recorded but also with the proper tag keys and values.
// See Issue https://github.com/census-instrumentation/opencensus-service/issues/63
//
// Note: we are intentionally skipping the ocgrpc.ServerDefaultViews as this
// test is to ensure exactness, but with the mentioned views registered, the
// output will be quite noisy.
func TestEnsureRecordedMetrics(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping test on Windows, see https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/17574")
	}
	tt, err := obsreporttest.SetupTelemetry(receiverID)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, tt.Shutdown(context.Background()))
	}()

	addr, doneReceiverFn := ocReceiverOnGRPCServer(t, consumertest.NewNop(), receiver.CreateSettings{ID: receiverID, TelemetrySettings: tt.TelemetrySettings, BuildInfo: component.NewDefaultBuildInfo()})
	defer doneReceiverFn()

	ctx, cancel := context.WithTimeout(context.Background(), traceSvcTimeout)
	defer cancel()

	n := 20
	// Now for the traceExporter that sends 0 length spans
	traceConn, traceClient, err := makeTraceServiceClient(ctx, addr)
	require.NoError(t, err, "Failed to create the trace service client: %v", err)
	defer traceConn.Close()

	spans := []*tracepb.Span{{TraceId: []byte("abcdefghijklmnop"), SpanId: []byte("12345678")}}
	for i := 0; i < n; i++ {
		err = traceClient.Send(&agenttracepb.ExportTraceServiceRequest{Spans: spans, Node: &commonpb.Node{}})
		require.NoError(t, err, "Failed to send requests to the service: %v", err)
	}
	err = flush(traceClient)
	require.NoError(t, err, "Failed to flush responses to the service: %v", err)

	require.NoError(t, tt.CheckReceiverTraces("grpc", int64(n), 0))
}

func TestEnsureRecordedMetrics_zeroLengthSpansSender(t *testing.T) {
	tt, err := obsreporttest.SetupTelemetry(receiverID)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, tt.Shutdown(context.Background()))
	}()

	addr, doneFn := ocReceiverOnGRPCServer(t, consumertest.NewNop(), receiver.CreateSettings{ID: receiverID, TelemetrySettings: tt.TelemetrySettings, BuildInfo: component.NewDefaultBuildInfo()})
	defer doneFn()

	ctx, cancel := context.WithTimeout(context.Background(), traceSvcTimeout)
	defer cancel()

	n := 20
	// Now for the traceExporter that sends 0 length spans
	traceConn, traceClient, err := makeTraceServiceClient(ctx, addr)
	require.NoError(t, err, "Failed to create the trace service client: %v", err)
	defer traceConn.Close()

	for i := 0; i <= n; i++ {
		err = traceClient.Send(&agenttracepb.ExportTraceServiceRequest{Spans: nil, Node: &commonpb.Node{}})
		require.NoError(t, err, "Failed to send requests to the service: %v", err)
	}
	err = flush(traceClient)
	require.NoError(t, err, "Failed to flush responses to the service: %v", err)

	require.NoError(t, tt.CheckReceiverTraces("grpc", 0, 0))
}

func TestExportSpanLinkingMaintainsParentLink(t *testing.T) {
	tt, err := obsreporttest.SetupTelemetry(receiverID)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, tt.Shutdown(context.Background()))
	}()

	otel.SetTracerProvider(tt.TracerProvider)
	defer otel.SetTracerProvider(nooptrace.NewTracerProvider())

	addr, doneFn := ocReceiverOnGRPCServer(t, consumertest.NewNop(), receiver.CreateSettings{ID: receiverID, TelemetrySettings: tt.TelemetrySettings, BuildInfo: component.NewDefaultBuildInfo()})
	defer doneFn()

	ctx, cancel := context.WithTimeout(context.Background(), traceSvcTimeout)
	defer cancel()

	traceConn, traceClient, err := makeTraceServiceClient(ctx, addr)
	require.NoError(t, err, "Failed to create the trace service client: %v", err)
	defer traceConn.Close()

	n := 5
	for i := 0; i < n; i++ {
		sl := []*tracepb.Span{{TraceId: []byte("abcdefghijklmnop"), SpanId: []byte{byte(i + 1), 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}}}
		err = traceClient.Send(&agenttracepb.ExportTraceServiceRequest{Spans: sl, Node: &commonpb.Node{}})
		require.NoError(t, err, "Failed to send requests to the service: %v", err)
	}

	err = flush(traceClient)
	require.NoError(t, err, "Failed to flush responses to the service: %v", err)

	// Inspection time!
	gotSpanData := tt.SpanRecorder.Ended()
	assert.Equal(t, n+1, len(gotSpanData))

	receiverSpanData := gotSpanData[0]
	assert.Len(t, receiverSpanData.Links(), 1)

	// The rpc span is always last in the list
	rpcSpanData := gotSpanData[len(gotSpanData)-1]

	// Ensure that the link matches up exactly!
	wantLink := sdktrace.Link{SpanContext: rpcSpanData.SpanContext()}

	assert.Equal(t, wantLink, receiverSpanData.Links()[0])
	assert.Equal(t, "receiver/opencensus/TraceDataReceived", receiverSpanData.Name())

	// And then for the receiverSpanData itself, it SHOULD NOT
	// have a ParentID, so let's enforce all the conditions below:
	// 1. That it doesn't have the RPC spanID as its ParentSpanID
	// 2. That it actually has no ParentSpanID i.e. has a blank SpanID
	assert.NotEqual(t, rpcSpanData.SpanContext().SpanID(), receiverSpanData.Parent().SpanID(),
		"ReceiverSpanData.ParentSpanID unfortunately was linked to the RPC span")
	assert.False(t, receiverSpanData.Parent().IsValid())
}

// flush wait for stream complete
func flush(traceClient agenttracepb.TraceService_ExportClient) error {
	if err := traceClient.CloseSend(); err != nil {
		return err
	}

	for {
		_, err := traceClient.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return status.Errorf(status.Code(err), "receiving stream export message: %v", err)
		}
	}

	return nil
}
