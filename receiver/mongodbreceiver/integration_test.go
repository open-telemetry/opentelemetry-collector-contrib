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

package mongodbreceiver

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
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/consumer/consumertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/scrapertest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/scrapertest/golden"
)

var (
	LPUSetupScript      = []string{"/lpu.sh"}
	setupScript         = []string{"/setup.sh"}
	containerRequest2_6 = testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    filepath.Join("testdata", "integration"),
			Dockerfile: "Dockerfile.mongodb.2_6",
		},
		ExposedPorts: []string{"27017:27017"},
		WaitingFor:   wait.ForListeningPort("27017").WithStartupTimeout(2 * time.Minute),
	}
	containerRequest3_0 = testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    filepath.Join("testdata", "integration"),
			Dockerfile: "Dockerfile.mongodb.3_0",
		},
		ExposedPorts: []string{"27117:27017"},
		WaitingFor:   wait.ForListeningPort("27017").WithStartupTimeout(2 * time.Minute),
	}
	containerRequest4_0 = testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    filepath.Join("testdata", "integration"),
			Dockerfile: "Dockerfile.mongodb.4_0",
		},
		ExposedPorts: []string{"27217:27017"},
		WaitingFor:   wait.ForListeningPort("27017").WithStartupTimeout(2 * time.Minute),
	}
	containerRequest4_4LPU = testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    filepath.Join("testdata", "integration"),
			Dockerfile: "Dockerfile.mongodb.4_4.lpu",
		},
		ExposedPorts: []string{"27317:27017"},
		WaitingFor:   wait.ForListeningPort("27017").WithStartupTimeout(2 * time.Minute),
	}
	containerRequest5_0 = testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    filepath.Join("testdata", "integration"),
			Dockerfile: "Dockerfile.mongodb.5_0",
		},
		ExposedPorts: []string{"27417:27017"},
		WaitingFor:   wait.ForListeningPort("27017").WithStartupTimeout(2 * time.Minute),
	}
)

