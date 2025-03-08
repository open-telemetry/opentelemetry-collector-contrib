// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlecloudmonitoringreceiver

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	monitoring "cloud.google.com/go/monitoring/apiv3/v2"
	"cloud.google.com/go/monitoring/apiv3/v2/monitoringpb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
	"go.uber.org/zap"
	"google.golang.org/api/option"
	"google.golang.org/genproto/googleapis/api/distribution"
	"google.golang.org/genproto/googleapis/api/metric"
	"google.golang.org/genproto/googleapis/api/monitoredres"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type fakeMetricServiceServer struct {
	monitoringpb.UnimplementedMetricServiceServer
	metricDescriptors []*metric.MetricDescriptor
	timeSeries        []*monitoringpb.TimeSeries
}

type metricsTestServer struct {
	lis      net.Listener
	srv      *grpc.Server
	Endpoint string
}

func newFakeMetricTestServer() (*metricsTestServer, error) {
	srv := grpc.NewServer()
	lis, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return nil, err
	}
	testServer := &metricsTestServer{
		Endpoint: lis.Addr().String(),
		lis:      lis,
		srv:      srv,
	}

	monitoringpb.RegisterMetricServiceServer(
		srv,
		&fakeMetricServiceServer{
			metricDescriptors: mockMetricDescriptors(),
			timeSeries:        mockTimeSeriesData(),
		},
	)

	return testServer, nil
}

func mockMetricDescriptors() []*metric.MetricDescriptor {
	return []*metric.MetricDescriptor{
		{
			Name: "projects/fake-project/metricDescriptors/metric.type",
			Type: "custom.googleapis.com/fake_metric",
		},
	}
}

func mockTimeSeriesData() []*monitoringpb.TimeSeries {
	return []*monitoringpb.TimeSeries{
		{
			Metric: &metric.Metric{
				Type: "custom.googleapis.com/fake_metric",
			},
			Resource: &monitoredres.MonitoredResource{
				Type:   "global",
				Labels: map[string]string{"project_id": "fake-project"},
			},
			Points: []*monitoringpb.Point{
				{
					Interval: &monitoringpb.TimeInterval{
						StartTime: timestamppb.Now(),
						EndTime:   timestamppb.Now(),
					},
					Value: &monitoringpb.TypedValue{
						Value: &monitoringpb.TypedValue_Int64Value{Int64Value: 42},
					},
				},
			},
		},
	}
}

// Test cases for `initializeClient`
func TestInitializeClient_Success(t *testing.T) {
	ctx := context.Background()

	// Set up fake credentials for testing
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "./testdata/serviceAccount.json")
	defer os.Unsetenv("GOOGLE_APPLICATION_CREDENTIALS")

	logger := zap.NewNop()
	cfg := &Config{
		ProjectID:   "fake-project",
		MetricsList: []MetricConfig{{MetricName: "custom.googleapis.com/fake_metric"}},
		ControllerConfig: scraperhelper.ControllerConfig{
			CollectionInterval: 1 * time.Minute,
		},
	}

	receiver := newGoogleCloudMonitoringReceiver(cfg, logger)
	err := receiver.initializeClient(ctx)
	assert.NoError(t, err)
}

func TestInitializeClient_Failure(t *testing.T) {
	ctx := context.Background()

	logger := zap.NewNop()
	cfg := &Config{
		ProjectID:   "fake-project",
		MetricsList: []MetricConfig{{MetricName: "custom.googleapis.com/fake_metric"}},
		ControllerConfig: scraperhelper.ControllerConfig{
			CollectionInterval: 1 * time.Minute,
		},
	}

	receiver := newGoogleCloudMonitoringReceiver(cfg, logger)
	err := receiver.initializeClient(ctx)
	assert.Error(t, err)
}

