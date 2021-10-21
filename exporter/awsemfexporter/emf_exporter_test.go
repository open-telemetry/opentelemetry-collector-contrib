// Copyright 2020, OpenTelemetry Authors
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

package awsemfexporter

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go/aws/awserr"
	commonpb "github.com/census-instrumentation/opencensus-proto/gen-go/agent/common/v1"
	agentmetricspb "github.com/census-instrumentation/opencensus-proto/gen-go/agent/metrics/v1"
	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	internaldata "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/opencensus"
)

func init() {
	os.Setenv("AWS_ACCESS_KEY_ID", "test")
	os.Setenv("AWS_SECRET_ACCESS_KEY", "test")
}

type mockPusher struct {
	mock.Mock
}

func (p *mockPusher) addLogEntry(logEvent *logEvent) error {
	args := p.Called(nil)
	errorStr := args.String(0)
	if errorStr != "" {
		return awserr.NewRequestFailure(nil, 400, "").(error)
	}
	return nil
}

func (p *mockPusher) forceFlush() error {
	args := p.Called(nil)
	errorStr := args.String(0)
	if errorStr != "" {
		return awserr.NewRequestFailure(nil, 400, "").(error)
	}
	return nil
}

func TestConsumeMetrics(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	factory := NewFactory()
	expCfg := factory.CreateDefaultConfig().(*Config)
	expCfg.Region = "us-west-2"
	expCfg.MaxRetries = 0
	exp, err := newEmfPusher(expCfg, componenttest.NewNopExporterCreateSettings())
	assert.Nil(t, err)
	assert.NotNil(t, exp)

	mdata := agentmetricspb.ExportMetricsServiceRequest{
		Node: &commonpb.Node{
			ServiceInfo: &commonpb.ServiceInfo{Name: "test-emf"},
			LibraryInfo: &commonpb.LibraryInfo{ExporterVersion: "SomeVersion"},
		},
		Resource: &resourcepb.Resource{
			Labels: map[string]string{
				"resource": "R1",
			},
		},
		Metrics: []*metricspb.Metric{},
	}
	for i := 0; i < 2; i++ {
		m := &metricspb.Metric{
			MetricDescriptor: &metricspb.MetricDescriptor{
				Name:        "spanCounter",
				Description: "Counting all the spans",
				Unit:        "Count",
				Type:        metricspb.MetricDescriptor_CUMULATIVE_INT64,
				LabelKeys: []*metricspb.LabelKey{
					{Key: "spanName"},
					{Key: "isItAnError"},
				},
			},
			Timeseries: []*metricspb.TimeSeries{
				{
					LabelValues: []*metricspb.LabelValue{
						{Value: "testSpan"},
						{Value: "false"},
					},
					Points: []*metricspb.Point{
						{
							Timestamp: &timestamp.Timestamp{
								Seconds: int64(i),
							},
							Value: &metricspb.Point_Int64Value{
								Int64Value: 1,
							},
						},
					},
				},
			},
		}
		mdata.Metrics = append(mdata.Metrics, m)
	}
	md := internaldata.OCToMetrics(mdata.Node, mdata.Resource, mdata.Metrics)
	require.Error(t, exp.ConsumeMetrics(ctx, md))
	require.NoError(t, exp.Shutdown(ctx))
}

