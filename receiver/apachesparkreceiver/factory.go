// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package apachesparkreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/apachesparkreceiver"

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/apachesparkreceiver/internal/metadata"
)

var errConfigNotSpark = errors.New("config was not a Spark receiver config")

// NewFactory creates a new receiver factory for Spark
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability))
}

// createDefaultConfig creates a config for Spark with as many default values as possible
func createDefaultConfig() component.Config {
	cfg := scraperhelper.NewDefaultScraperControllerSettings(metadata.Type)
	cfg.CollectionInterval = defaultCollectionInterval

	return &Config{
		ScraperControllerSettings: cfg,
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: defaultEndpoint,
		},
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
	}
}

// createMetricsReceiver creates the metric receiver for Spark
func createMetricsReceiver(
	_ context.Context,
	params receiver.CreateSettings,
	config component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	sparkConfig, ok := config.(*Config)
	if !ok {
		return nil, errConfigNotSpark
	}

	sparkScraper := newSparkScraper(params.Logger, sparkConfig, params)
	scraper, err := scraperhelper.NewScraper(metadata.Type, sparkScraper.scrape,
		scraperhelper.WithStart(sparkScraper.start))
	if err != nil {
		return nil, err
	}

	return scraperhelper.NewScraperControllerReceiver(&sparkConfig.ScraperControllerSettings, params,
		consumer, scraperhelper.AddScraper(scraper))
}
