// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package raidscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/raidscraper"

import (
	"context"
	"errors"
	"runtime"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/scraper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/raidscraper/internal/metadata"
)

const defaultSysDeviceFilesystem = "/sys"

// NewFactory for System scraper.
func NewFactory() scraper.Factory {
	return scraper.NewFactory(metadata.Type, createDefaultConfig, scraper.WithMetrics(createMetricsScraper, metadata.MetricsStability))
}

// createDefaultConfig creates the default configuration for the Scraper.
func createDefaultConfig() component.Config {
	return &Config{
		SysDeviceFilesystem:  defaultSysDeviceFilesystem,
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
	}
}

// createMetricsScraper creates a resource scraper based on provided config.
func createMetricsScraper(
	ctx context.Context,
	settings scraper.Settings,
	cfg component.Config,
) (scraper.Metrics, error) {

	// left darwin as supported OS for unit testing on mac - it is mocked
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		return nil, errors.New("process scraper only available on Linux, Windows, macOS, or FreeBSD")
	}

	raidScraper, err := newRaidScraper(ctx, settings, cfg.(*Config))
	if err != nil {
		return nil, err
	}

	return scraper.NewMetrics(
		raidScraper.scrape,
		scraper.WithStart(raidScraper.start),
	)
}
