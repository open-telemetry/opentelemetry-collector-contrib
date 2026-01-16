// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package spanpruningprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/spanpruningprocessor"

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gobwas/glob"
	"go.opentelemetry.io/collector/component"
)

// OutlierMethod defines the statistical method for outlier detection.
type OutlierMethod string

const (
	// OutlierMethodIQR uses Interquartile Range for outlier detection.
	// Threshold: Q3 + (IQRMultiplier * IQR)
	OutlierMethodIQR OutlierMethod = "iqr"

	// OutlierMethodMAD uses Median Absolute Deviation for outlier detection.
	// Threshold: median + (MADMultiplier * MAD * 1.4826)
	// MAD is more robust to extreme outliers than IQR.
	OutlierMethodMAD OutlierMethod = "mad"
)

// OutlierAnalysisConfig controls outlier detection and attribute correlation.
type OutlierAnalysisConfig struct {
	// Method selects the statistical method for outlier detection.
	// Valid values: "iqr" (default), "mad"
	Method OutlierMethod `mapstructure:"method"`

	// IQRMultiplier sets the threshold for IQR-based outlier detection.
	// Outliers are spans with duration > Q3 + (IQRMultiplier * IQR).
	// Common values: 1.5 (standard), 3.0 (extreme only).
	// Default: 1.5
	IQRMultiplier float64 `mapstructure:"iqr_multiplier"`

	// MADMultiplier sets the threshold for MAD-based outlier detection.
	// Outliers are spans with duration > median + (MADMultiplier * MAD * 1.4826).
	// Common values: 2.5-3.0 (standard), 3.5+ (extreme only).
	// Default: 3.0
	MADMultiplier float64 `mapstructure:"mad_multiplier"`

	// MinGroupSize is the minimum number of spans needed for reliable
	// outlier detection. Groups smaller than this skip outlier analysis.
	// Must be at least 4 (need quartiles).
	// Default: 7
	MinGroupSize int `mapstructure:"min_group_size"`

	// CorrelationMinOccurrence is the minimum fraction of outliers that must
	// share an attribute value for it to be reported as correlated.
	// Range: (0.0, 1.0]
	// Default: 0.75 (75% of outliers must share the value)
	CorrelationMinOccurrence float64 `mapstructure:"correlation_min_occurrence"`

	// CorrelationMaxNormalOccurrence is the maximum fraction of normal spans
	// that can have the correlated value. Lower values mean stronger signal.
	// Range: [0.0, 1.0)
	// Default: 0.25 (at most 25% of normal spans can have the value)
	CorrelationMaxNormalOccurrence float64 `mapstructure:"correlation_max_normal_occurrence"`

	// MaxCorrelatedAttributes limits how many correlated attributes are
	// reported in the summary span attribute.
	// Default: 5
	MaxCorrelatedAttributes int `mapstructure:"max_correlated_attributes"`

	// PreserveOutliers controls whether outlier spans are kept as individual
	// spans instead of being aggregated. When true, only normal spans are
	// aggregated; outliers remain in the trace.
	// Default: false (aggregate all, add summary attributes)
	PreserveOutliers bool `mapstructure:"preserve_outliers"`

	// MaxPreservedOutliers limits how many outlier spans are preserved per
	// aggregation group. Spans are selected by most extreme duration first.
	// 0 = preserve all detected outliers.
	// Default: 2
	MaxPreservedOutliers int `mapstructure:"max_preserved_outliers"`

	// PreserveOnlyWithCorrelation only preserves outliers when a strong
	// attribute correlation is found. This avoids preserving outliers that
	// are just random variance.
	// Default: false
	PreserveOnlyWithCorrelation bool `mapstructure:"preserve_only_with_correlation"`
}

// Config defines the configuration options for the span pruning processor
// and the rules used to identify and aggregate similar spans.
type Config struct {
	// GroupByAttributes lists attribute patterns used to decide which leaf spans
	// belong in the same aggregation group. Spans must share the span name and
	// have identical values for every matched attribute to be grouped. Patterns
	// accept glob syntax, for example:
	//   - "db.*" matches db.operation, db.name, db.statement, etc.
	//   - "http.request.*" matches http.request.method, http.request.header, etc.
	//   - "service" matches only the exact key "service"
	// Examples: ["db.*", "http.method"], ["rpc.*"].
	GroupByAttributes []string `mapstructure:"group_by_attributes"`

	// MinSpansToAggregate is the minimum number of similar spans required before
	// aggregation occurs. Groups smaller than this threshold are preserved.
	// Default: 5
	MinSpansToAggregate int `mapstructure:"min_spans_to_aggregate"`

	// MaxParentDepth bounds how many ancestor levels above the aggregated leaves
	// can also be aggregated. Use 0 to aggregate only leaves, -1 for unlimited
	// depth, or a positive integer to cap traversal.
	// Default: 1
	MaxParentDepth int `mapstructure:"max_parent_depth"`

	// AggregationAttributePrefix prefixes all aggregation-related attributes that
	// are added to summary spans.
	// Default: "aggregation."
	AggregationAttributePrefix string `mapstructure:"aggregation_attribute_prefix"`

	// AggregationHistogramBuckets lists cumulative histogram bucket upper bounds
	// for latency tracking on aggregated spans. Empty slice disables histograms.
	// Example: [5*time.Millisecond, 10*time.Millisecond, 100*time.Millisecond]
	// Default: [5*time.Millisecond, 10*time.Millisecond, 25*time.Millisecond, 50*time.Millisecond, 100*time.Millisecond, 250*time.Millisecond, 500*time.Millisecond, time.Second, 2500*time.Millisecond, 5*time.Second, 10*time.Second]
	AggregationHistogramBuckets []time.Duration `mapstructure:"aggregation_histogram_buckets"`

	// EnableAttributeLossAnalysis toggles analysis of attribute loss during
	// aggregation. When enabled, the processor compares attribute sets across
	// aggregated spans, records loss metrics, and annotates summary spans.
	// Default: false (to reduce telemetry overhead)
	EnableAttributeLossAnalysis bool `mapstructure:"enable_attribute_loss_analysis"`

	// AttributeLossExemplarSampleRate controls the fraction of attribute-loss
	// metric recordings that include exemplars when loss analysis is enabled.
	// Range: 0.0 (disabled) to 1.0 (always). Default: 0.01 (1%).
	AttributeLossExemplarSampleRate float64 `mapstructure:"attribute_loss_exemplar_sample_rate"`

	// EnableOutlierAnalysis toggles IQR-based outlier detection and attribute
	// correlation. When enabled, adds duration_median_ns and outlier_correlated_attributes
	// to summary spans.
	// Default: false
	EnableOutlierAnalysis bool `mapstructure:"enable_outlier_analysis"`

	// OutlierAnalysis configures IQR-based outlier detection and
	// attribute correlation for aggregation groups.
	OutlierAnalysis OutlierAnalysisConfig `mapstructure:"outlier_analysis"`
}

