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

// +build windows

package windowsperfcountersreceiver

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/receiver/receiverhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/windowsperfcountersreceiver/internal/third_party/telegraf/win_perf_counters"
)

type mockPerfCounter struct {
	scrapeErr error
	closeErr  error
}

func newMockPerfCounter(scrapeErr, closeErr error) *mockPerfCounter {
	return &mockPerfCounter{scrapeErr: scrapeErr, closeErr: closeErr}
}

// Path
func (mpc *mockPerfCounter) Path() string {
	return ""
}

// ScrapeData
func (mpc *mockPerfCounter) ScrapeData() ([]win_perf_counters.CounterValue, error) {
	return []win_perf_counters.CounterValue{{Value: 0}}, mpc.scrapeErr
}

// Close
func (mpc *mockPerfCounter) Close() error {
	return mpc.closeErr
}

func Test_WindowsPerfCounterScraper(t *testing.T) {
	type testCase struct {
		name string
		cfg  *Config

		newErr        string
		initializeErr string
		scrapeErr     error
		closeErr      error

		expectedMetrics int
	}

	defaultConfig := &Config{
		PerfCounters: []PerfCounterConfig{
			{Object: "Memory", Counters: []string{"Committed Bytes"}},
		},
		ScraperControllerSettings: receiverhelper.ScraperControllerSettings{CollectionInterval: time.Minute},
	}

	testCases := []testCase{
		{
			name:            "Standard",
			expectedMetrics: 1,
		},
		{
			name: "InvalidCounter",
			cfg: &Config{
				PerfCounters: []PerfCounterConfig{
					{
						Object:   "Memory",
						Counters: []string{"Committed Bytes"},
					},
					{
						Object:   "Invalid Object",
						Counters: []string{"Invalid Counter"},
					},
				},
				ScraperControllerSettings: receiverhelper.ScraperControllerSettings{CollectionInterval: time.Minute},
			},
			initializeErr: "The specified object was not found on the computer.\r\n",
		},
		{
			name:      "ScrapeError",
			scrapeErr: errors.New("err1"),
		},
		{
			name:            "CloseError",
			expectedMetrics: 1,
			closeErr:        errors.New("err1"),
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			cfg := test.cfg
			if cfg == nil {
				cfg = defaultConfig
			}
			scraper, err := newScraper(cfg)
			if test.newErr != "" {
				assert.EqualError(t, err, test.newErr)
				return
			}

			err = scraper.initialize(context.Background())
			if test.initializeErr != "" {
				assert.EqualError(t, err, test.initializeErr)
				return
			}
			require.NoError(t, err)

			if test.scrapeErr != nil || test.closeErr != nil {
				for i := range scraper.counters {
					scraper.counters[i] = newMockPerfCounter(test.scrapeErr, test.closeErr)
				}
			}

			metrics, err := scraper.scrape(context.Background())
			if test.scrapeErr != nil {
				assert.Equal(t, err, test.scrapeErr)
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, test.expectedMetrics, metrics.Len())

			err = scraper.close(context.Background())
			if test.closeErr != nil {
				assert.Equal(t, err, test.closeErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
