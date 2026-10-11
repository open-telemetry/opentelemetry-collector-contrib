// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dnscheckreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/dnscheckreceiver"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	collectorscraper "go.opentelemetry.io/collector/scraper"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/dnscheckreceiver/internal/metadata"
)

func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability),
	)
}

func createDefaultConfig() component.Config {
	cfg := scraperhelper.NewDefaultControllerConfig()

	return &Config{
		ControllerConfig:     cfg,
		MetricsBuilderConfig: metadata.NewDefaultMetricsBuilderConfig(),
		DNSServers:           []DNSServerConfig{},
		Hostnames:            []HostnameConfig{},
	}
}

func createMetricsReceiver(
	_ context.Context,
	settings receiver.Settings,
	cfg component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	config := cfg.(*Config)

	scrp := newScraper(config, settings)
	s, err := collectorscraper.NewMetrics(scrp.scrape)
	if err != nil {
		return nil, err
	}

	return scraperhelper.NewMetricsController(
		&config.ControllerConfig,
		settings,
		consumer,
		scraperhelper.AddMetricsScraper(metadata.Type, s),
	)
}
