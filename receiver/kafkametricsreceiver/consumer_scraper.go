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
	"time"

	"github.com/Shopify/sarama"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/consumer/simple"
	"go.opentelemetry.io/collector/receiver/scrapererror"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkametricsreceiver/internal/metadata"
)

type consumerScraper struct {
	client       sarama.Client
	logger       *zap.Logger
	groupFilter  *regexp.Regexp
	topicFilter  *regexp.Regexp
	clusterAdmin sarama.ClusterAdmin
	saramaConfig *sarama.Config
	config       Config
}

func (s *consumerScraper) Name() string {
	return consumersScraperName
}

func (s *consumerScraper) start(context.Context, component.Host) error {
	client, err := newSaramaClient(s.config.Brokers, s.saramaConfig)
	if err != nil {
		return fmt.Errorf("failed to create client while starting consumer scraper: %w", err)
	}
	clusterAdmin, err := newClusterAdmin(s.config.Brokers, s.saramaConfig)
	if err != nil {
		if client != nil {
			_ = client.Close()
		}
		return fmt.Errorf("failed to create cluster admin while starting consumer scraper: %w", err)
	}
	s.client = client
	s.clusterAdmin = clusterAdmin
	return nil
}

func (s *consumerScraper) shutdown(_ context.Context) error {
	if !s.client.Closed() {
		return s.client.Close()
	}
	return nil
}

func (s *consumerScraper) scrape(context.Context) (pdata.ResourceMetricsSlice, error) {
	metrics := simple.Metrics{
		Metrics:                    pdata.NewMetrics(),
		Timestamp:                  time.Now(),
		MetricFactoriesByName:      metadata.M.FactoriesByName(),
		InstrumentationLibraryName: InstrumentationLibName,
	}

	cgs, listErr := s.clusterAdmin.ListConsumerGroups()
	if listErr != nil {
		return metrics.ResourceMetrics(), listErr
	}

	var matchedGrpIds []string
	for grpID := range cgs {
		if s.groupFilter.MatchString(grpID) {
			matchedGrpIds = append(matchedGrpIds, grpID)
		}
	}

	allTopics, listErr := s.clusterAdmin.ListTopics()
	if listErr != nil {
		return metrics.ResourceMetrics(), listErr
	}

	matchedTopics := map[string]sarama.TopicDetail{}
	for t, d := range allTopics {
		if s.topicFilter.MatchString(t) {
			matchedTopics[t] = d
		}
	}
	scrapeErrors := scrapererror.ScrapeErrors{}
	// partitionIds in matchedTopics
	topicPartitions := map[string][]int32{}
	// currentOffset for each partition in matchedTopics
	topicPartitionOffset := map[string]map[int32]int64{}
	for topic := range matchedTopics {
		topicPartitionOffset[topic] = map[int32]int64{}
		partitions, err := s.client.Partitions(topic)
		if err != nil {
			scrapeErrors.Add(err)
			continue
		}
		for _, p := range partitions {
			o, err := s.client.GetOffset(topic, p, sarama.OffsetNewest)
			if err != nil {
				scrapeErrors.Add(err)
				continue
			}
			topicPartitions[topic] = append(topicPartitions[topic], p)
			topicPartitionOffset[topic][p] = o
		}
	}
	consumerGroups, listErr := s.clusterAdmin.DescribeConsumerGroups(matchedGrpIds)
	if listErr != nil {
		return metrics.ResourceMetrics(), listErr
	}
	for _, group := range consumerGroups {
		grpMetrics := metrics.WithLabels(map[string]string{metadata.L.Group: group.GroupId})
		grpMetrics.AddGaugeDataPoint(metadata.M.KafkaConsumerGroupMembers.Name(), int64(len(group.Members)))
		groupOffsetFetchResponse, err := s.clusterAdmin.ListConsumerGroupOffsets(group.GroupId, topicPartitions)
		if err != nil {
			scrapeErrors.Add(err)
			continue
		}
		for topic, partitions := range groupOffsetFetchResponse.Blocks {
			// tracking matchedTopics consumed by this group
			// by checking if any of the blocks has an offset
			isConsumed := false
			for _, block := range partitions {
				if block.Offset != -1 {
					isConsumed = true
					break
				}
			}
			grpTopicMetrics := grpMetrics.WithLabels(map[string]string{metadata.L.Topic: topic})
			if isConsumed {
				var lagSum int64 = 0
				var offsetSum int64 = 0
				for partition, block := range partitions {
					grpPartitionMetrics := grpTopicMetrics.WithLabels(map[string]string{metadata.L.Partition: string(partition)})
					consumerOffset := block.Offset
					offsetSum += consumerOffset
					grpPartitionMetrics.AddGaugeDataPoint(metadata.M.KafkaConsumerGroupOffset.Name(), consumerOffset)
					// default -1 to indicate no lag measured.
					var consumerLag int64 = -1
					if partitionOffset, ok := topicPartitionOffset[topic][partition]; ok {
						// only consider partitions with an offset
						if block.Offset != -1 {
							consumerLag = partitionOffset - consumerOffset
							lagSum += consumerLag
						}
					}
					grpPartitionMetrics.AddGaugeDataPoint(metadata.M.KafkaConsumerGroupLag.Name(), consumerLag)
				}
				grpTopicMetrics.AddGaugeDataPoint(metadata.M.KafkaConsumerGroupOffsetSum.Name(), offsetSum)
				grpTopicMetrics.AddGaugeDataPoint(metadata.M.KafkaConsumerGroupLagSum.Name(), lagSum)
			}
		}
	}

	return metrics.ResourceMetrics(), scrapeErrors.Combine()
}

func createConsumerScraper(_ context.Context, config Config, saramaConfig *sarama.Config, logger *zap.Logger) (scraperhelper.ResourceMetricsScraper, error) {
	groupFilter, err := regexp.Compile(config.GroupMatch)
	if err != nil {
		return nil, fmt.Errorf("failed to compile group_match: %w", err)
	}
	topicFilter, err := regexp.Compile(config.TopicMatch)
	if err != nil {
		return nil, fmt.Errorf("failed to compile topic filter: %w", err)
	}
	s := consumerScraper{
		logger:       logger,
		groupFilter:  groupFilter,
		topicFilter:  topicFilter,
		config:       config,
		saramaConfig: saramaConfig,
	}
	return scraperhelper.NewResourceMetricsScraper(
		s.Name(),
		s.scrape,
		scraperhelper.WithShutdown(s.shutdown),
		scraperhelper.WithStart(s.start),
	), nil
}
