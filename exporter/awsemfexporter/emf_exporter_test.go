// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsemfexporter

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"

	"github.com/amazon-contributing/opentelemetry-collector-contrib/extension/awsmiddleware"
	"github.com/aws/smithy-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsemfexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/cwlogs"
)

const defaultRetryCount = 1

func init() {
	os.Setenv("AWS_ACCESS_KEY_ID", "test")
	os.Setenv("AWS_SECRET_ACCESS_KEY", "test")
}

type mockPusher struct {
	mock.Mock
}

func (p *mockPusher) AddLogEntry(_ context.Context, _ *cwlogs.Event) error {
	args := p.Called(nil)
	errorStr := args.String(0)
	if errorStr != "" {
		return &smithy.GenericAPIError{
			Code:    "",
			Message: "",
			Fault:   smithy.FaultClient,
		}
	}
	return nil
}

func (p *mockPusher) ForceFlush(_ context.Context) error {
	args := p.Called(nil)
	errorStr := args.String(0)
	if errorStr != "" {
		return &smithy.GenericAPIError{
			Code:    "",
			Message: "",
			Fault:   smithy.FaultClient,
		}
	}
	return nil
}

type mockHost struct {
	component.Host
}

func (m *mockHost) GetExtensions() map[component.ID]component.Component {
	return nil
}

func TestConsumeMetrics(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	factory := NewFactory()
	expCfg := factory.CreateDefaultConfig().(*Config)
	expCfg.Region = "us-west-2"
	expCfg.MaxRetries = 0
	exp, err := newEmfExporter(ctx, expCfg, exportertest.NewNopSettings(metadata.Type))
	assert.NoError(t, err)
	assert.NotNil(t, exp)

	mockHost := &mockHost{}
	err = exp.start(ctx, mockHost)
	assert.NoError(t, err)

	md := generateTestMetrics(testMetric{
		metricNames:  []string{"metric_1", "metric_2"},
		metricValues: [][]float64{{100}, {4}},
	})
	require.Error(t, exp.pushMetricsData(ctx, md))
	require.NoError(t, exp.shutdown(ctx))
}

func TestConsumeMetricsWithNaNValues(t *testing.T) {
	tests := []struct {
		testName     string
		generateFunc func(string) pmetric.Metrics
	}{
		{
			"histogram-with-nan",
			generateTestHistogramMetricWithNaNs,
		}, {
			"gauge-with-nan",
			generateTestGaugeMetricNaN,
		}, {
			"summary-with-nan",
			generateTestSummaryMetricWithNaN,
		}, {
			"exponentialHistogram-with-nan",
			generateTestExponentialHistogramMetricWithNaNs,
		},
	}

	for _, tc := range tests {
		t.Run(tc.testName, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			factory := NewFactory()
			expCfg := factory.CreateDefaultConfig().(*Config)
			expCfg.Region = "us-west-2"
			expCfg.MaxRetries = 0
			expCfg.OutputDestination = "stdout"
			exp, err := newEmfExporter(ctx, expCfg, exportertest.NewNopSettings(metadata.Type))
			assert.NoError(t, err)
			assert.NotNil(t, exp)
			md := tc.generateFunc(tc.testName)
			require.NoError(t, exp.pushMetricsData(ctx, md))
			require.NoError(t, exp.shutdown(ctx))
		})
	}
}

