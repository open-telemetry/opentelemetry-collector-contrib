// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkametricsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkametricsreceiver"

import (
	"context"
	"fmt"

	"github.com/IBM/sarama"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/kafka/configkafka"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkametricsreceiver/internal/metadata"
)

const (
	brokersScraperType   = "brokers"
	topicsScraperType    = "topics"
	consumersScraperType = "consumers"
)

type (
	newSaramaClientFunc             func(context.Context, configkafka.ClientConfig) (sarama.Client, error)
	newSaramaClusterAdminClientFunc func(sarama.Client) (sarama.ClusterAdmin, error)
)

var newMetricsReceiver = func(
	config Config,
	settings receiver.Settings,
	consumer consumer.Metrics,
	newSaramaClient newSaramaClientFunc,
	newSaramaClusterAdminClient newSaramaClusterAdminClientFunc,
) (receiver.Metrics, error) {
	scraperControllerOptions := make([]scraperhelper.ControllerOption, 0, len(config.Scrapers))
	for _, scraperType := range config.Scrapers {
		var scraper scraper.Metrics
		var err error
		switch scraperType {
		case brokersScraperType:
			scraper, err = newBrokerScraper(config, settings, newSaramaClient, newSaramaClusterAdminClient)
		case topicsScraperType:
			scraper, err = newTopicsScraper(config, settings, newSaramaClient, newSaramaClusterAdminClient)
		case consumersScraperType:
			scraper, err = newConsumerScraper(config, settings, newSaramaClient, newSaramaClusterAdminClient)
		default:
			return nil, fmt.Errorf("unknown scraper type %q", scraperType)
		}
		if err != nil {
			return nil, fmt.Errorf("error creating scraper type %q: %w", scraperType, err)
		}

		scraperControllerOptions = append(scraperControllerOptions,
			scraperhelper.AddScraper(metadata.Type, scraper),
		)
	}

	return scraperhelper.NewMetricsController(
		&config.ControllerConfig,
		settings,
		consumer,
		scraperControllerOptions...,
	)
}
