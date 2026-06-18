// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hardwarescraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/hardwarescraper"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/scraper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/hardwarescraper/internal/metadata"
)

// NewFactory creates a new factory for hardwarescraper.
func NewFactory() scraper.Factory {
	return scraper.NewFactory(metadata.Type, createDefaultConfig, scraper.WithMetrics(createMetricsScraper, metadata.MetricsStability))
}

// createDefaultConfig creates the default configuration for the scraper.
func createDefaultConfig() component.Config {
	return &Config{
		MetricsBuilderConfig: metadata.NewDefaultMetricsBuilderConfig(),
		HwmonPath:            defaultHwmonPath,
		Temperature: &TemperatureConfig{
			Include: MatchConfig{
				Config:  filterset.Config{MatchType: filterset.Regexp},
				Sensors: []string{".*"},
			},
		},
	}
}

// createMetricsScraper creates a hardwarescraper based on provided config.
func createMetricsScraper(
	ctx context.Context,
	settings scraper.Settings,
	config component.Config,
) (scraper.Metrics, error) {
	cfg := config.(*Config)
	s := newHardwareScraper(ctx, settings, cfg)

	return scraper.NewMetrics(s.scrape, scraper.WithStart(s.start))
}
