// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package valkeyreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/valkeyreceiver"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
	"go.opentelemetry.io/collector/scraper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/valkeyreceiver/internal/metadata"
)

// NewFactory creates a factory for Valkey receiver.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability))
}

func createDefaultConfig() component.Config {
	scs := scraperhelper.NewDefaultControllerConfig()
	scs.CollectionInterval = 10 * time.Second
	return &Config{
		AddrConfig: confignet.AddrConfig{
			Transport: confignet.TransportTypeTCP,
		},
		TLS: configtls.ClientConfig{
			Insecure: true,
		},
		ControllerConfig:     scs,
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
	}
}

func createMetricsReceiver(
	_ context.Context,
	set receiver.Settings,
	cfg component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	oCfg := cfg.(*Config)

	scrp, err := newValkeyScraper(oCfg, set)
	if err != nil {
		return nil, err
	}

	scraper, err := scraper.NewMetrics(
		scraper.ScrapeMetricsFunc(scrp.scrape),
		scraper.WithShutdown(scrp.shutdown),
	)
	if err != err {
		return nil, err
	}

	return scraperhelper.NewScraperControllerReceiver(&oCfg.ControllerConfig, set, consumer, scraperhelper.AddScraper(metadata.Type, scraper))
}
