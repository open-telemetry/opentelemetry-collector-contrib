// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package pagingscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/pagingscraper"

import (
	"context"
	"fmt"
	"time"

	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/mem"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/scraper"
	"go.opentelemetry.io/collector/scraper/scrapererror"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/pagingscraper/internal/metadata"
)

const (
	pagingUsageMetricsLen = 2
	pagingMetricsLen      = 2
)

// scraper for Paging Metrics
type pagingScraper struct {
	settings scraper.Settings
	config   *Config
	mb       *metadata.MetricsBuilder

	// for mocking
	bootTime         func(context.Context) (uint64, error)
	getPageFileStats func() ([]*pageFileStats, error)
	swapMemory       func(context.Context) (*mem.SwapMemoryStat, error)
}

// newPagingScraper creates a Paging Scraper
func newPagingScraper(_ context.Context, settings scraper.Settings, cfg *Config) *pagingScraper {
	return &pagingScraper{
		settings:         settings,
		config:           cfg,
		bootTime:         host.BootTimeWithContext,
		getPageFileStats: getPageFileStats,
		swapMemory:       mem.SwapMemoryWithContext,
	}
}

func (s *pagingScraper) start(ctx context.Context, _ component.Host) error {
	bootTime, err := s.bootTime(ctx)
	if err != nil {
		return err
	}

	s.mb = metadata.NewMetricsBuilder(s.config.MetricsBuilderConfig, s.settings, metadata.WithStartTime(pcommon.Timestamp(bootTime*1e9)))
	return nil
}

func (s *pagingScraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	var errors scrapererror.ScrapeErrors

	err := s.scrapePagingUsageMetric()
	if err != nil {
		errors.AddPartial(pagingUsageMetricsLen, err)
	}

	err = s.scrapePagingMetrics(ctx)
	if err != nil {
		errors.AddPartial(pagingMetricsLen, err)
	}

	return s.mb.Emit(), errors.Combine()
}

func (s *pagingScraper) scrapePagingUsageMetric() error {
	now := pcommon.NewTimestampFromTime(time.Now())
	pageFileStats, err := s.getPageFileStats()
	if err != nil {
		return fmt.Errorf("failed to read page file stats: %w", err)
	}

	s.recordPagingUsageDataPoints(now, pageFileStats)
	s.recordPagingUtilizationDataPoints(now, pageFileStats)
	return nil
}

func (s *pagingScraper) recordPagingUsageDataPoints(now pcommon.Timestamp, pageFileStats []*pageFileStats) {
	for _, pageFile := range pageFileStats {
		s.mb.RecordSystemPagingUsageDataPoint(now, int64(pageFile.usedBytes), pageFile.deviceName, metadata.AttributeStateUsed)
		s.mb.RecordSystemPagingUsageDataPoint(now, int64(pageFile.freeBytes), pageFile.deviceName, metadata.AttributeStateFree)
		if pageFile.cachedBytes != nil {
			s.mb.RecordSystemPagingUsageDataPoint(now, int64(*pageFile.cachedBytes), pageFile.deviceName, metadata.AttributeStateCached)
		}
	}
}

func (s *pagingScraper) recordPagingUtilizationDataPoints(now pcommon.Timestamp, pageFileStats []*pageFileStats) {
	for _, pageFile := range pageFileStats {
		s.mb.RecordSystemPagingUtilizationDataPoint(now, float64(pageFile.usedBytes)/float64(pageFile.totalBytes), pageFile.deviceName, metadata.AttributeStateUsed)
		s.mb.RecordSystemPagingUtilizationDataPoint(now, float64(pageFile.freeBytes)/float64(pageFile.totalBytes), pageFile.deviceName, metadata.AttributeStateFree)
		if pageFile.cachedBytes != nil {
			s.mb.RecordSystemPagingUtilizationDataPoint(now, float64(*pageFile.cachedBytes)/float64(pageFile.totalBytes), pageFile.deviceName, metadata.AttributeStateCached)
		}
	}
}

func (s *pagingScraper) scrapePagingMetrics(ctx context.Context) error {
	now := pcommon.NewTimestampFromTime(time.Now())
	swap, err := s.swapMemory(ctx)
	if err != nil {
		return fmt.Errorf("failed to read swap info: %w", err)
	}

	s.recordPagingOperationsDataPoints(now, swap)
	s.recordPageFaultsDataPoints(now, swap)
	return nil
}

func (s *pagingScraper) recordPagingOperationsDataPoints(now pcommon.Timestamp, swap *mem.SwapMemoryStat) {
	s.mb.RecordSystemPagingOperationsDataPoint(now, int64(swap.Sin), metadata.AttributeDirectionPageIn, metadata.AttributeTypeMajor)
	s.mb.RecordSystemPagingOperationsDataPoint(now, int64(swap.Sout), metadata.AttributeDirectionPageOut, metadata.AttributeTypeMajor)
	s.mb.RecordSystemPagingOperationsDataPoint(now, int64(swap.PgIn), metadata.AttributeDirectionPageIn, metadata.AttributeTypeMinor)
	s.mb.RecordSystemPagingOperationsDataPoint(now, int64(swap.PgOut), metadata.AttributeDirectionPageOut, metadata.AttributeTypeMinor)
}

func (s *pagingScraper) recordPageFaultsDataPoints(now pcommon.Timestamp, swap *mem.SwapMemoryStat) {
	s.mb.RecordSystemPagingFaultsDataPoint(now, int64(swap.PgMajFault), metadata.AttributeTypeMajor)
	s.mb.RecordSystemPagingFaultsDataPoint(now, int64(swap.PgFault-swap.PgMajFault), metadata.AttributeTypeMinor)
}
