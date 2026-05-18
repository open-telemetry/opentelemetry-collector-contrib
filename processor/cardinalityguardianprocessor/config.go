// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package cardinalityguardianprocessor is documented in doc.go.
package cardinalityguardianprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/cardinalityguardianprocessor"

import (
	"errors"
	"fmt"
	"strings"
)

// Config defines the user-facing configuration for the cardinality_guardian
// processor. Every field maps directly to a key in the OpenTelemetry Collector
// YAML configuration file under the processor's stanza, for example:
//
//	processors:
//	  cardinality_guardian:
//	    max_cardinality_delta_per_epoch: 500
//	    epoch_duration_seconds: 300
//	    never_drop_labels: [region, http.status_code]
//	    enforcement_mode: tag_only
//	    estimated_cost_per_metric_month: 0.05
type Config struct {
	// MaxCardinalityDeltaPerEpoch is the maximum number of new unique label
	// values that are allowed for a single metric+label-key combination within
	// one epoch. Once this threshold is exceeded, additional unique values are
	// handled according to the configured EnforcementMode.
	//
	// The processor measures cardinality growth using a HyperLogLog sketch and
	// compares the current epoch's estimate against the previous epoch's
	// estimate. Only the *delta* (new unique values seen this epoch) is checked,
	// not the absolute cardinality. This prevents the processor from penalizing
	// stable high-cardinality metrics that have already reached a steady state.
	//
	// Must be greater than 0.
	MaxCardinalityDeltaPerEpoch int `mapstructure:"max_cardinality_delta_per_epoch"`

	// EpochDurationSeconds controls how often the sliding cardinality window
	// advances. At the end of each epoch the processor promotes the current
	// HyperLogLog sketch to "previous" and starts a fresh sketch for the new
	// epoch. The delta check then measures growth relative to the boundary of
	// the last epoch, not the lifetime of the processor.
	//
	// Shorter epochs are more sensitive to sudden cardinality explosions but
	// may produce noisier decisions for metrics with naturally bursty label
	// spaces. Longer epochs smooth out transient bursts at the cost of
	// slower reaction time.
	//
	// Must be at least 10 seconds to avoid runaway ticker behavior.
	EpochDurationSeconds int `mapstructure:"epoch_duration_seconds"`

	// NeverDropLabels is the list of label keys that the processor will never
	// strip or tag, regardless of how high their cardinality grows. Use this
	// for labels whose values are essential for query correctness, such as
	// "region", "http.status_code", or "service.name".
	//
	// The lookup is O(1) via a pre-built map; the slice is only read at
	// construction time and never accessed in the hot path.
	NeverDropLabels []string `mapstructure:"never_drop_labels"`

	// EnforcementMode controls how the processor handles high-cardinality
	// attributes once the delta threshold is exceeded. Three modes are available:
	//
	//   - "tag_only" — preserves all attributes and injects
	//     "otel.metric.overflow: true" for downstream routing. No data mutation.
	//     This is the safest mode and recommended for initial deployment.
	//
	//   - "overflow_attribute" — replaces the high-cardinality attribute value
	//     with a sentinel "otel.cardinality_overflow" string. All overflowed
	//     data points merge into a single overflow bucket. This is OTel-SDK-
	//     spec-aligned and avoids the Single-Writer violation because all
	//     overflow points share one identity.
	//
	//   - "strip_and_reaggregate" — removes the offending attribute and performs
	//     inline spatial reaggregation to merge data points that now share the
	//     same identity. Currently supports Delta Sum and Gauge metric types.
	//     Cumulative Sums, Histograms, and ExponentialHistograms fall back to
	//     tag_only behavior with a warning log.
	//
	// If empty, defaults to "tag_only".
	EnforcementMode EnforcementMode `mapstructure:"enforcement_mode"`

	// EstimatedCostPerMetricMonth configures the theoretical cost per active time-series. This is
	// used solely to populate the "otelcol_processor_cardinality_estimated_savings_dollars_total" OTel counter
	// emitted by this processor. It allows platform owners to quantify FinOps ROI directly in the processor's
	// cardinality-enforcement logic.
	//
	// A reasonable starting point is $0.05/metric/month, which corresponds
	// roughly to the per-series pricing of managed Prometheus offerings.
	// Set to 0.0 to disable cost tracking without affecting enforcement.
	//
	// Must be ≥ 0.
	EstimatedCostPerMetricMonth float64 `mapstructure:"estimated_cost_per_metric_month"`

	// TopOffendersCount is the number of highest-delta (metric, label) pairs
	// to report via the "otelcol_processor_cardinality_top_offenders" gauge. The snapshot is
	// computed once per epoch rotation, so it adds no hot-path cost.
	//
	// Set to 0 to disable the Top-N gauge entirely.
	// Must be ≥ 0.
	TopOffendersCount int `mapstructure:"top_offenders_count"`

	// MaxTrackerCount is the absolute maximum number of concurrent
	// (metric, label) tracking sketches across all shards.
	// If this limit is reached, new (metric, label) pairs are silently
	// ignored and passed through until existing trackers are evicted.
	//
	// Set to 0 to disable the limit entirely (allow unlimited growth).
	// Must be ≥ 0 and ≤ 10,000,000.
	MaxTrackerCount int `mapstructure:"max_tracker_count"`

	// MetricOverrides allows per-metric cardinality limits that override the
	// global MaxCardinalityDeltaPerEpoch. This is useful when specific metrics
	// (e.g. http.server.request.duration) legitimately need higher headroom
	// than the global default, while keeping the global safety net tight.
	//
	// Unspecified metrics fall back to MaxCardinalityDeltaPerEpoch.
	// Each override value must be > 0.
	MetricOverrides map[string]int `mapstructure:"metric_overrides"`

	// DropLogMaxPerEpoch caps the number of "Dropping high-cardinality
	// attribute" Warn logs emitted per epoch. After this many warnings,
	// further drops are silently counted and a single summary line is
	// emitted at the next epoch rotation.
	//
	// Set to 0 to disable the cap (log every drop — not recommended at scale).
	// Must be ≥ 0.
	DropLogMaxPerEpoch int `mapstructure:"drop_log_max_per_epoch"`
}