func TestConsumeMetricsWithOutputDestination(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	factory := NewFactory()
	expCfg := factory.CreateDefaultConfig().(*Config)
	expCfg.Region = "us-west-2"
	expCfg.MaxRetries = 0
	expCfg.OutputDestination = "stdout"
	exp, err := newEmfPusher(expCfg, componenttest.NewNopExporterCreateSettings())
	assert.Nil(t, err)
	assert.NotNil(t, exp)

	mdata := agentmetricspb.ExportMetricsServiceRequest{
		Node: &commonpb.Node{
			ServiceInfo: &commonpb.ServiceInfo{Name: "test-emf"},
			LibraryInfo: &commonpb.LibraryInfo{ExporterVersion: "SomeVersion"},
		},
		Resource: &resourcepb.Resource{
			Labels: map[string]string{
				"resource": "R1",
			},
		},
		Metrics: []*metricspb.Metric{
			{
				MetricDescriptor: &metricspb.MetricDescriptor{
					Name:        "spanCounter",
					Description: "Counting all the spans",
					Unit:        "Count",
					Type:        metricspb.MetricDescriptor_CUMULATIVE_INT64,
					LabelKeys: []*metricspb.LabelKey{
						{Key: "spanName"},
						{Key: "isItAnError"},
					},
				},
				Timeseries: []*metricspb.TimeSeries{
					{
						LabelValues: []*metricspb.LabelValue{
							{Value: "testSpan", HasValue: true},
							{Value: "false", HasValue: true},
						},
						Points: []*metricspb.Point{
							{
								Timestamp: &timestamp.Timestamp{
									Seconds: 1234567890123,
								},
								Value: &metricspb.Point_Int64Value{
									Int64Value: 1,
								},
							},
						},
					},
				},
			},
		},
	}
	md := internaldata.OCToMetrics(mdata.Node, mdata.Resource, mdata.Metrics)
	require.NoError(t, exp.ConsumeMetrics(ctx, md))
	require.NoError(t, exp.Shutdown(ctx))
}

func TestConsumeMetricsWithLogGroupStreamConfig(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	factory := NewFactory()
	expCfg := factory.CreateDefaultConfig().(*Config)
	expCfg.Region = "us-west-2"
	expCfg.MaxRetries = defaultRetryCount
	expCfg.LogGroupName = "test-logGroupName"
	expCfg.LogStreamName = "test-logStreamName"
	exp, err := newEmfPusher(expCfg, componenttest.NewNopExporterCreateSettings())
	assert.Nil(t, err)
	assert.NotNil(t, exp)

	mdata := agentmetricspb.ExportMetricsServiceRequest{
		Node: &commonpb.Node{
			ServiceInfo: &commonpb.ServiceInfo{Name: "test-emf"},
			LibraryInfo: &commonpb.LibraryInfo{ExporterVersion: "SomeVersion"},
		},
		Resource: &resourcepb.Resource{
			Labels: map[string]string{
				"resource": "R1",
			},
		},
		Metrics: []*metricspb.Metric{},
	}
	for i := 0; i < 2; i++ {
		m := &metricspb.Metric{
			MetricDescriptor: &metricspb.MetricDescriptor{
				Name:        "spanCounter",
				Description: "Counting all the spans",
				Unit:        "Count",
				Type:        metricspb.MetricDescriptor_CUMULATIVE_INT64,
				LabelKeys: []*metricspb.LabelKey{
					{Key: "spanName"},
					{Key: "isItAnError"},
				},
			},
			Timeseries: []*metricspb.TimeSeries{
				{
					LabelValues: []*metricspb.LabelValue{
						{Value: "testSpan"},
						{Value: "false"},
					},
					Points: []*metricspb.Point{
						{
							Timestamp: &timestamp.Timestamp{
								Seconds: int64(i),
							},
							Value: &metricspb.Point_Int64Value{
								Int64Value: int64(i),
							},
						},
					},
				},
			},
		}
		mdata.Metrics = append(mdata.Metrics, m)
	}

	md := internaldata.OCToMetrics(mdata.Node, mdata.Resource, mdata.Metrics)
	require.NoError(t, exp.Start(ctx, nil))
	require.Error(t, exp.ConsumeMetrics(ctx, md))
	require.NoError(t, exp.Shutdown(ctx))
	streamToPusherMap, ok := exp.(*emfExporter).groupStreamToPusherMap["test-logGroupName"]
	assert.True(t, ok)
	emfPusher, ok := streamToPusherMap["test-logStreamName"]
	assert.True(t, ok)
	assert.NotNil(t, emfPusher)
}

