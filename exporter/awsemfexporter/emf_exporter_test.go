// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsemfexporter

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/cwlogs"
)

const defaultRetryCount = 1

type mockPusher struct {
	mock.Mock
}

func (p *mockPusher) AddLogEntry(_ *cwlogs.Event) error {
	args := p.Called(nil)
	errorStr := args.String(0)
	if errorStr != "" {
		return awserr.NewRequestFailure(nil, 400, "").(error)
	}
	return nil
}

func (p *mockPusher) ForceFlush() error {
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
	exp, err := newEmfExporter(expCfg, exportertest.NewNopCreateSettings())
	assert.Nil(t, err)
	assert.NotNil(t, exp)

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
			"histograme-with-nan",
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
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			factory := NewFactory()
			expCfg := factory.CreateDefaultConfig().(*Config)
			expCfg.Region = "us-west-2"
			expCfg.MaxRetries = 0
			expCfg.OutputDestination = "stdout"
			exp, err := newEmfExporter(expCfg, exportertest.NewNopCreateSettings())
			assert.Nil(t, err)
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
			"histograme-with-inf",
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
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			factory := NewFactory()
			expCfg := factory.CreateDefaultConfig().(*Config)
			expCfg.Region = "us-west-2"
			expCfg.MaxRetries = 0
			expCfg.OutputDestination = "stdout"
			exp, err := newEmfExporter(expCfg, exportertest.NewNopCreateSettings())
			assert.Nil(t, err)
			assert.NotNil(t, exp)
			md := tc.generateFunc(tc.testName)
			require.NoError(t, exp.pushMetricsData(ctx, md))
			require.NoError(t, exp.shutdown(ctx))
		})
	}

}

func TestConsumeMetricsWithOutputDestination(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	factory := NewFactory()
	expCfg := factory.CreateDefaultConfig().(*Config)
	expCfg.Region = "us-west-2"
	expCfg.MaxRetries = 0
	expCfg.OutputDestination = "stdout"
	exp, err := newEmfExporter(expCfg, exportertest.NewNopCreateSettings())
	assert.Nil(t, err)
	assert.NotNil(t, exp)

	md := generateTestMetrics(testMetric{
		metricNames:  []string{"metric_1", "metric_2"},
		metricValues: [][]float64{{100}, {4}},
	})
	require.NoError(t, exp.pushMetricsData(ctx, md))
	require.NoError(t, exp.shutdown(ctx))
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
	exp, err := newEmfExporter(expCfg, exportertest.NewNopCreateSettings())
	assert.Nil(t, err)
	assert.NotNil(t, exp)

	md := generateTestMetrics(testMetric{
		metricNames:  []string{"metric_1", "metric_2"},
		metricValues: [][]float64{{100}, {4}},
	})
	require.Error(t, exp.pushMetricsData(ctx, md))
	require.NoError(t, exp.shutdown(ctx))
	pusherMap, ok := exp.pusherMap[cwlogs.StreamKey{
		LogGroupName:  expCfg.LogGroupName,
		LogStreamName: expCfg.LogStreamName,
	}]
	assert.True(t, ok)
	assert.NotNil(t, pusherMap)
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
	exp, err := newEmfExporter(expCfg, exportertest.NewNopCreateSettings())
	assert.Nil(t, err)
	assert.NotNil(t, exp)

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
	pusherMap, ok := exp.pusherMap[cwlogs.StreamKey{
		LogGroupName:  "/aws/ecs/containerinsights/test-cluster-name/performance",
		LogStreamName: "test-task-id",
	}]
	assert.True(t, ok)
	assert.NotNil(t, pusherMap)
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
	exp, err := newEmfExporter(expCfg, exportertest.NewNopCreateSettings())
	assert.Nil(t, err)
	assert.NotNil(t, exp)

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
	pusherMap, ok := exp.pusherMap[cwlogs.StreamKey{
		LogGroupName:  expCfg.LogGroupName,
		LogStreamName: "test-task-id",
	}]
	assert.True(t, ok)
	assert.NotNil(t, pusherMap)
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
	exp, err := newEmfExporter(expCfg, exportertest.NewNopCreateSettings())
	assert.Nil(t, err)
	assert.NotNil(t, exp)

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
	pusherMap, ok := exp.pusherMap[cwlogs.StreamKey{
		LogGroupName:  expCfg.LogGroupName,
		LogStreamName: expCfg.LogStreamName,
	}]
	assert.True(t, ok)
	assert.NotNil(t, pusherMap)
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
	exp, err := newEmfExporter(expCfg, exportertest.NewNopCreateSettings())
	assert.Nil(t, err)
	assert.NotNil(t, exp)

	logPusher := new(mockPusher)
	logPusher.On("AddLogEntry", nil).Return("some error").Once()
	logPusher.On("AddLogEntry", nil).Return("").Twice()
	logPusher.On("ForceFlush", nil).Return("some error").Once()
	logPusher.On("ForceFlush", nil).Return("").Once()
	logPusher.On("ForceFlush", nil).Return("some error").Once()
	exp.pusherMap = map[cwlogs.StreamKey]cwlogs.Pusher{}
	exp.pusherMap[cwlogs.StreamKey{
		LogGroupName:  "test-logGroupName",
		LogStreamName: "test-logStreamName",
	}] = logPusher

	md := generateTestMetrics(testMetric{
		metricNames:  []string{"metric_1", "metric_2"},
		metricValues: [][]float64{{100}, {4}},
	})
	assert.NotNil(t, exp.pushMetricsData(ctx, md))
	assert.NotNil(t, exp.pushMetricsData(ctx, md))
	assert.Nil(t, exp.pushMetricsData(ctx, md))
	assert.Nil(t, exp.shutdown(ctx))
}

