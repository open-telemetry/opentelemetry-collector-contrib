// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package memoryscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/memoryscraper"

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/mem"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/scraper"
	"go.opentelemetry.io/collector/scraper/scrapererror"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/memoryscraper/internal/metadata"
)

const metricsLen = 2

var ErrInvalidTotalMem = errors.New("invalid total memory")

// scraper for Memory Metrics
type memoryScraper struct {
	settings scraper.Settings
	config   *Config
	mb       *metadata.MetricsBuilder

	// for mocking gopsutil mem.VirtualMemory
	bootTime      func(context.Context) (uint64, error)
	virtualMemory func(context.Context) (*mem.VirtualMemoryStat, error)

	pageSize int64
}

// newMemoryScraper creates a Memory Scraper
func newMemoryScraper(_ context.Context, settings scraper.Settings, cfg *Config) *memoryScraper {
	return &memoryScraper{
		settings:      settings,
		config:        cfg,
		bootTime:      host.BootTimeWithContext,
		virtualMemory: mem.VirtualMemoryWithContext,
		pageSize:      int64(os.Getpagesize()),
	}
}

func (s *memoryScraper) start(ctx context.Context, _ component.Host) error {
	bootTime, err := s.bootTime(ctx)
	if err != nil {
		return err
	}

	s.mb = metadata.NewMetricsBuilder(s.config.MetricsBuilderConfig, s.settings, metadata.WithStartTime(pcommon.Timestamp(bootTime*1e9)))
	return nil
}

func (s *memoryScraper) recordMemoryLimitMetric(now pcommon.Timestamp, memInfo *mem.VirtualMemoryStat) {
	s.mb.RecordSystemMemoryLimitDataPoint(now, int64(memInfo.Total))
}

func (s *memoryScraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	now := pcommon.NewTimestampFromTime(time.Now())
	memInfo, err := s.virtualMemory(ctx)
	if err != nil {
		return pmetric.NewMetrics(), scrapererror.NewPartialScrapeError(err, metricsLen)
	}

	if memInfo != nil {
		s.recordMemoryUsageMetric(now, memInfo)
		if memInfo.Total <= 0 {
			return pmetric.NewMetrics(), scrapererror.NewPartialScrapeError(fmt.Errorf("%w: %d", ErrInvalidTotalMem,
				memInfo.Total), metricsLen)
		}
		s.recordMemoryUtilizationMetric(now, memInfo)
		s.recordMemoryLimitMetric(now, memInfo)
		s.recordSystemSpecificMetrics(now, memInfo)
		s.recordMemoryPageSizeMetric(now)
	}

	return s.mb.Emit(), nil
}

func (s *memoryScraper) recordMemoryPageSizeMetric(now pcommon.Timestamp) {
	s.mb.RecordSystemMemoryPageSizeDataPoint(now, s.pageSize)
}
