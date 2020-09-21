// Copyright 2019 OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package stackdriverexporter

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumerdata"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/testutil/metricstestutil"
	"go.opentelemetry.io/collector/translator/internaldata"
	"google.golang.org/api/option"
	cloudmetricpb "google.golang.org/genproto/googleapis/api/metric"
	cloudtracepb "google.golang.org/genproto/googleapis/devtools/cloudtrace/v2"
	cloudmonitoringpb "google.golang.org/genproto/googleapis/monitoring/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type testServer struct {
	reqCh chan *cloudtracepb.BatchWriteSpansRequest
}

func (ts *testServer) BatchWriteSpans(ctx context.Context, req *cloudtracepb.BatchWriteSpansRequest) (*emptypb.Empty, error) {
	go func() { ts.reqCh <- req }()
	return &emptypb.Empty{}, nil
}

// Creates a new span.
func (ts *testServer) CreateSpan(context.Context, *cloudtracepb.Span) (*cloudtracepb.Span, error) {
	return nil, nil
}

func TestStackdriverTraceExport(t *testing.T) {
	srv := grpc.NewServer()

	reqCh := make(chan *cloudtracepb.BatchWriteSpansRequest)

	cloudtracepb.RegisterTraceServiceServer(srv, &testServer{reqCh: reqCh})

	lis, err := net.Listen("tcp", ":8080")
	require.NoError(t, err)
	defer lis.Close()

	go srv.Serve(lis)

	sde, err := newStackdriverTraceExporter(
		&Config{ProjectID: "idk", Endpoint: "127.0.0.1:8080", UseInsecure: true},
		"v0.0.1",
	)
	require.NoError(t, err)
	defer func() { require.NoError(t, sde.Shutdown(context.Background())) }()

	testTime := time.Now()
	spanName := "foobar"

	resource := pdata.NewResource()
	resource.InitEmpty()
	traces := pdata.NewTraces()
	traces.ResourceSpans().Resize(1)
	rspans := traces.ResourceSpans().At(0)
	resource.CopyTo(rspans.Resource())
	rspans.InstrumentationLibrarySpans().Resize(1)
	ispans := rspans.InstrumentationLibrarySpans().At(0)
	ispans.Spans().Resize(1)
	span := pdata.NewSpan()
	span.InitEmpty()
	span.SetName(spanName)
	span.SetStartTime(pdata.TimestampUnixNano(testTime.UnixNano()))
	span.CopyTo(ispans.Spans().At(0))
	err = sde.ConsumeTraces(context.Background(), traces)
	assert.NoError(t, err)

	r := <-reqCh
	assert.Len(t, r.Spans, 1)
	assert.Equal(t, fmt.Sprintf("Span.internal-%s", spanName), r.Spans[0].GetDisplayName().Value)
	assert.Equal(t, timestamppb.New(testTime), r.Spans[0].StartTime)
}

type mockMetricServer struct {
	cloudmonitoringpb.MetricServiceServer

	descriptorReqCh chan *requestWithMetadata
	timeSeriesReqCh chan *requestWithMetadata
}

type requestWithMetadata struct {
	req      interface{}
	metadata metadata.MD
}

func (ms *mockMetricServer) CreateMetricDescriptor(ctx context.Context, req *cloudmonitoringpb.CreateMetricDescriptorRequest) (*cloudmetricpb.MetricDescriptor, error) {
	reqWithMetadata := &requestWithMetadata{req: req}
	metadata, ok := metadata.FromIncomingContext(ctx)
	if ok {
		reqWithMetadata.metadata = metadata
	}

	go func() { ms.descriptorReqCh <- reqWithMetadata }()
	return &cloudmetricpb.MetricDescriptor{}, nil
}

func (ms *mockMetricServer) CreateTimeSeries(ctx context.Context, req *cloudmonitoringpb.CreateTimeSeriesRequest) (*emptypb.Empty, error) {
	reqWithMetadata := &requestWithMetadata{req: req}
	metadata, ok := metadata.FromIncomingContext(ctx)
	if ok {
		reqWithMetadata.metadata = metadata
	}

	go func() { ms.timeSeriesReqCh <- reqWithMetadata }()
	return &emptypb.Empty{}, nil
}

func TestStackdriverMetricExport(t *testing.T) {
	srv := grpc.NewServer()

	descriptorReqCh := make(chan *requestWithMetadata)
	timeSeriesReqCh := make(chan *requestWithMetadata)

	mockServer := &mockMetricServer{descriptorReqCh: descriptorReqCh, timeSeriesReqCh: timeSeriesReqCh}
	cloudmonitoringpb.RegisterMetricServiceServer(srv, mockServer)

	lis, err := net.Listen("tcp", ":8080")
	require.NoError(t, err)
	defer lis.Close()

	go srv.Serve(lis)

	// Example with overridden client options
	clientOptions := []option.ClientOption{
		option.WithoutAuthentication(),
		option.WithTelemetryDisabled(),
	}

	sde, err := newStackdriverMetricsExporter(&Config{
		ProjectID:   "idk",
		Endpoint:    "127.0.0.1:8080",
		UserAgent:   "MyAgent {{version}}",
		UseInsecure: true,
		GetClientOptions: func() []option.ClientOption {
			return clientOptions
		},
	}, "v0.0.1")
	require.NoError(t, err)
	defer func() { require.NoError(t, sde.Shutdown(context.Background())) }()

	md := consumerdata.MetricsData{
		Resource: &resourcepb.Resource{
			Type: "test",
			Labels: map[string]string{
				"attr": "attr_value",
			},
		},
		Metrics: []*metricspb.Metric{
			metricstestutil.Gauge(
				"test_gauge",
				[]string{"k0", "k1"},
				metricstestutil.Timeseries(
					time.Now(),
					[]string{"v0", "v1"},
					metricstestutil.Double(time.Now(), 123))),
		},
	}

	assert.NoError(t, sde.ConsumeMetrics(context.Background(), internaldata.OCToMetrics(md)), err)

	drm := <-descriptorReqCh
	assert.Regexp(t, "MyAgent v0\\.0\\.1", drm.metadata["user-agent"])
	dr := drm.req.(*cloudmonitoringpb.CreateMetricDescriptorRequest)
	assert.Equal(t, "projects/idk/metricDescriptors/custom.googleapis.com/opencensus/test_gauge", dr.MetricDescriptor.Name)

	trm := <-timeSeriesReqCh
	assert.Regexp(t, "MyAgent v0\\.0\\.1", trm.metadata["user-agent"])
	tr := trm.req.(*cloudmonitoringpb.CreateTimeSeriesRequest)
	require.Len(t, tr.TimeSeries, 1)
	assert.Equal(t, map[string]string{"k0": "v0", "k1": "v1"}, tr.TimeSeries[0].Metric.Labels)
	require.Len(t, tr.TimeSeries[0].Points, 1)
	assert.Equal(t, float64(123), tr.TimeSeries[0].Points[0].Value.GetDoubleValue())
}
