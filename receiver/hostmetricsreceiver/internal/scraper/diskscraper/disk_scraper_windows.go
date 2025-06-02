// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package diskscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/diskscraper"

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

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/winperfcounters"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/diskscraper/internal/metadata"
)

const (
	metricsLen = 5

	logicalDisk = "LogicalDisk"

	readsPerSec  = "Disk Reads/sec"
	writesPerSec = "Disk Writes/sec"

	readBytesPerSec  = "Disk Read Bytes/sec"
	writeBytesPerSec = "Disk Write Bytes/sec"

	idleTime = "% Idle Time"

	avgDiskSecsPerRead  = "Avg. Disk sec/Read"
	avgDiskSecsPerWrite = "Avg. Disk sec/Write"

	queueLength = "Current Disk Queue Length"
)

var counterNames = []string{
	readBytesPerSec,
	writeBytesPerSec,
	readsPerSec,
	writesPerSec,
	idleTime,
	avgDiskSecsPerRead,
	avgDiskSecsPerWrite,
	queueLength,
}

// diskScraper for Disk Metrics
type diskScraper struct {
	settings  scraper.Settings
	config    *Config
	startTime pcommon.Timestamp
	mb        *metadata.MetricsBuilder
	includeFS filterset.FilterSet
	excludeFS filterset.FilterSet

	perfCounters []winperfcounters.PerfCounterWatcher
	skipScrape   bool

	// for mocking
	bootTime           func(ctx context.Context) (uint64, error)
	perfCounterFactory func(string, string, string) (winperfcounters.PerfCounterWatcher, error)
}

// newDiskScraper creates a Disk Scraper
func newDiskScraper(_ context.Context, settings scraper.Settings, cfg *Config) (*diskScraper, error) {
	scraper := &diskScraper{
		settings:           settings,
		config:             cfg,
		bootTime:           host.BootTimeWithContext,
		perfCounterFactory: winperfcounters.NewWatcher,
	}

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

	// Initialize the performance counter watchers
	s.perfCounters = make([]winperfcounters.PerfCounterWatcher, len(counterNames))
	for i, counterName := range counterNames {
		s.perfCounters[i], err = s.perfCounterFactory(logicalDisk, "*", counterName)
		if err != nil {
			s.skipScrape = true
			s.settings.Logger.Error(
				"Failed to create performance counter watcher, disk metrics will not be scraped",
				zap.String("counter", counterName),
				zap.Error(err))
		}
	}

	return nil
}

func (s *diskScraper) scrape(_ context.Context) (pmetric.Metrics, error) {
	if s.skipScrape {
		return pmetric.NewMetrics(), nil
	}

	instanceToRawCounters := make(map[string][]int64)
	now := pcommon.NewTimestampFromTime(time.Now())
	for i := range counterNames {
		counterValues, err := s.perfCounters[i].ScrapeRawValues()
		if err != nil {
			return pmetric.NewMetrics(), scrapererror.NewPartialScrapeError(err, metricsLen)
		}

		for _, counterValue := range counterValues {
			instanceName := counterValue.InstanceName
			if instanceValues, ok := instanceToRawCounters[instanceName]; ok {
				instanceValues[i] = counterValue.RawValue
				continue
			}

			if includeDevice(instanceName, s.includeFS, s.excludeFS) {
				instanceToRawCounters[instanceName] = make([]int64, len(counterNames))
				instanceToRawCounters[instanceName][i] = counterValue.RawValue
			}
		}
	}

	// For each counter and respective set of values record the metrics
	for instance, values := range instanceToRawCounters {
		for i := range counterNames {
			switch counterNames[i] {
			case readBytesPerSec:
				s.mb.RecordSystemDiskIoDataPoint(now, values[i], instance, metadata.AttributeDirectionRead)
			case writeBytesPerSec:
				s.mb.RecordSystemDiskIoDataPoint(now, values[i], instance, metadata.AttributeDirectionWrite)
			case readsPerSec:
				s.mb.RecordSystemDiskOperationsDataPoint(now, values[i], instance, metadata.AttributeDirectionRead)
			case writesPerSec:
				s.mb.RecordSystemDiskOperationsDataPoint(now, values[i], instance, metadata.AttributeDirectionWrite)
			case idleTime:
				s.mb.RecordSystemDiskIoTimeDataPoint(now, float64(now-s.startTime)/1e9-float64(values[i])/1e7, instance)
			case avgDiskSecsPerRead:
				s.mb.RecordSystemDiskOperationTimeDataPoint(now, float64(values[i])/1e7, instance, metadata.AttributeDirectionRead)
			case avgDiskSecsPerWrite:
				s.mb.RecordSystemDiskOperationTimeDataPoint(now, float64(values[i])/1e7, instance, metadata.AttributeDirectionWrite)
			case queueLength:
				s.mb.RecordSystemDiskPendingOperationsDataPoint(now, values[i], instance)
			default:
				s.settings.Logger.Error("Unknown counter name", zap.String("counter", counterNames[i]))
			}
		}
	}

	return s.mb.Emit(), nil
}

func includeDevice(deviceName string, includeFS, excludeFS filterset.FilterSet) bool {
	return (includeFS == nil || includeFS.Matches(deviceName)) &&
		(excludeFS == nil || !excludeFS.Matches(deviceName))
}
