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
	"go.opentelemetry.io/collector/receiver/scraperhelper"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkametricsreceiver/internal/metadata"
)

type consumerScraper struct {
	client       sarama.Client
	settings     receiver.CreateSettings
	groupFilter  *regexp.Regexp
	topicFilter  *regexp.Regexp
	clusterAdmin sarama.ClusterAdmin
	saramaConfig *sarama.Config
	config       Config
	mb           *metadata.MetricsBuilder
}

func (s *consumerScraper) Name() string {
	return consumersScraperName
}

func (s *consumerScraper) start(_ context.Context, _ component.Host) error {
	s.mb = metadata.NewMetricsBuilder(s.config.MetricsBuilderConfig, s.settings)
	return nil
}

func (s *consumerScraper) shutdown(_ context.Context) error {
	if s.client != nil && !s.client.Closed() {
		return s.client.Close()
	}
	return nil
}

func (s *consumerScraper) scrape(context.Context) (pmetric.Metrics, error) {
	if s.client == nil {
		client, err := newSaramaClient(s.config.Brokers, s.saramaConfig)
		if err != nil {
			return pmetric.Metrics{}, fmt.Errorf("failed to create client in consumer scraper: %w", err)
		}
		clusterAdmin, err := newClusterAdmin(s.config.Brokers, s.saramaConfig)
		if err != nil {
			if client != nil {
				_ = client.Close()
			}
			return pmetric.Metrics{}, fmt.Errorf("failed to create cluster admin in consumer scraper: %w", err)
		}
		s.client = client
		s.clusterAdmin = clusterAdmin
	}

	cgs, listErr := s.clusterAdmin.ListConsumerGroups()
	if listErr != nil {
		return pmetric.Metrics{}, listErr
	}

	var matchedGrpIds []string
	for grpID := range cgs {
		if s.groupFilter.MatchString(grpID) {
			matchedGrpIds = append(matchedGrpIds, grpID)
		}
	}

	allTopics, listErr := s.clusterAdmin.ListTopics()
	if listErr != nil {
		return pmetric.Metrics{}, listErr
	}

	matchedTopics := map[string]sarama.TopicDetail{}
	for t, d := range allTopics {
		if s.topicFilter.MatchString(t) {
			matchedTopics[t] = d
		}
	}
	var scrapeError error
	// partitionIds in matchedTopics
	topicPartitions := map[string][]int32{}
	// currentOffset for each partition in matchedTopics
	topicPartitionOffset := map[string]map[int32]int64{}
	for topic := range matchedTopics {
		topicPartitionOffset[topic] = map[int32]int64{}
		partitions, err := s.client.Partitions(topic)
		if err != nil {
			scrapeError = multierr.Append(scrapeError, err)
			continue
		}
		for _, p := range partitions {
			var offset int64
			offset, err = s.client.GetOffset(topic, p, sarama.OffsetNewest)
			if err != nil {
				scrapeError = multierr.Append(scrapeError, err)
				continue
			}
			topicPartitions[topic] = append(topicPartitions[topic], p)
			topicPartitionOffset[topic][p] = offset
		}
	}
	consumerGroups, listErr := s.clusterAdmin.DescribeConsumerGroups(matchedGrpIds)
	if listErr != nil {
		return pmetric.Metrics{}, listErr
	}

	now := pcommon.NewTimestampFromTime(time.Now())

	for _, group := range consumerGroups {
		s.mb.RecordKafkaConsumerGroupMembersDataPoint(now, int64(len(group.Members)), group.GroupId)

		groupOffsetFetchResponse, err := s.clusterAdmin.ListConsumerGroupOffsets(group.GroupId, topicPartitions)
		if err != nil {
			scrapeError = multierr.Append(scrapeError, err)
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
			if isConsumed {
				var lagSum int64
				var offsetSum int64
				for partition, block := range partitions {
					consumerOffset := block.Offset
					offsetSum += consumerOffset
					s.mb.RecordKafkaConsumerGroupOffsetDataPoint(now, offsetSum, group.GroupId, topic, int64(partition))

					// default -1 to indicate no lag measured.
					var consumerLag int64 = -1
					if partitionOffset, ok := topicPartitionOffset[topic][partition]; ok {
						// only consider partitions with an offset
						if block.Offset != -1 {
							consumerLag = partitionOffset - consumerOffset
							lagSum += consumerLag
						}
					}
					s.mb.RecordKafkaConsumerGroupLagDataPoint(now, consumerLag, group.GroupId, topic, int64(partition))
				}
				s.mb.RecordKafkaConsumerGroupOffsetSumDataPoint(now, offsetSum, group.GroupId, topic)
				s.mb.RecordKafkaConsumerGroupLagSumDataPoint(now, lagSum, group.GroupId, topic)
			}
		}
	}

	return s.mb.Emit(), scrapeError
}

func createConsumerScraper(_ context.Context, cfg Config, saramaConfig *sarama.Config,
	settings receiver.CreateSettings) (scraperhelper.Scraper, error) {
	groupFilter, err := regexp.Compile(cfg.GroupMatch)
	if err != nil {
		return nil, fmt.Errorf("failed to compile group_match: %w", err)
	}
	topicFilter, err := regexp.Compile(cfg.TopicMatch)
	if err != nil {
		return nil, fmt.Errorf("failed to compile topic filter: %w", err)
	}
	s := consumerScraper{
		settings:     settings,
		groupFilter:  groupFilter,
		topicFilter:  topicFilter,
		config:       cfg,
		saramaConfig: saramaConfig,
	}
	return scraperhelper.NewScraper(
		s.Name(),
		s.scrape,
		scraperhelper.WithStart(s.start),
		scraperhelper.WithShutdown(s.shutdown),
	)
}