func TestNewExporterWithoutConfig(t *testing.T) {
	factory := NewFactory()
	expCfg := factory.CreateDefaultConfig().(*Config)
	settings := exportertest.NewNopCreateSettings()
	t.Setenv("AWS_STS_REGIONAL_ENDPOINTS", "fake")

	exp, err := newEmfExporter(expCfg, settings)
	assert.NotNil(t, err)
	assert.Nil(t, exp)
	assert.Equal(t, settings.Logger, expCfg.logger)
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
	params := exportertest.NewNopCreateSettings()
	params.Logger = zap.New(obs)

	exp, err := newEmfExporter(expCfg, params)
	assert.Nil(t, err)
	assert.NotNil(t, exp)
	err = expCfg.Validate()
	assert.Nil(t, err)

	// Invalid metric declaration should be filtered out
	assert.Equal(t, 3, len(exp.config.MetricDeclarations))
	// Invalid dimensions (> 10 dims) should be filtered out
	assert.Equal(t, 1, len(exp.config.MetricDeclarations[2].Dimensions))

	// Test output warning logs
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
	exp, err := newEmfExporter(nil, exportertest.NewNopCreateSettings())
	assert.NotNil(t, err)
	assert.Nil(t, exp)
}

func TestWrapErrorIfBadRequest(t *testing.T) {
	awsErr := awserr.NewRequestFailure(nil, 400, "").(error)
	err := wrapErrorIfBadRequest(awsErr)
	assert.True(t, consumererror.IsPermanent(err))
	awsErr = awserr.NewRequestFailure(nil, 500, "").(error)
	err = wrapErrorIfBadRequest(awsErr)
	assert.False(t, consumererror.IsPermanent(err))
}

// This test verifies that if func newEmfExporter() returns an error then newEmfExporter()
// will do so.
func TestNewEmfExporterWithoutConfig(t *testing.T) {
	factory := NewFactory()
	expCfg := factory.CreateDefaultConfig().(*Config)
	settings := exportertest.NewNopCreateSettings()
	t.Setenv("AWS_STS_REGIONAL_ENDPOINTS", "fake")

	exp, err := newEmfExporter(expCfg, settings)
	assert.NotNil(t, err)
	assert.Nil(t, exp)
	assert.Equal(t, settings.Logger, expCfg.logger)
}
