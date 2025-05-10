// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ntpreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ntpreceiver"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ntpreceiver/internal/metadata"
)

func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability),
	)
}

func createDefaultConfig() component.Config {
	scraperConfig := scraperhelper.NewDefaultControllerConfig()
	scraperConfig.CollectionInterval = 30 * time.Minute
	return &Config{
		ControllerConfig:     scraperConfig,
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
		Version:              4,
		Endpoint:             "pool.ntp.org:123",
	}
}

func createMetricsReceiver(_ context.Context, settings receiver.Settings, cfg component.Config, consumer consumer.Metrics) (receiver.Metrics, error) {
	rCfg := cfg.(*Config)
	mp := newScraper(rCfg, settings)
	s, err := scraperhelper.NewScraper(metadata.Type, mp.scrape)
	if err != nil {
		return nil, err
	}
	opt := scraperhelper.AddScraper(s)

	return scraperhelper.NewScraperControllerReceiver(
		&rCfg.ControllerConfig,
		settings,
		consumer,
		opt,
	)
}
