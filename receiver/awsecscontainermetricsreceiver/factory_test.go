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
	"net/url"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configcheck"
	"go.opentelemetry.io/collector/consumer/consumertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsecscontainermetricsreceiver/internal/awsecscontainermetrics"
)

func TestValidConfig(t *testing.T) {
	err := configcheck.ValidateConfig(createDefaultConfig())
	require.NoError(t, err)
}

func TestCreateMetricsReceiver(t *testing.T) {
	os.Unsetenv(awsecscontainermetrics.EndpointEnvKey)

	metricsReceiver, err := createMetricsReceiver(
		context.Background(),
		componenttest.NewNopReceiverCreateSettings(),
		createDefaultConfig(),
		consumertest.NewNop(),
	)
	require.Error(t, err, "No Env Variable Error")
	require.Nil(t, metricsReceiver)
}

func TestCreateMetricsReceiverWithEnv(t *testing.T) {
	os.Setenv(awsecscontainermetrics.EndpointEnvKey, "http://www.test.com")

	metricsReceiver, err := createMetricsReceiver(
		context.Background(),
		componenttest.NewNopReceiverCreateSettings(),
		createDefaultConfig(),
		consumertest.NewNop(),
	)
	require.NoError(t, err)
	require.NotNil(t, metricsReceiver)
}

func TestCreateMetricsReceiverWithBadUrl(t *testing.T) {
	os.Setenv(awsecscontainermetrics.EndpointEnvKey, "bad-url-format")

	metricsReceiver, err := createMetricsReceiver(
		context.Background(),
		componenttest.NewNopReceiverCreateSettings(),
		createDefaultConfig(),
		consumertest.NewNop(),
	)
	require.Error(t, err)
	require.Nil(t, metricsReceiver)
}

func TestCreateMetricsReceiverWithNilConsumer(t *testing.T) {
	metricsReceiver, err := createMetricsReceiver(
		context.Background(),
		componenttest.NewNopReceiverCreateSettings(),
		createDefaultConfig(),
		nil,
	)

	require.Error(t, err, "Nil Comsumer")
	require.Nil(t, metricsReceiver)
}

func TestRestClient(t *testing.T) {
	u, _ := url.Parse("http://www.test.com")
	rest := restClient(nil, *u)

	require.NotNil(t, rest)
}
