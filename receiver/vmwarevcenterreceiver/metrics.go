package vmwarevcenterreceiver

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/vmware/govmomi/performance"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
	"go.opentelemetry.io/collector/model/pdata"
	"go.opentelemetry.io/collector/receiver/scrapererror"
)

func (v *vmwareVcenterScraper) recordHostSystemMemoryUsage(
	now pdata.Timestamp,
	hs mo.HostSystem,
	hostname string,
) {
	s := hs.Summary
	h := s.Hardware
	z := s.QuickStats

	memUsage := z.OverallMemoryUsage
	v.mb.RecordVcenterHostMemoryUsageDataPoint(now, int64(memUsage), hostname)

	memUtilization := 100 * float64(z.OverallMemoryUsage) / float64(h.MemorySize>>20)
	v.mb.RecordVcenterHostMemoryUtilizationDataPoint(now, memUtilization, hostname)

	ncpu := int32(h.NumCpuCores)
	cpuUsage := z.OverallCpuUsage
	cpuUtilization := 100 * float64(z.OverallCpuUsage) / float64(ncpu*h.CpuMhz)

	v.mb.RecordVcenterHostCPUUsageDataPoint(now, int64(cpuUsage), hostname)
	v.mb.RecordVcenterHostCPUUtilizationDataPoint(now, cpuUtilization, hostname)
}

func (v *vmwareVcenterScraper) recordVMUsages(
	now pdata.Timestamp,
	vm mo.VirtualMachine,
	instanceID string,
	instanceName string,
) {
	ps := string(vm.Runtime.PowerState)

	memUsage := vm.Summary.QuickStats.GuestMemoryUsage
	balloonedMem := vm.Summary.QuickStats.BalloonedMemory
	v.mb.RecordVcenterVMMemoryUsageDataPoint(now, int64(memUsage), instanceName, instanceID, ps)
	v.mb.RecordVcenterVMMemoryBalloonedDataPoint(now, int64(balloonedMem), instanceName, instanceID, ps)

	diskUsed := vm.Summary.Storage.Committed
	diskFree := vm.Summary.Storage.Uncommitted
	diskUtilization := float64(diskUsed) / float64(diskFree) * 100
	v.mb.RecordVcenterVMDiskUsageDataPoint(now, diskUsed, instanceName, instanceID, ps, "used")
	v.mb.RecordVcenterVMDiskUsageDataPoint(now, diskFree, instanceName, instanceID, ps, "available")
	v.mb.RecordVcenterVMDiskUtilizationDataPoint(now, diskUtilization, instanceName, instanceID, ps)
}

func (v *vmwareVcenterScraper) recordDatastoreProperties(
	now pdata.Timestamp,
	ds mo.Datastore,
) {
	s := ds.Summary
	diskUsage := s.Capacity - s.FreeSpace
	diskUtilization := float64(diskUsage) / float64(s.Capacity) * 100
	v.mb.RecordVcenterDatastoreDiskUsageDataPoint(now, diskUsage, ds.Name, ds.Name)
	v.mb.RecordVcenterDatastoreDiskUtilizationDataPoint(now, diskUtilization, ds.Name)
}

func (v *vmwareVcenterScraper) recordResourcePool(
	now pdata.Timestamp,
	rp mo.ResourcePool,
) {
	s := rp.Summary
	v.mb.RecordVcenterResourcePoolCPUUsageDataPoint(now, s.GetResourcePoolSummary().QuickStats.OverallCpuUsage, rp.Name)
	v.mb.RecordVcenterResourcePoolCPUSharesDataPoint(now, int64(s.GetResourcePoolSummary().Config.CpuAllocation.Shares.Shares), rp.Name)
	v.mb.RecordVcenterResourcePoolMemorySharesDataPoint(now, int64(s.GetResourcePoolSummary().Config.MemoryAllocation.Shares.Shares), rp.Name)
	v.mb.RecordVcenterResourcePoolMemoryUsageDataPoint(now, s.GetResourcePoolSummary().QuickStats.GuestMemoryUsage, rp.Name)
}

func (v *vmwareVcenterScraper) recordVMPerformance(
	ctx context.Context,
	vm mo.VirtualMachine,
	vmUUID, ps string,
	startTime time.Time,
	endTime time.Time,
	errs *scrapererror.ScrapeErrors,
) {
	specs := []types.PerfQuerySpec{
		{
			Entity:    vm.Reference(),
			MetricId:  []types.PerfMetricId{},
			StartTime: &startTime,
			EndTime:   &endTime,
			Format:    "normal",
		},
	}
	vmRef := vm.Reference()
	metrics, err := v.client.PerformanceQuery(ctx, &vmRef, specs, startTime, endTime)
	if err != nil {
		errs.AddPartial(1, err)
		return
	}
	v.processVMPerformanceMetrics(metrics, vm, vmUUID, ps)
}