func TestStart_Success(t *testing.T) {
	ctx := context.Background()
	testServer, err := newFakeMetricTestServer()
	require.NoError(t, err)
	go func() {
		err = testServer.srv.Serve(testServer.lis)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			t.Logf("Server exited with error: %v", err)
		}
	}()
	defer testServer.srv.Stop()

	clientOpts := []option.ClientOption{
		option.WithEndpoint(testServer.Endpoint),
		option.WithoutAuthentication(),
		option.WithGRPCDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())),
	}

	logger := zap.NewNop()
	cfg := &Config{
		ProjectID:   "fake-project",
		MetricsList: []MetricConfig{{MetricName: "custom.googleapis.com/fake_metric"}},
		ControllerConfig: scraperhelper.ControllerConfig{
			CollectionInterval: 1 * time.Minute,
		},
	}

	receiver := newGoogleCloudMonitoringReceiver(cfg, logger)
	receiver.client, err = monitoring.NewMetricClient(ctx, clientOpts...)
	require.NoError(t, err)

	// Test the Start function
	err = receiver.Start(ctx, nil)
	assert.NoError(t, err)

	// Verify that the receiver is properly initialized
	assert.NotNil(t, receiver.client)
	assert.NotNil(t, receiver.metricDescriptors)
}

func TestStart_Failure_NoClient(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()
	cfg := &Config{
		ProjectID:   "fake-project",
		MetricsList: []MetricConfig{{MetricName: "custom.googleapis.com/fake_metric"}},
		ControllerConfig: scraperhelper.ControllerConfig{
			CollectionInterval: 1 * time.Minute,
		},
	}

	// Don't initialize the client to test failure case
	receiver := newGoogleCloudMonitoringReceiver(cfg, logger)
	err := receiver.Start(ctx, nil)                // Test the Start function
	assert.Error(t, err)                           // Assert error presence
	assert.Contains(t, err.Error(), "credentials") // Assert error contains a specific substring
}

func TestStart_Failure_InvalidMetricDescriptor_ProjectNotFound(t *testing.T) {
	ctx := context.Background()
	testServer, err := newFakeMetricTestServer()
	require.NoError(t, err)
	go func() {
		err = testServer.srv.Serve(testServer.lis)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			t.Logf("Server exited with error: %v", err)
		}
	}()
	defer testServer.srv.Stop()

	clientOpts := []option.ClientOption{
		option.WithEndpoint(testServer.Endpoint),
		option.WithoutAuthentication(),
		option.WithGRPCDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())),
	}

	logger := zap.NewNop()
	cfg := &Config{
		MetricsList: []MetricConfig{{MetricName: "invalid.metric.name"}}, // Invalid metric name
		ControllerConfig: scraperhelper.ControllerConfig{
			CollectionInterval: 1 * time.Minute,
		},
	}

	receiver := newGoogleCloudMonitoringReceiver(cfg, logger)
	receiver.client, err = monitoring.NewMetricClient(ctx, clientOpts...)
	require.NoError(t, err)

	// Test the Start function
	err = receiver.Start(ctx, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
	assert.Equal(t, codes.NotFound, status.Code(err))
}

func TestStart_Failure_InvalidMetricDescriptor_Unauthenticated_User(t *testing.T) {
	ctx := context.Background()
	testServer, err := newFakeMetricTestServer()
	require.NoError(t, err)
	go func() {
		err = testServer.srv.Serve(testServer.lis)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			t.Logf("Server exited with error: %v", err)
		}
	}()
	defer testServer.srv.Stop()

	// Set up fake credentials for testing
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "./../googlecloudspannerreceiver/testdata/serviceAccount.json")
	defer os.Unsetenv("GOOGLE_APPLICATION_CREDENTIALS")

	logger := zap.NewNop()
	cfg := &Config{
		ProjectID:   "fake-project",
		MetricsList: []MetricConfig{{MetricName: "custom.googleapis.com/fake_metric"}},
		ControllerConfig: scraperhelper.ControllerConfig{
			CollectionInterval: 1 * time.Minute,
		},
	}

	receiver := newGoogleCloudMonitoringReceiver(cfg, logger)
	require.NoError(t, err)

	// Test the Start function
	err = receiver.Start(ctx, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Unauthenticated")
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestScrape_Success(t *testing.T) {
	// Setup fake server
	ctx := context.Background()
	testServer, err := newFakeMetricTestServer()
	require.NoError(t, err)
	go func() {
		err = testServer.srv.Serve(testServer.lis)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			t.Logf("Server exited with error: %v", err)
		}
	}()
	defer testServer.srv.Stop()

	clientOpts := []option.ClientOption{
		option.WithEndpoint(testServer.Endpoint),
		option.WithoutAuthentication(),
		option.WithGRPCDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())),
	}

	logger := zap.NewNop()
	cfg := &Config{
		ProjectID: "fake-project",
		MetricsList: []MetricConfig{
			{MetricName: "custom.googleapis.com/test_metric"},
		},
		ControllerConfig: scraperhelper.ControllerConfig{
			CollectionInterval: 1 * time.Minute,
		},
	}

	// Create and initialize receiver
	receiver := newGoogleCloudMonitoringReceiver(cfg, logger)
	receiver.client, err = monitoring.NewMetricClient(ctx, clientOpts...)
	require.NoError(t, err)

	// Initialize metric descriptors
	receiver.metricDescriptors = map[string]*metric.MetricDescriptor{
		"custom.googleapis.com/test_metric": {
			Name:       "custom.googleapis.com/test_metric",
			MetricKind: metric.MetricDescriptor_GAUGE,
			ValueType:  metric.MetricDescriptor_DOUBLE,
			Metadata: &metric.MetricDescriptor_MetricDescriptorMetadata{
				IngestDelay: durationpb.New(5 * time.Minute),
			},
		},
	}

	// Execute Scrape
	metrics, err := receiver.Scrape(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, metrics)
}

