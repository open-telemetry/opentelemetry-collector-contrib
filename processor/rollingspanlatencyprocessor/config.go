// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package rollingspanlatencyprocessor is documented in doc.go.
package rollingspanlatencyprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/rollingspanlatencyprocessor"

import (
	"errors"
	"time"
)

// Config defines the user-facing configuration for the rolling_span_latency
// processor. Every field maps directly to a key in the OpenTelemetry Collector
// YAML configuration file under the processor's stanza, for example:
//
//	processors:
//	  rolling_span_latency:
//	    half_life: 2h
//	    slow_threshold: 3.0
//	    very_slow_threshold: 4.0
type Config struct {
	// AttributeKey is the name of the span attribute the processor writes
	// with a value of "slow" or "very_slow" once a baseline has warmed up.
	//
	// Must be non-empty.
	AttributeKey string `mapstructure:"attribute_key"`

	// ResourceKeyAttributes lists the resource attributes, in order, that are
	// combined with the span name to form the key each rolling latency
	// baseline is tracked under.
	//
	// Must be non-empty.
	ResourceKeyAttributes []string `mapstructure:"resource_key_attributes"`

	// HalfLife controls how quickly the rolling mean/variance baseline
	// forgets older observations: after HalfLife has elapsed, an
	// observation's weight in the EWMA is halved.
	//
	// Must be greater than 0.
	HalfLife time.Duration `mapstructure:"half_life"`

	// IdleTimeout is how long a baseline key can go without a new
	// observation before it becomes eligible for eviction.
	//
	// Must be greater than 0.
	IdleTimeout time.Duration `mapstructure:"idle_timeout"`

	// EvictionInterval controls how often the processor scans for and
	// removes baseline keys that have been idle longer than IdleTimeout.
	//
	// Must be greater than 0.
	EvictionInterval time.Duration `mapstructure:"eviction_interval"`

	// SlowThreshold is the number of standard deviations above the rolling
	// mean a span's duration must exceed to be labeled "slow".
	//
	// Must be greater than 0.
	SlowThreshold float64 `mapstructure:"slow_threshold"`

	// VerySlowThreshold is the number of standard deviations above the
	// rolling mean a span's duration must exceed to be labeled "very_slow"
	// instead of "slow".
	//
	// Must be greater than SlowThreshold.
	VerySlowThreshold float64 `mapstructure:"very_slow_threshold"`

	// ChurnWarningRatio is the fraction of tracked baseline keys, evicted in
	// a single eviction sweep, above which the processor logs a warning that
	// ResourceKeyAttributes may be producing high-cardinality keys.
	//
	// Must be greater than 0 and less than or equal to 1.
	ChurnWarningRatio float64 `mapstructure:"churn_warning_ratio"`

	// MinStddev is the floor applied to a baseline's rolling standard
	// deviation before computing how many deviations a span's duration is
	// from the mean. This avoids flagging spans as slow purely from natural
	// noise in an otherwise very stable, low-variance baseline.
	//
	// Must be greater than or equal to 0.
	MinStddev time.Duration `mapstructure:"min_stddev"`

	// MaxBaselines caps the number of concurrently tracked baseline keys.
	// New keys observed once the cap is reached are dropped and left
	// unlabeled until existing keys are evicted.
	//
	// Set to 0 to disable the cap. Must be greater than or equal to 0.
	MaxBaselines int `mapstructure:"max_baselines"`

	// WarmupCount is the number of observations a baseline key must
	// accumulate before the processor starts labeling spans for that key.
	// This avoids labeling spans against a statistically unreliable, freshly
	// created baseline.
	//
	// Must be greater than 0.
	WarmupCount int `mapstructure:"warmup_count"`
}

// Validate checks that all required Config fields are within their
// acceptable ranges and returns a descriptive error if any constraint is
// violated. The OTel Collector framework calls Validate automatically during
// pipeline construction; a non-nil return value prevents the pipeline from
// starting.
func (c *Config) Validate() error {
	if c.AttributeKey == "" {
		return errors.New("attribute_key must not be empty")
	}
	if len(c.ResourceKeyAttributes) == 0 {
		return errors.New("resource_key_attributes must not be empty")
	}
	if c.HalfLife <= 0 {
		return errors.New("half_life must be greater than 0")
	}
	if c.IdleTimeout <= 0 {
		return errors.New("idle_timeout must be greater than 0")
	}
	if c.EvictionInterval <= 0 {
		return errors.New("eviction_interval must be greater than 0")
	}
	if c.SlowThreshold <= 0 {
		return errors.New("slow_threshold must be greater than 0")
	}
	if c.VerySlowThreshold <= c.SlowThreshold {
		return errors.New("very_slow_threshold must be greater than slow_threshold")
	}
	if c.ChurnWarningRatio <= 0 || c.ChurnWarningRatio > 1 {
		return errors.New("churn_warning_ratio must be greater than 0 and less than or equal to 1")
	}
	if c.MinStddev < 0 {
		return errors.New("min_stddev must be greater than or equal to 0")
	}
	if c.MaxBaselines < 0 {
		return errors.New("max_baselines must be greater than or equal to 0")
	}
	if c.WarmupCount <= 0 {
		return errors.New("warmup_count must be greater than 0")
	}
	return nil
}
