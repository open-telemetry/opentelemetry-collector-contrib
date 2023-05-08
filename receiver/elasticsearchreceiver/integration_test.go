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

package elasticsearchreceiver

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
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

var (
	containerRequest7_0_0 = testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    filepath.Join("testdata", "integration"),
			Dockerfile: "Dockerfile.elasticsearch.7_0_0",
		},
		ExposedPorts: []string{"9600:9200"},
		WaitingFor: wait.ForListeningPort("9200").
			WithStartupTimeout(2 * time.Minute),
	}
	containerRequest7_9_3 = testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    filepath.Join("testdata", "integration"),
			Dockerfile: "Dockerfile.elasticsearch.7_9_3",
		},
		ExposedPorts: []string{"9200:9200"},
		WaitingFor: wait.ForListeningPort("9200").
			WithStartupTimeout(2 * time.Minute),
	}
	containerRequest7_16_3 = testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    filepath.Join("testdata", "integration"),
			Dockerfile: "Dockerfile.elasticsearch.7_16_3",
		},
		ExposedPorts: []string{"9300:9200"},
		WaitingFor: wait.ForListeningPort("9200").
			WithStartupTimeout(2 * time.Minute),
	}
)

func TestElasticsearchIntegration(t *testing.T) {
	// Starts an elasticsearch docker container
	t.Run("Running elasticsearch 7.0.0", func(t *testing.T) {
		t.Skip("Skipping as the test fails intermittently, see https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/19755")
		t.Parallel()
		container := getContainer(t, containerRequest7_0_0)
		defer func() {
			require.NoError(t, container.Terminate(context.Background()))
		}()
		hostname, err := container.Host(context.Background())
		require.NoError(t, err)

		f := NewFactory()
		cfg := f.CreateDefaultConfig().(*Config)
		cfg.Endpoint = fmt.Sprintf("http://%s:9600", hostname)

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

		expectedFile := filepath.Join("testdata", "integration", "expected.7_0_0.yaml")
		expectedMetrics, err := golden.ReadMetrics(expectedFile)
		require.NoError(t, err)

		require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, actualMetrics,
			pmetrictest.IgnoreResourceAttributeValue("elasticsearch.node.name"),
			pmetrictest.IgnoreTimestamp(),
			pmetrictest.IgnoreStartTimestamp(),
			pmetrictest.IgnoreMetricValues(),
			pmetrictest.IgnoreMetricDataPointsOrder(),
			pmetrictest.IgnoreScopeMetricsOrder(),
			pmetrictest.IgnoreResourceMetricsOrder(),
		))
	})
	t.Run("Running elasticsearch 7.9.3", func(t *testing.T) {
		t.Skip("Skipping as the test fails intermittently, see https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/19755")
		t.Parallel()
		container := getContainer(t, containerRequest7_9_3)
		defer func() {
			require.NoError(t, container.Terminate(context.Background()))
		}()
		hostname, err := container.Host(context.Background())
		require.NoError(t, err)

		f := NewFactory()
		cfg := f.CreateDefaultConfig().(*Config)
		cfg.Endpoint = fmt.Sprintf("http://%s:9200", hostname)

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

		expectedFile := filepath.Join("testdata", "integration", "expected.7_9_3.yaml")
		expectedMetrics, err := golden.ReadMetrics(expectedFile)
		require.NoError(t, err)

		require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, actualMetrics,
			pmetrictest.IgnoreResourceAttributeValue("elasticsearch.node.name"),
			pmetrictest.IgnoreTimestamp(),
			pmetrictest.IgnoreStartTimestamp(),
			pmetrictest.IgnoreMetricValues(),
			pmetrictest.IgnoreMetricDataPointsOrder(),
			pmetrictest.IgnoreScopeMetricsOrder(),
			pmetrictest.IgnoreResourceMetricsOrder(),
		))
	})
	t.Run("Running elasticsearch 7.16.3", func(t *testing.T) {
		t.Skip("Skipping as the test fails intermittently, see https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/19755")
		t.Parallel()
		container := getContainer(t, containerRequest7_16_3)
		defer func() {
			require.NoError(t, container.Terminate(context.Background()))
		}()
		hostname, err := container.Host(context.Background())
		require.NoError(t, err)

		f := NewFactory()
		cfg := f.CreateDefaultConfig().(*Config)
		cfg.Endpoint = fmt.Sprintf("http://%s:9300", hostname)

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

		expectedFile := filepath.Join("testdata", "integration", "expected.7_16_3.yaml")
		expectedMetrics, err := golden.ReadMetrics(expectedFile)
		require.NoError(t, err)

		require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, actualMetrics,
			pmetrictest.IgnoreResourceAttributeValue("elasticsearch.node.name"),
			pmetrictest.IgnoreTimestamp(),
			pmetrictest.IgnoreStartTimestamp(),
			pmetrictest.IgnoreMetricValues(),
			pmetrictest.IgnoreMetricDataPointsOrder(),
			pmetrictest.IgnoreScopeMetricsOrder(),
			pmetrictest.IgnoreResourceMetricsOrder(),
		))
	})
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

	err = container.Start(context.Background())
	require.NoError(t, err)
	return container
}
