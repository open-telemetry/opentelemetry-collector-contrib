// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package haproxyreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/haproxyreceiver"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/haproxyreceiver/internal/metadata"
)

// NewFactory creates a new HAProxy receiver factory.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		newDefaultConfig,
		receiver.WithMetrics(newReceiver, metadata.MetricsStability))
}

func newDefaultConfig() component.Config {
	return &Config{
		ControllerConfig:     scraperhelper.NewDefaultControllerConfig(),
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
	}
}

func newReceiver(
	_ context.Context,
	settings receiver.Settings,
	cfg component.Config,
	consumer consumer.Metrics,
) (receiver.Metrics, error) {
	haProxyCfg := cfg.(*Config)
	mp := newScraper(haProxyCfg, settings)
	s, err := scraper.NewMetrics(mp.scrape, scraper.WithStart(mp.start))
	if err != nil {
		return nil, err
	}

	return scraperhelper.NewScraperControllerReceiver(
		&haProxyCfg.ControllerConfig,
		settings,
		consumer,
		scraperhelper.AddScraper(metadata.Type, s),
	)
}
