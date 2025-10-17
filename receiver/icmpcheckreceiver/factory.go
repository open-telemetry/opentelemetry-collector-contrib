// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package icmpcheckreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/icmpcheckreceiver"

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/icmpcheckreceiver/internal/metadata"
)

var errConfigNotICMPCheck = errors.New("config was not a ICMP check receiver config")

func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability))
}

func createDefaultConfig() component.Config {
	return &Config{
		ControllerConfig:     scraperhelper.NewDefaultControllerConfig(),
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
		Targets:              []PingTarget{},
	}
}

func createMetricsReceiver(
	_ context.Context,
	settings receiver.Settings,
	cfg component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	config, ok := cfg.(*Config)
	if !ok {
		return nil, errConfigNotICMPCheck
	}

	icmpCheckScraper := newScraper(config, settings)
	s, err := scraper.NewMetrics(icmpCheckScraper.scrape, scraper.WithStart(icmpCheckScraper.start))
	if err != nil {
		return nil, err
	}

	return scraperhelper.NewMetricsController(
		&config.ControllerConfig,
		settings,
		consumer,
		scraperhelper.AddScraper(metadata.Type, s),
	)
}
