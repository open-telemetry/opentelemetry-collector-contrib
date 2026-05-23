// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pprofreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/pprofreceiver"

import (
	"errors"

	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/scraper/scraperhelper"
)

// Config defines the configuration for the pprof receiver.
type Config struct {
	scraperhelper.ControllerConfig `mapstructure:",squash"`
	confighttp.ClientConfig        `mapstructure:",squash"`

	// Include is the glob pattern for pprof files to scrape.
	Include string `mapstructure:"include"`

	// Self-scraper only:
	// Fraction of blocking events that are profiled. A value <= 0 disables
	// profiling. See https://golang.org/pkg/runtime/#SetBlockProfileRate for details.
	BlockProfileFraction int `mapstructure:"block_profile_fraction"`

	// Self-scraper only:
	// Fraction of mutex contention events that are profiled. A value <= 0
	// disables profiling. See https://golang.org/pkg/runtime/#SetMutexProfileFraction
	// for details.
	MutexProfileFraction int `mapstructure:"mutex_profile_fraction"`
}

func (c *Config) Validate() error {
	if c.Include != "" && c.Endpoint != "" {
		return errors.New("either include or endpoint must be specified")
	}

	return nil
}
