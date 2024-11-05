// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package systemscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/systemscraper"

import (
	"context"
	"errors"
	"runtime"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/systemscraper/internal/metadata"
)

// This file implements Factory for System scraper.

const (
	// TypeStr the value of "type" key in configuration.
	TypeStr = "system"
)

var (
	// scraperType is the component type used for the built scraper.
	scraperType component.Type = component.MustNewType(TypeStr)
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

// CreateMetricsScraper creates a resource scraper based on provided config.
func (f *Factory) CreateMetricsScraper(
	ctx context.Context,
	settings receiver.Settings,
	cfg internal.Config,
) (scraperhelper.Scraper, error) {
	if runtime.GOOS != "linux" && runtime.GOOS != "windows" && runtime.GOOS != "darwin" {
		return nil, errors.New("uptime scraper only available on Linux, Windows, or MacOS")
	}

	uptimeScraper := newUptimeScraper(ctx, settings, cfg.(*Config))

	return scraperhelper.NewScraper(
		scraperType,
		uptimeScraper.scrape,
		scraperhelper.WithStart(uptimeScraper.start),
	)
}
