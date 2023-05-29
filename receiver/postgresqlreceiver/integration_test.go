// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration
// +build integration

package postgresqlreceiver

import (
	"context"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

type configFunc func(hostname string) *Config

// cleanupFunc exists to allow integration test cases to clean any registries it had
// to modify in order to change behavior of the integration test. i.e. featuregates
type cleanupFunc func()

type testCase struct {
	name         string
	cfg          configFunc
	cleanup      cleanupFunc
	expectedFile string
}

func TestIntegration(t *testing.T) {
	testCases := []testCase{
		{
			name: "single_db",
			cfg: func(hostname string) *Config {
				f := NewFactory()
				cfg := f.CreateDefaultConfig().(*Config)
				cfg.Endpoint = net.JoinHostPort(hostname, "15432")
				cfg.Databases = []string{"otel"}
				cfg.Username = "otelu"
				cfg.Password = "otelp"
				cfg.Insecure = true
				return cfg
			},
			expectedFile: filepath.Join("testdata", "integration", "expected_single_db.yaml"),
		},
		{
			name: "multi_db",
			cfg: func(hostname string) *Config {
				f := NewFactory()
				cfg := f.CreateDefaultConfig().(*Config)
				cfg.Endpoint = net.JoinHostPort(hostname, "15432")
				cfg.Databases = []string{"otel", "otel2"}
				cfg.Username = "otelu"
				cfg.Password = "otelp"
				cfg.Insecure = true
				return cfg
			},
			expectedFile: filepath.Join("testdata", "integration", "expected_multi_db.yaml"),
		},
		{
			name: "all_db",
			cfg: func(hostname string) *Config {
				f := NewFactory()
				cfg := f.CreateDefaultConfig().(*Config)
				cfg.Endpoint = net.JoinHostPort(hostname, "15432")
				cfg.Databases = []string{}
				cfg.Username = "otelu"
				cfg.Password = "otelp"
				cfg.Insecure = true
				return cfg
			},
			expectedFile: filepath.Join("testdata", "integration", "expected_all_db.yaml"),
		},
	}

	container := getContainer(t, testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    filepath.Join("testdata", "integration"),
			Dockerfile: "Dockerfile.postgresql",
		},
		ExposedPorts: []string{"15432:5432"},
		WaitingFor: wait.ForListeningPort("5432").
			WithStartupTimeout(2 * time.Minute),
	})
	defer func() {
		require.NoError(t, container.Terminate(context.Background()))
	}()
	hostname, err := container.Host(context.Background())
	require.NoError(t, err)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.cleanup != nil {
				defer tc.cleanup()
			}
			expectedMetrics, err := golden.ReadMetrics(tc.expectedFile)
			require.NoError(t, err)

			f := NewFactory()
			consumer := new(consumertest.MetricsSink)
			settings := receivertest.NewNopCreateSettings()
			rcvr, err := f.CreateMetricsReceiver(context.Background(), settings, tc.cfg(hostname), consumer)
			require.NoError(t, err, "failed creating metrics receiver")
			require.NoError(t, rcvr.Start(context.Background(), componenttest.NewNopHost()))
			require.Eventuallyf(t, func() bool {
				return consumer.DataPointCount() > 0
			}, 2*time.Minute, 1*time.Second, "failed to receive more than 0 metrics")

			actualMetrics := consumer.AllMetrics()[0]

			require.NoError(t, pmetrictest.CompareMetrics(
				expectedMetrics, actualMetrics,
				pmetrictest.IgnoreResourceMetricsOrder(),
				pmetrictest.IgnoreMetricValues(),
				pmetrictest.IgnoreSubsequentDataPoints("postgresql.backends"),
				pmetrictest.IgnoreMetricDataPointsOrder(),
				pmetrictest.IgnoreStartTimestamp(),
				pmetrictest.IgnoreTimestamp(),
			))
		})
	}
}

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
