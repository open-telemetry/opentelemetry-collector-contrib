// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package psiscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/psiscraper"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/psiscraper/internal/metadata"
)

// Config relating to PSI Metric Scraper.
type Config struct {
	// MetricsBuilderConfig allows to customize scraped metrics/attributes representation.
	metadata.MetricsBuilderConfig `mapstructure:",squash"`
	// RootPath is the host's root directory (linux only).
	RootPath string `mapstructure:"-"`
}

func (cfg *Config) SetRootPath(rootPath string) {
	cfg.RootPath = rootPath
}
