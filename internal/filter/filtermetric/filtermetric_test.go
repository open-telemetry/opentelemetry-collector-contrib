// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filtermetric

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterottl"
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
		cfg         *filterconfig.MetricMatchProperties
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

			matches, err := matcher.Eval(context.Background(), ottlmetric.NewTransformContext(test.metric, pmetric.NewMetricSlice(), pcommon.NewInstrumentationScope(), pcommon.NewResource(), pmetric.NewScopeMetrics(), pmetric.NewResourceMetrics()))
			assert.NoError(t, err)
			assert.Equal(t, test.shouldMatch, matches)
		})
	}
}

func Test_NewSkipExpr_With_Bridge(t *testing.T) {
	tests := []struct {
		name    string
		include *filterconfig.MetricMatchProperties
		exclude *filterconfig.MetricMatchProperties
		err     error
	}{
		// Metric Name
		{
			name: "single static metric name include",
			include: &filterconfig.MetricMatchProperties{
				MatchType:   filterconfig.MetricStrict,
				MetricNames: []string{"metricA"},
			},
		},
		{
			name: "multiple static service name include",
			include: &filterconfig.MetricMatchProperties{
				MatchType:   filterconfig.MetricStrict,
				MetricNames: []string{"metricB", "metricC"},
			},
		},
		{
			name: "single regex service name include",
			include: &filterconfig.MetricMatchProperties{
				MatchType:   filterconfig.MetricRegexp,
				MetricNames: []string{".*A"},
			},
		},
		{
			name: "multiple regex service name include",
			include: &filterconfig.MetricMatchProperties{
				MatchType:   filterconfig.MetricRegexp,
				MetricNames: []string{".*B", ".*C"},
			},
		},
		{
			name: "single static metric name exclude",
			exclude: &filterconfig.MetricMatchProperties{
				MatchType:   filterconfig.MetricStrict,
				MetricNames: []string{"metricA"},
			},
		},
		{
			name: "multiple static service name exclude",
			exclude: &filterconfig.MetricMatchProperties{
				MatchType:   filterconfig.MetricStrict,
				MetricNames: []string{"metricB", "metricC"},
			},
		},
		{
			name: "single regex service name exclude",
			exclude: &filterconfig.MetricMatchProperties{
				MatchType:   filterconfig.MetricRegexp,
				MetricNames: []string{".*A"},
			},
		},
		{
			name: "multiple regex service name exclude",
			exclude: &filterconfig.MetricMatchProperties{
				MatchType:   filterconfig.MetricRegexp,
				MetricNames: []string{".*B", ".*C"},
			},
		},

		// Expression
		{
			name: "expression errors",
			include: &filterconfig.MetricMatchProperties{
				MatchType:   filterconfig.MetricExpr,
				Expressions: []string{"MetricName == metricA"},
			},
			err: errors.New("expressions configuration cannot be converted to OTTL - see https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/processor/filterprocessor#configuration for OTTL configuration"),
		},

		// Complex
		{
			name: "complex",
			include: &filterconfig.MetricMatchProperties{
				MatchType:   filterconfig.MetricStrict,
				MetricNames: []string{"metricA"},
			},
			exclude: &filterconfig.MetricMatchProperties{
				MatchType:   filterconfig.MetricRegexp,
				MetricNames: []string{".*B", ".*C"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metric := pmetric.NewMetric()
			metric.SetName("metricA")

			resource := pcommon.NewResource()

			scope := pcommon.NewInstrumentationScope()

			tCtx := ottlmetric.NewTransformContext(metric, pmetric.NewMetricSlice(), scope, resource, pmetric.NewScopeMetrics(), pmetric.NewResourceMetrics())

			boolExpr, err := NewSkipExpr(tt.include, tt.exclude)
			require.NoError(t, err)
			expectedResult, err := boolExpr.Eval(context.Background(), tCtx)
			assert.NoError(t, err)

			ottlBoolExpr, err := filterottl.NewMetricSkipExprBridge(tt.include, tt.exclude)

			if tt.err != nil {
				assert.Equal(t, tt.err, err)
			} else {
				assert.NoError(t, err)
				ottlResult, err := ottlBoolExpr.Eval(context.Background(), tCtx)
				assert.NoError(t, err)

				assert.Equal(t, expectedResult, ottlResult)
			}
		})
	}
}
