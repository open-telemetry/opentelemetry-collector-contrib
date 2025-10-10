// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package icmpcheckreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/icmpcheckreceiver"

import (
	"errors"
	"time"

	"go.opentelemetry.io/collector/scraper/scraperhelper"
	"go.uber.org/multierr"
)

var (
	errMissingTarget     = errors.New("must specify at least one target")
	errMissingTargetHost = errors.New("target host is required")
)

type Config struct {
	scraperhelper.ControllerConfig `mapstructure:",squash"`
	Targets                        []PingTarget `mapstructure:"targets"`

	// prevent unkeyed literal initialization
	_ struct{}
}
type PingTarget struct {
	Host         string        `mapstructure:"host"`
	PingCount    int           `mapstructure:"ping_count,omitempty"`
	PingTimeout  time.Duration `mapstructure:"ping_timeout,omitempty"`
	PingInterval time.Duration `mapstructure:"ping_interval,omitempty"`
}

func (c *Config) Validate() error {
	var err error

	if len(c.Targets) == 0 {
		return multierr.Append(err, errMissingTarget)
	}

	for _, target := range c.Targets {
		if target.Host == "" {
			err = multierr.Append(err, errMissingTargetHost)
		}
	}

	return err
}
