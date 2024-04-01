// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package intervalprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/intervalprocessor"

import (
	"errors"
	"time"

	"go.opentelemetry.io/collector/component"
)

var (
	ErrInvalidIntervalValue     = errors.New("invalid interval value")
	ErrInvalidMaxStatenessValue = errors.New("invalid max_stateless value")
)

var _ component.Config = (*Config)(nil)

// Config defines the configuration for the processor.
type Config struct {
	// Interval is the time
	Interval time.Duration `mapstructure:"interval"`

	// MaxStaleness is the total time a state entry will live past the time it was last seen. Set to 0 to retain state indefinitely.
	MaxStaleness time.Duration `mapstructure:"max_staleness"`
}

// Validate checks whether the input configuration has all of the required fields for the processor.
// An error is returned if there are any invalid inputs.
func (config *Config) Validate() error {
	if config.Interval <= 0 {
		return ErrInvalidIntervalValue
	}

	if config.MaxStaleness < 0 {
		return ErrInvalidMaxStatenessValue
	}

	return nil
}
