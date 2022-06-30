package azuredataexplorerexporter

import (
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
)

type AdxLog struct {
	Timestamp          string                 //The timestamp of the occurance. Formatted into string as RFC3339
	ObservedTimestamp  string                 //The timestamp of logs observed in opentelemetry collector.  Formatted into string as RFC3339
	TraceId            string                 //TraceId associated to the log
	SpanId             string                 //SpanId associated to the log
	SeverityText       string                 //The severity level of the log
	SeverityNumber     int32                  //The severity number associated to the log
	Body               string                 //The body/Text of the log
	ResourceAttributes map[string]interface{} //JSON Resource attributes that can then be parsed.
	LogsAttributes     map[string]interface{} //JSON attributes that can then be parsed.
}

/*
Convert the plog to the type ADXLog, this matches the scheme in the Log table in the database
*/

func mapToAdxLog(resource pcommon.Resource, scope pcommon.InstrumentationScope, logData plog.LogRecord, logger *zap.Logger) *AdxLog {

	logattrib := logData.Attributes().AsRaw()
	clonedlogattrib := cloneMap(logattrib)
	copyAttributes(clonedlogattrib, getScopeMap(scope))
	adxLog := &AdxLog{
		Timestamp:          logData.Timestamp().AsTime().Format(time.RFC3339),
		ObservedTimestamp:  logData.ObservedTimestamp().AsTime().Format(time.RFC3339),
		TraceId:            logData.TraceID().HexString(),
		SpanId:             logData.SpanID().HexString(),
		SeverityText:       logData.SeverityText(),
		SeverityNumber:     int32(logData.SeverityNumber()),
		Body:               logData.Body().AsString(),
		ResourceAttributes: resource.Attributes().AsRaw(),
		LogsAttributes:     clonedlogattrib,
	}
	return adxLog
}
