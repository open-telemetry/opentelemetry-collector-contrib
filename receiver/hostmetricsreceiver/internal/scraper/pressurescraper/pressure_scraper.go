// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pressurescraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/pressurescraper"

import (
	"context"
	"errors"
	"time"

	"github.com/shirou/gopsutil/v4/host"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/scraper"
	"go.opentelemetry.io/collector/scraper/scrapererror"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/pressurescraper/internal/metadata"
)

const metricsLen = 2

var ErrInvalidTotalMem = errors.New("invalid total memory")

// scraper for Memory Metrics
type pressureScraper struct {
	settings scraper.Settings
	config   *Config
	mb       *metadata.MetricsBuilder

	bootTime         func(context.Context) (uint64, error)
	getPressureStats func(pressureResource) (*PressureResourceStats, error)
}

// newPressureScraper creates a Memory Scraper
func newPressureScraper(_ context.Context, settings scraper.Settings, cfg *Config) *pressureScraper {
	return &pressureScraper{
		settings:         settings,
		config:           cfg,
		bootTime:         host.BootTimeWithContext,
		getPressureStats: getPressureStats,
	}
}

func (s *pressureScraper) start(ctx context.Context, _ component.Host) error {
	bootTime, err := s.bootTime(ctx)
	if err != nil {
		return err
	}

	s.mb = metadata.NewMetricsBuilder(s.config.MetricsBuilderConfig, s.settings, metadata.WithStartTime(pcommon.Timestamp(bootTime*1e9)))
	return nil
}

func (s *pressureScraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	now := pcommon.NewTimestampFromTime(time.Now())
	cpuPressureInfo, err := s.getPressureStats(cpuResource)
	if err != nil {
		return pmetric.NewMetrics(), scrapererror.NewPartialScrapeError(err, metricsLen)
	}

	if cpuPressureInfo != nil {
		// some tasks
		s.mb.RecordSystemCPULinuxPressure10sDataPoint(now, cpuPressureInfo.some.avg10, metadata.AttributeSystemPressureStallTypeSome)
		s.mb.RecordSystemCPULinuxPressure1mDataPoint(now, cpuPressureInfo.some.avg60, metadata.AttributeSystemPressureStallTypeSome)
		s.mb.RecordSystemCPULinuxPressure5mDataPoint(now, cpuPressureInfo.some.avg300, metadata.AttributeSystemPressureStallTypeSome)
		s.mb.RecordSystemCPULinuxPressureTotalDataPoint(now, int64(cpuPressureInfo.some.total), metadata.AttributeSystemPressureStallTypeSome)

		// all tasks
		s.mb.RecordSystemCPULinuxPressure10sDataPoint(now, cpuPressureInfo.full.avg10, metadata.AttributeSystemPressureStallTypeFull)
		s.mb.RecordSystemCPULinuxPressure1mDataPoint(now, cpuPressureInfo.full.avg60, metadata.AttributeSystemPressureStallTypeFull)
		s.mb.RecordSystemCPULinuxPressure5mDataPoint(now, cpuPressureInfo.full.avg300, metadata.AttributeSystemPressureStallTypeFull)
		s.mb.RecordSystemCPULinuxPressureTotalDataPoint(now, int64(cpuPressureInfo.full.total), metadata.AttributeSystemPressureStallTypeFull)
	}

	memPressureInfo, err := s.getPressureStats(memResource)
	if err != nil {
		return pmetric.NewMetrics(), scrapererror.NewPartialScrapeError(err, metricsLen)
	}

	if memPressureInfo != nil {
		// some tasks
		s.mb.RecordSystemMemoryLinuxPressure10sDataPoint(now, memPressureInfo.some.avg10, metadata.AttributeSystemPressureStallTypeSome)
		s.mb.RecordSystemMemoryLinuxPressure1mDataPoint(now, memPressureInfo.some.avg60, metadata.AttributeSystemPressureStallTypeSome)
		s.mb.RecordSystemMemoryLinuxPressure5mDataPoint(now, memPressureInfo.some.avg300, metadata.AttributeSystemPressureStallTypeSome)
		s.mb.RecordSystemMemoryLinuxPressureTotalDataPoint(now, int64(memPressureInfo.some.total), metadata.AttributeSystemPressureStallTypeSome)

		// all tasks
		s.mb.RecordSystemMemoryLinuxPressure10sDataPoint(now, memPressureInfo.full.avg10, metadata.AttributeSystemPressureStallTypeFull)
		s.mb.RecordSystemMemoryLinuxPressure1mDataPoint(now, memPressureInfo.full.avg60, metadata.AttributeSystemPressureStallTypeFull)
		s.mb.RecordSystemMemoryLinuxPressure5mDataPoint(now, memPressureInfo.full.avg300, metadata.AttributeSystemPressureStallTypeFull)
		s.mb.RecordSystemMemoryLinuxPressureTotalDataPoint(now, int64(memPressureInfo.full.total), metadata.AttributeSystemPressureStallTypeFull)
	}

	return s.mb.Emit(), nil
}
