// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package vmwarevcenterreceiver

import (
	"context"
	"strconv"
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
	"disk.read.average",
}

func (v *vcenterMetricScraper) recordHostPerformanceMetrics(
	ctx context.Context,
	host mo.HostSystem,
	errs *scrapererror.ScrapeErrors,
) {
	st := time.Now().Add(-5 * time.Minute)
	et := time.Now().Add(-1 * time.Minute)
	spec := types.PerfQuerySpec{
		Entity:    host.Reference(),
		MaxSample: 5,
		Format:    string(types.PerfFormatNormal),
		MetricId:  []types.PerfMetricId{{Instance: "*"}},
		// right now we are only grabbing real time metrics from the performance
		// manager
		StartTime: &st,
		EndTime:   &et,
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
	"disk.write.average",
	"disk.totalWriteLatency.average",
	"virtualDisk.totalWriteLatency.average",
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
					v.mb.RecordVcenterVMNetworkThroughputDataPoint(pdata.NewTimestampFromTime(si.Timestamp), nestedValue, "transmitted")
				case "net.bytesRx.average":
					v.mb.RecordVcenterVMNetworkThroughputDataPoint(pdata.NewTimestampFromTime(si.Timestamp), nestedValue, "received")
				case "net.usage.average":
					v.mb.RecordVcenterVMNetworkUsageDataPoint(pdata.NewTimestampFromTime(si.Timestamp), nestedValue)
				case "net.packetsTx.summation":
					v.mb.RecordVcenterVMNetworkPacketsDataPoint(pdata.NewTimestampFromTime(si.Timestamp), nestedValue, "transmitted")
				case "net.packetsRx.summation":
					v.mb.RecordVcenterVMNetworkPacketsDataPoint(pdata.NewTimestampFromTime(si.Timestamp), nestedValue, "received")

				// Performance monitoring level 2 metrics required
				case "disk.totalReadLatency.average", "virtualDisk.totalReadLatency.average":
					v.mb.RecordVcenterVMDiskLatencyAvgDataPoint(pdata.NewTimestampFromTime(si.Timestamp), nestedValue, "read")
				case "disk.totalWriteLatency.average", "virtualDisk.totalWriteLatency.average":
					v.mb.RecordVcenterVMDiskLatencyAvgDataPoint(pdata.NewTimestampFromTime(si.Timestamp), nestedValue, "write")
				case "disk.maxTotalLatency":
					v.mb.RecordVcenterVMDiskLatencyMaxDataPoint(pdata.NewTimestampFromTime(si.Timestamp), nestedValue)
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
					v.mb.RecordVcenterHostNetworkUsageDataPoint(pdata.NewTimestampFromTime(si.Timestamp), nestedValue)
				case "net.bytesTx.average":
					v.mb.RecordVcenterHostNetworkThroughputDataPoint(pdata.NewTimestampFromTime(si.Timestamp), nestedValue, "transmitted")
				case "net.bytesRx.average":
					v.mb.RecordVcenterHostNetworkThroughputDataPoint(pdata.NewTimestampFromTime(si.Timestamp), nestedValue, "received")
				// case "net.bytesTx.average":
				case "net.packetsTx.summation":
					v.mb.RecordVcenterHostNetworkPacketsDataPoint(pdata.NewTimestampFromTime(si.Timestamp), nestedValue, "transmitted")
				case "net.packetsRx.summation":
					v.mb.RecordVcenterHostNetworkPacketsDataPoint(pdata.NewTimestampFromTime(si.Timestamp), nestedValue, "received")

				// Following requires performance level 2
				case "net.errorsRx.summation":
					v.mb.RecordVcenterHostNetworkErrorsDataPoint(pdata.NewTimestampFromTime(si.Timestamp), nestedValue, "received")
				case "net.errorsTx.summation":
					v.mb.RecordVcenterHostNetworkErrorsDataPoint(pdata.NewTimestampFromTime(si.Timestamp), nestedValue, "transmitted")

				case "disk.kernelReadLatency.average":
					v.mb.RecordVcenterHostDiskLatencyAvgDataPoint(pdata.NewTimestampFromTime(si.Timestamp), nestedValue, "read", "kernel")
				case "disk.kernelWriteLatency.average":
					v.mb.RecordVcenterHostDiskLatencyAvgDataPoint(pdata.NewTimestampFromTime(si.Timestamp), nestedValue, "write", "device")
				case "disk.totalWriteLatency.average":
					v.mb.RecordVcenterHostDiskLatencyTotalDataPoint(pdata.NewTimestampFromTime(si.Timestamp), nestedValue, "write")
				case "disk.totalReadLatency.average":
					v.mb.RecordVcenterHostDiskLatencyTotalDataPoint(pdata.NewTimestampFromTime(si.Timestamp), nestedValue, "read")
				}
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
	case throughputRead:
		value, err := parseInt(val)
		if err != nil {
			errs.AddPartial(1, err)
		} else {
			v.mb.RecordVcenterVMVsanThroughputDataPoint(now, value, "read")
		}
	case throughputWrite:
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
