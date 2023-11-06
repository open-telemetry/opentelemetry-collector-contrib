// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package deltatocumulativeprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor"

import (
	"go.opentelemetry.io/collector/config/configtelemetry"
	"time"

	"go.opentelemetry.io/collector/component"
)

// Config defines configuration for Prometheus exporter.
type Config struct {
	// MaxStaleness is the total time a state entry will live past the time it was last seen. Set to 0 to retain state indefinitely.
	MaxStaleness time.Duration `mapstructure:"max_staleness"`

	// SendInterval defines how frequently to emit the aggregated metric
	SendInterval time.Duration `mapstructure:"send_interval"`

	IdentifyMode IdentifyMode `mapstructure:"identity_mode"`
	//Metrics      []Metric      `mapstructure:"metrics"`
	Level configtelemetry.Level `mapstructure:"level"`
}
type IdentifyMode string

const (
	ScopeAndResourceAttributes IdentifyMode = "SCOPE_AND_RESOURCE_ATTRIBUTES"
	ScopeAttributesOnly        IdentifyMode = "SCOPE_ATTRIBUTES_ONLY"
)

//type Metric struct {
//	// Include match properties describe metrics that should be included in the Collector Service pipeline,
//	// all other metrics should be dropped from further processing.
//	// If both Include and Exclude are specified, Include filtering occurs first.
//	Include *filterconfig.MetricMatchProperties `mapstructure:"include"`
//
//	// Exclude match properties describe metrics that should be excluded from the Collector Service pipeline,
//	// all other metrics should be included.
//	// If both Include and Exclude are specified, Include filtering occurs first.
//	Exclude *filterconfig.MetricMatchProperties `mapstructure:"exclude"`
//
//	// RegexpConfig specifies options for the regexp match type
//	RegexpConfig *regexp.Config `mapstructure:"regexp"`
//}

var _ component.Config = (*Config)(nil)

// Validate checks if the exporter configuration is valid
func (cfg *Config) Validate() error {
	//if cfg.SendInterval > time.Duration(1) {
	//
	//}
	return nil
}
