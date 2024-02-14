// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkametricsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkametricsreceiver"

import (
	"context"
	"fmt"

	"github.com/IBM/sarama"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/kafka"
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

var newMetricsReceiver = func(
	ctx context.Context,
	config Config,
	params receiver.CreateSettings,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	sc := sarama.NewConfig()
	sc.ClientID = config.ClientID
	if config.ResolveCanonicalBootstrapServersOnly {
		sc.Net.ResolveCanonicalBootstrapServers = true
	}
	if config.ProtocolVersion != "" {
		version, err := sarama.ParseKafkaVersion(config.ProtocolVersion)
		if err != nil {
			return nil, err
		}
		sc.Version = version
	}
	if err := kafka.ConfigureAuthentication(config.Authentication, sc); err != nil {
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
