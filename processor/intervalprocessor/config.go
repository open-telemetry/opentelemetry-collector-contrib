// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package intervalprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/intervalprocessor"

import (
	"time"

	"go.opentelemetry.io/collector/component"
)

var _ component.Config = (*Config)(nil)

// Config defines the configuration for the processor.
type Config struct {
	// MaxStaleness is the total time a state entry will live past the time it was last seen. Set to 0 to retain state indefinitely.
	MaxStaleness time.Duration `mapstructure:"max_staleness"`
}

// Validate checks whether the input configuration has all of the required fields for the processor.
// An error is returned if there are any invalid inputs.
func (config *Config) Validate() error {
	return nil
}
