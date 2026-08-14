// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dynamicsamplingprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/dynamicsamplingprocessor"

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
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
	// RootSpanCondition is an OTTL boolean expression evaluated against every
	// span in the ottlspan context. When it returns true for a span, that span
	// triggers the trace to move from accumulation to the decision-delay phase.
	// If unset, defaults to `IsRootSpan()`: any span with an empty ParentSpanID
	// is treated as a trigger, preserving the historical behavior.
	//
	// Example use-cases:
	//   - Broaden detection to include cross-process server spans:
	//       IsRootSpan() or (span.kind == SPAN_KIND_SERVER and
	//                        resource.attributes["service.name"] == "gateway")
	//   - Accept a producer-side hint attribute:
	//       IsRootSpan() or span.attributes["otelcol.dynamic_sampling.root_span"] == true
	//   - Only trigger on explicit hints (no default):
	//       span.attributes["otelcol.dynamic_sampling.root_span"] == true
	RootSpanCondition string `mapstructure:"root_span_condition"`
	// Eviction controls what happens to the oldest pending trace when the
	// buffer is full (NumTraces reached) and a new trace arrives. Both
	// policies emit a real decision (recorded in the decision cache and, for
	// kept traces, stamped with ot=th) rather than silently dropping spans.
	Eviction EvictionConfig `mapstructure:"eviction"`
	// prevent unkeyed literal initialization
	_ struct{}
}

// defaultRootSpanCondition is applied when Config.RootSpanCondition is unset.
// It matches any span whose ParentSpanID is empty, preserving the behavior
// the processor shipped with before root_span_condition was configurable.
const defaultRootSpanCondition = "IsRootSpan()"

// EvictionPolicy selects how an evicted trace is decided.
type EvictionPolicy string

const (
	// EvictionEvaluate runs the evicted trace through the normal rules and
	// threshold path immediately, with the spans seen so far and no
	// decision_delay. Honors the configured rules at the cost of OTTL
	// evaluation per evicted span.
	EvictionEvaluate EvictionPolicy = "evaluate"
	// EvictionProbabilistic skips rule evaluation and decides the evicted
	// trace by comparing a threshold derived from SamplingPercentage against
	// the trace's randomness. Constant-time load shedding under pressure;
	// kept traces still carry a correct ot=th.
	EvictionProbabilistic EvictionPolicy = "probabilistic"
)

// EvictionConfig configures the decision made for evicted traces.
type EvictionConfig struct {
	// Policy selects the eviction decision mode. Defaults to evaluate.
	Policy EvictionPolicy `mapstructure:"policy"`
	// SamplingPercentage is the percentage of evicted traces to keep (0-100]
	// under the probabilistic policy. Required for probabilistic; must be
	// unset for evaluate.
	SamplingPercentage float64 `mapstructure:"sampling_percentage"`
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

// MatchMode controls how a rule's conditions are combined against the spans of
// an accumulated trace.
type MatchMode string

const (
	// MatchAnySpan (default) requires each condition to be satisfied by some
	// span in the trace, though not necessarily the same span. Preserves the
	// "trace has these characteristics" reading.
	MatchAnySpan MatchMode = "any_span"
	// MatchSameSpan requires that some single span in the trace satisfies all
	// the rule's conditions simultaneously.
	MatchSameSpan MatchMode = "same_span"
)

// RuleConfig defines a single rule entry: zero or more OTTL boolean conditions
// that, when all match under the configured MatchMode, select the sampler.
type RuleConfig struct {
	// Name identifies the rule in metrics and span attributes.
	Name string `mapstructure:"name"`
	// Conditions is a list of OTTL boolean expressions evaluated against the
	// spans of the accumulated trace. A rule with no conditions is a catch-all
	// (always matches).
	Conditions []string `mapstructure:"conditions"`
	// Match controls how conditions are combined. Defaults to MatchAnySpan.
	// Must be unset when Conditions is empty.
	Match MatchMode `mapstructure:"match"`
	// Sampler is the sampler invoked when this rule matches.
	Sampler SamplerConfig `mapstructure:"sampler"`
	// prevent unkeyed literal initialization
	_ struct{}
}

// SamplerConfig is a flat sampler configuration. `Type` selects the sampler
// implementation; the remaining fields apply only to the sampler types that
// use them (see the field docs). SamplerConfig.validate rejects fields set
// for a sampler type that does not use them.
type SamplerConfig struct {
	// Type is the kind of sampler to instantiate.
	Type SamplerType `mapstructure:"type"`

	// SamplingPercentage is the target percentage of traces to keep (0-100).
	// Used by: deterministic.
	SamplingPercentage float64 `mapstructure:"sampling_percentage"`

	// GoalSamplingPercentage is the target average percentage of traces to keep
	// across all sampling keys (0-100).
	// Used by: ema_dynamic.
	GoalSamplingPercentage float64 `mapstructure:"goal_sampling_percentage"`

	// GoalThroughputPerSec is the target sustained throughput in events/sec.
	// Used by: ema_throughput, windowed_throughput.
	GoalThroughputPerSec int `mapstructure:"goal_throughput_per_sec"`

	// KeyAttributes is the list of attribute names used to build the sampling
	// key. Values are sourced from resource attributes and span attributes
	// across the accumulated trace.
	// Used by: ema_dynamic, ema_throughput, windowed_throughput.
	KeyAttributes []string `mapstructure:"key_attributes"`

	// MaxKeys caps the number of distinct sampling keys the sampler tracks.
	// 0 means unlimited.
	// Used by: ema_dynamic, ema_throughput, windowed_throughput.
	MaxKeys int `mapstructure:"max_keys"`

	// AdjustmentInterval is how often the EMA recalculates rates from recent
	// observations.
	// Used by: ema_dynamic, ema_throughput.
	AdjustmentInterval time.Duration `mapstructure:"adjustment_interval"`

	// Weight is the EMA weighting factor in [0, 1). Higher values weight recent
	// observations more heavily.
	// Used by: ema_dynamic, ema_throughput.
	Weight float64 `mapstructure:"weight"`

	// UpdateFrequency is how often the windowed sampler recalculates rates.
	// Used by: windowed_throughput.
	UpdateFrequency time.Duration `mapstructure:"update_frequency"`

	// LookbackFrequency is the historical window the windowed sampler uses to
	// compute rates. Must be a multiple of UpdateFrequency.
	// Used by: windowed_throughput.
	LookbackFrequency time.Duration `mapstructure:"lookback_frequency"`

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
		if strings.HasPrefix(r.Name, "_") {
			return fmt.Errorf("rules[%d]: rule name %q is invalid: the \"_\" prefix is reserved for processor-internal decision labels", i, r.Name)
		}
		if _, dup := names[r.Name]; dup {
			return fmt.Errorf("rules[%d]: duplicate rule name %q", i, r.Name)
		}
		names[r.Name] = struct{}{}
		if err := r.validateMatch(); err != nil {
			return err
		}
		if err := r.Sampler.validate(r.Name); err != nil {
			return err
		}
	}
	if cond := c.effectiveRootSpanCondition(); cond != "" {
		settings := component.TelemetrySettings{Logger: zap.NewNop()}
		if _, err := filterottl.NewBoolExprForSpan([]string{cond}, filterottl.StandardSpanFuncs(), ottl.PropagateError, settings); err != nil {
			return fmt.Errorf("root_span_condition: %w", err)
		}
	}
	if err := c.Eviction.validate(); err != nil {
		return err
	}
	return nil
}

