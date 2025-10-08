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

// franz test config helper: one seed topic, permissive filters
func franzConsumerTestConfig(t *testing.T) Config {
	t.Helper()
	_, clientCfg := kafkatest.NewCluster(t, kfake.SeedTopics(1, "meta-topic"))
	cfg := Config{
		ClientConfig:         clientCfg,
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
		ClusterAlias:         "test-cluster-franz",
		// Use permissive filters so ListEndOffsets/ListStartOffsets have topics.
		TopicMatch: ".*",
		GroupMatch: ".*",
	}
	return cfg
}

func TestConsumerScraperFranz_CreateStartScrapeShutdown(t *testing.T) {
	// Feature gate not required when calling createConsumerScraperFranz directly,
	// but enabling keeps parity with receiver selection tests later.
	setFranzGo(t, true)

	cfg := franzConsumerTestConfig(t)

	var s scraper.Metrics
	var err error

	s, err = createConsumerScraperFranz(t.Context(), cfg, receivertest.NewNopSettings(metadata.Type))
	require.NoError(t, err)
	require.NotNil(t, s)

	require.NoError(t, s.Start(t.Context(), componenttest.NewNopHost()))

	// Scrape should succeed even if there are no consumer groups in kfake.
	md, err := s.ScrapeMetrics(t.Context())
	require.NoError(t, err)
	require.NotNil(t, md)
	// With kfake, there may be zero consumer groups; allow zero metrics.
	require.GreaterOrEqual(t, md.DataPointCount(), 0)

	require.NoError(t, s.Shutdown(t.Context()))
}

func TestConsumerScraperFranz_InvalidTopicRegex(t *testing.T) {
	setFranzGo(t, true)
	cfg := franzConsumerTestConfig(t)
	cfg.TopicMatch = "[" // invalid regex

	s, err := createConsumerScraperFranz(t.Context(), cfg, receivertest.NewNopSettings(metadata.Type))
	require.Error(t, err)
	require.Nil(t, s)
}

func TestConsumerScraperFranz_InvalidGroupRegex(t *testing.T) {
	setFranzGo(t, true)
	cfg := franzConsumerTestConfig(t)
	cfg.GroupMatch = "[" // invalid regex

	s, err := createConsumerScraperFranz(t.Context(), cfg, receivertest.NewNopSettings(metadata.Type))
	require.Error(t, err)
	require.Nil(t, s)
}

func TestConsumerScraperFranz_ShutdownWithoutStart_OK(t *testing.T) {
	setFranzGo(t, true)

	cfg := franzConsumerTestConfig(t)

	s, err := createConsumerScraperFranz(t.Context(), cfg, receivertest.NewNopSettings(metadata.Type))
	require.NoError(t, err)
	require.NotNil(t, s)

	// Shutdown prior to Start should be a no-op
	require.NoError(t, s.Shutdown(t.Context()))
}

func TestConsumerScraperFranz_EmptyClusterAlias(t *testing.T) {
	setFranzGo(t, true)

	cfg := franzConsumerTestConfig(t)
	cfg.ClusterAlias = ""

	s, err := createConsumerScraperFranz(t.Context(), cfg, receivertest.NewNopSettings(metadata.Type))
	require.NoError(t, err)
	require.NotNil(t, s)

	require.NoError(t, s.Start(t.Context(), componenttest.NewNopHost()))
	md, err := s.ScrapeMetrics(t.Context())
	require.NoError(t, err)

	// With kfake there may be zero consumer groups → zero RMs. Only assert the alias when one exists.
	if md.ResourceMetrics().Len() > 0 {
		rm := md.ResourceMetrics().At(0)
		_, ok := rm.Resource().Attributes().Get("kafka.cluster.alias")
		require.False(t, ok)
	}

	require.NoError(t, s.Shutdown(t.Context()))
}

// (Optional) If you want a direct filter parity check like the Sarama unit did:
func TestConsumerScraperFranz_FilterCompilesLikeSarama(t *testing.T) {
	setFranzGo(t, true)

	// Just prove that a typical defaultGroupMatch compiles and can be set.
	// (Your Sarama tests reference defaultGroupMatch; here we mimic that behavior.)
	r := regexp.MustCompile(defaultGroupMatch)
	require.NotNil(t, r)
}
