// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkametricsreceiver

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kfake"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/collector/scraper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/kafka/kafkatest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkametricsreceiver/internal/metadata"
)

// minimal franz test config for topics
func franzTopicsTestConfig(t *testing.T) Config {
	t.Helper()
	_, clientCfg := kafkatest.NewCluster(t, kfake.SeedTopics(1, "topic-a"))
	cfg := Config{
		ClientConfig:         clientCfg,
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
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

// Optional parity check: default regex compiles (like Sarama test style)
func TestTopicScraperFranz_FilterCompilesLikeSarama(t *testing.T) {
	setFranzGo(t, true)

	r := regexp.MustCompile(defaultTopicMatch)
	require.NotNil(t, r)
}
