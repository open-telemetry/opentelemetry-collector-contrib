// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hwscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/hwscraper"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/scraper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/hwscraper/internal/metadata"
)

// NewFactory creates a new factory for hw scraper.
func NewFactory() scraper.Factory {
	return scraper.NewFactory(metadata.Type, createDefaultConfig, scraper.WithMetrics(createMetricsScraper, metadata.MetricsStability))
}

// createDefaultConfig creates the default configuration for the scraper.
func createDefaultConfig() component.Config {
	return &Config{
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
		HwmonPath:            "/sys/class/hwmon",
		Temperature: &TemperatureConfig{
			Include: MatchConfig{
				Config:  filterset.Config{MatchType: filterset.Regexp},
				Sensors: []string{".*"},
			},
		},
	}
}

// createMetricsScraper creates a hw scraper based on provided config.
func createMetricsScraper(
	ctx context.Context,
	settings scraper.Settings,
	config component.Config,
) (scraper.Metrics, error) {
	cfg := config.(*Config)
	s := newHwScraper(ctx, settings, cfg)

	return scraper.NewMetrics(s.scrape, scraper.WithStart(s.start))
}
