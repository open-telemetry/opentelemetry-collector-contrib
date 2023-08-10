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
	bytesTx                    []perfMetric
	bytesRx                    []perfMetric
	usage                      []perfMetric
	packetsTx                  []perfMetric
	packetsRx                  []perfMetric
	diskReadLatencyAvg         []perfMetric
	virtualDiskReadLatencyAvg  []perfMetric
	diskWriteLatencyAvg        []perfMetric
	virtualDiskWriteLatencyAvg []perfMetric
	diskMaxTotalLatency        []perfMetric
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
				pm := perfMetric{
					timestamp: si.Timestamp,
					value:     nestedValue,
				}
				switch val.Name {
				// Performance monitoring level 1 metrics
				case "net.bytesTx.average":
					if p, ok := perfMap[val.Instance]; ok {
						p.bytesTx = append(p.bytesTx, pm)
					} else {
						perfMap[val.Instance] = vmPerformanceMetrics{bytesTx: []perfMetric{pm}}
					}
				case "net.bytesRx.average":
					if p, ok := perfMap[val.Instance]; ok {
						p.bytesRx = append(p.bytesRx, pm)
					} else {
						perfMap[val.Instance] = vmPerformanceMetrics{bytesRx: []perfMetric{pm}}
					}
				case "net.usage.average":
					if p, ok := perfMap[val.Instance]; ok {
						p.usage = append(p.usage, pm)
					} else {
						perfMap[val.Instance] = vmPerformanceMetrics{usage: []perfMetric{pm}}
					}
				case "net.packetsTx.summation":
					if p, ok := perfMap[val.Instance]; ok {
						p.packetsTx = append(p.packetsTx, pm)
					} else {
						perfMap[val.Instance] = vmPerformanceMetrics{packetsTx: []perfMetric{pm}}
					}
				case "net.packetsRx.summation":
					if p, ok := perfMap[val.Instance]; ok {
						p.packetsRx = append(p.packetsRx, pm)
					} else {
						perfMap[val.Instance] = vmPerformanceMetrics{packetsRx: []perfMetric{pm}}
					}

				// Performance monitoring level 2 metrics required
				case "disk.totalReadLatency.average":
					if p, ok := perfMap[val.Instance]; ok {
						p.diskReadLatencyAvg = append(p.diskReadLatencyAvg, pm)
					} else {
						perfMap[val.Instance] = vmPerformanceMetrics{diskReadLatencyAvg: []perfMetric{pm}}
					}
				case "virtualDisk.totalReadLatency.average":
					if p, ok := perfMap[val.Instance]; ok {
						p.virtualDiskReadLatencyAvg = append(p.virtualDiskReadLatencyAvg, pm)
					} else {
						perfMap[val.Instance] = vmPerformanceMetrics{virtualDiskReadLatencyAvg: []perfMetric{pm}}
					}
				case "disk.totalWriteLatency.average":
					if p, ok := perfMap[val.Instance]; ok {
						p.diskWriteLatencyAvg = append(p.diskWriteLatencyAvg, pm)
					} else {
						perfMap[val.Instance] = vmPerformanceMetrics{diskWriteLatencyAvg: []perfMetric{pm}}
					}
				case "virtualDisk.totalWriteLatency.average":
					if p, ok := perfMap[val.Instance]; ok {
						p.virtualDiskWriteLatencyAvg = append(p.virtualDiskWriteLatencyAvg, pm)
					} else {
						perfMap[val.Instance] = vmPerformanceMetrics{virtualDiskWriteLatencyAvg: []perfMetric{pm}}
					}
				case "disk.maxTotalLatency.latest":
					if p, ok := perfMap[val.Instance]; ok {
						p.diskMaxTotalLatency = append(p.diskMaxTotalLatency, pm)
					} else {
						perfMap[val.Instance] = vmPerformanceMetrics{diskMaxTotalLatency: []perfMetric{pm}}
					}
				}
			}
		}
	}

	for deviceName, perf := range perfMap {
		rb := v.mb.NewResourceBuilder()
		for _, p := range perf.bytesTx {
			v.mb.RecordVcenterVMNetworkThroughputDataPoint(
				pcommon.NewTimestampFromTime(p.timestamp),
				p.value,
				metadata.AttributeThroughputDirectionTransmitted,
			)
		}
		for _, p := range perf.bytesRx {
			v.mb.RecordVcenterVMNetworkThroughputDataPoint(
				pcommon.NewTimestampFromTime(p.timestamp),
				p.value,
				metadata.AttributeThroughputDirectionReceived,
			)
		}
		for _, p := range perf.usage {
			v.mb.RecordVcenterVMNetworkUsageDataPoint(
				pcommon.NewTimestampFromTime(p.timestamp),
				p.value,
			)
		}
		for _, p := range perf.packetsTx {
			v.mb.RecordVcenterVMNetworkPacketCountDataPoint(
				pcommon.NewTimestampFromTime(p.timestamp),
				p.value,
				metadata.AttributeThroughputDirectionTransmitted,
			)
		}
		for _, p := range perf.packetsRx {
			v.mb.RecordVcenterVMNetworkPacketCountDataPoint(
				pcommon.NewTimestampFromTime(p.timestamp),
				p.value,
				metadata.AttributeThroughputDirectionReceived,
			)
		}
		for _, p := range perf.diskReadLatencyAvg {
			v.mb.RecordVcenterVMDiskLatencyAvgDataPoint(
				pcommon.NewTimestampFromTime(p.timestamp),
				p.value,
				metadata.AttributeDiskDirectionRead,
				metadata.AttributeDiskTypePhysical,
			)
		}
		for _, p := range perf.virtualDiskReadLatencyAvg {
			v.mb.RecordVcenterVMDiskLatencyAvgDataPoint(
				pcommon.NewTimestampFromTime(p.timestamp),
				p.value,
				metadata.AttributeDiskDirectionRead,
				metadata.AttributeDiskTypeVirtual,
			)
		}
		for _, p := range perf.diskWriteLatencyAvg {
			v.mb.RecordVcenterVMDiskLatencyAvgDataPoint(
				pcommon.NewTimestampFromTime(p.timestamp),
				p.value,
				metadata.AttributeDiskDirectionWrite,
				metadata.AttributeDiskTypePhysical,
			)
		}
		for _, p := range perf.virtualDiskWriteLatencyAvg {
			v.mb.RecordVcenterVMDiskLatencyAvgDataPoint(
				pcommon.NewTimestampFromTime(p.timestamp),
				p.value,
				metadata.AttributeDiskDirectionWrite,
				metadata.AttributeDiskTypeVirtual,
			)
		}
		for _, p := range perf.diskMaxTotalLatency {
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
	netUsageAverage          []perfMetric
	netBytesTx               []perfMetric
	netBytesRx               []perfMetric
	netPacketsTx             []perfMetric
	netPacketsRx             []perfMetric
	netErrorsRx              []perfMetric
	netErrorsTx              []perfMetric
	diskTotalWriteLatencyAvg []perfMetric
	diskTotalReadLatencyAvg  []perfMetric
	diskTotalMaxLatency      []perfMetric
	diskReadAverage          []perfMetric
	diskWriteAverage         []perfMetric
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
						p.netUsageAverage = append(p.netUsageAverage, pm)
					} else {
						perfMap[val.Instance] = hostPerformanceMetrics{netUsageAverage: []perfMetric{pm}}
					}
				case "net.bytesTx.average":
					if p, ok := perfMap[val.Instance]; ok {
						p.netBytesTx = append(p.netBytesTx, pm)
					} else {
						perfMap[val.Instance] = hostPerformanceMetrics{netBytesTx: []perfMetric{pm}}
					}
				case "net.bytesRx.average":
					if p, ok := perfMap[val.Instance]; ok {
						p.netBytesRx = append(p.netBytesRx, pm)
					} else {
						perfMap[val.Instance] = hostPerformanceMetrics{netBytesRx: []perfMetric{pm}}
					}
				case "net.packetsTx.summation":
					if p, ok := perfMap[val.Instance]; ok {
						p.netPacketsTx = append(p.netPacketsTx, pm)
					} else {
						perfMap[val.Instance] = hostPerformanceMetrics{netPacketsTx: []perfMetric{pm}}
					}
				case "net.packetsRx.summation":
					if p, ok := perfMap[val.Instance]; ok {
						p.netPacketsRx = append(p.netPacketsRx, pm)
					} else {
						perfMap[val.Instance] = hostPerformanceMetrics{netPacketsRx: []perfMetric{pm}}
					}
				// Following requires performance level 2
				case "net.errorsRx.summation":
					if p, ok := perfMap[val.Instance]; ok {
						p.netErrorsRx = append(p.netErrorsRx, pm)
					} else {
						perfMap[val.Instance] = hostPerformanceMetrics{netErrorsRx: []perfMetric{pm}}
					}
				case "net.errorsTx.summation":
					if p, ok := perfMap[val.Instance]; ok {
						p.netErrorsTx = append(p.netErrorsTx, pm)
					} else {
						perfMap[val.Instance] = hostPerformanceMetrics{netErrorsTx: []perfMetric{pm}}
					}
				case "disk.totalWriteLatency.average":
					if p, ok := perfMap[val.Instance]; ok {
						p.diskTotalWriteLatencyAvg = append(p.diskTotalWriteLatencyAvg, pm)
					} else {
						perfMap[val.Instance] = hostPerformanceMetrics{diskTotalWriteLatencyAvg: []perfMetric{pm}}
					}
				case "disk.totalReadLatency.average":
					if p, ok := perfMap[val.Instance]; ok {
						p.diskTotalReadLatencyAvg = append(p.diskTotalReadLatencyAvg, pm)
					} else {
						perfMap[val.Instance] = hostPerformanceMetrics{diskTotalReadLatencyAvg: []perfMetric{pm}}
					}
				case "disk.maxTotalLatency.latest":
					if p, ok := perfMap[val.Instance]; ok {
						p.diskTotalMaxLatency = append(p.diskTotalMaxLatency, pm)
					} else {
						perfMap[val.Instance] = hostPerformanceMetrics{diskTotalMaxLatency: []perfMetric{pm}}
					}
				// Following requires performance level 4
				case "disk.read.average":
					if p, ok := perfMap[val.Instance]; ok {
						p.diskReadAverage = append(p.diskReadAverage, pm)
					} else {
						perfMap[val.Instance] = hostPerformanceMetrics{diskReadAverage: []perfMetric{pm}}
					}
				case "disk.write.average":
					if p, ok := perfMap[val.Instance]; ok {
						p.diskWriteAverage = append(p.diskWriteAverage, pm)
					} else {
						perfMap[val.Instance] = hostPerformanceMetrics{diskWriteAverage: []perfMetric{pm}}
					}
				}
			}
		}
	}

	for deviceName, perf := range perfMap {
		for _, p := range perf.netUsageAverage {
			v.mb.RecordVcenterHostNetworkUsageDataPoint(
				pcommon.NewTimestampFromTime(p.timestamp),
				p.value,
			)
		}
		for _, p := range perf.netBytesTx {
			v.mb.RecordVcenterHostNetworkThroughputDataPoint(
				pcommon.NewTimestampFromTime(p.timestamp),
				p.value,
				metadata.AttributeThroughputDirectionTransmitted,
			)
		}
		for _, p := range perf.netBytesRx {
			v.mb.RecordVcenterHostNetworkThroughputDataPoint(
				pcommon.NewTimestampFromTime(p.timestamp),
				p.value,
				metadata.AttributeThroughputDirectionReceived,
			)
		}
		for _, p := range perf.netPacketsTx {
			v.mb.RecordVcenterHostNetworkPacketCountDataPoint(
				pcommon.NewTimestampFromTime(p.timestamp),
				p.value,
				metadata.AttributeThroughputDirectionTransmitted,
			)
		}
		for _, p := range perf.netPacketsRx {
			v.mb.RecordVcenterHostNetworkPacketCountDataPoint(
				pcommon.NewTimestampFromTime(p.timestamp),
				p.value,
				metadata.AttributeThroughputDirectionReceived,
			)
		}
		for _, p := range perf.netErrorsTx {
			v.mb.RecordVcenterHostNetworkPacketErrorsDataPoint(
				pcommon.NewTimestampFromTime(p.timestamp),
				p.value,
				metadata.AttributeThroughputDirectionTransmitted,
			)
		}
		for _, p := range perf.netErrorsRx {
			v.mb.RecordVcenterHostNetworkPacketErrorsDataPoint(
				pcommon.NewTimestampFromTime(p.timestamp),
				p.value,
				metadata.AttributeThroughputDirectionReceived,
			)
		}
		for _, p := range perf.diskTotalWriteLatencyAvg {
			v.mb.RecordVcenterHostDiskLatencyAvgDataPoint(
				pcommon.NewTimestampFromTime(p.timestamp),
				p.value,
				metadata.AttributeDiskDirectionWrite,
			)
		}
		for _, p := range perf.diskTotalReadLatencyAvg {
			v.mb.RecordVcenterHostDiskLatencyAvgDataPoint(
				pcommon.NewTimestampFromTime(p.timestamp),
				p.value,
				metadata.AttributeDiskDirectionRead,
			)
		}
		for _, p := range perf.diskTotalMaxLatency {
			v.mb.RecordVcenterHostDiskLatencyMaxDataPoint(
				pcommon.NewTimestampFromTime(p.timestamp),
				p.value,
			)
		}
		for _, p := range perf.diskReadAverage {
			v.mb.RecordVcenterHostDiskThroughputDataPoint(
				pcommon.NewTimestampFromTime(p.timestamp),
				p.value,
				metadata.AttributeDiskDirectionRead,
			)
		}
		for _, p := range perf.diskWriteAverage {
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
