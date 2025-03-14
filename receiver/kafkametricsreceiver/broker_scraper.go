// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkametricsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkametricsreceiver"

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/IBM/sarama"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper"
	"go.opentelemetry.io/collector/scraper/scrapererror"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkametricsreceiver/internal/metadata"
)

const (
	logRetentionHours = "log.retention.hours"
)

type brokerScraper struct {
	kafkaScraper
}

func newBrokerScraper(
	cfg Config,
	settings receiver.Settings,
	newSaramaClient newSaramaClientFunc,
	newSaramaClusterAdminClient newSaramaClusterAdminClientFunc,
) (scraper.Metrics, error) {
	s := &brokerScraper{
		kafkaScraper: kafkaScraper{
			settings:                    settings,
			config:                      cfg,
			mb:                          metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, settings),
			newSaramaClient:             newSaramaClient,
			newSaramaClusterAdminClient: newSaramaClusterAdminClient,
		},
	}
	return scraper.NewMetrics(s.scrape, scraper.WithStart(s.start), scraper.WithShutdown(s.shutdown))
}

func (s *brokerScraper) scrape(context.Context) (pmetric.Metrics, error) {
	scrapeErrors := scrapererror.ScrapeErrors{}
	now := pcommon.NewTimestampFromTime(time.Now())
	rb := s.mb.NewResourceBuilder()
	rb.SetKafkaClusterAlias(s.config.ClusterAlias)

	brokers := s.client.Brokers()
	s.mb.RecordKafkaBrokersDataPoint(now, int64(len(brokers)))
	if !s.config.Metrics.KafkaBrokerLogRetentionPeriod.Enabled {
		return s.mb.Emit(metadata.WithResource(rb.Emit())), scrapeErrors.Combine()
	}

	for _, broker := range brokers {
		id := strconv.Itoa(int(broker.ID()))
		configEntries, err := s.adminClient.DescribeConfig(sarama.ConfigResource{
			Type:        sarama.BrokerResource,
			Name:        id,
			ConfigNames: []string{logRetentionHours},
		})
		if err != nil {
			scrapeErrors.AddPartial(1, fmt.Errorf("failed to fetch the `%s` metric from %s: %w", logRetentionHours, broker.Addr(), err))
			continue
		}
		for _, config := range configEntries {
			if config.Name != logRetentionHours {
				continue
			}
			val, err := strconv.Atoi(config.Value)
			if err != nil {
				scrapeErrors.AddPartial(1, fmt.Errorf("error converting `%s` for %s: value was %s", logRetentionHours, broker.Addr(), config.Value))
			}
			s.mb.RecordKafkaBrokerLogRetentionPeriodDataPoint(now, int64(val*3600), id)
		}
	}

	return s.mb.Emit(metadata.WithResource(rb.Emit())), scrapeErrors.Combine()
}
