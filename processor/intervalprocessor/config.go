// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package intervalprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/intervalprocessor"

import (
	"errors"
	"time"

	"go.opentelemetry.io/collector/component"
)

var ErrInvalidIntervalValue = errors.New("invalid interval value")

var _ component.Config = (*Config)(nil)

// Config defines the configuration for the processor.
type Config struct {
	// Interval is the time interval at which the processor will aggregate metrics.
	Interval time.Duration `mapstructure:"interval"`
	// PassThrough is a configuration that determines whether gauge and summary metrics should be passed through
	// as they are or aggregated.
	PassThrough PassThrough `mapstructure:"pass_through"`
}

type PassThrough struct {
	// Gauge is a flag that determines whether gauge metrics should be passed through
	// as they are or aggregated.
	Gauge bool `mapstructure:"gauge"`
	// Summary is a flag that determines whether summary metrics should be passed through
	// as they are or aggregated.
	Summary bool `mapstructure:"summary"`
}

// Validate checks whether the input configuration has all of the required fields for the processor.
// An error is returned if there are any invalid inputs.
func (config *Config) Validate() error {
	if config.Interval <= 0 {
		return ErrInvalidIntervalValue
	}

	return nil
}
