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
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configcheck"
	"go.opentelemetry.io/collector/testbed/testbed"
	"go.uber.org/zap"
)

func TestValidConfig(t *testing.T) {
	err := configcheck.ValidateConfig(createDefaultConfig())
	require.NoError(t, err)
}

func TestCreateMetricsReceiver(t *testing.T) {
	metricsReceiver, err := createMetricsReceiver(
		context.Background(),
		component.ReceiverCreateParams{Logger: zap.NewNop()},
		createDefaultConfig(),
		&testbed.MockMetricConsumer{},
	)
	require.Error(t, err, "No Env Variable Error")
	require.Nil(t, metricsReceiver)
}

func TestCreateMetricsReceiverWithNilConsumer(t *testing.T) {
	metricsReceiver, err := createMetricsReceiver(
		context.Background(),
		component.ReceiverCreateParams{Logger: zap.NewNop()},
		createDefaultConfig(),
		nil,
	)

	require.Error(t, err, "Nil Comsumer")
	require.Nil(t, metricsReceiver)

}
