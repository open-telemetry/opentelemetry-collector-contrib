// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dynamicsamplingprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/dynamicsamplingprocessor"

import (
	"errors"
	"fmt"
	"time"
)

// SamplerType identifies the kind of sampler attached to a rule.
type SamplerType string

const (
	// AlwaysSample keeps every trace that matches the rule.
	AlwaysSample SamplerType = "always_sample"
	// Deterministic samples a fixed percentage of traces deterministically by traceID.
	Deterministic SamplerType = "deterministic"
	// EMADynamic adjusts sample rates per traffic key using an exponential moving average.
	EMADynamic SamplerType = "ema_dynamic"
	// EMAThroughput targets a fixed events-per-second rate using an EMA over key frequencies.
	EMAThroughput SamplerType = "ema_throughput"
	// WindowedThroughput targets a fixed events-per-second rate using a sliding window over key frequencies.
	WindowedThroughput SamplerType = "windowed_throughput"
)

// Config holds the top-level configuration for the dynamic sampling processor.
type Config struct {
	// TraceTimeout is the maximum time a trace can sit in the accumulation buffer
	// before its decision is forced. Acts as a safety net for traces that never
	// receive a root span. Triggered by a timeout, the decision is made after
	// DecisionDelay has elapsed.
	TraceTimeout time.Duration `mapstructure:"trace_timeout"`
	// DecisionDelay is the pause between a triggering event (root span arrival or
	// trace timeout) and the actual decision evaluation. Allows in-flight
	// straggler spans to land before the trace is decided.
	DecisionDelay time.Duration `mapstructure:"decision_delay"`
	// NumTraces is the maximum number of traces held in the in-memory buffer at
	// any time. Older traces are evicted when this limit is exceeded.
	NumTraces int `mapstructure:"num_traces"`
	// DecisionCache configures the LRU caches that record decisions for already
	// decided traces, so late-arriving spans receive the same treatment as the
	// original trace.
	DecisionCache DecisionCacheConfig `mapstructure:"decision_cache"`
	// Rules are evaluated in order, first match wins. The matched rule's sampler
	// produces the sample rate for the trace.
	Rules []RuleConfig `mapstructure:"rules"`
	// prevent unkeyed literal initialization
	_ struct{}
}

// DecisionCacheConfig sizes the LRU caches that record sampling decisions.
// When a span arrives for a traceID that already has a recorded decision, the
// processor short-circuits the accumulation path: sampled traces are forwarded
// with the original rule and ot=th annotations; not-sampled traces are dropped.
// A size of 0 disables that cache.
type DecisionCacheConfig struct {
	// SampledCacheSize is the maximum number of sampled traces tracked for
	// late-span attribution. 0 disables the sampled cache.
	SampledCacheSize int `mapstructure:"sampled_cache_size"`
	// NonSampledCacheSize is the maximum number of not-sampled traces tracked.
	// 0 disables the not-sampled cache.
	NonSampledCacheSize int `mapstructure:"non_sampled_cache_size"`
	// prevent unkeyed literal initialization
	_ struct{}
}

// RuleConfig defines a single rule entry: zero or more conditions that, when
// all match, select the configured sampler.
type RuleConfig struct {
	// Name identifies the rule in metrics and span attributes.
	Name string `mapstructure:"name"`
	// Conditions is a list of simple match expressions evaluated against trace
	// data. A rule with no conditions is a catch-all (always matches).
	Conditions []string `mapstructure:"conditions"`
	// Sampler is the sampler invoked when this rule matches.
	Sampler SamplerConfig `mapstructure:"sampler"`
	// prevent unkeyed literal initialization
	_ struct{}
}

// SamplerConfig selects a sampler type and its configuration.
type SamplerConfig struct {
	// Type is the kind of sampler to instantiate.
	Type SamplerType `mapstructure:"type"`
	// Deterministic holds settings for the deterministic sampler.
	Deterministic DeterministicConfig `mapstructure:"deterministic"`
	// EMADynamic holds settings for the EMA dynamic sampler.
	EMADynamic EMADynamicConfig `mapstructure:"ema_dynamic"`
	// EMAThroughput holds settings for the EMA throughput sampler.
	EMAThroughput EMAThroughputConfig `mapstructure:"ema_throughput"`
	// WindowedThroughput holds settings for the windowed throughput sampler.
	WindowedThroughput WindowedThroughputConfig `mapstructure:"windowed_throughput"`
	// prevent unkeyed literal initialization
	_ struct{}
}

