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

type postgresVersion string

var (
	pg96 postgresVersion = "pg9.6"
	pg10 postgresVersion = "pg10"
	pg14 postgresVersion = "pg14"

	containerRequest9_6 = testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    filepath.Join("testdata", "integration"),
			Dockerfile: filepath.Join("docker", "Dockerfile.9.6"),
		},
		ExposedPorts: []string{"15432:5432"},
		WaitingFor: wait.ForListeningPort("5432").
			WithStartupTimeout(2 * time.Minute),
	}

	containerRequest10 = testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    filepath.Join("testdata", "integration"),
			Dockerfile: filepath.Join("docker", "Dockerfile.10"),
		},
		ExposedPorts: []string{"15433:5432"},
		WaitingFor: wait.ForListeningPort("5432").
			WithStartupTimeout(2 * time.Minute),
	}

	containerRequest14 = testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    filepath.Join("testdata", "integration"),
			Dockerfile: filepath.Join("docker", "Dockerfile.14"),
		},
		ExposedPorts: []string{"15434:5432"},
		WaitingFor: wait.ForListeningPort("5432").
			WithStartupTimeout(2 * time.Minute),
	}
)

type testCase struct {
	name            string
	cfg             configFunc
	expectedFile    string
	postgresVersion postgresVersion
	scraperOptions  []scrapertest.CompareOption
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
			postgresVersion: pg96,
			expectedFile:    filepath.Join("testdata", "integration", "expected_single_db.json"),
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
			postgresVersion: pg96,
			expectedFile:    filepath.Join("testdata", "integration", "expected_multi_db.json"),
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
			postgresVersion: pg96,
			expectedFile:    filepath.Join("testdata", "integration", "expected_all_db.json"),
		},
		{
			name: "query_stats",
			cfg: func(hostname string) *Config {
				f := NewFactory()
				cfg := f.CreateDefaultConfig().(*Config)
				cfg.Endpoint = net.JoinHostPort(hostname, "15433")
				cfg.Username = "otel"
				cfg.Password = "otel"
				cfg.Insecure = true
				cfg.CollectQueryPerformance = true
				return cfg
			},
			// query metrics require Postgres > 10
			postgresVersion: pg10,
			expectedFile:    filepath.Join("testdata", "integration", "expected_with_query_stats.json"),
			// ignore time dependent query metric attribute
			scraperOptions: []scrapertest.CompareOption{scrapertest.IgnoreMetricAttributeValue(
				"query",
				"postgresql.query.count",
				"postgresql.query.block.count",
				"postgresql.query.duration.total",
				"postgresql.query.duration.average",
			)},
		},
		{
			name: "single_db_pg14",
			cfg: func(hostname string) *Config {
				f := NewFactory()
				cfg := f.CreateDefaultConfig().(*Config)
				cfg.Endpoint = net.JoinHostPort(hostname, "15433")
				cfg.Databases = []string{"otel"}
				cfg.Username = "otel"
				cfg.Password = "otel"
				cfg.Insecure = true
				return cfg
			},
			postgresVersion: pg14,
			expectedFile:    filepath.Join("testdata", "integration", "expected_single_db_14.json"),
		},
	}

	t.Run("postgres9.6", func(t *testing.T) {
		t.Parallel()
		container := getContainer(t, containerRequest9_6)
		defer func() {
			require.NoError(t, container.Terminate(context.Background()))
		}()
		hostname, err := container.Host(context.Background())
		require.NoError(t, err)

		for _, tc := range testCases {
			if tc.postgresVersion != pg96 {
				continue
			}
			t.Run(tc.name, func(t *testing.T) {
				runTest(t, tc, hostname)
			})
		}
	})

	t.Run("postgres10.21", func(t *testing.T) {
		t.Parallel()
		container := getContainer(t, containerRequest10)
		defer func() {
			require.NoError(t, container.Terminate(context.Background()))
		}()
		hostname, err := container.Host(context.Background())
		require.NoError(t, err)

		for _, tc := range testCases {
			if tc.postgresVersion != pg10 {
				continue
			}
			t.Run(tc.name, func(t *testing.T) {
				runTest(t, tc, hostname)
			})
		}
	})

	t.Run("postgres14", func(t *testing.T) {
		t.Parallel()
		container := getContainer(t, containerRequest14)
		defer func() {
			require.NoError(t, container.Terminate(context.Background()))
		}()
		hostname, err := container.Host(context.Background())
		require.NoError(t, err)
		for _, tc := range testCases {
			if tc.postgresVersion != pg14 {
				continue
			}
			runTest(t, tc, hostname)
		}
	})
}

func runTest(t *testing.T, tc testCase, hostname string) {
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
	require.NoError(t, rcvr.Shutdown(context.Background()))

	actualMetrics := consumer.AllMetrics()[0]

	options := []scrapertest.CompareOption{scrapertest.IgnoreMetricValues()}
	options = append(options, tc.scraperOptions...)
	require.NoError(t, scrapertest.CompareMetrics(expectedMetrics, actualMetrics, options...))
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
