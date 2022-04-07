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
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/winperfcounters/internal/third_party/telegraf/win_perf_counters"
)

// Test_PathBuilder tests that paths are built correctly given a scraperCfg
func Test_PathBuilder(t *testing.T) {
	testCases := []struct {
		name          string
		cfgs          []WatcherCfg
		expectedErr   string
		expectedPaths []string
	}{
		{
			name: "basicPath",
			cfgs: []WatcherCfg{
				{
					ObjectCfg: ObjectConfig{
						Object:   "Memory",
						Counters: []CounterConfig{{Name: "Committed Bytes"}},
					},
				},
			},
			expectedPaths: []string{"\\Memory\\Committed Bytes"},
		},
		{
			name: "multiplePaths",
			cfgs: []WatcherCfg{
				{
					ObjectCfg: ObjectConfig{
						Object:   "Memory",
						Counters: []CounterConfig{{Name: "Committed Bytes"}},
					},
				},
				{
					ObjectCfg: ObjectConfig{
						Object:   "Memory",
						Counters: []CounterConfig{{Name: "Available Bytes"}},
					},
				},
			},
			expectedPaths: []string{"\\Memory\\Committed Bytes", "\\Memory\\Available Bytes"},
		},
		{
			name: "invalidCounter",
			cfgs: []WatcherCfg{
				{
					ObjectCfg: ObjectConfig{
						Object:   "Broken",
						Counters: []CounterConfig{{Name: "Broken Counter"}},
					},
				},
			},
			expectedErr: "counter \\Broken\\Broken Counter: The specified object was not found on the computer.\r\n",
		},
		{
			name: "multipleInvalidCounters",
			cfgs: []WatcherCfg{
				{
					ObjectCfg: ObjectConfig{
						Object:   "Broken",
						Counters: []CounterConfig{{Name: "Broken Counter"}},
					},
				},
				{
					ObjectCfg: ObjectConfig{
						Object:   "Broken part 2",
						Counters: []CounterConfig{{Name: "Broken again"}},
					},
				},
			},
			expectedErr: "counter \\Broken\\Broken Counter: The specified object was not found on the computer.\r\n; counter \\Broken part 2\\Broken again: The specified object was not found on the computer.\r\n",
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			scrapers, err := BuildPaths(test.cfgs)

			if test.expectedErr != "" {
				require.EqualError(t, err, test.expectedErr)
				return
			}

			actualPaths := []string{}
			for _, scraper := range scrapers {
				actualPaths = append(actualPaths, scraper.Path())
			}

			require.Equal(t, test.expectedPaths, actualPaths)
		})
	}
}

type mockPerfCounter struct {
	path        string
	scrapeErr   error
	shutdownErr error
	value       float64
	MetricRep
}

func newMockPerfCounter(path string, scrapeErr, shutdownErr error, value float64, metric MetricRep) *mockPerfCounter {
	return &mockPerfCounter{path: path, scrapeErr: scrapeErr, shutdownErr: shutdownErr, value: value, MetricRep: metric}
}

// Path
func (mpc *mockPerfCounter) Path() string {
	return mpc.path
}

// ScrapeData
func (mpc *mockPerfCounter) ScrapeData() ([]win_perf_counters.CounterValue, error) {
	return []win_perf_counters.CounterValue{{Value: mpc.value}}, mpc.scrapeErr
}

// Close
func (mpc *mockPerfCounter) Close() error {
	return mpc.shutdownErr
}

func (mpc *mockPerfCounter) GetMetricRep() MetricRep {
	return MetricRep{}
}

// Test_Scraping ensures that scrapers scrape appropriately using mocked perfcounters to
// pass valus through
func Test_Scraping(t *testing.T) {
	testCases := []struct {
		name            string
		scrapers        []PerfCounterWatcher
		expectedErr     string
		expectedScraped []CounterValue
	}{
		{
			name: "basicWatcher",
			scrapers: []PerfCounterWatcher{
				newMockPerfCounter("path", nil, nil, 1, MetricRep{
					Name: "metric",
				}),
			},
			expectedScraped: []CounterValue{
				{
					MetricRep: MetricRep{
						Name: "metric",
					},
					Value: 1,
				},
			},
		},
		{
			name: "multipleWatchers",
			scrapers: []PerfCounterWatcher{
				newMockPerfCounter("path", nil, nil, 1, MetricRep{
					Name: "metric",
				}),
				newMockPerfCounter("path2", nil, nil, 2, MetricRep{
					Name: "metric",
				}),
			},
			expectedScraped: []CounterValue{
				{
					MetricRep: MetricRep{
						Name: "metric",
					},
					Value: 1,
				},
				{
					MetricRep: MetricRep{
						Name: "metric2",
					},
					Value: 2,
				},
			},
		},
		{
			name: "brokenWatcher",
			scrapers: []PerfCounterWatcher{
				newMockPerfCounter("path2", fmt.Errorf("failed to scrape"), nil, 2, MetricRep{}),
			},
			expectedErr: "failed to scrape",
		},
		{
			name: "multipleBrokenWatchers",
			scrapers: []PerfCounterWatcher{
				newMockPerfCounter("path2", fmt.Errorf("failed to scrape"), nil, 2, MetricRep{}),
				newMockPerfCounter("path2", fmt.Errorf("failed to scrape again"), nil, 2, MetricRep{}),
			},
			expectedErr: "failed to scrape; failed to scrape again",
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			scrapers, err := WatchCounters(test.scrapers)

			if test.expectedErr != "" {
				require.EqualError(t, err, test.expectedErr)
				return
			}
			require.NoError(t, err)

			require.Equal(t, test.expectedScraped, scrapers)
		})
	}
}

// Test_Closing ensures that scrapers close appropriately
func Test_Closing(t *testing.T) {
	testCases := []struct {
		name        string
		scrapers    []PerfCounterWatcher
		expectedErr string
	}{
		{
			name: "closeWithNoFail",
			scrapers: []PerfCounterWatcher{
				newMockPerfCounter("path", nil, nil, 1, MetricRep{}),
			},
		},
		{
			name: "brokenWatcher",
			scrapers: []PerfCounterWatcher{
				newMockPerfCounter("path2", nil, fmt.Errorf("failed to close"), 2, MetricRep{}),
			},
			expectedErr: "failed to close",
		},
		{
			name: "multipleBrokenWatchers",
			scrapers: []PerfCounterWatcher{
				newMockPerfCounter("path2", nil, fmt.Errorf("failed to close again"), 2, MetricRep{}),
			},
			expectedErr: "failed to close; failed to close again",
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			err := CloseCounters(test.scrapers)

			if test.expectedErr != "" {
				require.EqualError(t, err, test.expectedErr)
				return
			}
			require.NoError(t, err)
		})
	}
}