func TestConsumeMetricsWithLogGroupStreamValidPlaceholder(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	factory := NewFactory()
	expCfg := factory.CreateDefaultConfig().(*Config)
	expCfg.Region = "us-west-2"
	expCfg.MaxRetries = defaultRetryCount
	expCfg.LogGroupName = "/aws/ecs/containerinsights/{ClusterName}/performance"
	expCfg.LogStreamName = "{TaskId}"
	exp, err := newEmfPusher(expCfg, componenttest.NewNopExporterCreateSettings())
	assert.Nil(t, err)
	assert.NotNil(t, exp)

	mdata := agentmetricspb.ExportMetricsServiceRequest{
		Node: &commonpb.Node{
			ServiceInfo: &commonpb.ServiceInfo{Name: "test-emf"},
			LibraryInfo: &commonpb.LibraryInfo{ExporterVersion: "SomeVersion"},
		},
		Resource: &resourcepb.Resource{
			Labels: map[string]string{
				"aws.ecs.cluster.name": "test-cluster-name",
				"aws.ecs.task.id":      "test-task-id",
			},
		},
		Metrics: []*metricspb.Metric{},
	}
	for i := 0; i < 2; i++ {
		m := &metricspb.Metric{
			MetricDescriptor: &metricspb.MetricDescriptor{
				Name:        "spanCounter",
				Description: "Counting all the spans",
				Unit:        "Count",
				Type:        metricspb.MetricDescriptor_CUMULATIVE_INT64,
				LabelKeys: []*metricspb.LabelKey{
					{Key: "spanName"},
					{Key: "isItAnError"},
				},
			},
			Timeseries: []*metricspb.TimeSeries{
				{
					LabelValues: []*metricspb.LabelValue{
						{Value: "testSpan"},
						{Value: "false"},
					},
					Points: []*metricspb.Point{
						{
							Timestamp: &timestamp.Timestamp{
								Seconds: int64(i),
							},
							Value: &metricspb.Point_Int64Value{
								Int64Value: int64(i),
							},
						},
					},
				},
			},
		}
		mdata.Metrics = append(mdata.Metrics, m)
	}
	md := internaldata.OCToMetrics(mdata.Node, mdata.Resource, mdata.Metrics)
	require.NoError(t, exp.Start(ctx, nil))
	require.Error(t, exp.ConsumeMetrics(ctx, md))
	require.NoError(t, exp.Shutdown(ctx))
	streamToPusherMap, ok := exp.(*emfExporter).groupStreamToPusherMap["/aws/ecs/containerinsights/test-cluster-name/performance"]
	assert.True(t, ok)
	emfPusher, ok := streamToPusherMap["test-task-id"]
	assert.True(t, ok)
	assert.NotNil(t, emfPusher)
}

