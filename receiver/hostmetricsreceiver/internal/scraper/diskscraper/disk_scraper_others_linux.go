// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package diskscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/diskscraper"

import (
	"github.com/shirou/gopsutil/v4/disk"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/diskscraper/internal/metadata"
)

const systemSpecificMetricsLen = 2

func (s *diskScraper) recordSystemSpecificDataPoints(now pcommon.Timestamp, ioCounters map[string]disk.IOCountersStat) {
	s.recordDiskWeightedIOTimeMetric(now, ioCounters)
	s.recordDiskMergedMetric(now, ioCounters)
}

func (s *diskScraper) recordDiskWeightedIOTimeMetric(now pcommon.Timestamp, ioCounters map[string]disk.IOCountersStat) {
	for device, ioCounter := range ioCounters {
		s.mb.RecordSystemDiskWeightedIoTimeDataPoint(now, float64(ioCounter.WeightedIO)/1e3, device)
	}
}

func (s *diskScraper) recordDiskMergedMetric(now pcommon.Timestamp, ioCounters map[string]disk.IOCountersStat) {
	for device, ioCounter := range ioCounters {
		s.mb.RecordSystemDiskMergedDataPoint(now, int64(ioCounter.MergedReadCount), device, metadata.AttributeDirectionRead)
		s.mb.RecordSystemDiskMergedDataPoint(now, int64(ioCounter.MergedWriteCount), device, metadata.AttributeDirectionWrite)
	}
}
