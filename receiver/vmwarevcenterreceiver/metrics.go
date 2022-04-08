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

func (v *vcenterMetricScraper) recordHostSystemMemoryUsage(
	now pdata.Timestamp,
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
	now pdata.Timestamp,
	vm mo.VirtualMachine,
) {
	memUsage := vm.Summary.QuickStats.GuestMemoryUsage
	balloonedMem := vm.Summary.QuickStats.BalloonedMemory
	v.mb.RecordVcenterVMMemoryUsageDataPoint(now, int64(memUsage))
	v.mb.RecordVcenterVMMemoryBalloonedDataPoint(now, int64(balloonedMem))

	diskUsed := vm.Summary.Storage.Committed
	diskFree := vm.Summary.Storage.Uncommitted
	diskUtilization := float64(diskUsed) / float64(diskFree) * 100
	v.mb.RecordVcenterVMDiskUsageDataPoint(now, diskUsed, "used")
	v.mb.RecordVcenterVMDiskUsageDataPoint(now, diskFree, "total")
	v.mb.RecordVcenterVMDiskUtilizationDataPoint(now, diskUtilization)
}

func (v *vcenterMetricScraper) recordDatastoreProperties(
	now pdata.Timestamp,
	ds mo.Datastore,
) {
	s := ds.Summary
	diskUsage := s.Capacity - s.FreeSpace
	diskUtilization := float64(diskUsage) / float64(s.Capacity) * 100
	v.mb.RecordVcenterDatastoreDiskUsageDataPoint(now, diskUsage, "used")
	v.mb.RecordVcenterDatastoreDiskUsageDataPoint(now, s.Capacity, "total")
	v.mb.RecordVcenterDatastoreDiskUtilizationDataPoint(now, diskUtilization)
}

func (v *vcenterMetricScraper) recordResourcePool(
	now pdata.Timestamp,
	rp mo.ResourcePool,
) {
	s := rp.Summary
	v.mb.RecordVcenterResourcePoolCPUUsageDataPoint(now, s.GetResourcePoolSummary().QuickStats.OverallCpuUsage)
	v.mb.RecordVcenterResourcePoolCPUSharesDataPoint(now, int64(s.GetResourcePoolSummary().Config.CpuAllocation.Shares.Shares))
	v.mb.RecordVcenterResourcePoolMemorySharesDataPoint(now, int64(s.GetResourcePoolSummary().Config.MemoryAllocation.Shares.Shares))
	v.mb.RecordVcenterResourcePoolMemoryUsageDataPoint(now, s.GetResourcePoolSummary().QuickStats.GuestMemoryUsage)
}

func (v *vcenterMetricScraper) recordHostPerformanceMetrics(
	ctx context.Context,
	host mo.HostSystem,
	startTime time.Time,
	endTime time.Time,
	errs *scrapererror.ScrapeErrors,
) {
	specs := []types.PerfQuerySpec{
		{
			Entity:    host.Reference(),
			MetricId:  []types.PerfMetricId{},
			StartTime: &startTime,
			EndTime:   &endTime,
			Format:    "normal",
		},
	}
	hostRef := host.Reference()
	metrics, err := v.client.PerformanceQuery(ctx, &hostRef, specs, startTime, endTime)

	if err != nil {
		errs.AddPartial(1, err)
		return
	}
	v.processHostPerformance(metrics)
}

