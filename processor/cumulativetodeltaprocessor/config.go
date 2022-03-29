// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cumulativetodeltaprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/cumulativetodeltaprocessor"

import (
	"fmt"
	"time"

	"go.opentelemetry.io/collector/config"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/processor/filterset"
)

// Config defines the configuration for the processor.
type Config struct {
	config.ProcessorSettings `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct

	// MaxStaleness is the total time a state entry will live past the time it was last seen. Set to 0 to retain state indefinitely.
	MaxStaleness time.Duration `mapstructure:"max_staleness"`

	// Deprecated. List of cumulative metrics to convert to delta.
	// Cannot be used with Include/Exclude.
	Metrics []string `mapstructure:"metrics"`

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

var _ config.Processor = (*Config)(nil)

// Validate checks whether the input configuration has all of the required fields for the processor.
// An error is returned if there are any invalid inputs.
func (config *Config) Validate() error {
	if len(config.Metrics) > 0 && (len(config.Include.Metrics) > 0 || len(config.Exclude.Metrics) > 0) {
		return fmt.Errorf("metrics and include/exclude cannot be used at the same time")
	}
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
