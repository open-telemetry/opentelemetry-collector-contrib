// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !windows
// +build !windows

package pagingscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/pagingscraper"

import (
	"context"
	"fmt"
	"time"

	"github.com/shirou/gopsutil/v3/common"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scrapererror"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/pagingscraper/internal/metadata"
)

const (
	pagingUsageMetricsLen = 2
	pagingMetricsLen      = 2
)

// scraper for Paging Metrics
type scraper struct {
	settings receiver.CreateSettings
	config   *Config
	mb       *metadata.MetricsBuilder
	envMap   common.EnvMap

	// for mocking
	bootTime         func(context.Context) (uint64, error)
	getPageFileStats func() ([]*pageFileStats, error)
	swapMemory       func(context.Context) (*mem.SwapMemoryStat, error)
}

// newPagingScraper creates a Paging Scraper
func newPagingScraper(_ context.Context, settings receiver.CreateSettings, cfg *Config, envMap common.EnvMap) *scraper {
	return &scraper{
		settings:         settings,
		config:           cfg,
		bootTime:         host.BootTimeWithContext,
		getPageFileStats: getPageFileStats,
		swapMemory:       mem.SwapMemoryWithContext,
		envMap:           envMap,
	}
}

func (s *scraper) start(ctx context.Context, _ component.Host) error {
	ctx = context.WithValue(ctx, common.EnvKey, s.envMap)
	bootTime, err := s.bootTime(ctx)
	if err != nil {
		return err
	}

	s.mb = metadata.NewMetricsBuilder(s.config.MetricsBuilderConfig, s.settings, metadata.WithStartTime(pcommon.Timestamp(bootTime*1e9)))
	return nil
}

func (s *scraper) scrape(_ context.Context) (pmetric.Metrics, error) {
	var errors scrapererror.ScrapeErrors

	err := s.scrapePagingUsageMetric()
	if err != nil {
		errors.AddPartial(pagingUsageMetricsLen, err)
	}

	err = s.scrapePagingMetrics()
	if err != nil {
		errors.AddPartial(pagingMetricsLen, err)
	}

	return s.mb.Emit(), errors.Combine()
}

func (s *scraper) scrapePagingUsageMetric() error {
	now := pcommon.NewTimestampFromTime(time.Now())
	pageFileStats, err := s.getPageFileStats()
	if err != nil {
		return fmt.Errorf("failed to read page file stats: %w", err)
	}

	s.recordPagingUsageDataPoints(now, pageFileStats)
	s.recordPagingUtilizationDataPoints(now, pageFileStats)
	return nil
}

func (s *scraper) recordPagingUsageDataPoints(now pcommon.Timestamp, pageFileStats []*pageFileStats) {
	for _, pageFile := range pageFileStats {
		s.mb.RecordSystemPagingUsageDataPoint(now, int64(pageFile.usedBytes), pageFile.deviceName, metadata.AttributeStateUsed)
		s.mb.RecordSystemPagingUsageDataPoint(now, int64(pageFile.freeBytes), pageFile.deviceName, metadata.AttributeStateFree)
		if pageFile.cachedBytes != nil {
			s.mb.RecordSystemPagingUsageDataPoint(now, int64(*pageFile.cachedBytes), pageFile.deviceName, metadata.AttributeStateCached)
		}
	}
}

func (s *scraper) recordPagingUtilizationDataPoints(now pcommon.Timestamp, pageFileStats []*pageFileStats) {
	for _, pageFile := range pageFileStats {
		s.mb.RecordSystemPagingUtilizationDataPoint(now, float64(pageFile.usedBytes)/float64(pageFile.totalBytes), pageFile.deviceName, metadata.AttributeStateUsed)
		s.mb.RecordSystemPagingUtilizationDataPoint(now, float64(pageFile.freeBytes)/float64(pageFile.totalBytes), pageFile.deviceName, metadata.AttributeStateFree)
		if pageFile.cachedBytes != nil {
			s.mb.RecordSystemPagingUtilizationDataPoint(now, float64(*pageFile.cachedBytes)/float64(pageFile.totalBytes), pageFile.deviceName, metadata.AttributeStateCached)
		}
	}
}

func (s *scraper) scrapePagingMetrics() error {
	ctx := context.WithValue(context.Background(), common.EnvKey, s.envMap)
	now := pcommon.NewTimestampFromTime(time.Now())
	swap, err := s.swapMemory(ctx)
	if err != nil {
		return fmt.Errorf("failed to read swap info: %w", err)
	}

	s.recordPagingOperationsDataPoints(now, swap)
	s.recordPageFaultsDataPoints(now, swap)
	return nil
}

func (s *scraper) recordPagingOperationsDataPoints(now pcommon.Timestamp, swap *mem.SwapMemoryStat) {
	s.mb.RecordSystemPagingOperationsDataPoint(now, int64(swap.Sin), metadata.AttributeDirectionPageIn, metadata.AttributeTypeMajor)
	s.mb.RecordSystemPagingOperationsDataPoint(now, int64(swap.Sout), metadata.AttributeDirectionPageOut, metadata.AttributeTypeMajor)
	s.mb.RecordSystemPagingOperationsDataPoint(now, int64(swap.PgIn), metadata.AttributeDirectionPageIn, metadata.AttributeTypeMinor)
	s.mb.RecordSystemPagingOperationsDataPoint(now, int64(swap.PgOut), metadata.AttributeDirectionPageOut, metadata.AttributeTypeMinor)
}

func (s *scraper) recordPageFaultsDataPoints(now pcommon.Timestamp, swap *mem.SwapMemoryStat) {
	s.mb.RecordSystemPagingFaultsDataPoint(now, int64(swap.PgMajFault), metadata.AttributeTypeMajor)
	s.mb.RecordSystemPagingFaultsDataPoint(now, int64(swap.PgFault-swap.PgMajFault), metadata.AttributeTypeMinor)
}
