// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hwscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/hwscraper"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/hwscraper/internal/metadata"
)

// Config relating to HW Sensor Metric Scraper.
type Config struct {
	metadata.MetricsBuilderConfig `mapstructure:",squash"`

	// HwmonPath specifies the path to hwmon directory
	// Default: /sys/class/hwmon
	HwmonPath string `mapstructure:"hwmon_path"`

	// Temperature specifies temperature sensor configuration
	Temperature *TemperatureConfig `mapstructure:"temperature"`
}

// TemperatureConfig configures temperature sensor scraping
type TemperatureConfig struct {
	// Include specifies a filter on sensor names to include
	Include MatchConfig `mapstructure:"include"`

	// Exclude specifies a filter on sensor names to exclude
	Exclude MatchConfig `mapstructure:"exclude"`
}

// MatchConfig configures sensor name matching (shared by sub-scrapers)
type MatchConfig struct {
	filterset.Config `mapstructure:",squash"`

	// Sensors specifies sensor name patterns to match
	Sensors []string `mapstructure:"sensors"`
}
