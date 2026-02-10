// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filter

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/internal/stanzatime"
)

func TestExcludeOlderThanFilter(t *testing.T) {
	twoHoursAgo := time.Now().Add(-2 * time.Hour)
	threeHoursAgo := twoHoursAgo.Add(-1 * time.Hour)

	cases := map[string]struct {
		files            []string
		fileMTimes       []time.Time
		excludeOlderThan time.Duration

		expect             []string
		expectedErr        string
		expectedWindowsErr string
	}{
		"no_files": {
			files:            []string{},
			fileMTimes:       []time.Time{},
			excludeOlderThan: 2 * time.Hour,

			expect:      []string{},
			expectedErr: "",
		},
		"exclude_no_files": {
			files:            []string{"a.log", "b.log"},
			fileMTimes:       []time.Time{twoHoursAgo, twoHoursAgo},
			excludeOlderThan: 3 * time.Hour,

			expect:      []string{"a.log", "b.log"},
			expectedErr: "",
		},
		"exclude_some_files": {
			files:            []string{"a.log", "b.log"},
			fileMTimes:       []time.Time{twoHoursAgo, threeHoursAgo},
			excludeOlderThan: 3 * time.Hour,

			expect:      []string{"a.log"},
			expectedErr: "",
		},
		"exclude_all_files": {
			files:            []string{"a.log", "b.log"},
			fileMTimes:       []time.Time{twoHoursAgo, threeHoursAgo},
			excludeOlderThan: 90 * time.Minute,

			expect:      []string{},
			expectedErr: "",
		},
		"file_not_present": {
			files:            []string{"a.log", "b.log"},
			fileMTimes:       []time.Time{twoHoursAgo, {}},
			excludeOlderThan: 3 * time.Hour,

			expect:             []string{"a.log"},
			expectedErr:        "b.log: no such file or directory",
			expectedWindowsErr: "b.log: The system cannot find the file specified.",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			stanzatime.Since = stanzatime.NewAlwaysIncreasingClock().Since
			defer func() { stanzatime.Since = time.Since }()

			tmpDir := t.TempDir()
			var items []*item
			// Create files with specified mtime
			for i, file := range tc.files {
				mtime := tc.fileMTimes[i]
				fullPath := filepath.Join(tmpDir, file)

				// Only create file if mtime is specified
				if !mtime.IsZero() {
					f, err := os.Create(fullPath)
					require.NoError(t, err)
					require.NoError(t, f.Close())
					require.NoError(t, os.Chtimes(fullPath, twoHoursAgo, mtime))
				}

				it, err := newItem(fullPath, nil)
				require.NoError(t, err)

				items = append(items, it)
			}

			f := ExcludeOlderThan(tc.excludeOlderThan)
			result, err := f.apply(items)
			if tc.expectedErr != "" {
				if runtime.GOOS == "windows" {
					require.ErrorContains(t, err, tc.expectedWindowsErr)
				} else {
					require.ErrorContains(t, err, tc.expectedErr)
				}
			} else {
				require.NoError(t, err)
			}

			relativeResult := make([]string, 0, len(result))
			for _, r := range result {
				rel, err := filepath.Rel(tmpDir, r.value)
				require.NoError(t, err)
				relativeResult = append(relativeResult, rel)
			}

			require.Equal(t, tc.expect, relativeResult)
		})
	}
}
