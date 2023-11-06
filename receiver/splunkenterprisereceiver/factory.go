// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splunkenterprisereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/splunkenterprisereceiver"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/splunkenterprisereceiver/internal/metadata"
)

const (
	defaultInterval          = 10 * time.Minute
	defaultMaxSearchWaitTime = 60 * time.Second
)

func createDefaultConfig() component.Config {
	// Default HttpClient settings
	httpCfg := confighttp.NewDefaultHTTPClientSettings()
	httpCfg.Headers = map[string]configopaque.String{
		"Content-Type": "application/x-www-form-urlencoded",
	}

	// Default ScraperController settings
	scfg := scraperhelper.NewDefaultScraperControllerSettings(metadata.Type)
	scfg.CollectionInterval = defaultInterval
	scfg.Timeout = defaultMaxSearchWaitTime

	return &Config{
		HTTPClientSettings:        httpCfg,
		ScraperControllerSettings: scfg,
		MetricsBuilderConfig:      metadata.DefaultMetricsBuilderConfig(),
	}
}

func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability),
	)
}

func createMetricsReceiver(
	_ context.Context,
	params receiver.CreateSettings,
	baseCfg component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	cfg := baseCfg.(*Config)
	splunkScraper := newSplunkMetricsScraper(params, cfg)

	scraper, err := scraperhelper.NewScraper(metadata.Type,
		splunkScraper.scrape,
		scraperhelper.WithStart(splunkScraper.start))
	if err != nil {
		return nil, err
	}

	return scraperhelper.NewScraperControllerReceiver(
		&cfg.ScraperControllerSettings,
		params,
		consumer,
		scraperhelper.AddScraper(scraper),
	)
}
