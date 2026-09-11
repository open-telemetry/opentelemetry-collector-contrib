//go:build live

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package rabbitmqreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/rabbitmqreceiver"

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/rabbitmqreceiver/internal/metadata"
)

// TestLiveGetClusterName exercises GetClusterName against a real, running
// RabbitMQ broker (management plugin) at localhost:15672 with guest/guest.
func TestLiveGetClusterName(t *testing.T) {
	clientConfig := confighttp.NewDefaultClientConfig()
	clientConfig.Endpoint = "http://localhost:15672"

	cfg := &Config{
		ClientConfig: clientConfig,
		Username:     "guest",
		Password:     "guest",
	}

	c, err := newClient(t.Context(), cfg, componenttest.NewNopHost(), componenttest.NewNopTelemetrySettings(), zap.NewNop())
	require.NoError(t, err)
	t.Cleanup(func() { c.(*rabbitmqClient).client.CloseIdleConnections() })

	name, err := c.GetClusterName(t.Context())
	require.NoError(t, err)
	require.NotEmpty(t, name)
	t.Logf("live cluster name: %q", name)
}

// TestLiveScrapeSetsClusterNameResourceAttribute runs a full scrape against a
// real, running RabbitMQ broker and confirms rabbitmq.cluster.name is set on
// every resulting resource when the attribute is enabled.
func TestLiveScrapeSetsClusterNameResourceAttribute(t *testing.T) {
	clientConfig := confighttp.NewDefaultClientConfig()
	clientConfig.Endpoint = "http://localhost:15672"

	cfg := createDefaultConfig().(*Config)
	cfg.ClientConfig = clientConfig
	cfg.Username = "guest"
	cfg.Password = "guest"
	cfg.MetricsBuilderConfig.ResourceAttributes.RabbitmqClusterName.Enabled = true

	scraper := newScraper(zap.NewNop(), cfg, receivertest.NewNopSettings(metadata.Type))
	err := scraper.start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)

	metrics, err := scraper.scrape(t.Context())
	require.NoError(t, err)
	require.Positive(t, metrics.ResourceMetrics().Len(), "expected at least one resource (queue/node/exchange) from the live broker")

	for i := 0; i < metrics.ResourceMetrics().Len(); i++ {
		val, ok := metrics.ResourceMetrics().At(i).Resource().Attributes().Get("rabbitmq.cluster.name")
		require.True(t, ok, "resource %d missing rabbitmq.cluster.name", i)
		require.NotEmpty(t, val.Str())
		t.Logf("resource %d rabbitmq.cluster.name=%q", i, val.Str())
	}
}
