// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package vcenterreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/vcenterreceiver"

import (
	"context"

	"github.com/vmware/govmomi/performance"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/receiver/scrapererror"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/vcenterreceiver/internal/metadata"
)

func (v *vcenterMetricScraper) recordHostSystemMemoryUsage(
	now pcommon.Timestamp,
	hs mo.HostSystem,
) {
	s := hs.Summary
	h := s.Hardware
	z := s.QuickStats

	memUsage := z.OverallMemoryUsage
	v.mb.RecordVcenterHostMemoryUsageDataPoint(now, int64(memUsage))

	memUtilization := 100 * float64(z.OverallMemoryUsage) / float64(h.MemorySize>>20)
	v.mb.RecordVcenterHostMemoryUtilizationDataPoint(now, memUtilization)

	ncpu := int32(h.NumCpuCores)
	cpuUsage := z.OverallCpuUsage
	cpuUtilization := 100 * float64(z.OverallCpuUsage) / float64(ncpu*h.CpuMhz)

	v.mb.RecordVcenterHostCPUUsageDataPoint(now, int64(cpuUsage))
	v.mb.RecordVcenterHostCPUUtilizationDataPoint(now, cpuUtilization)
}

func (v *vcenterMetricScraper) recordVMUsages(
	now pcommon.Timestamp,
	vm mo.VirtualMachine,
	hs mo.HostSystem,
) {
	memUsage := vm.Summary.QuickStats.GuestMemoryUsage
	balloonedMem := vm.Summary.QuickStats.BalloonedMemory
	swappedMem := vm.Summary.QuickStats.SwappedMemory
	swappedSSDMem := vm.Summary.QuickStats.SsdSwappedMemory

	if totalMemory := vm.Summary.Config.MemorySizeMB; totalMemory > 0 && memUsage > 0 {
		memoryUtilization := float64(memUsage) / float64(totalMemory) * 100
		v.mb.RecordVcenterVMMemoryUtilizationDataPoint(now, memoryUtilization)
	}

	v.mb.RecordVcenterVMMemoryUsageDataPoint(now, int64(memUsage))
	v.mb.RecordVcenterVMMemoryBalloonedDataPoint(now, int64(balloonedMem))
	v.mb.RecordVcenterVMMemorySwappedDataPoint(now, int64(swappedMem))
	v.mb.RecordVcenterVMMemorySwappedSsdDataPoint(now, swappedSSDMem)

	diskUsed := vm.Summary.Storage.Committed
	diskFree := vm.Summary.Storage.Uncommitted

	v.mb.RecordVcenterVMDiskUsageDataPoint(now, diskUsed, metadata.AttributeDiskStateUsed)
	v.mb.RecordVcenterVMDiskUsageDataPoint(now, diskFree, metadata.AttributeDiskStateAvailable)
	if diskFree != 0 {
		diskUtilization := float64(diskUsed) / float64(diskFree+diskUsed) * 100
		v.mb.RecordVcenterVMDiskUtilizationDataPoint(now, diskUtilization)
	}

	s := vm.Summary
	z := s.QuickStats

	cpuUsage := z.OverallCpuUsage
	ncpu := vm.Config.Hardware.NumCPU

	// if no cpu usage, we probably shouldn't return a value. Most likely the VM is unavailable
	// or is unreachable.
	if cpuUsage == 0 {
		return
	}

	// https://communities.vmware.com/t5/VMware-code-Documents/Resource-Management/ta-p/2783456
	// VirtualMachine.runtime.maxCpuUsage is a property of the virtual machine, indicating the limit value.
	// This value is always equal to the limit value set for that virtual machine.
	// If no limit, it has full host mhz * vm.Config.Hardware.NumCPU.
	var cpuUtil float64
	if vm.Runtime.MaxCpuUsage != 0 {
		cpuUtil = 100 * float64(cpuUsage) / float64(vm.Runtime.MaxCpuUsage)
	} else {
		cpuUtil = 100 * float64(cpuUsage) / float64(ncpu*hs.Summary.Hardware.CpuMhz)
	}

	v.mb.RecordVcenterVMCPUUsageDataPoint(now, int64(cpuUsage))
	v.mb.RecordVcenterVMCPUUtilizationDataPoint(now, cpuUtil)

}

func (v *vcenterMetricScraper) recordDatastoreProperties(
	now pcommon.Timestamp,
	ds mo.Datastore,
) {
	s := ds.Summary
	diskUsage := s.Capacity - s.FreeSpace
	diskUtilization := float64(diskUsage) / float64(s.Capacity) * 100
	v.mb.RecordVcenterDatastoreDiskUsageDataPoint(now, diskUsage, metadata.AttributeDiskStateUsed)
	v.mb.RecordVcenterDatastoreDiskUsageDataPoint(now, s.Capacity, metadata.AttributeDiskStateUsed)
	v.mb.RecordVcenterDatastoreDiskUtilizationDataPoint(now, diskUtilization)
}

