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

package kafkametricsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkametricsreceiver"

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/Shopify/sarama"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scrapererror"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkametricsreceiver/internal/metadata"
)

type topicScraper struct {
	client       sarama.Client
	settings     receiver.CreateSettings
	topicFilter  *regexp.Regexp
	saramaConfig *sarama.Config
	config       Config
	mb           *metadata.MetricsBuilder
}

func (s *topicScraper) Name() string {
	return topicsScraperName
}

func (s *topicScraper) shutdown(context.Context) error {
	if s.client != nil && !s.client.Closed() {
		return s.client.Close()
	}
	return nil
}

func (s *topicScraper) start(_ context.Context, _ component.Host) error {
	s.mb = metadata.NewMetricsBuilder(s.config.MetricsBuilderConfig, s.settings)
	return nil
}

func (s *topicScraper) scrape(context.Context) (pmetric.Metrics, error) {
	if s.client == nil {
		client, err := newSaramaClient(s.config.Brokers, s.saramaConfig)
		if err != nil {
			return pmetric.Metrics{}, fmt.Errorf("failed to create client in topics scraper: %w", err)
		}
		s.client = client
	}

	topics, err := s.client.Topics()
	if err != nil {
		s.settings.Logger.Error("Error fetching cluster topics ", zap.Error(err))
		return pmetric.Metrics{}, err
	}

	var scrapeErrors = scrapererror.ScrapeErrors{}

	now := pcommon.NewTimestampFromTime(time.Now())

	for _, topic := range topics {
		if !s.topicFilter.MatchString(topic) {
			continue
		}
		partitions, err := s.client.Partitions(topic)
		if err != nil {
			scrapeErrors.Add(err)
			continue
		}

		s.mb.RecordKafkaTopicPartitionsDataPoint(now, int64(len(partitions)), topic)
		for _, partition := range partitions {
			currentOffset, err := s.client.GetOffset(topic, partition, sarama.OffsetNewest)
			if err != nil {
				scrapeErrors.AddPartial(1, err)
			} else {
				s.mb.RecordKafkaPartitionCurrentOffsetDataPoint(now, currentOffset, topic, int64(partition))
			}
			oldestOffset, err := s.client.GetOffset(topic, partition, sarama.OffsetOldest)
			if err != nil {
				scrapeErrors.AddPartial(1, err)
			} else {
				s.mb.RecordKafkaPartitionOldestOffsetDataPoint(now, oldestOffset, topic, int64(partition))
			}
			replicas, err := s.client.Replicas(topic, partition)
			if err != nil {
				scrapeErrors.AddPartial(1, err)
			} else {
				s.mb.RecordKafkaPartitionReplicasDataPoint(now, int64(len(replicas)), topic, int64(partition))
			}
			replicasInSync, err := s.client.InSyncReplicas(topic, partition)
			if err != nil {
				scrapeErrors.AddPartial(1, err)
			} else {
				s.mb.RecordKafkaPartitionReplicasInSyncDataPoint(now, int64(len(replicasInSync)), topic, int64(partition))
			}
		}
	}
	return s.mb.Emit(), scrapeErrors.Combine()
}

func createTopicsScraper(_ context.Context, cfg Config, saramaConfig *sarama.Config, settings receiver.CreateSettings) (scraperhelper.Scraper, error) {
	topicFilter, err := regexp.Compile(cfg.TopicMatch)
	if err != nil {
		return nil, fmt.Errorf("failed to compile topic filter: %w", err)
	}
	s := topicScraper{
		settings:     settings,
		topicFilter:  topicFilter,
		saramaConfig: saramaConfig,
		config:       cfg,
	}
	return scraperhelper.NewScraper(
		s.Name(),
		s.scrape,
		scraperhelper.WithStart(s.start),
		scraperhelper.WithShutdown(s.shutdown),
	)
}
