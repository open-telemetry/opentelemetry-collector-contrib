// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package vcenterreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/vcenterreceiver"

import (
	"github.com/vmware/govmomi/performance"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/vcenterreceiver/internal/metadata"
)

var enableResourcePoolMemoryUsageAttr = featuregate.GlobalRegistry().MustRegister(
	"receiver.vcenter.resourcePoolMemoryUsageAttribute",
	featuregate.StageAlpha,
	featuregate.WithRegisterFromVersion("v0.104.0"),
	featuregate.WithRegisterDescription("Enables the memory usage type attribute for the vcenter.resource_pool.memory.usage metric"),
	featuregate.WithRegisterToVersion("v0.107.0"))

// recordDatacenterStats records stat metrics for a vSphere Datacenter
func (v *vcenterMetricScraper) recordDatacenterStats(
	ts pcommon.Timestamp,
	dcStat *DatacenterStats,
) {
	// Cluster metrics
	v.mb.RecordVcenterDatacenterClusterCountDataPoint(ts, dcStat.ClusterStatusCounts[types.ManagedEntityStatusRed], metadata.AttributeEntityStatusRed)
	v.mb.RecordVcenterDatacenterClusterCountDataPoint(ts, dcStat.ClusterStatusCounts[types.ManagedEntityStatusYellow], metadata.AttributeEntityStatusYellow)
	v.mb.RecordVcenterDatacenterClusterCountDataPoint(ts, dcStat.ClusterStatusCounts[types.ManagedEntityStatusGreen], metadata.AttributeEntityStatusGreen)
	v.mb.RecordVcenterDatacenterClusterCountDataPoint(ts, dcStat.ClusterStatusCounts[types.ManagedEntityStatusGray], metadata.AttributeEntityStatusGray)

	// VM metrics
	for powerState, vmStatusCounts := range dcStat.VMStats {
		for status, count := range vmStatusCounts {
			entityStatus, okStatus := getEntityStatusAttribute(status)
			vmPowerState, okPowerState := getVMPowerStateAttribute(powerState)
			switch {
			case okStatus && okPowerState:
				v.mb.RecordVcenterDatacenterVMCountDataPoint(ts, count, entityStatus, vmPowerState)
			case !okStatus && okPowerState:
				v.mb.RecordVcenterDatacenterVMCountDataPoint(ts, count, metadata.AttributeEntityStatusGray, vmPowerState)
			case okStatus && !okPowerState:
				v.mb.RecordVcenterDatacenterVMCountDataPoint(ts, count, entityStatus, metadata.AttributeVMCountPowerStateUnknown)
			default:
				v.mb.RecordVcenterDatacenterVMCountDataPoint(ts, count, metadata.AttributeEntityStatusGray, metadata.AttributeVMCountPowerStateUnknown)
			}
		}
	}

	// Host metrics
	for powerState, hostStatusCounts := range dcStat.HostStats {
		for status, count := range hostStatusCounts {
			entityStatus, okStatus := getEntityStatusAttribute(status)
			hostPowerState, okPowerState := getHostPowerStateAttribute(powerState)
			switch {
			case okStatus && okPowerState:
				v.mb.RecordVcenterDatacenterHostCountDataPoint(ts, count, entityStatus, hostPowerState)
			case !okStatus && okPowerState:
				v.mb.RecordVcenterDatacenterHostCountDataPoint(ts, count, metadata.AttributeEntityStatusGray, hostPowerState)
			case okStatus && !okPowerState:
				v.mb.RecordVcenterDatacenterHostCountDataPoint(ts, count, entityStatus, metadata.AttributeHostPowerStateUnknown)
			default:
				v.mb.RecordVcenterDatacenterHostCountDataPoint(ts, count, metadata.AttributeEntityStatusGray, metadata.AttributeHostPowerStateUnknown)
			}
		}
	}

	// Datacenter stats
	v.mb.RecordVcenterDatacenterDatastoreCountDataPoint(ts, dcStat.DatastoreCount)
	v.mb.RecordVcenterDatacenterDiskSpaceDataPoint(ts, (dcStat.DiskCapacity - dcStat.DiskFree), metadata.AttributeDiskStateUsed)
	v.mb.RecordVcenterDatacenterDiskSpaceDataPoint(ts, dcStat.DiskFree, metadata.AttributeDiskStateAvailable)
	v.mb.RecordVcenterDatacenterCPULimitDataPoint(ts, dcStat.CPULimit)
	v.mb.RecordVcenterDatacenterMemoryLimitDataPoint(ts, dcStat.MemoryLimit)
}