func TestScrape_Failure_MetricDescriptorNotFound(t *testing.T) {
	// Setup fake server
	ctx := context.Background()
	testServer, err := newFakeMetricTestServer()
	require.NoError(t, err)
	go func() {
		err = testServer.srv.Serve(testServer.lis)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			t.Logf("Server exited with error: %v", err)
		}
	}()
	defer testServer.srv.Stop()

	clientOpts := []option.ClientOption{
		option.WithEndpoint(testServer.Endpoint),
		option.WithoutAuthentication(),
		option.WithGRPCDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())),
	}

	logger := zap.NewNop()
	cfg := &Config{
		ProjectID: "fake-project",
		MetricsList: []MetricConfig{
			{MetricName: "nonexistent.metric"},
		},
		ControllerConfig: scraperhelper.ControllerConfig{
			CollectionInterval: 1 * time.Minute,
		},
	}

	// Create and initialize receiver
	receiver := newGoogleCloudMonitoringReceiver(cfg, logger)
	receiver.client, err = monitoring.NewMetricClient(ctx, clientOpts...)
	require.NoError(t, err)

	// Empty metric descriptors map to simulate missing descriptor
	receiver.metricDescriptors = map[string]*metric.MetricDescriptor{}

	// Execute Scrape
	metrics, err := receiver.Scrape(ctx)
	assert.NoError(t, err) // Should not return error as it just show logs warning
	assert.NotNil(t, metrics)
	assert.Equal(t, 0, metrics.ResourceMetrics().Len())
}

func TestScrape_Failure_InvalidProjectID_WithMaxInterval(t *testing.T) {
	// Setup fake server
	ctx := context.Background()
	testServer, err := newFakeMetricTestServer()
	require.NoError(t, err)
	go func() {
		err = testServer.srv.Serve(testServer.lis)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			t.Logf("Server exited with error: %v", err)
		}
	}()
	defer testServer.srv.Stop()

	clientOpts := []option.ClientOption{
		option.WithEndpoint(testServer.Endpoint),
		option.WithoutAuthentication(),
		option.WithGRPCDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())),
	}

	logger := zap.NewNop()
	cfg := &Config{
		ProjectID: "", // Invalid project ID
		MetricsList: []MetricConfig{
			{MetricName: "custom.googleapis.com/test_metric"},
		},
		ControllerConfig: scraperhelper.ControllerConfig{
			CollectionInterval: 24 * time.Hour,
		},
	}

	// Create and initialize receiver
	receiver := newGoogleCloudMonitoringReceiver(cfg, logger)
	receiver.client, err = monitoring.NewMetricClient(ctx, clientOpts...)
	require.NoError(t, err)

	// Initialize metric descriptors
	receiver.metricDescriptors = map[string]*metric.MetricDescriptor{
		"custom.googleapis.com/test_metric": {
			Name:       "custom.googleapis.com/test_metric",
			MetricKind: metric.MetricDescriptor_GAUGE,
			ValueType:  metric.MetricDescriptor_DOUBLE,
			Metadata: &metric.MetricDescriptor_MetricDescriptorMetadata{
				IngestDelay: durationpb.New(5 * time.Minute),
			},
		},
	}

	// Execute Scrape
	metrics, err := receiver.Scrape(ctx)
	assert.Error(t, err)
	assert.NotNil(t, metrics)
	assert.Contains(t, err.Error(), "not found")
	assert.Equal(t, codes.NotFound, status.Code(err))
}

