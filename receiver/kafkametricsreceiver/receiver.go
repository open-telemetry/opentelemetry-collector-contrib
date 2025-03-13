// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkametricsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkametricsreceiver"

import (
	"context"
	"fmt"

	"github.com/IBM/sarama"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/kafka"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkametricsreceiver/internal/metadata"
)

type createKafkaScraper func(context.Context, Config, *sarama.Config, receiver.Settings) (scraper.Metrics, error)

var (
	brokersScraperType   = component.MustNewType("brokers")
	topicsScraperType    = component.MustNewType("topics")
	consumersScraperType = component.MustNewType("consumers")
	allScrapers          = map[string]createKafkaScraper{
		brokersScraperType.String():   createBrokerScraper,
		topicsScraperType.String():    createTopicsScraper,
		consumersScraperType.String(): createConsumerScraper,
	}
)

var newMetricsReceiver = func(
	ctx context.Context,
	config Config,
	params receiver.Settings,
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

	if config.RefreshFrequency != 0 {
		sc.Metadata.RefreshFrequency = config.RefreshFrequency
	}

	if err := kafka.ConfigureSaramaAuthentication(ctx, config.Authentication, sc); err != nil {
		return nil, err
	}
	scraperControllerOptions := make([]scraperhelper.ControllerOption, 0, len(config.Scrapers))
	for _, scraper := range config.Scrapers {
		if s, ok := allScrapers[scraper]; ok {
			s, err := s(ctx, config, sc, params)
			if err != nil {
				return nil, err
			}
			scraperControllerOptions = append(scraperControllerOptions, scraperhelper.AddScraper(metadata.Type, s))
			continue
		}
		return nil, fmt.Errorf("no scraper found for key: %s", scraper)
	}

	return scraperhelper.NewMetricsController(
		&config.ControllerConfig,
		params,
		consumer,
		scraperControllerOptions...,
	)
}
