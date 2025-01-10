// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cpuscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/cpuscraper"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/cpuscraper/internal/metadata"
)

// This file implements Factory for CPU scraper.

// Type the value of "type" key in configuration.
var Type = component.MustNewType("cpu")

// Factory is the Factory for scraper.
type Factory struct{}

// CreateDefaultConfig creates the default configuration for the Scraper.
func (f *Factory) CreateDefaultConfig() component.Config {
	return &Config{
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
	}
}

// CreateMetricsScraper creates a scraper based on provided config.
func (f *Factory) CreateMetricsScraper(
	ctx context.Context,
	settings receiver.Settings,
	config component.Config,
) (scraper.Metrics, error) {
	cfg := config.(*Config)
	s := newCPUScraper(ctx, settings, cfg)

	return scraper.NewMetrics(s.scrape,
		scraper.WithStart(s.start),
	)
}
