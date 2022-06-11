// Copyright  The OpenTelemetry Authors
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

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/scrapertest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/scrapertest/golden"
)

type configFunc func(hostname string) *Config

type testCase struct {
	name         string
	cfg          configFunc
	expectedFile string
}

func TestPostgreSQLIntegration(t *testing.T) {
	testCases := []testCase{
		{
			name: "single_db",
			cfg: func(hostname string) *Config {
				f := NewFactory()
				cfg := f.CreateDefaultConfig().(*Config)
				cfg.Endpoint = net.JoinHostPort(hostname, "15432")
				cfg.Databases = []string{"otel"}
				cfg.Username = "otel"
				cfg.Password = "otel"
				cfg.Insecure = true
				return cfg
			},
			expectedFile: filepath.Join("testdata", "integration", "expected_single_db.json"),
		},
		{
			name: "multi_db",
			cfg: func(hostname string) *Config {
				f := NewFactory()
				cfg := f.CreateDefaultConfig().(*Config)
				cfg.Endpoint = net.JoinHostPort(hostname, "15432")
				cfg.Databases = []string{"otel", "otel2"}
				cfg.Username = "otel"
				cfg.Password = "otel"
				cfg.Insecure = true
				return cfg
			},
			expectedFile: filepath.Join("testdata", "integration", "expected_multi_db.json"),
		},
		{
			name: "all_db",
			cfg: func(hostname string) *Config {
				f := NewFactory()
				cfg := f.CreateDefaultConfig().(*Config)
				cfg.Endpoint = net.JoinHostPort(hostname, "15432")
				cfg.Databases = []string{}
				cfg.Username = "otel"
				cfg.Password = "otel"
				cfg.Insecure = true
				return cfg
			},
			expectedFile: filepath.Join("testdata", "integration", "expected_all_db.json"),
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
			expectedMetrics, err := golden.ReadMetrics(tc.expectedFile)
			require.NoError(t, err)

			f := NewFactory()
			consumer := new(consumertest.MetricsSink)
			settings := componenttest.NewNopReceiverCreateSettings()
			rcvr, err := f.CreateMetricsReceiver(context.Background(), settings, tc.cfg(hostname), consumer)
			require.NoError(t, err, "failed creating metrics receiver")
			require.NoError(t, rcvr.Start(context.Background(), componenttest.NewNopHost()))
			require.Eventuallyf(t, func() bool {
				return consumer.DataPointCount() > 0
			}, 2*time.Minute, 1*time.Second, "failed to receive more than 0 metrics")

			actualMetrics := consumer.AllMetrics()[0]

			require.NoError(t, scrapertest.CompareMetrics(expectedMetrics, actualMetrics, scrapertest.IgnoreMetricValues()))
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
