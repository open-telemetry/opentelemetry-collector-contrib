// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filter

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSortOptionErr(t *testing.T) {
	var err error
	_, err = SortNumeric("", true)
	require.Error(t, err)
	_, err = SortNumeric("", false)
	require.Error(t, err)

	_, err = SortAlphabetical("", true)
	require.Error(t, err)
	_, err = SortAlphabetical("", false)
	require.Error(t, err)

	_, err = SortTemporal("", true, "%%Y%m%d%H", "")
	require.Error(t, err)
	_, err = SortTemporal("", false, "%%Y%m%d%H", "")
	require.Error(t, err)

	_, err = SortTemporal("foo", true, "", "")
	require.Error(t, err)
	_, err = SortTemporal("foo", false, "", "")
	require.Error(t, err)

	_, err = SortTemporal("foo", true, "%%Y%m%d%H", "nowhere")
	require.Error(t, err)
	_, err = SortTemporal("foo", false, "%%Y%m%d%H", "nowhere")
	require.Error(t, err)
}

type withOpts func(t *testing.T, ascending bool) []Option

func TestSort(t *testing.T) {
	cases := []struct {
		name     string
		withOpts withOpts

		regex  string
		values []string

		expectApplyErr  string
		expectAscending []string
	}{
		{
			name: "Numeric",
			withOpts: func(t *testing.T, ascending bool) []Option {
				o, err := SortNumeric("num", ascending)
				require.NoError(t, err)
				return []Option{o}
			},
			regex:           `(?P<num>\d{2}).*log`,
			values:          []string{"55.log", "22.log", "66.log", "44.log"},
			expectAscending: []string{"22.log", "44.log", "55.log", "66.log"},
		},
		{
			name: "NumericParseErr",
			withOpts: func(t *testing.T, ascending bool) []Option {
				o, err := SortNumeric("num", ascending)
				require.NoError(t, err)
				return []Option{o}
			},
			regex:           `(?P<num>[a-z0-9]{2}).*log`,
			values:          []string{"bb.log", "66.log", "aa.log", "zz.log", "44.log"},
			expectApplyErr:  "strconv.Atoi: parsing \"zz\": invalid syntax; strconv.Atoi: parsing \"aa\": invalid syntax; strconv.Atoi: parsing \"bb\": invalid syntax",
			expectAscending: []string{"44.log", "66.log"},
		},
		{
			name: "Alphabetical",
			withOpts: func(t *testing.T, ascending bool) []Option {
				o, err := SortAlphabetical("word", ascending)
				require.NoError(t, err)
				return []Option{o}
			},
			regex:           `(?P<word>[a-z0-9]+).*log`,
			values:          []string{"5b2c.log", "a.log", "foo.log", "99.log"},
			expectAscending: []string{"5b2c.log", "99.log", "a.log", "foo.log"},
		},
		{
			name: "Temporal",
			withOpts: func(t *testing.T, ascending bool) []Option {
				o, err := SortTemporal("time", ascending, "%Y%m%d%H", "")
				require.NoError(t, err)
				return []Option{o}
			},
			regex:           `(?P<time>\d{4}\d{2}\d{2}\d{2}).*log`,
			values:          []string{"2023020611.log", "2023020612.log", "2023020610.log", "2023020609.log"},
			expectAscending: []string{"2023020609.log", "2023020610.log", "2023020611.log", "2023020612.log"},
		},
		{
			name: "TemporalParseErr",
			withOpts: func(t *testing.T, ascending bool) []Option {
				o, err := SortTemporal("time", ascending, "%Y%m%d%H", "")
				require.NoError(t, err)
				return []Option{o}
			},
			regex:           `(?P<time>[0-9a-z]+).*log`,
			values:          []string{"2023020xyz611.log", "2023020612.log", "2023asdf020610.log", "2023020609.log"},
			expectApplyErr:  "parsing time \"2023asdf020610\" as \"2006010215\": cannot parse \"asdf020610\" as \"01\"; parsing time \"2023020xyz611\" as \"2006010215\": cannot parse \"0xyz611\" as \"02\"",
			expectAscending: []string{"2023020609.log", "2023020612.log"},
		},
		{
			name: "SoloItem",
			withOpts: func(t *testing.T, ascending bool) []Option {
				o, err := SortNumeric("num", ascending)
				require.NoError(t, err)
				return []Option{o}
			},
			regex:           `(?P<num>[a-z0-9]{2}).*log`,
			values:          []string{"22.log"},
			expectAscending: []string{"22.log"},
		},
		{
			name: "SoloErr",
			withOpts: func(t *testing.T, ascending bool) []Option {
				o, err := SortNumeric("num", ascending)
				require.NoError(t, err)
				return []Option{o}
			},
			regex:           `(?P<num>[a-z0-9]{2}).*log`,
			values:          []string{"aa.log"},
			expectApplyErr:  `strconv.Atoi: parsing "aa": invalid syntax`,
			expectAscending: []string{},
		},
		{
			name: "AllErr",
			withOpts: func(t *testing.T, ascending bool) []Option {
				o, err := SortNumeric("num", ascending)
				require.NoError(t, err)
				return []Option{o}
			},
			regex:           `(?P<num>[a-z0-9]{2}).*log`,
			values:          []string{"bb.log", "aa.log", "zz.log"},
			expectApplyErr:  "strconv.Atoi: parsing \"zz\": invalid syntax; strconv.Atoi: parsing \"aa\": invalid syntax; strconv.Atoi: parsing \"bb\": invalid syntax",
			expectAscending: []string{},
		},
	}
	for _, tc := range cases {
		for _, ascending := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s/%t", tc.name, ascending), func(t *testing.T) {
				result, err := Filter(tc.values, regexp.MustCompile(tc.regex), tc.withOpts(t, ascending)...)
				if tc.expectApplyErr != "" {
					assert.EqualError(t, err, tc.expectApplyErr)
				} else {
					assert.NoError(t, err)
				}

				if ascending {
					assert.Equal(t, tc.expectAscending, result)
				} else {
					descending := make([]string, 0, len(tc.expectAscending))
					for i := len(tc.expectAscending) - 1; i >= 0; i-- {
						descending = append(descending, tc.expectAscending[i])
					}
					assert.Equal(t, descending, result)
				}
			})
		}
	}
}

