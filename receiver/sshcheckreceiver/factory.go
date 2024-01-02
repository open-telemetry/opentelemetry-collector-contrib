// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sshcheckreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sshcheckreceiver"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sshcheckreceiver/internal/configssh"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sshcheckreceiver/internal/metadata"
)

// NewFactory creates a new receiver factory
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		metadata.Type,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, metadata.MetricsStability))
}

func createDefaultConfig() component.Config {
	cfg := scraperhelper.NewDefaultScraperControllerSettings(metadata.Type)
	cfg.CollectionInterval = 10 * time.Second

	return &Config{
		ScraperControllerSettings: cfg,
		SSHClientSettings: configssh.SSHClientSettings{
			Timeout: 10 * time.Second,
		},
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
	}
}

func createMetricsReceiver(_ context.Context, params receiver.CreateSettings, rConf component.Config, consumer consumer.Metrics) (receiver.Metrics, error) {
	// return error if sshcheckreceiver on Windows
	if !supportedOS() {
		return nil, errWindowsUnsupported
	}

	cfg, ok := rConf.(*Config)
	if !ok {
		return nil, errConfigNotSSHCheck
	}

	sshcheckScraper := newScraper(cfg, params)
	scraper, err := scraperhelper.NewScraper(metadata.Type, sshcheckScraper.scrape, scraperhelper.WithStart(sshcheckScraper.start))
	if err != nil {
		return nil, err
	}

	return scraperhelper.NewScraperControllerReceiver(&cfg.ScraperControllerSettings, params, consumer, scraperhelper.AddScraper(scraper))
}
