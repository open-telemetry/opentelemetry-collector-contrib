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

package kafkametricsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkametricsreceiver"

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/Shopify/sarama"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
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
	return topicsScraperName
}

func (s *topicScraper) setupClient() error {
	if s.client != nil {
		return nil
	}
	client, err := newSaramaClient(s.config.Brokers, s.saramaConfig)
	if err != nil {
		return fmt.Errorf("failed to create client in topics scraper: %w", err)
	}
	s.client = client
	return nil
}

func (s *topicScraper) shutdown(context.Context) error {
	if !s.client.Closed() {
		return s.client.Close()
	}
	return nil
}

func (s *topicScraper) scrape(context.Context) (pmetric.Metrics, error) {
	if err := s.setupClient(); err != nil {
		return pmetric.Metrics{}, err
	}

	topics, err := s.client.Topics()
	if err != nil {
		s.logger.Error("Error fetching cluster topics ", zap.Error(err))
		return pmetric.Metrics{}, err
	}

	var scrapeErrors = scrapererror.ScrapeErrors{}

	now := pcommon.NewTimestampFromTime(time.Now())
	md := pmetric.NewMetrics()
	ilm := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty()
	ilm.Scope().SetName(instrumentationLibName)
	for _, topic := range topics {
		if !s.topicFilter.MatchString(topic) {
			continue
		}
		partitions, err := s.client.Partitions(topic)
		if err != nil {
			scrapeErrors.Add(err)
			continue
		}
		labels := pcommon.NewMap()
		labels.UpsertString(metadata.A.Topic, topic)
		addIntGauge(ilm.Metrics(), metadata.M.KafkaTopicPartitions.Name(), now, labels, int64(len(partitions)))
		for _, partition := range partitions {
			labels.UpsertInt(metadata.A.Partition, int64(partition))
			currentOffset, err := s.client.GetOffset(topic, partition, sarama.OffsetNewest)
			if err != nil {
				scrapeErrors.AddPartial(1, err)
			} else {
				addIntGauge(ilm.Metrics(), metadata.M.KafkaPartitionCurrentOffset.Name(), now, labels, currentOffset)
			}
			oldestOffset, err := s.client.GetOffset(topic, partition, sarama.OffsetOldest)
			if err != nil {
				scrapeErrors.AddPartial(1, err)
			} else {
				addIntGauge(ilm.Metrics(), metadata.M.KafkaPartitionOldestOffset.Name(), now, labels, oldestOffset)
			}
			replicas, err := s.client.Replicas(topic, partition)
			if err != nil {
				scrapeErrors.AddPartial(1, err)
			} else {
				addIntGauge(ilm.Metrics(), metadata.M.KafkaPartitionReplicas.Name(), now, labels, int64(len(replicas)))
			}
			replicasInSync, err := s.client.InSyncReplicas(topic, partition)
			if err != nil {
				scrapeErrors.AddPartial(1, err)
			} else {
				addIntGauge(ilm.Metrics(), metadata.M.KafkaPartitionReplicasInSync.Name(), now, labels, int64(len(replicasInSync)))
			}
		}
	}
	return md, scrapeErrors.Combine()
}

func createTopicsScraper(_ context.Context, cfg Config, saramaConfig *sarama.Config, logger *zap.Logger) (scraperhelper.Scraper, error) {
	topicFilter, err := regexp.Compile(cfg.TopicMatch)
	if err != nil {
		return nil, fmt.Errorf("failed to compile topic filter: %w", err)
	}
	s := topicScraper{
		logger:       logger,
		topicFilter:  topicFilter,
		saramaConfig: saramaConfig,
		config:       cfg,
	}
	return scraperhelper.NewScraper(
		s.Name(),
		s.scrape,
		scraperhelper.WithShutdown(s.shutdown),
	)
}

func addIntGauge(ms pmetric.MetricSlice, name string, now pcommon.Timestamp, labels pcommon.Map, value int64) {
	m := ms.AppendEmpty()
	m.SetName(name)
	m.SetDataType(pmetric.MetricDataTypeGauge)
	dp := m.Gauge().DataPoints().AppendEmpty()
	dp.SetTimestamp(now)
	dp.SetIntVal(value)
	labels.CopyTo(dp.Attributes())
}
