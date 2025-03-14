// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkametricsreceiver

import (
	"context"
	"testing"

	"github.com/IBM/sarama"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/collector/scraper"
	"go.opentelemetry.io/collector/scraper/scrapererror"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/kafka/configkafka"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkametricsreceiver/internal/metadata"
)

func TestTopicScraper_StartShutdown(t *testing.T) {
	testScraperStartShutdown(t, func(f1 newSaramaClientFunc, f2 newSaramaClusterAdminClientFunc) (scraper.Metrics, error) {
		return newTopicsScraper(
			Config{TopicMatch: ".*"},
			receivertest.NewNopSettings(metadata.Type), f1, f2,
		)
	})
}

func TestNewTopicScraper_InvalidTopicMatch(t *testing.T) {
	scraper, err := newTopicsScraper(
		Config{TopicMatch: "["},
		receivertest.NewNopSettings(metadata.Type),
		mockNewSaramaClient, mockNewClusterAdmin,
	)
	assert.EqualError(t, err, "failed to compile topic filter: error parsing regexp: missing closing ]: `[`")
	assert.Nil(t, scraper)
}

func TestTopicScraper_ScrapeMetrics(t *testing.T) {
	var testOffset int64 = 5
	mockClient := newMockClient()
	mockClient.offset = testOffset
	config := *createDefaultConfig().(*Config)
	config.Metrics.KafkaTopicLogRetentionPeriod.Enabled = true
	config.Metrics.KafkaTopicLogRetentionSize.Enabled = true
	config.Metrics.KafkaTopicMinInsyncReplicas.Enabled = true
	config.Metrics.KafkaTopicReplicationFactor.Enabled = true
	scraper := newTestTopicsScraper(t, config, mockClient)

	md, err := scraper.ScrapeMetrics(context.Background())
	assert.NoError(t, err)

	expected, err := golden.ReadMetrics("testdata/golden/" + t.Name() + ".yaml")
	assert.NoError(t, err)
	assert.NoError(t, pmetrictest.CompareMetrics(
		expected, md,
		pmetrictest.IgnoreStartTimestamp(),
		pmetrictest.IgnoreTimestamp(),
	))
}

func TestTopicScraper_ScrapeMetrics_Errors(t *testing.T) {
	mockClient := newMockClient()
	scraper := newTestTopicsScraper(t, *createDefaultConfig().(*Config), mockClient)

	// Trigger failure to scrape topic-partitions, which results in a partial error.
	mockClient.replicas = nil
	mockClient.inSyncReplicas = nil
	mockClient.replicas = nil
	mockClient.offset = -1
	_, err := scraper.ScrapeMetrics(context.Background())
	assert.EqualError(t, err, "mock offset error; mock offset error; mock replicas error; mock in sync replicas error")
	assert.True(t, scrapererror.IsPartialScrapeError(err))

	// trigger mockSaramaClient.Partitions() to return an error
	mockClient.partitions = nil
	_, err = scraper.ScrapeMetrics(context.Background())
	assert.EqualError(t, err, "mock partition error")
	assert.False(t, scrapererror.IsPartialScrapeError(err))

	// trigger mockSaramaClient.Topics to return an error
	mockClient.topics = nil
	_, err = scraper.ScrapeMetrics(context.Background())
	assert.EqualError(t, err, "mock topic error")
	assert.False(t, scrapererror.IsPartialScrapeError(err))
}

func newTestTopicsScraper(tb testing.TB, config Config, mockClient sarama.Client) scraper.Metrics {
	scraper, err := newTopicsScraper(
		config, receivertest.NewNopSettings(metadata.Type),
		func(context.Context, configkafka.ClientConfig) (sarama.Client, error) {
			return mockClient, nil
		},
		mockNewClusterAdmin,
	)
	require.NoError(tb, err)

	err = scraper.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(tb, err)
	tb.Cleanup(func() {
		assert.NoError(tb, scraper.Shutdown(context.Background()))
	})
	return scraper
}
