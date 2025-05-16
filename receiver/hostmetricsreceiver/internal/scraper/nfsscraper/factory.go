// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package nfsscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/nfsscraper"

import (
	"context"
	"errors"
	"runtime"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/scraper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/nfsscraper/internal/metadata"
)

// NewFactory for NFS scraper.
func NewFactory() scraper.Factory {
	return scraper.NewFactory(metadata.Type, createDefaultConfig, scraper.WithMetrics(createMetricsScraper, metadata.MetricsStability))
}

// createDefaultConfig creates the default configuration for the Scraper.
func createDefaultConfig() component.Config {
	return &Config{
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
	}
}

// createMetricsScraper creates a resource scraper based on provided config.
func createMetricsScraper(
	_ context.Context,
	settings scraper.Settings,
	cfg component.Config,
) (scraper.Metrics, error) {
	if runtime.GOOS != "linux" {
		return nil, errors.New("uptime scraper only available on Linux")
	}

	nfsScraper, err := newNfsScraper(settings, cfg.(*Config))
	if err != nil {
		return nil, err
	}

	return scraper.NewMetrics(
		nfsScraper.scrape,
		scraper.WithStart(nfsScraper.start),
	)
}
