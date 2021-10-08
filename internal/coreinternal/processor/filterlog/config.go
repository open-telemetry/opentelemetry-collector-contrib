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

package filterlog

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/processor/filterconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/processor/filterset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/processor/filterset/regexp"
)

// MatchType specifies the strategy for matching against `pdata.Log`s. This
// is distinct from filterset.MatchType which matches against metric (and
// tracing) names only. To support matching against log names and
// `pdata.Log`s, filterlog.MatchType is effectively a superset of
// filterset.MatchType.
type MatchType string

// These are the MatchTypes that users can specify for filtering
// `pdata.Metric`s.
const (
	Regexp           = MatchType(filterset.Regexp)
	Strict           = MatchType(filterset.Strict)
	Expr   MatchType = "expr"
)

// LogMatchProperties specifies the set of properties in a log to match against and the
// type of string pattern matching to use.
type LogMatchProperties struct {
	// MatchType specifies the type of matching desired
	MatchType MatchType `mapstructure:"match_type"`

	// RegexpConfig specifies options for the Regexp match type
	RegexpConfig *regexp.Config `mapstructure:"regexp"`

	// Expressions specifies the list of expr expressions to match metrics against.
	// A match occurs if any datapoint in a metric matches at least one expression in this list.
	Expressions []string `mapstructure:"expressions"`

	// ResourceAttributes defines a list of possible resource attributes to match logs against.
	// A match occurs if any resource attribute matches all expressions in this given list.
	ResourceAttributes []filterconfig.Attribute `mapstructure:"resource_attributes"`

	// RecordAttributes defines a list of possible record attributes to match logs against.
	// A match occurs if any record attribute matches at least one expression in this given list.
	RecordAttributes []filterconfig.Attribute `mapstructure:"record_attributes"`
}

func (lmp *LogMatchProperties) convertToFilterConfig() *filterconfig.MatchProperties {
	return &filterconfig.MatchProperties{
		Config: filterset.Config{
			MatchType:    filterset.MatchType(lmp.MatchType),
			RegexpConfig: lmp.RegexpConfig,
		},
		Attributes: lmp.RecordAttributes,
		Resources:  lmp.ResourceAttributes,
	}
}