func getEntityStatusAttribute(status types.ManagedEntityStatus) (metadata.AttributeEntityStatus, bool) {
	entityStatusToAttribute := map[types.ManagedEntityStatus]metadata.AttributeEntityStatus{
		types.ManagedEntityStatusRed:    metadata.AttributeEntityStatusRed,
		types.ManagedEntityStatusYellow: metadata.AttributeEntityStatusYellow,
		types.ManagedEntityStatusGreen:  metadata.AttributeEntityStatusGreen,
		types.ManagedEntityStatusGray:   metadata.AttributeEntityStatusGray,
	}
	attr, ok := entityStatusToAttribute[status]
	return attr, ok
}

func getVMPowerStateAttribute(state string) (metadata.AttributeVMCountPowerState, bool) {
	vmPowerStateToAttribute := map[string]metadata.AttributeVMCountPowerState{
		"poweredOn":  metadata.AttributeVMCountPowerStateOn,
		"poweredOff": metadata.AttributeVMCountPowerStateOff,
		"suspended":  metadata.AttributeVMCountPowerStateSuspended,
	}
	attr, ok := vmPowerStateToAttribute[state]
	return attr, ok
}

func getHostPowerStateAttribute(state string) (metadata.AttributeHostPowerState, bool) {
	hostPowerStateToAttribute := map[string]metadata.AttributeHostPowerState{
		"poweredOn":  metadata.AttributeHostPowerStateOn,
		"poweredOff": metadata.AttributeHostPowerStateOff,
		"standby":    metadata.AttributeHostPowerStateStandby,
		"unknown":    metadata.AttributeHostPowerStateUnknown,
	}
	attr, ok := hostPowerStateToAttribute[state]
	return attr, ok
}

// recordDatastoreStats records stat metrics for a vSphere Datastore
func (v *vcenterMetricScraper) recordDatastoreStats(
	ts pcommon.Timestamp,
	ds *mo.Datastore,
) {
	s := ds.Summary
	diskUsage := s.Capacity - s.FreeSpace
	diskUtilization := float64(diskUsage) / float64(s.Capacity) * 100
	v.mb.RecordVcenterDatastoreDiskUsageDataPoint(ts, diskUsage, metadata.AttributeDiskStateUsed)
	v.mb.RecordVcenterDatastoreDiskUsageDataPoint(ts, s.FreeSpace, metadata.AttributeDiskStateAvailable)
	v.mb.RecordVcenterDatastoreDiskUtilizationDataPoint(ts, diskUtilization)
}

// recordClusterStats records stat metrics for a vSphere Cluster
func (v *vcenterMetricScraper) recordClusterStats(
	ts pcommon.Timestamp,
	cr *mo.ComputeResource,
	vmGroupInfo *vmGroupInfo,
) {
	if vmGroupInfo != nil {
		poweredOnVMs := vmGroupInfo.poweredOn
		poweredOffVMs := vmGroupInfo.poweredOff
		suspendedVMs := vmGroupInfo.suspended
		templates := vmGroupInfo.templates
		v.mb.RecordVcenterClusterVMCountDataPoint(ts, poweredOnVMs, metadata.AttributeVMCountPowerStateOn)
		v.mb.RecordVcenterClusterVMCountDataPoint(ts, poweredOffVMs, metadata.AttributeVMCountPowerStateOff)
		v.mb.RecordVcenterClusterVMCountDataPoint(ts, suspendedVMs, metadata.AttributeVMCountPowerStateSuspended)
		v.mb.RecordVcenterClusterVMTemplateCountDataPoint(ts, templates)
	}

	s := cr.Summary.GetComputeResourceSummary()
	v.mb.RecordVcenterClusterCPULimitDataPoint(ts, int64(s.TotalCpu))
	v.mb.RecordVcenterClusterCPUEffectiveDataPoint(ts, int64(s.EffectiveCpu))
	v.mb.RecordVcenterClusterMemoryEffectiveDataPoint(ts, s.EffectiveMemory<<20)
	v.mb.RecordVcenterClusterMemoryLimitDataPoint(ts, s.TotalMemory)
	v.mb.RecordVcenterClusterHostCountDataPoint(ts, int64(s.NumHosts-s.NumEffectiveHosts), false)
	v.mb.RecordVcenterClusterHostCountDataPoint(ts, int64(s.NumEffectiveHosts), true)
}

