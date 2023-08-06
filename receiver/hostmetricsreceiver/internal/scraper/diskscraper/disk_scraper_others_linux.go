// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux
// +build linux

package diskscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/diskscraper"

import (
	"github.com/shirou/gopsutil/v3/disk"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/diskscraper/internal/metadata"
)

const systemSpecificMetricsLen = 2

func (s *scraper) recordSystemSpecificDataPoints(now pcommon.Timestamp, ioCounters map[string]disk.IOCountersStat) {
	s.recordDiskWeightedIOTimeMetric(now, ioCounters)
	s.recordDiskMergedMetric(now, ioCounters)
}

func (s *scraper) recordDiskWeightedIOTimeMetric(now pcommon.Timestamp, ioCounters map[string]disk.IOCountersStat) {
	rmb := s.mb.ResourceMetricsBuilder(pcommon.NewResource())
	for device, ioCounter := range ioCounters {
		rmb.RecordSystemDiskWeightedIoTimeDataPoint(now, float64(ioCounter.WeightedIO)/1e3, device)
	}
}

func (s *scraper) recordDiskMergedMetric(now pcommon.Timestamp, ioCounters map[string]disk.IOCountersStat) {
	rmb := s.mb.ResourceMetricsBuilder(pcommon.NewResource())
	for device, ioCounter := range ioCounters {
		rmb.RecordSystemDiskMergedDataPoint(now, int64(ioCounter.MergedReadCount), device, metadata.AttributeDirectionRead)
		rmb.RecordSystemDiskMergedDataPoint(now, int64(ioCounter.MergedWriteCount), device, metadata.AttributeDirectionWrite)
	}
}