// EnforcementMode determines how the processor handles attributes that exceed
// the cardinality delta threshold.
type EnforcementMode string

const (
	// EnforcementTagOnly preserves all attributes and injects
	// "otel.metric.overflow: true" for downstream routing decisions.
	EnforcementTagOnly EnforcementMode = "tag_only"

	// EnforcementOverflowAttribute replaces the high-cardinality attribute
	// value with a sentinel "otel.cardinality_overflow" string. All overflowed
	// data points for a given (metric, attribute_key) collapse into a single
	// overflow identity, avoiding the Single-Writer violation.
	EnforcementOverflowAttribute EnforcementMode = "overflow_attribute"

	// EnforcementStripAndReaggregate removes the offending attribute and
	// performs inline spatial reaggregation to merge data points that now
	// share the same identity. Currently supports Delta Sum and Gauge.
	EnforcementStripAndReaggregate EnforcementMode = "strip_and_reaggregate"
)

// resolvedEnforcementMode returns the effective enforcement mode.
// If EnforcementMode is not explicitly set, it defaults to tag_only.
func (c *Config) resolvedEnforcementMode() EnforcementMode {
	if c.EnforcementMode != "" {
		return EnforcementMode(strings.ToLower(string(c.EnforcementMode)))
	}
	return EnforcementTagOnly
}

// Validate checks that all required Config fields are within their acceptable
// ranges and returns a descriptive error if any constraint is violated. The
// OTel Collector framework calls Validate automatically during pipeline
// construction; a non-nil return value prevents the pipeline from starting.
func (c *Config) Validate() error {
	if c.MaxCardinalityDeltaPerEpoch <= 0 {
		return errors.New("max_cardinality_delta_per_epoch must be greater than 0")
	}
	if c.EpochDurationSeconds < 10 {
		return errors.New("epoch_duration_seconds must be at least 10")
	}
	if c.EstimatedCostPerMetricMonth < 0 {
		return errors.New("estimated_cost_per_metric_month cannot be negative")
	}
	if c.TopOffendersCount < 0 || c.TopOffendersCount > 500 {
		return errors.New("top_offenders_count must be between 0 and 500")
	}
	if c.MaxTrackerCount < 0 || c.MaxTrackerCount > 10000000 {
		return errors.New("max_tracker_count must be between 0 and 10,000,000")
	}
	for name, limit := range c.MetricOverrides {
		if name == "" {
			return errors.New("metric_overrides contains an empty metric name")
		}
		if limit <= 0 {
			return fmt.Errorf("metric_overrides[%q] must be greater than 0", name)
		}
	}
	if c.DropLogMaxPerEpoch < 0 {
		return errors.New("drop_log_max_per_epoch must be >= 0")
	}
	if c.EnforcementMode != "" {
		normalized := EnforcementMode(strings.ToLower(string(c.EnforcementMode)))
		switch normalized {
		case EnforcementTagOnly, EnforcementOverflowAttribute, EnforcementStripAndReaggregate:
			// valid
		default:
			return fmt.Errorf("enforcement_mode must be one of: tag_only, overflow_attribute, strip_and_reaggregate; got %q", c.EnforcementMode)
		}
	}
	return nil
}