// recordClusterVSANMetrics records vSAN metrics for a vSphere Cluster
func (v *vcenterMetricScraper) recordClusterVSANMetrics(vSANMetrics *vSANMetricResults) {
	for _, metric := range vSANMetrics.MetricDetails {
		for i, value := range metric.Values {
			timestamp := metric.Timestamps[i]
			switch metric.MetricLabel {
			case "iopsRead":
				v.mb.RecordVcenterClusterVsanOperationsDataPoint(pcommon.NewTimestampFromTime(*timestamp), value, metadata.AttributeVsanOperationTypeRead)
			case "iopsWrite":
				v.mb.RecordVcenterClusterVsanOperationsDataPoint(pcommon.NewTimestampFromTime(*timestamp), value, metadata.AttributeVsanOperationTypeWrite)
			case "throughputRead":
				readRate := float64(value) / float64(metric.Interval)
				v.mb.RecordVcenterClusterVsanThroughputDataPoint(pcommon.NewTimestampFromTime(*timestamp), readRate, metadata.AttributeVsanThroughputDirectionRead)
			case "throughputWrite":
				writeRate := float64(value) / float64(metric.Interval)
				v.mb.RecordVcenterClusterVsanThroughputDataPoint(pcommon.NewTimestampFromTime(*timestamp), writeRate, metadata.AttributeVsanThroughputDirectionWrite)
			case "latencyAvgRead":
				v.mb.RecordVcenterClusterVsanLatencyAvgDataPoint(pcommon.NewTimestampFromTime(*timestamp), value, metadata.AttributeVsanLatencyTypeRead)
			case "latencyAvgWrite":
				v.mb.RecordVcenterClusterVsanLatencyAvgDataPoint(pcommon.NewTimestampFromTime(*timestamp), value, metadata.AttributeVsanLatencyTypeWrite)
			case "congestion":
				rate := float64(value) / float64(metric.Interval)
				v.mb.RecordVcenterClusterVsanCongestionsDataPoint(pcommon.NewTimestampFromTime(*timestamp), rate)
			}
		}
	}
}

// recordResourcePoolStats records stat metrics for a vSphere Resource Pool
func (v *vcenterMetricScraper) recordResourcePoolStats(
	ts pcommon.Timestamp,
	rp *mo.ResourcePool,
) {
	s := rp.Summary.GetResourcePoolSummary()
	if s.QuickStats != nil {
		v.mb.RecordVcenterResourcePoolCPUUsageDataPoint(ts, s.QuickStats.OverallCpuUsage)

		if enableResourcePoolMemoryUsageAttr.IsEnabled() {
			v.mb.RecordVcenterResourcePoolMemoryUsageDataPoint(ts, s.QuickStats.GuestMemoryUsage, metadata.AttributeMemoryUsageTypeGuest)
			v.mb.RecordVcenterResourcePoolMemoryUsageDataPoint(ts, s.QuickStats.HostMemoryUsage, metadata.AttributeMemoryUsageTypeHost)
			v.mb.RecordVcenterResourcePoolMemoryUsageDataPoint(ts, s.QuickStats.OverheadMemory, metadata.AttributeMemoryUsageTypeOverhead)
		} else {
			v.mb.RecordVcenterResourcePoolMemoryUsageDataPointWithoutTypeAttribute(ts, s.QuickStats.GuestMemoryUsage)
		}

		v.mb.RecordVcenterResourcePoolMemorySwappedDataPoint(ts, s.QuickStats.SwappedMemory)
		v.mb.RecordVcenterResourcePoolMemoryBalloonedDataPoint(ts, s.QuickStats.BalloonedMemory)
		v.mb.RecordVcenterResourcePoolMemoryGrantedDataPoint(ts, s.QuickStats.PrivateMemory, metadata.AttributeMemoryGrantedTypePrivate)
		v.mb.RecordVcenterResourcePoolMemoryGrantedDataPoint(ts, s.QuickStats.SharedMemory, metadata.AttributeMemoryGrantedTypeShared)
	}

	v.mb.RecordVcenterResourcePoolCPUSharesDataPoint(ts, int64(s.Config.CpuAllocation.Shares.Shares))
	v.mb.RecordVcenterResourcePoolMemorySharesDataPoint(ts, int64(s.Config.MemoryAllocation.Shares.Shares))
}