func TestConsumeMetricsWithOnlyLogStreamPlaceholder(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	factory := NewFactory()
	expCfg := factory.CreateDefaultConfig().(*Config)
	expCfg.Region = "us-west-2"
	expCfg.MaxRetries = defaultRetryCount
	expCfg.LogGroupName = "test-logGroupName"
	expCfg.LogStreamName = "{TaskId}"
	exp, err := newEmfPusher(expCfg, componenttest.NewNopExporterCreateSettings())
	assert.Nil(t, err)
	assert.NotNil(t, exp)

	mdata := agentmetricspb.ExportMetricsServiceRequest{
		Node: &commonpb.Node{
			ServiceInfo: &commonpb.ServiceInfo{Name: "test-emf"},
			LibraryInfo: &commonpb.LibraryInfo{ExporterVersion: "SomeVersion"},
		},
		Resource: &resourcepb.Resource{
			Labels: map[string]string{
				"aws.ecs.cluster.name": "test-cluster-name",
				"aws.ecs.task.id":      "test-task-id",
			},
		},
		Metrics: []*metricspb.Metric{},
	}
	for i := 0; i < 2; i++ {
		m := &metricspb.Metric{
			MetricDescriptor: &metricspb.MetricDescriptor{
				Name:        "spanCounter",
				Description: "Counting all the spans",
				Unit:        "Count",
				Type:        metricspb.MetricDescriptor_CUMULATIVE_INT64,
				LabelKeys: []*metricspb.LabelKey{
					{Key: "spanName"},
					{Key: "isItAnError"},
				},
			},
			Timeseries: []*metricspb.TimeSeries{
				{
					LabelValues: []*metricspb.LabelValue{
						{Value: "testSpan"},
						{Value: "false"},
					},
					Points: []*metricspb.Point{
						{
							Timestamp: &timestamp.Timestamp{
								Seconds: int64(i),
							},
							Value: &metricspb.Point_Int64Value{
								Int64Value: int64(i),
							},
						},
					},
				},
			},
		}
		mdata.Metrics = append(mdata.Metrics, m)
	}
	md := internaldata.OCToMetrics(mdata.Node, mdata.Resource, mdata.Metrics)
	require.NoError(t, exp.Start(ctx, nil))
	require.Error(t, exp.ConsumeMetrics(ctx, md))
	require.NoError(t, exp.Shutdown(ctx))
	streamToPusherMap, ok := exp.(*emfExporter).groupStreamToPusherMap["test-logGroupName"]
	assert.True(t, ok)
	emfPusher, ok := streamToPusherMap["test-task-id"]
	assert.True(t, ok)
	assert.NotNil(t, emfPusher)
}

func TestConsumeMetricsWithWrongPlaceholder(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	factory := NewFactory()
	expCfg := factory.CreateDefaultConfig().(*Config)
	expCfg.Region = "us-west-2"
	expCfg.MaxRetries = defaultRetryCount
	expCfg.LogGroupName = "test-logGroupName"
	expCfg.LogStreamName = "{WrongKey}"
	exp, err := newEmfPusher(expCfg, componenttest.NewNopExporterCreateSettings())
	assert.Nil(t, err)
	assert.NotNil(t, exp)

	mdata := agentmetricspb.ExportMetricsServiceRequest{
		Node: &commonpb.Node{
			ServiceInfo: &commonpb.ServiceInfo{Name: "test-emf"},
			LibraryInfo: &commonpb.LibraryInfo{ExporterVersion: "SomeVersion"},
		},
		Resource: &resourcepb.Resource{
			Labels: map[string]string{
				"aws.ecs.cluster.name": "test-cluster-name",
				"aws.ecs.task.id":      "test-task-id",
			},
		},
		Metrics: []*metricspb.Metric{},
	}
	for i := 0; i < 2; i++ {
		m := &metricspb.Metric{
			MetricDescriptor: &metricspb.MetricDescriptor{
				Name:        "spanCounter",
				Description: "Counting all the spans",
				Unit:        "Count",
				Type:        metricspb.MetricDescriptor_CUMULATIVE_INT64,
				LabelKeys: []*metricspb.LabelKey{
					{Key: "spanName"},
					{Key: "isItAnError"},
				},
			},
			Timeseries: []*metricspb.TimeSeries{
				{
					LabelValues: []*metricspb.LabelValue{
						{Value: "testSpan"},
						{Value: "false"},
					},
					Points: []*metricspb.Point{
						{
							Timestamp: &timestamp.Timestamp{
								Seconds: int64(i),
							},
							Value: &metricspb.Point_Int64Value{
								Int64Value: int64(i),
							},
						},
					},
				},
			},
		}
		mdata.Metrics = append(mdata.Metrics, m)
	}
	md := internaldata.OCToMetrics(mdata.Node, mdata.Resource, mdata.Metrics)
	require.NoError(t, exp.Start(ctx, nil))
	require.Error(t, exp.ConsumeMetrics(ctx, md))
	require.NoError(t, exp.Shutdown(ctx))
	streamToPusherMap, ok := exp.(*emfExporter).groupStreamToPusherMap["test-logGroupName"]
	assert.True(t, ok)
	emfPusher, ok := streamToPusherMap["{WrongKey}"]
	assert.True(t, ok)
	assert.NotNil(t, emfPusher)
}

