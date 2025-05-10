// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/githubreceiver/internal"

import (
	"context"

	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
)

type ScraperFactory interface {
	// Create the default configuration for the sub sccraper.
	CreateDefaultConfig() Config
	// Create a scraper based on the configuration passed or return an error if not valid.
	CreateMetricsScraper(ctx context.Context, params receiver.Settings, cfg Config) (scraperhelper.Scraper, error)
}

type Config any

type ScraperConfig struct {
}