// recordClusterStats records stat metrics for a vSphere Host
func (v *vcenterMetricScraper) recordHostSystemStats(
	ts pcommon.Timestamp,
	hs *mo.HostSystem,
) {
	s := hs.Summary
	h := s.Hardware
	z := s.QuickStats

	v.mb.RecordVcenterHostMemoryUsageDataPoint(ts, int64(z.OverallMemoryUsage))
	memUtilization := 100 * float64(z.OverallMemoryUsage) / float64(h.MemorySize>>20)
	v.mb.RecordVcenterHostMemoryUtilizationDataPoint(ts, memUtilization)
	v.mb.RecordVcenterHostCPUUsageDataPoint(ts, int64(z.OverallCpuUsage))

	cpuCapacity := float64(int32(h.NumCpuCores) * h.CpuMhz)
	v.mb.RecordVcenterHostCPUCapacityDataPoint(ts, int64(cpuCapacity))
	v.mb.RecordVcenterHostMemoryCapacityDataPoint(ts, float64(h.MemorySize>>20))
	cpuUtilization := 100 * float64(z.OverallCpuUsage) / cpuCapacity
	v.mb.RecordVcenterHostCPUUtilizationDataPoint(ts, cpuUtilization)
}

// recordHostVSANMetrics records vSAN metrics for a vSphere host
func (v *vcenterMetricScraper) recordHostVSANMetrics(vSANMetrics *vSANMetricResults) {
	for _, metric := range vSANMetrics.MetricDetails {
		for i, value := range metric.Values {
			timestamp := metric.Timestamps[i]
			switch metric.MetricLabel {
			case "iopsRead":
				v.mb.RecordVcenterHostVsanOperationsDataPoint(pcommon.NewTimestampFromTime(*timestamp), value, metadata.AttributeVsanOperationTypeRead)
			case "iopsWrite":
				v.mb.RecordVcenterHostVsanOperationsDataPoint(pcommon.NewTimestampFromTime(*timestamp), value, metadata.AttributeVsanOperationTypeWrite)
			case "throughputRead":
				readRate := float64(value) / float64(metric.Interval)
				v.mb.RecordVcenterHostVsanThroughputDataPoint(pcommon.NewTimestampFromTime(*timestamp), readRate, metadata.AttributeVsanThroughputDirectionRead)
			case "throughputWrite":
				writeRate := float64(value) / float64(metric.Interval)
				v.mb.RecordVcenterHostVsanThroughputDataPoint(pcommon.NewTimestampFromTime(*timestamp), writeRate, metadata.AttributeVsanThroughputDirectionWrite)
			case "latencyAvgRead":
				v.mb.RecordVcenterHostVsanLatencyAvgDataPoint(pcommon.NewTimestampFromTime(*timestamp), value, metadata.AttributeVsanLatencyTypeRead)
			case "latencyAvgWrite":
				v.mb.RecordVcenterHostVsanLatencyAvgDataPoint(pcommon.NewTimestampFromTime(*timestamp), value, metadata.AttributeVsanLatencyTypeWrite)
			case "congestion":
				congestionRate := float64(value) / float64(metric.Interval)
				v.mb.RecordVcenterHostVsanCongestionsDataPoint(pcommon.NewTimestampFromTime(*timestamp), congestionRate)
			case "clientCacheHitRate":
				v.mb.RecordVcenterHostVsanCacheHitRateDataPoint(pcommon.NewTimestampFromTime(*timestamp), value)
			}
		}
	}
}

