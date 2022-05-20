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

//go:build windows
// +build windows

package winperfcounters // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/winperfcounters"

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/multierr"
)

// Test_PathBuilder tests that paths are built correctly given a ObjectConfig
func Test_PathBuilder(t *testing.T) {
	testCases := []struct {
		name          string
		cfgs          []ObjectConfig
		expectedErr   string
		expectedPaths []string
	}{
		{
			name: "basicPath",
			cfgs: []ObjectConfig{
				{
					Object:   "Memory",
					Counters: []CounterConfig{{Name: "Committed Bytes"}},
				},
			},
			expectedPaths: []string{"\\Memory\\Committed Bytes"},
		},
		{
			name: "multiplePaths",
			cfgs: []ObjectConfig{
				{
					Object:   "Memory",
					Counters: []CounterConfig{{Name: "Committed Bytes"}},
				},
				{
					Object:   "Memory",
					Counters: []CounterConfig{{Name: "Available Bytes"}},
				},
			},
			expectedPaths: []string{"\\Memory\\Committed Bytes", "\\Memory\\Available Bytes"},
		},
		{
			name: "multipleIndividualCounters",
			cfgs: []ObjectConfig{
				{
					Object: "Memory",
					Counters: []CounterConfig{
						{Name: "Committed Bytes"},
						{Name: "Available Bytes"},
					},
				},
				{
					Object:   "Memory",
					Counters: []CounterConfig{},
				},
			},
			expectedPaths: []string{"\\Memory\\Committed Bytes", "\\Memory\\Available Bytes"},
		},
		{
			name: "invalidCounter",
			cfgs: []ObjectConfig{
				{
					Object:   "Broken",
					Counters: []CounterConfig{{Name: "Broken Counter"}},
				},
			},

			expectedErr: "counter \\Broken\\Broken Counter: The specified object was not found on the computer.\r\n",
		},
		{
			name: "multipleInvalidCounters",
			cfgs: []ObjectConfig{
				{
					Object:   "Broken",
					Counters: []CounterConfig{{Name: "Broken Counter"}},
				},
				{
					Object:   "Broken part 2",
					Counters: []CounterConfig{{Name: "Broken again"}},
				},
			},
			expectedErr: "counter \\Broken\\Broken Counter: The specified object was not found on the computer.\r\n; counter \\Broken part 2\\Broken again: The specified object was not found on the computer.\r\n",
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			var errs error
			allWatchers := []PerfCounterWatcher{}
			for _, cfg := range test.cfgs {
				watchers, err := cfg.BuildPaths()
				if err != nil {
					errs = multierr.Append(errs, err)
					continue
				}
				allWatchers = append(allWatchers, watchers...)
			}

			if test.expectedErr != "" {
				require.EqualError(t, errs, test.expectedErr)
				return
			}

			actualPaths := []string{}
			for _, watcher := range allWatchers {
				actualPaths = append(actualPaths, watcher.Path())
			}

			require.Equal(t, test.expectedPaths, actualPaths)
		})
	}
}

// Test_Scraping_Wildcard tests that wildcard instances pull out values
func Test_Scraping_Wildcard(t *testing.T) {
	counterVals := []CounterValue{}
	var errs error

	cfg := ObjectConfig{
		Object:    "LogicalDisk",
		Instances: []string{"*"},
		Counters:  []CounterConfig{{Name: "Free Megabytes"}},
	}
	watchers, err := cfg.BuildPaths()
	require.NoError(t, err)

	for _, watcher := range watchers {
		value, err := watcher.ScrapeData()
		if err != nil {
			errs = multierr.Append(errs, err)
			continue
		}

		require.NoError(t, err)
		counterVals = append(counterVals, value...)
	}

	require.GreaterOrEqual(t, len(counterVals), 3)
}
