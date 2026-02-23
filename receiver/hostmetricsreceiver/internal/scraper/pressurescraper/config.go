// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pressurescraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/pressurescraper"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/pressurescraper/internal/metadata"
)

// Config relating to Pressure Metric Scraper.
type Config struct {
	metadata.MetricsBuilderConfig `mapstructure:",squash"`

	rootPath string `mapstructure:"-"`
}

func (cfg *Config) SetRootPath(rootPath string) {
	cfg.rootPath = rootPath
}
