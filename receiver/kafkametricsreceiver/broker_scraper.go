// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkametricsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkametricsreceiver"

import (
	"context"
	"fmt"
	"time"

	"github.com/Shopify/sarama"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkametricsreceiver/internal/metadata"
)

type brokerScraper struct {
	client       sarama.Client
	settings     receiver.CreateSettings
	config       Config
	saramaConfig *sarama.Config
	mb           *metadata.MetricsBuilder
}

func (s *brokerScraper) Name() string {
	return brokersScraperName
}

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
	if s.client == nil {
		client, err := newSaramaClient(s.config.Brokers, s.saramaConfig)
		if err != nil {
			return pmetric.Metrics{}, fmt.Errorf("failed to create client in brokers scraper: %w", err)
		}
		s.client = client
	}

	brokers := s.client.Brokers()

	s.mb.RecordKafkaBrokersDataPoint(pcommon.NewTimestampFromTime(time.Now()), int64(len(brokers)))

	return s.mb.Emit(), nil
}

func createBrokerScraper(_ context.Context, cfg Config, saramaConfig *sarama.Config,
	settings receiver.CreateSettings) (scraperhelper.Scraper, error) {
	s := brokerScraper{
		settings:     settings,
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
