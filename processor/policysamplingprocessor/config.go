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

package policysamplingprocessor

import (
	"go.opentelemetry.io/collector/config/configmodels"
)

// PolicyType indicates the type of sampling policy.
type PolicyType string

const (
	// AlwaysSample samples all traces, typically used for debugging.
	AlwaysSample PolicyType = "always_sample"
	// NumericAttribute sample traces that have a given numeric attribute in a specified
	// range, e.g.: attribute "http.status_code" >= 399 and <= 999.
	NumericAttribute PolicyType = "numeric_attribute"
	// StringAttribute sample traces that a attribute, of type string, matching
	// one of the listed values.
	StringAttribute PolicyType = "string_attribute"
	// RateLimiting allows all traces until the specified limits are satisfied.
	RateLimiting PolicyType = "rate_limiting"
)

// PolicyCfg holds the common configuration to all policies.
type PolicyCfg struct {
	// Name given to the instance of the policy to make easy to identify it in metrics and logs.
	Name string `mapstructure:"name"`
	// Type of the policy this will be used to match the proper configuration of the policy.
	Type PolicyType `mapstructure:"type"`
	// Configs for numeric attribute filter sampling policy evaluator.
	NumericAttributeCfg NumericAttributeCfg `mapstructure:"numeric_attribute"`
	// Configs for string attribute filter sampling policy evaluator.
	StringAttributeCfg StringAttributeCfg `mapstructure:"string_attribute"`
	// Configs for rate limiting filter sampling policy evaluator.
	RateLimitingCfg RateLimitingCfg `mapstructure:"rate_limiting"`
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

// RateLimitingCfg holds the configurable settings to create a rate limiting
// sampling policy evaluator.
type RateLimitingCfg struct {
	// SpansPerSecond sets the limit on the maximum nuber of spans that can be processed each second.
	SpansPerSecond int64 `mapstructure:"spans_per_second"`
}

// Config holds the configuration for policy sampling.
type Config struct {
	configmodels.ProcessorSettings `mapstructure:",squash"`
	// PolicyCfgs sets the policy sampling policy which makes a sampling decision for a given trace when requested.
	PolicyCfgs []PolicyCfg `mapstructure:"policies"`
}