func (v *vmwareVcenterScraper) processVMPerformanceMetrics(metrics []performance.EntityMetric, vm mo.VirtualMachine, vmUUID, ps string) {
	for _, m := range metrics {
		for i := 0; i < len(m.SampleInfo); i++ {
			val := m.Value[i]
			si := m.SampleInfo[i]
			switch strings.ToLower(val.Name) {
			// Performance monitoring level 1 metrics
			case "net.bytesTx.average":
				v.mb.RecordVcenterVMNetworkThroughputDataPoint(pdata.NewTimestampFromTime(si.Timestamp), val.Value[i], vm.Name, vmUUID, ps, "transmitted")
			case "net.bytesRx.average":
				v.mb.RecordVcenterVMNetworkThroughputDataPoint(pdata.NewTimestampFromTime(si.Timestamp), val.Value[i], vm.Name, vmUUID, ps, "received")
			case "cpu.usage.average":

			case "disk.totalReadLatency.average", "virtualDisk.totalReadLatency.average":
				v.mb.RecordVcenterVMDiskLatencyDataPoint(pdata.NewTimestampFromTime(si.Timestamp), val.Value[i], vm.Name, vmUUID, ps, "read")
			case "disk.totalWriteLatency.average", "virtualDisk.totalWriteLatency.average":
				v.mb.RecordVcenterVMDiskLatencyDataPoint(pdata.NewTimestampFromTime(si.Timestamp), val.Value[i], vm.Name, vmUUID, ps, "write")
				// Performance monitoring level 2 metrics
				//
			}
		}
	}
}

// list of metric IDs https://kb.vmware.com/s/article/2144493
var iopsRead = "iopsRead"
var iopsWrite = "iopsWrite"
var iopsUnmap = "iopsUnmap"
var throughputRead = "throughputRead"
var throughputWrite = "throughputWrite"
var throughputUnmap = "throughputUnmap"
var latencyAvgRead = "latencyAvgRead"
var latencyAvgWrite = "latencyAvgWrite"
var latencyAvgUnmap = "latencyAvgUnmap"
var outstandingIO = "oio"
var congestion = "congestion"
var clientCacheHits = "clientCacheHits"
var clientCacheHitRate = "clientCacheHitRate"

func (v *vmwareVcenterScraper) recordClusterVsanMetric(
	now pdata.Timestamp,
	metricID string,
	cluster string,
	val string,
	errs *scrapererror.ScrapeErrors,
) {
	switch metricID {
	case iopsRead:
		value, err := parseInt(val)
		if err != nil {
			errs.AddPartial(1, err)
		} else {
			v.mb.RecordVcenterClusterVsanOperationsDataPoint(now, value, "read", cluster)
		}
	case iopsWrite:
		value, err := parseInt(val)
		if err != nil {
			errs.AddPartial(1, err)
		} else {
			v.mb.RecordVcenterClusterVsanOperationsDataPoint(now, value, "write", cluster)
		}
	case throughputRead:
		value, err := parseInt(val)
		if err != nil {
			errs.AddPartial(1, err)
		} else {
			v.mb.RecordVcenterClusterVsanThroughputDataPoint(now, value, "read", cluster)
		}
	case throughputWrite:
		value, err := parseInt(val)
		if err != nil {
			errs.AddPartial(1, err)
		} else {
			v.mb.RecordVcenterClusterVsanThroughputDataPoint(now, value, "write", cluster)
		}
	case latencyAvgRead:
		value, err := parseInt(val)
		if err != nil {
			errs.AddPartial(1, err)
		} else {
			v.mb.RecordVcenterClusterVsanLatencyAvgDataPoint(now, value, "read", cluster)
		}
	case latencyAvgWrite:
		value, err := parseInt(val)
		if err != nil {
			errs.AddPartial(1, err)
		} else {
			v.mb.RecordVcenterClusterVsanLatencyAvgDataPoint(now, value, "write", cluster)
		}
	case outstandingIO:
		value, err := parseInt(val)
		if err != nil {
			errs.AddPartial(1, err)
		} else {
			v.mb.RecordVcenterClusterVsanOutstandingIoDataPoint(now, value, cluster)
		}
	case congestion:
		value, err := parseInt(val)
		if err != nil {
			errs.AddPartial(1, err)
		} else {
			v.mb.RecordVcenterClusterVsanCongestionsDataPoint(now, value, cluster)
		}
	}
}

