// Copyright The OpenTelemetry Authors
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
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/comparetest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/comparetest/golden"
)

const (
	mongoDBPort = "27017/tcp"
)

var (
	LPUSetupScript      = []string{"/lpu.sh"}
	setupScript         = []string{"/setup.sh"}
	containerRequest4_0 = testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    filepath.Join("testdata", "integration"),
			Dockerfile: "Dockerfile.mongodb.4_0",
		},
		ExposedPorts: []string{mongoDBPort},
		WaitingFor:   wait.ForListeningPort(mongoDBPort).WithStartupTimeout(2 * time.Minute),
	}
	containerRequest4_2 = testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    filepath.Join("testdata", "integration"),
			Dockerfile: "Dockerfile.mongodb.4_2",
		},
		ExposedPorts: []string{mongoDBPort},
		WaitingFor:   wait.ForListeningPort(mongoDBPort).WithStartupTimeout(2 * time.Minute),
	}
	containerRequest4_4LPU = testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    filepath.Join("testdata", "integration"),
			Dockerfile: "Dockerfile.mongodb.4_4.lpu",
		},
		ExposedPorts: []string{mongoDBPort},
		WaitingFor:   wait.ForListeningPort(mongoDBPort).WithStartupTimeout(2 * time.Minute),
	}
	containerRequest5_0 = testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    filepath.Join("testdata", "integration"),
			Dockerfile: "Dockerfile.mongodb.5_0",
		},
		ExposedPorts: []string{mongoDBPort},
		WaitingFor:   wait.ForListeningPort(mongoDBPort).WithStartupTimeout(2 * time.Minute),
	}
)

func TestMongodbIntegration(t *testing.T) {
	t.Run("Running mongodb 4.0", func(t *testing.T) {
		t.Skip("Refer to https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/17070")
		t.Parallel()
		container, endpoint := getContainer(t, containerRequest4_0, setupScript)
		defer func() {
			require.NoError(t, container.Terminate(context.Background()))
		}()

		f := NewFactory()
		cfg := f.CreateDefaultConfig().(*Config)
		cfg.Metrics.MongodbLockAcquireTime.Enabled = false
		cfg.Hosts = []confignet.NetAddr{
			{
				Endpoint: endpoint,
			},
		}
		cfg.Insecure = true

		consumer := new(consumertest.MetricsSink)
		settings := receivertest.NewNopCreateSettings()
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

		golden.WriteMetrics("actual_metrics.json", actualMetrics)

		err = comparetest.CompareMetrics(expectedMetrics, actualMetrics, comparetest.IgnoreMetricValues())
		require.NoError(t, err)
	})
	t.Run("Running mongodb 4.2", func(t *testing.T) {
		t.Skip("Refer to https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/17070")
		t.Parallel()
		container, endpoint := getContainer(t, containerRequest4_2, setupScript)
		defer func() {
			require.NoError(t, container.Terminate(context.Background()))
		}()

		f := NewFactory()
		cfg := f.CreateDefaultConfig().(*Config)
		cfg.Hosts = []confignet.NetAddr{
			{
				Endpoint: endpoint,
			},
		}
		cfg.Insecure = true

		consumer := new(consumertest.MetricsSink)
		settings := receivertest.NewNopCreateSettings()
		rcvr, err := f.CreateMetricsReceiver(context.Background(), settings, cfg, consumer)
		require.NoError(t, err, "failed creating metrics receiver")

		require.NoError(t, rcvr.Start(context.Background(), componenttest.NewNopHost()))
		require.Eventuallyf(t, func() bool {
			return len(consumer.AllMetrics()) > 0
		}, 2*time.Minute, 1*time.Second, "failed to receive more than 0 metrics")
		require.NoError(t, rcvr.Shutdown(context.Background()))

		actualMetrics := consumer.AllMetrics()[0]

		expectedFile := filepath.Join("testdata", "integration", "expected.4_2.json")
		expectedMetrics, err := golden.ReadMetrics(expectedFile)
		require.NoError(t, err)

		err = comparetest.CompareMetrics(expectedMetrics, actualMetrics, comparetest.IgnoreMetricValues())
		require.NoError(t, err)
	})
	t.Run("Running mongodb 4.4 as LPU", func(t *testing.T) {
		t.Parallel()
		container, endpoint := getContainer(t, containerRequest4_4LPU, LPUSetupScript)
		defer func() {
			require.NoError(t, container.Terminate(context.Background()))
		}()

		f := NewFactory()
		cfg := f.CreateDefaultConfig().(*Config)
		cfg.Username = "otelu"
		cfg.Password = "otelp"
		cfg.Hosts = []confignet.NetAddr{
			{
				Endpoint: endpoint,
			},
		}
		cfg.Insecure = true

		consumer := new(consumertest.MetricsSink)
		settings := receivertest.NewNopCreateSettings()
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

		err = comparetest.CompareMetrics(expectedMetrics, actualMetrics, comparetest.IgnoreMetricValues())
		require.NoError(t, err)
	})
	t.Run("Running mongodb 5.0", func(t *testing.T) {
		t.Parallel()
		container, endpoint := getContainer(t, containerRequest5_0, setupScript)
		defer func() {
			require.NoError(t, container.Terminate(context.Background()))
		}()

		f := NewFactory()
		cfg := f.CreateDefaultConfig().(*Config)
		cfg.Hosts = []confignet.NetAddr{
			{
				Endpoint: endpoint,
			},
		}
		cfg.Insecure = true

		consumer := new(consumertest.MetricsSink)
		settings := receivertest.NewNopCreateSettings()
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

		require.NoError(t, comparetest.CompareMetrics(expectedMetrics, actualMetrics, comparetest.IgnoreMetricValues()))
	})
}

func getContainer(t *testing.T, req testcontainers.ContainerRequest, script []string) (testcontainers.Container, string) {
	require.NoError(t, req.Validate())

	ctx := context.Background()

	container, err := testcontainers.GenericContainer(
		ctx,
		testcontainers.GenericContainerRequest{
			ContainerRequest: req,
			Started:          true,
		})
	require.NoError(t, err)

	code, _, err := container.Exec(context.Background(), script)
	require.NoError(t, err)
	require.Equal(t, 0, code)

	err = container.Start(context.Background())
	require.NoError(t, err)

	mappedPort, err := container.MappedPort(ctx, mongoDBPort)
	require.Nil(t, err)

	hostIP, err := container.Host(ctx)
	require.Nil(t, err)

	mongoDBEndpoint := fmt.Sprintf("%s:%s", hostIP, mappedPort.Port())

	return container, mongoDBEndpoint
}