func TestConvertGCPTimeSeriesToMetrics(t *testing.T) {
	// Setup fake server
	ctx := context.Background()
	testServer, err := newFakeMetricTestServer()
	require.NoError(t, err)
	go func() {
		err = testServer.srv.Serve(testServer.lis)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			t.Logf("Server exited with error: %v", err)
		}
	}()
	defer testServer.srv.Stop()

	clientOpts := []option.ClientOption{
		option.WithEndpoint(testServer.Endpoint),
		option.WithoutAuthentication(),
		option.WithGRPCDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())),
	}

	logger := zap.NewNop()
	cfg := &Config{
		ProjectID: "fake-project",
		MetricsList: []MetricConfig{
			{MetricName: "compute.googleapis.com/instance/cpu/utilization"},
			{MetricName: "custom.googleapis.com/latency"},
			{MetricName: "compute.googleapis.com/instance/network/received_bytes_count"},
		},
		ControllerConfig: scraperhelper.ControllerConfig{
			CollectionInterval: 1 * time.Minute,
		},
	}

	// Create and initialize receiver
	receiver := newGoogleCloudMonitoringReceiver(cfg, logger)
	receiver.client, err = monitoring.NewMetricClient(ctx, clientOpts...)
	require.NoError(t, err)

	// Initialize metric descriptors
	receiver.metricDescriptors = map[string]*metric.MetricDescriptor{
		"compute.googleapis.com/instance/cpu/utilization": {
			Type:        "compute.googleapis.com/instance/cpu/utilization",
			MetricKind:  metric.MetricDescriptor_GAUGE,
			ValueType:   metric.MetricDescriptor_DOUBLE,
			Unit:        metric.MetricDescriptor_MetricKind_name[0],
			Name:        metric.MetricDescriptor_MetricKind_name[1],
			Description: "CPU utilize per instance",
		},
		"custom.googleapis.com/latency": {
			Type:        "custom.googleapis.com/latency",
			MetricKind:  metric.MetricDescriptor_DELTA,
			ValueType:   metric.MetricDescriptor_DISTRIBUTION,
			Unit:        "ms",
			Name:        metric.MetricDescriptor_MetricKind_name[2],
			Description: "Distribution of latency values",
		},
		"compute.googleapis.com/instance/network/received_bytes_count": {
			Type:        "compute.googleapis.com/instance/network/received_bytes_count",
			MetricKind:  metric.MetricDescriptor_CUMULATIVE,
			ValueType:   metric.MetricDescriptor_INT64,
			Unit:        "By",
			Name:        metric.MetricDescriptor_MetricKind_name[3],
			Description: "Number of bytes received over the network",
		},
	}

	tests := []struct {
		name        string
		metricName  string
		timeSeries  *monitoringpb.TimeSeries
		expectError bool
		verifyFunc  func(t *testing.T, timeSeries *monitoringpb.TimeSeries)
	}{
		{
			name:       "success_gauge_metric",
			metricName: "compute.googleapis.com/instance/cpu/utilization",
			timeSeries: &monitoringpb.TimeSeries{
				Metric: &metric.Metric{
					Type: "compute.googleapis.com/instance/cpu/utilization",
					Labels: map[string]string{
						"instance_name": "test-instance",
					},
				},
				Resource: &monitoredres.MonitoredResource{
					Type: "gce_instance",
					Labels: map[string]string{
						"instance_id": "instance-1",
						"zone":        "us-central1-a",
					},
				},
				Metadata: &monitoredres.MonitoredResourceMetadata{
					UserLabels: map[string]string{
						"user_label1": "value1",
						"user_label2": "value2",
					},
					SystemLabels: &structpb.Struct{
						Fields: map[string]*structpb.Value{
							"system_label1": {
								Kind: &structpb.Value_StringValue{
									StringValue: "sysvalue1",
								},
							},
							"system_label2": {
								Kind: &structpb.Value_NumberValue{
									NumberValue: 123.45,
								},
							},
						},
					},
				},
				Points: []*monitoringpb.Point{
					{
						Value: &monitoringpb.TypedValue{
							Value: &monitoringpb.TypedValue_DoubleValue{
								DoubleValue: 0.75,
							},
						},
						Interval: &monitoringpb.TimeInterval{
							EndTime: &timestamppb.Timestamp{
								Seconds: time.Now().Unix(),
							},
						},
					},
				},
				MetricKind: metric.MetricDescriptor_GAUGE,
			},
			expectError: false,
			verifyFunc: func(t *testing.T, timeSeries *monitoringpb.TimeSeries) {
				// Verify TimeSeries Resource
				assert.Equal(t, "gce_instance", timeSeries.Resource.Type)
				assert.Equal(t, "instance-1", timeSeries.Resource.Labels["instance_id"])
				assert.Equal(t, "us-central1-a", timeSeries.Resource.Labels["zone"])

				// Verify Metadata User Labels
				assert.Equal(t, "value1", timeSeries.Metadata.UserLabels["user_label1"])
				assert.Equal(t, "value2", timeSeries.Metadata.UserLabels["user_label2"])

				// Verify Metadata System Labels
				sysLabels := timeSeries.Metadata.SystemLabels.Fields
				assert.Equal(t, "sysvalue1", sysLabels["system_label1"].GetStringValue())
				assert.Equal(t, 123.45, sysLabels["system_label2"].GetNumberValue())

				// Verify TimeSeries Metric
				assert.Equal(t, "compute.googleapis.com/instance/cpu/utilization", timeSeries.Metric.Type)
				assert.Equal(t, "test-instance", timeSeries.Metric.Labels["instance_name"])

				// Verify TimeSeries Points
				assert.Len(t, timeSeries.Points, 1)
				assert.Equal(t, 0.75, timeSeries.Points[0].Value.GetDoubleValue())

				// Verify MetricKind
				assert.Equal(t, metric.MetricDescriptor_GAUGE, timeSeries.MetricKind)
			},
		},
		{
			name:       "success_delta_distribution_metric",
			metricName: "custom.googleapis.com/latency",
			timeSeries: &monitoringpb.TimeSeries{
				Metric: &metric.Metric{
					Type: "custom.googleapis.com/latency",
					Labels: map[string]string{
						"service": "api",
					},
				},
				Resource: &monitoredres.MonitoredResource{
					Type: "global",
					Labels: map[string]string{
						"project_id": "test-project",
					},
				},
				Points: []*monitoringpb.Point{
					{
						Value: &monitoringpb.TypedValue{
							Value: &monitoringpb.TypedValue_DistributionValue{
								DistributionValue: &distribution.Distribution{
									Count:                 100,
									Mean:                  50.0,
									SumOfSquaredDeviation: 1000.0,
									BucketCounts:          []int64{10, 20, 30, 40},
									BucketOptions: &distribution.Distribution_BucketOptions{
										Options: &distribution.Distribution_BucketOptions_ExplicitBuckets{
											ExplicitBuckets: &distribution.Distribution_BucketOptions_Explicit{
												Bounds: []float64{0, 25, 50, 75, 100},
											},
										},
									},
								},
							},
						},
						Interval: &monitoringpb.TimeInterval{
							StartTime: &timestamppb.Timestamp{
								Seconds: time.Now().Add(-5 * time.Minute).Unix(),
							},
							EndTime: &timestamppb.Timestamp{
								Seconds: time.Now().Unix(),
							},
						},
					},
				},
				MetricKind: metric.MetricDescriptor_DELTA,
			},
			expectError: false,
			verifyFunc: func(t *testing.T, timeSeries *monitoringpb.TimeSeries) {
				// Verify TimeSeries Resource
				assert.Equal(t, "global", timeSeries.Resource.Type)
				assert.Equal(t, map[string]string{
					"project_id": "test-project",
				}, timeSeries.Resource.Labels)

				// Verify TimeSeries Metric
				assert.Equal(t, "custom.googleapis.com/latency", timeSeries.Metric.Type)
				assert.Equal(t, map[string]string{
					"service": "api",
				}, timeSeries.Metric.Labels)

				// Verify Points
				assert.Len(t, timeSeries.Points, 1)
				point := timeSeries.Points[0]
				distribution := point.Value.GetDistributionValue()

				// Verify Distribution Values
				assert.Equal(t, int64(100), distribution.Count)
				assert.Equal(t, 50.0, distribution.Mean)
				assert.Equal(t, 5000.0, distribution.Mean*float64(distribution.Count))

				// Verify Bucket Options
				bucketOptions := distribution.BucketOptions.GetExplicitBuckets()
				assert.Equal(t, []float64{0, 25, 50, 75, 100}, bucketOptions.Bounds)

				// Verify Bucket Counts
				assert.Equal(t, []int64{10, 20, 30, 40}, distribution.BucketCounts)

				// Verify MetricKind
				assert.Equal(t, metric.MetricDescriptor_DELTA, timeSeries.MetricKind)

				// Verify Time Interval
				interval := point.Interval
				assert.NotNil(t, interval.StartTime)
				assert.NotNil(t, interval.EndTime)
				assert.True(t, interval.StartTime.AsTime().Before(interval.EndTime.AsTime()))
			},
		},
		{
			name:       "success_cumulative_metric",
			metricName: "compute.googleapis.com/instance/network/received_bytes_count",
			timeSeries: &monitoringpb.TimeSeries{
				Metric: &metric.Metric{
					Type: "compute.googleapis.com/instance/network/received_bytes_count",
					Labels: map[string]string{
						"instance_name": "test-instance",
					},
				},
				Resource: &monitoredres.MonitoredResource{
					Type: "gce_instance",
					Labels: map[string]string{
						"instance_id": "instance-1",
						"zone":        "us-central1-a",
					},
				},
				Points: []*monitoringpb.Point{
					{
						Value: &monitoringpb.TypedValue{
							Value: &monitoringpb.TypedValue_Int64Value{
								Int64Value: 1000,
							},
						},
						Interval: &monitoringpb.TimeInterval{
							StartTime: &timestamppb.Timestamp{
								Seconds: time.Now().Add(-1 * time.Hour).Unix(),
							},
							EndTime: &timestamppb.Timestamp{
								Seconds: time.Now().Unix(),
							},
						},
					},
				},
				MetricKind: metric.MetricDescriptor_CUMULATIVE,
			},
			expectError: false,
			verifyFunc: func(t *testing.T, timeSeries *monitoringpb.TimeSeries) {
				// Verify TimeSeries Resource
				assert.Equal(t, "gce_instance", timeSeries.Resource.Type)
				assert.Equal(t, map[string]string{
					"instance_id": "instance-1",
					"zone":        "us-central1-a",
				}, timeSeries.Resource.Labels)

				// Verify TimeSeries Metric
				assert.Equal(t, "compute.googleapis.com/instance/network/received_bytes_count", timeSeries.Metric.Type)
				assert.Equal(t, map[string]string{
					"instance_name": "test-instance",
				}, timeSeries.Metric.Labels)

				// Verify Points
				assert.Len(t, timeSeries.Points, 1)
				point := timeSeries.Points[0]

				// Verify Value
				assert.Equal(t, int64(1000), point.Value.GetInt64Value())

				// Verify MetricKind
				assert.Equal(t, metric.MetricDescriptor_CUMULATIVE, timeSeries.MetricKind)

				// Verify Time Interval
				interval := point.Interval
				assert.NotNil(t, interval.StartTime)
				assert.NotNil(t, interval.EndTime)

				// For cumulative metrics, start time should be at least 1 hour before end time
				startTime := interval.StartTime.AsTime()
				endTime := interval.EndTime.AsTime()
				assert.True(t, startTime.Before(endTime))
				duration := endTime.Sub(startTime)
				assert.GreaterOrEqual(t, duration, time.Hour)
			},
		},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metrics := pmetric.NewMetrics()
			metricDesc := receiver.metricDescriptors[tt.metricName]
			require.NotNil(t, metricDesc, "Metric descriptor not found for %s", tt.metricName)

			// Call the conversion function
			receiver.convertGCPTimeSeriesToMetrics(metrics, metricDesc, tests[i].timeSeries)

			if tt.verifyFunc != nil {
				tt.verifyFunc(t, tt.timeSeries)
			}
		})
	}
}

