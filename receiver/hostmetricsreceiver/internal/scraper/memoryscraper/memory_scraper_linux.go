// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package memoryscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/memoryscraper"

import (
	"github.com/shirou/gopsutil/v4/mem"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/memoryscraper/internal/metadata"
)

func (s *memoryScraper) recordMemoryUsageMetric(now pcommon.Timestamp, memInfo *mem.VirtualMemoryStat) {
	s.mb.RecordSystemMemoryUsageDataPoint(now, int64(memInfo.Used), metadata.AttributeStateUsed)
	s.mb.RecordSystemMemoryUsageDataPoint(now, int64(memInfo.Free), metadata.AttributeStateFree)
	s.mb.RecordSystemMemoryUsageDataPoint(now, int64(memInfo.Buffers), metadata.AttributeStateBuffered)
	s.mb.RecordSystemMemoryUsageDataPoint(now, int64(memInfo.Cached), metadata.AttributeStateCached)
	s.mb.RecordSystemMemoryUsageDataPoint(now, int64(memInfo.Sreclaimable), metadata.AttributeStateSlabReclaimable)
	s.mb.RecordSystemMemoryUsageDataPoint(now, int64(memInfo.Sunreclaim), metadata.AttributeStateSlabUnreclaimable)
}

func (s *memoryScraper) recordMemoryUtilizationMetric(now pcommon.Timestamp, memInfo *mem.VirtualMemoryStat) {
	s.mb.RecordSystemMemoryUtilizationDataPoint(now, float64(memInfo.Used)/float64(memInfo.Total), metadata.AttributeStateUsed)
	s.mb.RecordSystemMemoryUtilizationDataPoint(now, float64(memInfo.Free)/float64(memInfo.Total), metadata.AttributeStateFree)
	s.mb.RecordSystemMemoryUtilizationDataPoint(now, float64(memInfo.Buffers)/float64(memInfo.Total), metadata.AttributeStateBuffered)
	s.mb.RecordSystemMemoryUtilizationDataPoint(now, float64(memInfo.Cached)/float64(memInfo.Total), metadata.AttributeStateCached)
	s.mb.RecordSystemMemoryUtilizationDataPoint(now, float64(memInfo.Sreclaimable)/float64(memInfo.Total), metadata.AttributeStateSlabReclaimable)
	s.mb.RecordSystemMemoryUtilizationDataPoint(now, float64(memInfo.Sunreclaim)/float64(memInfo.Total), metadata.AttributeStateSlabUnreclaimable)
}

func (s *memoryScraper) recordLinuxMemoryAvailableMetric(now pcommon.Timestamp, memInfo *mem.VirtualMemoryStat) {
	s.mb.RecordSystemLinuxMemoryAvailableDataPoint(now, int64(memInfo.Available))
}

func (s *memoryScraper) recordLinuxMemoryDirtyMetric(now pcommon.Timestamp, memInfo *mem.VirtualMemoryStat) {
	// This value is collected from /proc/meminfo and converted from kB to bytes in gopsutil:
	// https://github.com/shirou/gopsutil/blob/d8750909ba41f2de9750c90a6d2074c68dfc677e/mem/mem_linux.go#L148
	s.mb.RecordSystemLinuxMemoryDirtyDataPoint(now, int64(memInfo.Dirty))
}

func (s *memoryScraper) recordSystemSpecificMetrics(now pcommon.Timestamp, memInfo *mem.VirtualMemoryStat) {
	s.recordLinuxMemoryAvailableMetric(now, memInfo)
	s.recordLinuxMemoryDirtyMetric(now, memInfo)
}
