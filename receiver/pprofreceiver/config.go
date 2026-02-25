// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pprofreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/pprofreceiver"

import (
	"go.opentelemetry.io/collector/scraper/scraperhelper"
)

// Config defines the configuration for the pprof receiver.
type Config struct {
	scraperhelper.ControllerConfig `mapstructure:",squash"`

	// Include is the glob pattern for pprof files to scrape.
	Include string `mapstructure:"include"`
}
