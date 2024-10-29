// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package uptimescraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/uptimescraper"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/uptimescraper/internal/metadata"
)

// Config relating to Uptime Metric Scraper.
type Config struct {
	// MetricsBuilderConfig allows to customize scraped metrics/attributes representation.
	metadata.MetricsBuilderConfig `mapstructure:",squash"`
	internal.ScraperConfig
}
