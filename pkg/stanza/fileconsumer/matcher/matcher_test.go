// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package matcher

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/featuregate"
)

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
			name: "RegexEmpty",
			criteria: Criteria{
				Include: []string{"*.log"},
				OrderingCriteria: OrderingCriteria{
					Regex: "",
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
				Include: []string{"*.log"},
				OrderingCriteria: OrderingCriteria{
					Regex: `(?P<num>\d{2}).*log`,
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
	t.Parallel()
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
				TopN:  3,
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
				TopN:  2,
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
			name:    "Numeric Sorting Ascending",
			files:   []string{"err.123456789.log", "err.123456788.log", "err.123456786.log", "err.123456787.log"},
			include: []string{"err.*.log"},
			exclude: []string{},
			filterCriteria: OrderingCriteria{
				Regex: `err\.(?P<value>\d+).*log`,
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
				TopN:  2,
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
				TopN:  4,
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
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cwd, err := os.Getwd()
			require.NoError(t, err)
			require.NoError(t, os.Chdir(t.TempDir()))
			defer func() {
				require.NoError(t, os.Chdir(cwd))
			}()
			for _, f := range tc.files {
				require.NoError(t, os.MkdirAll(filepath.Dir(f), 0700))
				file, fErr := os.OpenFile(f, os.O_CREATE|os.O_RDWR, 0600)
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
			assert.Equal(t, tc.expected, files)
		})
	}
}

func enableSortByMTimeFeature(t *testing.T) {
	if !mtimeSortTypeFeatureGate.IsEnabled() {
		require.NoError(t, featuregate.GlobalRegistry().Set(mtimeSortTypeFeatureGate.ID(), true))
		t.Cleanup(func() {
			require.NoError(t, featuregate.GlobalRegistry().Set(mtimeSortTypeFeatureGate.ID(), false))
		})
	}
}
