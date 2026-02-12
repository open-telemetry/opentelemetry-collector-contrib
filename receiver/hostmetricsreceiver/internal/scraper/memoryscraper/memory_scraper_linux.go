// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package memoryscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/memoryscraper"

import (
	"github.com/shirou/gopsutil/v4/mem"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/memoryscraper/internal/metadata"
)

var useMemAvailable = featuregate.GlobalRegistry().MustRegister(
	"receiver.hostmetricsreceiver.UseLinuxMemAvailable",
	featuregate.StageBeta,
	featuregate.WithRegisterFromVersion("v0.137.0"),
	featuregate.WithRegisterDescription("When enabled, the used value for the system.memory.usage and system.memory.utilization metrics will be based on the Linux kernel’s MemAvailable statistic instead of MemFree, Buffers, and Cached."),
	featuregate.WithRegisterReferenceURL("https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/42221"),
)

func (s *memoryScraper) recordMemoryUsageMetric(now pcommon.Timestamp, memInfo *mem.VirtualMemoryStat) {
	// TODO: rely on memInfo.Used value once https://github.com/shirou/gopsutil/pull/1882 is released
	// gopsutil formula: https://github.com/shirou/gopsutil/pull/1882/files#diff-5af8322731595fb792b48f3c38f31ddb24f596cf11a74a9c37b19734597baef6R321
	if useMemAvailable.IsEnabled() {
		s.mb.RecordSystemMemoryUsageDataPoint(now, int64(memInfo.Total-memInfo.Available), metadata.AttributeStateUsed)
	} else {
		// gopsutil legacy "Used" memory formula = Total - Free - Buffers - Cache
		s.mb.RecordSystemMemoryUsageDataPoint(now, int64(memInfo.Total-memInfo.Free-memInfo.Buffers-memInfo.Cached), metadata.AttributeStateUsed)
	}
	s.mb.RecordSystemMemoryUsageDataPoint(now, int64(memInfo.Free), metadata.AttributeStateFree)
	s.mb.RecordSystemMemoryUsageDataPoint(now, int64(memInfo.Buffers), metadata.AttributeStateBuffered)
	s.mb.RecordSystemMemoryUsageDataPoint(now, int64(memInfo.Cached), metadata.AttributeStateCached)
	s.mb.RecordSystemMemoryUsageDataPoint(now, int64(memInfo.Sreclaimable), metadata.AttributeStateSlabReclaimable)
	s.mb.RecordSystemMemoryUsageDataPoint(now, int64(memInfo.Sunreclaim), metadata.AttributeStateSlabUnreclaimable)
}

func (s *memoryScraper) recordMemoryLinuxSharedMetric(now pcommon.Timestamp, memInfo *mem.VirtualMemoryStat) {
	s.mb.RecordSystemMemoryLinuxSharedDataPoint(now, int64(memInfo.Shared))
}

func (s *memoryScraper) recordMemoryUtilizationMetric(now pcommon.Timestamp, memInfo *mem.VirtualMemoryStat) {
	// TODO: rely on memInfo.Used value once https://github.com/shirou/gopsutil/pull/1882 is released
	// gopsutil formula: https://github.com/shirou/gopsutil/pull/1882/files#diff-5af8322731595fb792b48f3c38f31ddb24f596cf11a74a9c37b19734597baef6R321
	if useMemAvailable.IsEnabled() {
		s.mb.RecordSystemMemoryUtilizationDataPoint(now, float64(memInfo.Total-memInfo.Available)/float64(memInfo.Total), metadata.AttributeStateUsed)
	} else {
		// gopsutil legacy "Used" memory formula = Total - Free - Buffers - Cache
		s.mb.RecordSystemMemoryUtilizationDataPoint(now, float64(memInfo.Total-memInfo.Free-memInfo.Buffers-memInfo.Cached)/float64(memInfo.Total), metadata.AttributeStateUsed)
	}
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

func (s *memoryScraper) recordLinuxHugePagesMetrics(now pcommon.Timestamp, memInfo *mem.VirtualMemoryStat) {
	// Record page size in bytes (converted from kB to bytes in gopsutil):
	// https://github.com/shirou/gopsutil/blob/v4.25.12/mem/mem_linux.go#L303
	s.mb.RecordSystemMemoryLinuxHugepagesPageSizeDataPoint(now, int64(memInfo.HugePageSize))

	// Record limit (total hugepages available) in number of pages
	s.mb.RecordSystemMemoryLinuxHugepagesLimitDataPoint(now, int64(memInfo.HugePagesTotal))

	// Calculate used pages
	hugePagesUsed := int64(memInfo.HugePagesTotal - memInfo.HugePagesFree)

	// Record usage with state attributes in number of pages
	s.mb.RecordSystemMemoryLinuxHugepagesUsageDataPoint(now, int64(memInfo.HugePagesFree), metadata.AttributeSystemMemoryLinuxHugepagesStateFree)
	s.mb.RecordSystemMemoryLinuxHugepagesUsageDataPoint(now, hugePagesUsed, metadata.AttributeSystemMemoryLinuxHugepagesStateUsed)

	// Record reserved as a separate metric (not a state, since reserved pages are included in free_huge_pages
	// but cannot be used for non-reserved allocations)
	s.mb.RecordSystemMemoryLinuxHugepagesReservedDataPoint(now, int64(memInfo.HugePagesRsvd))

	// Record surplus as a separate metric (not a state, since surplus pages can also be in used/free states)
	s.mb.RecordSystemMemoryLinuxHugepagesSurplusDataPoint(now, int64(memInfo.HugePagesSurp))

	// Record utilization with state attributes as percentage
	if memInfo.HugePagesTotal != 0 {
		s.mb.RecordSystemMemoryLinuxHugepagesUtilizationDataPoint(now, float64(memInfo.HugePagesFree)/float64(memInfo.HugePagesTotal), metadata.AttributeSystemMemoryLinuxHugepagesStateFree)
		s.mb.RecordSystemMemoryLinuxHugepagesUtilizationDataPoint(now, float64(hugePagesUsed)/float64(memInfo.HugePagesTotal), metadata.AttributeSystemMemoryLinuxHugepagesStateUsed)
	}
}

func (s *memoryScraper) recordSystemSpecificMetrics(now pcommon.Timestamp, memInfo *mem.VirtualMemoryStat) {
	s.recordLinuxMemoryAvailableMetric(now, memInfo)
	s.recordLinuxMemoryDirtyMetric(now, memInfo)
	s.recordMemoryLinuxSharedMetric(now, memInfo)
	s.recordLinuxHugePagesMetrics(now, memInfo)
}
