// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsecscontainermetricsreceiver

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/ecsutil/endpoints"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsecscontainermetricsreceiver/internal/metadata"
)

func TestValidConfig(t *testing.T) {
	err := componenttest.CheckConfigStruct(createDefaultConfig())
	require.NoError(t, err)
}

func TestCreateMetrics(t *testing.T) {
	metricsReceiver, err := createMetricsReceiver(
		t.Context(),
		receivertest.NewNopSettings(metadata.Type),
		createDefaultConfig(),
		consumertest.NewNop(),
	)
	require.Error(t, err, "No Env Variable Error")
	require.Nil(t, metricsReceiver)
}

func TestCreateMetricsWithEnv(t *testing.T) {
	t.Setenv(endpoints.TaskMetadataEndpointV4EnvVar, "http://www.test.com")

	metricsReceiver, err := createMetricsReceiver(
		t.Context(),
		receivertest.NewNopSettings(metadata.Type),
		createDefaultConfig(),
		consumertest.NewNop(),
	)
	require.NoError(t, err)
	require.NotNil(t, metricsReceiver)
}

func TestCreateMetricsWithBadUrl(t *testing.T) {
	t.Setenv(endpoints.TaskMetadataEndpointV4EnvVar, "bad-url-format")

	metricsReceiver, err := createMetricsReceiver(
		t.Context(),
		receivertest.NewNopSettings(metadata.Type),
		createDefaultConfig(),
		consumertest.NewNop(),
	)
	require.Error(t, err)
	require.Nil(t, metricsReceiver)
}

func TestCreateMetricsWithNilConsumer(t *testing.T) {
	metricsReceiver, err := createMetricsReceiver(
		t.Context(),
		receivertest.NewNopSettings(metadata.Type),
		createDefaultConfig(),
		nil,
	)

	require.Error(t, err, "Nil Consumer")
	require.Nil(t, metricsReceiver)
}

func TestCreateMetricsRequiresEmitV1WhenDontEmitV0(t *testing.T) {
	defer testutil.SetFeatureGateForTest(t, metadata.ReceiverAwsecscontainermetricsEmitV1ContainerConventionsFeatureGate, false)()
	defer testutil.SetFeatureGateForTest(t, metadata.ReceiverAwsecscontainermetricsDontEmitV0ContainerConventionsFeatureGate, true)()
	t.Setenv(endpoints.TaskMetadataEndpointV4EnvVar, "http://www.test.com")

	metricsReceiver, err := createMetricsReceiver(t.Context(), receivertest.NewNopSettings(metadata.Type), createDefaultConfig(), consumertest.NewNop())
	require.ErrorContains(t, err, "requires receiver.awsecscontainermetrics.EmitV1ContainerConventions to be enabled")
	require.Nil(t, metricsReceiver)
}