func (e *EvictionConfig) validate() error {
	switch e.Policy {
	case "", EvictionEvaluate:
		if e.SamplingPercentage != 0 {
			return errors.New("eviction: sampling_percentage is only used by the probabilistic policy")
		}
	case EvictionProbabilistic:
		if e.SamplingPercentage <= 0 || e.SamplingPercentage > 100 {
			return errors.New("eviction: sampling_percentage must be in (0, 100] for the probabilistic policy")
		}
	default:
		return fmt.Errorf("eviction: policy must be %q or %q", EvictionEvaluate, EvictionProbabilistic)
	}
	return nil
}

// effectiveRootSpanCondition returns the OTTL expression that should decide
// which spans trigger the accumulate to decision transition. Falls back to
// defaultRootSpanCondition when the operator did not set one.
func (c *Config) effectiveRootSpanCondition() string {
	if c.RootSpanCondition == "" {
		return defaultRootSpanCondition
	}
	return c.RootSpanCondition
}

func (r *RuleConfig) validateMatch() error {
	switch r.Match {
	case "", MatchAnySpan, MatchSameSpan:
	default:
		return fmt.Errorf("rule %q: match must be %q or %q", r.Name, MatchAnySpan, MatchSameSpan)
	}
	if len(r.Conditions) == 0 && r.Match != "" {
		return fmt.Errorf("rule %q: match cannot be set on a catch-all rule (no conditions)", r.Name)
	}
	if err := validateOTTLConditions(r.Name, r.Conditions); err != nil {
		return err
	}
	return nil
}

// validateOTTLConditions parses each condition against a nop-telemetry OTTL
// span parser to surface syntax errors during config validation, before the
// processor is instantiated. The parsed forms are discarded; the real parse
// happens once in newProcessor with the component's real telemetry settings.
func validateOTTLConditions(ruleName string, conditions []string) error {
	if len(conditions) == 0 {
		return nil
	}
	settings := component.TelemetrySettings{Logger: zap.NewNop()}
	for i, cond := range conditions {
		_, err := filterottl.NewBoolExprForSpan([]string{cond}, filterottl.StandardSpanFuncs(), ottl.PropagateError, settings)
		if err != nil {
			return fmt.Errorf("rule %q: conditions[%d]: %w", ruleName, i, err)
		}
	}
	return nil
}