func TestConsumeMetricsWithInfValues(t *testing.T) {
	tests := []struct {
		testName     string
		generateFunc func(string) pmetric.Metrics
	}{
		{
			"histogram-with-inf",
			generateTestHistogramMetricWithInfs,
		}, {
			"gauge-with-inf",
			generateTestGaugeMetricInf,
		}, {
			"summary-with-inf",
			generateTestSummaryMetricWithInf,
		}, {
			"exponentialHistogram-with-inf",
			generateTestExponentialHistogramMetricWithInfs,
		},
	}

	for _, tc := range tests {
		t.Run(tc.testName, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			factory := NewFactory()
			expCfg := factory.CreateDefaultConfig().(*Config)
			expCfg.Region = "us-west-2"
			expCfg.MaxRetries = 0
			expCfg.OutputDestination = "stdout"
			exp, err := newEmfExporter(ctx, expCfg, exportertest.NewNopSettings(metadata.Type))
			assert.NoError(t, err)
			assert.NotNil(t, exp)
			md := tc.generateFunc(tc.testName)
			require.NoError(t, exp.pushMetricsData(ctx, md))
			require.NoError(t, exp.shutdown(ctx))
		})
	}
}

func TestConsumeMetricsWithOutputDestination(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	factory := NewFactory()
	expCfg := factory.CreateDefaultConfig().(*Config)
	expCfg.Region = "us-west-2"
	expCfg.MaxRetries = 0
	expCfg.OutputDestination = "stdout"
	exp, err := newEmfExporter(ctx, expCfg, exportertest.NewNopSettings(metadata.Type))
	assert.NoError(t, err)
	assert.NotNil(t, exp)

	md := generateTestMetrics(testMetric{
		metricNames:  []string{"metric_1", "metric_2"},
		metricValues: [][]float64{{100}, {4}},
	})
	require.NoError(t, exp.pushMetricsData(ctx, md))
	require.NoError(t, exp.shutdown(ctx))
}

func TestConsumeMetricsWithLogGroupStreamConfig(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	factory := NewFactory()
	expCfg := factory.CreateDefaultConfig().(*Config)
	expCfg.Region = "us-west-2"
	expCfg.MaxRetries = defaultRetryCount
	expCfg.LogGroupName = "test-logGroupName"
	expCfg.LogStreamName = "test-logStreamName"
	exp, err := newEmfExporter(ctx, expCfg, exportertest.NewNopSettings(metadata.Type))
	assert.NoError(t, err)
	assert.NotNil(t, exp)
	mockHost := &mockHost{}
	err = exp.start(ctx, mockHost)
	assert.NoError(t, err)

	md := generateTestMetrics(testMetric{
		metricNames:  []string{"metric_1", "metric_2"},
		metricValues: [][]float64{{100}, {4}},
	})
	require.Error(t, exp.pushMetricsData(ctx, md))
	require.NoError(t, exp.shutdown(ctx))
	val, ok := exp.pusherMap.Load(cwlogs.StreamKey{
		LogGroupName:  expCfg.LogGroupName,
		LogStreamName: expCfg.LogStreamName,
	})
	assert.True(t, ok)
	assert.NotNil(t, val)
}

func TestConsumeMetricsWithLogGroupStreamValidPlaceholder(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	factory := NewFactory()
	expCfg := factory.CreateDefaultConfig().(*Config)
	expCfg.Region = "us-west-2"
	expCfg.MaxRetries = defaultRetryCount
	expCfg.LogGroupName = "/aws/ecs/containerinsights/{ClusterName}/performance"
	expCfg.LogStreamName = "{TaskId}"
	exp, err := newEmfExporter(ctx, expCfg, exportertest.NewNopSettings(metadata.Type))
	assert.NoError(t, err)
	assert.NotNil(t, exp)
	mockHost := &mockHost{}
	err = exp.start(ctx, mockHost)
	assert.NoError(t, err)

	md := generateTestMetrics(testMetric{
		metricNames:  []string{"metric_1", "metric_2"},
		metricValues: [][]float64{{100}, {4}},
		resourceAttributeMap: map[string]any{
			"aws.ecs.cluster.name": "test-cluster-name",
			"aws.ecs.task.id":      "test-task-id",
		},
	})
	require.Error(t, exp.pushMetricsData(ctx, md))
	require.NoError(t, exp.shutdown(ctx))
	val, ok := exp.pusherMap.Load(cwlogs.StreamKey{
		LogGroupName:  "/aws/ecs/containerinsights/test-cluster-name/performance",
		LogStreamName: "test-task-id",
	})
	assert.True(t, ok)
	assert.NotNil(t, val)
}

