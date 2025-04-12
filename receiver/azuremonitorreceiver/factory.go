// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuremonitorreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azuremonitorreceiver"

import (
	"context"
	"errors"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azuremonitorreceiver/internal/metadata"
)

const (
	defaultCollectionInterval = 10 * time.Second
	defaultCloud              = azureCloud
)

var errConfigNotAzureMonitor = errors.New("Config was not a Azure Monitor receiver config")

// NewFactory creates a new receiver factory
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability))
}

// createDefaultConfig creates the default configuration for the receiver
func createDefaultConfig() component.Config {
	cfg := scraperhelper.NewDefaultControllerConfig()
	cfg.CollectionInterval = defaultCollectionInterval

	return &Config{
		ControllerConfig:                  cfg,
		MetricsBuilderConfig:              metadata.DefaultMetricsBuilderConfig(),
		CacheResources:                    24 * 60 * 60,
		CacheResourcesDefinitions:         24 * 60 * 60,
		MaximumNumberOfMetricsInACall:     20,
		MaximumNumberOfRecordsPerResource: 10,
		Services:                          monitorServices,
		Authentication:                    servicePrincipal,
		Cloud:                             defaultCloud,
	}
}

func createMetricsReceiver(_ context.Context, params receiver.Settings, rConf component.Config, consumer consumer.Metrics) (receiver.Metrics, error) {
	cfg, ok := rConf.(*Config)
	if !ok {
		return nil, errConfigNotAzureMonitor
	}

	azureScraper := newScraper(cfg, params)
	s, err := scraper.NewMetrics(azureScraper.scrape, scraper.WithStart(azureScraper.start))
	if err != nil {
		return nil, err
	}

	return scraperhelper.NewMetricsController(&cfg.ControllerConfig, params, consumer, scraperhelper.AddScraper(metadata.Type, s))
}
