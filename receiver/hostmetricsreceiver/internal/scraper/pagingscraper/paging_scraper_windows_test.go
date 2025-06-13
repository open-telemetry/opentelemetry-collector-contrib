// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package pagingscraper

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/scraper/scrapererror"
	"go.opentelemetry.io/collector/scraper/scrapertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/winperfcounters"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/pagingscraper/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/testmocks"
)

func TestScrape_Errors(t *testing.T) {
	type testCase struct {
		name                         string
		pageSize                     uint64
		getPageFileStats             func() ([]*pageFileStats, error)
		pageReadsScrapeErr           error
		pageWritesScrapeErr          error
		pageMajFaultsScrapeErr       error
		pageFaultsScrapeErr          error
		expectedErr                  string
		expectedErrCount             int
		expectedUsedValue            int64
		expectedFreeValue            int64
		expectedUtilizationFreeValue float64
		expectedUtilizationUsedValue float64
	}

	testPageSize := uint64(4096)
	testPageFileData := &pageFileStats{usedBytes: 200, freeBytes: 800, totalBytes: 1000}

	testCases := []testCase{
		{
			name:     "standard",
			pageSize: testPageSize,
			getPageFileStats: func() ([]*pageFileStats, error) {
				return []*pageFileStats{testPageFileData}, nil
			},
			expectedUsedValue:            int64(testPageFileData.usedBytes),
			expectedFreeValue:            int64(testPageFileData.freeBytes),
			expectedUtilizationFreeValue: 0.8,
			expectedUtilizationUsedValue: 0.2,
		},
		{
			name:             "pageFileError",
			getPageFileStats: func() ([]*pageFileStats, error) { return nil, errors.New("err1") },
			expectedErr:      "failed to read page file stats: err1",
			expectedErrCount: pagingUsageMetricsLen,
		},
		{
			name:               "pageReadsScrapeError",
			pageReadsScrapeErr: errors.New("err1"),
			expectedErr:        "err1",
			expectedErrCount:   pagingMetricsLen,
		},
		{
			name:                "pageWritesScrapeError",
			pageWritesScrapeErr: errors.New("err2"),
			expectedErr:         "err2",
			expectedErrCount:    pagingMetricsLen,
		},
		{
			name:                   "pageMajFaultsScrapeError",
			pageMajFaultsScrapeErr: errors.New("err3"),
			expectedErr:            "err3",
			expectedErrCount:       2, // If major faults failed to be scraped, the code can't report minor faults either
		},
		{
			name:                "pageFaultsScrapeError",
			pageFaultsScrapeErr: errors.New("err4"),
			expectedErr:         "err4",
			expectedErrCount:    1,
		},
		{
			name:                   "multipleErrors-majorFaultErr",
			getPageFileStats:       func() ([]*pageFileStats, error) { return nil, errors.New("err1") },
			pageReadsScrapeErr:     errors.New("err2"),
			pageMajFaultsScrapeErr: errors.New("err3"),
			expectedErr:            "failed to read page file stats: err1; err2; err3",
			expectedErrCount:       4, // If major faults failed to be scraped, the code can't report minor faults either
		},
		{
			name:                "multipleErrors-minorFaultErr",
			getPageFileStats:    func() ([]*pageFileStats, error) { return nil, errors.New("err1") },
			pageReadsScrapeErr:  errors.New("err2"),
			pageFaultsScrapeErr: errors.New("err3"),
			expectedErr:         "failed to read page file stats: err1; err2; err3",
			expectedErrCount:    3,
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			metricsConfig := metadata.DefaultMetricsBuilderConfig()
			metricsConfig.Metrics.SystemPagingUtilization.Enabled = true

			scraper := newPagingScraper(context.Background(), scrapertest.NewNopSettings(metadata.Type), &Config{MetricsBuilderConfig: metricsConfig})
			if test.getPageFileStats != nil {
				scraper.pageFileStats = test.getPageFileStats
			}

			pageSizeOnce.Do(func() {}) // Run this now so what we set pageSize to here is not overridden
			if test.pageSize > 0 {
				pageSize = test.pageSize
			} else {
				pageSize = getPageSize()
				assert.Positive(t, pageSize)
				assert.Zero(t, pageSize%4096) // page size on Windows should always be a multiple of 4KB
			}

			const (
				defaultPageReadsPerSec  int64 = 1000
				defaultPageWritesPerSec int64 = 500
				defaultPageFaultsPerSec int64 = 300
				defaultPageMajPerSec    int64 = 200
			)
			scraper.perfCounterFactory = func(_, _, counter string) (winperfcounters.PerfCounterWatcher, error) {
				perfCounterMock := &testmocks.PerfCounterWatcherMock{}
				switch counter {
				case pageReadsPerSec:
					perfCounterMock.Val = defaultPageReadsPerSec
					if test.pageReadsScrapeErr != nil {
						return &testmocks.PerfCounterWatcherMock{
							ScrapeErr: test.pageReadsScrapeErr,
						}, nil
					}
				case pageWritesPerSec:
					perfCounterMock.Val = defaultPageWritesPerSec
					if test.pageWritesScrapeErr != nil {
						return &testmocks.PerfCounterWatcherMock{
							ScrapeErr: test.pageWritesScrapeErr,
						}, nil
					}
				case pageFaultsPerSec:
					perfCounterMock.Val = defaultPageFaultsPerSec
					if test.pageFaultsScrapeErr != nil {
						return &testmocks.PerfCounterWatcherMock{
							ScrapeErr: test.pageFaultsScrapeErr,
						}, nil
					}
				case pageMajPerSec:
					perfCounterMock.Val = defaultPageMajPerSec
					if test.pageMajFaultsScrapeErr != nil {
						return &testmocks.PerfCounterWatcherMock{
							ScrapeErr: test.pageMajFaultsScrapeErr,
						}, nil
					}
				}

				return perfCounterMock, nil
			}

			err := scraper.start(context.Background(), componenttest.NewNopHost())
			require.NoError(t, err, "Failed to initialize paging scraper: %v", err)

			md, err := scraper.scrape(context.Background())
			if test.expectedErr != "" {
				assert.EqualError(t, err, test.expectedErr)

				isPartial := scrapererror.IsPartialScrapeError(err)
				assert.True(t, isPartial)
				if isPartial {
					var scraperErr scrapererror.PartialScrapeError
					require.ErrorAs(t, err, &scraperErr)
					assert.Equal(t, test.expectedErrCount, scraperErr.Failed)
				}

				return
			}

			assert.NoError(t, err)

			metrics := md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
			for i := 0; i < metrics.Len(); i++ {
				metric := metrics.At(i)
				switch metric.Name() {
				case metadata.MetricsInfo.SystemPagingFaults.Name:
					assert.Equal(t, defaultPageMajPerSec, metric.Sum().DataPoints().At(0).IntValue())
					assert.Equal(t, defaultPageFaultsPerSec-defaultPageMajPerSec, metric.Sum().DataPoints().At(1).IntValue())
				case metadata.MetricsInfo.SystemPagingOperations.Name:
					assert.Equal(t, defaultPageReadsPerSec, metric.Sum().DataPoints().At(0).IntValue())
					assert.Equal(t, defaultPageWritesPerSec, metric.Sum().DataPoints().At(1).IntValue())
				case metadata.MetricsInfo.SystemPagingUsage.Name:
					assert.Equal(t, test.expectedUsedValue, metric.Sum().DataPoints().At(0).IntValue())
					assert.Equal(t, test.expectedFreeValue, metric.Sum().DataPoints().At(1).IntValue())
				case metadata.MetricsInfo.SystemPagingUtilization.Name:
					assert.Equal(t, test.expectedUtilizationUsedValue, metric.Gauge().DataPoints().At(0).DoubleValue())
					assert.Equal(t, test.expectedUtilizationFreeValue, metric.Gauge().DataPoints().At(1).DoubleValue())
				default:
					assert.Fail(t, "Unexpected metric found", metric.Name())
				}
			}
		})
	}
}

