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

package googlecloudexporter

import (
	"context"
	"net"
	"testing"
	"time"

	agentmetricspb "github.com/census-instrumentation/opencensus-proto/gen-go/agent/metrics/v1"
	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"google.golang.org/api/option"
	cloudmetricpb "google.golang.org/genproto/googleapis/api/metric"
	cloudmonitoringpb "google.golang.org/genproto/googleapis/monitoring/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/metricstestutil"
	internaldata "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/opencensus"
)

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

func TestGoogleCloudMetricExport(t *testing.T) {
	srv := grpc.NewServer()

	descriptorReqCh := make(chan *requestWithMetadata)
	timeSeriesReqCh := make(chan *requestWithMetadata)

	mockServer := &mockMetricServer{descriptorReqCh: descriptorReqCh, timeSeriesReqCh: timeSeriesReqCh}
	cloudmonitoringpb.RegisterMetricServiceServer(srv, mockServer)

	lis, err := net.Listen("tcp", "localhost:8080")
	require.NoError(t, err)
	defer lis.Close()
	defer srv.Stop()

	go func() {
		require.NoError(t, srv.Serve(lis))
	}()

	// Example with overridden client options
	clientOptions := []option.ClientOption{
		option.WithoutAuthentication(),
		option.WithTelemetryDisabled(),
	}

	creationParams := componenttest.NewNopExporterCreateSettings()
	creationParams.BuildInfo = component.BuildInfo{
		Version: "v0.0.1",
	}

	sde, err := newLegacyGoogleCloudMetricsExporter(&LegacyConfig{
		ExporterSettings: config.NewExporterSettings(config.NewComponentID(typeStr)),
		ProjectID:        "idk",
		Endpoint:         "127.0.0.1:8080",
		UserAgent:        "MyAgent {{version}}",
		UseInsecure:      true,
		GetClientOptions: func() []option.ClientOption {
			return clientOptions
		},
	}, creationParams)
	require.NoError(t, err)
	defer func() { require.NoError(t, sde.Shutdown(context.Background())) }()

	md := agentmetricspb.ExportMetricsServiceRequest{
		Resource: &resourcepb.Resource{
			Type: "host",
			Labels: map[string]string{
				"cloud.availability_zone": "us-central1",
				"host.name":               "foo",
				"k8s.cluster.name":        "test",
				"contrib.opencensus.io/exporter/stackdriver/project_id": "1234567",
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
			metricstestutil.Gauge(
				"test_gauge4",
				[]string{"k0", "k1", "k2", "k3"},
				metricstestutil.Timeseries(
					time.Now(),
					[]string{"v0", "v1", "v2", "v3"},
					metricstestutil.Double(time.Now(), 1234))),
			metricstestutil.Gauge(
				"test_gauge5",
				[]string{"k4", "k5"},
				metricstestutil.Timeseries(
					time.Now(),
					[]string{"v4", "v5"},
					metricstestutil.Double(time.Now(), 34))),
		},
	}
	md.Metrics[2].Resource = &resourcepb.Resource{
		Type: "host",
		Labels: map[string]string{
			"cloud.availability_zone": "us-central1",
			"host.name":               "bar",
			"k8s.cluster.name":        "test",
			"contrib.opencensus.io/exporter/stackdriver/project_id": "1234567",
		},
	}
	md.Metrics[3].Resource = &resourcepb.Resource{
		Type: "host",
		Labels: map[string]string{
			"contrib.opencensus.io/exporter/stackdriver/project_id": "1234567",
		},
	}
	md.Metrics[4].Resource = &resourcepb.Resource{
		Type: "test",
	}

	assert.NoError(t, sde.ConsumeMetrics(context.Background(), internaldata.OCToMetrics(md.Node, md.Resource, md.Metrics)), err)

	expectedNames := map[string]struct{}{
		"projects/idk/metricDescriptors/custom.googleapis.com/opencensus/test_gauge1": {},
		"projects/idk/metricDescriptors/custom.googleapis.com/opencensus/test_gauge2": {},
		"projects/idk/metricDescriptors/custom.googleapis.com/opencensus/test_gauge3": {},
		"projects/idk/metricDescriptors/custom.googleapis.com/opencensus/test_gauge4": {},
		"projects/idk/metricDescriptors/custom.googleapis.com/opencensus/test_gauge5": {},
	}
	for i := 0; i < 5; i++ {
		drm := <-descriptorReqCh
		assert.Regexp(t, "MyAgent v0\\.0\\.1", drm.metadata["user-agent"])
		dr := drm.req.(*cloudmonitoringpb.CreateMetricDescriptorRequest)
		assert.Contains(t, expectedNames, dr.MetricDescriptor.Name)
		delete(expectedNames, dr.MetricDescriptor.Name)
	}

	trm := <-timeSeriesReqCh
	assert.Regexp(t, "MyAgent v0\\.0\\.1", trm.metadata["user-agent"])
	tr := trm.req.(*cloudmonitoringpb.CreateTimeSeriesRequest)
	require.Len(t, tr.TimeSeries, 5)

	resourceFoo := map[string]string{
		"node_name":    "foo",
		"cluster_name": "test",
		"location":     "us-central1",
		"project_id":   "1234567",
	}

	resourceBar := map[string]string{
		"node_name":    "bar",
		"cluster_name": "test",
		"location":     "us-central1",
		"project_id":   "1234567",
	}

	resourceProjectID := map[string]string{
		"project_id": "1234567",
	}

	expectedTimeSeries := map[string]struct {
		value          float64
		labels         map[string]string
		resourceLabels map[string]string
	}{
		"custom.googleapis.com/opencensus/test_gauge1": {
			value:          float64(1),
			labels:         map[string]string{"k0": "v0"},
			resourceLabels: resourceFoo,
		},
		"custom.googleapis.com/opencensus/test_gauge2": {
			value:          float64(12),
			labels:         map[string]string{"k0": "v0", "k1": "v1"},
			resourceLabels: resourceFoo,
		},
		"custom.googleapis.com/opencensus/test_gauge3": {
			value:          float64(123),
			labels:         map[string]string{"k0": "v0", "k1": "v1", "k2": "v2"},
			resourceLabels: resourceBar,
		},
		"custom.googleapis.com/opencensus/test_gauge4": {
			value:          float64(1234),
			labels:         map[string]string{"k0": "v0", "k1": "v1", "k2": "v2", "k3": "v3"},
			resourceLabels: resourceProjectID,
		},
		"custom.googleapis.com/opencensus/test_gauge5": {
			value:          float64(34),
			labels:         map[string]string{"k4": "v4", "k5": "v5"},
			resourceLabels: nil,
		},
	}
	for i := 0; i < 5; i++ {
		require.Contains(t, expectedTimeSeries, tr.TimeSeries[i].Metric.Type)
		ts := expectedTimeSeries[tr.TimeSeries[i].Metric.Type]
		assert.Equal(t, ts.labels, tr.TimeSeries[i].Metric.Labels)
		require.Len(t, tr.TimeSeries[i].Points, 1)
		assert.Equal(t, ts.value, tr.TimeSeries[i].Points[0].Value.GetDoubleValue())
		assert.Equal(t, ts.resourceLabels, tr.TimeSeries[i].Resource.Labels)
	}
}
