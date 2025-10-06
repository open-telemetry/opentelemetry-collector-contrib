// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkametricsreceiver

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kfake"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/collector/scraper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/kafka/kafkatest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkametricsreceiver/internal/metadata"
)

// helper to make a minimal franz config for tests
func franzTestConfig(t *testing.T) Config {
	t.Helper()
	_, clientCfg := kafkatest.NewCluster(t, kfake.SeedTopics(1, "meta-topic"))
	cfg := Config{
		ClientConfig:         clientCfg,
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
		ClusterAlias:         "test-cluster",
	}
	// keep retention metric disabled here (kfake does not expose broker config values)
	cfg.Metrics.KafkaBrokerLogRetentionPeriod.Enabled = false
	return cfg
}

func TestBrokerScraperFranz_CreateStartScrapeShutdown(t *testing.T) {
	setFranzGo(t, true)

	cfg := franzTestConfig(t)

	// create franz-backed scraper directly (unit test)
	var s scraper.Metrics
	var err error

	s, err = createBrokerScraperFranz(t.Context(), cfg, receivertest.NewNopSettings(metadata.Type))
	require.NoError(t, err)
	require.NotNil(t, s)

	// Start
	require.NoError(t, s.Start(t.Context(), componenttest.NewNopHost()))

	// Scrape
	md, err := s.ScrapeMetrics(t.Context())
	require.NoError(t, err)

	// Validate resource + at least the brokers count metric exists
	require.Equal(t, 1, md.ResourceMetrics().Len())
	rm := md.ResourceMetrics().At(0)
	sm := rm.ScopeMetrics()
	require.Equal(t, 1, sm.Len())
	ms := sm.At(0).Metrics()

	var foundBrokers bool
	for i := 0; i < ms.Len(); i++ {
		m := ms.At(i)
		if m.Name() == "kafka.brokers" {
			foundBrokers = true
			// With one kfake cluster, we should have >=1 broker; exact count comes from kfake
			require.GreaterOrEqual(t, m.Sum().DataPoints().At(0).IntValue(), int64(1))
			break
		}
	}
	require.True(t, foundBrokers, "expected kafka.brokers metric")

	// Shutdown
	require.NoError(t, s.Shutdown(t.Context()))
}

func TestBrokerScraperFranz_EmptyClusterAlias(t *testing.T) {
	setFranzGo(t, true)

	cfg := franzTestConfig(t)
	cfg.ClusterAlias = "" // ensure empty alias behaves like Sarama test

	s, err := createBrokerScraperFranz(t.Context(), cfg, receivertest.NewNopSettings(metadata.Type))
	require.NoError(t, err)
	require.NotNil(t, s)

	require.NoError(t, s.Start(t.Context(), componenttest.NewNopHost()))
	md, err := s.ScrapeMetrics(t.Context())
	require.NoError(t, err)

	require.Equal(t, 1, md.ResourceMetrics().Len())
	rm := md.ResourceMetrics().At(0)

	// attribute should be absent when alias is empty
	_, ok := rm.Resource().Attributes().Get("kafka.cluster.alias")
	require.False(t, ok)

	require.NoError(t, s.Shutdown(t.Context()))
}

func TestBrokerScraperFranz_Create(t *testing.T) {
	setFranzGo(t, true)

	cfg := franzTestConfig(t)
	s, err := createBrokerScraperFranz(t.Context(), cfg, receivertest.NewNopSettings(metadata.Type))
	require.NoError(t, err)
	require.NotNil(t, s)
}

func TestBrokerScraperFranz_Start(t *testing.T) {
	setFranzGo(t, true)

	cfg := franzTestConfig(t)
	s, err := createBrokerScraperFranz(t.Context(), cfg, receivertest.NewNopSettings(metadata.Type))
	require.NoError(t, err)
	require.NotNil(t, s)

	require.NoError(t, s.Start(t.Context(), componenttest.NewNopHost()))
	require.NoError(t, s.Shutdown(t.Context()))
}

// func TestBrokerScraperFranz_ScrapeHandlesClientError(t *testing.T) {
// 	setFranzGo(t, true)

// 	// stub the ctor to fail
// 	orig := newFranzAdminClient
// 	t.Cleanup(func() { newFranzAdminClient = orig })
// 	newFranzAdminClient = func(ctx context.Context, cfg configkafka.ClientConfig, lg *zap.Logger, opts ...kgo.Opt) (*kadm.Client, *kgo.Client, error) {
// 		return nil, nil, errors.New("new franz admin failed")
// 	}

// 	cfg := franzTestConfig(t)
// 	s, err := createBrokerScraperFranz(t.Context(), cfg, receivertest.NewNopSettings(metadata.Type))
// 	require.NoError(t, err)

// 	_, err = s.ScrapeMetrics(t.Context())
// 	require.Error(t, err)
// }

func TestBrokerScraperFranz_ShutdownWithoutStart_OK(t *testing.T) {
	setFranzGo(t, true)

	// create a legit cfg but never start
	_, _ = kafkatest.NewCluster(t, kfake.SeedTopics(1, "meta-topic"))
	cfg := franzTestConfig(t)

	s, err := createBrokerScraperFranz(t.Context(), cfg, receivertest.NewNopSettings(metadata.Type))
	require.NoError(t, err)
	require.NotNil(t, s)

	// should not panic / should return nil
	require.NoError(t, s.Shutdown(t.Context()))
}
