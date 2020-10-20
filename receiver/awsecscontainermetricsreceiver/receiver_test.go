// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package awsecscontainermetricsreceiver

import (
	"context"
	"fmt"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.uber.org/zap"
)

type fakeRestClient struct {
}

func (f fakeRestClient) EndpointResponse() ([]byte, []byte, error) {
	taskStats, err := ioutil.ReadFile("testdata/task_stats.json")
	if err != nil {
		return nil, nil, err
	}
	taskMetadata, err := ioutil.ReadFile("testdata/task_metadata.json")
	if err != nil {
		return nil, nil, err
	}
	return taskStats, taskMetadata, nil
}

func TestReceiver(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	metricsReceiver, err := New(
		zap.NewNop(),
		cfg,
		consumertest.NewMetricsNop(),
		&fakeRestClient{},
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
	metricsReceiver, err := New(
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
	metricsReceiver, err := New(
		zap.NewNop(),
		cfg,
		new(consumertest.MetricsSink),
		&fakeRestClient{},
	)

	require.NoError(t, err)
	require.NotNil(t, metricsReceiver)

	r := metricsReceiver.(*awsEcsContainerMetricsReceiver)
	ctx := context.Background()

	err = r.collectDataFromEndpoint(ctx, "")
	require.NoError(t, err)
}

func TestCollectDataFromEndpointWithConsumerError(t *testing.T) {
	cfg := createDefaultConfig().(*Config)

	sme := new(consumertest.MetricsSink)
	e := fmt.Errorf("Test Error for Metrics Consumer")
	sme.SetConsumeError(e)

	metricsReceiver, err := New(
		zap.NewNop(),
		cfg,
		sme,
		&fakeRestClient{},
	)

	require.NoError(t, err)
	require.NotNil(t, metricsReceiver)

	r := metricsReceiver.(*awsEcsContainerMetricsReceiver)
	ctx := context.Background()

	err = r.collectDataFromEndpoint(ctx, "")
	require.EqualError(t, err, "Test Error for Metrics Consumer")
}

type invalidFakeClient struct {
}

func (f invalidFakeClient) EndpointResponse() ([]byte, []byte, error) {
	taskStats, err := ioutil.ReadFile("testdata/wrong_file.json")
	if err != nil {
		return nil, nil, err
	}
	taskMetadata, err := ioutil.ReadFile("testdata/wrong_file.json")
	if err != nil {
		return nil, nil, err
	}
	return taskStats, taskMetadata, nil
}

func TestCollectDataFromEndpointWithEndpointError(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	metricsReceiver, err := New(
		zap.NewNop(),
		cfg,
		new(consumertest.MetricsSink),
		&invalidFakeClient{},
	)

	require.NoError(t, err)
	require.NotNil(t, metricsReceiver)

	r := metricsReceiver.(*awsEcsContainerMetricsReceiver)
	ctx := context.Background()

	err = r.collectDataFromEndpoint(ctx, "")
	require.Error(t, err)
}
