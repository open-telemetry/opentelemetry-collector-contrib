// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package intervalprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/intervalprocessor"

import (
	"errors"
	"time"

	"go.opentelemetry.io/collector/component"
)

var (
	ErrInvalidIntervalValue = errors.New("invalid interval value")
)

var _ component.Config = (*Config)(nil)

// Config defines the configuration for the processor.
type Config struct {
	// Interval is the time
	Interval time.Duration `mapstructure:"interval"`
}

// Validate checks whether the input configuration has all of the required fields for the processor.
// An error is returned if there are any invalid inputs.
func (config *Config) Validate() error {
	if config.Interval <= 0 {
		return ErrInvalidIntervalValue
	}

	return nil
}