func TestMongodbIntegration(t *testing.T) {
	t.Run("Running mongodb 2.6", func(t *testing.T) {
		t.Parallel()
		container := getContainer(t, containerRequest2_6, setupScript)
		defer func() {
			require.NoError(t, container.Terminate(context.Background()))
		}()
		hostname, err := container.Host(context.Background())
		require.NoError(t, err)

		f := NewFactory()
		cfg := f.CreateDefaultConfig().(*Config)
		cfg.Hosts = []confignet.NetAddr{
			{
				Endpoint: net.JoinHostPort(hostname, "27017"),
			},
		}
		cfg.Insecure = true

		consumer := new(consumertest.MetricsSink)
		settings := componenttest.NewNopReceiverCreateSettings()
		rcvr, err := f.CreateMetricsReceiver(context.Background(), settings, cfg, consumer)
		require.NoError(t, err, "failed creating metrics receiver")

		require.NoError(t, rcvr.Start(context.Background(), componenttest.NewNopHost()))
		require.Eventuallyf(t, func() bool {
			return len(consumer.AllMetrics()) > 0
		}, 2*time.Minute, 1*time.Second, "failed to receive more than 0 metrics")
		require.NoError(t, rcvr.Shutdown(context.Background()))

		actualMetrics := consumer.AllMetrics()[0]

		expectedFile := filepath.Join("testdata", "integration", "expected.2_6.json")
		expectedMetrics, err := golden.ReadMetrics(expectedFile)
		require.NoError(t, err)

		err = scrapertest.CompareMetrics(expectedMetrics, actualMetrics, scrapertest.IgnoreMetricValues())
		require.NoError(t, err)
	})
	t.Run("Running mongodb 3.0", func(t *testing.T) {
		t.Parallel()
		container := getContainer(t, containerRequest3_0, setupScript)
		defer func() {
			require.NoError(t, container.Terminate(context.Background()))
		}()
		hostname, err := container.Host(context.Background())
		require.NoError(t, err)

		f := NewFactory()
		cfg := f.CreateDefaultConfig().(*Config)
		cfg.Hosts = []confignet.NetAddr{
			{
				Endpoint: net.JoinHostPort(hostname, "27117"),
			},
		}
		cfg.Insecure = true

		consumer := new(consumertest.MetricsSink)
		settings := componenttest.NewNopReceiverCreateSettings()
		rcvr, err := f.CreateMetricsReceiver(context.Background(), settings, cfg, consumer)
		require.NoError(t, err, "failed creating metrics receiver")

		require.NoError(t, rcvr.Start(context.Background(), componenttest.NewNopHost()))
		require.Eventuallyf(t, func() bool {
			return len(consumer.AllMetrics()) > 0
		}, 2*time.Minute, 1*time.Second, "failed to receive more than 0 metrics")
		require.NoError(t, rcvr.Shutdown(context.Background()))

		actualMetrics := consumer.AllMetrics()[0]

		expectedFile := filepath.Join("testdata", "integration", "expected.3_0.json")
		expectedMetrics, err := golden.ReadMetrics(expectedFile)
		require.NoError(t, err)

		err = scrapertest.CompareMetrics(expectedMetrics, actualMetrics, scrapertest.IgnoreMetricValues())
		require.NoError(t, err)
	})
	t.Run("Running mongodb 4.0", func(t *testing.T) {
		t.Parallel()
		container := getContainer(t, containerRequest4_0, setupScript)
		defer func() {
			require.NoError(t, container.Terminate(context.Background()))
		}()
		hostname, err := container.Host(context.Background())
		require.NoError(t, err)

		f := NewFactory()
		cfg := f.CreateDefaultConfig().(*Config)
		cfg.Hosts = []confignet.NetAddr{
			{
				Endpoint: net.JoinHostPort(hostname, "27217"),
			},
		}
		cfg.Insecure = true

		consumer := new(consumertest.MetricsSink)
		settings := componenttest.NewNopReceiverCreateSettings()
		rcvr, err := f.CreateMetricsReceiver(context.Background(), settings, cfg, consumer)
		require.NoError(t, err, "failed creating metrics receiver")

		require.NoError(t, rcvr.Start(context.Background(), componenttest.NewNopHost()))
		require.Eventuallyf(t, func() bool {
			return len(consumer.AllMetrics()) > 0
		}, 2*time.Minute, 1*time.Second, "failed to receive more than 0 metrics")
		require.NoError(t, rcvr.Shutdown(context.Background()))

		actualMetrics := consumer.AllMetrics()[0]

		expectedFile := filepath.Join("testdata", "integration", "expected.4_0.json")
		expectedMetrics, err := golden.ReadMetrics(expectedFile)
		require.NoError(t, err)

		err = scrapertest.CompareMetrics(expectedMetrics, actualMetrics, scrapertest.IgnoreMetricValues())
		require.NoError(t, err)
	})
	t.Run("Running mongodb 4.4 as LPU", func(t *testing.T) {
		t.Parallel()
		container := getContainer(t, containerRequest4_4LPU, LPUSetupScript)
		defer func() {
			require.NoError(t, container.Terminate(context.Background()))
		}()
		hostname, err := container.Host(context.Background())
		require.NoError(t, err)

		f := NewFactory()
		cfg := f.CreateDefaultConfig().(*Config)
		cfg.Username = "otelu"
		cfg.Password = "otelp"
		cfg.Hosts = []confignet.NetAddr{
			{
				Endpoint: net.JoinHostPort(hostname, "27317"),
			},
		}
		cfg.Insecure = true

		consumer := new(consumertest.MetricsSink)
		settings := componenttest.NewNopReceiverCreateSettings()
		rcvr, err := f.CreateMetricsReceiver(context.Background(), settings, cfg, consumer)
		require.NoError(t, err, "failed creating metrics receiver")

		require.NoError(t, rcvr.Start(context.Background(), componenttest.NewNopHost()))
		require.Eventuallyf(t, func() bool {
			return len(consumer.AllMetrics()) > 0
		}, 2*time.Minute, 1*time.Second, "failed to receive more than 0 metrics")
		require.NoError(t, rcvr.Shutdown(context.Background()))

		actualMetrics := consumer.AllMetrics()[0]

		expectedFile := filepath.Join("testdata", "integration", "expected.4_4.lpu.json")
		expectedMetrics, err := golden.ReadMetrics(expectedFile)
		require.NoError(t, err)

		err = scrapertest.CompareMetrics(expectedMetrics, actualMetrics, scrapertest.IgnoreMetricValues())
		require.NoError(t, err)
	})
	t.Run("Running mongodb 5.0", func(t *testing.T) {
		t.Parallel()
		container := getContainer(t, containerRequest5_0, setupScript)
		defer func() {
			require.NoError(t, container.Terminate(context.Background()))
		}()
		hostname, err := container.Host(context.Background())
		require.NoError(t, err)

		f := NewFactory()
		cfg := f.CreateDefaultConfig().(*Config)
		cfg.Hosts = []confignet.NetAddr{
			{
				Endpoint: net.JoinHostPort(hostname, "27417"),
			},
		}
		cfg.Insecure = true

		consumer := new(consumertest.MetricsSink)
		settings := componenttest.NewNopReceiverCreateSettings()
		rcvr, err := f.CreateMetricsReceiver(context.Background(), settings, cfg, consumer)
		require.NoError(t, err, "failed creating metrics receiver")

		require.NoError(t, rcvr.Start(context.Background(), componenttest.NewNopHost()))
		require.Eventuallyf(t, func() bool {
			return len(consumer.AllMetrics()) > 0
		}, 2*time.Minute, 1*time.Second, "failed to receive more than 0 metrics")
		require.NoError(t, rcvr.Shutdown(context.Background()))

		actualMetrics := consumer.AllMetrics()[0]

		expectedFile := filepath.Join("testdata", "integration", "expected.5_0.json")
		expectedMetrics, err := golden.ReadMetrics(expectedFile)
		require.NoError(t, err)

		require.NoError(t, scrapertest.CompareMetrics(expectedMetrics, actualMetrics, scrapertest.IgnoreMetricValues()))
	})
}

func getContainer(t *testing.T, req testcontainers.ContainerRequest, script []string) testcontainers.Container {
	require.NoError(t, req.Validate())
	container, err := testcontainers.GenericContainer(
		context.Background(),
		testcontainers.GenericContainerRequest{
			ContainerRequest: req,
			Started:          true,
		})
	require.NoError(t, err)

	code, err := container.Exec(context.Background(), script)
	require.NoError(t, err)
	require.Equal(t, 0, code)

	err = container.Start(context.Background())
	require.NoError(t, err)
	return container
}
