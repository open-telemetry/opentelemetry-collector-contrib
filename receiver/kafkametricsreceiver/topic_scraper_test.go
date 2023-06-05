// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package kafkametricsreceiver

import (
	"context"
	"fmt"
	"regexp"
	"testing"

	"github.com/Shopify/sarama"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/receiver/receivertest"
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
		settings: receivertest.NewNopCreateSettings(),
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
		settings: receivertest.NewNopCreateSettings(),
		config:   Config{},
	}
	_ = scraper.shutdown(context.Background())
	client.AssertExpectations(t)
}

func TestTopicScraper_Name(t *testing.T) {
	s := topicScraper{}
	assert.Equal(t, s.Name(), topicsScraperName)
}

func TestTopicScraper_createsScraper(t *testing.T) {
	sc := sarama.NewConfig()
	newSaramaClient = mockNewSaramaClient
	ms, err := createTopicsScraper(context.Background(), Config{}, sc, receivertest.NewNopCreateSettings())
	assert.NoError(t, err)
	assert.NotNil(t, ms)
}

func TestTopicScraper_ScrapeHandlesError(t *testing.T) {
	newSaramaClient = func(addrs []string, conf *sarama.Config) (sarama.Client, error) {
		return nil, fmt.Errorf("no scraper here")
	}
	sc := sarama.NewConfig()
	ms, err := createTopicsScraper(context.Background(), Config{}, sc, receivertest.NewNopCreateSettings())
	assert.NotNil(t, ms)
	assert.Nil(t, err)
	_, err = ms.Scrape(context.Background())
	assert.Error(t, err)
}

func TestTopicScraper_ShutdownHandlesNilClient(t *testing.T) {
	newSaramaClient = func(addrs []string, conf *sarama.Config) (sarama.Client, error) {
		return nil, fmt.Errorf("no scraper here")
	}
	sc := sarama.NewConfig()
	ms, err := createTopicsScraper(context.Background(), Config{}, sc, receivertest.NewNopCreateSettings())
	assert.NotNil(t, ms)
	assert.Nil(t, err)
	err = ms.Shutdown(context.Background())
	assert.NoError(t, err)
}

func TestTopicScraper_startScraperCreatesClient(t *testing.T) {
	newSaramaClient = mockNewSaramaClient
	sc := sarama.NewConfig()
	ms, err := createTopicsScraper(context.Background(), Config{}, sc, receivertest.NewNopCreateSettings())
	assert.NotNil(t, ms)
	assert.NoError(t, err)
	err = ms.Start(context.Background(), nil)
	assert.NoError(t, err)
}

func TestTopicScraper_createScraperHandles_invalid_topicMatch(t *testing.T) {
	newSaramaClient = mockNewSaramaClient
	sc := sarama.NewConfig()
	ms, err := createTopicsScraper(context.Background(), Config{
		TopicMatch: "[",
	}, sc, receivertest.NewNopCreateSettings())
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
		client:      client,
		settings:    receivertest.NewNopCreateSettings(),
		config:      *config,
		topicFilter: match,
	}
	require.NoError(t, scraper.start(context.Background(), componenttest.NewNopHost()))
	md, err := scraper.scrape(context.Background())
	assert.NoError(t, err)
	require.Equal(t, 1, md.ResourceMetrics().Len())
	require.Equal(t, 1, md.ResourceMetrics().At(0).ScopeMetrics().Len())
	ms := md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
	for i := 0; i < ms.Len(); i++ {
		m := ms.At(i)
		dp := m.Gauge().DataPoints().At(0)
		switch m.Name() {
		case "kafka.topic.partitions":
			assert.Equal(t, dp.IntValue(), int64(len(testPartitions)))
		case "kafka.partition.current_offset":
			assert.Equal(t, dp.IntValue(), testOffset)
		case "kafka.partition.oldest_offset":
			assert.Equal(t, dp.IntValue(), testOffset)
		case "kafka.partition.replicas":
			assert.Equal(t, dp.IntValue(), int64(len(testReplicas)))
		case "kafka.partition.replicas_in_sync":
			assert.Equal(t, dp.IntValue(), int64(len(testReplicas)))
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
		settings:    receivertest.NewNopCreateSettings(),
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
		client:      client,
		settings:    receivertest.NewNopCreateSettings(),
		topicFilter: match,
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
		client:      client,
		settings:    receivertest.NewNopCreateSettings(),
		topicFilter: match,
	}
	require.NoError(t, scraper.start(context.Background(), componenttest.NewNopHost()))
	_, err := scraper.scrape(context.Background())
	assert.Error(t, err)
}
