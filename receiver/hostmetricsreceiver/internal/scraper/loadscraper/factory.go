// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/loadscraper"

import (
	"context"

	"github.com/shirou/gopsutil/v3/common"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/loadscraper/internal/metadata"
)

// This file implements Factory for Load scraper.

const (
	// TypeStr the value of "type" key in configuration.
	TypeStr = "load"
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
	envMap common.EnvMap,
) (scraperhelper.Scraper, error) {
	cfg := config.(*Config)
	s := newLoadScraper(ctx, settings, cfg, envMap)

	return scraperhelper.NewScraper(
		TypeStr,
		s.scrape,
		scraperhelper.WithStart(s.start),
		scraperhelper.WithShutdown(s.shutdown),
	)
}
