// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkametricsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkametricsreceiver"

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/IBM/sarama"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper"
	"go.opentelemetry.io/collector/scraper/scrapererror"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkametricsreceiver/internal/metadata"
)

type consumerScraper struct {
	kafkaScraper
	groupFilter *regexp.Regexp
	topicFilter *regexp.Regexp
}

func newConsumerScraper(
	cfg Config,
	settings receiver.Settings,
	newSaramaClient newSaramaClientFunc,
	newSaramaClusterAdminClient newSaramaClusterAdminClientFunc,
) (scraper.Metrics, error) {
	groupFilter, err := regexp.Compile(cfg.GroupMatch)
	if err != nil {
		return nil, fmt.Errorf("failed to compile group filter: %w", err)
	}
	topicFilter, err := regexp.Compile(cfg.TopicMatch)
	if err != nil {
		return nil, fmt.Errorf("failed to compile topic filter: %w", err)
	}
	s := consumerScraper{
		kafkaScraper: kafkaScraper{
			settings:                    settings,
			config:                      cfg,
			mb:                          metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, settings),
			newSaramaClient:             newSaramaClient,
			newSaramaClusterAdminClient: newSaramaClusterAdminClient,
		},
		groupFilter: groupFilter,
		topicFilter: topicFilter,
	}
	return scraper.NewMetrics(
		s.scrape,
		scraper.WithStart(s.start),
		scraper.WithShutdown(s.shutdown),
	)
}

func (s *consumerScraper) scrape(context.Context) (pmetric.Metrics, error) {
	cgs, listErr := s.adminClient.ListConsumerGroups()
	if listErr != nil {
		return pmetric.Metrics{}, listErr
	}

	var matchedGrpIDs []string
	for grpID := range cgs {
		if s.groupFilter.MatchString(grpID) {
			matchedGrpIDs = append(matchedGrpIDs, grpID)
		}
	}

	allTopics, listErr := s.adminClient.ListTopics()
	if listErr != nil {
		return pmetric.Metrics{}, listErr
	}

	matchedTopics := map[string]sarama.TopicDetail{}
	for t, d := range allTopics {
		if s.topicFilter.MatchString(t) {
			matchedTopics[t] = d
		}
	}
	// partitionIds in matchedTopics
	topicPartitions := map[string][]int32{}
	// currentOffset for each partition in matchedTopics
	topicPartitionOffset := map[string]map[int32]int64{}

	scrapeErrors := scrapererror.ScrapeErrors{}
	for topic := range matchedTopics {
		topicPartitionOffset[topic] = map[int32]int64{}
		partitions, err := s.client.Partitions(topic)
		if err != nil {
			scrapeErrors.AddPartial(1, err)
			continue
		}
		for _, p := range partitions {
			var offset int64
			offset, err = s.client.GetOffset(topic, p, sarama.OffsetNewest)
			if err != nil {
				scrapeErrors.AddPartial(1, err)
				continue
			}
			topicPartitions[topic] = append(topicPartitions[topic], p)
			topicPartitionOffset[topic][p] = offset
		}
	}
	consumerGroups, listErr := s.adminClient.DescribeConsumerGroups(matchedGrpIDs)
	if listErr != nil {
		return pmetric.Metrics{}, listErr
	}

	now := pcommon.NewTimestampFromTime(time.Now())

	for _, group := range consumerGroups {
		s.mb.RecordKafkaConsumerGroupMembersDataPoint(now, int64(len(group.Members)), group.GroupId)

		groupOffsetFetchResponse, err := s.adminClient.ListConsumerGroupOffsets(group.GroupId, topicPartitions)
		if err != nil {
			scrapeErrors.AddPartial(1, err)
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
					s.mb.RecordKafkaConsumerGroupOffsetDataPoint(now, consumerOffset, group.GroupId, topic, int64(partition))

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

	rb := s.mb.NewResourceBuilder()
	rb.SetKafkaClusterAlias(s.config.ClusterAlias)

	return s.mb.Emit(metadata.WithResource(rb.Emit())), scrapeErrors.Combine()
}
