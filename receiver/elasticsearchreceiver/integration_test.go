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
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

var (
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

func SimpleAPITest(t *testing.T, baseURL string) {
	client, err := newElasticsearchClient(componenttest.NewNopTelemetrySettings(), Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{Endpoint: baseURL},
	}, componenttest.NewNopHost())
	require.NoError(t, err)
	ctx := context.Background()

	_, err = client.Nodes(ctx, []string{})
	require.NoError(t, err, "Failed to get Nodes metrics from elasticsearch API")
	_, err = client.NodeStats(ctx, []string{})
	require.NoError(t, err, "Failed to get NodeStats metrics from elasticsearch API")
	_, err = client.ClusterHealth(ctx)
	require.NoError(t, err, "Failed to get ClusterHealth metrics from elasticsearch API")
	_, err = client.IndexStats(ctx, []string{})
	require.NoError(t, err, "Failed to get IndexStats metrics from elasticsearch API")
	_, err = client.ClusterMetadata(ctx)
	require.NoError(t, err, "Failed to get ClusterMetadata metrics from elasticsearch API")
	_, err = client.ClusterStats(ctx, []string{"_all"})
	require.NoError(t, err, "Failed to get ClusterStats metrics from elasticsearch API")
}

func TestElasticsearchIntegration(t *testing.T) {
	// Let this test check if it works with the features disabled and the unit test will test the feature enabled.
	require.NoError(t, featuregate.GlobalRegistry().Set(emitNodeVersionAttr.ID(), false))

	// Starts an elasticsearch docker container
	t.Run("Running elasticsearch 7.9", func(t *testing.T) {
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

		SimpleAPITest(t, cfg.Endpoint)
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

		expectedFile := filepath.Join("testdata", "integration", "expected.7_9_3.json")
		expectedMetrics, err := golden.ReadMetrics(expectedFile)
		require.NoError(t, err)

		require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, actualMetrics, //nolint:errcheck
			pmetrictest.IgnoreResourceMetricsOrder(),
			pmetrictest.IgnoreMetricValues(),
			pmetrictest.IgnoreResourceAttributeValue("elasticsearch.node.name")))
	})
	t.Run("Running elasticsearch 7.16.3", func(t *testing.T) {
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

		expectedFile := filepath.Join("testdata", "integration", "expected.7_16_3.json")
		expectedMetrics, err := golden.ReadMetrics(expectedFile)
		require.NoError(t, err)

		require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, actualMetrics, //nolint:errcheck
			pmetrictest.IgnoreResourceMetricsOrder(),
			pmetrictest.IgnoreMetricValues(),
			pmetrictest.IgnoreResourceAttributeValue("elasticsearch.node.name")))
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
