// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filesystemscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/filesystemscraper"

import (
	"context"
	"os"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/scraper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/filesystemscraper/internal/metadata"
)

// NewFactory for FileSystem scraper.
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

	if cfg.rootPath == "" {
		inContainer := os.Getpid() == 1
		for _, p := range []string{
			"/.dockerenv",        // Mounted by dockerd when starting a container by default
			"/run/.containerenv", // Mounted by podman as described here: https://github.com/containers/podman/blob/ecbb52cb478309cfd59cc061f082702b69f0f4b7/docs/source/markdown/podman-run.1.md.in#L31
		} {
			if _, err := os.Stat(p); err == nil {
				inContainer = true
				break
			}
		}
		if inContainer {
			settings.Logger.Warn(
				"No `root_path` config set when running in docker environment, will report container filesystem stats." +
					" See https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/hostmetricsreceiver#collecting-host-metrics-from-inside-a-container-linux-only")
		}
	}

	s, err := newFileSystemScraper(ctx, settings, cfg)
	if err != nil {
		return nil, err
	}

	return scraper.NewMetrics(s.scrape, scraper.WithStart(s.start))
}
