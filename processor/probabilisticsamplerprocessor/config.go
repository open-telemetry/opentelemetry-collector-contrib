// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package probabilisticsamplerprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/probabilisticsamplerprocessor"

import (
	"errors"
	"fmt"
	"math"

	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling"
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

	// Mode selects the sampling behavior. Supported values:
	//
	// - "hash_seed": the legacy behavior of this processor.
	//   Using an FNV hash combined with the HashSeed value, this
	//   sampler performs a non-consistent probabilistic
	//   downsampling.  The number of spans output is expected to
	//   equal SamplingPercentage (as a ratio) times the number of
	//   spans inpout, assuming good behavior from FNV and good
	//   entropy in the hashed attributes or TraceID.
	//
	// - "equalizing": Using an OTel-specified consistent sampling
	//   mechanism, this sampler selectively reduces the effective
	//   sampling probability of arriving spans.  This can be
	//   useful to select a small fraction of complete traces from
	//   a stream with mixed sampling rates.  The rate of spans
	//   passing through depends on how much sampling has already
	//   been applied.  If an arriving span was head sampled at
	//   the same probability it passes through.  If the span
	//   arrives with lower probability, a warning is logged
	//   because it means this sampler is configured with too
	//   large a sampling probability to ensure complete traces.
	//
	// - "proportional": Using an OTel-specified consistent sampling
	//   mechanism, this sampler reduces the effective sampling
	//   probability of each span by `SamplingProbability`.
	Mode SamplerMode `mapstructure:"mode"`

	// FailClosed indicates to not sample data (the processor will
	// fail "closed") in case of error, such as failure to parse
	// the tracestate field or missing the randomness attribute.
	//
	// By default, failure cases are sampled (the processor is
	// fails "open").  Sampling priority-based decisions are made after
	// FailClosed is processed, making it possible to sample
	// despite errors using priority.
	FailClosed bool `mapstructure:"fail_closed"`

	// SamplingPrecision is how many hex digits of sampling
	// threshold will be encoded, from 1 up to 14.  Default is 4.
	// 0 is treated as full precision.
	SamplingPrecision int `mapstructure:"sampling_precision"`

	///////
	// Logs only fields below.

	// AttributeSource (logs only) defines where to look for the attribute in from_attribute. The allowed values are
	// `traceID` or `record`. Default is `traceID`.
	AttributeSource `mapstructure:"attribute_source"`

	// FromAttribute (logs only) The optional name of a log record attribute used for sampling purposes, such as a
	// unique log record ID. The value of the attribute is only used if the trace ID is absent or if `attribute_source` is set to `record`.
	FromAttribute string `mapstructure:"from_attribute"`

	// SamplingPriority (logs only) enables using a log record attribute as the sampling priority of the log record.
	SamplingPriority string `mapstructure:"sampling_priority"`
}

var _ component.Config = (*Config)(nil)

// Validate checks if the processor configuration is valid
func (cfg *Config) Validate() error {
	pct := float64(cfg.SamplingPercentage)

	if math.IsInf(pct, 0) || math.IsNaN(pct) {
		return fmt.Errorf("sampling rate is invalid: %f%%", cfg.SamplingPercentage)
	}
	ratio := pct / 100.0

	switch {
	case ratio < 0:
		return fmt.Errorf("sampling rate is negative: %f%%", cfg.SamplingPercentage)
	case ratio == 0:
		// Special case
	case ratio < sampling.MinSamplingProbability:
		// Too-small case
		return fmt.Errorf("sampling rate is too small: %g%%", cfg.SamplingPercentage)
	default:
		// Note that ratio > 1 is specifically allowed by the README, taken to mean 100%
	}

	if cfg.AttributeSource != "" && !validAttributeSource[cfg.AttributeSource] {
		return fmt.Errorf("invalid attribute source: %v. Expected: %v or %v", cfg.AttributeSource, traceIDAttributeSource, recordAttributeSource)
	}

	if cfg.SamplingPrecision == 0 {
		return errors.New("invalid sampling precision: 0")
	} else if cfg.SamplingPrecision > sampling.NumHexDigits {
		return fmt.Errorf("sampling precision is too great, should be <= 14: %d", cfg.SamplingPrecision)
	}

	return nil
}
