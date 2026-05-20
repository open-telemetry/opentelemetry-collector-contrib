// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkametricsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkametricsreceiver"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkametricsreceiver/internal/metadata"
)

type createKafkaScraper func(context.Context, Config, receiver.Settings) (scraper.Metrics, error)

var (
	brokersScraperType   = component.MustNewType("brokers")
	topicsScraperType    = component.MustNewType("topics")
	consumersScraperType = component.MustNewType("consumers")

	allScrapers = map[string]createKafkaScraper{
		brokersScraperType.String():   createBrokerScraperFranz,
		topicsScraperType.String():    createTopicsScraperFranz,
		consumersScraperType.String(): createConsumerScraperFranz,
	}
)

var newMetricsReceiver = func(
	ctx context.Context,
	config Config,
	params receiver.Settings,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	scraperControllerOptions := make([]scraperhelper.ControllerOption, 0, len(config.Scrapers))
	for _, key := range config.Scrapers {
		factory, ok := allScrapers[key]
		if !ok {
			return nil, fmt.Errorf("no scraper found for key: %s", key)
		}
		s, err := factory(ctx, config, params)
		if err != nil {
			return nil, err
		}
		scraperControllerOptions = append(scraperControllerOptions, scraperhelper.AddMetricsScraper(metadata.Type, s))
	}

	return scraperhelper.NewMetricsController(
		&config.ControllerConfig,
		params,
		consumer,
		scraperControllerOptions...,
	)
}
