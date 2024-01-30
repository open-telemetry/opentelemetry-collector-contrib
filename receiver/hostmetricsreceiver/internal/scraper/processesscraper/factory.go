// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package processesscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processesscraper"

import (
	"context"

	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processesscraper/internal/metadata"
)

// This file implements Factory for Processes scraper.

const (
	// TypeStr the value of "type" key in configuration.
	TypeStr = "processes"
)

// Factory is the Factory for scraper.
type Factory struct {
}

// CreateDefaultConfig creates the default configuration for the Scraper.
func (f *Factory) CreateDefaultConfig() internal.Config {
	return &Config{
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
	}
}

// CreateMetricsScraper creates a scraper based on provided config.
func (f *Factory) CreateMetricsScraper(
	ctx context.Context,
	settings receiver.CreateSettings,
	config internal.Config,
) (scraperhelper.Scraper, error) {
	settings.Logger.Warn(`processes scraping is deprecated, system.processes.created and system.processes.count metrics have been moved to the process scraper.
	To enable them, apply the following config:

	scrapers:
	  process:
	    metrics:
	      system.processes.created:
	        enabled: true
	      system.processes.count:
	        enabled: true
`)
	cfg := config.(*Config)
	s := NewProcessesScraper(ctx, settings, cfg)

	return scraperhelper.NewScraper(
		TypeStr,
		s.Scrape,
		scraperhelper.WithStart(s.Start),
	)
}
