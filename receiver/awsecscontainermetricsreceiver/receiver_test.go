// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsecscontainermetricsreceiver

import (
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/ecsutil/ecsutiltest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsecscontainermetricsreceiver/internal/awsecscontainermetrics"
)

type fakeRestClient struct{ *testing.T }

func (f fakeRestClient) GetResponse(path string) ([]byte, error) {
	if body, err := ecsutiltest.GetTestdataResponseByPath(f.T, path); body != nil || err != nil {
		return body, err
	}
	if path == awsecscontainermetrics.TaskStatsPath {
		return os.ReadFile("testdata/task_stats.json")
	}
	return nil, nil
}

func TestReceiver(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	metricsReceiver, err := newAWSECSContainermetrics(
		zap.NewNop(),
		cfg,
		consumertest.NewNop(),
		&fakeRestClient{t},
	)

	require.NoError(t, err)
	require.NotNil(t, metricsReceiver)

	r := metricsReceiver.(*awsEcsContainerMetricsReceiver)
	ctx := t.Context()

	err = r.Start(ctx, componenttest.NewNopHost())
	require.NoError(t, err)

	err = r.Shutdown(ctx)
	require.NoError(t, err)
}

func TestCollectDataFromEndpoint(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	sink := new(consumertest.MetricsSink)
	metricsReceiver, err := newAWSECSContainermetrics(
		zap.NewNop(),
		cfg,
		sink,
		&fakeRestClient{},
	)

	require.NoError(t, err)
	require.NotNil(t, metricsReceiver)

	r := metricsReceiver.(*awsEcsContainerMetricsReceiver)
	ctx := t.Context()

	err = r.collectDataFromEndpoint(ctx)
	require.NoError(t, err)

	allMetrics := sink.AllMetrics()
	require.NotEmpty(t, allMetrics)
	sumCount := 0
	for _, md := range allMetrics {
		rms := md.ResourceMetrics()
		for i := 0; i < rms.Len(); i++ {
			sms := rms.At(i).ScopeMetrics()
			for j := 0; j < sms.Len(); j++ {
				metrics := sms.At(j).Metrics()
				for k := 0; k < metrics.Len(); k++ {
					metric := metrics.At(k)
					if metric.Type() == pmetric.MetricTypeSum {
						sumCount++
						assert.True(t, metric.Sum().IsMonotonic(), "expected sum metric %s to be monotonic", metric.Name())
						assert.Equal(t, pmetric.AggregationTemporalityCumulative, metric.Sum().AggregationTemporality())
						require.Greater(t, metric.Sum().DataPoints().Len(), 0)
						dp := metric.Sum().DataPoints().At(0)
						assert.NotZero(t, dp.StartTimestamp(), "expected start timestamp on metric %s to be non-zero", metric.Name())
						assert.LessOrEqual(t, dp.StartTimestamp(), dp.Timestamp())
					}
				}
			}
		}
	}
	assert.Greater(t, sumCount, 0)
}

func TestCollectDataFromEndpointWithConsumerError(t *testing.T) {
	cfg := createDefaultConfig().(*Config)

	metricsReceiver, err := newAWSECSContainermetrics(
		zap.NewNop(),
		cfg,
		consumertest.NewErr(errors.New("Test Error for Metrics Consumer")),
		&fakeRestClient{},
	)

	require.NoError(t, err)
	require.NotNil(t, metricsReceiver)

	r := metricsReceiver.(*awsEcsContainerMetricsReceiver)
	ctx := t.Context()

	err = r.collectDataFromEndpoint(ctx)
	require.EqualError(t, err, "Test Error for Metrics Consumer")
}

type invalidFakeClient struct{}

func (invalidFakeClient) GetResponse(string) ([]byte, error) {
	return nil, errors.New("intentional error")
}

func TestCollectDataFromEndpointWithEndpointError(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	metricsReceiver, err := newAWSECSContainermetrics(
		zap.NewNop(),
		cfg,
		new(consumertest.MetricsSink),
		&invalidFakeClient{},
	)

	require.NoError(t, err)
	require.NotNil(t, metricsReceiver)

	r := metricsReceiver.(*awsEcsContainerMetricsReceiver)
	ctx := t.Context()

	err = r.collectDataFromEndpoint(ctx)
	require.Error(t, err)
}
