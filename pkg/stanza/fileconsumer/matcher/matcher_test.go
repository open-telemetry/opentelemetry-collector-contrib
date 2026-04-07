// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package matcher

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/featuregate"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/metadata"
)

func intPtr(i int) *int { return &i }

func TestNew(t *testing.T) {
	cases := []struct {
		name                   string
		criteria               Criteria
		expectedErr            string
		enableMtimeFeatureGate bool
	}{
		{
			name: "IncludeEmpty",
			criteria: Criteria{
				Include: []string{},
			},
			expectedErr: "'include' must be specified",
		},
		{
			name: "IncludeSingle",
			criteria: Criteria{
				Include: []string{"*.log"},
			},
		},
		{
			name: "IncludeMultiple",
			criteria: Criteria{
				Include: []string{"*.log", "*.txt"},
			},
		},
		{
			name: "IncludeInvalidGlob",
			criteria: Criteria{
				Include: []string{"*.log", "[a-z"},
			},
			expectedErr: "include: parse glob: syntax error in pattern",
		},
		{
			name: "ExcludeSingle",
			criteria: Criteria{
				Include: []string{"*.log"},
				Exclude: []string{"a.log"},
			},
		},
		{
			name: "ExcludeMultiple",
			criteria: Criteria{
				Include: []string{"*.log"},
				Exclude: []string{"a.log", "b.log"},
			},
		},
		{
			name: "ExcludeInvalidGlob",
			criteria: Criteria{
				Include: []string{"*.log"},
				Exclude: []string{"*.log", "[a-z"},
			},
			expectedErr: "exclude: parse glob: syntax error in pattern",
		},
		{
			name: "GroupBy",
			criteria: Criteria{
				Include: []string{"*.log"},
				OrderingCriteria: OrderingCriteria{
					GroupBy: "[a-z]",
				},
			},
		},
		{
			name: "RegexEmpty",
			criteria: Criteria{
				Include: []string{"*.log"},
				OrderingCriteria: OrderingCriteria{
					Regex: "",
					TopN:  intPtr(1),
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
			name: "RegexInvalid",
			criteria: Criteria{
				Include: []string{"*.log"},
				OrderingCriteria: OrderingCriteria{
					Regex: "[a-z",
					TopN:  intPtr(1),
					SortBy: []Sort{
						{
							SortType: "numeric",
							RegexKey: "key",
						},
					},
				},
			},
			expectedErr: "compile regex: error parsing regexp: missing closing ]: `[a-z`",
		},
		{
			name: "TopN is negative",
			criteria: Criteria{
				Include: []string{"*.log"},
				OrderingCriteria: OrderingCriteria{
					Regex: "[a-z]",
					TopN:  intPtr(-1),
					SortBy: []Sort{
						{
							SortType: "numeric",
							RegexKey: "key",
						},
					},
				},
			},
			expectedErr: "'top_n' must not be negative",
		},
		{
			name: "GroupBy error",
			criteria: Criteria{
				Include: []string{"*.log"},
				OrderingCriteria: OrderingCriteria{
					GroupBy: "[a-z",
				},
			},
			expectedErr: "compile group_by regex: error parsing regexp: missing closing ]: `[a-z`",
		},
		{
			name: "SortTypeEmpty",
			criteria: Criteria{
				Include: []string{"*.log"},
				OrderingCriteria: OrderingCriteria{
					Regex: `(?P<num>\d{2}).*log`,
					TopN:  intPtr(1),
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
				Include: []string{"*.log"},
				OrderingCriteria: OrderingCriteria{
					Regex: `(?P<num>\d{2}).*log`,
					TopN:  intPtr(1),
					SortBy: []Sort{
						{
							SortType: "numeric",
							RegexKey: "",
						},
					},
				},
			},
			expectedErr: "numeric sort: regex key must be specified",
		},
		{
			name: "SortAlphabeticalInvalid",
			criteria: Criteria{
				Include: []string{"*.log"},
				OrderingCriteria: OrderingCriteria{
					Regex: `(?P<num>[a-z]+).*log`,
					TopN:  intPtr(1),
					SortBy: []Sort{
						{
							SortType: "alphabetical",
							RegexKey: "",
						},
					},
				},
			},
			expectedErr: "alphabetical sort: regex key must be specified",
		},
		{
			name: "SortTimestampInvalid",
			criteria: Criteria{
				Include: []string{"*.log"},
				OrderingCriteria: OrderingCriteria{
					Regex: `(?P<num>\d{2}).*log`,
					TopN:  intPtr(1),
					SortBy: []Sort{
						{
							SortType: "timestamp",
							RegexKey: "",
							Layout:   "%Y%m%d%H",
						},
					},
				},
			},
			expectedErr: "timestamp sort: regex key must be specified",
		},
		{
			name: "SortByMtime",
			criteria: Criteria{
				Include: []string{"*.log"},
				OrderingCriteria: OrderingCriteria{
					TopN: intPtr(1),
					SortBy: []Sort{
						{
							SortType: "mtime",
						},
					},
				},
			},
			enableMtimeFeatureGate: true,
		},
		{
			name: "SortByMtimeGateDisabled",
			criteria: Criteria{
				Include: []string{"*.log"},
				OrderingCriteria: OrderingCriteria{
					TopN: intPtr(1),
					SortBy: []Sort{
						{
							SortType: "mtime",
						},
					},
				},
			},
			expectedErr: `the "filelog.mtimeSortType" feature gate must be enabled to use "mtime" sort type`,
		},
		{
			name: "ExcludeOlderThan",
			criteria: Criteria{
				Include:          []string{"*.log"},
				ExcludeOlderThan: 24 * time.Hour,
			},
		},
		{
			name: "TopN unset with sort_by is an error",
			criteria: Criteria{
				Include: []string{"*.log"},
				OrderingCriteria: OrderingCriteria{
					SortBy: []Sort{{SortType: "mtime"}},
				},
			},
			enableMtimeFeatureGate: true,
			expectedErr:            "'top_n' must be set explicitly when 'ordering_criteria.sort_by' is configured (use 0 to match all files)",
		},
		{
			name: "TopN zero with sort_by matches all",
			criteria: Criteria{
				Include: []string{"*.log"},
				OrderingCriteria: OrderingCriteria{
					TopN:   intPtr(0),
					SortBy: []Sort{{SortType: "mtime"}},
				},
			},
			enableMtimeFeatureGate: true,
		},
		{
			name: "TopN positive with sort_by is fine",
			criteria: Criteria{
				Include: []string{"*.log"},
				OrderingCriteria: OrderingCriteria{
					TopN:   intPtr(5),
					SortBy: []Sort{{SortType: "mtime"}},
				},
			},
			enableMtimeFeatureGate: true,
		},
		{
			name: "TopN unset without sort_by is fine",
			criteria: Criteria{
				Include: []string{"*.log"},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.enableMtimeFeatureGate {
				enableSortByMTimeFeature(t)
			}

			matcher, err := New(tc.criteria)
			if tc.expectedErr != "" {
				assert.EqualError(t, err, tc.expectedErr)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, matcher)
			}
		})
	}
}

func TestMatcher(t *testing.T) {
	cases := []struct {
		name           string
		files          []string
		include        []string
		exclude        []string
		filterCriteria OrderingCriteria
		expectErr      string
		expected       []string
	}{
		{
			name:      "NoMatch",
			files:     []string{},
			include:   []string{"*.log"},
			expectErr: "no files match the configured criteria",
			expected:  []string{},
		},
		{
			name:     "OneMatch",
			files:    []string{"a.log"},
			include:  []string{"*.log"},
			expected: []string{"a.log"},
		},
		{
			name:      "AllExcluded",
			files:     []string{"2023020611.log", "2023020612.log", "2023020610.log", "2023020609.log"},
			include:   []string{"*.log"},
			exclude:   []string{"*.log"},
			expectErr: "no files match the configured criteria",
			expected:  []string{},
		},
		{
			name:    "AllFiltered",
			files:   []string{"4567.log"},
			include: []string{"*.log"},
			filterCriteria: OrderingCriteria{
				Regex: `(?P<value>\d{4}).*log`, // input will match this
				TopN:  intPtr(1),
				SortBy: []Sort{
					{
						SortType: sortTypeNumeric,
						RegexKey: "wrong", // but will have this regex key
					},
				},
			},
			expectErr: `strconv.Atoi: parsing "": invalid syntax`,
			expected:  []string{},
		},
		{
			name:     "NoFilterOpts",
			files:    []string{"a.log"},
			include:  []string{"*.log"},
			expected: []string{"a.log"},
		},
		{
			name:    "Timestamp Sorting",
			files:   []string{"err.2023020611.log", "err.2023020612.log", "err.2023020610.log", "err.2023020609.log"},
			include: []string{"err.*.log"},
			exclude: []string{},
			filterCriteria: OrderingCriteria{
				Regex: `err\.(?P<value>\d{4}\d{2}\d{2}\d{2}).*log`,
				TopN:  intPtr(1),
				SortBy: []Sort{
					{
						SortType:  sortTypeTimestamp,
						RegexKey:  "value",
						Ascending: false,
						Location:  "UTC",
						Layout:    `%Y%m%d%H`,
					},
				},
			},
			expected: []string{"err.2023020612.log"},
		},
		{
			name:    "TopN > number of files",
			files:   []string{"err.2023020611.log", "err.2023020612.log"},
			include: []string{"err.*.log"},
			exclude: []string{},
			filterCriteria: OrderingCriteria{
				Regex: `err\.(?P<value>\d{4}\d{2}\d{2}\d{2}).*log`,
				TopN:  intPtr(3),
				SortBy: []Sort{
					{
						SortType:  sortTypeTimestamp,
						RegexKey:  "value",
						Ascending: false,
						Location:  "UTC",
						Layout:    `%Y%m%d%H`,
					},
				},
			},
			expected: []string{"err.2023020612.log", "err.2023020611.log"},
		},
		{
			name:    "TopN == number of files",
			files:   []string{"err.2023020611.log", "err.2023020612.log"},
			include: []string{"err.*.log"},
			exclude: []string{},
			filterCriteria: OrderingCriteria{
				Regex: `err\.(?P<value>\d{4}\d{2}\d{2}\d{2}).*log`,
				TopN:  intPtr(2),
				SortBy: []Sort{
					{
						SortType:  sortTypeTimestamp,
						RegexKey:  "value",
						Ascending: false,
						Location:  "UTC",
						Layout:    `%Y%m%d%H`,
					},
				},
			},
			expected: []string{"err.2023020612.log", "err.2023020611.log"},
		},
		{
			name:    "Timestamp Sorting Ascending",
			files:   []string{"err.2023020612.log", "err.2023020611.log", "err.2023020609.log", "err.2023020610.log"},
			include: []string{"err.*.log"},
			exclude: []string{},
			filterCriteria: OrderingCriteria{
				Regex: `err\.(?P<value>\d{4}\d{2}\d{2}\d{2}).*log`,
				TopN:  intPtr(1),
				SortBy: []Sort{
					{
						SortType:  sortTypeTimestamp,
						RegexKey:  "value",
						Ascending: true,
						Location:  "UTC",
						Layout:    `%Y%m%d%H`,
					},
				},
			},
			expected: []string{"err.2023020609.log"},
		},
		{
			name:    "Numeric Sorting",
			files:   []string{"err.123456788.log", "err.123456789.log", "err.123456787.log", "err.123456786.log"},
			include: []string{"err.*.log"},
			exclude: []string{},
			filterCriteria: OrderingCriteria{
				Regex: `err\.(?P<value>\d+).*log`,
				TopN:  intPtr(1),
				SortBy: []Sort{
					{
						SortType:  sortTypeNumeric,
						RegexKey:  "value",
						Ascending: false,
					},
				},
			},
			expected: []string{"err.123456789.log"},
		},
		{
			name:    "Numeric Sorting",
			files:   []string{"err.a.123456788.log", "err.a.123456789.log", "err.a.123456787.log", "err.a.123456786.log", "err.b.123456788.log", "err.b.123456789.log"},
			include: []string{"err.*.log"},
			exclude: []string{},
			filterCriteria: OrderingCriteria{
				TopN:  intPtr(6),
				Regex: `err\.[a-z]\.(?P<value>\d+).*log`,
				SortBy: []Sort{
					{
						SortType:  sortTypeNumeric,
						RegexKey:  "value",
						Ascending: false,
					},
				},
			},
			expected: []string{"err.a.123456789.log", "err.b.123456789.log", "err.a.123456788.log", "err.b.123456788.log", "err.a.123456787.log", "err.a.123456786.log"},
		},
		{
			name:    "Numeric Sorting with grouping",
			files:   []string{"err.a.123456788.log", "err.a.123456789.log", "err.a.123456787.log", "err.a.123456786.log", "err.b.123456788.log", "err.b.123456789.log"},
			include: []string{"err.*.log"},
			exclude: []string{},
			filterCriteria: OrderingCriteria{
				TopN:    intPtr(6),
				GroupBy: `err\.(?P<value>[a-z]+).[0-9]*.*log`,
				Regex:   `err\.[a-z]\.(?P<value>\d+).*log`,
				SortBy: []Sort{
					{
						SortType:  sortTypeNumeric,
						RegexKey:  "value",
						Ascending: false,
					},
				},
			},
			expected: []string{"err.a.123456789.log", "err.a.123456788.log", "err.a.123456787.log", "err.a.123456786.log", "err.b.123456789.log", "err.b.123456788.log"},
		},
		{
			name:    "Grouping",
			files:   []string{"err.a.123456788.log", "err.a.123456789.log", "err.a.123456787.log", "err.b.123456788.log", "err.a.123456786.log", "err.b.123456789.log"},
			include: []string{"err.*.log"},
			exclude: []string{},
			filterCriteria: OrderingCriteria{
				TopN:    intPtr(6),
				GroupBy: `err\.(?P<value>[a-z]+).[0-9]*.*log`,
			},
			expected: []string{"err.a.123456786.log", "err.a.123456787.log", "err.a.123456788.log", "err.a.123456789.log", "err.b.123456788.log", "err.b.123456789.log"},
		},
		{
			name:    "Numeric Sorting Ascending",
			files:   []string{"err.123456789.log", "err.123456788.log", "err.123456786.log", "err.123456787.log"},
			include: []string{"err.*.log"},
			exclude: []string{},
			filterCriteria: OrderingCriteria{
				Regex: `err\.(?P<value>\d+).*log`,
				TopN:  intPtr(1),
				SortBy: []Sort{
					{
						SortType:  sortTypeNumeric,
						RegexKey:  "value",
						Ascending: true,
					},
				},
			},
			expected: []string{"err.123456786.log"},
		},
		{
			name:    "Alphabetical Sorting",
			files:   []string{"err.a.log", "err.d.log", "err.b.log", "err.c.log"},
			include: []string{"err.*.log"},
			exclude: []string{},
			filterCriteria: OrderingCriteria{
				Regex: `err\.(?P<value>[a-zA-Z]+).*log`,
				TopN:  intPtr(1),
				SortBy: []Sort{
					{
						SortType:  sortTypeAlphabetical,
						RegexKey:  "value",
						Ascending: false,
					},
				},
			},
			expected: []string{"err.d.log"},
		},
		{
			name:    "Alphabetical Sorting - Top 2",
			files:   []string{"err.a.log", "err.d.log", "err.b.log", "err.c.log"},
			include: []string{"err.*.log"},
			exclude: []string{},
			filterCriteria: OrderingCriteria{
				Regex: `err\.(?P<value>[a-zA-Z]+).*log`,
				TopN:  intPtr(2),
				SortBy: []Sort{
					{
						SortType:  sortTypeAlphabetical,
						RegexKey:  "value",
						Ascending: false,
					},
				},
			},
			expected: []string{"err.d.log", "err.c.log"},
		},
		{
			name:    "Alphabetical Sorting Ascending",
			files:   []string{"err.b.log", "err.a.log", "err.c.log", "err.d.log"},
			include: []string{"err.*.log"},
			exclude: []string{},
			filterCriteria: OrderingCriteria{
				Regex: `err\.(?P<value>[a-zA-Z]+).*log`,
				TopN:  intPtr(1),
				SortBy: []Sort{
					{
						SortType:  sortTypeAlphabetical,
						RegexKey:  "value",
						Ascending: true,
					},
				},
			},
			expected: []string{"err.a.log"},
		},
		{
			name: "Multiple Sorting - timestamp priority sort - Top 4",
			files: []string{
				"err.b.1.2023020601.log",
				"err.b.2.2023020601.log",
				"err.a.1.2023020601.log",
				"err.a.2.2023020601.log",
				"err.b.1.2023020602.log",
				"err.a.2.2023020602.log",
				"err.b.2.2023020602.log",
				"err.a.1.2023020602.log",
			},
			include: []string{"err.*.log"},
			exclude: []string{},
			filterCriteria: OrderingCriteria{
				Regex: `err\.(?P<alpha>[a-zA-Z])\.(?P<number>\d+)\.(?P<time>\d{10})\.log`,
				TopN:  intPtr(4),
				SortBy: []Sort{
					{
						SortType:  sortTypeAlphabetical,
						RegexKey:  "alpha",
						Ascending: false,
					},
					{
						SortType:  sortTypeNumeric,
						RegexKey:  "number",
						Ascending: false,
					},
					{
						SortType:  sortTypeTimestamp,
						RegexKey:  "time",
						Ascending: false,
						Location:  "UTC",
						Layout:    `%Y%m%d%H`,
					},
				},
			},
			expected: []string{"err.b.2.2023020602.log", "err.a.2.2023020602.log", "err.b.1.2023020602.log", "err.a.1.2023020602.log"},
		},
		{
			name: "Multiple Sorting - timestamp priority sort",
			files: []string{
				"err.b.1.2023020601.log",
				"err.b.2.2023020601.log",
				"err.a.1.2023020601.log",
				"err.a.2.2023020601.log",
				"err.b.1.2023020602.log",
				"err.a.2.2023020602.log",
				"err.b.2.2023020602.log",
				"err.a.1.2023020602.log",
			},
			include: []string{"err.*.log"},
			exclude: []string{},
			filterCriteria: OrderingCriteria{
				Regex: `err\.(?P<alpha>[a-zA-Z])\.(?P<number>\d+)\.(?P<time>\d{10})\.log`,
				TopN:  intPtr(1),
				SortBy: []Sort{
					{
						SortType:  sortTypeAlphabetical,
						RegexKey:  "alpha",
						Ascending: false,
					},
					{
						SortType:  sortTypeNumeric,
						RegexKey:  "number",
						Ascending: false,
					},
					{
						SortType:  sortTypeTimestamp,
						RegexKey:  "time",
						Ascending: false,
						Location:  "UTC",
						Layout:    `%Y%m%d%H`,
					},
				},
			},
			expected: []string{"err.b.2.2023020602.log"},
		},
		{
			name: "Multiple Sorting - timestamp priority sort - numeric ascending",
			files: []string{
				"err.b.1.2023020601.log",
				"err.b.2.2023020601.log",
				"err.a.1.2023020601.log",
				"err.a.2.2023020601.log",
				"err.b.1.2023020602.log",
				"err.a.2.2023020602.log",
				"err.b.2.2023020602.log",
				"err.a.1.2023020602.log",
			},
			include: []string{"err.*.log"},
			exclude: []string{},
			filterCriteria: OrderingCriteria{
				Regex: `err\.(?P<alpha>[a-zA-Z])\.(?P<number>\d+)\.(?P<time>\d{10})\.log`,
				TopN:  intPtr(1),
				SortBy: []Sort{
					{
						SortType:  sortTypeAlphabetical,
						RegexKey:  "alpha",
						Ascending: false,
					},
					{
						SortType:  sortTypeNumeric,
						RegexKey:  "number",
						Ascending: true,
					},
					{
						SortType:  sortTypeTimestamp,
						RegexKey:  "time",
						Ascending: false,
						Location:  "UTC",
						Layout:    `%Y%m%d%H`,
					},
				},
			},
			expected: []string{"err.b.1.2023020602.log"},
		},
		{
			name: "Multiple Sorting - timestamp priority sort",
			files: []string{
				"err.b.1.2023020601.log",
				"err.b.2.2023020601.log",
				"err.a.1.2023020601.log",
				"err.a.2.2023020601.log",
				"err.b.1.2023020602.log",
				"err.a.2.2023020602.log",
				"err.b.2.2023020602.log",
				"err.a.1.2023020602.log",
			},
			include: []string{"err.*.log"},
			exclude: []string{},
			filterCriteria: OrderingCriteria{
				Regex: `err\.(?P<alpha>[a-zA-Z])\.(?P<number>\d+)\.(?P<time>\d{10})\.log`,
				TopN:  intPtr(1),
				SortBy: []Sort{
					{
						SortType:  sortTypeNumeric,
						RegexKey:  "number",
						Ascending: false,
					},
					{
						SortType:  sortTypeTimestamp,
						RegexKey:  "time",
						Ascending: false,
						Location:  "UTC",
						Layout:    `%Y%m%d%H`,
					},
					{
						SortType:  sortTypeAlphabetical,
						RegexKey:  "alpha",
						Ascending: false,
					},
				},
			},
			expected: []string{"err.b.2.2023020602.log"},
		},
		{
			name: "Multiple Sorting - alpha priority sort - alpha ascending",
			files: []string{
				"err.b.1.2023020601.log",
				"err.b.2.2023020601.log",
				"err.a.1.2023020601.log",
				"err.a.2.2023020601.log",
				"err.b.1.2023020602.log",
				"err.a.2.2023020602.log",
				"err.b.2.2023020602.log",
				"err.a.1.2023020602.log",
			},
			include: []string{"err.*.log"},
			exclude: []string{},
			filterCriteria: OrderingCriteria{
				Regex: `err\.(?P<alpha>[a-zA-Z])\.(?P<number>\d+)\.(?P<time>\d{10})\.log`,
				TopN:  intPtr(1),
				SortBy: []Sort{
					{
						SortType:  sortTypeNumeric,
						RegexKey:  "number",
						Ascending: false,
					},
					{
						SortType:  sortTypeTimestamp,
						RegexKey:  "time",
						Ascending: false,
						Location:  "UTC",
						Layout:    `%Y%m%d%H`,
					},
					{
						SortType:  sortTypeAlphabetical,
						RegexKey:  "alpha",
						Ascending: true,
					},
				},
			},
			expected: []string{"err.a.2.2023020602.log"},
		},
		{
			name: "Multiple Sorting - alpha priority sort - timestamp ascending",
			files: []string{
				"err.b.1.2023020601.log",
				"err.b.2.2023020601.log",
				"err.a.1.2023020601.log",
				"err.a.2.2023020601.log",
				"err.b.1.2023020602.log",
				"err.a.2.2023020602.log",
				"err.b.2.2023020602.log",
				"err.a.1.2023020602.log",
			},
			include: []string{"err.*.log"},
			exclude: []string{},
			filterCriteria: OrderingCriteria{
				Regex: `err\.(?P<alpha>[a-zA-Z])\.(?P<number>\d+)\.(?P<time>\d{10})\.log`,
				TopN:  intPtr(1),
				SortBy: []Sort{
					{
						SortType:  sortTypeNumeric,
						RegexKey:  "number",
						Ascending: false,
					},
					{
						SortType:  sortTypeTimestamp,
						RegexKey:  "time",
						Ascending: true,
						Location:  "UTC",
						Layout:    `%Y%m%d%H`,
					},
					{
						SortType:  sortTypeAlphabetical,
						RegexKey:  "alpha",
						Ascending: false,
					},
				},
			},
			expected: []string{"err.b.2.2023020601.log"},
		},
		{
			name: "Multiple Sorting - alpha priority sort - timestamp ascending",
			files: []string{
				"err.b.1.2023020601.log",
				"err.b.2.2023020601.log",
				"err.a.1.2023020601.log",
				"err.a.2.2023020601.log",
				"err.b.1.2023020602.log",
				"err.a.2.2023020602.log",
				"err.b.2.2023020602.log",
				"err.a.1.2023020602.log",
			},
			include: []string{"err.*.log"},
			exclude: []string{},
			filterCriteria: OrderingCriteria{
				Regex: `err\.(?P<alpha>[a-zA-Z])\.(?P<number>\d+)\.(?P<time>\d{10})\.log`,
				TopN:  intPtr(1),
				SortBy: []Sort{
					{
						SortType:  sortTypeNumeric,
						RegexKey:  "number",
						Ascending: true,
					},
					{
						SortType:  sortTypeTimestamp,
						RegexKey:  "time",
						Ascending: false,
						Location:  "UTC",
						Layout:    `%Y%m%d%H`,
					},
					{
						SortType:  sortTypeAlphabetical,
						RegexKey:  "alpha",
						Ascending: false,
					},
				},
			},
			expected: []string{"err.b.1.2023020602.log"},
		},
		{
			name: "Recursive match - include",
			files: []string{
				filepath.Join("a", "1.log"),
				filepath.Join("a", "2.log"),
				filepath.Join("a", "b", "1.log"),
				filepath.Join("a", "b", "2.log"),
				filepath.Join("a", "b", "c", "1.log"),
				filepath.Join("a", "b", "c", "2.log"),
			},
			include: []string{filepath.Join("**", "1.log")},
			exclude: []string{},
			expected: []string{
				filepath.Join("a", "1.log"),
				filepath.Join("a", "b", "1.log"),
				filepath.Join("a", "b", "c", "1.log"),
			},
		},
		{
			name: "Recursive match - include and exclude",
			files: []string{
				filepath.Join("a", "1.log"),
				filepath.Join("a", "2.log"),
				filepath.Join("a", "b", "1.log"),
				filepath.Join("a", "b", "2.log"),
				filepath.Join("a", "b", "c", "1.log"),
				filepath.Join("a", "b", "c", "2.log"),
			},
			include: []string{filepath.Join("**", "1.log")},
			exclude: []string{filepath.Join("**", "b", "**", "1.log")},
			expected: []string{
				filepath.Join("a", "1.log"),
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			for _, f := range tc.files {
				require.NoError(t, os.MkdirAll(filepath.Dir(f), 0o700))
				file, fErr := os.OpenFile(f, os.O_CREATE|os.O_RDWR, 0o600)
				require.NoError(t, fErr)

				_, fErr = file.WriteString(filepath.Base(f))
				require.NoError(t, fErr)
				require.NoError(t, file.Close())
			}
			matcher, err := New(Criteria{
				Include:          tc.include,
				Exclude:          tc.exclude,
				OrderingCriteria: tc.filterCriteria,
			})
			require.NoError(t, err)

			files, err := matcher.MatchFiles()
			if tc.expectErr != "" {
				assert.EqualError(t, err, tc.expectErr)
			} else {
				assert.NoError(t, err)
			}
			assert.ElementsMatch(t, tc.expected, files)
		})
	}
}

// TestMtimeSortWithTopNZeroReturnsAllFiles is regression coverage for #47444:
// when sort_by is mtime and top_n is explicitly set to 0, MatchFiles must
// return all matching files. The previous default silently limited the
// matcher to one file per poll, which caused log duplication with multiple
// actively-written files.
func TestMtimeSortWithTopNZeroReturnsAllFiles(t *testing.T) {
	enableSortByMTimeFeature(t)

	t.Chdir(t.TempDir())

	files := []string{"a.log", "b.log", "c.log", "d.log"}
	for _, f := range files {
		file, err := os.OpenFile(f, os.O_CREATE|os.O_RDWR, 0o600)
		require.NoError(t, err)
		_, err = file.WriteString(f)
		require.NoError(t, err)
		require.NoError(t, file.Close())
		time.Sleep(10 * time.Millisecond) // stagger mtimes
	}

	m, err := New(Criteria{
		Include: []string{"*.log"},
		OrderingCriteria: OrderingCriteria{
			TopN: intPtr(0), // 0 = match all files
			SortBy: []Sort{
				{SortType: sortTypeMtime},
			},
		},
	})
	require.NoError(t, err)

	result, err := m.MatchFiles()
	require.NoError(t, err)
	assert.ElementsMatch(t, files, result, "mtime sort with top_n=0 should return all files")
}

func enableSortByMTimeFeature(t *testing.T) {
	if !metadata.FilelogMtimeSortTypeFeatureGate.IsEnabled() {
		require.NoError(t, featuregate.GlobalRegistry().Set(metadata.FilelogMtimeSortTypeFeatureGate.ID(), true))
		t.Cleanup(func() {
			require.NoError(t, featuregate.GlobalRegistry().Set(metadata.FilelogMtimeSortTypeFeatureGate.ID(), false))
		})
	}
}
