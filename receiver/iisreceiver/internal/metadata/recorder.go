package metadata

import (
	"go.opentelemetry.io/collector/model/pdata"
)

func (mb *MetricsBuilder) RecordAny(ts pdata.Timestamp, val int64, name string, attributes map[string]string) {
	switch name {
	case "iis.connection.active":
		mb.RecordIisConnectionActiveDataPoint(ts, val)
	case "iis.connection.anonymous":
		mb.RecordIisConnectionAnonymousDataPoint(ts, val)
	case "iis.connection.attempt.count":
		mb.RecordIisConnectionAttemptCountDataPoint(ts, val)
	case "iis.network.blocked":
		mb.RecordIisNetworkBlockedDataPoint(ts, val)
	case "iis.network.file.count":
		mb.RecordIisNetworkFileCountDataPoint(ts, val, attributes["direction"])
	case "iis.network.io":
		mb.RecordIisNetworkIoDataPoint(ts, val, attributes["direction"])
	case "iis.request.count":
		mb.RecordIisRequestCountDataPoint(ts, val, attributes["request"])
	case "iis.request.queue.age.max":
		mb.RecordIisRequestQueueAgeMaxDataPoint(ts, val)
	case "iis.request.queue.count":
		mb.RecordIisRequestQueueCountDataPoint(ts, val)
	case "iis.request.rejected":
		mb.RecordIisRequestRejectedDataPoint(ts, val)
	case "iis.thread.active":
		mb.RecordIisThreadActiveDataPoint(ts, val)
	case "iis.uptime":
		mb.RecordIisUptimeDataPoint(ts, val)
	}

}
