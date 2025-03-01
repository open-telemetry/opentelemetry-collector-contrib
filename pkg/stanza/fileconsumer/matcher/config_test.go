// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package matcher

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/confmap/xconfmap"
)

func TestFinderConfigValidate(t *testing.T) {
	cases := []struct {
		name        string
		globs       []string
		expectedErr string
	}{
		{
			name:  "Empty",
			globs: []string{},
		},
		{
			name:  "Single",
			globs: []string{"*.log"},
		},
		{
			name:  "Multiple",
			globs: []string{"*.log", "*.txt"},
		},
		{
			name:        "Invalid",
			globs:       []string{"[a-z"},
			expectedErr: "parse glob: syntax error in pattern",
		},
		{
			name:        "ValidAndInvalid",
			globs:       []string{"*.log", "[a-z"},
			expectedErr: "parse glob: syntax error in pattern",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateGlobs(tc.globs)
			if tc.expectedErr != "" {
				assert.EqualError(t, err, tc.expectedErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfigValidate(t *testing.T) {
	cases := []struct {
		name        string
		criteria    Criteria
		expectedErr string
	}{
		{
			name: "IncludeEmpty",
			criteria: Criteria{
				FinderConfig: FinderConfig{
					Include: []string{},
				},
			},
			expectedErr: "'include' must be specified",
		},
		{
			name: "IncludeInvalidGlob",
			criteria: Criteria{
				FinderConfig: FinderConfig{
					Include: []string{"*.log", "[a-z"},
				},
			},
			expectedErr: "include' is invalid: parse glob: syntax error in pattern",
		},
		{
			name: "ExcludeInvalidGlob",
			criteria: Criteria{
				FinderConfig: FinderConfig{
					Include: []string{"*.log"},
					Exclude: []string{"*.log", "[a-z"},
				},
			},
			expectedErr: "'exclude' is invalid: parse glob: syntax error in pattern",
		},
		{
			name: "RegexEmpty",
			criteria: Criteria{
				FinderConfig: FinderConfig{
					Include: []string{"*.log"},
				},
				OrderingCriteria: OrderingCriteria{
					Regex: nil,
					SortBy: []Sort{
						{
							SortType: "numeric",
							RegexKey: "key",
						},
					},
				},
			},
			expectedErr: "'regex' must be specified when 'sort_by' is specified",
		},
		{
			name: "TopN is negative",
			criteria: Criteria{
				FinderConfig: FinderConfig{
					Include: []string{"*.log"},
				},
				OrderingCriteria: OrderingCriteria{
					Regex: regexp.MustCompile("[a-z]"),
					TopN:  -1,
					SortBy: []Sort{
						{
							SortType: "numeric",
							RegexKey: "key",
						},
					},
				},
			},
			expectedErr: "'top_n' must be a positive integer",
		},
		{
			name: "SortTypeEmpty",
			criteria: Criteria{
				FinderConfig: FinderConfig{
					Include: []string{"*.log"},
				},
				OrderingCriteria: OrderingCriteria{
					Regex: regexp.MustCompile(`(?P<num>\d{2}).*log`),
					SortBy: []Sort{
						{
							SortType: "",
						},
					},
				},
			},
			expectedErr: "'sort_type' must be specified",
		},
		{
			name: "SortNumericInvalid",
			criteria: Criteria{
				FinderConfig: FinderConfig{
					Include: []string{"*.log"},
				},
				OrderingCriteria: OrderingCriteria{
					Regex: regexp.MustCompile(`(?P<num>\d{2}).*log`),
					SortBy: []Sort{
						{
							SortType: "numeric",
						},
					},
				},
			},
			expectedErr: "`regex_key` must be specified",
		},
		{
			name: "SortAlphabeticalInvalid",
			criteria: Criteria{
				FinderConfig: FinderConfig{
					Include: []string{"*.log"},
				},
				OrderingCriteria: OrderingCriteria{
					Regex: regexp.MustCompile(`(?P<num>[a-z]+).*log`),
					SortBy: []Sort{
						{
							SortType: "alphabetical",
						},
					},
				},
			},
			expectedErr: "`regex_key` must be specified",
		},
		{
			name: "SortTimestampInvalid",
			criteria: Criteria{
				FinderConfig: FinderConfig{
					Include: []string{"*.log"},
				},
				OrderingCriteria: OrderingCriteria{
					Regex: regexp.MustCompile(`(?P<num>\d{2}).*log`),
					SortBy: []Sort{
						{
							SortType: "timestamp",
							Layout:   "%Y%m%d%H",
						},
					},
				},
			},
			expectedErr: "`regex_key` must be specified",
		},
		{
			name: "SortByMtimeGateDisabled",
			criteria: Criteria{
				FinderConfig: FinderConfig{
					Include: []string{"*.log"},
				},
				OrderingCriteria: OrderingCriteria{
					SortBy: []Sort{
						{
							SortType: "mtime",
						},
					},
				},
			},
			expectedErr: `the "filelog.mtimeSortType" feature gate must be enabled to use "mtime" sort type`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := xconfmap.Validate(tc.criteria)
			if tc.expectedErr != "" {
				assert.ErrorContains(t, err, tc.expectedErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
