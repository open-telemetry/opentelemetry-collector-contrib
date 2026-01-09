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

// Config defines the configuration for the span pruning processor.
type Config struct {
	// GroupByAttributes specifies which span attributes to use for grouping
	// similar leaf spans. Spans with the same name AND same values for these
	// attributes will be grouped together.
	// Supports glob patterns for matching attribute keys:
	//   - "db.*" matches db.operation, db.name, db.statement, etc.
	//   - "http.request.*" matches http.request.method, http.request.header, etc.
	//   - "service" matches only the exact key "service"
	// Example: ["db.*", "http.method"] or ["rpc.*"]
	GroupByAttributes []string `mapstructure:"group_by_attributes"`

	// MinSpansToAggregate is the minimum number of similar leaf spans required
	// before aggregation occurs. If a group has fewer spans, they are left unchanged.
	// Default: 5
	MinSpansToAggregate int `mapstructure:"min_spans_to_aggregate"`

	// MaxParentDepth limits how deep parent span aggregation can go above the leaf spans.
	// Set to 0 to only aggregate leaf spans (no parent aggregation).
	// Set to -1 for unlimited depth.
	// Default: 1
	MaxParentDepth int `mapstructure:"max_parent_depth"`

	// AggregationAttributePrefix is the prefix for aggregation attributes
	// added to the summary span.
	// Default: "aggregation."
	AggregationAttributePrefix string `mapstructure:"aggregation_attribute_prefix"`

	// AggregationHistogramBuckets defines the upper bounds for histogram buckets
	// used to track latency distributions of aggregated spans.
	// Example: [5*time.Millisecond, 10*time.Millisecond, 100*time.Millisecond]
	// Default: [5*time.Millisecond, 10*time.Millisecond, 25*time.Millisecond, 50*time.Millisecond, 100*time.Millisecond, 250*time.Millisecond, 500*time.Millisecond, time.Second, 2500*time.Millisecond, 5*time.Second, 10*time.Second]
	AggregationHistogramBuckets []time.Duration `mapstructure:"aggregation_histogram_buckets"`

	// EnableAttributeLossAnalysis controls whether to analyze and report attribute loss
	// during aggregation. When enabled, the processor analyzes attribute differences across spans being
	// aggregated, records histogram metrics, and adds summary attributes to aggregated spans.
	// Default: false (to reduce telemetry overhead)
	EnableAttributeLossAnalysis bool `mapstructure:"enable_attribute_loss_analysis"`
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
	if strings.TrimSpace(cfg.AggregationAttributePrefix) == "" {
		return errors.New("aggregation_attribute_prefix cannot be empty")
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

	return nil
}
