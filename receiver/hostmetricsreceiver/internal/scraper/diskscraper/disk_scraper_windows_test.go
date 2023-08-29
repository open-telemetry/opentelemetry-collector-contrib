// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows
// +build windows

package diskscraper

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/collector/receiver/scrapererror"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/perfcounters"
)

func TestScrape_Error(t *testing.T) {
	type testCase struct {
		name         string
		scrapeErr    error
		getObjectErr error
		getValuesErr error
		expectedErr  string
	}

	testCases := []testCase{
		{
			name:        "scrapeError",
			scrapeErr:   errors.New("err1"),
			expectedErr: "err1",
		},
		{
			name:         "getObjectErr",
			getObjectErr: errors.New("err1"),
			expectedErr:  "err1",
		},
		{
			name:         "getValuesErr",
			getValuesErr: errors.New("err1"),
			expectedErr:  "err1",
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			scraper, err := newDiskScraper(context.Background(), receivertest.NewNopCreateSettings(), &Config{})
			require.NoError(t, err, "Failed to create disk scraper: %v", err)

			scraper.perfCounterScraper = perfcounters.NewMockPerfCounterScraperError(test.scrapeErr, test.getObjectErr, test.getValuesErr, nil)

			err = scraper.start(context.Background(), componenttest.NewNopHost())
			require.NoError(t, err, "Failed to initialize disk scraper: %v", err)

			_, err = scraper.scrape(context.Background())
			assert.EqualError(t, err, test.expectedErr)

			isPartial := scrapererror.IsPartialScrapeError(err)
			assert.True(t, isPartial)
			if isPartial {
				var scraperErr scrapererror.PartialScrapeError
				require.ErrorAs(t, err, &scraperErr)
				assert.Equal(t, metricsLen, scraperErr.Failed)
			}
		})
	}
}

func TestStart_Error(t *testing.T) {
	testCases := []struct {
		name               string
		initError          error
		expectedSkipScrape bool
		expectedErr        string
	}{
		{
			name:               "Perfcounter partially fails to init",
			expectedSkipScrape: false,
		},
		{
			name: "Perfcounter fully fails to init",
			initError: &perfcounters.PerfCounterInitError{
				FailedObjects: []string{"Logical Disk"},
			},
			expectedSkipScrape: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			scraper, err := newDiskScraper(context.Background(), receivertest.NewNopCreateSettings(), &Config{})
			require.NoError(t, err, "Failed to create disk scraper: %v", err)

			scraper.perfCounterScraper = perfcounters.NewMockPerfCounterScraperError(nil, nil, nil, tc.initError)

			err = scraper.start(context.Background(), componenttest.NewNopHost())
			require.NoError(t, err, "Failed to initialize disk scraper: %v", err)

			require.Equal(t, tc.expectedSkipScrape, scraper.skipScrape)
		})
	}
}
