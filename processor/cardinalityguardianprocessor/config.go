// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package cardinalityguardianprocessor is documented in processor.go.
package cardinalityguardianprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/cardinalityguardianprocessor"

import (
	"errors"
	"fmt"
	"strings"
)

// Config defines the user-facing configuration for the cardinality_guardian processor.
type Config struct {
	// MaxCardinalityDeltaPerEpoch is the maximum number of new unique label
	// values allowed for a single metric+label-key combination within one epoch.
	// Must be greater than 0.
	MaxCardinalityDeltaPerEpoch int `mapstructure:"max_cardinality_delta_per_epoch"`

	// EpochDurationSeconds controls how often the sliding cardinality window advances.
	// Must be at least 10 seconds.
	EpochDurationSeconds int `mapstructure:"epoch_duration_seconds"`

	// NeverDropLabels is the list of label keys the processor will never strip or tag.
	NeverDropLabels []string `mapstructure:"never_drop_labels"`

	// TagOnly switches the processor to dual-route tagging mode.
	// Deprecated: Use EnforcementMode instead. TagOnly: true maps to
	// EnforcementMode "tag_only"; TagOnly: false maps to "strip_and_reaggregate".
	TagOnly bool `mapstructure:"tag_only"`

	// EnforcementMode controls how the processor handles high-cardinality attributes.
	//
	//   - "tag_only" — preserves all attributes and injects "otel.metric.overflow: true".
	//   - "overflow_attribute" — replaces the high-cardinality value with a sentinel string.
	//   - "strip_and_reaggregate" — removes the attribute and merges colliding data points inline.
	//     Supports Delta Sum and Gauge. Cumulative Sums, Histograms, and
	//     ExponentialHistograms fall back to tag_only with a warning log.
	EnforcementMode EnforcementMode `mapstructure:"enforcement_mode"`

	// EstimatedCostPerMetricMonth is used to populate the estimated savings counter.
	// Set to 0 to disable. Must be >= 0.
	EstimatedCostPerMetricMonth float64 `mapstructure:"estimated_cost_per_metric_month"`

	// TopOffendersCount is the number of highest-delta (metric, label) pairs to report.
	// Set to 0 to disable. Must be between 0 and 500.
	TopOffendersCount int `mapstructure:"top_offenders_count"`

	// MaxTrackerCount is the maximum number of concurrent (metric, label) tracking sketches.
	// Set to 0 to disable. Must be between 0 and 10,000,000.
	MaxTrackerCount int `mapstructure:"max_tracker_count"`

	// MetricOverrides allows per-metric cardinality limits that override the global default.
	MetricOverrides map[string]int `mapstructure:"metric_overrides"`

	// DropLogMaxPerEpoch caps the number of drop Warn logs per epoch.
	// Set to 0 to disable the cap. Must be >= 0.
	DropLogMaxPerEpoch int `mapstructure:"drop_log_max_per_epoch"`
}

// EnforcementMode determines how the processor handles attributes that exceed
// the cardinality delta threshold.
type EnforcementMode string

const (
	// EnforcementTagOnly preserves all attributes and injects "otel.metric.overflow: true".
	EnforcementTagOnly EnforcementMode = "tag_only"

	// EnforcementOverflowAttribute replaces the high-cardinality attribute value
	// with a sentinel "otel.cardinality_overflow" string.
	EnforcementOverflowAttribute EnforcementMode = "overflow_attribute"

	// EnforcementStripAndReaggregate removes the offending attribute and performs
	// inline spatial reaggregation to merge colliding data points.
	EnforcementStripAndReaggregate EnforcementMode = "strip_and_reaggregate"
)

// overflowSentinel is the value used when EnforcementOverflowAttribute mode is active.
const overflowSentinel = "otel.cardinality_overflow"

// ResolvedEnforcementMode returns the effective enforcement mode, normalizing
// to lowercase and resolving the deprecated TagOnly field if EnforcementMode is not set.
func (c *Config) ResolvedEnforcementMode() EnforcementMode {
	if c.EnforcementMode != "" {
		return EnforcementMode(strings.ToLower(string(c.EnforcementMode)))
	}
	if c.TagOnly {
		return EnforcementTagOnly
	}
	return EnforcementStripAndReaggregate
}

// Validate checks that all Config fields are within acceptable ranges.
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
