// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package windowsservicereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/windowsservicereceiver"

import (
	"go.opentelemetry.io/collector/receiver/scraperhelper"
)

// Config defines configuration for windowsservice receiver.
type Config struct {
	scraperhelper.ControllerConfig `mapstructure:",squash"`
	IncludeServices                []string `mapstructure:"include_services"` // user provided list of services to monitor with receiver
	ExcludeServices                []string `mapstructure:"exclude_services"` // user provided list of services to be excluded
}

// Validate checks the receiver configuration is valid
func (cfg *Config) Validate() error { return nil }
