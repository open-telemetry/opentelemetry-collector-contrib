// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package config

import (
	"time"

	"go.opentelemetry.io/collector/config"
)

// PolicyCfg holds the common configuration to all policies.
type PolicyCfg struct {
	// Name given to the instance of the policy to make easy to identify it in metrics and logs.
	Name string `mapstructure:"name"`
	// Configs for numeric attribute filter sampling policy evaluator.
	NumericAttributeCfg *NumericAttributeCfg `mapstructure:"numeric_attribute"`
	// Configs for string attribute filter sampling policy evaluator.
	StringAttributeCfg *StringAttributeCfg `mapstructure:"string_attribute"`
	// Configs for properties sampling policy evaluator.
	PropertiesCfg PropertiesCfg `mapstructure:"properties"`
	// SpansPerSecond specifies the rule budget that should never be exceeded for it
	SpansPerSecond int64 `mapstructure:"spans_per_second"`
	// InvertMatch specifies if the match should be inverted. Default: false
	InvertMatch bool `mapstructure:"invert_match"`
}

// PropertiesCfg holds the configurable settings to create a duration filter
type PropertiesCfg struct {
	// NamePattern (optional) describes a regular expression that must be met by any span operation name.
	NamePattern *string `mapstructure:"name_pattern"`
	// MinDuration (optional) is the minimum duration of trace to be considered a match.
	MinDuration *time.Duration `mapstructure:"min_duration"`
	// MinNumberOfSpans (optional) is the minimum number spans that must be present in a matching trace.
	MinNumberOfSpans *int `mapstructure:"min_number_of_spans"`
}

// NumericAttributeCfg holds the configurable settings to create a numeric attribute filter
// sampling policy evaluator.
type NumericAttributeCfg struct {
	// Tag that the filter is going to be matching against.
	Key string `mapstructure:"key"`
	// MinValue is the minimum value of the attribute to be considered a match.
	MinValue int64 `mapstructure:"min_value"`
	// MaxValue is the maximum value of the attribute to be considered a match.
	MaxValue int64 `mapstructure:"max_value"`
}

// StringAttributeCfg holds the configurable settings to create a string attribute filter
// sampling policy evaluator.
type StringAttributeCfg struct {
	// Tag that the filter is going to be matching against.
	Key string `mapstructure:"key"`
	// Values is the set of attribute values that if any is equal to the actual attribute value to be considered a match.
	Values []string `mapstructure:"values"`
}

// Config holds the configuration for cascading-filter-based sampling.
type Config struct {
	*config.ProcessorSettings `mapstructure:"-"`
	// DecisionWait is the desired wait time from the arrival of the first span of
	// trace until the decision about sampling it or not is evaluated.
	DecisionWait time.Duration `mapstructure:"decision_wait"`
	// SpansPerSecond specifies the total budget that should never be exceeded
	SpansPerSecond int64 `mapstructure:"spans_per_second"`
	// ProbabilisticFilteringRatio describes which part (0.0-1.0) of the SpansPerSecond budget
	// is exclusively allocated for probabilistically selected spans
	ProbabilisticFilteringRatio *float32 `mapstructure:"probabilistic_filtering_ratio"`
	// NumTraces is the number of traces kept on memory. Typically most of the data
	// of a trace is released after a sampling decision is taken.
	NumTraces uint64 `mapstructure:"num_traces"`
	// ExpectedNewTracesPerSec sets the expected number of new traces sending to the Cascading Filter processor
	// per second. This helps with allocating data structures with closer to actual usage size.
	ExpectedNewTracesPerSec uint64 `mapstructure:"expected_new_traces_per_sec"`
	// PolicyCfgs sets the cascading-filter-based sampling policy which makes a sampling decision
	// for a given trace when requested.
	PolicyCfgs []PolicyCfg `mapstructure:"policies"`
}
