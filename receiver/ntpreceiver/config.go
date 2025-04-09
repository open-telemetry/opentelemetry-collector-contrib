// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ntpreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ntpreceiver"

import (
	"errors"
	"fmt"
	"net"
	"time"

	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ntpreceiver/internal/metadata"
)

// Config is the configuration for the NSX receiver
type Config struct {
	scraperhelper.ControllerConfig `mapstructure:",squash"`
	metadata.MetricsBuilderConfig  `mapstructure:",squash"`
	Version                        int    `mapstructure:"version"`
	Endpoint                       string `mapstructure:"endpoint"`
}

func (c *Config) Validate() error {
	var errs []error
	_, _, err := net.SplitHostPort(c.Endpoint)
	if err != nil {
		errs = append(errs, err)
	}
	// respect terms of service https://www.pool.ntp.org/tos.html
	if c.CollectionInterval < 30*time.Minute {
		errs = append(errs, fmt.Errorf("collection interval %v is less than minimum 30m", c.CollectionInterval))
	}
	return errors.Join(errs...)
}
