// Copyright  OpenTelemetry Authors
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
	"go.uber.org/zap"

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
		client: client,
		logger: zap.NewNop(),
		config: Config{},
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
		client: client,
		logger: zap.NewNop(),
		config: Config{},
	}
	_ = scraper.shutdown(context.Background())
	client.AssertExpectations(t)
}

func TestTopicScraper_Name(t *testing.T) {
	s := topicScraper{}
	assert.Equal(t, s.Name(), "topics")
}

func TestTopicScraper_createsScraper(t *testing.T) {
	sc := sarama.NewConfig()
	newSaramaClient = mockNewSaramaClient
	ms, err := createTopicsScraper(context.Background(), Config{}, sc, zap.NewNop())
	assert.Nil(t, err)
	assert.NotNil(t, ms)
}

func TestTopicScraper_createScraperHandlesError(t *testing.T) {
	newSaramaClient = func(addrs []string, conf *sarama.Config) (sarama.Client, error) {
		return nil, fmt.Errorf("no scraper here")
	}
	sc := sarama.NewConfig()
	ms, err := createTopicsScraper(context.Background(), Config{}, sc, zap.NewNop())
	assert.NotNil(t, err)
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
		logger:      zap.NewNop(),
		topicFilter: match,
	}
	rm, err := scraper.scrape(context.Background())
	assert.Nil(t, err)
	assert.NotNil(t, rm)
	ms := rm.At(0).InstrumentationLibraryMetrics().At(0).Metrics()
	for i := 0; i < ms.Len(); i++ {
		m := ms.At(i)
		dp := m.IntGauge().DataPoints().At(0)
		switch m.Name() {
		case metadata.M.KafkaTopicPartitions.Name():
			assert.Equal(t, dp.Value(), int64(len(testPartitions)))
		case metadata.M.KafkaPartitionCurrentOffset.Name():
			assert.Equal(t, dp.Value(), testOffset)
		case metadata.M.KafkaPartitionOldestOffset.Name():
			assert.Equal(t, dp.Value(), testOffset)
		case metadata.M.KafkaPartitionReplicas.Name():
			assert.Equal(t, dp.Value(), int64(len(testReplicas)))
		case metadata.M.KafkaPartitionReplicasInSync.Name():
			assert.Equal(t, dp.Value(), int64(len(testReplicas)))
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
		logger:      zap.NewNop(),
		topicFilter: match,
	}
	_, err := scraper.scrape(context.Background())
	assert.NotNil(t, err)
}

func TestTopicScraper_scrape_handlesPartitionError(t *testing.T) {
	client := newMockClient()
	client.partitions = nil
	config := createDefaultConfig().(*Config)
	match := regexp.MustCompile(config.TopicMatch)
	scraper := topicScraper{
		client:      client,
		logger:      zap.NewNop(),
		topicFilter: match,
	}
	_, err := scraper.scrape(context.Background())
	assert.NotNil(t, err)
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
		logger:      zap.NewNop(),
		topicFilter: match,
	}
	_, err := scraper.scrape(context.Background())
	assert.NotNil(t, err)
}