func TestPagingScrapeWithRealData(t *testing.T) {
	config := Config{
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
	}
	scraper := newPagingScraper(context.Background(), scrapertest.NewNopSettings(metadata.Type), &config)

	err := scraper.start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err, "Failed to start the paging scraper")

	metrics, err := scraper.scrape(context.Background())
	require.NoError(t, err, "Failed to scrape metrics")
	require.NotNil(t, metrics, "Metrics cannot be nil")

	// Expected metric names for paging scraper.
	expectedMetrics := map[string]bool{
		metadata.MetricsInfo.SystemPagingOperations.Name: false,
		metadata.MetricsInfo.SystemPagingUsage.Name:      false,
		metadata.MetricsInfo.SystemPagingFaults.Name:     false,
	}

	internal.AssertExpectedMetrics(t, expectedMetrics, metrics)
}

func TestStart_Error(t *testing.T) {
	testCases := []struct {
		name                  string
		newPerfCounterFactory func(string, string, string) (winperfcounters.PerfCounterWatcher, error)
		expectedSkipScrape    bool
	}{
		{
			name:               "new_perf_counter_watcher_succeeds",
			expectedSkipScrape: false,
		},
		{
			name: "new_perf_counter_watcher_fails",
			newPerfCounterFactory: func(string, string, string) (winperfcounters.PerfCounterWatcher, error) {
				return nil, errors.New("err1")
			},
			expectedSkipScrape: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			metricsConfig := metadata.DefaultMetricsBuilderConfig()
			metricsConfig.Metrics.SystemPagingUtilization.Enabled = true

			scraper := newPagingScraper(context.Background(), scrapertest.NewNopSettings(metadata.Type), &Config{MetricsBuilderConfig: metricsConfig})

			if tc.newPerfCounterFactory != nil {
				scraper.perfCounterFactory = tc.newPerfCounterFactory
			}

			err := scraper.start(context.Background(), componenttest.NewNopHost())
			require.NoError(t, err, "Failed to initialize paging scraper: %v", err)

			require.Equal(t, tc.expectedSkipScrape, scraper.skipScrape)
		})
	}
}
