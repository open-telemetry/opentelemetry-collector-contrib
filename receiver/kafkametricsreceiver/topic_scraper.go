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
	"regexp"
	"time"

	"github.com/Shopify/sarama"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/consumer/simple"
	"go.opentelemetry.io/collector/receiver/scrapererror"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkametricsreceiver/internal/metadata"
)

type topicScraper struct {
	client       sarama.Client
	logger       *zap.Logger
	topicFilter  *regexp.Regexp
	saramaConfig *sarama.Config
	config       Config
}

func (s *topicScraper) Name() string {
	return "topics"
}

func (s *topicScraper) shutdown(context.Context) error {
	if !s.client.Closed() {
		return s.client.Close()
	}
	return nil
}

func (s *topicScraper) scrape(context.Context) (pdata.ResourceMetricsSlice, error) {
	topics, err := s.client.Topics()
	metrics := simple.Metrics{
		Metrics:                    pdata.NewMetrics(),
		MetricFactoriesByName:      metadata.M.FactoriesByName(),
		InstrumentationLibraryName: InstrumentationLibName,
		Timestamp:                  time.Now(),
	}
	if err != nil {
		s.logger.Error("Error fetching cluster topics ", zap.Error(err))
		return metrics.Metrics.ResourceMetrics(), err
	}

	var scrapeErrors = scrapererror.ScrapeErrors{}

	var matchedTopics []string
	for _, t := range topics {
		if s.topicFilter.MatchString(t) {
			matchedTopics = append(matchedTopics, t)
		}
	}
	for _, topic := range matchedTopics {
		partitions, err := s.client.Partitions(topic)
		if err != nil {
			scrapeErrors.Add(err)
			continue
		}
		topicMetrics := metrics.WithLabels(map[string]string{
			metadata.L.Topic: topic,
		})
		topicMetrics.AddGaugeDataPoint(metadata.M.KafkaTopicPartitions.Name(), int64(len(partitions)))
		for _, partition := range partitions {
			partitionMetrics := topicMetrics.WithLabels(map[string]string{
				metadata.L.Partition: string(partition),
			})
			currentOffset, err := s.client.GetOffset(topic, partition, sarama.OffsetNewest)
			if err != nil {
				scrapeErrors.AddPartial(1, err)
			} else {
				partitionMetrics.AddGaugeDataPoint(metadata.M.KafkaPartitionCurrentOffset.Name(), currentOffset)
			}
			oldestOffset, err := s.client.GetOffset(topic, partition, sarama.OffsetOldest)
			if err != nil {
				scrapeErrors.AddPartial(1, err)
			} else {
				partitionMetrics.AddGaugeDataPoint(metadata.M.KafkaPartitionOldestOffset.Name(), oldestOffset)
			}
			replicas, err := s.client.Replicas(topic, partition)
			if err != nil {
				scrapeErrors.AddPartial(1, err)
			} else {
				partitionMetrics.AddGaugeDataPoint(metadata.M.KafkaPartitionReplicas.Name(), int64(len(replicas)))
			}
			replicasInSync, err := s.client.InSyncReplicas(topic, partition)
			if err != nil {
				scrapeErrors.AddPartial(1, err)
			} else {
				partitionMetrics.AddGaugeDataPoint(metadata.M.KafkaPartitionReplicasInSync.Name(), int64(len(replicasInSync)))
			}
		}
	}
	return metrics.Metrics.ResourceMetrics(), scrapeErrors.Combine()
}

func createTopicsScraper(_ context.Context, config Config, saramaConfig *sarama.Config, logger *zap.Logger) (scraperhelper.ResourceMetricsScraper, error) {
	topicFilter := regexp.MustCompile(config.TopicMatch)
	client, err := newSaramaClient(config.Brokers, saramaConfig)
	if err != nil {
		return nil, err
	}
	s := topicScraper{
		client:       client,
		logger:       logger,
		topicFilter:  topicFilter,
		saramaConfig: saramaConfig,
		config:       config,
	}
	return scraperhelper.NewResourceMetricsScraper(
		s.Name(),
		s.scrape,
		scraperhelper.WithShutdown(s.shutdown),
	), nil
}