func TestConsumeMetricsWithOnlyLogStreamPlaceholder(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	factory := NewFactory()
	expCfg := factory.CreateDefaultConfig().(*Config)
	expCfg.Region = "us-west-2"
	expCfg.MaxRetries = defaultRetryCount
	expCfg.LogGroupName = "test-logGroupName"
	expCfg.LogStreamName = "{TaskId}"
	exp, err := newEmfExporter(ctx, expCfg, exportertest.NewNopSettings(metadata.Type))
	assert.NoError(t, err)
	assert.NotNil(t, exp)
	mockHost := &mockHost{}
	err = exp.start(ctx, mockHost)
	assert.NoError(t, err)

	md := generateTestMetrics(testMetric{
		metricNames:  []string{"metric_1", "metric_2"},
		metricValues: [][]float64{{100}, {4}},
		resourceAttributeMap: map[string]any{
			"aws.ecs.cluster.name": "test-cluster-name",
			"aws.ecs.task.id":      "test-task-id",
		},
	})
	require.Error(t, exp.pushMetricsData(ctx, md))
	require.NoError(t, exp.shutdown(ctx))
	val, ok := exp.pusherMap.Load(cwlogs.StreamKey{
		LogGroupName:  expCfg.LogGroupName,
		LogStreamName: "test-task-id",
	})
	assert.True(t, ok)
	assert.NotNil(t, val)
}

func TestConsumeMetricsWithWrongPlaceholder(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	factory := NewFactory()
	expCfg := factory.CreateDefaultConfig().(*Config)
	expCfg.Region = "us-west-2"
	expCfg.MaxRetries = defaultRetryCount
	expCfg.LogGroupName = "test-logGroupName"
	expCfg.LogStreamName = "{WrongKey}"

	logger, _ := zap.NewProduction()
	settings := exportertest.NewNopSettings(metadata.Type)
	settings.Logger = logger

	exp, err := newEmfExporter(ctx, expCfg, settings)
	assert.NoError(t, err)
	assert.NotNil(t, exp)

	exp.config.logger = logger

	mockHost := componenttest.NewNopHost()
	err = exp.start(ctx, mockHost)
	assert.NoError(t, err)

	md := generateTestMetrics(testMetric{
		metricNames:  []string{"metric_1", "metric_2"},
		metricValues: [][]float64{{100}, {4}},
		resourceAttributeMap: map[string]any{
			"aws.ecs.cluster.name": "test-cluster-name",
			"aws.ecs.task.id":      "test-task-id",
		},
	})
	require.Error(t, exp.pushMetricsData(ctx, md))
	require.NoError(t, exp.shutdown(ctx))

	val, ok := exp.pusherMap.Load(cwlogs.StreamKey{
		LogGroupName:  expCfg.LogGroupName,
		LogStreamName: expCfg.LogStreamName,
	})
	assert.True(t, ok)
	assert.NotNil(t, val)
}

