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

	cloudmonitoringpb "cloud.google.com/go/monitoring/apiv3/v2/monitoringpb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"google.golang.org/api/option"
	cloudmetricpb "google.golang.org/genproto/googleapis/api/metric"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/occonventions"
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

	creationParams := exportertest.NewNopCreateSettings()
	creationParams.BuildInfo = component.BuildInfo{
		Version: "v0.0.1",
	}

	sde, err := newLegacyGoogleCloudMetricsExporter(&LegacyConfig{
		ProjectID:   "idk",
		Endpoint:    "127.0.0.1:8080",
		UserAgent:   "MyAgent {{version}}",
		UseInsecure: true,
		GetClientOptions: func() []option.ClientOption {
			return clientOptions
		},
	}, creationParams)
	require.NoError(t, err)
	defer func() { require.NoError(t, sde.Shutdown(context.Background())) }()

	md := pmetric.NewMetrics()
	rm1 := md.ResourceMetrics().AppendEmpty()
	assert.NoError(t, rm1.Resource().Attributes().FromRaw(map[string]interface{}{
		occonventions.AttributeResourceType: "host",
		"cloud.availability_zone":           "us-central1",
		"host.name":                         "foo",
		"k8s.cluster.name":                  "test",
		"contrib.opencensus.io/exporter/stackdriver/project_id": "1234567",
	}))
	initGaugeMetric0(rm1.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty())
	initGaugeMetric1(rm1.ScopeMetrics().At(0).Metrics().AppendEmpty())
	rm2 := md.ResourceMetrics().AppendEmpty()
	assert.NoError(t, rm2.Resource().Attributes().FromRaw(map[string]interface{}{
		occonventions.AttributeResourceType: "host",
		"cloud.availability_zone":           "us-central1",
		"host.name":                         "bar",
		"k8s.cluster.name":                  "test",
		"contrib.opencensus.io/exporter/stackdriver/project_id": "1234567",
	}))
	initGaugeMetric2(rm2.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty())
	rm3 := md.ResourceMetrics().AppendEmpty()
	assert.NoError(t, rm3.Resource().Attributes().FromRaw(map[string]interface{}{
		occonventions.AttributeResourceType:                     "host",
		"contrib.opencensus.io/exporter/stackdriver/project_id": "1234567",
	}))
	initGaugeMetric3(rm3.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty())
	rm4 := md.ResourceMetrics().AppendEmpty()
	assert.NoError(t, rm4.Resource().Attributes().FromRaw(map[string]interface{}{
		occonventions.AttributeResourceType: "test",
	}))
	initGaugeMetric4(rm4.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty())
	assert.NoError(t, sde.ConsumeMetrics(context.Background(), md))

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

func initGaugeMetric0(dest pmetric.Metric) {
	dest.SetName("test_gauge1")
	dp := dest.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.Attributes().PutStr("k0", "v0")
	dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	dp.SetDoubleValue(1)
}

func initGaugeMetric1(dest pmetric.Metric) {
	dest.SetName("test_gauge2")
	dp := dest.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.Attributes().PutStr("k0", "v0")
	dp.Attributes().PutStr("k1", "v1")
	dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	dp.SetDoubleValue(12)
}

func initGaugeMetric2(dest pmetric.Metric) {
	dest.SetName("test_gauge3")
	dp := dest.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.Attributes().PutStr("k0", "v0")
	dp.Attributes().PutStr("k1", "v1")
	dp.Attributes().PutStr("k2", "v2")
	dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	dp.SetDoubleValue(123)
}

func initGaugeMetric3(dest pmetric.Metric) {
	dest.SetName("test_gauge4")
	dp := dest.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.Attributes().PutStr("k0", "v0")
	dp.Attributes().PutStr("k1", "v1")
	dp.Attributes().PutStr("k2", "v2")
	dp.Attributes().PutStr("k3", "v3")
	dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	dp.SetDoubleValue(1234)
}

func initGaugeMetric4(dest pmetric.Metric) {
	dest.SetName("test_gauge5")
	dp := dest.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.Attributes().PutStr("k4", "v4")
	dp.Attributes().PutStr("k5", "v5")
	dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	dp.SetDoubleValue(34)
}
