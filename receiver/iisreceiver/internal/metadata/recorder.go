package metadata

import (
	"go.opentelemetry.io/collector/model/pdata"
)

func (mb *MetricsBuilder) RecordAny(ts pdata.Timestamp, val float64, name string, attributes map[string]string) {
	switch name {
	case "iis.connection.active":
		mb.RecordIisConnectionActiveDataPoint(ts, int64(val))
	case "iis.connection.anonymous":
		mb.RecordIisConnectionAnonymousDataPoint(ts, int64(val))
	case "iis.connection.attempt.count":
		mb.RecordIisConnectionAttemptCountDataPoint(ts, int64(val))
	case "iis.network.blocked":
		mb.RecordIisNetworkBlockedDataPoint(ts, int64(val))
	case "iis.network.file.count":
		mb.RecordIisNetworkFileCountDataPoint(ts, int64(val), attributes["direction"])
	case "iis.network.io":
		mb.RecordIisNetworkIoDataPoint(ts, int64(val), attributes["direction"])
	case "iis.request.count":
		mb.RecordIisRequestCountDataPoint(ts, int64(val), attributes["request"])
	case "iis.request.queue.age.max":
		mb.RecordIisRequestQueueAgeMaxDataPoint(ts, int64(val))
	case "iis.request.queue.count":
		mb.RecordIisRequestQueueCountDataPoint(ts, int64(val))
	case "iis.request.rejected":
		mb.RecordIisRequestRejectedDataPoint(ts, int64(val))
	case "iis.thread.active":
		mb.RecordIisThreadActiveDataPoint(ts, int64(val))
	case "iis.uptime":
		mb.RecordIisUptimeDataPoint(ts, int64(val))
	}
}