func TestScrape_WithDefaultCollectionInterval(t *testing.T) {
	// Setup fake server
	ctx := context.Background()
	testServer, err := newFakeMetricTestServer()
	require.NoError(t, err)
	go func() {
		err = testServer.srv.Serve(testServer.lis)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			t.Logf("Server exited with error: %v", err)
		}
	}()
	defer testServer.srv.Stop()

	clientOpts := []option.ClientOption{
		option.WithEndpoint(testServer.Endpoint),
		option.WithoutAuthentication(),
		option.WithGRPCDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())),
	}

	logger := zap.NewNop()
	cfg := &Config{
		ProjectID: "fake-project",
		MetricsList: []MetricConfig{
			{MetricName: "custom.googleapis.com/test_metric"},
		},
	}

	// Create and initialize receiver
	receiver := newGoogleCloudMonitoringReceiver(cfg, logger)
	receiver.client, err = monitoring.NewMetricClient(ctx, clientOpts...)
	require.NoError(t, err)

	// Initialize metric descriptors
	receiver.metricDescriptors = map[string]*metric.MetricDescriptor{
		"custom.googleapis.com/test_metric": {
			Name:       "custom.googleapis.com/test_metric",
			MetricKind: metric.MetricDescriptor_GAUGE,
			ValueType:  metric.MetricDescriptor_DOUBLE,
			Metadata: &metric.MetricDescriptor_MetricDescriptorMetadata{
				IngestDelay: durationpb.New(5 * time.Minute),
			},
		},
	}

	// Execute Scrape
	metrics, err := receiver.Scrape(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, metrics)
}