func TestPushMetricsDataWithErr(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	factory := NewFactory()
	expCfg := factory.CreateDefaultConfig().(*Config)
	expCfg.Region = "us-west-2"
	expCfg.MaxRetries = 0
	expCfg.LogGroupName = "test-logGroupName"
	expCfg.LogStreamName = "test-logStreamName"
	exp, err := newEmfPusher(expCfg, componenttest.NewNopExporterCreateSettings())
	assert.Nil(t, err)
	assert.NotNil(t, exp)

	logPusher := new(mockPusher)
	logPusher.On("addLogEntry", nil).Return("some error").Once()
	logPusher.On("addLogEntry", nil).Return("").Twice()
	logPusher.On("forceFlush", nil).Return("some error").Once()
	logPusher.On("forceFlush", nil).Return("").Once()
	logPusher.On("forceFlush", nil).Return("some error").Once()
	streamToPusherMap := map[string]pusher{"test-logStreamName": logPusher}
	exp.(*emfExporter).groupStreamToPusherMap = map[string]map[string]pusher{}
	exp.(*emfExporter).groupStreamToPusherMap["test-logGroupName"] = streamToPusherMap

	mdata := agentmetricspb.ExportMetricsServiceRequest{
		Node: &commonpb.Node{
			ServiceInfo: &commonpb.ServiceInfo{Name: "test-emf"},
			LibraryInfo: &commonpb.LibraryInfo{ExporterVersion: "SomeVersion"},
		},
		Resource: &resourcepb.Resource{
			Labels: map[string]string{
				"resource": "R1",
			},
		},
		Metrics: []*metricspb.Metric{
			{
				MetricDescriptor: &metricspb.MetricDescriptor{
					Name:        "spanCounter",
					Description: "Counting all the spans",
					Unit:        "Count",
					Type:        metricspb.MetricDescriptor_CUMULATIVE_INT64,
					LabelKeys: []*metricspb.LabelKey{
						{Key: "spanName"},
						{Key: "isItAnError"},
					},
				},
				Timeseries: []*metricspb.TimeSeries{
					{
						LabelValues: []*metricspb.LabelValue{
							{Value: "testSpan"},
							{Value: "false"},
						},
						Points: []*metricspb.Point{
							{
								Timestamp: &timestamp.Timestamp{
									Seconds: 100,
								},
								Value: &metricspb.Point_Int64Value{
									Int64Value: 1,
								},
							},
						},
					},
				},
			},
		},
	}
	md := internaldata.OCToMetrics(mdata.Node, mdata.Resource, mdata.Metrics)
	assert.NotNil(t, exp.(*emfExporter).pushMetricsData(ctx, md))
	assert.NotNil(t, exp.(*emfExporter).pushMetricsData(ctx, md))
	assert.Nil(t, exp.(*emfExporter).pushMetricsData(ctx, md))
	assert.Nil(t, exp.(*emfExporter).Shutdown(ctx))
}

func TestNewExporterWithoutConfig(t *testing.T) {
	factory := NewFactory()
	expCfg := factory.CreateDefaultConfig().(*Config)
	env := stashEnv()
	defer popEnv(env)
	os.Setenv("AWS_STS_REGIONAL_ENDPOINTS", "fake")

	assert.Nil(t, expCfg.logger)
	exp, err := newEmfPusher(expCfg, componenttest.NewNopExporterCreateSettings())
	assert.NotNil(t, err)
	assert.Nil(t, exp)
	assert.NotNil(t, expCfg.logger)
}

