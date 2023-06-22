// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkametricsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkametricsreceiver"

import (
	"context"
	"fmt"

	"github.com/Shopify/sarama"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter"
)

const (
	brokersScraperName   = "brokers"
	topicsScraperName    = "topics"
	consumersScraperName = "consumers"
)

type createKafkaScraper func(context.Context, Config, *sarama.Config, receiver.CreateSettings) (scraperhelper.Scraper, error)

var (
	allScrapers = map[string]createKafkaScraper{
		brokersScraperName:   createBrokerScraper,
		topicsScraperName:    createTopicsScraper,
		consumersScraperName: createConsumerScraper,
	}
)

func logDeprecation(logger *zap.Logger) {
	logger.Warn("kafka.brokers attribute is deprecated and will be removed in a future release. Use kafka.brokers.count instead.")
}

var newMetricsReceiver = func(
	ctx context.Context,
	config Config,
	params receiver.CreateSettings,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {

	if config.Metrics.KafkaBrokers.Enabled {
		logDeprecation(params.Logger)
	}

	sc := sarama.NewConfig()
	sc.ClientID = config.ClientID
	if config.ProtocolVersion != "" {
		version, err := sarama.ParseKafkaVersion(config.ProtocolVersion)
		if err != nil {
			return nil, err
		}
		sc.Version = version
	}
	if err := kafkaexporter.ConfigureAuthentication(config.Authentication, sc); err != nil {
		return nil, err
	}
	scraperControllerOptions := make([]scraperhelper.ScraperControllerOption, 0, len(config.Scrapers))
	for _, scraper := range config.Scrapers {
		if s, ok := allScrapers[scraper]; ok {
			s, err := s(ctx, config, sc, params)
			if err != nil {
				return nil, err
			}
			scraperControllerOptions = append(scraperControllerOptions, scraperhelper.AddScraper(s))
			continue
		}
		return nil, fmt.Errorf("no scraper found for key: %s", scraper)
	}

	return scraperhelper.NewScraperControllerReceiver(
		&config.ScraperControllerSettings,
		params,
		consumer,
		scraperControllerOptions...,
	)
}