// recordVMStats records stat metrics for a vSphere Virtual Machine
func (v *vcenterMetricScraper) recordVMStats(
	ts pcommon.Timestamp,
	vm *mo.VirtualMachine,
	hs *mo.HostSystem,
) {
	diskUsed := vm.Summary.Storage.Committed
	diskFree := vm.Summary.Storage.Uncommitted

	v.mb.RecordVcenterVMDiskUsageDataPoint(ts, diskUsed, metadata.AttributeDiskStateUsed)
	v.mb.RecordVcenterVMDiskUsageDataPoint(ts, diskFree, metadata.AttributeDiskStateAvailable)

	if vm.Config.Template {
		return
	}

	if diskFree != 0 {
		diskUtilization := float64(diskUsed) / float64(diskFree+diskUsed) * 100
		v.mb.RecordVcenterVMDiskUtilizationDataPoint(ts, diskUtilization)
	}

	memUsage := vm.Summary.QuickStats.GuestMemoryUsage
	balloonedMem := vm.Summary.QuickStats.BalloonedMemory
	swappedMem := vm.Summary.QuickStats.SwappedMemory
	swappedSSDMem := vm.Summary.QuickStats.SsdSwappedMemory
	grantedMem := vm.Summary.QuickStats.GrantedMemory

	if totalMemory := vm.Summary.Config.MemorySizeMB; totalMemory > 0 && memUsage > 0 {
		memoryUtilization := float64(memUsage) / float64(totalMemory) * 100
		v.mb.RecordVcenterVMMemoryUtilizationDataPoint(ts, memoryUtilization)
	}

	v.mb.RecordVcenterVMMemoryUsageDataPoint(ts, int64(memUsage))
	v.mb.RecordVcenterVMMemoryBalloonedDataPoint(ts, int64(balloonedMem))
	v.mb.RecordVcenterVMMemorySwappedDataPoint(ts, int64(swappedMem))
	v.mb.RecordVcenterVMMemorySwappedSsdDataPoint(ts, swappedSSDMem)
	v.mb.RecordVcenterVMMemoryGrantedDataPoint(ts, int64(grantedMem))

	cpuUsage := vm.Summary.QuickStats.OverallCpuUsage
	if cpuUsage == 0 {
		// Most likely the VM is unavailable or is unreachable.
		return
	}
	v.mb.RecordVcenterVMCPUUsageDataPoint(ts, int64(cpuUsage))

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
	v.mb.RecordVcenterVMCPUUtilizationDataPoint(ts, 100*float64(cpuUsage)/float64(cpuLimit))

	cpuReadiness := vm.Summary.QuickStats.OverallCpuReadiness
	v.mb.RecordVcenterVMCPUReadinessDataPoint(ts, int64(cpuReadiness))
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
	"net.droppedTx.summation",
	"net.droppedRx.summation",
	// disk metrics
	"disk.totalReadLatency.average",
	"disk.totalWriteLatency.average",
	"disk.maxTotalLatency.latest",
	"disk.read.average",
	"disk.write.average",
	// cpu metrics
	"cpu.reservedCapacity.average",
	"cpu.totalCapacity.average",
}