func TestGoogleCloudMonitoringReceiver(t *testing.T) {
	ctx := context.Background()
	testServer, err := newFakeMetricTestServer()
	require.NoError(t, err)
	go func() {
		err = testServer.srv.Serve(testServer.lis)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			t.Logf("Server exited with error: %v", err)
		}
	}()
	defer testServer.srv.Stop()

	clientOpts := []option.ClientOption{
		option.WithEndpoint(testServer.Endpoint),
		option.WithoutAuthentication(),
		option.WithGRPCDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())),
	}

	logger := zap.NewNop()
	cfg := &Config{
		ProjectID:   "fake-project",
		MetricsList: []MetricConfig{{MetricName: "custom.googleapis.com/fake_metric"}},
		ControllerConfig: scraperhelper.ControllerConfig{
			CollectionInterval: 1 * time.Minute,
		},
	}

	receiver := newGoogleCloudMonitoringReceiver(cfg, logger)
	receiver.client, err = monitoring.NewMetricClient(ctx, clientOpts...)
	require.NoError(t, err)

	err = receiver.Start(ctx, nil)
	require.NoError(t, err)

	metrics, err := receiver.Scrape(ctx)
	require.NoError(t, err)
	assert.NotNil(t, metrics)

	// Validate metric descriptors
	assert.Contains(t, receiver.metricDescriptors, "custom.googleapis.com/fake_metric")

	err = receiver.Shutdown(ctx)
	require.NoError(t, err)
}

