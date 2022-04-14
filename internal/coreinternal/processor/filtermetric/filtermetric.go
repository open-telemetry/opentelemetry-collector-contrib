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
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

type Matcher interface {
	MatchMetric(metric pmetric.Metric) (bool, error)
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

// Filters have the ability to include and exclude metrics based on the metric's properties.
// The default is to not skip. If include is defined, the metric must match or it will be skipped.
// If include is not defined but exclude is, metric will be skipped if it matches exclude. Metric
// is included if neither specified.
func SkipMetric(include, exclude Matcher, metric pmetric.Metric, logger *zap.Logger) bool {
	if include != nil {
		// A false (or an error) returned in this case means the metric should not be processed.
		i, err := include.MatchMetric(metric)
		if !i || err != nil {
			logger.Debug("Skipping metric",
				zap.String("metric_name", (metric.Name())),
				zap.Error(err)) // zap.Error handles case where err is nil
			return true
		}
	}

	if exclude != nil {
		// A true (or an error) returned in this case means the metric should not be processed.
		e, err := exclude.MatchMetric(metric)
		if e || err != nil {
			logger.Debug("Skipping metric",
				zap.String("metric_name", (metric.Name())),
				zap.Error(err)) // zap.Error handles case where err is nil
			return true
		}
	}

	return false
}
