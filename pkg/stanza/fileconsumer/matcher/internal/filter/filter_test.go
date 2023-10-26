// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filter

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewItem(t *testing.T) {
	cases := []struct {
		name        string
		regex       string
		value       string
		expectedErr string
		expect      *regexItem
	}{
		{
			name:  "SingleCapture",
			regex: `err\.(?P<value>\d{4}\d{2}\d{2}\d{2}).*log`,
			value: "err.2023020611.log",
			expect: &regexItem{
				value:    "err.2023020611.log",
				captures: map[string]string{"value": "2023020611"},
			},
		},
		{
			name:  "MultipleCapture",
			regex: `foo\.(?P<alpha>[a-zA-Z])\.(?P<number>\d+)\.(?P<time>\d{10})\.log`,
			value: "foo.b.1.2023020601.log",
			expect: &regexItem{
				value: "foo.b.1.2023020601.log",
				captures: map[string]string{
					"alpha":  "b",
					"number": "1",
					"time":   "2023020601",
				},
			},
		},
		{
			name:        "Invalid",
			regex:       `err\.(?P<value>\d{4}\d{2}\d{2}\d{2}).*log`,
			value:       "foo.2023020612.log",
			expectedErr: "'foo.2023020612.log' does not match regex",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			it, err := newRegexItem(tc.value, regexp.MustCompile(tc.regex))
			if tc.expectedErr != "" {
				assert.EqualError(t, err, tc.expectedErr)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tc.expect, it)
		})
	}
}

func TestRegexFilter(t *testing.T) {
	cases := []struct {
		name        string
		regex       string
		values      []string
		expectedErr string
		expect      []string
		numOpts     int
	}{
		{
			name:   "SingleCapture",
			regex:  `err\.(?P<value>\d{4}\d{2}\d{2}\d{2}).*log`,
			values: []string{"err.2023020611.log", "err.2023020612.log"},
			expect: []string{"err.2023020611.log", "err.2023020612.log"},
		},
		{
			name:    "MultipleCapture",
			regex:   `foo\.(?P<alpha>[a-zA-Z])\.(?P<number>\d+)\.(?P<time>\d{10})\.log`,
			values:  []string{"foo.b.1.2023020601.log", "foo.a.2.2023020602.log", "foo.a.2.2023020603.log"},
			expect:  []string{"foo.a.2.2023020603.log"},
			numOpts: 2,
		},
		{
			name:        "OneInvalid",
			regex:       `err\.(?P<value>\d{4}\d{2}\d{2}\d{2}).*log`,
			values:      []string{"err.2023020611.log", "foo.2023020612.log"},
			expectedErr: "'foo.2023020612.log' does not match regex",
			expect:      []string{"err.2023020611.log"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			opts := make([]RegexFilterOption, 0, tc.numOpts)
			for i := 0; i < tc.numOpts; i++ {
				opts = append(opts, &removeFirst{})
			}

			f := NewRegexFilter(regexp.MustCompile(tc.regex), opts...)
			result, err := f.Filter(tc.values)
			if tc.expectedErr != "" {
				assert.EqualError(t, err, tc.expectedErr)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tc.expect, result)
		})
	}
}

type removeFirst struct{}

func (o *removeFirst) apply(items []*regexItem) ([]*regexItem, error) {
	if len(items) == 0 {
		return items, nil
	}
	return items[1:], nil
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
			fullPaths := []string{}
			// Create files with specified mtime
			for i, file := range tc.files {
				mtime := tc.fileMTimes[i]
				fullPath := filepath.Join(tmpDir, file)

				f, err := os.Create(fullPath)
				require.NoError(t, err)
				require.NoError(t, f.Close())
				require.NoError(t, os.Chtimes(fullPath, epoch, mtime))

				fullPaths = append(fullPaths, fullPath)
			}

			f := NewMTimeFilter()
			result, err := f.Filter(fullPaths)
			if tc.expectedErr != "" {
				assert.EqualError(t, err, tc.expectedErr)
			} else {
				assert.NoError(t, err)
			}

			relativeResult := []string{}
			for _, r := range result {
				rel, err := filepath.Rel(tmpDir, r)
				require.NoError(t, err)
				relativeResult = append(relativeResult, rel)
			}

			assert.Equal(t, tc.expect, relativeResult)
		})
	}
}
