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

	pageReadsPerSec  = "Page Reads/sec"
	pageWritesPerSec = "Page Writes/sec"
)

// scraper for Paging Metrics
type pagingScraper struct {
	settings scraper.Settings
	config   *Config
	mb       *metadata.MetricsBuilder

	pageReadsPerfCounter  winperfcounters.PerfCounterWatcher
	pageWritesPerfCounter winperfcounters.PerfCounterWatcher
	skipScrape            bool

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
