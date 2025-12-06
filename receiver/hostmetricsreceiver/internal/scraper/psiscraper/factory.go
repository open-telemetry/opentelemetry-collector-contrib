// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package psiscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/psiscraper"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/scraper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/psiscraper/internal/metadata"
)

// NewFactory for PSI scraper.
func NewFactory() scraper.Factory {
	return scraper.NewFactory(metadata.Type, createDefaultConfig, scraper.WithMetrics(createMetricsScraper, metadata.MetricsStability))
}

// createDefaultConfig creates the default configuration for the Scraper.
func createDefaultConfig() component.Config {
	return &Config{
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
	}
}

// createMetricsScraper creates a scraper based on provided config.
func createMetricsScraper(
	ctx context.Context,
	settings scraper.Settings,
	config component.Config,
) (scraper.Metrics, error) {
	cfg := config.(*Config)
	s := newPSIScraper(ctx, settings, cfg)

	return scraper.NewMetrics(
		s.scrape,
		scraper.WithStart(s.start),
	)
}