func TestPushMetricsDataWithErr(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	factory := NewFactory()
	expCfg := factory.CreateDefaultConfig().(*Config)
	expCfg.Region = "us-west-2"
	expCfg.MaxRetries = 0
	expCfg.LogGroupName = "test-logGroupName"
	expCfg.LogStreamName = "test-logStreamName"
	exp, err := newEmfExporter(ctx, expCfg, exportertest.NewNopSettings(metadata.Type))
	assert.NoError(t, err)
	assert.NotNil(t, exp)

	mockHost := &mockHost{}
	err = exp.start(ctx, mockHost)
	assert.NoError(t, err)

	logPusher := new(mockPusher)
	logPusher.On("AddLogEntry", nil).Return("some error").Once()
	logPusher.On("AddLogEntry", nil).Return("").Twice()
	logPusher.On("ForceFlush", nil).Return("some error").Once()
	logPusher.On("ForceFlush", nil).Return("").Once()
	logPusher.On("ForceFlush", nil).Return("some error").Once()
	exp.pusherMap.Store(cwlogs.StreamKey{
		LogGroupName:  "test-logGroupName",
		LogStreamName: "test-logStreamName",
	}, logPusher)

	md := generateTestMetrics(testMetric{
		metricNames:  []string{"metric_1", "metric_2"},
		metricValues: [][]float64{{100}, {4}},
	})
	assert.Error(t, exp.pushMetricsData(ctx, md))
	assert.Error(t, exp.pushMetricsData(ctx, md))
	assert.NoError(t, exp.pushMetricsData(ctx, md))
	assert.NoError(t, exp.shutdown(ctx))
}

func TestNewExporterWithoutConfig(t *testing.T) {
	factory := NewFactory()
	expCfg := factory.CreateDefaultConfig().(*Config)
	settings := exportertest.NewNopSettings(metadata.Type)
	t.Setenv("AWS_STS_REGIONAL_ENDPOINTS", "fake")

	ctx := t.Context()
	exp, err := newEmfExporter(ctx, expCfg, settings)
	assert.NoError(t, err)
	assert.NotNil(t, exp)
	assert.Equal(t, expCfg.logger, settings.Logger)

	mockHost := &mockHost{}
	err = exp.start(ctx, mockHost)
	assert.Error(t, err)
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
	params := exportertest.NewNopSettings(metadata.Type)
	params.Logger = zap.New(obs)

	ctx := t.Context()
	exp, err := newEmfExporter(ctx, expCfg, params)
	assert.NoError(t, err)
	assert.NotNil(t, exp)
	err = expCfg.Validate()
	assert.NoError(t, err)

	// Invalid metric declaration should be filtered out
	assert.Len(t, exp.config.MetricDeclarations, 3)
	// Invalid dimensions (> 10 dims) should be filtered out
	assert.Len(t, exp.config.MetricDeclarations[2].Dimensions, 1)

	expectedLogs := []observer.LoggedEntry{
		{
			Entry: zapcore.Entry{Level: zap.WarnLevel, Message: "the default value for DimensionRollupOption will be changing to NoDimensionRollup" +
				"in a future release. See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/23997 for more" +
				"information"},
			Context: []zapcore.Field{},
		},
		{
			Entry:   zapcore.Entry{Level: zap.WarnLevel, Message: "Dropped metric declaration."},
			Context: []zapcore.Field{zap.Error(errors.New("invalid metric declaration: no metric name selectors defined"))},
		},
		{
			Entry:   zapcore.Entry{Level: zap.WarnLevel, Message: "Dropped dimension set: > 10 dimensions specified."},
			Context: []zapcore.Field{zap.String("dimensions", "a,b,c,d,e,f,g,h,i,j,k")},
		},
	}
	assert.Equal(t, len(expectedLogs), logs.Len())
	assert.Equal(t, expectedLogs, logs.AllUntimed())
}

func TestNewExporterWithoutSession(t *testing.T) {
	ctx := t.Context()
	exp, err := newEmfExporter(ctx, nil, exportertest.NewNopSettings(metadata.Type))
	assert.Error(t, err)
	assert.Nil(t, exp)
}

func TestWrapErrorIfBadRequest(t *testing.T) {
	awsErr := &smithy.GenericAPIError{
		Code:    "",
		Message: "",
		Fault:   smithy.FaultClient,
	}
	err := wrapErrorIfBadRequest(awsErr)
	assert.True(t, consumererror.IsPermanent(err))
	awsErr = &smithy.GenericAPIError{
		Code:    "",
		Message: "",
		Fault:   smithy.FaultServer,
	}
	err = wrapErrorIfBadRequest(awsErr)
	assert.False(t, consumererror.IsPermanent(err))
}

