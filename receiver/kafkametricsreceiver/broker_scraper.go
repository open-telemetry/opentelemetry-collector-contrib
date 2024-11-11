// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkametricsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkametricsreceiver"

import (
	"context"
	"fmt"
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

type brokerScraper struct {
	client       sarama.Client
	settings     receiver.Settings
	config       Config
	saramaConfig *sarama.Config
	clusterAdmin sarama.ClusterAdmin
	mb           *metadata.MetricsBuilder
}

const (
	logRetentionHours = "log.retention.hours"
)

func (s *brokerScraper) start(_ context.Context, _ component.Host) error {
	s.mb = metadata.NewMetricsBuilder(s.config.MetricsBuilderConfig, s.settings)
	return nil
}

func (s *brokerScraper) shutdown(context.Context) error {
	if s.client != nil && !s.client.Closed() {
		return s.client.Close()
	}
	return nil
}

func (s *brokerScraper) scrape(context.Context) (pmetric.Metrics, error) {
	var scrapeErrors = scrapererror.ScrapeErrors{}

	if s.client == nil {
		client, err := newSaramaClient(s.config.Brokers, s.saramaConfig)
		if err != nil {
			return pmetric.Metrics{}, fmt.Errorf("failed to create client in brokers scraper: %w", err)
		}
		s.client = client
	}

	now := pcommon.NewTimestampFromTime(time.Now())
	rb := s.mb.NewResourceBuilder()
	rb.SetKafkaClusterAlias(s.config.ClusterAlias)

	brokers := s.client.Brokers()
	s.mb.RecordKafkaBrokersDataPoint(now, int64(len(brokers)))
	if !s.config.Metrics.KafkaBrokerLogRetentionPeriod.Enabled {
		return s.mb.Emit(metadata.WithResource(rb.Emit())), scrapeErrors.Combine()
	}

	if s.clusterAdmin == nil {
		admin, err := newClusterAdmin(s.config.Brokers, s.saramaConfig)
		if err != nil {
			s.settings.Logger.Error("Error creating kafka client with admin privileges", zap.Error(err))
			return s.mb.Emit(metadata.WithResource(rb.Emit())), scrapeErrors.Combine()
		}
		s.clusterAdmin = admin
	}

	for _, broker := range brokers {
		id := strconv.Itoa(int(broker.ID()))
		configEntries, err := s.clusterAdmin.DescribeConfig(sarama.ConfigResource{
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

func createBrokerScraper(_ context.Context, cfg Config, saramaConfig *sarama.Config,
	settings receiver.Settings) (scraperhelper.Scraper, error) {
	s := brokerScraper{
		settings:     settings,
		config:       cfg,
		saramaConfig: saramaConfig,
	}
	return scraperhelper.NewScraper(
		brokersScraperType,
		s.scrape,
		scraperhelper.WithStart(s.start),
		scraperhelper.WithShutdown(s.shutdown),
	)
}
