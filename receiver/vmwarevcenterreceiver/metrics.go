package vmwarevcenterreceiver

import (
	"strconv"

	"github.com/vmware/govmomi/vim25/mo"
	"go.opentelemetry.io/collector/model/pdata"
	"go.opentelemetry.io/collector/receiver/scrapererror"
)

func (v *vmwareVcenterScraper) recordHostSystemMemoryUsage(
	now pdata.Timestamp,
	hs mo.HostSystem,
	hostname string,
	errs *scrapererror.ScrapeErrors,
) {
	totalMemory := hs.Summary.Hardware.MemorySize
	memUsage := hs.Summary.QuickStats.OverallMemoryUsage
	v.mb.RecordVcenterHostMemoryUsageDataPoint(now, totalMemory, hostname)

	memUtilization := float64(int64(memUsage) * 1024 * 1024 / totalMemory)
	v.mb.RecordVcenterHostMemoryUtilizationDataPoint(now, memUtilization, hostname)
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
	val string,
	errs *scrapererror.ScrapeErrors,
) {
	switch metricID {
	case iopsRead:
		value, err := parseInt(val)
		if err != nil {
			errs.AddPartial(1, err)
		} else {
			v.mb.RecordVcenterVMVsanOperationsDataPoint(now, value, instanceName, "read")
		}
	case iopsWrite:
		value, err := parseInt(val)
		if err != nil {
			errs.AddPartial(1, err)
		} else {
			v.mb.RecordVcenterVMVsanOperationsDataPoint(now, value, instanceName, "write")
		}
	case throughputRead:
		value, err := parseInt(val)
		if err != nil {
			errs.AddPartial(1, err)
		} else {
			v.mb.RecordVcenterVMVsanThroughputDataPoint(now, value, instanceName, "read")
		}
	case throughputWrite:
		value, err := parseInt(val)
		if err != nil {
			errs.AddPartial(1, err)
		} else {
			v.mb.RecordVcenterVMVsanThroughputDataPoint(now, value, instanceName, "write")
		}
	case latencyAvgRead:
		value, err := parseInt(val)
		if err != nil {
			errs.AddPartial(1, err)
		} else {
			v.mb.RecordVcenterVMVsanLatencyAvgDataPoint(now, value, instanceName, "read")
		}
	case latencyAvgWrite:
		value, err := parseInt(val)
		if err != nil {
			errs.AddPartial(1, err)
		} else {
			v.mb.RecordVcenterVMVsanLatencyAvgDataPoint(now, value, instanceName, "write")
		}
	case outstandingIO:
		value, err := parseInt(val)
		if err != nil {
			errs.AddPartial(1, err)
		} else {
			v.mb.RecordVcenterVMVsanOutstandingIoDataPoint(now, value, instanceName)
		}
	case congestion:
		value, err := parseInt(val)
		if err != nil {
			errs.AddPartial(1, err)
		} else {
			v.mb.RecordVcenterVMVsanCongestionsDataPoint(now, value, instanceName)
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
