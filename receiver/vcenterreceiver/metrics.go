// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package vcenterreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/vcenterreceiver"

import (
	"context"
	"time"

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

	v.mb.RecordVcenterHostMemoryUsageDataPoint(now, int64(z.OverallMemoryUsage))
	memUtilization := 100 * float64(z.OverallMemoryUsage) / float64(h.MemorySize>>20)
	v.mb.RecordVcenterHostMemoryUtilizationDataPoint(now, memUtilization)

	v.mb.RecordVcenterHostCPUUsageDataPoint(now, int64(z.OverallCpuUsage))
	cpuUtilization := 100 * float64(z.OverallCpuUsage) / float64(int32(h.NumCpuCores)*h.CpuMhz)
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

	cpuUsage := vm.Summary.QuickStats.OverallCpuUsage
	if cpuUsage == 0 {
		// Most likely the VM is unavailable or is unreachable.
		return
	}
	v.mb.RecordVcenterVMCPUUsageDataPoint(now, int64(cpuUsage))

	// https://communities.vmware.com/t5/VMware-code-Documents/Resource-Management/ta-p/2783456
	// VirtualMachine.runtime.maxCpuUsage is a property of the virtual machine, indicating the limit value.
	// This value is always equal to the limit value set for that virtual machine.
	// If no limit, it has full host mhz * vm.Config.Hardware.NumCPU.
	cpuLimit := vm.Config.Hardware.NumCPU * hs.Summary.Hardware.CpuMhz
	if vm.Runtime.MaxCpuUsage != 0 {
		cpuLimit = vm.Runtime.MaxCpuUsage
	}
	if cpuLimit == 0 {
		// This shouldn't happen, but protect against division by zero.
		return
	}
	v.mb.RecordVcenterVMCPUUtilizationDataPoint(now, 100*float64(cpuUsage)/float64(cpuLimit))
}

func (v *vcenterMetricScraper) recordDatastoreProperties(
	now pcommon.Timestamp,
	ds mo.Datastore,
) {
	s := ds.Summary
	diskUsage := s.Capacity - s.FreeSpace
	diskUtilization := float64(diskUsage) / float64(s.Capacity) * 100
	v.mb.RecordVcenterDatastoreDiskUsageDataPoint(now, diskUsage, metadata.AttributeDiskStateUsed)
	v.mb.RecordVcenterDatastoreDiskUsageDataPoint(now, s.FreeSpace, metadata.AttributeDiskStateAvailable)
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
	hri hostResourceInfo,
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
	v.processHostPerformance(info.results, hri)
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
	vri vmResourceInfo,
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

	v.processVMPerformanceMetrics(info, vri)
}

type perfMetric struct {
	timestamp time.Time
	value     int64
}

type vmResourceInfo struct {
	cluster string
	host    string
	vm      string
	vmID    string
}

type vmPerformanceMetrics struct {
	bytesTx                    *perfMetric
	bytesRx                    *perfMetric
	usage                      *perfMetric
	packetsTx                  *perfMetric
	packetsRx                  *perfMetric
	cpuUsagePercent            *perfMetric
	memoryUsagePercent         *perfMetric
	diskUsagePercent           *perfMetric
	networkUsagePercent        *perfMetric
	diskReadLatencyAvg         *perfMetric
	virtualDiskReadLatencyAvg  *perfMetric
	diskWriteLatencyAvg        *perfMetric
	virtualDiskWriteLatencyAvg *perfMetric
	diskMaxTotalLatency        *perfMetric
}

