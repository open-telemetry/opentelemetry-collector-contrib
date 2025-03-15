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
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkametricsreceiver/internal/metadata"
)

func TestConsumerScraper_StartShutdown(t *testing.T) {
	testScraperStartShutdown(t, func(f1 newSaramaClientFunc, f2 newSaramaClusterAdminClientFunc) (scraper.Metrics, error) {
		return newConsumerScraper(Config{}, receivertest.NewNopSettings(metadata.Type), f1, f2)
	})
}

func TestNewConsumerScraper_InvalidTopicMatch(t *testing.T) {
	scraper, err := newConsumerScraper(
		Config{TopicMatch: "["},
		receivertest.NewNopSettings(metadata.Type),
		mockNewSaramaClient, mockNewClusterAdmin,
	)
	assert.EqualError(t, err, "failed to compile topic filter: error parsing regexp: missing closing ]: `[`")
	assert.Nil(t, scraper)
}

func TestNewConsumerScraper_InvalidGroupMatch(t *testing.T) {
	scraper, err := newConsumerScraper(
		Config{GroupMatch: "["},
		receivertest.NewNopSettings(metadata.Type),
		mockNewSaramaClient, mockNewClusterAdmin,
	)
	assert.EqualError(t, err, "failed to compile group filter: error parsing regexp: missing closing ]: `[`")
	assert.Nil(t, scraper)
}

func TestConsumerScraper_ScrapeMetrics(t *testing.T) {
	mockClient := newMockClient()
	mockAdminClient := newMockClusterAdmin()
	scraper := newTestConsumerScraper(t, *createDefaultConfig().(*Config), mockClient, mockAdminClient)

	md, err := scraper.ScrapeMetrics(context.Background())
	require.NoError(t, err)
	assert.NotZero(t, md.DataPointCount())
	// TODO check the metric contents
}

func TestConsumerScraper_ScrapeMetrics_Errors(t *testing.T) {
	mockClient := newMockClient()
	mockAdminClient := newMockClusterAdmin()
	scraper := newTestConsumerScraper(t, *createDefaultConfig().(*Config), mockClient, mockAdminClient)

	// Trigger failure to scrape offsets, which results in a partial error.
	mockClient.offset = -1
	mockAdminClient.consumerGroupOffsets = nil
	_, err := scraper.ScrapeMetrics(context.Background())
	assert.EqualError(t, err, "mock offset error; mock consumer group offset error")
	assert.True(t, scrapererror.IsPartialScrapeError(err))

	// Trigger failure to scrape partitions, which results in a partial error.
	mockClient.partitions = nil
	mockAdminClient.consumerGroupOffsets = nil
	_, err = scraper.ScrapeMetrics(context.Background())
	assert.EqualError(t, err, "mock partition error; mock consumer group offset error")
	assert.True(t, scrapererror.IsPartialScrapeError(err))

	// Trigger failure to describe consumer groups
	mockAdminClient.consumerGroupDescriptions = nil
	_, err = scraper.ScrapeMetrics(context.Background())
	assert.EqualError(t, err, "error describing consumer groups")
	assert.False(t, scrapererror.IsPartialScrapeError(err))

	// Trigger failure to list topics
	mockAdminClient.topics = nil
	_, err = scraper.ScrapeMetrics(context.Background())
	assert.EqualError(t, err, "error getting topics")
	assert.False(t, scrapererror.IsPartialScrapeError(err))

	// Trigger failure to list consumer groups
	mockAdminClient.consumerGroups = nil
	_, err = scraper.ScrapeMetrics(context.Background())
	assert.EqualError(t, err, "error getting consumer groups")
	assert.False(t, scrapererror.IsPartialScrapeError(err))
}

func newTestConsumerScraper(
	tb testing.TB, config Config,
	mockClient sarama.Client, mockAdminClient sarama.ClusterAdmin,
) scraper.Metrics {
	scraper, err := newConsumerScraper(
		config, receivertest.NewNopSettings(metadata.Type),
		func(context.Context, configkafka.ClientConfig) (sarama.Client, error) {
			return mockClient, nil
		},
		func(sarama.Client) (sarama.ClusterAdmin, error) {
			return mockAdminClient, nil
		},
	)
	require.NoError(tb, err)

	err = scraper.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(tb, err)
	tb.Cleanup(func() {
		assert.NoError(tb, scraper.Shutdown(context.Background()))
	})
	return scraper
}
