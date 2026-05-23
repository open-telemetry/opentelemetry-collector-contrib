// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package pagingscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/pagingscraper"

import (
	"context"
	"fmt"
	"time"

	"github.com/shirou/gopsutil/v4/host"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/scraper"
	"go.opentelemetry.io/collector/scraper/scrapererror"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/winperfcounters"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/pagingscraper/internal/metadata"
)

const (
	pagingUsageMetricsLen = 1
	pagingMetricsLen      = 1

	memory = "Memory"

	// The counters below are per second rates, but, instead of reading the calculated rates,
	// we read the raw values and post them as cumulative metrics.
	pageReadsPerSec  = "Page Reads/sec"
	pageWritesPerSec = "Page Writes/sec"
	pageFaultsPerSec = "Page Faults/sec" // All page faults, including minor and major, aka soft and hard faults
	pageMajPerSec    = "Pages/sec"       // Only major, aka hard, page faults.
)

// scraper for Paging Metrics
type pagingScraper struct {
	settings scraper.Settings
	config   *Config
	mb       *metadata.MetricsBuilder

	pageReadsPerfCounter     winperfcounters.PerfCounterWatcher
	pageWritesPerfCounter    winperfcounters.PerfCounterWatcher
	pageFaultsPerfCounter    winperfcounters.PerfCounterWatcher
	pageMajFaultsPerfCounter winperfcounters.PerfCounterWatcher
	skipScrape               bool

	// for mocking
	bootTime           func(context.Context) (uint64, error)
	pageFileStats      func() ([]*pageFileStats, error)
	perfCounterFactory func(string, string, string) (winperfcounters.PerfCounterWatcher, error)
}

// newPagingScraper creates a Paging Scraper
func newPagingScraper(_ context.Context, settings scraper.Settings, cfg *Config) *pagingScraper {
	return &pagingScraper{
		settings:           settings,
		config:             cfg,
		bootTime:           host.BootTimeWithContext,
		pageFileStats:      getPageFileStats,
		perfCounterFactory: winperfcounters.NewWatcher,
	}
}

func (s *pagingScraper) start(ctx context.Context, _ component.Host) error {
	bootTime, err := s.bootTime(ctx)
	if err != nil {
		return err
	}

	s.mb = metadata.NewMetricsBuilder(s.config.MetricsBuilderConfig, s.settings, metadata.WithStartTime(pcommon.Timestamp(bootTime*1e9)))

	s.pageReadsPerfCounter, err = s.perfCounterFactory(memory, "", pageReadsPerSec)
	if err != nil {
		s.settings.Logger.Error("Failed to create performance counter to read pages read / sec", zap.Error(err))
		s.skipScrape = true
	}

	s.pageWritesPerfCounter, err = s.perfCounterFactory(memory, "", pageWritesPerSec)
	if err != nil {
		s.settings.Logger.Error("Failed to create performance counter to write pages read / sec", zap.Error(err))
		s.skipScrape = true
	}

	s.pageFaultsPerfCounter, err = s.perfCounterFactory(memory, "", pageFaultsPerSec)
	if err != nil {
		s.settings.Logger.Error("Failed to create performance counter for page faults / sec", zap.Error(err))
		s.skipScrape = true
	}

	s.pageMajFaultsPerfCounter, err = s.perfCounterFactory(memory, "", pageMajPerSec)
	if err != nil {
		s.settings.Logger.Error("Failed to create performance counter for major, aka hard, page faults / sec", zap.Error(err))
		s.skipScrape = true
	}

	return nil
}

func (s *pagingScraper) scrape(context.Context) (pmetric.Metrics, error) {
	if s.skipScrape {
		return pmetric.NewMetrics(), nil
	}

	var errors scrapererror.ScrapeErrors

	err := s.scrapePagingUsageMetric()
	if err != nil {
		errors.AddPartial(pagingUsageMetricsLen, err)
	}

	err = s.scrapePagingOperationsMetric()
	if err != nil {
		errors.AddPartial(pagingMetricsLen, err)
	}

	s.scrapePagingFaultsMetric(&errors)

	return s.mb.Emit(), errors.Combine()
}

