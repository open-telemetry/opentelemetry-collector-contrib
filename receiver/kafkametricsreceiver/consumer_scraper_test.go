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

func TestConsumerShutdown(t *testing.T) {
	client := newMockClient()
	client.closed = false
	client.close = nil
	client.Mock.
		On("Close").Return(nil).
		On("Closed").Return(false)
	scraper := consumerScraper{
		client: client,
	}
	_ = scraper.shutdown(context.Background())
	client.AssertExpectations(t)
}

func TestConsumerShutdown_closed(t *testing.T) {
	client := newMockClient()
	client.closed = true
	client.Mock.
		On("Closed").Return(true)
	scraper := consumerScraper{
		client: client,
	}
	_ = scraper.shutdown(context.Background())
	client.AssertExpectations(t)
}

func TestConsumerScraper_createConsumerScraper(t *testing.T) {
	newSaramaClient = mockNewSaramaClient
	newClusterAdmin = mockNewClusterAdmin
	cs, err := createConsumerScraper(context.Background(), Config{}, receivertest.NewNopSettings(metadata.Type))
	assert.NoError(t, err)
	assert.NotNil(t, cs)
}

func TestConsumerScraper_scrape_handles_client_error(t *testing.T) {
	newSaramaClient = func(context.Context, configkafka.ClientConfig) (sarama.Client, error) {
		return nil, errors.New("new client failed")
	}
	cs, err := createConsumerScraper(context.Background(), Config{}, receivertest.NewNopSettings(metadata.Type))
	assert.NoError(t, err)
	assert.NotNil(t, cs)
	_, err = cs.ScrapeMetrics(context.Background())
	assert.Error(t, err)
}

func TestConsumerScraper_scrape_handles_nil_client(t *testing.T) {
	newSaramaClient = func(context.Context, configkafka.ClientConfig) (sarama.Client, error) {
		return nil, errors.New("new client failed")
	}
	cs, err := createConsumerScraper(context.Background(), Config{}, receivertest.NewNopSettings(metadata.Type))
	assert.NoError(t, err)
	assert.NotNil(t, cs)
	err = cs.Shutdown(context.Background())
	assert.NoError(t, err)
}

func TestConsumerScraper_scrape_handles_clusterAdmin_error(t *testing.T) {
	newSaramaClient = func(context.Context, configkafka.ClientConfig) (sarama.Client, error) {
		client := newMockClient()
		client.Mock.
			On("Close").Return(nil)
		return client, nil
	}
	newClusterAdmin = func(sarama.Client) (sarama.ClusterAdmin, error) {
		return nil, errors.New("new cluster admin failed")
	}
	cs, err := createConsumerScraper(context.Background(), Config{}, receivertest.NewNopSettings(metadata.Type))
	assert.NoError(t, err)
	assert.NotNil(t, cs)
	_, err = cs.ScrapeMetrics(context.Background())
	assert.Error(t, err)
}

func TestConsumerScraperStart(t *testing.T) {
	newSaramaClient = mockNewSaramaClient
	newClusterAdmin = mockNewClusterAdmin
	cs, err := createConsumerScraper(context.Background(), Config{}, receivertest.NewNopSettings(metadata.Type))
	assert.NoError(t, err)
	assert.NotNil(t, cs)
	err = cs.Start(context.Background(), nil)
	assert.NoError(t, err)
}

func TestConsumerScraper_createScraper_handles_invalid_topic_match(t *testing.T) {
	newSaramaClient = mockNewSaramaClient
	newClusterAdmin = mockNewClusterAdmin
	cs, err := createConsumerScraper(context.Background(), Config{
		TopicMatch: "[",
	}, receivertest.NewNopSettings(metadata.Type))
	assert.Error(t, err)
	assert.Nil(t, cs)
}

