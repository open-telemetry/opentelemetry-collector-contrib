// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package telemetrypolicyprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/telemetrypolicyprocessor"

import (
	"errors"

	"go.opentelemetry.io/collector/component"
)

// Config defines configuration for the telemetry_policy processor.
type Config struct {
	// Providers is the list of policy provider extension IDs to consume policies from.
	Providers []component.ID `mapstructure:"providers"`

	// prevent unkeyed literal initialization
	_ struct{}
}

var _ component.Config = (*Config)(nil)

// Validate checks if the processor configuration is valid.
func (cfg *Config) Validate() error {
	if len(cfg.Providers) == 0 {
		return errors.New("at least one provider must be specified")
	}
	for _, provider := range cfg.Providers {
		if provider.String() == "" {
			return errors.New("provider ID cannot be empty")
		}
	}
	return nil
}
