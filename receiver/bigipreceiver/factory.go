// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package bigipreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/bigipreceiver"

import (
	"context"
	"errors"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/bigipreceiver/internal/metadata"
)

var errConfigNotBigip = errors.New("config was not a Big-IP receiver config")

// NewFactory creates a new receiver factory for Big-IP
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability))
}

// createDefaultConfig creates a config for Big-IP with as many default values as possible
func createDefaultConfig() component.Config {
	return &Config{
		ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
			CollectionInterval: 10 * time.Second,
		},
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Endpoint: defaultEndpoint,
			Timeout:  10 * time.Second,
		},
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
	}
}

// creates the metric receiver for Big-IP
func createMetricsReceiver(_ context.Context, params receiver.CreateSettings, rConf component.Config, consumer consumer.Metrics) (receiver.Metrics, error) {
	cfg, ok := rConf.(*Config)
	if !ok {
		return nil, errConfigNotBigip
	}

	bigipScraper := newScraper(params.Logger, cfg, params)
	scraper, err := scraperhelper.NewScraper(metadata.Type, bigipScraper.scrape, scraperhelper.WithStart(bigipScraper.start))
	if err != nil {
		return nil, err
	}

	return scraperhelper.NewScraperControllerReceiver(&cfg.ScraperControllerSettings, params, consumer, scraperhelper.AddScraper(scraper))
}
