// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !windows
// +build !windows

package pagingscraper

import (
	"context"
	"errors"
	"testing"

	"github.com/shirou/gopsutil/v3/mem"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/collector/receiver/scrapererror"
)

func TestScrape_Errors(t *testing.T) {
	type testCase struct {
		name              string
		virtualMemoryFunc func() ([]*pageFileStats, error)
		swapMemoryFunc    func() (*mem.SwapMemoryStat, error)
		expectedError     string
		expectedErrCount  int
	}

	testCases := []testCase{
		{
			name:              "virtualMemoryError",
			virtualMemoryFunc: func() ([]*pageFileStats, error) { return nil, errors.New("err1") },
			expectedError:     "failed to read page file stats: err1",
			expectedErrCount:  pagingUsageMetricsLen,
		},
		{
			name:             "swapMemoryError",
			swapMemoryFunc:   func() (*mem.SwapMemoryStat, error) { return nil, errors.New("err2") },
			expectedError:    "failed to read swap info: err2",
			expectedErrCount: pagingMetricsLen,
		},
		{
			name:              "multipleErrors",
			virtualMemoryFunc: func() ([]*pageFileStats, error) { return nil, errors.New("err1") },
			swapMemoryFunc:    func() (*mem.SwapMemoryStat, error) { return nil, errors.New("err2") },
			expectedError:     "failed to read page file stats: err1; failed to read swap info: err2",
			expectedErrCount:  pagingUsageMetricsLen + pagingMetricsLen,
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			scraper := newPagingScraper(context.Background(), receivertest.NewNopCreateSettings(), &Config{})
			if test.virtualMemoryFunc != nil {
				scraper.getPageFileStats = test.virtualMemoryFunc
			}
			if test.swapMemoryFunc != nil {
				scraper.swapMemory = test.swapMemoryFunc
			}

			err := scraper.start(context.Background(), componenttest.NewNopHost())
			require.NoError(t, err, "Failed to initialize paging scraper: %v", err)

			_, err = scraper.scrape(context.Background())
			assert.EqualError(t, err, test.expectedError)

			isPartial := scrapererror.IsPartialScrapeError(err)
			assert.True(t, isPartial)
			if isPartial {
				var scraperErr scrapererror.PartialScrapeError
				require.ErrorAs(t, err, &scraperErr)
				assert.Equal(t, test.expectedErrCount, scraperErr.Failed)
			}
		})
	}
}
