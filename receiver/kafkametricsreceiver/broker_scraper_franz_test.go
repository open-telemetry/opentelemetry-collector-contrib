// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkametricsreceiver

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kfake"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/collector/scraper"
	"go.opentelemetry.io/collector/scraper/scrapererror"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/kafka/kafkatest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkametricsreceiver/internal/metadata"
)

// helper to make a minimal franz config for tests
func franzTestConfig(t *testing.T) Config {
	t.Helper()
	_, clientCfg := kafkatest.NewCluster(t, kfake.SeedTopics(1, "meta-topic"))
	cfg := Config{
		ClientConfig:         clientCfg,
		MetricsBuilderConfig: metadata.NewDefaultMetricsBuilderConfig(),
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

func TestBrokerScraperFranz_ScrapeMetricValues(t *testing.T) {
	setFranzGo(t, true)

	const numBrokers = 3
	const logRetentionHoursValue = 168
	_, clientCfg := kafkatest.NewCluster(t,
		kfake.SeedTopics(1, "meta-topic"),
		kfake.NumBrokers(numBrokers),
		kfake.BrokerConfigs(map[string]string{
			logRetentionHours: "168",
		}),
	)
	cfg := Config{
		ClientConfig:         clientCfg,
		MetricsBuilderConfig: metadata.NewDefaultMetricsBuilderConfig(),
		ClusterAlias:         "test-cluster",
	}
	cfg.ResourceAttributes.KafkaClusterAlias.Enabled = true
	cfg.Metrics.KafkaBrokerLogRetentionPeriod.Enabled = true

	s, err := createBrokerScraperFranz(t.Context(), cfg, receivertest.NewNopSettings(metadata.Type))
	require.NoError(t, err)
	require.NoError(t, s.Start(t.Context(), componenttest.NewNopHost()))
	t.Cleanup(func() { require.NoError(t, s.Shutdown(t.Context())) })

	md, err := s.ScrapeMetrics(t.Context())
	require.NoError(t, err)
	require.Equal(t, 1, md.ResourceMetrics().Len())

	rm := md.ResourceMetrics().At(0)
	val, ok := rm.Resource().Attributes().Get("kafka.cluster.alias")
	require.True(t, ok)
	require.Equal(t, "test-cluster", val.Str())

	ms := rm.ScopeMetrics().At(0).Metrics()
	var sawBrokers, sawRetention bool
	for i := 0; i < ms.Len(); i++ {
		m := ms.At(i)
		switch m.Name() {
		case "kafka.brokers":
			require.Equal(t, pmetric.MetricTypeSum, m.Type())
			require.Equal(t, int64(numBrokers), m.Sum().DataPoints().At(0).IntValue())
			sawBrokers = true
		case "kafka.broker.log_retention_period":
			require.Equal(t, pmetric.MetricTypeGauge, m.Type())
			dps := m.Gauge().DataPoints()
			require.Equal(t, numBrokers, dps.Len())
			for j := 0; j < dps.Len(); j++ {
				require.Equal(t, int64(logRetentionHoursValue*3600), dps.At(j).IntValue())
			}
			sawRetention = true
		}
	}
	require.True(t, sawBrokers, "kafka.brokers not emitted")
	require.True(t, sawRetention, "kafka.broker.log_retention_period not emitted")
}

func TestBrokerScraperFranz_ScrapePartialError_UnparseableRetention(t *testing.T) {
	setFranzGo(t, true)

	const numBrokers = 2
	_, clientCfg := kafkatest.NewCluster(t,
		kfake.SeedTopics(1, "meta-topic"),
		kfake.NumBrokers(numBrokers),
		kfake.BrokerConfigs(map[string]string{
			logRetentionHours: "not-a-number",
		}),
	)
	cfg := Config{
		ClientConfig:         clientCfg,
		MetricsBuilderConfig: metadata.NewDefaultMetricsBuilderConfig(),
	}
	cfg.Metrics.KafkaBrokerLogRetentionPeriod.Enabled = true

	s, err := createBrokerScraperFranz(t.Context(), cfg, receivertest.NewNopSettings(metadata.Type))
	require.NoError(t, err)
	require.NoError(t, s.Start(t.Context(), componenttest.NewNopHost()))
	t.Cleanup(func() { require.NoError(t, s.Shutdown(t.Context())) })

	md, err := s.ScrapeMetrics(t.Context())
	// Per-broker parse failure should produce a partial scrape error, not a hard error.
	require.Error(t, err)
	var partial scrapererror.PartialScrapeError
	require.ErrorAs(t, err, &partial, "expected PartialScrapeError, got %T: %v", err, err)
	require.Equal(t, numBrokers, partial.Failed)

	ms := md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
	var sawBrokers, sawRetention bool
	for i := 0; i < ms.Len(); i++ {
		switch ms.At(i).Name() {
		case "kafka.brokers":
			sawBrokers = true
		case "kafka.broker.log_retention_period":
			sawRetention = true
		}
	}
	require.True(t, sawBrokers)
	require.False(t, sawRetention, "log retention metric should be skipped on parse failure")
}

func TestBrokerScraperFranz_ScrapeUnreachable(t *testing.T) {
	setFranzGo(t, true)

	cluster, clientCfg := kafkatest.NewCluster(t, kfake.SeedTopics(1, "meta-topic"))
	cfg := Config{
		ClientConfig:         clientCfg,
		MetricsBuilderConfig: metadata.NewDefaultMetricsBuilderConfig(),
	}

	s, err := createBrokerScraperFranz(t.Context(), cfg, receivertest.NewNopSettings(metadata.Type))
	require.NoError(t, err)
	require.NoError(t, s.Start(t.Context(), componenttest.NewNopHost()))
	t.Cleanup(func() { require.NoError(t, s.Shutdown(t.Context())) })

	cluster.Close() // simulate unreachable cluster

	ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
	defer cancel()
	_, err = s.ScrapeMetrics(ctx)
	require.Error(t, err)
}

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
