// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package diskscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/diskscraper"

import (
	"context"
	"fmt"
	"time"

	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/scraper"
	"go.opentelemetry.io/collector/scraper/scrapererror"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/diskscraper/internal/metadata"
)

const (
	standardMetricsLen = 5
	metricsLen         = standardMetricsLen + systemSpecificMetricsLen
)

// scraper for Disk Metrics
type diskScraper struct {
	settings  scraper.Settings
	config    *Config
	startTime pcommon.Timestamp
	mb        *metadata.MetricsBuilder
	includeFS filterset.FilterSet
	excludeFS filterset.FilterSet

	// for mocking
	bootTime   func(context.Context) (uint64, error)
	ioCounters func(ctx context.Context, names ...string) (map[string]disk.IOCountersStat, error)
}

// newDiskScraper creates a Disk Scraper
func newDiskScraper(_ context.Context, settings scraper.Settings, cfg *Config) (*diskScraper, error) {
	scraper := &diskScraper{settings: settings, config: cfg, bootTime: host.BootTimeWithContext, ioCounters: disk.IOCountersWithContext}

	var err error

	if len(cfg.Include.Devices) > 0 {
		scraper.includeFS, err = filterset.CreateFilterSet(cfg.Include.Devices, &cfg.Include.Config)
		if err != nil {
			return nil, fmt.Errorf("error creating device include filters: %w", err)
		}
	}

	if len(cfg.Exclude.Devices) > 0 {
		scraper.excludeFS, err = filterset.CreateFilterSet(cfg.Exclude.Devices, &cfg.Exclude.Config)
		if err != nil {
			return nil, fmt.Errorf("error creating device exclude filters: %w", err)
		}
	}

	return scraper, nil
}

func (s *diskScraper) start(ctx context.Context, _ component.Host) error {
	bootTime, err := s.bootTime(ctx)
	if err != nil {
		return err
	}

	s.startTime = pcommon.Timestamp(bootTime * 1e9)
	s.mb = metadata.NewMetricsBuilder(s.config.MetricsBuilderConfig, s.settings, metadata.WithStartTime(s.startTime))
	return nil
}

func (s *diskScraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	now := pcommon.NewTimestampFromTime(time.Now())
	ioCounters, err := s.ioCounters(ctx)
	if err != nil {
		return pmetric.NewMetrics(), scrapererror.NewPartialScrapeError(err, metricsLen)
	}

	// filter devices by name
	ioCounters = s.filterByDevice(ioCounters)

	if len(ioCounters) > 0 {
		s.recordDiskIOMetric(now, ioCounters)
		s.recordDiskOperationsMetric(now, ioCounters)
		s.recordDiskIOTimeMetric(now, ioCounters)
		s.recordDiskOperationTimeMetric(now, ioCounters)
		s.recordDiskPendingOperationsMetric(now, ioCounters)
		s.recordSystemSpecificDataPoints(now, ioCounters)
	}

	return s.mb.Emit(), nil
}

func (s *diskScraper) recordDiskIOMetric(now pcommon.Timestamp, ioCounters map[string]disk.IOCountersStat) {
	for device, ioCounter := range ioCounters {
		s.mb.RecordSystemDiskIoDataPoint(now, int64(ioCounter.ReadBytes), device, metadata.AttributeDirectionRead)
		s.mb.RecordSystemDiskIoDataPoint(now, int64(ioCounter.WriteBytes), device, metadata.AttributeDirectionWrite)
	}
}

func (s *diskScraper) recordDiskOperationsMetric(now pcommon.Timestamp, ioCounters map[string]disk.IOCountersStat) {
	for device, ioCounter := range ioCounters {
		s.mb.RecordSystemDiskOperationsDataPoint(now, int64(ioCounter.ReadCount), device, metadata.AttributeDirectionRead)
		s.mb.RecordSystemDiskOperationsDataPoint(now, int64(ioCounter.WriteCount), device, metadata.AttributeDirectionWrite)
	}
}

func (s *diskScraper) recordDiskIOTimeMetric(now pcommon.Timestamp, ioCounters map[string]disk.IOCountersStat) {
	for device, ioCounter := range ioCounters {
		s.mb.RecordSystemDiskIoTimeDataPoint(now, float64(ioCounter.IoTime)/1e3, device)
	}
}

func (s *diskScraper) recordDiskOperationTimeMetric(now pcommon.Timestamp, ioCounters map[string]disk.IOCountersStat) {
	for device, ioCounter := range ioCounters {
		s.mb.RecordSystemDiskOperationTimeDataPoint(now, float64(ioCounter.ReadTime)/1e3, device, metadata.AttributeDirectionRead)
		s.mb.RecordSystemDiskOperationTimeDataPoint(now, float64(ioCounter.WriteTime)/1e3, device, metadata.AttributeDirectionWrite)
	}
}

func (s *diskScraper) recordDiskPendingOperationsMetric(now pcommon.Timestamp, ioCounters map[string]disk.IOCountersStat) {
	for device, ioCounter := range ioCounters {
		s.mb.RecordSystemDiskPendingOperationsDataPoint(now, int64(ioCounter.IopsInProgress), device)
	}
}

func (s *diskScraper) filterByDevice(ioCounters map[string]disk.IOCountersStat) map[string]disk.IOCountersStat {
	if s.includeFS == nil && s.excludeFS == nil {
		return ioCounters
	}

	for device := range ioCounters {
		if !s.includeDevice(device) {
			delete(ioCounters, device)
		}
	}
	return ioCounters
}

func (s *diskScraper) includeDevice(deviceName string) bool {
	return (s.includeFS == nil || s.includeFS.Matches(deviceName)) &&
		(s.excludeFS == nil || !s.excludeFS.Matches(deviceName))
}