// DeterministicConfig configures the deterministic sampler.
type DeterministicConfig struct {
	// SamplingPercentage is the target percentage of traces to keep (0-100).
	SamplingPercentage float64 `mapstructure:"sampling_percentage"`
	// prevent unkeyed literal initialization
	_ struct{}
}

// EMAThroughputConfig configures the EMA throughput sampler, which targets a
// fixed events-per-second rate while preserving per-key proportions via EMA.
// See dynsampler-go's EMAThroughput for the underlying algorithm.
type EMAThroughputConfig struct {
	// GoalThroughputPerSec is the target sustained throughput in events/sec.
	GoalThroughputPerSec int `mapstructure:"goal_throughput_per_sec"`
	// KeyFields is the list of attribute names used to build the sampling key.
	KeyFields []string `mapstructure:"key_fields"`
	// InitialSamplingRate is the rate used until the first adjustment interval
	// completes. 0 lets the sampler choose its default.
	InitialSamplingRate int `mapstructure:"initial_sampling_rate"`
	// AdjustmentInterval is how often the EMA recalculates rates from recent
	// observations.
	AdjustmentInterval time.Duration `mapstructure:"adjustment_interval"`
	// Weight is the EMA weighting factor in (0, 1). Higher values weight recent
	// observations more heavily.
	Weight float64 `mapstructure:"weight"`
	// MaxKeys caps the number of distinct keys tracked. 0 means unlimited.
	MaxKeys int `mapstructure:"max_keys"`
	// prevent unkeyed literal initialization
	_ struct{}
}

// WindowedThroughputConfig configures the windowed throughput sampler, which
// adjusts rates faster than EMA by decoupling the update frequency from the
// lookback window. See dynsampler-go's WindowedThroughput for the underlying
// algorithm.
type WindowedThroughputConfig struct {
	// GoalThroughputPerSec is the target sustained throughput in events/sec.
	GoalThroughputPerSec float64 `mapstructure:"goal_throughput_per_sec"`
	// KeyFields is the list of attribute names used to build the sampling key.
	KeyFields []string `mapstructure:"key_fields"`
	// UpdateFrequency is how often the sampler recalculates rates.
	UpdateFrequency time.Duration `mapstructure:"update_frequency"`
	// LookbackFrequency is the historical window the sampler uses to compute
	// rates. Must be a multiple of UpdateFrequency.
	LookbackFrequency time.Duration `mapstructure:"lookback_frequency"`
	// MaxKeys caps the number of distinct keys tracked. 0 means unlimited.
	MaxKeys int `mapstructure:"max_keys"`
	// prevent unkeyed literal initialization
	_ struct{}
}

// EMADynamicConfig configures the EMA dynamic sampler. See dynsampler-go's
// EMASampleRate for the underlying algorithm.
type EMADynamicConfig struct {
	// GoalSamplingPercentage is the target average percentage of traces to keep
	// across all keys (0-100).
	GoalSamplingPercentage float64 `mapstructure:"goal_sampling_percentage"`
	// KeyFields is the list of attribute names used to build the sampling key.
	// Values are sourced from resource attributes and span attributes across the
	// accumulated trace.
	KeyFields []string `mapstructure:"key_fields"`
	// AdjustmentInterval is how often the EMA recalculates rates from recent
	// observations.
	AdjustmentInterval time.Duration `mapstructure:"adjustment_interval"`
	// Weight is the EMA weighting factor in (0, 1). Higher values weight recent
	// observations more heavily.
	Weight float64 `mapstructure:"weight"`
	// MaxKeys caps the number of distinct keys tracked. 0 means unlimited.
	MaxKeys int `mapstructure:"max_keys"`
	// prevent unkeyed literal initialization
	_ struct{}
}