// TestNewEmfExporterWithoutConfig — fork variant: factory succeeds (lazy
// construction), error surfaces in start() because of fake STS endpoint.
func TestNewEmfExporterWithoutConfig(t *testing.T) {
	factory := NewFactory()
	expCfg := factory.CreateDefaultConfig().(*Config)
	settings := exportertest.NewNopSettings(metadata.Type)
	t.Setenv("AWS_STS_REGIONAL_ENDPOINTS", "fake")

	ctx := t.Context()
	exp, err := newEmfExporter(ctx, expCfg, settings)
	assert.NoError(t, err)
	assert.NotNil(t, exp)
	assert.Equal(t, expCfg.logger, settings.Logger)

	mockHost := &mockHost{}
	err = exp.start(ctx, mockHost)
	assert.Error(t, err) // fake AWS_STS_REGIONAL_ENDPOINTS forces failure
	// svcStructuredLog is still nil after failed start
	assert.Nil(t, exp.svcStructuredLog)
}

// TestMiddleware exercises the awsmiddleware extension wiring.
func TestMiddleware(t *testing.T) {
	testType, _ := component.NewType("test")
	id := component.NewID(testType)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	factory := NewFactory()
	expCfg := factory.CreateDefaultConfig().(*Config)
	expCfg.Region = "us-west-2"
	expCfg.MaxRetries = 0
	expCfg.MiddlewareID = &id
	handler := new(awsmiddleware.MockHandler)
	handler.On("ID").Return("test")
	handler.On("Position").Return(awsmiddleware.After)
	handler.On("HandleRequest", mock.Anything, mock.Anything)
	handler.On("HandleResponse", mock.Anything, mock.Anything)
	middleware := new(awsmiddleware.MockMiddlewareExtension)
	middleware.On("Handlers").Return(
		[]awsmiddleware.RequestHandler{handler},
		[]awsmiddleware.ResponseHandler{handler},
	)
	extensions := map[component.ID]component.Component{id: middleware}
	exp, err := newEmfExporter(ctx, expCfg, exportertest.NewNopSettings(metadata.Type))
	assert.NoError(t, err)
	assert.NotNil(t, exp)
	host := new(awsmiddleware.MockExtensionsHost)
	host.On("GetExtensions").Return(extensions)
	assert.NoError(t, exp.start(ctx, host))
	md := generateTestMetrics(testMetric{
		metricNames:  []string{"metric_1", "metric_2"},
		metricValues: [][]float64{{100}, {4}},
	})
	require.Error(t, exp.pushMetricsData(ctx, md))
	require.NoError(t, exp.shutdown(ctx))
	handler.AssertCalled(t, "HandleRequest", mock.Anything, mock.Anything)
	handler.AssertCalled(t, "HandleResponse", mock.Anything, mock.Anything)
}

// TestGetPusherConcurrent verifies thread-safe lazy pusher construction.
func TestGetPusherConcurrent(t *testing.T) {
	ctx := t.Context()
	factory := NewFactory()
	expCfg := factory.CreateDefaultConfig().(*Config)
	expCfg.Region = "us-west-2"
	expCfg.MaxRetries = 0
	exp, err := newEmfExporter(ctx, expCfg, exportertest.NewNopSettings(metadata.Type))
	require.NoError(t, err)

	mockHost := &mockHost{}
	require.NoError(t, exp.start(ctx, mockHost))

	var wg sync.WaitGroup
	for i := range 100 {
		wg.Go(func() {
			key := cwlogs.StreamKey{
				LogGroupName:  "test-group",
				LogStreamName: fmt.Sprintf("stream-%d", i%5),
			}
			p, err := exp.getPusher(key)
			assert.NoError(t, err)
			assert.NotNil(t, p)
		})
	}
	wg.Wait()
}