func TestNewExporterWithMetricDeclarations(t *testing.T) {
	factory := NewFactory()
	expCfg := factory.CreateDefaultConfig().(*Config)
	expCfg.Region = "us-west-2"
	expCfg.MaxRetries = defaultRetryCount
	expCfg.LogGroupName = "test-logGroupName"
	expCfg.LogStreamName = "test-logStreamName"
	mds := []*MetricDeclaration{
		{
			MetricNameSelectors: []string{"a", "b"},
		},
		{
			MetricNameSelectors: []string{"c", "d"},
		},
		{
			MetricNameSelectors: nil,
		},
		{
			Dimensions: [][]string{
				{"foo"},
				{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"},
			},
			MetricNameSelectors: []string{"a"},
		},
	}
	expCfg.MetricDeclarations = mds

	obs, logs := observer.New(zap.WarnLevel)
	params := componenttest.NewNopExporterCreateSettings()
	params.Logger = zap.New(obs)

	exp, err := newEmfPusher(expCfg, params)
	assert.Nil(t, err)
	assert.NotNil(t, exp)

	emfExporter := exp.(*emfExporter)
	config := emfExporter.config.(*Config)
	// Invalid metric declaration should be filtered out
	assert.Equal(t, 3, len(config.MetricDeclarations))
	// Invalid dimensions (> 10 dims) should be filtered out
	assert.Equal(t, 1, len(config.MetricDeclarations[2].Dimensions))

	// Test output warning logs
	expectedLogs := []observer.LoggedEntry{
		{
			Entry:   zapcore.Entry{Level: zap.WarnLevel, Message: "Dropped metric declaration."},
			Context: []zapcore.Field{zap.Error(errors.New("invalid metric declaration: no metric name selectors defined"))},
		},
		{
			Entry:   zapcore.Entry{Level: zap.WarnLevel, Message: "Dropped dimension set: > 10 dimensions specified."},
			Context: []zapcore.Field{zap.String("dimensions", "a,b,c,d,e,f,g,h,i,j,k")},
		},
	}
	assert.Equal(t, 2, logs.Len())
	assert.Equal(t, expectedLogs, logs.AllUntimed())
}

func TestNewExporterWithoutSession(t *testing.T) {
	exp, err := newEmfPusher(nil, componenttest.NewNopExporterCreateSettings())
	assert.NotNil(t, err)
	assert.Nil(t, exp)
}

func TestWrapErrorIfBadRequest(t *testing.T) {
	awsErr := awserr.NewRequestFailure(nil, 400, "").(error)
	err := wrapErrorIfBadRequest(&awsErr)
	assert.True(t, consumererror.IsPermanent(err))
	awsErr = awserr.NewRequestFailure(nil, 500, "").(error)
	err = wrapErrorIfBadRequest(&awsErr)
	assert.False(t, consumererror.IsPermanent(err))
}

// This test verifies that if func newEmfPusher() returns an error then newEmfExporter()
// will do so.
func TestNewEmfExporterWithoutConfig(t *testing.T) {
	factory := NewFactory()
	expCfg := factory.CreateDefaultConfig().(*Config)
	env := stashEnv()
	defer popEnv(env)
	os.Setenv("AWS_STS_REGIONAL_ENDPOINTS", "fake")

	assert.Nil(t, expCfg.logger)
	exp, err := newEmfExporter(expCfg, componenttest.NewNopExporterCreateSettings())
	assert.NotNil(t, err)
	assert.Nil(t, exp)
	assert.NotNil(t, expCfg.logger)
}

func stashEnv() []string {
	env := os.Environ()
	os.Clearenv()

	return env
}

func popEnv(env []string) {
	os.Clearenv()

	for _, e := range env {
		p := strings.SplitN(e, "=", 2)
		os.Setenv(p[0], p[1])
	}
}
