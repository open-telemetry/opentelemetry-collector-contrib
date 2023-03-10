// Copyright The OpenTelemetry Authors
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

package logzioexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/logzioexporter"

import (
	"encoding/hex"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

// convertLogRecordToJSON Takes `plog.LogRecord` and `pcommon.Resource` input, outputs byte array that represents the log record as json string
func convertLogRecordToJSON(log plog.LogRecord, resource pcommon.Resource) map[string]interface{} {
	jsonLog := map[string]interface{}{}
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
	// add resource attributes to each json log
	resource.Attributes().Range(func(k string, v pcommon.Value) bool {
		jsonLog[k] = v.AsRaw()
		return true
	})
	// add log record attributes to each json log
	log.Attributes().Range(func(k string, v pcommon.Value) bool {
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