func (v *vmwareVcenterScraper) recordHostVsanMetric(
	now pdata.Timestamp,
	metricID string,
	hostname string,
	val string,
	errs *scrapererror.ScrapeErrors,
) {
	switch metricID {
	case clientCacheHits:
		value, err := parseInt(val)
		if err != nil {
			errs.AddPartial(1, err)
		} else {
			v.mb.RecordVcenterHostVsanCacheReadsDataPoint(now, value, hostname)
		}
	case clientCacheHitRate:
		value, err := parseFloat(val)
		if err != nil {
			errs.AddPartial(1, err)
		} else {
			v.mb.RecordVcenterHostVsanCacheHitRateDataPoint(now, value, hostname)
		}
	case iopsRead:
		value, err := parseInt(val)
		if err != nil {
			errs.AddPartial(1, err)
		} else {
			v.mb.RecordVcenterHostVsanOperationsDataPoint(now, value, "read", hostname)
		}
	case iopsWrite:
		value, err := parseInt(val)
		if err != nil {
			errs.AddPartial(1, err)
		} else {
			v.mb.RecordVcenterHostVsanOperationsDataPoint(now, value, "write", hostname)
		}
	case iopsUnmap:
		value, err := parseInt(val)
		if err != nil {
			errs.AddPartial(1, err)
		} else {
			v.mb.RecordVcenterHostVsanOperationsDataPoint(now, value, "unmap", hostname)
		}
	case throughputRead:
		value, err := parseInt(val)
		if err != nil {
			errs.AddPartial(1, err)
		} else {
			v.mb.RecordVcenterHostVsanThroughputDataPoint(now, value, hostname, "read")
		}
	case throughputWrite:
		value, err := parseInt(val)
		if err != nil {
			errs.AddPartial(1, err)
		} else {
			v.mb.RecordVcenterHostVsanThroughputDataPoint(now, value, hostname, "write")
		}
	case throughputUnmap:
		value, err := parseInt(val)
		if err != nil {
			errs.AddPartial(1, err)
		} else {
			v.mb.RecordVcenterHostVsanThroughputDataPoint(now, value, hostname, "unmap")
		}
	case latencyAvgRead:
		value, err := parseInt(val)
		if err != nil {
			errs.AddPartial(1, err)
		} else {
			v.mb.RecordVcenterHostVsanLatencyAvgDataPoint(now, value, hostname, "read")
		}
	case latencyAvgWrite:
		value, err := parseInt(val)
		if err != nil {
			errs.AddPartial(1, err)
		} else {
			v.mb.RecordVcenterHostVsanLatencyAvgDataPoint(now, value, hostname, "write")
		}
	case latencyAvgUnmap:
		value, err := parseInt(val)
		if err != nil {
			errs.AddPartial(1, err)
		} else {
			v.mb.RecordVcenterHostVsanLatencyAvgDataPoint(now, value, hostname, "unmap")
		}
	case outstandingIO:
		value, err := parseInt(val)
		if err != nil {
			errs.AddPartial(1, err)
		} else {
			v.mb.RecordVcenterHostVsanOutstandingIoDataPoint(now, value, hostname)
		}
	case congestion:
		value, err := parseInt(val)
		if err != nil {
			errs.AddPartial(1, err)
		} else {
			v.mb.RecordVcenterHostVsanOutstandingIoDataPoint(now, value, hostname)
		}
	}
}

func (v *vmwareVcenterScraper) recordVMVsanMetric(
	now pdata.Timestamp,
	metricID string,
	instanceName string,
	instanceUUID string,
	powerState string,
	val string,
	errs *scrapererror.ScrapeErrors,
) {
	switch metricID {
	case iopsRead:
		value, err := parseInt(val)
		if err != nil {
			errs.AddPartial(1, err)
		} else {
			v.mb.RecordVcenterVMVsanOperationsDataPoint(now, value, instanceName, "read", instanceUUID, powerState)
		}
	case iopsWrite:
		value, err := parseInt(val)
		if err != nil {
			errs.AddPartial(1, err)
		} else {
			v.mb.RecordVcenterVMVsanOperationsDataPoint(now, value, instanceName, "write", instanceUUID, powerState)
		}
	case "throughputRead":
		value, err := parseInt(val)
		if err != nil {
			errs.AddPartial(1, err)
		} else {
			v.mb.RecordVcenterVMVsanThroughputDataPoint(now, value, instanceName, "read", instanceUUID, powerState)
		}
	case "throughputWrite":
		value, err := parseInt(val)
		if err != nil {
			errs.AddPartial(1, err)
		} else {
			v.mb.RecordVcenterVMVsanThroughputDataPoint(now, value, instanceName, "write", instanceUUID, powerState)
		}
	case "latencyReadAvg":
		value, err := parseInt(val)
		if err != nil {
			errs.AddPartial(1, err)
		} else {
			v.mb.RecordVcenterVMVsanLatencyAvgDataPoint(now, value, instanceName, "read", instanceUUID, powerState)
		}
	case "latencyWriteAvg":
		value, err := parseInt(val)
		if err != nil {
			errs.AddPartial(1, err)
		} else {
			v.mb.RecordVcenterVMVsanLatencyAvgDataPoint(now, value, instanceName, "write", instanceUUID, powerState)
		}
	}
}

func parseInt(val string) (int64, error) {
	i, err := strconv.Atoi(val)
	return int64(i), err
}

func parseFloat(val string) (float64, error) {
	return strconv.ParseFloat(val, 64)
}
