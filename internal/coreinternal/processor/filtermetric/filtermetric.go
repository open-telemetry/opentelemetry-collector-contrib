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

package filtermetric // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/processor/filtermetric"

import (
	"go.opentelemetry.io/collector/model/pdata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/processor/filterconfig"
)

type Matcher interface {
	MatchMetric(metric pdata.Metric) (bool, error)
}

func CreateMatchPropertiesFromDefault(properties *filterconfig.MatchProperties) *MatchProperties {
	if properties == nil {
		return nil
	}

	return &MatchProperties{
		MatchType:          MatchType(properties.Config.MatchType),
		RegexpConfig:       properties.Config.RegexpConfig,
		MetricNames:        properties.MetricNames,
		Expressions:        properties.Expressions,
		ResourceAttributes: properties.ResourceAttributes,
	}
}

// NewMatcher constructs a metric Matcher. If an 'expr' match type is specified,
// returns an expr matcher, otherwise a name matcher.
func NewMatcher(config *MatchProperties) (Matcher, error) {
	if config == nil {
		return nil, nil
	}

	if config.MatchType == Expr {
		return newExprMatcher(config.Expressions)
	}
	return newNameMatcher(config)
}

func SkipMetric(include, exclude Matcher, metric pdata.Metric) bool {
	if include != nil {
		// A false (or an error) returned in this case means the metric should not be processed.
		i, err := include.MatchMetric(metric)
		if !i || err != nil {
			return true
		}
	}

	if exclude != nil {
		// A true (or an error) returned in this case means the span should not be processed.
		e, err := exclude.MatchMetric(metric)
		if e || err != nil {
			return true
		}
	}

	return false
}
