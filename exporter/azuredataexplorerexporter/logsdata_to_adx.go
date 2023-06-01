// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuredataexplorerexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azuredataexplorerexporter"

import (
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/traceutil"
)

type AdxLog struct {
	Timestamp          string                 // The timestamp of the occurrence. Formatted into string as RFC3339Nano
	ObservedTimestamp  string                 // The timestamp of logs observed in opentelemetry collector.  Formatted into string as RFC3339Nano
	TraceID            string                 // TraceId associated to the log
	SpanID             string                 // SpanId associated to the log
	SeverityText       string                 // The severity level of the log
	SeverityNumber     int32                  // The severity number associated to the log
	Body               string                 // The body/Text of the log
	ResourceAttributes map[string]interface{} // JSON Resource attributes that can then be parsed.
	LogsAttributes     map[string]interface{} // JSON attributes that can then be parsed.
}

// Convert the plog to the type ADXLog, this matches the scheme in the Log table in the database
func mapToAdxLog(resource pcommon.Resource, scope pcommon.InstrumentationScope, logData plog.LogRecord, _ *zap.Logger) *AdxLog {

	logAttrib := logData.Attributes().AsRaw()
	clonedLogAttrib := cloneMap(logAttrib)
	copyMap(clonedLogAttrib, getScopeMap(scope))
	adxLog := &AdxLog{
		Timestamp:          logData.Timestamp().AsTime().Format(time.RFC3339Nano),
		ObservedTimestamp:  logData.ObservedTimestamp().AsTime().Format(time.RFC3339Nano),
		TraceID:            traceutil.TraceIDToHexOrEmptyString(logData.TraceID()),
		SpanID:             traceutil.SpanIDToHexOrEmptyString(logData.SpanID()),
		SeverityText:       logData.SeverityText(),
		SeverityNumber:     int32(logData.SeverityNumber()),
		Body:               logData.Body().AsString(),
		ResourceAttributes: resource.Attributes().AsRaw(),
		LogsAttributes:     clonedLogAttrib,
	}
	return adxLog
}
