// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkametricsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkametricsreceiver"

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"time"

	"github.com/IBM/sarama"
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
	clusterAdmin sarama.ClusterAdmin
	settings     receiver.Settings
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

	s.scrapeTopicConfigs(now, scrapeErrors)
	for _, topic := range topics {
		if !s.topicFilter.MatchString(topic) {
			continue
		}
		partitions, err := s.client.Partitions(topic)
		if err != nil {
			scrapeErrors.Add(err)
			continue
		}

		s.mb.RecordKafkaTopicPartitionsDataPoint(now, int64(len(partitions)), topic, s.config.ClusterAlias)
		for _, partition := range partitions {
			currentOffset, err := s.client.GetOffset(topic, partition, sarama.OffsetNewest)
			if err != nil {
				scrapeErrors.AddPartial(1, err)
			} else {
				s.mb.RecordKafkaPartitionCurrentOffsetDataPoint(now, currentOffset, topic, int64(partition), s.config.ClusterAlias)
			}
			oldestOffset, err := s.client.GetOffset(topic, partition, sarama.OffsetOldest)
			if err != nil {
				scrapeErrors.AddPartial(1, err)
			} else {
				s.mb.RecordKafkaPartitionOldestOffsetDataPoint(now, oldestOffset, topic, int64(partition), s.config.ClusterAlias)
			}
			replicas, err := s.client.Replicas(topic, partition)
			if err != nil {
				scrapeErrors.AddPartial(1, err)
			} else {
				s.mb.RecordKafkaPartitionReplicasDataPoint(now, int64(len(replicas)), topic, int64(partition), s.config.ClusterAlias)
			}
			replicasInSync, err := s.client.InSyncReplicas(topic, partition)
			if err != nil {
				scrapeErrors.AddPartial(1, err)
			} else {
				s.mb.RecordKafkaPartitionReplicasInSyncDataPoint(now, int64(len(replicasInSync)), topic, int64(partition), s.config.ClusterAlias)
			}
		}
	}
	return s.mb.Emit(), scrapeErrors.Combine()
}

func (s *topicScraper) scrapeTopicConfigs(now pcommon.Timestamp, errors scrapererror.ScrapeErrors) {
	if s.clusterAdmin == nil {
		admin, err := newClusterAdmin(s.config.Brokers, s.saramaConfig)
		if err != nil {
			s.settings.Logger.Error("Error creating kafka client with admin priviledges", zap.Error(err))
			return
		}
		s.clusterAdmin = admin
	}
	topics, err := s.clusterAdmin.ListTopics()
	if err != nil {
		s.settings.Logger.Error("Error fetching cluster topic configurations", zap.Error(err))
		return
	}

	for name, topic := range topics {
		s.mb.RecordKafkaTopicReplicationFactorDataPoint(now, int64(topic.ReplicationFactor), name, s.config.ClusterAlias)
		configEntries, _ := s.clusterAdmin.DescribeConfig(sarama.ConfigResource{
			Type:        sarama.TopicResource,
			Name:        name,
			ConfigNames: []string{"min.insync.replicas", "retention.ms", "retention.bytes"},
		})

		for _, config := range configEntries {
			switch config.Name {
			case "min.insync.replicas":
				if val, err := strconv.Atoi(config.Value); err == nil {
					s.mb.RecordKafkaTopicMinInsyncReplicasDataPoint(now, int64(val), name, s.config.ClusterAlias)
				} else {
					errors.AddPartial(1, err)
				}
			case "retention.ms":
				if val, err := strconv.Atoi(config.Value); err == nil {
					s.mb.RecordKafkaTopicLogRetentionMsDataPoint(now, int64(val), name, s.config.ClusterAlias)
				} else {
					errors.AddPartial(1, err)
				}
			case "retention.bytes":
				if val, err := strconv.Atoi(config.Value); err == nil {
					s.mb.RecordKafkaTopicLogRetentionBytesDataPoint(now, int64(val), name, s.config.ClusterAlias)
				} else {
					errors.AddPartial(1, err)
				}
			}
		}
	}
}

func createTopicsScraper(_ context.Context, cfg Config, saramaConfig *sarama.Config, settings receiver.Settings) (scraperhelper.Scraper, error) {
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
