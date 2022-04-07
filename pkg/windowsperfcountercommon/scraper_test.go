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

package windowsperfcountercommon // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/windowsperfcountercommon"

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/windowsperfcountercommon/internal/third_party/telegraf/win_perf_counters"
)

// Test_PathBuilder tests that paths are built correctly given a scraperCfg
func Test_PathBuilder(t *testing.T) {
	testCases := []struct {
		name          string
		cfgs          []ScraperCfg
		expectedErr   string
		expectedPaths []string
	}{
		{
			name: "basicPath",
			cfgs: []ScraperCfg{
				{
					CounterCfg: PerfCounterConfig{
						Object:   "Memory",
						Counters: []CounterConfig{{Name: "Committed Bytes"}},
					},
				},
			},
			expectedPaths: []string{"\\Memory\\Committed Bytes"},
		},
		{
			name: "multiplePaths",
			cfgs: []ScraperCfg{
				{
					CounterCfg: PerfCounterConfig{
						Object:   "Memory",
						Counters: []CounterConfig{{Name: "Committed Bytes"}},
					},
				},
				{
					CounterCfg: PerfCounterConfig{
						Object:   "Memory",
						Counters: []CounterConfig{{Name: "Available Bytes"}},
					},
				},
			},
			expectedPaths: []string{"\\Memory\\Committed Bytes", "\\Memory\\Available Bytes"},
		},
		{
			name: "invalidCounter",
			cfgs: []ScraperCfg{
				{
					CounterCfg: PerfCounterConfig{
						Object:   "Broken",
						Counters: []CounterConfig{{Name: "Broken Counter"}},
					},
				},
			},
			expectedErr: "counter \\Broken\\Broken Counter: The specified object was not found on the computer.\r\n",
		},
		{
			name: "multipleInvalidCounters",
			cfgs: []ScraperCfg{
				{
					CounterCfg: PerfCounterConfig{
						Object:   "Broken",
						Counters: []CounterConfig{{Name: "Broken Counter"}},
					},
				},
				{
					CounterCfg: PerfCounterConfig{
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
			core, obs := observer.New(zapcore.WarnLevel)
			logger := zap.New(core)

			scrapers := BuildPaths(test.cfgs, logger)

			if test.expectedErr != "" {
				require.Equal(t, 1, obs.Len())
				log := obs.All()[0]
				require.EqualError(t, log.Context[0].Interface.(error), test.expectedErr)
				return
			}

			actualPaths := []string{}
			for _, scraper := range scrapers {
				actualPaths = append(actualPaths, scraper.Counter.Path())
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
}

func newMockPerfCounter(path string, scrapeErr, shutdownErr error, value float64) *mockPerfCounter {
	return &mockPerfCounter{path: path, scrapeErr: scrapeErr, shutdownErr: shutdownErr, value: value}
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

// Test_Scraping ensures that scrapers scrape appropriately using mocked perfcounters to
// pass valus through
func Test_Scraping(t *testing.T) {
	testCases := []struct {
		name            string
		scrapers        []Scraper
		expectedErr     string
		expectedScraped []ScrapedValues
	}{
		{
			name: "basicScraper",
			scrapers: []Scraper{
				{
					Counter: newMockPerfCounter("path", nil, nil, 1),
					Metric: MetricRep{
						Name: "metric",
					},
				},
			},
			expectedScraped: []ScrapedValues{
				{
					Metric: MetricRep{
						Name: "metric",
					},
					Value: 1,
				},
			},
		},
		{
			name: "multipleScrapers",
			scrapers: []Scraper{
				{
					Counter: newMockPerfCounter("path", nil, nil, 1),
					Metric: MetricRep{
						Name: "metric",
					},
				},
				{
					Counter: newMockPerfCounter("path2", nil, nil, 2),
					Metric: MetricRep{
						Name: "metric2",
					},
				},
			},
			expectedScraped: []ScrapedValues{
				{
					Metric: MetricRep{
						Name: "metric",
					},
					Value: 1,
				},
				{
					Metric: MetricRep{
						Name: "metric2",
					},
					Value: 2,
				},
			},
		},
		{
			name: "brokenScraper",
			scrapers: []Scraper{
				{
					Counter: newMockPerfCounter("path2", fmt.Errorf("failed to scrape"), nil, 2),
					Metric: MetricRep{
						Name: "metric2",
					},
				},
			},
			expectedErr: "failed to scrape",
		},
		{
			name: "multipleBrokenScrapers",
			scrapers: []Scraper{
				{
					Counter: newMockPerfCounter("path2", fmt.Errorf("failed to scrape"), nil, 2),
					Metric: MetricRep{
						Name: "metric2",
					},
				},
				{
					Counter: newMockPerfCounter("path2", fmt.Errorf("failed to scrape again"), nil, 2),
					Metric: MetricRep{
						Name: "metric2",
					},
				},
			},
			expectedErr: "failed to scrape; failed to scrape again",
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			scrapers, err := ScrapeCounters(test.scrapers)

			if test.expectedErr != "" {
				require.EqualError(t, err, test.expectedErr)
				return
			}

			require.Equal(t, test.expectedScraped, scrapers)
		})
	}
}

// Test_Closing ensures that scrapers close appropriately
func Test_Closing(t *testing.T) {
	testCases := []struct {
		name        string
		scrapers    []Scraper
		expectedErr string
	}{
		{
			name: "closeWithNoFail",
			scrapers: []Scraper{
				{
					Counter: newMockPerfCounter("path", nil, nil, 1),
					Metric: MetricRep{
						Name: "metric",
					},
				},
			},
		},
		{
			name: "brokenScraper",
			scrapers: []Scraper{
				{
					Counter: newMockPerfCounter("path2", nil, fmt.Errorf("failed to close"), 2),
					Metric: MetricRep{
						Name: "metric2",
					},
				},
			},
			expectedErr: "failed to close",
		},
		{
			name: "multipleBrokenScrapers",
			scrapers: []Scraper{
				{
					Counter: newMockPerfCounter("path2", nil, fmt.Errorf("failed to close"), 2),
					Metric: MetricRep{
						Name: "metric2",
					},
				},
				{
					Counter: newMockPerfCounter("path2", nil, fmt.Errorf("failed to close again"), 2),
					Metric: MetricRep{
						Name: "metric2",
					},
				},
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
