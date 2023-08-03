// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package octrace

import (
	"context"
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
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

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
	tt, err := obsreporttest.SetupTelemetry(component.NewID("opencensus"))
	require.NoError(t, err)
	defer func() {
		require.NoError(t, tt.Shutdown(context.Background()))
	}()

	addr, doneReceiverFn := ocReceiverOnGRPCServer(t, consumertest.NewNop(), tt.ToReceiverCreateSettings())
	defer doneReceiverFn()

	n := 20
	// Now for the traceExporter that sends 0 length spans
	traceSvcClient, traceSvcDoneFn, err := makeTraceServiceClient(addr)
	require.NoError(t, err, "Failed to create the trace service client: %v", err)
	spans := []*tracepb.Span{{TraceId: []byte("abcdefghijklmnop"), SpanId: []byte("12345678")}}
	for i := 0; i < n; i++ {
		err = traceSvcClient.Send(&agenttracepb.ExportTraceServiceRequest{Spans: spans, Node: &commonpb.Node{}})
		require.NoError(t, err, "Failed to send requests to the service: %v", err)
	}
	flush(traceSvcDoneFn)

	require.NoError(t, tt.CheckReceiverTraces("grpc", int64(n), 0))
}

func TestEnsureRecordedMetrics_zeroLengthSpansSender(t *testing.T) {
	tt, err := obsreporttest.SetupTelemetry(component.NewID("opencensus"))
	require.NoError(t, err)
	defer func() {
		require.NoError(t, tt.Shutdown(context.Background()))
	}()

	port, doneFn := ocReceiverOnGRPCServer(t, consumertest.NewNop(), tt.ToReceiverCreateSettings())
	defer doneFn()

	n := 20
	// Now for the traceExporter that sends 0 length spans
	traceSvcClient, traceSvcDoneFn, err := makeTraceServiceClient(port)
	require.NoError(t, err, "Failed to create the trace service client: %v", err)
	for i := 0; i <= n; i++ {
		err = traceSvcClient.Send(&agenttracepb.ExportTraceServiceRequest{Spans: nil, Node: &commonpb.Node{}})
		require.NoError(t, err, "Failed to send requests to the service: %v", err)
	}
	flush(traceSvcDoneFn)

	require.NoError(t, tt.CheckReceiverTraces("grpc", 0, 0))
}

func TestExportSpanLinkingMaintainsParentLink(t *testing.T) {
	tt, err := obsreporttest.SetupTelemetry(component.NewID("opencensus"))
	require.NoError(t, err)
	defer func() {
		require.NoError(t, tt.Shutdown(context.Background()))
	}()

	otel.SetTracerProvider(tt.TracerProvider)
	defer otel.SetTracerProvider(trace.NewNoopTracerProvider())

	port, doneFn := ocReceiverOnGRPCServer(t, consumertest.NewNop(), tt.ToReceiverCreateSettings())
	defer doneFn()

	traceSvcClient, traceSvcDoneFn, err := makeTraceServiceClient(port)
	require.NoError(t, err, "Failed to create the trace service client: %v", err)

	n := 5
	for i := 0; i < n; i++ {
		sl := []*tracepb.Span{{TraceId: []byte("abcdefghijklmnop"), SpanId: []byte{byte(i + 1), 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}}}
		err = traceSvcClient.Send(&agenttracepb.ExportTraceServiceRequest{Spans: sl, Node: &commonpb.Node{}})
		require.NoError(t, err, "Failed to send requests to the service: %v", err)
	}

	flush(traceSvcDoneFn)

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

// TODO: Determine how to do this deterministic.
func flush(traceSvcDoneFn func()) {
	// Give it enough time to process the streamed spans.
	<-time.After(40 * time.Millisecond)

	// End the gRPC service to complete the RPC trace so that we
	// can examine the RPC trace as well.
	traceSvcDoneFn()

	// Give it some more time to complete the RPC trace and export.
	<-time.After(40 * time.Millisecond)
}
