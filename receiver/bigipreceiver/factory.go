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
	clientConfig := confighttp.NewDefaultClientConfig()
	clientConfig.Endpoint = defaultEndpoint
	clientConfig.Timeout = 10 * time.Second
	return &Config{
		ControllerConfig: scraperhelper.ControllerConfig{
			CollectionInterval: 10 * time.Second,
		},
		ClientConfig:         clientConfig,
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
	}
}

// creates the metric receiver for Big-IP
func createMetricsReceiver(_ context.Context, params receiver.Settings, rConf component.Config, consumer consumer.Metrics) (receiver.Metrics, error) {
	cfg, ok := rConf.(*Config)
	if !ok {
		return nil, errConfigNotBigip
	}

	bigipScraper := newScraper(params.Logger, cfg, params)
	scraper, err := scraperhelper.NewScraperWithoutType(bigipScraper.scrape, scraperhelper.WithStart(bigipScraper.start))
	if err != nil {
		return nil, err
	}

	return scraperhelper.NewScraperControllerReceiver(&cfg.ControllerConfig, params, consumer, scraperhelper.AddScraperWithType(metadata.Type, scraper))
}
