// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package windowsservicereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/windowsservicereceiver"

import (
	"go.opentelemetry.io/collector/receiver/scraperhelper"
)

// Config defines configuration for windowsservice receiver.
type Config struct {
	scraperhelper.ControllerConfig `mapstructure:",squash"`
	Services                       []string `mapstructure:"services"`    // user provided list of services to monitor with receiver
	MonitorAll                     bool     `mapstructure:"monitor_all"` // monitor all services on host machine. supercedes services
}

// Validate checks the receiver configuration is valid
func (cfg *Config) Validate() error { return nil }
