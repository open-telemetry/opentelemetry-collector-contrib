// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlecloudmonitoringreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudmonitoringreceiver"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudmonitoringreceiver/internal/metadata"
)

func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability))
}

// createDefaultConfig creates the default exporter configuration
func createDefaultConfig() component.Config {
	cfg := scraperhelper.NewDefaultControllerConfig()
	cfg.CollectionInterval = defaultCollectionInterval

	return &Config{
		ControllerConfig: cfg,
	}
}

func createMetricsReceiver(
	_ context.Context,
	settings receiver.Settings,
	baseCfg component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	rCfg := baseCfg.(*Config)
	r := newGoogleCloudMonitoringReceiver(rCfg, settings.Logger)

	s, err := scraper.NewMetrics(r.Scrape, scraper.WithStart(r.Start),
		scraper.WithShutdown(r.Shutdown))
	if err != nil {
		return nil, err
	}

	return scraperhelper.NewScraperControllerReceiver(&rCfg.ControllerConfig, settings, consumer,
		scraperhelper.AddScraper(metadata.Type, s))
}