// recordHostPerformanceMetrics records performance metrics for a vSphere Host
func (v *vcenterMetricScraper) recordHostPerformanceMetrics(entityMetric *performance.EntityMetric) {
	for _, val := range entityMetric.Value {
		for j, nestedValue := range val.Value {
			si := entityMetric.SampleInfo[j]
			switch val.Name {
			/******************************************/
			// Performance Monitoring Level 1 Metrics //
			/******************************************/
			// (per device requires level 3)
			case "disk.maxTotalLatency.latest":
				v.mb.RecordVcenterHostDiskLatencyMaxDataPoint(pcommon.NewTimestampFromTime(si.Timestamp), nestedValue, val.Instance)
			// (per device requires level 4)
			case "net.usage.average":
				v.mb.RecordVcenterHostNetworkUsageDataPoint(pcommon.NewTimestampFromTime(si.Timestamp), nestedValue, val.Instance)
			/******************************************/
			// Following Requires Performance Level 2 //
			/******************************************/
			// (per device requires level 3)
			case "net.bytesTx.average":
				v.mb.RecordVcenterHostNetworkThroughputDataPoint(pcommon.NewTimestampFromTime(si.Timestamp), nestedValue, metadata.AttributeThroughputDirectionTransmitted, val.Instance)
			case "net.bytesRx.average":
				v.mb.RecordVcenterHostNetworkThroughputDataPoint(pcommon.NewTimestampFromTime(si.Timestamp), nestedValue, metadata.AttributeThroughputDirectionReceived, val.Instance)
			case "net.packetsTx.summation":
				txRate := float64(nestedValue) / 20
				v.mb.RecordVcenterHostNetworkPacketRateDataPoint(pcommon.NewTimestampFromTime(si.Timestamp), txRate, metadata.AttributeThroughputDirectionTransmitted, val.Instance)
			case "net.packetsRx.summation":
				rxRate := float64(nestedValue) / 20
				v.mb.RecordVcenterHostNetworkPacketRateDataPoint(pcommon.NewTimestampFromTime(si.Timestamp), rxRate, metadata.AttributeThroughputDirectionReceived, val.Instance)
			case "net.droppedTx.summation":
				txRate := float64(nestedValue) / 20
				v.mb.RecordVcenterHostNetworkPacketDropRateDataPoint(pcommon.NewTimestampFromTime(si.Timestamp), txRate, metadata.AttributeThroughputDirectionTransmitted, val.Instance)
			case "net.droppedRx.summation":
				rxRate := float64(nestedValue) / 20
				v.mb.RecordVcenterHostNetworkPacketDropRateDataPoint(pcommon.NewTimestampFromTime(si.Timestamp), rxRate, metadata.AttributeThroughputDirectionReceived, val.Instance)
			case "net.errorsRx.summation":
				rxRate := float64(nestedValue) / 20
				v.mb.RecordVcenterHostNetworkPacketErrorRateDataPoint(pcommon.NewTimestampFromTime(si.Timestamp), rxRate, metadata.AttributeThroughputDirectionReceived, val.Instance)
			case "net.errorsTx.summation":
				txRate := float64(nestedValue) / 20
				v.mb.RecordVcenterHostNetworkPacketErrorRateDataPoint(pcommon.NewTimestampFromTime(si.Timestamp), txRate, metadata.AttributeThroughputDirectionTransmitted, val.Instance)
			case "cpu.reservedCapacity.average":
				v.mb.RecordVcenterHostCPUReservedDataPoint(pcommon.NewTimestampFromTime(si.Timestamp), nestedValue, metadata.AttributeCPUReservationTypeUsed)
			case "cpu.totalCapacity.average":
				v.mb.RecordVcenterHostCPUReservedDataPoint(pcommon.NewTimestampFromTime(si.Timestamp), nestedValue, metadata.AttributeCPUReservationTypeTotal)
			case "disk.totalWriteLatency.average":
				v.mb.RecordVcenterHostDiskLatencyAvgDataPoint(pcommon.NewTimestampFromTime(si.Timestamp), nestedValue, metadata.AttributeDiskDirectionWrite, val.Instance)
			case "disk.totalReadLatency.average":
				v.mb.RecordVcenterHostDiskLatencyAvgDataPoint(pcommon.NewTimestampFromTime(si.Timestamp), nestedValue, metadata.AttributeDiskDirectionRead, val.Instance)
			case "disk.read.average":
				v.mb.RecordVcenterHostDiskThroughputDataPoint(pcommon.NewTimestampFromTime(si.Timestamp), nestedValue, metadata.AttributeDiskDirectionRead, val.Instance)
			case "disk.write.average":
				v.mb.RecordVcenterHostDiskThroughputDataPoint(pcommon.NewTimestampFromTime(si.Timestamp), nestedValue, metadata.AttributeDiskDirectionWrite, val.Instance)
			}
		}
	}
}

// vmPerfMetricList may be customizable in the future but here is the full list of Virtual Machine Performance Counters
// https://docs.vmware.com/en/vRealize-Operations/8.6/com.vmware.vcom.metrics.doc/GUID-1322F5A4-DA1D-481F-BBEA-99B228E96AF2.html
var vmPerfMetricList = []string{
	// network metrics
	"net.packetsTx.summation",
	"net.packetsRx.summation",
	"net.droppedTx.summation",
	"net.droppedRx.summation",
	"net.bytesRx.average",
	"net.bytesTx.average",
	"net.usage.average",
	"net.broadcastRx.summation",
	"net.broadcastTx.summation",
	"net.multicastRx.summation",
	"net.multicastTx.summation",

	// disk metrics
	"disk.totalWriteLatency.average",
	"disk.totalReadLatency.average",
	"disk.maxTotalLatency.latest",
	"virtualDisk.totalWriteLatency.average",
	"virtualDisk.totalReadLatency.average",
	"virtualDisk.read.average",
	"virtualDisk.write.average",

	// cpu metrics
	"cpu.idle.summation",
	"cpu.wait.summation",
	"cpu.ready.summation",
}