func TestMTimeFilter(t *testing.T) {
	epoch := time.Unix(0, 0)
	cases := []struct {
		name        string
		files       []string
		fileMTimes  []time.Time
		expectedErr string
		expect      []string
	}{
		{
			name:       "No files",
			files:      []string{},
			fileMTimes: []time.Time{},
			expect:     []string{},
		},
		{
			name:       "Single file",
			files:      []string{"a.log"},
			fileMTimes: []time.Time{epoch},
			expect:     []string{"a.log"},
		},
		{
			name:       "Multiple files",
			files:      []string{"a.log", "b.log"},
			fileMTimes: []time.Time{epoch, epoch.Add(time.Hour)},
			expect:     []string{"b.log", "a.log"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			items := []*item{}
			// Create files with specified mtime
			for i, file := range tc.files {
				mtime := tc.fileMTimes[i]
				fullPath := filepath.Join(tmpDir, file)

				f, err := os.Create(fullPath)
				require.NoError(t, err)
				require.NoError(t, f.Close())
				require.NoError(t, os.Chtimes(fullPath, epoch, mtime))

				it, err := newItem(fullPath, nil)
				require.NoError(t, err)

				items = append(items, it)
			}

			f := SortMtime()
			result, err := f.apply(items)
			if tc.expectedErr != "" {
				require.EqualError(t, err, tc.expectedErr)
			} else {
				require.NoError(t, err)
			}

			relativeResult := []string{}
			for _, r := range result {
				rel, err := filepath.Rel(tmpDir, r.value)
				require.NoError(t, err)
				relativeResult = append(relativeResult, rel)
			}

			require.Equal(t, tc.expect, relativeResult)
		})
	}
}
