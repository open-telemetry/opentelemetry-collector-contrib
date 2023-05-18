// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package system // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/system"

import "fmt"

// Config defines user-specified configurations unique to the system detector
type Config struct {
	// The HostnameSources is a priority list of sources from which hostname will be fetched.
	// In case of the error in fetching hostname from source,
	// the next source from the list will be considered.(**default**: `["dns", "os"]`)
	HostnameSources []string `mapstructure:"hostname_sources"`
}

// Validate config
func (cfg *Config) Validate() error {
	for _, hostnameSource := range cfg.HostnameSources {
		_, exists := hostnameSourcesMap[hostnameSource]
		if !exists {
			return fmt.Errorf("hostname_sources contains invalid value: %q", hostnameSource)
		}
	}
	return nil
}
