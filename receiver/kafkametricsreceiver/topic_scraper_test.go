// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkametricsreceiver

import (
	"context"
	"errors"
	"regexp"
	"testing"

	"github.com/IBM/sarama"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/kafka/configkafka"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkametricsreceiver/internal/metadata"
)

func TestTopicShutdown(t *testing.T) {
	client := newMockClient()
	client.closed = false
	client.close = nil
	client.Mock.
		On("Close").Return(nil).
		On("Closed").Return(false)
	scraper := brokerScraper{
		client:   client,
		settings: receivertest.NewNopSettings(metadata.Type),
		config:   Config{},
	}
	_ = scraper.shutdown(context.Background())
	client.AssertExpectations(t)
}

func TestTopicShutdown_closed(t *testing.T) {
	client := newMockClient()
	client.closed = true
	client.Mock.
		On("Closed").Return(true)
	scraper := topicScraper{
		client:   client,
		settings: receivertest.NewNopSettings(metadata.Type),
		config:   Config{},
	}
	_ = scraper.shutdown(context.Background())
	client.AssertExpectations(t)
}

func TestTopicScraper_createsScraper(t *testing.T) {
	newSaramaClient = mockNewSaramaClient
	ms, err := createTopicsScraper(context.Background(), Config{}, receivertest.NewNopSettings(metadata.Type))
	assert.NoError(t, err)
	assert.NotNil(t, ms)
}

func TestTopicScraper_ScrapeHandlesError(t *testing.T) {
	newSaramaClient = func(context.Context, configkafka.ClientConfig) (sarama.Client, error) {
		return nil, errors.New("no scraper here")
	}
	ms, err := createTopicsScraper(context.Background(), Config{}, receivertest.NewNopSettings(metadata.Type))
	assert.NotNil(t, ms)
	assert.NoError(t, err)
	_, err = ms.ScrapeMetrics(context.Background())
	assert.Error(t, err)
}

func TestTopicScraper_ShutdownHandlesNilClient(t *testing.T) {
	newSaramaClient = func(context.Context, configkafka.ClientConfig) (sarama.Client, error) {
		return nil, errors.New("no scraper here")
	}
	ms, err := createTopicsScraper(context.Background(), Config{}, receivertest.NewNopSettings(metadata.Type))
	assert.NotNil(t, ms)
	assert.NoError(t, err)
	err = ms.Shutdown(context.Background())
	assert.NoError(t, err)
}

func TestTopicScraper_startScraperCreatesClient(t *testing.T) {
	newSaramaClient = mockNewSaramaClient
	ms, err := createTopicsScraper(context.Background(), Config{}, receivertest.NewNopSettings(metadata.Type))
	assert.NotNil(t, ms)
	assert.NoError(t, err)
	err = ms.Start(context.Background(), nil)
	assert.NoError(t, err)
}

func TestTopicScraper_createScraperHandles_invalid_topicMatch(t *testing.T) {
	newSaramaClient = mockNewSaramaClient
	ms, err := createTopicsScraper(context.Background(), Config{
		TopicMatch: "[",
	}, receivertest.NewNopSettings(metadata.Type))
	assert.Error(t, err)
	assert.Nil(t, ms)
}

func TestTopicScraper_scrapes(t *testing.T) {
	client := newMockClient()
	var testOffset int64 = 5
	client.offset = testOffset
	config := createDefaultConfig().(*Config)
	match := regexp.MustCompile(config.TopicMatch)
	scraper := topicScraper{
		client:       client,
		clusterAdmin: newMockClusterAdmin(),
		settings:     receivertest.NewNopSettings(metadata.Type),
		config:       *config,
		topicFilter:  match,
	}
	require.NoError(t, scraper.start(context.Background(), componenttest.NewNopHost()))
	md, err := scraper.scrape(context.Background())
	assert.NoError(t, err)
	require.Equal(t, 1, md.ResourceMetrics().Len())
	require.Equal(t, 1, md.ResourceMetrics().At(0).ScopeMetrics().Len())
	if val, ok := md.ResourceMetrics().At(0).Resource().Attributes().Get("kafka.cluster.alias"); ok {
		require.Equal(t, testClusterAlias, val.Str())
	}
	ms := md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
	for i := 0; i < ms.Len(); i++ {
		m := ms.At(i)
		switch m.Name() {
		case "kafka.topic.partitions":
			assert.Equal(t, m.Sum().DataPoints().At(0).IntValue(), int64(len(testPartitions)))
		case "kafka.topic.replication_factor":
			assert.Equal(t, m.Gauge().DataPoints().At(0).IntValue(), int64(testReplicationFactor))
		case "kafka.topic.min_insync_replicas":
			assert.Equal(t, m.Gauge().DataPoints().At(0).IntValue(), int64(testMinInsyncReplicas))
		case "kafka.topic.log_retention_period":
			assert.Equal(t, m.Gauge().DataPoints().At(0).IntValue(), int64(testLogRetentionMs/1000))
		case "kafka.topic.log_retention_size":
			assert.Equal(t, m.Gauge().DataPoints().At(0).IntValue(), int64(testLogRetentionBytes))
		case "kafka.partition.current_offset":
			assert.Equal(t, m.Gauge().DataPoints().At(0).IntValue(), testOffset)
		case "kafka.partition.oldest_offset":
			assert.Equal(t, m.Gauge().DataPoints().At(0).IntValue(), testOffset)
		case "kafka.partition.replicas":
			assert.Equal(t, m.Sum().DataPoints().At(0).IntValue(), int64(len(testReplicas)))
		case "kafka.partition.replicas_in_sync":
			assert.Equal(t, m.Sum().DataPoints().At(0).IntValue(), int64(len(testReplicas)))
		}
	}
}

func TestTopicScraper_scrape_handlesTopicError(t *testing.T) {
	client := newMockClient()
	client.topics = nil
	config := createDefaultConfig().(*Config)
	match := regexp.MustCompile(config.TopicMatch)
	scraper := topicScraper{
		client:      client,
		settings:    receivertest.NewNopSettings(metadata.Type),
		topicFilter: match,
	}
	_, err := scraper.scrape(context.Background())
	assert.Error(t, err)
}

func TestTopicScraper_scrape_handlesPartitionError(t *testing.T) {
	client := newMockClient()
	client.partitions = nil
	config := createDefaultConfig().(*Config)
	match := regexp.MustCompile(config.TopicMatch)
	scraper := topicScraper{
		client:       client,
		clusterAdmin: newMockClusterAdmin(),
		settings:     receivertest.NewNopSettings(metadata.Type),
		topicFilter:  match,
	}
	require.NoError(t, scraper.start(context.Background(), componenttest.NewNopHost()))
	_, err := scraper.scrape(context.Background())
	assert.Error(t, err)
}

func TestTopicScraper_scrape_handlesPartialScrapeErrors(t *testing.T) {
	client := newMockClient()
	client.replicas = nil
	client.inSyncReplicas = nil
	client.replicas = nil
	client.offset = -1
	config := createDefaultConfig().(*Config)
	match := regexp.MustCompile(config.TopicMatch)
	scraper := topicScraper{
		client:       client,
		clusterAdmin: newMockClusterAdmin(),
		settings:     receivertest.NewNopSettings(metadata.Type),
		topicFilter:  match,
	}
	require.NoError(t, scraper.start(context.Background(), componenttest.NewNopHost()))
	_, err := scraper.scrape(context.Background())
	assert.Error(t, err)
}
