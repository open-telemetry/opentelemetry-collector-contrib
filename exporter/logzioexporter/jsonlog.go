// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logzioexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/logzioexporter"

import (
	"encoding/hex"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

// convertLogRecordToJSON Takes `plog.LogRecord` and `pcommon.Resource` input, outputs byte array that represents the log record as json string
func convertLogRecordToJSON(log plog.LogRecord, attributes pcommon.Map) map[string]any {
	jsonLog := map[string]any{}
	if spanID := log.SpanID(); !spanID.IsEmpty() {
		jsonLog["spanID"] = hex.EncodeToString(spanID[:])
	}
	if traceID := log.TraceID(); !traceID.IsEmpty() {
		jsonLog["traceID"] = hex.EncodeToString(traceID[:])
	}
	if log.SeverityText() != "" {
		jsonLog["level"] = log.SeverityText()
	}
	// try to set timestamp if exists
	if log.Timestamp().AsTime().UnixMilli() != 0 {
		jsonLog["@timestamp"] = log.Timestamp().AsTime().UnixMilli()
	}

	// Add merged attributed to each json log
	attributes.Range(func(k string, v pcommon.Value) bool {
		jsonLog[k] = v.AsRaw()
		return true
	})

	switch log.Body().Type() {
	case pcommon.ValueTypeStr:
		jsonLog["message"] = log.Body().Str()
	case pcommon.ValueTypeMap:
		bodyFieldsMap := log.Body().Map().AsRaw()
		for key, value := range bodyFieldsMap {
			jsonLog[key] = value
		}
	}
	return jsonLog
}
