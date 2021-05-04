// Copyright  OpenTelemetry Authors
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

package awscontainerinsightreceiver

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"
)

func TestReceiver(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	metricsReceiver, err := New(
		zap.NewNop(),
		cfg,
		consumertest.NewNop(),
	)

	require.NoError(t, err)
	require.NotNil(t, metricsReceiver)

	r := metricsReceiver.(*awsContainerInsightReceiver)
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
	)

	require.NotNil(t, err)
	require.Nil(t, metricsReceiver)
}

func TestCollectData(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	metricsReceiver, err := New(
		zap.NewNop(),
		cfg,
		new(consumertest.MetricsSink),
	)

	require.NoError(t, err)
	require.NotNil(t, metricsReceiver)

	r := metricsReceiver.(*awsContainerInsightReceiver)
	r.Start(context.Background(), nil)
	ctx := context.Background()

	err = r.collectData(ctx)
	require.Nil(t, err)

	//test the case when cadvisor and k8sapiserver failed to initialize
	r.cadvisor = nil
	r.k8sapiserver = nil
	err = r.collectData(ctx)
	require.NotNil(t, err)
}

//Mock cadvisor
type MockCadvisor struct {
}

func (c *MockCadvisor) GetMetrics() []pdata.Metrics {
	md := pdata.NewMetrics()
	return []pdata.Metrics{md}
}

func TestCollectDataWithErrConsumer(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	metricsReceiver, err := New(
		zap.NewNop(),
		cfg,
		consumertest.NewErr(errors.New("an error")),
	)

	require.NoError(t, err)
	require.NotNil(t, metricsReceiver)

	r := metricsReceiver.(*awsContainerInsightReceiver)
	r.Start(context.Background(), nil)
	r.cadvisor = &MockCadvisor{}
	ctx := context.Background()

	err = r.collectData(ctx)
	require.NotNil(t, err)
}
