// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package snmpreceiver

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/otelcol/otelcoltest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/snmpreceiver/internal/metadata"
)

func TestIntegration(t *testing.T) {
	// remove nolint when https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/24240 is resolved
	// nolint:staticcheck
	testCases := []struct {
		desc                    string
		configFilename          string
		expectedResultsFilename string
	}{
		{
			desc:                    "Integration test with v2c configuration",
			configFilename:          "integration_test_v2c_config.yaml",
			expectedResultsFilename: "v2c_config_expected_metrics.yaml",
		},
		{
			desc:                    "Integration test with v3 configuration",
			configFilename:          "integration_test_v3_config.yaml",
			expectedResultsFilename: "v3_config_expected_metrics.yaml",
		},
	}

	container := getContainer(t, snmpAgentContainerRequest)
	defer func() {
		require.NoError(t, container.Terminate(context.Background()))
	}()
	_, err := container.Host(context.Background())
	require.NoError(t, err)
	factories, err := otelcoltest.NopFactories()
	require.NoError(t, err)

	for _, testCase := range testCases {
		t.Run(testCase.desc, func(t *testing.T) {
			t.Skip("Flaky test, see https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/21086")
			factory := NewFactory()
			factories.Receivers[metadata.Type] = factory
			configFile := filepath.Join("testdata", "integration", testCase.configFilename)
			// https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/33594
			// nolint:staticcheck
			cfg, err := otelcoltest.LoadConfigAndValidate(configFile, factories)
			require.NoError(t, err)
			snmpConfig := cfg.Receivers[component.NewID(metadata.Type)].(*Config)

			consumer := new(consumertest.MetricsSink)
			settings := receivertest.NewNopSettings()
			rcvr, err := factory.CreateMetrics(context.Background(), settings, snmpConfig, consumer)
			require.NoError(t, err, "failed creating metrics receiver")
			require.NoError(t, rcvr.Start(context.Background(), componenttest.NewNopHost()))
			require.Eventuallyf(t, func() bool {
				return len(consumer.AllMetrics()) > 0
			}, 2*time.Minute, 1*time.Second, "failed to receive more than 0 metrics")
			require.NoError(t, rcvr.Shutdown(context.Background()))

			actualMetrics := consumer.AllMetrics()[0]
			expectedFile := filepath.Join("testdata", "integration", testCase.expectedResultsFilename)
			expectedMetrics, err := golden.ReadMetrics(expectedFile)
			require.NoError(t, err)
			err = pmetrictest.CompareMetrics(expectedMetrics, actualMetrics, pmetrictest.IgnoreMetricsOrder(),
				pmetrictest.IgnoreTimestamp(), pmetrictest.IgnoreStartTimestamp())
			require.NoError(t, err)
		})
	}
}

var (
	snmpAgentContainerRequest = testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    filepath.Join("testdata", "integration", "docker"),
			Dockerfile: "snmp_agent.Dockerfile",
		},
		ExposedPorts: []string{"1024:1024/udp"},
	}
)

func getContainer(t *testing.T, req testcontainers.ContainerRequest) testcontainers.Container {
	require.NoError(t, req.Validate())
	container, err := testcontainers.GenericContainer(
		context.Background(),
		testcontainers.GenericContainerRequest{
			ContainerRequest: req,
			Started:          true,
		})
	require.NoError(t, err)
	return container
}