func (v *vcenterMetricScraper) recordResourcePool(
	now pcommon.Timestamp,
	rp mo.ResourcePool,
) {
	s := rp.Summary.GetResourcePoolSummary()
	if s.QuickStats != nil {
		v.mb.RecordVcenterResourcePoolCPUUsageDataPoint(now, s.QuickStats.OverallCpuUsage)
		v.mb.RecordVcenterResourcePoolMemoryUsageDataPoint(now, s.QuickStats.GuestMemoryUsage)
	}

	v.mb.RecordVcenterResourcePoolCPUSharesDataPoint(now, int64(s.Config.CpuAllocation.Shares.Shares))
	v.mb.RecordVcenterResourcePoolMemorySharesDataPoint(now, int64(s.Config.MemoryAllocation.Shares.Shares))

}

var hostPerfMetricList = []string{
	// network metrics
	"net.bytesTx.average",
	"net.bytesRx.average",
	"net.packetsTx.summation",
	"net.packetsRx.summation",
	"net.usage.average",
	"net.errorsRx.summation",
	"net.errorsTx.summation",

	// disk metrics
	"virtualDisk.totalWriteLatency.average",
	"disk.deviceReadLatency.average",
	"disk.deviceWriteLatency.average",
	"disk.kernelReadLatency.average",
	"disk.kernelWriteLatency.average",
	"disk.maxTotalLatency.latest",
	"disk.read.average",
	"disk.write.average",
}

func (v *vcenterMetricScraper) recordHostPerformanceMetrics(
	ctx context.Context,
	host mo.HostSystem,
	errs *scrapererror.ScrapeErrors,
) {
	spec := types.PerfQuerySpec{
		Entity:    host.Reference(),
		MaxSample: 5,
		Format:    string(types.PerfFormatNormal),
		MetricId:  []types.PerfMetricId{{Instance: "*"}},
		// right now we are only grabbing real time metrics from the performance
		// manager
		IntervalId: int32(20),
	}

	info, err := v.client.performanceQuery(ctx, spec, hostPerfMetricList, []types.ManagedObjectReference{host.Reference()})
	if err != nil {
		errs.AddPartial(1, err)
		return
	}
	v.processHostPerformance(info.results)
}

// vmPerfMetricList may be customizable in the future but here is the full list of Virtual Machine Performance Counters
// https://docs.vmware.com/en/vRealize-Operations/8.6/com.vmware.vcom.metrics.doc/GUID-1322F5A4-DA1D-481F-BBEA-99B228E96AF2.html
var vmPerfMetricList = []string{
	// network metrics
	"net.packetsTx.summation",
	"net.packetsRx.summation",
	"net.bytesRx.average",
	"net.bytesTx.average",
	"net.usage.average",

	// disk metrics
	"disk.totalWriteLatency.average",
	"disk.totalReadLatency.average",
	"disk.maxTotalLatency.latest",
	"virtualDisk.totalWriteLatency.average",
	"virtualDisk.totalReadLatency.average",
}

func (v *vcenterMetricScraper) recordVMPerformance(
	ctx context.Context,
	vm mo.VirtualMachine,
	errs *scrapererror.ScrapeErrors,
) {
	spec := types.PerfQuerySpec{
		Entity: vm.Reference(),
		Format: string(types.PerfFormatNormal),
		// Just grabbing real time performance metrics of the current
		// supported metrics by this receiver. If more are added we may need
		// a system of making this user customizable or adapt to use a 5 minute interval per metric
		IntervalId: int32(20),
	}

	info, err := v.client.performanceQuery(ctx, spec, vmPerfMetricList, []types.ManagedObjectReference{vm.Reference()})
	if err != nil {
		errs.AddPartial(1, err)
		return
	}

	v.processVMPerformanceMetrics(info)
}

