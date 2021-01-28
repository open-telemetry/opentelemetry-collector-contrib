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
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumerdata"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/testutil/metricstestutil"
	"go.opentelemetry.io/collector/translator/internaldata"
	"go.uber.org/zap"
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
	type testCase struct {
		name        string
		cfg         *Config
		expectedErr string
	}

	testCases := []testCase{
		{
			name: "Standard",
			cfg: &Config{
				ProjectID:   "idk",
				Endpoint:    "127.0.0.1:8080",
				UseInsecure: true,
			},
		},
		{
			name: "Standard_WithBundling",
			cfg: &Config{
				ProjectID:   "idk",
				Endpoint:    "127.0.0.1:8080",
				UseInsecure: true,
				TraceConfig: TraceConfig{
					BundleDelayThreshold: time.Nanosecond,
					BundleCountThreshold: 1,
					BundleByteThreshold:  1,
					BundleByteLimit:      1e9,
					BufferMaxBytes:       1e9,
				},
			},
		},
		{
			name: "Err_InvalidBundleDelayThreshold",
			cfg: &Config{
				ProjectID:   "idk",
				Endpoint:    "127.0.0.1:8080",
				UseInsecure: true,
				TraceConfig: TraceConfig{BundleDelayThreshold: -1},
			},
			expectedErr: "invalid value for: BundleDelayThreshold",
		},
		{
			name: "Err_InvalidBundleCountThreshold",
			cfg: &Config{
				ProjectID:   "idk",
				Endpoint:    "127.0.0.1:8080",
				UseInsecure: true,
				TraceConfig: TraceConfig{BundleCountThreshold: -1},
			},
			expectedErr: "invalid value for: BundleCountThreshold",
		},
		{
			name: "Err_InvalidBundleByteThreshold",
			cfg: &Config{
				ProjectID:   "idk",
				Endpoint:    "127.0.0.1:8080",
				UseInsecure: true,
				TraceConfig: TraceConfig{BundleByteThreshold: -1},
			},
			expectedErr: "invalid value for: BundleByteThreshold",
		},
		{
			name: "Err_InvalidBundleByteLimit",
			cfg: &Config{
				ProjectID:   "idk",
				Endpoint:    "127.0.0.1:8080",
				UseInsecure: true,
				TraceConfig: TraceConfig{BundleByteLimit: -1},
			},
			expectedErr: "invalid value for: BundleByteLimit",
		},
		{
			name: "Err_InvalidBufferMaxBytes",
			cfg: &Config{
				ProjectID:   "idk",
				Endpoint:    "127.0.0.1:8080",
				UseInsecure: true,
				TraceConfig: TraceConfig{BufferMaxBytes: -1},
			},
			expectedErr: "invalid value for: BufferMaxBytes",
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			srv := grpc.NewServer()
			reqCh := make(chan *cloudtracepb.BatchWriteSpansRequest)
			cloudtracepb.RegisterTraceServiceServer(srv, &testServer{reqCh: reqCh})

			lis, err := net.Listen("tcp", "localhost:8080")
			require.NoError(t, err)
			defer lis.Close()

			go srv.Serve(lis)

			createParams := component.ExporterCreateParams{Logger: zap.NewNop(), ApplicationStartInfo: component.ApplicationStartInfo{Version: "v0.0.1"}}
			sde, err := newStackdriverTraceExporter(test.cfg, createParams)
			if test.expectedErr != "" {
				assert.EqualError(t, err, test.expectedErr)
				return
			}
			require.NoError(t, err)
			defer func() { require.NoError(t, sde.Shutdown(context.Background())) }()

			testTime := time.Now()
			spanName := "foobar"

			resource := pdata.NewResource()
			traces := pdata.NewTraces()
			traces.ResourceSpans().Resize(1)
			rspans := traces.ResourceSpans().At(0)
			resource.CopyTo(rspans.Resource())
			rspans.InstrumentationLibrarySpans().Resize(1)
			ispans := rspans.InstrumentationLibrarySpans().At(0)
			ispans.Spans().Resize(1)
			span := ispans.Spans().At(0)
			span.SetName(spanName)
			span.SetStartTime(pdata.TimestampUnixNano(testTime.UnixNano()))
			err = sde.ConsumeTraces(context.Background(), traces)
			assert.NoError(t, err)

			r := <-reqCh
			assert.Len(t, r.Spans, 1)
			assert.Equal(t, fmt.Sprintf("Span.internal-%s", spanName), r.Spans[0].GetDisplayName().Value)
			assert.Equal(t, timestamppb.New(testTime), r.Spans[0].StartTime)
		})
	}
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

	lis, err := net.Listen("tcp", "localhost:8080")
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
	},
		component.ExporterCreateParams{
			Logger: zap.NewNop(),
			ApplicationStartInfo: component.ApplicationStartInfo{
				Version: "v0.0.1",
			},
		},
	)
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
				"test_gauge1",
				[]string{"k0"},
				metricstestutil.Timeseries(
					time.Now(),
					[]string{"v0"},
					metricstestutil.Double(time.Now(), 1))),
			metricstestutil.Gauge(
				"test_gauge2",
				[]string{"k0", "k1"},
				metricstestutil.Timeseries(
					time.Now(),
					[]string{"v0", "v1"},
					metricstestutil.Double(time.Now(), 12))),
			metricstestutil.Gauge(
				"test_gauge3",
				[]string{"k0", "k1", "k2"},
				metricstestutil.Timeseries(
					time.Now(),
					[]string{"v0", "v1", "v2"},
					metricstestutil.Double(time.Now(), 123))),
		},
	}

	assert.NoError(t, sde.ConsumeMetrics(context.Background(), internaldata.OCToMetrics(md)), err)

	expectedNames := map[string]struct{}{
		"projects/idk/metricDescriptors/custom.googleapis.com/opencensus/test_gauge1": {},
		"projects/idk/metricDescriptors/custom.googleapis.com/opencensus/test_gauge2": {},
		"projects/idk/metricDescriptors/custom.googleapis.com/opencensus/test_gauge3": {},
	}
	for i := 0; i < 3; i++ {
		drm := <-descriptorReqCh
		assert.Regexp(t, "MyAgent v0\\.0\\.1", drm.metadata["user-agent"])
		dr := drm.req.(*cloudmonitoringpb.CreateMetricDescriptorRequest)
		assert.Contains(t, expectedNames, dr.MetricDescriptor.Name)
		delete(expectedNames, dr.MetricDescriptor.Name)
	}

	trm := <-timeSeriesReqCh
	assert.Regexp(t, "MyAgent v0\\.0\\.1", trm.metadata["user-agent"])
	tr := trm.req.(*cloudmonitoringpb.CreateTimeSeriesRequest)
	require.Len(t, tr.TimeSeries, 3)

	expectedTimeSeries := map[string]struct {
		value  float64
		labels map[string]string
	}{
		"custom.googleapis.com/opencensus/test_gauge1": {
			value:  float64(1),
			labels: map[string]string{"k0": "v0"},
		},
		"custom.googleapis.com/opencensus/test_gauge2": {
			value:  float64(12),
			labels: map[string]string{"k0": "v0", "k1": "v1"},
		},
		"custom.googleapis.com/opencensus/test_gauge3": {
			value:  float64(123),
			labels: map[string]string{"k0": "v0", "k1": "v1", "k2": "v2"},
		},
	}
	for i := 0; i < 3; i++ {
		require.Contains(t, expectedTimeSeries, tr.TimeSeries[i].Metric.Type)
		ts := expectedTimeSeries[tr.TimeSeries[i].Metric.Type]
		assert.Equal(t, ts.labels, tr.TimeSeries[i].Metric.Labels)
		require.Len(t, tr.TimeSeries[i].Points, 1)
		assert.Equal(t, ts.value, tr.TimeSeries[i].Points[0].Value.GetDoubleValue())
	}
}
