// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkametricsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkametricsreceiver"

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/kafka"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkametricsreceiver/internal/metadata"
)

type consumerScraperFranz struct {
	adm *kadm.Client
	cl  *kgo.Client

	settings    receiver.Settings
	groupFilter *regexp.Regexp
	topicFilter *regexp.Regexp
	config      Config
	mb          *metadata.MetricsBuilder
}

func (s *consumerScraperFranz) start(_ context.Context, _ component.Host) error {
	s.mb = metadata.NewMetricsBuilder(s.config.MetricsBuilderConfig, s.settings)
	return nil
}

func (s *consumerScraperFranz) shutdown(_ context.Context) error {
	if s.adm != nil {
		s.adm.Close()
		s.adm = nil
	}
	if s.cl != nil {
		s.cl.Close()
		s.cl = nil
	}
	return nil
}

func (s *consumerScraperFranz) ensureClients(ctx context.Context) error {
	if s.adm != nil && s.cl != nil {
		return nil
	}
	adm, cl, err := kafka.NewFranzClusterAdminClient(ctx, s.config.ClientConfig, s.settings.Logger)
	if err != nil {
		return fmt.Errorf("failed to create franz-go admin client: %w", err)
	}
	s.adm = adm
	s.cl = cl
	return nil
}

func (s *consumerScraperFranz) scrape(ctx context.Context) (pmetric.Metrics, error) {
	if err := s.ensureClients(ctx); err != nil {
		return pmetric.Metrics{}, err
	}

	var scrapeErr error

	// 1) list & filter groups
	lgs, err := s.adm.ListGroups(ctx)
	if err != nil {
		return pmetric.Metrics{}, fmt.Errorf("franz-go: ListGroups failed: %w", err)
	}
	var matchedGrpIDs []string
	for _, g := range lgs.Sorted() {
		if s.groupFilter.MatchString(g.Group) {
			matchedGrpIDs = append(matchedGrpIDs, g.Group)
		}
	}

	// 2) list & filter topics
	td, err := s.adm.ListTopics(ctx) // non-internal only, same as sarama default
	if err != nil {
		return pmetric.Metrics{}, fmt.Errorf("franz-go: ListTopics failed: %w", err)
	}
	var matchedTopics []string
	for t := range td {
		if s.topicFilter.MatchString(t) {
			matchedTopics = append(matchedTopics, t)
		}
	}

	// 3) compute partition list + end offsets for matched topics
	endOffsets, err := s.adm.ListEndOffsets(ctx, matchedTopics...)
	if err != nil {
		return pmetric.Metrics{}, fmt.Errorf("franz-go: ListEndOffsets failed: %w", err)
	}
	// Build helpers equivalent to Sarama path
	topicPartitions := make(map[string][]int32, len(matchedTopics))
	topicPartitionOffset := make(map[string]map[int32]int64, len(matchedTopics))
	endOffsets.Each(func(lo kadm.ListedOffset) {
		// lo.Topic, lo.Partition, lo.Offset
		if _, ok := topicPartitions[lo.Topic]; !ok {
			topicPartitions[lo.Topic] = []int32{}
		}
		if _, ok := topicPartitionOffset[lo.Topic]; !ok {
			topicPartitionOffset[lo.Topic] = map[int32]int64{}
		}
		topicPartitions[lo.Topic] = append(topicPartitions[lo.Topic], lo.Partition)
		topicPartitionOffset[lo.Topic][lo.Partition] = lo.Offset
	})

	// 4) describe groups for member counts
	dgs, err := s.adm.DescribeGroups(ctx, matchedGrpIDs...)
	if err != nil {
		return pmetric.Metrics{}, fmt.Errorf("franz-go: DescribeGroups failed: %w", err)
	}

	now := pcommon.NewTimestampFromTime(time.Now())

	// 5) per group: fetch committed offsets for matched topics and compute metrics
	gs := dgs.Sorted()
	for i := range gs {
		g := &gs[i]
		grpID := g.Group
		s.mb.RecordKafkaConsumerGroupMembersDataPoint(now, int64(len(g.Members)), grpID)

		offsets, ferr := s.adm.FetchOffsetsForTopics(ctx, grpID, matchedTopics...)
		if ferr != nil {
			scrapeErr = multierr.Append(scrapeErr, fmt.Errorf("franz-go: FetchOffsetsForTopics(%s) failed: %w", grpID, ferr))
			continue
		}

		for topic, parts := range topicPartitions {
			isConsumed := false
			var lagSum int64
			var offsetSum int64

			for _, p := range parts {
				consumerOffset := int64(-1)
				if or, ok := offsets.Lookup(topic, p); ok && or.Err == nil {
					consumerOffset = or.At
				}
				offsetSum += consumerOffset
				s.mb.RecordKafkaConsumerGroupOffsetDataPoint(now, consumerOffset, grpID, topic, int64(p))

				var consumerLag int64 = -1
				if consumerOffset != -1 {
					isConsumed = true
					if end, ok := topicPartitionOffset[topic][p]; ok {
						consumerLag = end - consumerOffset
						lagSum += consumerLag
					}
				}
				s.mb.RecordKafkaConsumerGroupLagDataPoint(now, consumerLag, grpID, topic, int64(p))
			}

			if isConsumed {
				s.mb.RecordKafkaConsumerGroupOffsetSumDataPoint(now, offsetSum, grpID, topic)
				s.mb.RecordKafkaConsumerGroupLagSumDataPoint(now, lagSum, grpID, topic)
			}
		}
	}

	rb := s.mb.NewResourceBuilder()
	rb.SetKafkaClusterAlias(s.config.ClusterAlias)

	return s.mb.Emit(metadata.WithResource(rb.Emit())), scrapeErr
}

// Factory helper for franz-go path (selected under the feature gate later).
func createConsumerScraperFranz(_ context.Context, cfg Config, settings receiver.Settings) (scraper.Metrics, error) {
	groupFilter, err := regexp.Compile(cfg.GroupMatch)
	if err != nil {
		return nil, fmt.Errorf("failed to compile group_match: %w", err)
	}
	topicFilter, err := regexp.Compile(cfg.TopicMatch)
	if err != nil {
		return nil, fmt.Errorf("failed to compile topic filter: %w", err)
	}
	s := &consumerScraperFranz{
		settings:    settings,
		groupFilter: groupFilter,
		topicFilter: topicFilter,
		config:      cfg,
	}
	return scraper.NewMetrics(
		s.scrape,
		scraper.WithStart(s.start),
		scraper.WithShutdown(s.shutdown),
	)
}
