// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package deltatorateprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatorateprocessor"

import (
	"fmt"
)

// Config defines the configuration for the processor.
type Config struct {

	// List of delta sum metrics to convert to rates
	Metrics []string `mapstructure:"metrics"`
}

// Validate checks whether the input configuration has all of the required fields for the processor.
// An error is returned if there are any invalid inputs.
func (config *Config) Validate() error {
	if len(config.Metrics) == 0 {
		return fmt.Errorf("metric names are missing")
	}
	return nil
}
