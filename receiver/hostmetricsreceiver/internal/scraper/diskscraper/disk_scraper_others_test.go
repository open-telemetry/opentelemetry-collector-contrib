// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !windows
// +build !windows

package diskscraper

import (
	"context"
	"errors"
	"testing"

	"github.com/shirou/gopsutil/v3/disk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/collector/receiver/scrapererror"
)

func TestScrape_Others(t *testing.T) {
	type testCase struct {
		name           string
		ioCountersFunc func(ctx context.Context, names ...string) (map[string]disk.IOCountersStat, error)
		expectedErr    string
	}

	testCases := []testCase{
		{
			name: "Error",
			ioCountersFunc: func(_ context.Context, names ...string) (map[string]disk.IOCountersStat, error) {
				return nil, errors.New("err1")
			},
			expectedErr: "err1",
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			scraper, err := newDiskScraper(context.Background(), receivertest.NewNopCreateSettings(), &Config{})
			require.NoError(t, err, "Failed to create disk scraper: %v", err)

			if test.ioCountersFunc != nil {
				scraper.ioCounters = test.ioCountersFunc
			}

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