func (s *fakeMetricServiceServer) ListMetricDescriptors(
	ctx context.Context,
	req *monitoringpb.ListMetricDescriptorsRequest,
) (*monitoringpb.ListMetricDescriptorsResponse, error) {
	if req.Name != "projects/fake-project" {
		return nil, status.Error(codes.NotFound, "project not found")
	}

	return &monitoringpb.ListMetricDescriptorsResponse{
		MetricDescriptors: []*metric.MetricDescriptor{
			{
				Name: "projects/fake-project/metricDescriptors/custom.googleapis.com/fake_metric",
				Type: "custom.googleapis.com/fake_metric",
			},
		},
	}, nil
}

func (s *fakeMetricServiceServer) ListTimeSeries(
	ctx context.Context,
	req *monitoringpb.ListTimeSeriesRequest,
) (*monitoringpb.ListTimeSeriesResponse, error) {
	if req.Name != "projects/fake-project" {
		return nil, status.Error(codes.NotFound, "project not found")
	}

	// Create fake time series data
	timeSeries := []*monitoringpb.TimeSeries{
		{
			Metric: &metric.Metric{
				Type:   "custom.googleapis.com/fake_metric",
				Labels: map[string]string{"key": "value"},
			},
			Resource: &monitoredres.MonitoredResource{
				Type:   "global",
				Labels: map[string]string{"project_id": "fake-project"},
			},
			Points: []*monitoringpb.Point{
				{
					Interval: &monitoringpb.TimeInterval{
						StartTime: timestamppb.Now(),
						EndTime:   timestamppb.Now(),
					},
					Value: &monitoringpb.TypedValue{
						Value: &monitoringpb.TypedValue_Int64Value{Int64Value: 123},
					},
				},
			},
		},
	}

	return &monitoringpb.ListTimeSeriesResponse{
		TimeSeries: timeSeries,
	}, nil
}