func (v *vcenterMetricScraper) recordVMPerformance(
	ctx context.Context,
	vm mo.VirtualMachine,
	startTime time.Time,
	endTime time.Time,
	errs *scrapererror.ScrapeErrors,
) {
	specs := []types.PerfQuerySpec{
		{
			Entity: vm.Reference(),
			MetricId: []types.PerfMetricId{
				{
					CounterId: 0,
					Instance:  "",
				},
			},
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
	v.processVMPerformanceMetrics(metrics)
}

func (v *vcenterMetricScraper) processVMPerformanceMetrics(metrics []performance.EntityMetric) {
	for _, m := range metrics {
		for i := 0; i < len(m.SampleInfo); i++ {
			val := m.Value[i]
			si := m.SampleInfo[i]
			switch strings.ToLower(val.Name) {
			// Performance monitoring level 1 metrics
			case "net.bytesTx.average":
				v.mb.RecordVcenterVMNetworkThroughputDataPoint(pdata.NewTimestampFromTime(si.Timestamp), val.Value[i], "transmitted")
			case "net.bytesRx.average":
				v.mb.RecordVcenterVMNetworkThroughputDataPoint(pdata.NewTimestampFromTime(si.Timestamp), val.Value[i], "received")
			case "cpu.usage.average":
			case "net.usage.average":
				v.mb.RecordVcenterVMNetworkUsageDataPoint(pdata.NewTimestampFromTime(si.Timestamp), val.Value[i])
			case "disk.totalReadLatency.average", "virtualDisk.totalReadLatency.average":
				v.mb.RecordVcenterVMDiskLatencyDataPoint(pdata.NewTimestampFromTime(si.Timestamp), val.Value[i], "read")
			case "disk.totalWriteLatency.average", "virtualDisk.totalWriteLatency.average":
				v.mb.RecordVcenterVMDiskLatencyDataPoint(pdata.NewTimestampFromTime(si.Timestamp), val.Value[i], "write")
			// Performance monitoring level 2 metrics
			case "":

			}
		}
	}
}

func (v *vcenterMetricScraper) processHostPerformance(metrics []performance.EntityMetric) {
	for _, m := range metrics {
		for i := 0; i < len(m.SampleInfo); i++ {
			val := m.Value[i]
			si := m.SampleInfo[i]
			switch strings.ToLower(val.Name) {
			// Performance monitoring level 1 metrics
			case "net.bytesTx.average":
				v.mb.RecordVcenterHostNetworkThroughputDataPoint(pdata.NewTimestampFromTime(si.Timestamp), val.Value[i], "transmitted")
			case "net.bytesRx.average":
				v.mb.RecordVcenterHostNetworkThroughputDataPoint(pdata.NewTimestampFromTime(si.Timestamp), val.Value[i], "received")
			case "cpu.usage.average":

			case "disk.totalReadLatency.average", "virtualDisk.totalReadLatency.average":
				v.mb.RecordVcenterHostDiskLatencyAvgDataPoint(pdata.NewTimestampFromTime(si.Timestamp), val.Value[i], "read")
			case "disk.totalWriteLatency.average", "virtualDisk.totalWriteLatency.average":
				v.mb.RecordVcenterHostDiskLatencyAvgDataPoint(pdata.NewTimestampFromTime(si.Timestamp), val.Value[i], "write")
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

func (v *vcenterMetricScraper) recordClusterVsanMetric(
	now pdata.Timestamp,
	metricID string,
	val string,
	errs *scrapererror.ScrapeErrors,
) {
	switch metricID {
	case iopsRead:
		value, err := parseInt(val)
		if err != nil {
			errs.AddPartial(1, err)
		} else {
			v.mb.RecordVcenterClusterVsanOperationsDataPoint(now, value, "read")
		}
	case iopsWrite:
		value, err := parseInt(val)
		if err != nil {
			errs.AddPartial(1, err)
		} else {
			v.mb.RecordVcenterClusterVsanOperationsDataPoint(now, value, "write")
		}
	case throughputRead:
		value, err := parseInt(val)
		if err != nil {
			errs.AddPartial(1, err)
		} else {
			v.mb.RecordVcenterClusterVsanThroughputDataPoint(now, value, "read")
		}
	case throughputWrite:
		value, err := parseInt(val)
		if err != nil {
			errs.AddPartial(1, err)
		} else {
			v.mb.RecordVcenterClusterVsanThroughputDataPoint(now, value, "write")
		}
	case latencyAvgRead:
		value, err := parseInt(val)
		if err != nil {
			errs.AddPartial(1, err)
		} else {
			v.mb.RecordVcenterClusterVsanLatencyAvgDataPoint(now, value, "read")
		}
	case latencyAvgWrite:
		value, err := parseInt(val)
		if err != nil {
			errs.AddPartial(1, err)
		} else {
			v.mb.RecordVcenterClusterVsanLatencyAvgDataPoint(now, value, "write")
		}
	case outstandingIO:
		value, err := parseInt(val)
		if err != nil {
			errs.AddPartial(1, err)
		} else {
			v.mb.RecordVcenterClusterVsanOutstandingIoDataPoint(now, value)
		}
	case congestion:
		value, err := parseInt(val)
		if err != nil {
			errs.AddPartial(1, err)
		} else {
			v.mb.RecordVcenterClusterVsanCongestionsDataPoint(now, value)
		}
	}
}

func (v *vcenterMetricScraper) recordHostVsanMetric(
	now pdata.Timestamp,
	metricID string,
	val string,
	errs *scrapererror.ScrapeErrors,
) {
	switch metricID {
	case clientCacheHits:
		value, err := parseInt(val)
		if err != nil {
			errs.AddPartial(1, err)
		} else {
			v.mb.RecordVcenterHostVsanCacheReadsDataPoint(now, value)
		}
	case clientCacheHitRate:
		value, err := parseFloat(val)
		if err != nil {
			errs.AddPartial(1, err)
		} else {
			v.mb.RecordVcenterHostVsanCacheHitRateDataPoint(now, value)
		}
	case iopsRead:
		value, err := parseInt(val)
		if err != nil {
			errs.AddPartial(1, err)
		} else {
			v.mb.RecordVcenterHostVsanOperationsDataPoint(now, value, "read")
		}
	case iopsWrite:
		value, err := parseInt(val)
		if err != nil {
			errs.AddPartial(1, err)
		} else {
			v.mb.RecordVcenterHostVsanOperationsDataPoint(now, value, "write")
		}
	case iopsUnmap:
		value, err := parseInt(val)
		if err != nil {
			errs.AddPartial(1, err)
		} else {
			v.mb.RecordVcenterHostVsanOperationsDataPoint(now, value, "unmap")
		}
	case throughputRead:
		value, err := parseInt(val)
		if err != nil {
			errs.AddPartial(1, err)
		} else {
			v.mb.RecordVcenterHostVsanThroughputDataPoint(now, value, "read")
		}
	case throughputWrite:
		value, err := parseInt(val)
		if err != nil {
			errs.AddPartial(1, err)
		} else {
			v.mb.RecordVcenterHostVsanThroughputDataPoint(now, value, "write")
		}
	case throughputUnmap:
		value, err := parseInt(val)
		if err != nil {
			errs.AddPartial(1, err)
		} else {
			v.mb.RecordVcenterHostVsanThroughputDataPoint(now, value, "unmap")
		}
	case latencyAvgRead:
		value, err := parseInt(val)
		if err != nil {
			errs.AddPartial(1, err)
		} else {
			v.mb.RecordVcenterHostVsanLatencyAvgDataPoint(now, value, "read")
		}
	case latencyAvgWrite:
		value, err := parseInt(val)
		if err != nil {
			errs.AddPartial(1, err)
		} else {
			v.mb.RecordVcenterHostVsanLatencyAvgDataPoint(now, value, "write")
		}
	case latencyAvgUnmap:
		value, err := parseInt(val)
		if err != nil {
			errs.AddPartial(1, err)
		} else {
			v.mb.RecordVcenterHostVsanLatencyAvgDataPoint(now, value, "unmap")
		}
	case outstandingIO:
		value, err := parseInt(val)
		if err != nil {
			errs.AddPartial(1, err)
		} else {
			v.mb.RecordVcenterHostVsanOutstandingIoDataPoint(now, value)
		}
	case congestion:
		value, err := parseInt(val)
		if err != nil {
			errs.AddPartial(1, err)
		} else {
			v.mb.RecordVcenterHostVsanOutstandingIoDataPoint(now, value)
		}
	}
}

func (v *vcenterMetricScraper) recordVMVsanMetric(
	now pdata.Timestamp,
	metricID string,
	val string,
	errs *scrapererror.ScrapeErrors,
) {
	switch metricID {
	case iopsRead:
		value, err := parseInt(val)
		if err != nil {
			errs.AddPartial(1, err)
		} else {
			v.mb.RecordVcenterVMVsanOperationsDataPoint(now, value, "read")
		}
	case iopsWrite:
		value, err := parseInt(val)
		if err != nil {
			errs.AddPartial(1, err)
		} else {
			v.mb.RecordVcenterVMVsanOperationsDataPoint(now, value, "write")
		}
	case "throughputRead":
		value, err := parseInt(val)
		if err != nil {
			errs.AddPartial(1, err)
		} else {
			v.mb.RecordVcenterVMVsanThroughputDataPoint(now, value, "read")
		}
	case "throughputWrite":
		value, err := parseInt(val)
		if err != nil {
			errs.AddPartial(1, err)
		} else {
			v.mb.RecordVcenterVMVsanThroughputDataPoint(now, value, "write")
		}
	case "latencyReadAvg":
		value, err := parseInt(val)
		if err != nil {
			errs.AddPartial(1, err)
		} else {
			v.mb.RecordVcenterVMVsanLatencyAvgDataPoint(now, value, "read")
		}
	case "latencyWriteAvg":
		value, err := parseInt(val)
		if err != nil {
			errs.AddPartial(1, err)
		} else {
			v.mb.RecordVcenterVMVsanLatencyAvgDataPoint(now, value, "write")
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
