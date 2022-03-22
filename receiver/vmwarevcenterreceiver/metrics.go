package vmwarevcenterreceiver

import (
	"strconv"

	"go.opentelemetry.io/collector/model/pdata"
	"go.opentelemetry.io/collector/receiver/scrapererror"
)

var iopsRead = "iopsRead"
var iopsWrite = "iopsWrite"
var throughputRead = "throughputRead"
var throughputWrite = "throughputWrite"
var latencyAvgRead = "latencyAvgRead"
var latencyAvgWrite = "latencyAvgWrite"
var outstandingIO = "oio"
var congestion = "congestion"

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
	case latencyAvgRead:
		value, err := parseInt(val)
		if err != nil {
			errs.AddPartial(1, err)
		} else {
			v.mb.RecordVcenterHostVsanLatencyAvgDataPoint(now, value, "read", hostname)
		}
	case latencyAvgWrite:
		value, err := parseInt(val)
		if err != nil {
			errs.AddPartial(1, err)
		} else {
			v.mb.RecordVcenterHostVsanLatencyAvgDataPoint(now, value, "write", hostname)
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

func parseInt(val string) (int64, error) {
	i, err := strconv.Atoi(val)
	return int64(i), err
}