func (v *vcenterMetricScraper) processVMPerformanceMetrics(info *perfSampleResult, vri vmResourceInfo) {
	perfMap := map[string]vmPerformanceMetrics{}
	for _, m := range info.results {
		for _, val := range m.Value {
			// don't collect dimensionless data
			if val.Instance == "" {
				continue
			}
			for j, nestedValue := range val.Value {
				si := m.SampleInfo[j]
				pm := &perfMetric{
					timestamp: si.Timestamp,
					value:     int64(nestedValue),
				}
				switch val.Name {
				// Performance monitoring level 1 metrics
				case "net.bytesTx.average":
					if p, ok := perfMap[val.Instance]; ok {
						p.bytesTx = pm
					} else {
						perfMap[val.Instance] = vmPerformanceMetrics{bytesTx: pm}
					}
				case "net.bytesRx.average":
					if p, ok := perfMap[val.Instance]; ok {
						p.bytesRx = pm
					} else {
						perfMap[val.Instance] = vmPerformanceMetrics{bytesRx: pm}
					}
				case "net.usage.average":
					if p, ok := perfMap[val.Instance]; ok {
						p.usage = pm
					} else {
						perfMap[val.Instance] = vmPerformanceMetrics{usage: pm}
					}
				case "net.packetsTx.summation":
					if p, ok := perfMap[val.Instance]; ok {
						p.packetsTx = pm
					} else {
						perfMap[val.Instance] = vmPerformanceMetrics{packetsTx: pm}
					}
				case "net.packetsRx.summation":
					if p, ok := perfMap[val.Instance]; ok {
						p.packetsRx = pm
					} else {
						perfMap[val.Instance] = vmPerformanceMetrics{packetsRx: pm}
					}

				// Performance monitoring level 2 metrics required
				case "disk.totalReadLatency.average":
					if p, ok := perfMap[val.Instance]; ok {
						p.diskReadLatencyAvg = pm
					} else {
						perfMap[val.Instance] = vmPerformanceMetrics{diskReadLatencyAvg: pm}
					}
				case "virtualDisk.totalReadLatency.average":
					if p, ok := perfMap[val.Instance]; ok {
						p.virtualDiskReadLatencyAvg = pm
					} else {
						perfMap[val.Instance] = vmPerformanceMetrics{virtualDiskReadLatencyAvg: pm}
					}
				case "disk.totalWriteLatency.average":
					if p, ok := perfMap[val.Instance]; ok {
						p.diskWriteLatencyAvg = pm
					} else {
						perfMap[val.Instance] = vmPerformanceMetrics{diskWriteLatencyAvg: pm}
					}
				case "virtualDisk.totalWriteLatency.average":
					if p, ok := perfMap[val.Instance]; ok {
						p.virtualDiskWriteLatencyAvg = pm
					} else {
						perfMap[val.Instance] = vmPerformanceMetrics{virtualDiskWriteLatencyAvg: pm}
					}
				case "disk.maxTotalLatency.latest":
					if p, ok := perfMap[val.Instance]; ok {
						p.diskMaxTotalLatency = pm
					} else {
						perfMap[val.Instance] = vmPerformanceMetrics{diskMaxTotalLatency: pm}
					}
				}
			}
		}
	}

	for deviceName, perf := range perfMap {
		rb := v.mb.NewResourceBuilder()
		if p := perf.bytesTx; p != nil {
			v.mb.RecordVcenterVMNetworkThroughputDataPoint(
				pcommon.NewTimestampFromTime(p.timestamp),
				p.value,
				metadata.AttributeThroughputDirectionTransmitted,
			)
		}
		if p := perf.bytesRx; p != nil {
			v.mb.RecordVcenterVMNetworkThroughputDataPoint(
				pcommon.NewTimestampFromTime(p.timestamp),
				p.value,
				metadata.AttributeThroughputDirectionReceived,
			)
		}
		if p := perf.usage; p != nil {
			v.mb.RecordVcenterVMNetworkUsageDataPoint(
				pcommon.NewTimestampFromTime(p.timestamp),
				p.value,
			)
		}
		if p := perf.packetsTx; p != nil {
			v.mb.RecordVcenterVMNetworkPacketCountDataPoint(
				pcommon.NewTimestampFromTime(p.timestamp),
				p.value,
				metadata.AttributeThroughputDirectionTransmitted,
			)
		}
		if p := perf.packetsRx; p != nil {
			v.mb.RecordVcenterVMNetworkPacketCountDataPoint(
				pcommon.NewTimestampFromTime(p.timestamp),
				p.value,
				metadata.AttributeThroughputDirectionReceived,
			)
		}
		if p := perf.diskReadLatencyAvg; p != nil {
			v.mb.RecordVcenterVMDiskLatencyAvgDataPoint(
				pcommon.NewTimestampFromTime(p.timestamp),
				p.value,
				metadata.AttributeDiskDirectionRead,
				metadata.AttributeDiskTypePhysical,
			)
		}
		if p := perf.virtualDiskReadLatencyAvg; p != nil {
			v.mb.RecordVcenterVMDiskLatencyAvgDataPoint(
				pcommon.NewTimestampFromTime(p.timestamp),
				p.value,
				metadata.AttributeDiskDirectionRead,
				metadata.AttributeDiskTypeVirtual,
			)
		}
		if p := perf.diskWriteLatencyAvg; p != nil {
			v.mb.RecordVcenterVMDiskLatencyAvgDataPoint(
				pcommon.NewTimestampFromTime(p.timestamp),
				p.value,
				metadata.AttributeDiskDirectionWrite,
				metadata.AttributeDiskTypePhysical,
			)
		}
		if p := perf.virtualDiskWriteLatencyAvg; p != nil {
			v.mb.RecordVcenterVMDiskLatencyAvgDataPoint(
				pcommon.NewTimestampFromTime(p.timestamp),
				p.value,
				metadata.AttributeDiskDirectionWrite,
				metadata.AttributeDiskTypeVirtual,
			)
		}
		if p := perf.diskMaxTotalLatency; p != nil {
			v.mb.RecordVcenterVMDiskLatencyMaxDataPoint(
				pcommon.NewTimestampFromTime(p.timestamp),
				p.value,
			)
		}
		rb.SetVcenterClusterName(vri.cluster)
		rb.SetVcenterHostName(vri.host)
		rb.SetVcenterVMName(vri.vm)
		rb.SetVcenterVMID(vri.vmID)
		rb.SetVcenterDeviceName(deviceName)
		v.mb.EmitForResource(metadata.WithResource(rb.Emit()))
	}
}

