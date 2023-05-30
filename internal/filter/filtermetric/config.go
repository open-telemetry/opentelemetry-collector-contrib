// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filtermetric // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filtermetric"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterset/regexp"
)

// MatchType specifies the strategy for matching against `pmetric.Metric`s. This
// is distinct from filterset.MatchType which matches against metric (and
// tracing) names only. To support matching against metric names and
// `pmetric.Metric`s, filtermetric.MatchType is effectively a superset of
// filterset.MatchType.
type MatchType string

// These are the MatchTypes that users can specify for filtering
// `pmetric.Metric`s.
const (
	Regexp           = MatchType(filterset.Regexp)
	Strict           = MatchType(filterset.Strict)
	Expr   MatchType = "expr"
)

// MatchProperties specifies the set of properties in a metric to match against and the
// type of string pattern matching to use.
type MatchProperties struct {
	// MatchType specifies the type of matching desired
	MatchType MatchType `mapstructure:"match_type"`
	// RegexpConfig specifies options for the Regexp match type
	RegexpConfig *regexp.Config `mapstructure:"regexp"`

	// MetricNames specifies the list of string patterns to match metric names against.
	// A match occurs if the metric name matches at least one string pattern in this list.
	MetricNames []string `mapstructure:"metric_names"`

	// Expressions specifies the list of expr expressions to match metrics against.
	// A match occurs if any datapoint in a metric matches at least one expression in this list.
	Expressions []string `mapstructure:"expressions"`

	// ResourceAttributes defines a list of possible resource attributes to match metrics against.
	// A match occurs if any resource attribute matches all expressions in this given list.
	ResourceAttributes []filterconfig.Attribute `mapstructure:"resource_attributes"`
}

func CreateMatchPropertiesFromDefault(properties *filterconfig.MatchProperties) *MatchProperties {
	if properties == nil {
		return nil
	}

	return &MatchProperties{
		MatchType:          MatchType(properties.Config.MatchType),
		RegexpConfig:       properties.Config.RegexpConfig,
		MetricNames:        properties.MetricNames,
		ResourceAttributes: properties.Resources,
	}
}