var _ component.Config = (*Config)(nil)

// Validate checks if the processor configuration is valid
func (cfg *Config) Validate() error {
	if cfg.MinSpansToAggregate < 2 {
		return errors.New("min_spans_to_aggregate must be at least 2")
	}

	if cfg.MaxParentDepth < -1 {
		return errors.New("max_parent_depth must be -1 (unlimited) or >= 0")
	}

	// Validate AggregationAttributePrefix
	prefix := strings.TrimSpace(cfg.AggregationAttributePrefix)
	if prefix == "" {
		return errors.New("aggregation_attribute_prefix cannot be empty")
	}
	if strings.ContainsAny(prefix, " \t\n\r") {
		return errors.New("aggregation_attribute_prefix cannot contain whitespace")
	}

	// Validate GroupByAttributes glob patterns
	for i, pattern := range cfg.GroupByAttributes {
		if strings.TrimSpace(pattern) == "" {
			return fmt.Errorf("group_by_attributes[%d] cannot be empty", i)
		}
		// Try to compile the same way processor.go does to catch invalid syntax early
		_, err := glob.Compile(pattern)
		if err != nil {
			return fmt.Errorf("invalid glob pattern at group_by_attributes[%d]: %q: %w", i, pattern, err)
		}
	}

	// Validate histogram buckets
	for i, bucket := range cfg.AggregationHistogramBuckets {
		if bucket <= 0 {
			return errors.New("histogram bucket values must be positive")
		}
		if i > 0 && bucket <= cfg.AggregationHistogramBuckets[i-1] {
			return errors.New("histogram buckets must be sorted in ascending order")
		}
	}

	// Validate AttributeLossExemplarSampleRate
	if cfg.AttributeLossExemplarSampleRate < 0 || cfg.AttributeLossExemplarSampleRate > 1 {
		return errors.New("attribute_loss_exemplar_sample_rate must be between 0.0 and 1.0")
	}

	if err := cfg.OutlierAnalysis.Validate(cfg.EnableOutlierAnalysis); err != nil {
		return err
	}

	return nil
}

// Validate checks OutlierAnalysisConfig for invalid values.
func (cfg *OutlierAnalysisConfig) Validate(enabled bool) error {
	if !enabled {
		return nil // Skip validation when disabled
	}
	if cfg.Method != "" && cfg.Method != OutlierMethodIQR && cfg.Method != OutlierMethodMAD {
		return fmt.Errorf("outlier_analysis.method must be %q or %q", OutlierMethodIQR, OutlierMethodMAD)
	}
	if cfg.IQRMultiplier <= 0 {
		return errors.New("outlier_analysis.iqr_multiplier must be positive")
	}
	if cfg.MADMultiplier <= 0 {
		return errors.New("outlier_analysis.mad_multiplier must be positive")
	}
	if cfg.MinGroupSize < 4 {
		return errors.New("outlier_analysis.min_group_size must be at least 4")
	}
	if cfg.CorrelationMinOccurrence <= 0 || cfg.CorrelationMinOccurrence > 1 {
		return errors.New("outlier_analysis.correlation_min_occurrence must be in range (0.0, 1.0]")
	}
	if cfg.CorrelationMaxNormalOccurrence < 0 || cfg.CorrelationMaxNormalOccurrence >= 1 {
		return errors.New("outlier_analysis.correlation_max_normal_occurrence must be in range [0.0, 1.0)")
	}
	if cfg.MaxCorrelatedAttributes < 1 {
		return errors.New("outlier_analysis.max_correlated_attributes must be at least 1")
	}
	if cfg.PreserveOutliers && cfg.MaxPreservedOutliers < 0 {
		return errors.New("outlier_analysis.max_preserved_outliers must be >= 0")
	}
	return nil
}
