// Copyright The OpenTelemetry Authors
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

//go:build integration
// +build integration

package dockerstatsreceiver

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	testcontainers "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	rcvr "go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receivertest"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/docker"
)

type testHost struct {
	component.Host
	t *testing.T
}

// ReportFatalError causes the test to be run to fail.
func (h *testHost) ReportFatalError(err error) {
	h.t.Fatalf("receiver reported a fatal error: %v", err)
}

var _ component.Host = (*testHost)(nil)

func factory() (rcvr.Factory, *Config) {
	f := NewFactory()
	config := f.CreateDefaultConfig().(*Config)
	config.CollectionInterval = 1 * time.Second
	return f, config
}

func paramsAndContext(t *testing.T) (rcvr.CreateSettings, context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	logger := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller()))
	settings := receivertest.NewNopCreateSettings()
	settings.Logger = logger
	return settings, ctx, cancel
}

func TestDefaultMetricsIntegration(t *testing.T) {
	params, ctx, cancel := paramsAndContext(t)
	defer cancel()

	req := testcontainers.ContainerRequest{
		Image:        "docker.io/library/nginx:1.17",
		ExposedPorts: []string{"80/tcp"},
		WaitingFor:   wait.ForListeningPort("80/tcp"),
	}
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.Nil(t, err)
	require.NotNil(t, container)

	consumer := new(consumertest.MetricsSink)
	f, config := factory()
	recv, err := f.CreateMetricsReceiver(ctx, params, config, consumer)

	require.NoError(t, err, "failed creating metrics receiver")
	require.NoError(t, recv.Start(ctx, &testHost{
		t: t,
	}))

	assert.Eventuallyf(t, func() bool {
		return len(consumer.AllMetrics()) > 0
	}, 5*time.Second, 1*time.Second, "failed to receive any metrics")

	assert.NoError(t, recv.Shutdown(ctx))
}

func TestAllMetricsIntegration(t *testing.T) {
	req := testcontainers.ContainerRequest{
		Image:        "docker.io/library/nginx:1.17",
		ExposedPorts: []string{"80/tcp"},
		WaitingFor:   wait.ForListeningPort("80/tcp"),
	}
	container, err := testcontainers.GenericContainer(context.Background(), testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.Nil(t, err)
	require.NotNil(t, container)

	consumer := new(consumertest.MetricsSink)
	f, config := factory()

	params, ctx, cancel := paramsAndContext(t)
	defer cancel()

	recv, err := f.CreateMetricsReceiver(ctx, params, config, consumer)
	require.NoError(t, err, "failed creating metrics receiver")
	require.NoError(t, recv.Start(ctx, &testHost{
		t: t,
	}))

	assert.Eventuallyf(t, func() bool {
		return len(consumer.AllMetrics()) > 0
	}, 5*time.Second, 1*time.Second, "failed to receive any metrics")

	assert.NoError(t, recv.Shutdown(ctx))
}

func TestMonitoringAddedContainerIntegration(t *testing.T) {
	params, ctx, cancel := paramsAndContext(t)
	defer cancel()
	consumer := new(consumertest.MetricsSink)
	f, config := factory()

	recv, err := f.CreateMetricsReceiver(ctx, params, config, consumer)
	require.NoError(t, err, "failed creating metrics receiver")
	require.NoError(t, recv.Start(ctx, &testHost{
		t: t,
	}))

	req := testcontainers.ContainerRequest{
		Image:        "docker.io/library/nginx:1.17",
		ExposedPorts: []string{"80/tcp"},
		WaitingFor:   wait.ForListeningPort("80/tcp"),
	}
	container, err := testcontainers.GenericContainer(context.Background(), testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.Nil(t, err)
	require.NotNil(t, container)

	assert.Eventuallyf(t, func() bool {
		return len(consumer.AllMetrics()) > 0
	}, 5*time.Second, 1*time.Second, "failed to receive any metrics")

	assert.NoError(t, recv.Shutdown(ctx))
}

func TestExcludedImageProducesNoMetricsIntegration(t *testing.T) {
	params, ctx, cancel := paramsAndContext(t)
	defer cancel()

	req := testcontainers.ContainerRequest{
		Image:        "docker.io/library/redis:6.0.3",
		ExposedPorts: []string{"6379/tcp"},
		WaitingFor:   wait.ForListeningPort("6379/tcp"),
	}
	container, err := testcontainers.GenericContainer(context.Background(), testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.Nil(t, err)
	require.NotNil(t, container)

	f, config := factory()
	config.ExcludedImages = append(config.ExcludedImages, "*redis*")

	consumer := new(consumertest.MetricsSink)
	recv, err := f.CreateMetricsReceiver(ctx, params, config, consumer)
	require.NoError(t, err, "failed creating metrics receiver")
	require.NoError(t, recv.Start(ctx, &testHost{
		t: t,
	}))

	assert.Never(t, func() bool {
		if metrics := consumer.AllMetrics(); len(metrics) > 0 {
			for _, metric := range metrics {
				resourceMetrics := metric.ResourceMetrics()
				for i := 0; i < resourceMetrics.Len(); i++ {
					resourceMetric := resourceMetrics.At(i)
					resource := resourceMetric.Resource()
					if nameAttr, ok := resource.Attributes().Get(conventions.AttributeContainerImageName); ok {
						if strings.Contains(nameAttr.Str(), "redis") {
							return true
						}
					}
				}
			}
		}
		return false
	}, 5*time.Second, 1*time.Second, "received undesired metrics")

	assert.NoError(t, recv.Shutdown(ctx))
}

func TestRemovedContainerRemovesRecordsIntegration(t *testing.T) {
	_, config := factory()
	config.ExcludedImages = append(config.ExcludedImages, "!*nginx*")

	dConfig, err := docker.NewConfig(config.Endpoint, config.Timeout, config.ExcludedImages, config.DockerAPIVersion)
	require.NoError(t, err)

	client, err := docker.NewDockerClient(dConfig, zap.NewNop())
	require.NoError(t, err)
	require.NoError(t, client.LoadContainerList(context.Background()))
	go client.ContainerEventLoop(context.Background())

	initialCount := len(client.Containers())

	ctx := context.Background()
	req := testcontainers.ContainerRequest{
		Image:        "docker.io/library/nginx:1.17",
		ExposedPorts: []string{"80/tcp"},
		WaitingFor:   wait.ForListeningPort("80/tcp"),
		SkipReaper:   true,
	}
	nginx, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.Nil(t, err)
	require.NotNil(t, nginx)

	t.Log(nginx.GetContainerID())
	desiredAmount := func(numDesired int) func() bool {
		return func() bool {
			return len(client.Containers()) == numDesired
		}
	}

	require.Eventuallyf(t, desiredAmount(initialCount+1), 5*time.Second, 1*time.Millisecond, "failed to load container stores")

	err = nginx.Terminate(ctx)
	require.Nil(t, err)

	require.Eventuallyf(t, desiredAmount(initialCount), 5*time.Second, 1*time.Millisecond, "failed to clear container stores")

	// Confirm missing container paths
	dc := docker.Container{
		ContainerJSON: &types.ContainerJSON{
			ContainerJSONBase: &types.ContainerJSONBase{
				ID: nginx.GetContainerID(),
			},
		},
	}
	statsJSON, err := client.FetchContainerStatsAsJSON(context.Background(), dc)
	assert.Nil(t, statsJSON)
	require.Error(t, err)
	assert.Equal(t, fmt.Sprintf("Error response from daemon: No such container: %s", nginx.GetContainerID()), err.Error())
}
