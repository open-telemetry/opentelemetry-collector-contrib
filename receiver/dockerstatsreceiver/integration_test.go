// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package dockerstatsreceiver

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	rcvr "go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receivertest"
	conventions "go.opentelemetry.io/otel/semconv/v1.27.0"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/dockerstatsreceiver/internal/metadata"
)

func factory() (rcvr.Factory, *Config) {
	f := NewFactory()
	config := f.CreateDefaultConfig().(*Config)
	config.CollectionInterval = 1 * time.Second
	return f, config
}

func paramsAndContext(t *testing.T) (rcvr.Settings, context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	logger := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller()))
	settings := receivertest.NewNopSettings(metadata.Type)
	settings.Logger = logger
	return settings, ctx, cancel
}

func createNginxContainer(ctx context.Context, t *testing.T) testcontainers.Container {
	req := testcontainers.ContainerRequest{
		Image:        "docker.io/library/nginx:1.17",
		ExposedPorts: []string{"80/tcp"},
		WaitingFor:   wait.ForExposedPort(),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)
	require.NotNil(t, container)

	return container
}

func hasResourceScopeMetrics(containerID string, metrics []pmetric.Metrics) bool {
	for _, m := range metrics {
		for i := 0; i < m.ResourceMetrics().Len(); i++ {
			rm := m.ResourceMetrics().At(i)

			id, ok := rm.Resource().Attributes().Get(string(conventions.ContainerIDKey))
			if ok && id.AsString() == containerID && rm.ScopeMetrics().Len() > 0 {
				return true
			}
		}
	}
	return false
}

func TestDefaultMetricsIntegration(t *testing.T) {
	t.Parallel()
	params, ctx, cancel := paramsAndContext(t)
	defer cancel()

	container := createNginxContainer(ctx, t)

	consumer := new(consumertest.MetricsSink)
	f, config := factory()
	recv, err := f.CreateMetrics(ctx, params, config, consumer)

	require.NoError(t, err, "failed creating metrics receiver")
	require.NoError(t, recv.Start(ctx, &nopHost{
		reportFunc: func(event *componentstatus.Event) {
			require.NoError(t, event.Err())
		},
	}))

	assert.Eventuallyf(t, func() bool {
		return hasResourceScopeMetrics(container.GetContainerID(), consumer.AllMetrics())
	}, 5*time.Second, 1*time.Second, "failed to receive any metrics")

	assert.NoError(t, recv.Shutdown(ctx))
}

func TestMonitoringAddedAndRemovedContainerIntegration(t *testing.T) {
	t.Parallel()
	params, ctx, cancel := paramsAndContext(t)
	defer cancel()
	consumer := new(consumertest.MetricsSink)
	f, config := factory()

	recv, err := f.CreateMetrics(ctx, params, config, consumer)
	require.NoError(t, err, "failed creating metrics receiver")
	require.NoError(t, recv.Start(ctx, &nopHost{
		reportFunc: func(event *componentstatus.Event) {
			require.NoError(t, event.Err())
		},
	}))

	container := createNginxContainer(ctx, t)

	// Check metrics are collected for added container.
	assert.Eventuallyf(t, func() bool {
		return hasResourceScopeMetrics(container.GetContainerID(), consumer.AllMetrics())
	}, 5*time.Second, 1*time.Second, "failed to receive any metrics")

	require.NoError(t, container.Terminate(ctx))
	consumer.Reset()

	// Check metrics are not collected for removed container.
	assert.Never(t, func() bool {
		return hasResourceScopeMetrics(container.GetContainerID(), consumer.AllMetrics())
	}, 5*time.Second, 1*time.Second, "received undesired metrics")

	assert.NoError(t, recv.Shutdown(ctx))
}

func TestExcludedImageProducesNoMetricsIntegration(t *testing.T) {
	t.Parallel()
	params, ctx, cancel := paramsAndContext(t)
	defer cancel()

	container := createNginxContainer(ctx, t)

	f, config := factory()
	config.ExcludedImages = append(config.ExcludedImages, "*nginx*")

	consumer := new(consumertest.MetricsSink)
	recv, err := f.CreateMetrics(ctx, params, config, consumer)
	require.NoError(t, err, "failed creating metrics receiver")
	require.NoError(t, recv.Start(ctx, &nopHost{
		reportFunc: func(event *componentstatus.Event) {
			require.NoError(t, event.Err())
		},
	}))

	assert.Never(t, func() bool {
		return hasResourceScopeMetrics(container.GetContainerID(), consumer.AllMetrics())
	}, 5*time.Second, 1*time.Second, "received undesired metrics")

	assert.NoError(t, recv.Shutdown(ctx))
}

var _ componentstatus.Reporter = (*nopHost)(nil)

type nopHost struct {
	reportFunc func(event *componentstatus.Event)
}

func (nh *nopHost) GetExtensions() map[component.ID]component.Component {
	return nil
}

func (nh *nopHost) Report(event *componentstatus.Event) {
	nh.reportFunc(event)
}
