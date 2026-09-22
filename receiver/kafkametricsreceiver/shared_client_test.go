// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkametricsreceiver

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kfake"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/kafka/kafkatest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkametricsreceiver/internal/metadata"
)

// TestScrapers_ShareSingleAdminClient verifies that scrapers constructed with a
// shared provider all use the same admin client, closed only after the last one.
func TestScrapers_ShareSingleAdminClient(t *testing.T) {
	_, clientCfg := kafkatest.NewCluster(t, kfake.SeedTopics(1, "meta-topic"))

	cfg := Config{
		ClientConfig:         clientCfg,
		MetricsBuilderConfig: metadata.NewDefaultMetricsBuilderConfig(),
		TopicMatch:           ".*",
		GroupMatch:           ".*",
	}

	clients := newFranzAdminProvider(cfg.ClientConfig, zap.NewNop())

	settings := receivertest.NewNopSettings(metadata.Type)
	broker := &brokerScraperFranz{settings: settings, config: cfg, clients: clients}
	topic := &topicScraperFranz{settings: settings, config: cfg, clients: clients}
	consumer := &consumerScraperFranz{settings: settings, config: cfg, clients: clients}

	host := componenttest.NewNopHost()
	require.NoError(t, broker.start(t.Context(), host))
	require.NoError(t, topic.start(t.Context(), host))
	require.NoError(t, consumer.start(t.Context(), host))

	// Each scraper resolves the admin client; all must be the same instance.
	admBroker, err := broker.clients.admin(t.Context(), host)
	require.NoError(t, err)
	admTopic, err := topic.clients.admin(t.Context(), host)
	require.NoError(t, err)
	admConsumer, err := consumer.clients.admin(t.Context(), host)
	require.NoError(t, err)

	require.Same(t, admBroker, admTopic)
	require.Same(t, admBroker, admConsumer)

	// Shutting down two scrapers must not close the shared client.
	require.NoError(t, broker.shutdown(t.Context()))
	require.NoError(t, topic.shutdown(t.Context()))
	clients.mu.Lock()
	stillOpen := clients.adm != nil
	clients.mu.Unlock()
	require.True(t, stillOpen, "shared client closed before all scrapers shut down")

	// Shutting down the last scraper closes the shared client.
	require.NoError(t, consumer.shutdown(t.Context()))
	clients.mu.Lock()
	closed := clients.adm == nil
	clients.mu.Unlock()
	require.True(t, closed, "shared client not closed after all scrapers shut down")
}
