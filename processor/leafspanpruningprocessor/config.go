// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package leafspanpruningprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/leafspanpruningprocessor"

import (
	"errors"

	"go.opentelemetry.io/collector/component"
)

// Config defines the configuration for the leaf span pruning processor.
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
	// Default: 2
	MinSpansToAggregate int `mapstructure:"min_spans_to_aggregate"`

	// SummarySpanNameSuffix is appended to the original span name to create
	// the summary span name.
	// Default: "_aggregated"
	SummarySpanNameSuffix string `mapstructure:"summary_span_name_suffix"`

	// AggregationAttributePrefix is the prefix for aggregation attributes
	// added to the summary span.
	// Default: "aggregation."
	AggregationAttributePrefix string `mapstructure:"aggregation_attribute_prefix"`
}

var _ component.Config = (*Config)(nil)

// Validate checks if the processor configuration is valid
func (cfg *Config) Validate() error {
	if cfg.MinSpansToAggregate < 2 {
		return errors.New("min_spans_to_aggregate must be at least 2")
	}
	return nil
}
