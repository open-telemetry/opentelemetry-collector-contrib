// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build windows
// +build windows

package activedirectorydsreceiver

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/receiver/scrapererror"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/scrapertest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/scrapertest/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/winperfcounters"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/activedirectorydsreceiver/internal/metadata"
)

var goldenScrapePath = filepath.Join("testdata", "golden_scrape.json")
var partialScrapePath = filepath.Join("testdata", "partial_scrape.json")

func TestScrape(t *testing.T) {
	t.Run("Fully successful scrape", func(t *testing.T) {
		t.Parallel()

		mockWatchers := &watchers{
			DRAInboundBytesCompressed:              mockPerfCounterWatcher{val: 0},
			DRAInboundBytesNotCompressed:           mockPerfCounterWatcher{val: 1},
			DRAOutboundBytesCompressed:             mockPerfCounterWatcher{val: 2},
			DRAOutboundBytesNotCompressed:          mockPerfCounterWatcher{val: 3},
			DRAInboundFullSyncObjectsRemaining:     mockPerfCounterWatcher{val: 4},
			DRAInboundObjects:                      mockPerfCounterWatcher{val: 5},
			DRAOutboundObjects:                     mockPerfCounterWatcher{val: 6},
			DRAInboundProperties:                   mockPerfCounterWatcher{val: 7},
			DRAOutboundProperties:                  mockPerfCounterWatcher{val: 8},
			DRAInboundValuesDNs:                    mockPerfCounterWatcher{val: 9},
			DRAInboundValuesTotal:                  mockPerfCounterWatcher{val: 101},
			DRAOutboundValuesDNs:                   mockPerfCounterWatcher{val: 11},
			DRAOutboundValuesTotal:                 mockPerfCounterWatcher{val: 121},
			DRAPendingReplicationOperations:        mockPerfCounterWatcher{val: 13},
			DRASyncFailuresSchemaMismatch:          mockPerfCounterWatcher{val: 14},
			DRASyncRequestsSuccessful:              mockPerfCounterWatcher{val: 15},
			DRASyncRequestsMade:                    mockPerfCounterWatcher{val: 163},
			DSDirectoryReads:                       mockPerfCounterWatcher{val: 17},
			DSDirectoryWrites:                      mockPerfCounterWatcher{val: 18},
			DSDirectorySearches:                    mockPerfCounterWatcher{val: 19},
			DSClientBinds:                          mockPerfCounterWatcher{val: 20},
			DSServerBinds:                          mockPerfCounterWatcher{val: 21},
			DSNameCacheHitRate:                     mockPerfCounterWatcher{val: 22},
			DSNotifyQueueSize:                      mockPerfCounterWatcher{val: 23},
			DSSecurityDescriptorPropagationsEvents: mockPerfCounterWatcher{val: 24},
			DSSearchSubOperations:                  mockPerfCounterWatcher{val: 25},
			DSSecurityDescripterSubOperations:      mockPerfCounterWatcher{val: 26},
			DSThreadsInUse:                         mockPerfCounterWatcher{val: 27},
			LDAPClientSessions:                     mockPerfCounterWatcher{val: 28},
			LDAPBindTime:                           mockPerfCounterWatcher{val: 29},
			LDAPSuccessfulBinds:                    mockPerfCounterWatcher{val: 30},
			LDAPSearches:                           mockPerfCounterWatcher{val: 31},
		}

		scraper := &activeDirectoryDSScraper{
			mb: metadata.NewMetricsBuilder(metadata.DefaultMetricsSettings()),
			w:  mockWatchers,
		}

		scrapeData, err := scraper.scrape(context.Background())
		require.NoError(t, err)

		expectedMetrics, err := golden.ReadMetrics(goldenScrapePath)
		require.NoError(t, err)

		err = scrapertest.CompareMetrics(expectedMetrics, scrapeData)
		require.NoError(t, err)

		err = scraper.shutdown(context.Background())
		require.NoError(t, err)
	})

	t.Run("Scrape with errors", func(t *testing.T) {
		t.Parallel()

		fullSyncObjectsRemainingErr := errors.New("failed to scrape sync objects remaining")
		draInboundValuesDNErr := errors.New("failed to scrape sync inbound value DNs")

		mockWatchers := &watchers{
			DRAInboundBytesCompressed:              mockPerfCounterWatcher{val: 0},
			DRAInboundBytesNotCompressed:           mockPerfCounterWatcher{val: 1},
			DRAOutboundBytesCompressed:             mockPerfCounterWatcher{val: 2},
			DRAOutboundBytesNotCompressed:          mockPerfCounterWatcher{val: 3},
			DRAInboundFullSyncObjectsRemaining:     mockPerfCounterWatcher{scrapeErr: fullSyncObjectsRemainingErr},
			DRAInboundObjects:                      mockPerfCounterWatcher{val: 5},
			DRAOutboundObjects:                     mockPerfCounterWatcher{val: 6},
			DRAInboundProperties:                   mockPerfCounterWatcher{val: 7},
			DRAOutboundProperties:                  mockPerfCounterWatcher{val: 8},
			DRAInboundValuesDNs:                    mockPerfCounterWatcher{val: 9, scrapeErr: draInboundValuesDNErr},
			DRAInboundValuesTotal:                  mockPerfCounterWatcher{val: 10},
			DRAOutboundValuesDNs:                   mockPerfCounterWatcher{val: 11},
			DRAOutboundValuesTotal:                 mockPerfCounterWatcher{val: 12},
			DRAPendingReplicationOperations:        mockPerfCounterWatcher{val: 13},
			DRASyncFailuresSchemaMismatch:          mockPerfCounterWatcher{val: 14},
			DRASyncRequestsSuccessful:              mockPerfCounterWatcher{val: 15},
			DRASyncRequestsMade:                    mockPerfCounterWatcher{val: 16},
			DSDirectoryReads:                       mockPerfCounterWatcher{val: 17},
			DSDirectoryWrites:                      mockPerfCounterWatcher{val: 18},
			DSDirectorySearches:                    mockPerfCounterWatcher{val: 19},
			DSClientBinds:                          mockPerfCounterWatcher{val: 20},
			DSServerBinds:                          mockPerfCounterWatcher{val: 21},
			DSNameCacheHitRate:                     mockPerfCounterWatcher{val: 22},
			DSNotifyQueueSize:                      mockPerfCounterWatcher{val: 23},
			DSSecurityDescriptorPropagationsEvents: mockPerfCounterWatcher{val: 24},
			DSSearchSubOperations:                  mockPerfCounterWatcher{val: 25},
			DSSecurityDescripterSubOperations:      mockPerfCounterWatcher{val: 26},
			DSThreadsInUse:                         mockPerfCounterWatcher{val: 27},
			LDAPClientSessions:                     mockPerfCounterWatcher{val: 28},
			LDAPBindTime:                           mockPerfCounterWatcher{val: 29},
			LDAPSuccessfulBinds:                    mockPerfCounterWatcher{val: 30},
			LDAPSearches:                           mockPerfCounterWatcher{val: 31},
		}

		scraper := &activeDirectoryDSScraper{
			mb: metadata.NewMetricsBuilder(metadata.DefaultMetricsSettings()),
			w:  mockWatchers,
		}

		scrapeData, err := scraper.scrape(context.Background())
		require.Error(t, err)
		require.True(t, scrapererror.IsPartialScrapeError(err))
		require.Contains(t, err.Error(), fullSyncObjectsRemainingErr.Error())
		require.Contains(t, err.Error(), draInboundValuesDNErr.Error())

		expectedMetrics, err := golden.ReadMetrics(partialScrapePath)
		require.NoError(t, err)

		err = scrapertest.CompareMetrics(expectedMetrics, scrapeData)
		require.NoError(t, err)

		err = scraper.shutdown(context.Background())
		require.NoError(t, err)
	})

	t.Run("Close with errors", func(t *testing.T) {
		t.Parallel()

		fullSyncObjectsRemainingErr := errors.New("failed to close sync objects remaining")
		draInboundValuesDNErr := errors.New("failed to close sync inbound value DNs")

		mockWatchers := &watchers{
			DRAInboundBytesCompressed:              mockPerfCounterWatcher{val: 0},
			DRAInboundBytesNotCompressed:           mockPerfCounterWatcher{val: 1},
			DRAOutboundBytesCompressed:             mockPerfCounterWatcher{val: 2},
			DRAOutboundBytesNotCompressed:          mockPerfCounterWatcher{val: 3},
			DRAInboundFullSyncObjectsRemaining:     mockPerfCounterWatcher{closeErr: fullSyncObjectsRemainingErr},
			DRAInboundObjects:                      mockPerfCounterWatcher{val: 5},
			DRAOutboundObjects:                     mockPerfCounterWatcher{val: 6},
			DRAInboundProperties:                   mockPerfCounterWatcher{val: 7},
			DRAOutboundProperties:                  mockPerfCounterWatcher{val: 8},
			DRAInboundValuesDNs:                    mockPerfCounterWatcher{val: 9, closeErr: draInboundValuesDNErr},
			DRAInboundValuesTotal:                  mockPerfCounterWatcher{val: 10},
			DRAOutboundValuesDNs:                   mockPerfCounterWatcher{val: 11},
			DRAOutboundValuesTotal:                 mockPerfCounterWatcher{val: 12},
			DRAPendingReplicationOperations:        mockPerfCounterWatcher{val: 13},
			DRASyncFailuresSchemaMismatch:          mockPerfCounterWatcher{val: 14},
			DRASyncRequestsSuccessful:              mockPerfCounterWatcher{val: 15},
			DRASyncRequestsMade:                    mockPerfCounterWatcher{val: 16},
			DSDirectoryReads:                       mockPerfCounterWatcher{val: 17},
			DSDirectoryWrites:                      mockPerfCounterWatcher{val: 18},
			DSDirectorySearches:                    mockPerfCounterWatcher{val: 19},
			DSClientBinds:                          mockPerfCounterWatcher{val: 20},
			DSServerBinds:                          mockPerfCounterWatcher{val: 21},
			DSNameCacheHitRate:                     mockPerfCounterWatcher{val: 22},
			DSNotifyQueueSize:                      mockPerfCounterWatcher{val: 23},
			DSSecurityDescriptorPropagationsEvents: mockPerfCounterWatcher{val: 24},
			DSSearchSubOperations:                  mockPerfCounterWatcher{val: 25},
			DSSecurityDescripterSubOperations:      mockPerfCounterWatcher{val: 26},
			DSThreadsInUse:                         mockPerfCounterWatcher{val: 27},
			LDAPClientSessions:                     mockPerfCounterWatcher{val: 28},
			LDAPBindTime:                           mockPerfCounterWatcher{val: 29},
			LDAPSuccessfulBinds:                    mockPerfCounterWatcher{val: 30},
			LDAPSearches:                           mockPerfCounterWatcher{val: 31},
		}

		scraper := &activeDirectoryDSScraper{
			mb: metadata.NewMetricsBuilder(metadata.DefaultMetricsSettings()),
			w:  mockWatchers,
		}

		err := scraper.shutdown(context.Background())
		require.Error(t, err)
		require.Contains(t, err.Error(), fullSyncObjectsRemainingErr.Error())
		require.Contains(t, err.Error(), draInboundValuesDNErr.Error())
	})

	t.Run("Double shutdown does not error", func(t *testing.T) {
		t.Parallel()
		
		mockWatchers := &watchers{
			DRAInboundBytesCompressed:              mockPerfCounterWatcher{val: 0},
			DRAInboundBytesNotCompressed:           mockPerfCounterWatcher{val: 1},
			DRAOutboundBytesCompressed:             mockPerfCounterWatcher{val: 2},
			DRAOutboundBytesNotCompressed:          mockPerfCounterWatcher{val: 3},
			DRAInboundFullSyncObjectsRemaining:     mockPerfCounterWatcher{val: 4},
			DRAInboundObjects:                      mockPerfCounterWatcher{val: 5},
			DRAOutboundObjects:                     mockPerfCounterWatcher{val: 6},
			DRAInboundProperties:                   mockPerfCounterWatcher{val: 7},
			DRAOutboundProperties:                  mockPerfCounterWatcher{val: 8},
			DRAInboundValuesDNs:                    mockPerfCounterWatcher{val: 9},
			DRAInboundValuesTotal:                  mockPerfCounterWatcher{val: 10},
			DRAOutboundValuesDNs:                   mockPerfCounterWatcher{val: 11},
			DRAOutboundValuesTotal:                 mockPerfCounterWatcher{val: 12},
			DRAPendingReplicationOperations:        mockPerfCounterWatcher{val: 13},
			DRASyncFailuresSchemaMismatch:          mockPerfCounterWatcher{val: 14},
			DRASyncRequestsSuccessful:              mockPerfCounterWatcher{val: 15},
			DRASyncRequestsMade:                    mockPerfCounterWatcher{val: 16},
			DSDirectoryReads:                       mockPerfCounterWatcher{val: 17},
			DSDirectoryWrites:                      mockPerfCounterWatcher{val: 18},
			DSDirectorySearches:                    mockPerfCounterWatcher{val: 19},
			DSClientBinds:                          mockPerfCounterWatcher{val: 20},
			DSServerBinds:                          mockPerfCounterWatcher{val: 21},
			DSNameCacheHitRate:                     mockPerfCounterWatcher{val: 22},
			DSNotifyQueueSize:                      mockPerfCounterWatcher{val: 23},
			DSSecurityDescriptorPropagationsEvents: mockPerfCounterWatcher{val: 24},
			DSSearchSubOperations:                  mockPerfCounterWatcher{val: 25},
			DSSecurityDescripterSubOperations:      mockPerfCounterWatcher{val: 26},
			DSThreadsInUse:                         mockPerfCounterWatcher{val: 27},
			LDAPClientSessions:                     mockPerfCounterWatcher{val: 28},
			LDAPBindTime:                           mockPerfCounterWatcher{val: 29},
			LDAPSuccessfulBinds:                    mockPerfCounterWatcher{val: 30},
			LDAPSearches:                           mockPerfCounterWatcher{val: 31},
		}

		scraper := &activeDirectoryDSScraper{
			mb: metadata.NewMetricsBuilder(metadata.DefaultMetricsSettings()),
			w:  mockWatchers,
		}

		err := scraper.shutdown(context.Background())
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

// GetMetricRep panics; It should not be called
func (mockPerfCounterWatcher) GetMetricRep() winperfcounters.MetricRep {
	panic("mockPerfCounterWatcher::GetMetricRep is not implemented")
}
