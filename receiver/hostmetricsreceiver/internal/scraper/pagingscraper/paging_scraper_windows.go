// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows
// +build windows

package pagingscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/pagingscraper"

import (
	"context"
	"fmt"
	"time"

	"github.com/shirou/gopsutil/v3/common"
	"github.com/shirou/gopsutil/v3/host"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scrapererror"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/perfcounters"
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
type scraper struct {
	settings receiver.CreateSettings
	config   *Config
	mb       *metadata.MetricsBuilder

	perfCounterScraper perfcounters.PerfCounterScraper
	skipScrape         bool

	// for mocking
	bootTime      func(context.Context) (uint64, error)
	pageFileStats func() ([]*pageFileStats, error)
}

// newPagingScraper creates a Paging Scraper
func newPagingScraper(_ context.Context, settings receiver.CreateSettings, cfg *Config, _ common.EnvMap) *scraper {
	return &scraper{
		settings:           settings,
		config:             cfg,
		perfCounterScraper: &perfcounters.PerfLibScraper{},
		bootTime:           host.BootTimeWithContext,
		pageFileStats:      getPageFileStats,
	}
}

func (s *scraper) start(ctx context.Context, _ component.Host) error {
	bootTime, err := s.bootTime(ctx)
	if err != nil {
		return err
	}

	s.mb = metadata.NewMetricsBuilder(s.config.MetricsBuilderConfig, s.settings, metadata.WithStartTime(pcommon.Timestamp(bootTime*1e9)))

	if err = s.perfCounterScraper.Initialize(memory); err != nil {
		s.settings.Logger.Error("Failed to initialize performance counter, paging metrics will not scrape", zap.Error(err))
		s.skipScrape = true
	}
	return nil
}

func (s *scraper) scrape(context.Context) (pmetric.Metrics, error) {
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

func (s *scraper) scrapePagingUsageMetric() error {
	now := pcommon.NewTimestampFromTime(time.Now())
	pageFiles, err := s.pageFileStats()
	if err != nil {
		return fmt.Errorf("failed to read page file stats: %w", err)
	}

	s.recordPagingUsageDataPoints(now, pageFiles)
	s.recordPagingUtilizationDataPoints(now, pageFiles)

	return nil
}

func (s *scraper) recordPagingUsageDataPoints(now pcommon.Timestamp, pageFiles []*pageFileStats) {
	for _, pageFile := range pageFiles {
		s.mb.RecordSystemPagingUsageDataPoint(now, int64(pageFile.usedBytes), pageFile.deviceName, metadata.AttributeStateUsed)
		s.mb.RecordSystemPagingUsageDataPoint(now, int64(pageFile.freeBytes), pageFile.deviceName, metadata.AttributeStateFree)
	}
}

func (s *scraper) recordPagingUtilizationDataPoints(now pcommon.Timestamp, pageFiles []*pageFileStats) {
	for _, pageFile := range pageFiles {
		s.mb.RecordSystemPagingUtilizationDataPoint(now, float64(pageFile.usedBytes)/float64(pageFile.totalBytes), pageFile.deviceName, metadata.AttributeStateUsed)
		s.mb.RecordSystemPagingUtilizationDataPoint(now, float64(pageFile.freeBytes)/float64(pageFile.totalBytes), pageFile.deviceName, metadata.AttributeStateFree)
	}
}

func (s *scraper) scrapePagingOperationsMetric() error {
	now := pcommon.NewTimestampFromTime(time.Now())

	counters, err := s.perfCounterScraper.Scrape()
	if err != nil {
		return err
	}

	memoryObject, err := counters.GetObject(memory)
	if err != nil {
		return err
	}

	memoryCounterValues, err := memoryObject.GetValues(pageReadsPerSec, pageWritesPerSec)
	if err != nil {
		return err
	}

	if len(memoryCounterValues) > 0 {
		s.recordPagingOperationsDataPoints(now, memoryCounterValues[0])
	}
	return nil
}

func (s *scraper) recordPagingOperationsDataPoints(now pcommon.Timestamp, memoryCounterValues *perfcounters.CounterValues) {
	s.mb.RecordSystemPagingOperationsDataPoint(now, memoryCounterValues.Values[pageReadsPerSec], metadata.AttributeDirectionPageIn, metadata.AttributeTypeMajor)
	s.mb.RecordSystemPagingOperationsDataPoint(now, memoryCounterValues.Values[pageWritesPerSec], metadata.AttributeDirectionPageOut, metadata.AttributeTypeMajor)
}