// Validate checks the processor configuration for obvious errors.
func (c *Config) Validate() error {
	if c.TraceTimeout <= 0 {
		return errors.New("trace_timeout must be greater than zero")
	}
	if c.DecisionDelay <= 0 {
		return errors.New("decision_delay must be greater than zero")
	}
	if c.NumTraces <= 0 {
		return errors.New("num_traces must be greater than zero")
	}
	if c.DecisionCache.SampledCacheSize < 0 {
		return errors.New("decision_cache.sampled_cache_size must be non-negative")
	}
	if c.DecisionCache.NonSampledCacheSize < 0 {
		return errors.New("decision_cache.non_sampled_cache_size must be non-negative")
	}
	if len(c.Rules) == 0 {
		return errors.New("at least one rule is required")
	}
	names := make(map[string]struct{}, len(c.Rules))
	for i := range c.Rules {
		r := &c.Rules[i]
		if r.Name == "" {
			return fmt.Errorf("rules[%d]: name is required", i)
		}
		if _, dup := names[r.Name]; dup {
			return fmt.Errorf("rules[%d]: duplicate rule name %q", i, r.Name)
		}
		names[r.Name] = struct{}{}
		if err := r.Sampler.validate(r.Name); err != nil {
			return err
		}
	}
	return nil
}

func (s *SamplerConfig) validate(ruleName string) error {
	switch s.Type {
	case AlwaysSample:
		return nil
	case Deterministic:
		if s.Deterministic.SamplingPercentage <= 0 || s.Deterministic.SamplingPercentage > 100 {
			return fmt.Errorf("rule %q: deterministic.sampling_percentage must be in (0, 100]", ruleName)
		}
		return nil
	case EMADynamic:
		c := s.EMADynamic
		if c.GoalSamplingPercentage <= 0 || c.GoalSamplingPercentage > 100 {
			return fmt.Errorf("rule %q: ema_dynamic.goal_sampling_percentage must be in (0, 100]", ruleName)
		}
		if len(c.KeyFields) == 0 {
			return fmt.Errorf("rule %q: ema_dynamic.key_fields must contain at least one entry", ruleName)
		}
		if c.Weight < 0 || c.Weight >= 1 {
			return fmt.Errorf("rule %q: ema_dynamic.weight must be in [0, 1)", ruleName)
		}
		if c.MaxKeys < 0 {
			return fmt.Errorf("rule %q: ema_dynamic.max_keys must be non-negative", ruleName)
		}
		return nil
	case EMAThroughput:
		c := s.EMAThroughput
		if c.GoalThroughputPerSec <= 0 {
			return fmt.Errorf("rule %q: ema_throughput.goal_throughput_per_sec must be greater than zero", ruleName)
		}
		if len(c.KeyFields) == 0 {
			return fmt.Errorf("rule %q: ema_throughput.key_fields must contain at least one entry", ruleName)
		}
		if c.Weight < 0 || c.Weight >= 1 {
			return fmt.Errorf("rule %q: ema_throughput.weight must be in [0, 1)", ruleName)
		}
		if c.MaxKeys < 0 {
			return fmt.Errorf("rule %q: ema_throughput.max_keys must be non-negative", ruleName)
		}
		return nil
	case WindowedThroughput:
		c := s.WindowedThroughput
		if c.GoalThroughputPerSec <= 0 {
			return fmt.Errorf("rule %q: windowed_throughput.goal_throughput_per_sec must be greater than zero", ruleName)
		}
		if len(c.KeyFields) == 0 {
			return fmt.Errorf("rule %q: windowed_throughput.key_fields must contain at least one entry", ruleName)
		}
		if c.UpdateFrequency < 0 {
			return fmt.Errorf("rule %q: windowed_throughput.update_frequency must be non-negative", ruleName)
		}
		if c.LookbackFrequency < 0 {
			return fmt.Errorf("rule %q: windowed_throughput.lookback_frequency must be non-negative", ruleName)
		}
		if c.MaxKeys < 0 {
			return fmt.Errorf("rule %q: windowed_throughput.max_keys must be non-negative", ruleName)
		}
		return nil
	case "":
		return fmt.Errorf("rule %q: sampler.type is required", ruleName)
	default:
		return fmt.Errorf("rule %q: unknown sampler.type %q", ruleName, s.Type)
	}
}