func TestConsumerScraper_createScraper_handles_invalid_group_match(t *testing.T) {
	newSaramaClient = mockNewSaramaClient
	newClusterAdmin = mockNewClusterAdmin
	cs, err := createConsumerScraper(context.Background(), Config{
		GroupMatch: "[",
	}, receivertest.NewNopSettings(metadata.Type))
	assert.Error(t, err)
	assert.Nil(t, cs)
}

func TestConsumerScraper_scrape(t *testing.T) {
	filter := regexp.MustCompile(defaultGroupMatch)
	cs := consumerScraper{
		client:       newMockClient(),
		settings:     receivertest.NewNopSettings(metadata.Type),
		clusterAdmin: newMockClusterAdmin(),
		topicFilter:  filter,
		groupFilter:  filter,
	}
	require.NoError(t, cs.start(context.Background(), componenttest.NewNopHost()))
	md, err := cs.scrape(context.Background())
	assert.NoError(t, err)
	assert.NotNil(t, md)
}

func TestConsumerScraper_scrape_handlesListTopicError(t *testing.T) {
	filter := regexp.MustCompile(defaultGroupMatch)
	clusterAdmin := newMockClusterAdmin()
	client := newMockClient()
	clusterAdmin.topics = nil
	cs := consumerScraper{
		client:       client,
		settings:     receivertest.NewNopSettings(metadata.Type),
		clusterAdmin: clusterAdmin,
		topicFilter:  filter,
		groupFilter:  filter,
	}
	_, err := cs.scrape(context.Background())
	assert.Error(t, err)
}

func TestConsumerScraper_scrape_handlesListConsumerGroupError(t *testing.T) {
	filter := regexp.MustCompile(defaultGroupMatch)
	clusterAdmin := newMockClusterAdmin()
	clusterAdmin.consumerGroups = nil
	cs := consumerScraper{
		client:       newMockClient(),
		settings:     receivertest.NewNopSettings(metadata.Type),
		clusterAdmin: clusterAdmin,
		topicFilter:  filter,
		groupFilter:  filter,
	}
	_, err := cs.scrape(context.Background())
	assert.Error(t, err)
}

func TestConsumerScraper_scrape_handlesDescribeConsumerError(t *testing.T) {
	filter := regexp.MustCompile(defaultGroupMatch)
	clusterAdmin := newMockClusterAdmin()
	clusterAdmin.consumerGroupDescriptions = nil
	cs := consumerScraper{
		client:       newMockClient(),
		settings:     receivertest.NewNopSettings(metadata.Type),
		clusterAdmin: clusterAdmin,
		topicFilter:  filter,
		groupFilter:  filter,
	}
	_, err := cs.scrape(context.Background())
	assert.Error(t, err)
}

func TestConsumerScraper_scrape_handlesOffsetPartialError(t *testing.T) {
	filter := regexp.MustCompile(defaultGroupMatch)
	clusterAdmin := newMockClusterAdmin()
	client := newMockClient()
	client.offset = -1
	clusterAdmin.consumerGroupOffsets = nil
	cs := consumerScraper{
		client:       client,
		settings:     receivertest.NewNopSettings(metadata.Type),
		groupFilter:  filter,
		topicFilter:  filter,
		clusterAdmin: clusterAdmin,
	}
	require.NoError(t, cs.start(context.Background(), componenttest.NewNopHost()))
	_, err := cs.scrape(context.Background())
	assert.Error(t, err)
}

func TestConsumerScraper_scrape_handlesPartitionPartialError(t *testing.T) {
	filter := regexp.MustCompile(defaultGroupMatch)
	clusterAdmin := newMockClusterAdmin()
	client := newMockClient()
	client.partitions = nil
	clusterAdmin.consumerGroupOffsets = nil
	cs := consumerScraper{
		client:       client,
		settings:     receivertest.NewNopSettings(metadata.Type),
		groupFilter:  filter,
		topicFilter:  filter,
		clusterAdmin: clusterAdmin,
	}
	require.NoError(t, cs.start(context.Background(), componenttest.NewNopHost()))
	_, err := cs.scrape(context.Background())
	assert.Error(t, err)
}