func (v *vcenterMetricScraper) processVMPerformanceMetrics(info *perfSampleResult) {
	for _, m := range info.results {
		for _, val := range m.Value {
			for j, nestedValue := range val.Value {
				si := m.SampleInfo[j]
				switch val.Name {
				// Performance monitoring level 1 metrics
				case "net.bytesTx.average":
					v.mb.RecordVcenterVMNetworkThroughputDataPoint(pcommon.NewTimestampFromTime(si.Timestamp), nestedValue, metadata.AttributeThroughputDirectionTransmitted)
				case "net.bytesRx.average":
					v.mb.RecordVcenterVMNetworkThroughputDataPoint(pcommon.NewTimestampFromTime(si.Timestamp), nestedValue, metadata.AttributeThroughputDirectionReceived)
				case "net.usage.average":
					v.mb.RecordVcenterVMNetworkUsageDataPoint(pcommon.NewTimestampFromTime(si.Timestamp), nestedValue)
				case "net.packetsTx.summation":
					v.mb.RecordVcenterVMNetworkPacketCountDataPoint(pcommon.NewTimestampFromTime(si.Timestamp), nestedValue, metadata.AttributeThroughputDirectionTransmitted)
				case "net.packetsRx.summation":
					v.mb.RecordVcenterVMNetworkPacketCountDataPoint(pcommon.NewTimestampFromTime(si.Timestamp), nestedValue, metadata.AttributeThroughputDirectionReceived)

				// Performance monitoring level 2 metrics required
				case "disk.totalReadLatency.average":
					v.mb.RecordVcenterVMDiskLatencyAvgDataPoint(pcommon.NewTimestampFromTime(si.Timestamp), nestedValue, metadata.AttributeDiskDirectionRead, metadata.AttributeDiskTypePhysical)
				case "virtualDisk.totalReadLatency.average":
					v.mb.RecordVcenterVMDiskLatencyAvgDataPoint(pcommon.NewTimestampFromTime(si.Timestamp), nestedValue, metadata.AttributeDiskDirectionRead, metadata.AttributeDiskTypeVirtual)
				case "disk.totalWriteLatency.average":
					v.mb.RecordVcenterVMDiskLatencyAvgDataPoint(pcommon.NewTimestampFromTime(si.Timestamp), nestedValue, metadata.AttributeDiskDirectionWrite, metadata.AttributeDiskTypePhysical)
				case "virtualDisk.totalWriteLatency.average":
					v.mb.RecordVcenterVMDiskLatencyAvgDataPoint(pcommon.NewTimestampFromTime(si.Timestamp), nestedValue, metadata.AttributeDiskDirectionWrite, metadata.AttributeDiskTypeVirtual)
				case "disk.maxTotalLatency.latest":
					v.mb.RecordVcenterVMDiskLatencyMaxDataPoint(pcommon.NewTimestampFromTime(si.Timestamp), nestedValue)
				}
			}
		}
	}
}

func (v *vcenterMetricScraper) processHostPerformance(metrics []performance.EntityMetric) {
	for _, m := range metrics {
		for _, val := range m.Value {
			for j, nestedValue := range val.Value {
				si := m.SampleInfo[j]
				switch val.Name {
				// Performance monitoring level 1 metrics
				case "net.usage.average":
					v.mb.RecordVcenterHostNetworkUsageDataPoint(pcommon.NewTimestampFromTime(si.Timestamp), nestedValue)
				case "net.bytesTx.average":
					v.mb.RecordVcenterHostNetworkThroughputDataPoint(pcommon.NewTimestampFromTime(si.Timestamp), nestedValue, metadata.AttributeThroughputDirectionTransmitted)
				case "net.bytesRx.average":
					v.mb.RecordVcenterHostNetworkThroughputDataPoint(pcommon.NewTimestampFromTime(si.Timestamp), nestedValue, metadata.AttributeThroughputDirectionReceived)
				case "net.packetsTx.summation":
					v.mb.RecordVcenterHostNetworkPacketCountDataPoint(pcommon.NewTimestampFromTime(si.Timestamp), nestedValue, metadata.AttributeThroughputDirectionTransmitted)
				case "net.packetsRx.summation":
					v.mb.RecordVcenterHostNetworkPacketCountDataPoint(pcommon.NewTimestampFromTime(si.Timestamp), nestedValue, metadata.AttributeThroughputDirectionReceived)

				// Following requires performance level 2
				case "net.errorsRx.summation":
					v.mb.RecordVcenterHostNetworkPacketErrorsDataPoint(pcommon.NewTimestampFromTime(si.Timestamp), nestedValue, metadata.AttributeThroughputDirectionReceived)
				case "net.errorsTx.summation":
					v.mb.RecordVcenterHostNetworkPacketErrorsDataPoint(pcommon.NewTimestampFromTime(si.Timestamp), nestedValue, metadata.AttributeThroughputDirectionTransmitted)
				case "disk.totalWriteLatency.average":
					v.mb.RecordVcenterHostDiskLatencyAvgDataPoint(pcommon.NewTimestampFromTime(si.Timestamp), nestedValue, metadata.AttributeDiskDirectionWrite)
				case "disk.totalReadLatency.average":
					v.mb.RecordVcenterHostDiskLatencyAvgDataPoint(pcommon.NewTimestampFromTime(si.Timestamp), nestedValue, metadata.AttributeDiskDirectionRead)
				case "disk.maxTotalLatency.latest":
					v.mb.RecordVcenterHostDiskLatencyMaxDataPoint(pcommon.NewTimestampFromTime(si.Timestamp), nestedValue)

				// Following requires performance level 4
				case "disk.read.average":
					v.mb.RecordVcenterHostDiskThroughputDataPoint(pcommon.NewTimestampFromTime(si.Timestamp), nestedValue, metadata.AttributeDiskDirectionRead)
				case "disk.write.average":
					v.mb.RecordVcenterHostDiskThroughputDataPoint(pcommon.NewTimestampFromTime(si.Timestamp), nestedValue, metadata.AttributeDiskDirectionWrite)
				}
			}
		}
	}
}
