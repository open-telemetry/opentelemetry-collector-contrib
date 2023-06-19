// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal"

import (
	"context"

	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
)

// ScraperFactory can create a MetricScraper.
type ScraperFactory interface {
	// CreateDefaultConfig creates the default configuration for the Scraper.
	CreateDefaultConfig() Config

	// CreateMetricsScraper creates a scraper based on this config.
	// If the config is not valid, error will be returned instead.
	CreateMetricsScraper(ctx context.Context, settings receiver.CreateSettings, cfg Config) (scraperhelper.Scraper, error)
}

// Config is the configuration of a scraper.
type Config interface {
	SetRootPath(rootPath string)
}

type ScraperConfig struct {
	RootPath string `mapstructure:"-"`
}

func (p *ScraperConfig) SetRootPath(rootPath string) {
	p.RootPath = rootPath
}
