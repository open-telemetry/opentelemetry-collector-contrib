// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cumulativetodeltaprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/cumulativetodeltaprocessor"

import (
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/cumulativetodeltaprocessor/internal/tracking"
)

// Config defines the configuration for the processor.
type Config struct {
	// MaxStaleness is the total time a state entry will live past the time it was last seen. Set to 0 to retain state indefinitely.
	MaxStaleness time.Duration `mapstructure:"max_staleness"`

	// InitialValue determines how to handle the first datapoint for a given metric. Valid values:
	//
	//   - auto: (default) send the first point iff the startime is set AND the starttime happens after the component started AND the starttime is different from the timestamp
	//   - keep: always send the first point
	//   - drop: don't send the first point, but store it for subsequent delta calculations
	InitialValue tracking.InitialValue `mapstructure:"initial_value"`

	// Include specifies a filter on the metrics that should be converted.
	// Exclude specifies a filter on the metrics that should not be converted.
	// If neither `include` nor `exclude` are set, all metrics will be converted.
	// Cannot be used with deprecated Metrics config option.
	Include MatchMetrics `mapstructure:"include"`
	Exclude MatchMetrics `mapstructure:"exclude"`
}

type MatchMetrics struct {
	filterset.Config `mapstructure:",squash"`

	Metrics []string `mapstructure:"metrics"`
}

var _ component.Config = (*Config)(nil)

// Validate checks whether the input configuration has all of the required fields for the processor.
// An error is returned if there are any invalid inputs.
func (config *Config) Validate() error {
	if (len(config.Include.Metrics) > 0 && len(config.Include.MatchType) == 0) ||
		(len(config.Exclude.Metrics) > 0 && len(config.Exclude.MatchType) == 0) {
		return fmt.Errorf("match_type must be set if metrics are supplied")
	}
	if (len(config.Include.MatchType) > 0 && len(config.Include.Metrics) == 0) ||
		(len(config.Exclude.MatchType) > 0 && len(config.Exclude.Metrics) == 0) {
		return fmt.Errorf("metrics must be supplied if match_type is set")
	}
	return nil
}