// recordVMPerformanceMetrics records performance metrics for a vSphere Virtual Machine
func (v *vcenterMetricScraper) recordVMPerformanceMetrics(entityMetric *performance.EntityMetric) {
	for _, val := range entityMetric.Value {
		for j, nestedValue := range val.Value {
			si := entityMetric.SampleInfo[j]
			switch val.Name {
			/******************************************/
			// Performance Monitoring Level 1 Metrics //
			/******************************************/
			// (per device requires level 3)
			case "disk.maxTotalLatency.latest":
				v.mb.RecordVcenterVMDiskLatencyMaxDataPoint(pcommon.NewTimestampFromTime(si.Timestamp), nestedValue, val.Instance)
			case "virtualDisk.totalReadLatency.average":
				v.mb.RecordVcenterVMDiskLatencyAvgDataPoint(pcommon.NewTimestampFromTime(si.Timestamp), nestedValue, metadata.AttributeDiskDirectionRead, metadata.AttributeDiskTypeVirtual, val.Instance)
			case "virtualDisk.totalWriteLatency.average":
				v.mb.RecordVcenterVMDiskLatencyAvgDataPoint(pcommon.NewTimestampFromTime(si.Timestamp), nestedValue, metadata.AttributeDiskDirectionWrite, metadata.AttributeDiskTypeVirtual, val.Instance)
			// (per device requires level 4)
			case "net.usage.average":
				v.mb.RecordVcenterVMNetworkUsageDataPoint(pcommon.NewTimestampFromTime(si.Timestamp), nestedValue, val.Instance)
			/******************************************/
			// Following Requires Performance Level 2 //
			/******************************************/
			// (per device requires level 2)
			case "virtualDisk.read.average":
				v.mb.RecordVcenterVMDiskThroughputDataPoint(pcommon.NewTimestampFromTime(si.Timestamp), nestedValue, metadata.AttributeDiskDirectionRead, val.Instance)
			case "virtualDisk.write.average":
				v.mb.RecordVcenterVMDiskThroughputDataPoint(pcommon.NewTimestampFromTime(si.Timestamp), nestedValue, metadata.AttributeDiskDirectionWrite, val.Instance)
			// (per device requires level 3)
			case "net.bytesTx.average":
				v.mb.RecordVcenterVMNetworkThroughputDataPoint(pcommon.NewTimestampFromTime(si.Timestamp), nestedValue, metadata.AttributeThroughputDirectionTransmitted, val.Instance)
			case "net.bytesRx.average":
				v.mb.RecordVcenterVMNetworkThroughputDataPoint(pcommon.NewTimestampFromTime(si.Timestamp), nestedValue, metadata.AttributeThroughputDirectionReceived, val.Instance)
			case "net.packetsTx.summation":
				txRate := float64(nestedValue) / 20
				v.mb.RecordVcenterVMNetworkPacketRateDataPoint(pcommon.NewTimestampFromTime(si.Timestamp), txRate, metadata.AttributeThroughputDirectionTransmitted, val.Instance)
			case "net.packetsRx.summation":
				rxRate := float64(nestedValue) / 20
				v.mb.RecordVcenterVMNetworkPacketRateDataPoint(pcommon.NewTimestampFromTime(si.Timestamp), rxRate, metadata.AttributeThroughputDirectionReceived, val.Instance)
			case "net.droppedTx.summation":
				txRate := float64(nestedValue) / 20
				v.mb.RecordVcenterVMNetworkPacketDropRateDataPoint(pcommon.NewTimestampFromTime(si.Timestamp), txRate, metadata.AttributeThroughputDirectionTransmitted, val.Instance)
			case "net.droppedRx.summation":
				rxRate := float64(nestedValue) / 20
				v.mb.RecordVcenterVMNetworkPacketDropRateDataPoint(pcommon.NewTimestampFromTime(si.Timestamp), rxRate, metadata.AttributeThroughputDirectionReceived, val.Instance)
			case "net.multicastRx.summation":
				rxRate := float64(nestedValue) / 20
				v.mb.RecordVcenterVMNetworkMulticastPacketRateDataPoint(pcommon.NewTimestampFromTime(si.Timestamp), rxRate, metadata.AttributeThroughputDirectionReceived, val.Instance)
			case "net.multicastTx.summation":
				txRate := float64(nestedValue) / 20
				v.mb.RecordVcenterVMNetworkMulticastPacketRateDataPoint(pcommon.NewTimestampFromTime(si.Timestamp), txRate, metadata.AttributeThroughputDirectionTransmitted, val.Instance)
			case "cpu.idle.summation":
				idleTime := float64(nestedValue) / float64(si.Interval) * 10
				v.mb.RecordVcenterVMCPUTimeDataPoint(pcommon.NewTimestampFromTime(si.Timestamp), idleTime, metadata.AttributeCPUStateIdle, val.Instance)
			case "cpu.wait.summation":
				waitTime := float64(nestedValue) / float64(si.Interval) * 10
				v.mb.RecordVcenterVMCPUTimeDataPoint(pcommon.NewTimestampFromTime(si.Timestamp), waitTime, metadata.AttributeCPUStateWait, val.Instance)
			case "cpu.ready.summation":
				readyTime := float64(nestedValue) / float64(si.Interval) * 10
				v.mb.RecordVcenterVMCPUTimeDataPoint(pcommon.NewTimestampFromTime(si.Timestamp), readyTime, metadata.AttributeCPUStateReady, val.Instance)
			case "net.broadcastRx.summation":
				rxRate := float64(nestedValue) / 20
				v.mb.RecordVcenterVMNetworkBroadcastPacketRateDataPoint(pcommon.NewTimestampFromTime(si.Timestamp), rxRate, metadata.AttributeThroughputDirectionReceived, val.Instance)
			case "net.broadcastTx.summation":
				txRate := float64(nestedValue) / 20
				v.mb.RecordVcenterVMNetworkBroadcastPacketRateDataPoint(pcommon.NewTimestampFromTime(si.Timestamp), txRate, metadata.AttributeThroughputDirectionTransmitted, val.Instance)
			}
		}
	}
}