func (s *pagingScraper) scrapePagingUsageMetric() error {
	now := pcommon.NewTimestampFromTime(time.Now())
	pageFiles, err := s.pageFileStats()
	if err != nil {
		return fmt.Errorf("failed to read page file stats: %w", err)
	}

	s.recordPagingUsageDataPoints(now, pageFiles)
	s.recordPagingUtilizationDataPoints(now, pageFiles)

	return nil
}

func (s *pagingScraper) recordPagingUsageDataPoints(now pcommon.Timestamp, pageFiles []*pageFileStats) {
	for _, pageFile := range pageFiles {
		s.mb.RecordSystemPagingUsageDataPoint(now, int64(pageFile.usedBytes), pageFile.deviceName, metadata.AttributeStateUsed)
		s.mb.RecordSystemPagingUsageDataPoint(now, int64(pageFile.freeBytes), pageFile.deviceName, metadata.AttributeStateFree)
	}
}

func (s *pagingScraper) recordPagingUtilizationDataPoints(now pcommon.Timestamp, pageFiles []*pageFileStats) {
	for _, pageFile := range pageFiles {
		s.mb.RecordSystemPagingUtilizationDataPoint(now, float64(pageFile.usedBytes)/float64(pageFile.totalBytes), pageFile.deviceName, metadata.AttributeStateUsed)
		s.mb.RecordSystemPagingUtilizationDataPoint(now, float64(pageFile.freeBytes)/float64(pageFile.totalBytes), pageFile.deviceName, metadata.AttributeStateFree)
	}
}

func (s *pagingScraper) scrapePagingOperationsMetric() error {
	now := pcommon.NewTimestampFromTime(time.Now())

	var pageReadsPerSecValue int64
	pageReadsHasValue, err := s.pageReadsPerfCounter.ScrapeRawValue(&pageReadsPerSecValue)
	if err != nil {
		return err
	}
	if pageReadsHasValue {
		s.mb.RecordSystemPagingOperationsDataPoint(now, pageReadsPerSecValue, metadata.AttributeDirectionPageIn, metadata.AttributeTypeMajor)
	}

	var pageWritesPerSecValue int64
	pageWritesHasValue, err := s.pageWritesPerfCounter.ScrapeRawValue(&pageWritesPerSecValue)
	if err != nil {
		return err
	}
	if pageWritesHasValue {
		s.mb.RecordSystemPagingOperationsDataPoint(now, pageWritesPerSecValue, metadata.AttributeDirectionPageOut, metadata.AttributeTypeMajor)
	}

	return nil
}

func (s *pagingScraper) scrapePagingFaultsMetric(errors *scrapererror.ScrapeErrors) {
	now := pcommon.NewTimestampFromTime(time.Now())

	var pageMajFaultsPerSecValue int64
	pageMajFaultsHasValue, err := s.pageMajFaultsPerfCounter.ScrapeRawValue(&pageMajFaultsPerSecValue)
	if err != nil {
		// Count is 2 since without major page faults none of the paging metrics will be recorded
		errors.AddPartial(2, err)
		return
	}
	if !pageMajFaultsHasValue {
		s.settings.Logger.Debug(
			"Skipping paging faults metrics as no value was scraped for 'Pages/sec' performance counter")
		return
	}
	s.mb.RecordSystemPagingFaultsDataPoint(now, pageMajFaultsPerSecValue, metadata.AttributeTypeMajor)

	var pageFaultsPerSecValue int64
	pageFaultsHasValue, err := s.pageFaultsPerfCounter.ScrapeRawValue(&pageFaultsPerSecValue)
	if err != nil {
		errors.AddPartial(1, err)
	}
	if pageFaultsHasValue {
		s.mb.RecordSystemPagingFaultsDataPoint(now, pageFaultsPerSecValue-pageMajFaultsPerSecValue, metadata.AttributeTypeMinor)
	} else {
		s.settings.Logger.Debug(
			"Skipping minor paging faults metric as no value was scraped for 'Page Faults/sec' performance counter")
	}
}