type hostPerformanceMetrics struct {
	netUsageAverage          *perfMetric
	netBytesTx               *perfMetric
	netBytesRx               *perfMetric
	netPacketsTx             *perfMetric
	netPacketsRx             *perfMetric
	netErrorsRx              *perfMetric
	netErrorsTx              *perfMetric
	diskTotalWriteLatencyAvg *perfMetric
	diskTotalReadLatencyAvg  *perfMetric
	diskTotalMaxLatency      *perfMetric
	diskReadAverage          *perfMetric
	diskWriteAverage         *perfMetric
}

type hostResourceInfo struct {
	cluster string
	host    string
}

func (v *vcenterMetricScraper) processHostPerformance(metrics []performance.EntityMetric, hri hostResourceInfo) {
	perfMap := map[string]hostPerformanceMetrics{}
	for _, m := range metrics {
		for _, val := range m.Value {
			// Skip metrics without instance
			if val.Instance == "" {
				continue
			}
			for j, nestedValue := range val.Value {
				si := m.SampleInfo[j]
				pm := perfMetric{
					timestamp: si.Timestamp,
					value:     nestedValue,
				}
				switch val.Name {
				// Performance monitoring level 1 metrics
				case "net.usage.average":
					if p, ok := perfMap[val.Instance]; ok {
						p.netUsageAverage = &pm
					} else {
						perfMap[val.Instance] = hostPerformanceMetrics{netUsageAverage: &pm}
					}
				case "net.bytesTx.average":
					if p, ok := perfMap[val.Instance]; ok {
						p.netBytesTx = &pm
					} else {
						perfMap[val.Instance] = hostPerformanceMetrics{netBytesTx: &pm}
					}
				case "net.bytesRx.average":
					if p, ok := perfMap[val.Instance]; ok {
						p.netBytesRx = &pm
					} else {
						perfMap[val.Instance] = hostPerformanceMetrics{netBytesRx: &pm}
					}
				case "net.packetsTx.summation":
					if p, ok := perfMap[val.Instance]; ok {
						p.netPacketsTx = &pm
					} else {
						perfMap[val.Instance] = hostPerformanceMetrics{netPacketsTx: &pm}
					}
				case "net.packetsRx.summation":
					if p, ok := perfMap[val.Instance]; ok {
						p.netPacketsRx = &pm
					} else {
						perfMap[val.Instance] = hostPerformanceMetrics{netPacketsRx: &pm}
					}
				// Following requires performance level 2
				case "net.errorsRx.summation":
					if p, ok := perfMap[val.Instance]; ok {
						p.netErrorsRx = &pm
					} else {
						perfMap[val.Instance] = hostPerformanceMetrics{netErrorsRx: &pm}
					}
				case "net.errorsTx.summation":
					if p, ok := perfMap[val.Instance]; ok {
						p.netErrorsTx = &pm
					} else {
						perfMap[val.Instance] = hostPerformanceMetrics{netErrorsTx: &pm}
					}
				case "disk.totalWriteLatency.average":
					if p, ok := perfMap[val.Instance]; ok {
						p.diskTotalWriteLatencyAvg = &pm
					} else {
						perfMap[val.Instance] = hostPerformanceMetrics{diskTotalWriteLatencyAvg: &pm}
					}
				case "disk.totalReadLatency.average":
					if p, ok := perfMap[val.Instance]; ok {
						p.diskTotalReadLatencyAvg = &pm
					} else {
						perfMap[val.Instance] = hostPerformanceMetrics{diskTotalReadLatencyAvg: &pm}
					}
				case "disk.maxTotalLatency.latest":
					if p, ok := perfMap[val.Instance]; ok {
						p.diskTotalMaxLatency = &pm
					} else {
						perfMap[val.Instance] = hostPerformanceMetrics{diskTotalMaxLatency: &pm}
					}
				// Following requires performance level 4
				case "disk.read.average":
					if p, ok := perfMap[val.Instance]; ok {
						p.diskReadAverage = &pm
					} else {
						perfMap[val.Instance] = hostPerformanceMetrics{diskReadAverage: &pm}
					}
				case "disk.write.average":
					if p, ok := perfMap[val.Instance]; ok {
						p.diskWriteAverage = &pm
					} else {
						perfMap[val.Instance] = hostPerformanceMetrics{diskWriteAverage: &pm}
					}
				}
			}
		}
	}

	for deviceName, perf := range perfMap {
		if p := perf.netUsageAverage; p != nil {
			v.mb.RecordVcenterHostNetworkUsageDataPoint(
				pcommon.NewTimestampFromTime(p.timestamp),
				p.value,
			)
		}
		if p := perf.netBytesTx; p != nil {
			v.mb.RecordVcenterHostNetworkThroughputDataPoint(
				pcommon.NewTimestampFromTime(p.timestamp),
				p.value,
				metadata.AttributeThroughputDirectionTransmitted,
			)
		}
		if p := perf.netBytesRx; p != nil {
			v.mb.RecordVcenterHostNetworkThroughputDataPoint(
				pcommon.NewTimestampFromTime(p.timestamp),
				p.value,
				metadata.AttributeThroughputDirectionReceived,
			)
		}
		if p := perf.netPacketsTx; p != nil {
			v.mb.RecordVcenterHostNetworkPacketCountDataPoint(
				pcommon.NewTimestampFromTime(p.timestamp),
				p.value,
				metadata.AttributeThroughputDirectionTransmitted,
			)
		}
		if p := perf.netPacketsRx; p != nil {
			v.mb.RecordVcenterHostNetworkPacketCountDataPoint(
				pcommon.NewTimestampFromTime(p.timestamp),
				p.value,
				metadata.AttributeThroughputDirectionReceived,
			)
		}
		if p := perf.netErrorsTx; p != nil {
			v.mb.RecordVcenterHostNetworkPacketErrorsDataPoint(
				pcommon.NewTimestampFromTime(p.timestamp),
				p.value,
				metadata.AttributeThroughputDirectionTransmitted,
			)
		}
		if p := perf.netErrorsRx; p != nil {
			v.mb.RecordVcenterHostNetworkPacketErrorsDataPoint(
				pcommon.NewTimestampFromTime(p.timestamp),
				p.value,
				metadata.AttributeThroughputDirectionReceived,
			)
		}
		if p := perf.diskTotalWriteLatencyAvg; p != nil {
			v.mb.RecordVcenterHostDiskLatencyAvgDataPoint(
				pcommon.NewTimestampFromTime(p.timestamp),
				p.value,
				metadata.AttributeDiskDirectionWrite,
			)
		}
		if p := perf.diskTotalReadLatencyAvg; p != nil {
			v.mb.RecordVcenterHostDiskLatencyAvgDataPoint(
				pcommon.NewTimestampFromTime(p.timestamp),
				p.value,
				metadata.AttributeDiskDirectionRead,
			)
		}
		if p := perf.diskTotalMaxLatency; p != nil {
			v.mb.RecordVcenterHostDiskLatencyMaxDataPoint(
				pcommon.NewTimestampFromTime(p.timestamp),
				p.value,
			)
		}
		if p := perf.diskReadAverage; p != nil {
			v.mb.RecordVcenterHostDiskThroughputDataPoint(
				pcommon.NewTimestampFromTime(p.timestamp),
				p.value,
				metadata.AttributeDiskDirectionRead,
			)
		}
		if p := perf.diskWriteAverage; p != nil {
			v.mb.RecordVcenterHostDiskThroughputDataPoint(
				pcommon.NewTimestampFromTime(p.timestamp),
				p.value,
				metadata.AttributeDiskDirectionWrite,
			)
		}

		rb := v.mb.NewResourceBuilder()
		rb.SetVcenterHostName(hri.host)
		rb.SetVcenterClusterName(hri.cluster)
		rb.SetVcenterDeviceName(deviceName)
		v.mb.EmitForResource(metadata.WithResource(rb.Emit()))
	}
}
