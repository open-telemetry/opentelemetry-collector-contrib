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
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
<<<<<<< HEAD
	"go.opentelemetry.io/collector/exporter/exportertest"
=======
	"go.opentelemetry.io/collector/testbed/testbed"
>>>>>>> Add skeleton for AWS ECS container metrics receiver
	"go.uber.org/zap"
)

func TestReceiver(t *testing.T) {
<<<<<<< HEAD
	cfg := createDefaultConfig().(*Config)
	metricsReceiver, err := newAwsEcsContainerMetricsReceiver(
		zap.NewNop(),
		cfg,
		exportertest.NewNopMetricsExporter(),
=======
	factory := &Factory{}
	cfg := factory.CreateDefaultConfig().(*Config)
	metricsReceiver, err := New(
		zap.NewNop(),
		cfg,
		&testbed.MockMetricConsumer{},
>>>>>>> Add skeleton for AWS ECS container metrics receiver
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
<<<<<<< HEAD

func TestCollectDataFromEndpoint(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	metricsReceiver, err := newAwsEcsContainerMetricsReceiver(
		zap.NewNop(),
		cfg,
		new(exportertest.SinkMetricsExporter),
	)

	require.NoError(t, err)
	require.NotNil(t, metricsReceiver)

	r := metricsReceiver.(*awsEcsContainerMetricsReceiver)
	ctx := context.Background()

	err = r.collectDataFromEndpoint(ctx)
	require.NoError(t, err)
}
=======
>>>>>>> Add skeleton for AWS ECS container metrics receiver
