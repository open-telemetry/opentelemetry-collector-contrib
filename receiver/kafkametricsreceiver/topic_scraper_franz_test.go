// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkametricsreceiver

import (
	"context"
	"regexp"
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

// minimal franz test config for topics
func franzTopicsTestConfig(t *testing.T) Config {
	t.Helper()
	_, clientCfg := kafkatest.NewCluster(t, kfake.SeedTopics(1, "topic-a"))
	cfg := Config{
		ClientConfig:         clientCfg,
		MetricsBuilderConfig: metadata.NewDefaultMetricsBuilderConfig(),
		ClusterAlias:         "franz-topics",
		TopicMatch:           ".*", // allow our seeded topic
	}
	return cfg
}

func TestTopicScraperFranz_CreateStartScrapeShutdown(t *testing.T) {
	setFranzGo(t, true)

	cfg := franzTopicsTestConfig(t)

	var s scraper.Metrics
	var err error

	s, err = createTopicsScraperFranz(t.Context(), cfg, receivertest.NewNopSettings(metadata.Type))
	require.NoError(t, err)
	require.NotNil(t, s)

	require.NoError(t, s.Start(t.Context(), componenttest.NewNopHost()))

	md, err := s.ScrapeMetrics(t.Context())
	require.NoError(t, err)
	require.NotNil(t, md)

	// With kfake, depending on metadata & offsets availability, data points may be zero.
	// The key here is: scrape does not error.
	_ = md // intentionally not asserting counts

	require.NoError(t, s.Shutdown(t.Context()))
}

func TestTopicScraperFranz_InvalidTopicRegex(t *testing.T) {
	setFranzGo(t, true)

	cfg := franzTopicsTestConfig(t)
	cfg.TopicMatch = "[" // invalid

	s, err := createTopicsScraperFranz(t.Context(), cfg, receivertest.NewNopSettings(metadata.Type))
	require.Error(t, err)
	require.Nil(t, s)
}

func TestTopicScraperFranz_EmptyClusterAlias(t *testing.T) {
	setFranzGo(t, true)

	cfg := franzTopicsTestConfig(t)
	cfg.ClusterAlias = ""

	s, err := createTopicsScraperFranz(t.Context(), cfg, receivertest.NewNopSettings(metadata.Type))
	require.NoError(t, err)
	require.NotNil(t, s)

	require.NoError(t, s.Start(t.Context(), componenttest.NewNopHost()))
	md, err := s.ScrapeMetrics(t.Context())
	require.NoError(t, err)

	// Only assert alias absence if a resource entry exists.
	if md.ResourceMetrics().Len() > 0 {
		rm := md.ResourceMetrics().At(0)
		_, ok := rm.Resource().Attributes().Get("kafka.cluster.alias")
		require.False(t, ok)
	}

	require.NoError(t, s.Shutdown(t.Context()))
}

func TestTopicScraperFranz_ScrapeMetricValues(t *testing.T) {
	setFranzGo(t, true)

	const (
		topic         = "topic-a"
		numPartitions = 3
		numBrokers    = 2
	)
	_, clientCfg := kafkatest.NewCluster(t,
		kfake.SeedTopics(numPartitions, topic),
		kfake.NumBrokers(numBrokers),
	)
	cfg := Config{
		ClientConfig:         clientCfg,
		MetricsBuilderConfig: metadata.NewDefaultMetricsBuilderConfig(),
		ClusterAlias:         "franz-topics",
		TopicMatch:           ".*",
	}
	cfg.ResourceAttributes.KafkaClusterAlias.Enabled = true
	cfg.Metrics.KafkaTopicLogRetentionPeriod.Enabled = true
	cfg.Metrics.KafkaTopicLogRetentionSize.Enabled = true
	cfg.Metrics.KafkaTopicMinInsyncReplicas.Enabled = true
	cfg.Metrics.KafkaTopicReplicationFactor.Enabled = true

	s, err := createTopicsScraperFranz(t.Context(), cfg, receivertest.NewNopSettings(metadata.Type))
	require.NoError(t, err)
	require.NoError(t, s.Start(t.Context(), componenttest.NewNopHost()))
	t.Cleanup(func() { require.NoError(t, s.Shutdown(t.Context())) })

	md, err := s.ScrapeMetrics(t.Context())
	require.NoError(t, err)
	require.Equal(t, 1, md.ResourceMetrics().Len())

	rm := md.ResourceMetrics().At(0)
	val, ok := rm.Resource().Attributes().Get("kafka.cluster.alias")
	require.True(t, ok)
	require.Equal(t, "franz-topics", val.Str())

	// kfake topic config defaults
	const (
		expectedRetentionMs    = 604800000 // 7 days
		expectedRetentionBytes = -1
		expectedMinInsync      = 1
	)

	ms := rm.ScopeMetrics().At(0).Metrics()
	seen := map[string]bool{}
	for i := 0; i < ms.Len(); i++ {
		m := ms.At(i)
		seen[m.Name()] = true
		switch m.Name() {
		case "kafka.topic.partitions":
			require.Equal(t, pmetric.MetricTypeSum, m.Type())
			dps := m.Sum().DataPoints()
			require.Equal(t, 1, dps.Len())
			require.Equal(t, int64(numPartitions), dps.At(0).IntValue())
		case "kafka.topic.replication_factor":
			require.Equal(t, int64(numBrokers), m.Gauge().DataPoints().At(0).IntValue())
		case "kafka.topic.min_insync_replicas":
			require.Equal(t, int64(expectedMinInsync), m.Gauge().DataPoints().At(0).IntValue())
		case "kafka.topic.log_retention_period":
			require.Equal(t, int64(expectedRetentionMs/1000), m.Gauge().DataPoints().At(0).IntValue())
		case "kafka.topic.log_retention_size":
			require.Equal(t, int64(expectedRetentionBytes), m.Gauge().DataPoints().At(0).IntValue())
		case "kafka.partition.replicas":
			require.Equal(t, numPartitions, m.Sum().DataPoints().Len())
			for j := 0; j < m.Sum().DataPoints().Len(); j++ {
				require.Equal(t, int64(numBrokers), m.Sum().DataPoints().At(j).IntValue())
			}
		case "kafka.partition.replicas_in_sync":
			require.Equal(t, numPartitions, m.Sum().DataPoints().Len())
			for j := 0; j < m.Sum().DataPoints().Len(); j++ {
				require.Equal(t, int64(numBrokers), m.Sum().DataPoints().At(j).IntValue())
			}
		}
	}
	for _, name := range []string{
		"kafka.topic.partitions",
		"kafka.topic.replication_factor",
		"kafka.topic.min_insync_replicas",
		"kafka.topic.log_retention_period",
		"kafka.topic.log_retention_size",
		"kafka.partition.replicas",
		"kafka.partition.replicas_in_sync",
	} {
		require.True(t, seen[name], "metric %s not emitted", name)
	}
}

func TestTopicScraperFranz_TopicFilterExcludes(t *testing.T) {
	setFranzGo(t, true)

	_, clientCfg := kafkatest.NewCluster(t, kfake.SeedTopics(1, "include-me", "_internal"))
	cfg := Config{
		ClientConfig:         clientCfg,
		MetricsBuilderConfig: metadata.NewDefaultMetricsBuilderConfig(),
		TopicMatch:           "include-.*",
	}

	s, err := createTopicsScraperFranz(t.Context(), cfg, receivertest.NewNopSettings(metadata.Type))
	require.NoError(t, err)
	require.NoError(t, s.Start(t.Context(), componenttest.NewNopHost()))
	t.Cleanup(func() { require.NoError(t, s.Shutdown(t.Context())) })

	md, err := s.ScrapeMetrics(t.Context())
	require.NoError(t, err)

	if md.ResourceMetrics().Len() == 0 {
		return
	}
	ms := md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
	for i := 0; i < ms.Len(); i++ {
		m := ms.At(i)
		if m.Name() != "kafka.topic.partitions" {
			continue
		}
		dps := m.Sum().DataPoints()
		for j := 0; j < dps.Len(); j++ {
			topic, _ := dps.At(j).Attributes().Get("topic")
			require.Equal(t, "include-me", topic.Str())
		}
	}
}

func TestTopicScraperFranz_ScrapePartialError_UnparseableConfig(t *testing.T) {
	setFranzGo(t, true)

	const topic = "topic-a"
	_, clientCfg := kafkatest.NewCluster(t,
		kfake.SeedTopics(2, topic),
		// kfake propagates min.insync.replicas from broker config into the topic config.
		kfake.BrokerConfigs(map[string]string{
			minInsyncReplicas: "not-a-number",
		}),
	)
	cfg := Config{
		ClientConfig:         clientCfg,
		MetricsBuilderConfig: metadata.NewDefaultMetricsBuilderConfig(),
		TopicMatch:           ".*",
	}
	cfg.Metrics.KafkaTopicMinInsyncReplicas.Enabled = true

	s, err := createTopicsScraperFranz(t.Context(), cfg, receivertest.NewNopSettings(metadata.Type))
	require.NoError(t, err)
	require.NoError(t, s.Start(t.Context(), componenttest.NewNopHost()))
	t.Cleanup(func() { require.NoError(t, s.Shutdown(t.Context())) })

	md, err := s.ScrapeMetrics(t.Context())
	require.Error(t, err)
	var partial scrapererror.PartialScrapeError
	require.ErrorAs(t, err, &partial, "expected PartialScrapeError, got %T: %v", err, err)
	require.Equal(t, 1, partial.Failed)

	ms := md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
	var sawPartitions, sawMinInsync bool
	for i := 0; i < ms.Len(); i++ {
		switch ms.At(i).Name() {
		case "kafka.topic.partitions":
			sawPartitions = true
		case "kafka.topic.min_insync_replicas":
			sawMinInsync = true
		}
	}
	require.True(t, sawPartitions)
	require.False(t, sawMinInsync, "min_insync_replicas should be skipped on parse failure")
}

func TestTopicScraperFranz_ScrapeUnreachable(t *testing.T) {
	setFranzGo(t, true)

	cluster, clientCfg := kafkatest.NewCluster(t, kfake.SeedTopics(1, "topic-a"))
	cfg := Config{
		ClientConfig:         clientCfg,
		MetricsBuilderConfig: metadata.NewDefaultMetricsBuilderConfig(),
		TopicMatch:           ".*",
	}

	s, err := createTopicsScraperFranz(t.Context(), cfg, receivertest.NewNopSettings(metadata.Type))
	require.NoError(t, err)
	require.NoError(t, s.Start(t.Context(), componenttest.NewNopHost()))
	t.Cleanup(func() { require.NoError(t, s.Shutdown(t.Context())) })

	cluster.Close()

	ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
	defer cancel()
	_, err = s.ScrapeMetrics(ctx)
	require.Error(t, err)
}

// Optional parity check: default regex compiles (like Sarama test style)
func TestTopicScraperFranz_FilterCompilesLikeSarama(t *testing.T) {
	setFranzGo(t, true)

	r := regexp.MustCompile(defaultTopicMatch)
	require.NotNil(t, r)
}
