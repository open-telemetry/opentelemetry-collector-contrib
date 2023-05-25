// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows
// +build windows

package activedirectorydsreceiver

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/collector/receiver/scrapererror"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/winperfcounters"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/activedirectorydsreceiver/internal/metadata"
)

var goldenScrapePath = filepath.Join("testdata", "golden_scrape.yaml")
var partialScrapePath = filepath.Join("testdata", "partial_scrape.yaml")

func TestScrape(t *testing.T) {
	t.Run("Fully successful scrape", func(t *testing.T) {
		t.Parallel()

		mockWatchers, err := getWatchers(&mockCounterCreater{
			availableCounterNames: getAvailableCounters(t),
		})
		require.NoError(t, err)

		scraper := &activeDirectoryDSScraper{
			mb: metadata.NewMetricsBuilder(metadata.DefaultMetricsBuilderConfig(), receivertest.NewNopCreateSettings()),
			w:  mockWatchers,
		}

		scrapeData, err := scraper.scrape(context.Background())
		require.NoError(t, err)

		expectedMetrics, err := golden.ReadMetrics(goldenScrapePath)
		require.NoError(t, err)

		require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, scrapeData, pmetrictest.IgnoreStartTimestamp(),
			pmetrictest.IgnoreTimestamp()))

		err = scraper.shutdown(context.Background())
		require.NoError(t, err)
	})

	t.Run("Scrape with errors", func(t *testing.T) {
		t.Parallel()

		fullSyncObjectsRemainingErr := errors.New("failed to scrape sync objects remaining")
		draInboundValuesDNErr := errors.New("failed to scrape sync inbound value DNs")

		mockWatchers, err := getWatchers(&mockCounterCreater{
			availableCounterNames: getAvailableCounters(t),
		})
		require.NoError(t, err)

		mockWatchers.counterNameToWatcher[draInboundFullSyncObjectsRemaining].(*mockPerfCounterWatcher).scrapeErr = fullSyncObjectsRemainingErr
		mockWatchers.counterNameToWatcher[draInboundValuesDNs].(*mockPerfCounterWatcher).scrapeErr = draInboundValuesDNErr

		scraper := &activeDirectoryDSScraper{
			mb: metadata.NewMetricsBuilder(metadata.DefaultMetricsBuilderConfig(), receivertest.NewNopCreateSettings()),
			w:  mockWatchers,
		}

		scrapeData, err := scraper.scrape(context.Background())
		require.Error(t, err)
		require.True(t, scrapererror.IsPartialScrapeError(err))
		require.Contains(t, err.Error(), fullSyncObjectsRemainingErr.Error())
		require.Contains(t, err.Error(), draInboundValuesDNErr.Error())

		expectedMetrics, err := golden.ReadMetrics(partialScrapePath)
		require.NoError(t, err)

		require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, scrapeData, pmetrictest.IgnoreStartTimestamp(),
			pmetrictest.IgnoreTimestamp()))

		err = scraper.shutdown(context.Background())
		require.NoError(t, err)
	})

	t.Run("Close with errors", func(t *testing.T) {
		t.Parallel()

		fullSyncObjectsRemainingErr := errors.New("failed to close sync objects remaining")
		draInboundValuesDNErr := errors.New("failed to close sync inbound value DNs")

		mockWatchers, err := getWatchers(&mockCounterCreater{
			availableCounterNames: getAvailableCounters(t),
		})
		require.NoError(t, err)

		mockWatchers.counterNameToWatcher[draInboundFullSyncObjectsRemaining].(*mockPerfCounterWatcher).closeErr = fullSyncObjectsRemainingErr
		mockWatchers.counterNameToWatcher[draInboundValuesDNs].(*mockPerfCounterWatcher).closeErr = draInboundValuesDNErr

		scraper := &activeDirectoryDSScraper{
			mb: metadata.NewMetricsBuilder(metadata.DefaultMetricsBuilderConfig(), receivertest.NewNopCreateSettings()),
			w:  mockWatchers,
		}

		err = scraper.shutdown(context.Background())
		require.Error(t, err)
		require.Contains(t, err.Error(), fullSyncObjectsRemainingErr.Error())
		require.Contains(t, err.Error(), draInboundValuesDNErr.Error())
	})

	t.Run("Double shutdown does not error", func(t *testing.T) {
		t.Parallel()

		mockWatchers, err := getWatchers(&mockCounterCreater{
			availableCounterNames: getAvailableCounters(t),
		})
		require.NoError(t, err)

		scraper := &activeDirectoryDSScraper{
			mb: metadata.NewMetricsBuilder(metadata.DefaultMetricsBuilderConfig(), receivertest.NewNopCreateSettings()),
			w:  mockWatchers,
		}

		err = scraper.shutdown(context.Background())
		require.NoError(t, err)

		err = scraper.shutdown(context.Background())
		require.NoError(t, err)
	})
}

type mockPerfCounterWatcher struct {
	val       float64
	scrapeErr error
	closeErr  error
	closed    bool
}

// Path panics; It should not be called
func (mockPerfCounterWatcher) Path() string {
	panic("mockPerfCounterWatcher::Path is not implemented")
}

// ScrapeData returns scrapeErr if it's set, otherwise it returns a single countervalue with the mock's val
func (w mockPerfCounterWatcher) ScrapeData() ([]winperfcounters.CounterValue, error) {
	if w.scrapeErr != nil {
		return nil, w.scrapeErr
	}

	return []winperfcounters.CounterValue{
		{
			Value: w.val,
		},
	}, nil
}

// Close all counters/handles related to the query and free all associated memory.
func (w mockPerfCounterWatcher) Close() error {
	if w.closed {
		panic("mockPerfCounterWatcher was already closed!")
	}

	return w.closeErr
}
