// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusexecreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusexecreceiver"

import (
	"errors"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusexecreceiver/subprocessmanager"
)

// Config definition for prometheus_exec configuration
type Config struct {
	// Generic receiver config
	// ScrapeInterval is the time between each scrape completed by the Receiver
	ScrapeInterval time.Duration `mapstructure:"scrape_interval,omitempty"`
	// ScrapeTimeout is the time to wait before throttling a scrape request
	ScrapeTimeout time.Duration `mapstructure:"scrape_timeout,omitempty"`
	// Port is the port assigned to the Receiver, and to the {{port}} template variables
	Port int `mapstructure:"port"`
	// SubprocessConfig is the configuration needed for the subprocess
	SubprocessConfig subprocessmanager.SubprocessConfig `mapstructure:",squash"`
}

func (cfg *Config) Validate() error {
	if cfg.SubprocessConfig.Command == "" {
		return errors.New("command to execute must be non-empty")
	}
	return nil
}
