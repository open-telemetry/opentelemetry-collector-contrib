// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package probabilisticsamplerprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/probabilisticsamplerprocessor"

import (
	"fmt"

	"go.opentelemetry.io/collector/component"
)

type AttributeSource string

const (
	traceIDAttributeSource = AttributeSource("traceID")
	recordAttributeSource  = AttributeSource("record")

	defaultAttributeSource = traceIDAttributeSource
)

var validAttributeSource = map[AttributeSource]bool{
	traceIDAttributeSource: true,
	recordAttributeSource:  true,
}

// Config has the configuration guiding the sampler processor.
type Config struct {

	// SamplingPercentage is the percentage rate at which traces or logs are going to be sampled. Defaults to zero, i.e.: no sample.
	// Values greater or equal 100 are treated as "sample all traces/logs".
	SamplingPercentage float32 `mapstructure:"sampling_percentage"`

	// HashSeed allows one to configure the hashing seed. This is important in scenarios where multiple layers of collectors
	// have different sampling rates: if they use the same seed all passing one layer may pass the other even if they have
	// different sampling rates, configuring different seeds avoids that.
	HashSeed uint32 `mapstructure:"hash_seed"`

	// AttributeSource (logs only) defines where to look for the attribute in from_attribute. The allowed values are
	// `traceID` or `record`. Default is `traceID`.
	AttributeSource `mapstructure:"attribute_source"`

	// FromAttribute (logs only) The optional name of a log record attribute used for sampling purposes, such as a
	// unique log record ID. The value of the attribute is only used if the trace ID is absent or if `attribute_source` is set to `record`.
	FromAttribute string `mapstructure:"from_attribute"`

	// SamplingPriority (logs only) allows to use a log record attribute designed by the `sampling_priority` key
	// to be used as the sampling priority of the log record.
	SamplingPriority string `mapstructure:"sampling_priority"`
}

var _ component.Config = (*Config)(nil)

// Validate checks if the processor configuration is valid
func (cfg *Config) Validate() error {
	if cfg.SamplingPercentage < 0 {
		return fmt.Errorf("negative sampling rate: %.2f", cfg.SamplingPercentage)
	}
	if cfg.AttributeSource != "" && !validAttributeSource[cfg.AttributeSource] {
		return fmt.Errorf("invalid attribute source: %v. Expected: %v or %v", cfg.AttributeSource, traceIDAttributeSource, recordAttributeSource)
	}
	return nil
}
