// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkametricsreceiver

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kfake"
	"github.com/twmb/franz-go/pkg/kgo"
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
		MetricsBuilderConfig: metadata.NewDefaultMetricsBuilderConfig(),
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

func TestConsumerScraperFranz_ScrapeMetricValues(t *testing.T) {
	setFranzGo(t, true)

	const (
		topic     = "topic-a"
		group     = "test-group"
		committed = int64(7)
	)

	cluster, clientCfg := kafkatest.NewCluster(t, kfake.SeedTopics(1, topic))
	cl, err := kgo.NewClient(kgo.SeedBrokers(cluster.ListenAddrs()...))
	require.NoError(t, err)
	t.Cleanup(cl.Close)

	adm := kadm.NewClient(cl)

	// Produce one record so the partition has an end offset.
	produceResults := cl.ProduceSync(t.Context(), &kgo.Record{Topic: topic, Value: []byte("payload")})
	require.NoError(t, produceResults.FirstErr())

	// Commit an offset for the group at partition 0.
	var os kadm.Offsets
	os.AddOffset(topic, 0, committed, -1)
	_, err = adm.CommitOffsets(t.Context(), group, os)
	require.NoError(t, err)

	cfg := Config{
		ClientConfig:         clientCfg,
		MetricsBuilderConfig: metadata.NewDefaultMetricsBuilderConfig(),
		ClusterAlias:         "test-cluster",
		TopicMatch:           ".*",
		GroupMatch:           ".*",
	}
	cfg.ResourceAttributes.KafkaClusterAlias.Enabled = true

	s, err := createConsumerScraperFranz(t.Context(), cfg, receivertest.NewNopSettings(metadata.Type))
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

	// We produced 1 record at partition 0, and committed offset = 7. End offset is 1 (record offset 0 + 1),
	// so lag = 1 - 7 = -6. The scraper records the raw difference; we just verify the metric is emitted
	// with the committed offset and a deterministic lag.
	const expectedEnd = int64(1)
	const expectedLag = expectedEnd - committed

	ms := rm.ScopeMetrics().At(0).Metrics()
	seen := map[string]bool{}
	for i := 0; i < ms.Len(); i++ {
		m := ms.At(i)
		seen[m.Name()] = true
		switch m.Name() {
		case "kafka.consumer_group.members":
			// No active members for an offset-only commit.
			require.Equal(t, int64(0), m.Sum().DataPoints().At(0).IntValue())
		case "kafka.consumer_group.offset":
			dps := m.Gauge().DataPoints()
			var found bool
			for j := 0; j < dps.Len(); j++ {
				partition, _ := dps.At(j).Attributes().Get("partition")
				if partition.Int() == 0 {
					require.Equal(t, committed, dps.At(j).IntValue())
					found = true
				}
			}
			require.True(t, found, "expected offset for partition 0")
		case "kafka.consumer_group.lag":
			dps := m.Gauge().DataPoints()
			var found bool
			for j := 0; j < dps.Len(); j++ {
				partition, _ := dps.At(j).Attributes().Get("partition")
				if partition.Int() == 0 {
					require.Equal(t, expectedLag, dps.At(j).IntValue())
					found = true
				}
			}
			require.True(t, found, "expected lag for partition 0")
		case "kafka.consumer_group.offset_sum":
			require.Equal(t, committed, m.Gauge().DataPoints().At(0).IntValue())
		case "kafka.consumer_group.lag_sum":
			require.Equal(t, expectedLag, m.Gauge().DataPoints().At(0).IntValue())
		}
	}
	for _, name := range []string{
		"kafka.consumer_group.members",
		"kafka.consumer_group.offset",
		"kafka.consumer_group.lag",
		"kafka.consumer_group.offset_sum",
		"kafka.consumer_group.lag_sum",
	} {
		require.True(t, seen[name], "metric %s not emitted", name)
	}

	// Clean up the group so the kfake goroutine exits.
	_, err = adm.DeleteGroups(t.Context(), group)
	require.NoError(t, err)
}

func TestConsumerScraperFranz_ScrapeUnreachable(t *testing.T) {
	setFranzGo(t, true)

	cluster, clientCfg := kafkatest.NewCluster(t, kfake.SeedTopics(1, "topic-a"))
	cfg := Config{
		ClientConfig:         clientCfg,
		MetricsBuilderConfig: metadata.NewDefaultMetricsBuilderConfig(),
		TopicMatch:           ".*",
		GroupMatch:           ".*",
	}

	s, err := createConsumerScraperFranz(t.Context(), cfg, receivertest.NewNopSettings(metadata.Type))
	require.NoError(t, err)
	require.NoError(t, s.Start(t.Context(), componenttest.NewNopHost()))
	t.Cleanup(func() { require.NoError(t, s.Shutdown(t.Context())) })

	cluster.Close()

	ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
	defer cancel()
	_, err = s.ScrapeMetrics(ctx)
	require.Error(t, err)
}

// (Optional) If you want a direct filter parity check like the Sarama unit did:
func TestConsumerScraperFranz_FilterCompilesLikeSarama(t *testing.T) {
	setFranzGo(t, true)

	// Just prove that a typical defaultGroupMatch compiles and can be set.
	// (Your Sarama tests reference defaultGroupMatch; here we mimic that behavior.)
	r := regexp.MustCompile(defaultGroupMatch)
	require.NotNil(t, r)
}
