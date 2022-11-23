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

package filtermetric

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
)

var (
	regexpFilters = []string{
		"prefix/.*",
		"prefix_.*",
		".*/suffix",
		".*_suffix",
		".*/contains/.*",
		".*_contains_.*",
		"full/name/match",
		"full_name_match",
	}

	strictFilters = []string{
		"exact_string_match",
		".*/suffix",
		"(a|b)",
	}
)

func createMetric(name string) pmetric.Metric {
	metric := pmetric.NewMetric()
	metric.SetName(name)
	return metric
}

func TestMatcherMatches(t *testing.T) {
	tests := []struct {
		name        string
		cfg         *MatchProperties
		metric      pmetric.Metric
		shouldMatch bool
	}{
		{
			name:        "regexpNameMatch",
			cfg:         createConfig(regexpFilters, filterset.Regexp),
			metric:      createMetric("test/match/suffix"),
			shouldMatch: true,
		},
		{
			name:        "regexpNameMisatch",
			cfg:         createConfig(regexpFilters, filterset.Regexp),
			metric:      createMetric("test/match/wrongsuffix"),
			shouldMatch: false,
		},
		{
			name:        "strictNameMatch",
			cfg:         createConfig(strictFilters, filterset.Strict),
			metric:      createMetric("exact_string_match"),
			shouldMatch: true,
		},
		{
			name:        "strictNameMismatch",
			cfg:         createConfig(regexpFilters, filterset.Regexp),
			metric:      createMetric("wrong_string_match"),
			shouldMatch: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			matcher, err := newExpr(test.cfg)
			assert.NotNil(t, matcher)
			assert.NoError(t, err)

			matches, err := matcher.Eval(context.Background(), ottlmetric.NewTransformContext(test.metric, pcommon.NewInstrumentationScope(), pcommon.NewResource()))
			assert.NoError(t, err)
			assert.Equal(t, test.shouldMatch, matches)
		})
	}
}
