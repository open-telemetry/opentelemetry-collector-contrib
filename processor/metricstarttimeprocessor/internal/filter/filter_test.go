// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterset"
)

func TestNewFilter(t *testing.T) {
	tests := []struct {
		name        string
		include     FilterConfig
		exclude     FilterConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "empty_filter",
			include: FilterConfig{
				Metrics: []string{},
			},
			exclude: FilterConfig{
				Metrics: []string{},
			},
			expectError: false,
		},
		{
			name: "include_only_with_strict_matching",
			include: FilterConfig{
				Config:  filterset.Config{MatchType: filterset.Strict},
				Metrics: []string{"metric1", "metric2"},
			},
			exclude: FilterConfig{
				Metrics: []string{},
			},
			expectError: false,
		},
		{
			name: "exclude_only_with_regexp_matching",
			include: FilterConfig{
				Metrics: []string{},
			},
			exclude: FilterConfig{
				Config:  filterset.Config{MatchType: filterset.Regexp},
				Metrics: []string{"metric.*", "test_.*"},
			},
			expectError: false,
		},
		{
			name: "both_include_and_exclude",
			include: FilterConfig{
				Config:  filterset.Config{MatchType: filterset.Strict},
				Metrics: []string{"metric1", "metric2", "metric3"},
			},
			exclude: FilterConfig{
				Config:  filterset.Config{MatchType: filterset.Strict},
				Metrics: []string{"metric3"},
			},
			expectError: false,
		},
		{
			name: "invalid_regexp_in_include",
			include: FilterConfig{
				Config:  filterset.Config{MatchType: filterset.Regexp},
				Metrics: []string{"[invalid"},
			},
			exclude: FilterConfig{
				Metrics: []string{},
			},
			expectError: true,
			errorMsg:    "error parsing regexp",
		},
		{
			name: "invalid_regexp_in_exclude",
			include: FilterConfig{
				Metrics: []string{},
			},
			exclude: FilterConfig{
				Config:  filterset.Config{MatchType: filterset.Regexp},
				Metrics: []string{"[invalid"},
			},
			expectError: true,
			errorMsg:    "error parsing regexp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter, err := NewFilter(tt.include, tt.exclude)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
				assert.Nil(t, filter)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, filter)
			}
		})
	}
}

func TestFilter_Matches(t *testing.T) {
	tests := []struct {
		name        string
		include     FilterConfig
		exclude     FilterConfig
		metricName  string
		shouldMatch bool
	}{
		{
			name: "no_filters_matches_all",
			include: FilterConfig{
				Metrics: []string{},
			},
			exclude: FilterConfig{
				Metrics: []string{},
			},
			metricName:  "any_metric",
			shouldMatch: true,
		},
		{
			name: "include_strict_match",
			include: FilterConfig{
				Config:  filterset.Config{MatchType: filterset.Strict},
				Metrics: []string{"metric1", "metric2"},
			},
			exclude: FilterConfig{
				Metrics: []string{},
			},
			metricName:  "metric1",
			shouldMatch: true,
		},
		{
			name: "include_strict_no_match",
			include: FilterConfig{
				Config:  filterset.Config{MatchType: filterset.Strict},
				Metrics: []string{"metric1", "metric2"},
			},
			exclude: FilterConfig{
				Metrics: []string{},
			},
			metricName:  "metric3",
			shouldMatch: false,
		},
		{
			name: "exclude_strict_match",
			include: FilterConfig{
				Metrics: []string{},
			},
			exclude: FilterConfig{
				Config:  filterset.Config{MatchType: filterset.Strict},
				Metrics: []string{"bad_metric"},
			},
			metricName:  "bad_metric",
			shouldMatch: false,
		},
		{
			name: "exclude_strict_no_match",
			include: FilterConfig{
				Metrics: []string{},
			},
			exclude: FilterConfig{
				Config:  filterset.Config{MatchType: filterset.Strict},
				Metrics: []string{"bad_metric"},
			},
			metricName:  "good_metric",
			shouldMatch: true,
		},
		{
			name: "include_regexp_match",
			include: FilterConfig{
				Config:  filterset.Config{MatchType: filterset.Regexp},
				Metrics: []string{"metric.*", "test_.*"},
			},
			exclude: FilterConfig{
				Metrics: []string{},
			},
			metricName:  "metric_cpu",
			shouldMatch: true,
		},
		{
			name: "include_regexp_no_match",
			include: FilterConfig{
				Config:  filterset.Config{MatchType: filterset.Regexp},
				Metrics: []string{"metric.*", "test_.*"},
			},
			exclude: FilterConfig{
				Metrics: []string{},
			},
			metricName:  "system_cpu",
			shouldMatch: false,
		},
		{
			name: "exclude_regexp_match",
			include: FilterConfig{
				Metrics: []string{},
			},
			exclude: FilterConfig{
				Config:  filterset.Config{MatchType: filterset.Regexp},
				Metrics: []string{"temp_.*", "debug_.*"},
			},
			metricName:  "temp_metric",
			shouldMatch: false,
		},
		{
			name: "exclude_takes_precedence",
			include: FilterConfig{
				Config:  filterset.Config{MatchType: filterset.Strict},
				Metrics: []string{"metric1", "metric2", "metric3"},
			},
			exclude: FilterConfig{
				Config:  filterset.Config{MatchType: filterset.Strict},
				Metrics: []string{"metric2"},
			},
			metricName:  "metric2",
			shouldMatch: false,
		},
		{
			name: "include_and_exclude_both_active",
			include: FilterConfig{
				Config:  filterset.Config{MatchType: filterset.Regexp},
				Metrics: []string{"prod_.*"},
			},
			exclude: FilterConfig{
				Config:  filterset.Config{MatchType: filterset.Regexp},
				Metrics: []string{".*_temp"},
			},
			metricName:  "prod_cpu",
			shouldMatch: true,
		},
		{
			name: "include_and_exclude_conflict",
			include: FilterConfig{
				Config:  filterset.Config{MatchType: filterset.Regexp},
				Metrics: []string{"prod_.*"},
			},
			exclude: FilterConfig{
				Config:  filterset.Config{MatchType: filterset.Regexp},
				Metrics: []string{".*_temp"},
			},
			metricName:  "prod_temp",
			shouldMatch: false, // exclude takes precedence
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter, err := NewFilter(tt.include, tt.exclude)
			require.NoError(t, err)

			matches := filter.Matches(tt.metricName)
			assert.Equal(t, tt.shouldMatch, matches,
				"Expected Matches(%q) to return %v", tt.metricName, tt.shouldMatch)
		})
	}
}