// recordVMVSANMetrics records vSAN metrics for a vSphere Virtual Machine
func (v *vcenterMetricScraper) recordVMVSANMetrics(vSANMetrics *vSANMetricResults) {
	for _, metric := range vSANMetrics.MetricDetails {
		for i, value := range metric.Values {
			timestamp := metric.Timestamps[i]
			switch metric.MetricLabel {
			case "iopsRead":
				v.mb.RecordVcenterVMVsanOperationsDataPoint(pcommon.NewTimestampFromTime(*timestamp), value, metadata.AttributeVsanOperationTypeRead)
			case "iopsWrite":
				v.mb.RecordVcenterVMVsanOperationsDataPoint(pcommon.NewTimestampFromTime(*timestamp), value, metadata.AttributeVsanOperationTypeWrite)
			case "throughputRead":
				readRate := float64(value) / float64(metric.Interval)
				v.mb.RecordVcenterVMVsanThroughputDataPoint(pcommon.NewTimestampFromTime(*timestamp), readRate, metadata.AttributeVsanThroughputDirectionRead)
			case "throughputWrite":
				writeRate := float64(value) / float64(metric.Interval)
				v.mb.RecordVcenterVMVsanThroughputDataPoint(pcommon.NewTimestampFromTime(*timestamp), writeRate, metadata.AttributeVsanThroughputDirectionWrite)
			case "latencyRead":
				v.mb.RecordVcenterVMVsanLatencyAvgDataPoint(pcommon.NewTimestampFromTime(*timestamp), value, metadata.AttributeVsanLatencyTypeRead)
			case "latencyWrite":
				v.mb.RecordVcenterVMVsanLatencyAvgDataPoint(pcommon.NewTimestampFromTime(*timestamp), value, metadata.AttributeVsanLatencyTypeWrite)
			}
		}
	}
}
