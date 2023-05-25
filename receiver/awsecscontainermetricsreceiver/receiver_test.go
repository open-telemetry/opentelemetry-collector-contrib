// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsecscontainermetricsreceiver

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
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
	ctx := context.Background()

	err = r.Start(ctx, componenttest.NewNopHost())
	require.NoError(t, err)

	err = r.Shutdown(ctx)
	require.NoError(t, err)
}

func TestReceiverForNilConsumer(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	metricsReceiver, err := newAWSECSContainermetrics(
		zap.NewNop(),
		cfg,
		nil,
		&fakeRestClient{},
	)

	require.NotNil(t, err)
	require.Nil(t, metricsReceiver)
}

func TestCollectDataFromEndpoint(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	metricsReceiver, err := newAWSECSContainermetrics(
		zap.NewNop(),
		cfg,
		new(consumertest.MetricsSink),
		&fakeRestClient{},
	)

	require.NoError(t, err)
	require.NotNil(t, metricsReceiver)

	r := metricsReceiver.(*awsEcsContainerMetricsReceiver)
	ctx := context.Background()

	err = r.collectDataFromEndpoint(ctx)
	require.NoError(t, err)
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
	ctx := context.Background()

	err = r.collectDataFromEndpoint(ctx)
	require.EqualError(t, err, "Test Error for Metrics Consumer")
}

type invalidFakeClient struct {
}

func (f invalidFakeClient) GetResponse(path string) ([]byte, error) {
	return nil, fmt.Errorf("intentional error")
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
	ctx := context.Background()

	err = r.collectDataFromEndpoint(ctx)
	require.Error(t, err)
}