// validate checks the sampler config for its declared type. It also rejects
// fields set that do not apply to the chosen type, so config typos surface at
// validation rather than being silently ignored.
func (s *SamplerConfig) validate(ruleName string) error {
	switch s.Type {
	case "":
		return fmt.Errorf("rule %q: sampler.type is required", ruleName)
	case AlwaysSample:
		return s.rejectUnusedFields(ruleName, "always_sample", nil)
	case Deterministic:
		if s.SamplingPercentage <= 0 || s.SamplingPercentage > 100 {
			return fmt.Errorf("rule %q: sampling_percentage must be in (0, 100]", ruleName)
		}
		return s.rejectUnusedFields(ruleName, "deterministic", map[string]bool{"sampling_percentage": true})
	case EMADynamic:
		if s.GoalSamplingPercentage <= 0 || s.GoalSamplingPercentage > 100 {
			return fmt.Errorf("rule %q: goal_sampling_percentage must be in (0, 100]", ruleName)
		}
		if len(s.KeyAttributes) == 0 {
			return fmt.Errorf("rule %q: key_attributes must contain at least one entry", ruleName)
		}
		if s.Weight < 0 || s.Weight >= 1 {
			return fmt.Errorf("rule %q: weight must be in [0, 1)", ruleName)
		}
		if s.MaxKeys < 0 {
			return fmt.Errorf("rule %q: max_keys must be non-negative", ruleName)
		}
		return s.rejectUnusedFields(ruleName, "ema_dynamic", map[string]bool{
			"goal_sampling_percentage": true,
			"key_attributes":           true,
			"max_keys":                 true,
			"adjustment_interval":      true,
			"weight":                   true,
		})
	case EMAThroughput:
		if s.GoalThroughputPerSec <= 0 {
			return fmt.Errorf("rule %q: goal_throughput_per_sec must be greater than zero", ruleName)
		}
		if len(s.KeyAttributes) == 0 {
			return fmt.Errorf("rule %q: key_attributes must contain at least one entry", ruleName)
		}
		if s.Weight < 0 || s.Weight >= 1 {
			return fmt.Errorf("rule %q: weight must be in [0, 1)", ruleName)
		}
		if s.MaxKeys < 0 {
			return fmt.Errorf("rule %q: max_keys must be non-negative", ruleName)
		}
		return s.rejectUnusedFields(ruleName, "ema_throughput", map[string]bool{
			"goal_throughput_per_sec": true,
			"key_attributes":          true,
			"max_keys":                true,
			"adjustment_interval":     true,
			"weight":                  true,
		})
	case WindowedThroughput:
		if s.GoalThroughputPerSec <= 0 {
			return fmt.Errorf("rule %q: goal_throughput_per_sec must be greater than zero", ruleName)
		}
		if len(s.KeyAttributes) == 0 {
			return fmt.Errorf("rule %q: key_attributes must contain at least one entry", ruleName)
		}
		if s.UpdateFrequency < 0 {
			return fmt.Errorf("rule %q: update_frequency must be non-negative", ruleName)
		}
		if s.LookbackFrequency < 0 {
			return fmt.Errorf("rule %q: lookback_frequency must be non-negative", ruleName)
		}
		if s.MaxKeys < 0 {
			return fmt.Errorf("rule %q: max_keys must be non-negative", ruleName)
		}
		return s.rejectUnusedFields(ruleName, "windowed_throughput", map[string]bool{
			"goal_throughput_per_sec": true,
			"key_attributes":          true,
			"max_keys":                true,
			"update_frequency":        true,
			"lookback_frequency":      true,
		})
	default:
		return fmt.Errorf("rule %q: unknown sampler.type %q", ruleName, s.Type)
	}
}

// rejectUnusedFields returns an error if any field is set that does not apply
// to the sampler type. The `allowed` set names the fields the type does use;
// every other non-zero field is reported.
func (s *SamplerConfig) rejectUnusedFields(ruleName, typeName string, allowed map[string]bool) error {
	set := func(name string, isSet bool) error {
		if isSet && !allowed[name] {
			return fmt.Errorf("rule %q: %s does not use %s", ruleName, typeName, name)
		}
		return nil
	}
	if err := set("sampling_percentage", s.SamplingPercentage != 0); err != nil {
		return err
	}
	if err := set("goal_sampling_percentage", s.GoalSamplingPercentage != 0); err != nil {
		return err
	}
	if err := set("goal_throughput_per_sec", s.GoalThroughputPerSec != 0); err != nil {
		return err
	}
	if err := set("key_attributes", len(s.KeyAttributes) > 0); err != nil {
		return err
	}
	if err := set("max_keys", s.MaxKeys != 0); err != nil {
		return err
	}
	if err := set("adjustment_interval", s.AdjustmentInterval != 0); err != nil {
		return err
	}
	if err := set("weight", s.Weight != 0); err != nil {
		return err
	}
	if err := set("update_frequency", s.UpdateFrequency != 0); err != nil {
		return err
	}
	if err := set("lookback_frequency", s.LookbackFrequency != 0); err != nil {
		return err
	}
	return nil
}
