// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package diskscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/diskscraper"

import (
	"context"
	"fmt"
	"time"

	"github.com/shirou/gopsutil/v3/host"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scrapererror"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/perfcounters"
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

// scraper for Disk Metrics
type scraper struct {
	settings  receiver.CreateSettings
	config    *Config
	startTime pcommon.Timestamp
	mb        *metadata.MetricsBuilder
	includeFS filterset.FilterSet
	excludeFS filterset.FilterSet

	perfCounterScraper perfcounters.PerfCounterScraper
	skipScrape         bool

	// for mocking
	bootTime func() (uint64, error)
}

// newDiskScraper creates a Disk Scraper
func newDiskScraper(_ context.Context, settings receiver.CreateSettings, cfg *Config) (*scraper, error) {
	scraper := &scraper{settings: settings, config: cfg, perfCounterScraper: &perfcounters.PerfLibScraper{}, bootTime: host.BootTime}

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

func (s *scraper) start(context.Context, component.Host) error {
	bootTime, err := s.bootTime()
	if err != nil {
		return err
	}

	s.startTime = pcommon.Timestamp(bootTime * 1e9)
	s.mb = metadata.NewMetricsBuilder(s.config.MetricsBuilderConfig, s.settings, metadata.WithStartTime(s.startTime))

	if err = s.perfCounterScraper.Initialize(logicalDisk); err != nil {
		s.settings.Logger.Error("Failed to initialize performance counter, disk metrics will not be scraped", zap.Error(err))
		s.skipScrape = true
	}

	return nil
}

func (s *scraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	if s.skipScrape {
		return pmetric.NewMetrics(), nil
	}

	now := pcommon.NewTimestampFromTime(time.Now())

	counters, err := s.perfCounterScraper.Scrape()
	if err != nil {
		return pmetric.NewMetrics(), scrapererror.NewPartialScrapeError(err, metricsLen)
	}

	logicalDiskObject, err := counters.GetObject(logicalDisk)
	if err != nil {
		return pmetric.NewMetrics(), scrapererror.NewPartialScrapeError(err, metricsLen)
	}

	// filter devices by name
	logicalDiskObject.Filter(s.includeFS, s.excludeFS, false)

	logicalDiskCounterValues, err := logicalDiskObject.GetValues(readsPerSec, writesPerSec, readBytesPerSec, writeBytesPerSec, idleTime, avgDiskSecsPerRead, avgDiskSecsPerWrite, queueLength)
	if err != nil {
		return pmetric.NewMetrics(), scrapererror.NewPartialScrapeError(err, metricsLen)
	}

	if len(logicalDiskCounterValues) > 0 {
		s.recordDiskIOMetric(now, logicalDiskCounterValues)
		s.recordDiskOperationsMetric(now, logicalDiskCounterValues)
		s.recordDiskIOTimeMetric(now, logicalDiskCounterValues)
		s.recordDiskOperationTimeMetric(now, logicalDiskCounterValues)
		s.recordDiskPendingOperationsMetric(now, logicalDiskCounterValues)
	}

	return s.mb.Emit(), nil
}

func (s *scraper) recordDiskIOMetric(now pcommon.Timestamp, logicalDiskCounterValues []*perfcounters.CounterValues) {
	for _, logicalDiskCounter := range logicalDiskCounterValues {
		s.mb.RecordSystemDiskIoDataPoint(now, logicalDiskCounter.Values[readBytesPerSec], logicalDiskCounter.InstanceName, metadata.AttributeDirectionRead)
		s.mb.RecordSystemDiskIoDataPoint(now, logicalDiskCounter.Values[writeBytesPerSec], logicalDiskCounter.InstanceName, metadata.AttributeDirectionWrite)
	}
}

func (s *scraper) recordDiskOperationsMetric(now pcommon.Timestamp, logicalDiskCounterValues []*perfcounters.CounterValues) {
	for _, logicalDiskCounter := range logicalDiskCounterValues {
		s.mb.RecordSystemDiskOperationsDataPoint(now, logicalDiskCounter.Values[readsPerSec], logicalDiskCounter.InstanceName, metadata.AttributeDirectionRead)
		s.mb.RecordSystemDiskOperationsDataPoint(now, logicalDiskCounter.Values[writesPerSec], logicalDiskCounter.InstanceName, metadata.AttributeDirectionWrite)
	}
}

func (s *scraper) recordDiskIOTimeMetric(now pcommon.Timestamp, logicalDiskCounterValues []*perfcounters.CounterValues) {
	for _, logicalDiskCounter := range logicalDiskCounterValues {
		// disk active time = system boot time - disk idle time
		s.mb.RecordSystemDiskIoTimeDataPoint(now, float64(now-s.startTime)/1e9-float64(logicalDiskCounter.Values[idleTime])/1e7, logicalDiskCounter.InstanceName)
	}
}

func (s *scraper) recordDiskOperationTimeMetric(now pcommon.Timestamp, logicalDiskCounterValues []*perfcounters.CounterValues) {
	for _, logicalDiskCounter := range logicalDiskCounterValues {
		s.mb.RecordSystemDiskOperationTimeDataPoint(now, float64(logicalDiskCounter.Values[avgDiskSecsPerRead])/1e7, logicalDiskCounter.InstanceName, metadata.AttributeDirectionRead)
		s.mb.RecordSystemDiskOperationTimeDataPoint(now, float64(logicalDiskCounter.Values[avgDiskSecsPerWrite])/1e7, logicalDiskCounter.InstanceName, metadata.AttributeDirectionWrite)
	}
}

func (s *scraper) recordDiskPendingOperationsMetric(now pcommon.Timestamp, logicalDiskCounterValues []*perfcounters.CounterValues) {
	for _, logicalDiskCounter := range logicalDiskCounterValues {
		s.mb.RecordSystemDiskPendingOperationsDataPoint(now, logicalDiskCounter.Values[queueLength], logicalDiskCounter.InstanceName)
	}
}
