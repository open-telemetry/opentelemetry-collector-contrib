// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package redisreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/redisreceiver"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper"
	"go.opentelemetry.io/collector/scraper/scraperhelper"
	"go.opentelemetry.io/collector/scraper/xscraper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/redisreceiver/internal/metadata"
)

// NewFactory creates a factory for Redis receiver.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability),
	)
}

// NewScraperFactory creates a factory for the Redis scraper.
func NewScraperFactory() scraper.Factory {
	return xscraper.NewFactory(
		metadata.Type,
		createDefaultScraperConfig,
		xscraper.WithMetrics(createMetricsScraper, metadata.MetricsStability),
		xscraper.WithDiscoveryRule(metadata.DiscoveryRule()),
	)
}

func createDefaultConfig() component.Config {
	scs := scraperhelper.NewDefaultControllerConfig()
	scs.CollectionInterval = 10 * time.Second
	return &Config{
		ControllerConfig: scs,
		ScraperConfig:    *createDefaultScraperConfig().(*ScraperConfig),
	}
}

func createDefaultScraperConfig() component.Config {
	return &ScraperConfig{
		AddrConfig: confignet.AddrConfig{
			Transport: confignet.TransportTypeTCP,
		},
		TLS: configtls.ClientConfig{
			Insecure: true,
		},
		MetricsBuilderConfig: metadata.NewDefaultMetricsBuilderConfig(),
	}
}

func createMetricsScraper(_ context.Context, set scraper.Settings, cfg component.Config) (scraper.Metrics, error) {
	return newRedisScraper(cfg.(*ScraperConfig), set)
}

func createMetricsReceiver(
	_ context.Context,
	set receiver.Settings,
	cfg component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	oCfg := cfg.(*Config)
	return scraperhelper.NewMetricsController(
		&oCfg.ControllerConfig, set, consumer,
		scraperhelper.AddFactoryWithConfig(NewScraperFactory(), &oCfg.ScraperConfig),
	)
}
