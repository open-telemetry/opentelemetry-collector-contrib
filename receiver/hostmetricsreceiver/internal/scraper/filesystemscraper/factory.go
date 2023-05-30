// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filesystemscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/filesystemscraper"

import (
	"context"
	"os"

	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/filesystemscraper/internal/metadata"
)

// This file implements Factory for FileSystem scraper.

const (
	// TypeStr the value of "type" key in configuration.
	TypeStr = "filesystem"
)

// Factory is the Factory for scraper.
type Factory struct {
}

// Type gets the type of the scraper config created by this Factory.
func (f *Factory) Type() string {
	return TypeStr
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
	cfg := config.(*Config)

	if cfg.RootPath == "" {
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

	return scraperhelper.NewScraper(
		TypeStr, s.scrape, scraperhelper.WithStart(s.start))
}
