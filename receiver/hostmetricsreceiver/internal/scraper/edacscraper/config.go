// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package edacscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/edacscraper"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/edacscraper/internal/metadata"
)

// Config relating to EDAC Metric Scraper.
type Config struct {
	metadata.MetricsBuilderConfig `mapstructure:",squash"`
}
