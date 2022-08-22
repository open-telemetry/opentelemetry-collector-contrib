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

package logzioexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/logzioexporter"

import (
	"github.com/hashicorp/go-hclog"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
)

func convertAttributeValue(value pcommon.Value, logger hclog.Logger) interface{} {
	switch value.Type() {
	case pcommon.ValueTypeInt:
		return value.IntVal()
	case pcommon.ValueTypeBool:
		return value.BoolVal()
	case pcommon.ValueTypeDouble:
		return value.DoubleVal()
	case pcommon.ValueTypeString:
		return value.StringVal()
	case pcommon.ValueTypeMap:
		values := map[string]interface{}{}
		value.MapVal().Range(func(k string, v pcommon.Value) bool {
			values[k] = convertAttributeValue(v, logger)
			return true
		})
		return values
	case pcommon.ValueTypeSlice:
		arrayVal := value.SliceVal()
		values := make([]interface{}, arrayVal.Len())
		for i := 0; i < arrayVal.Len(); i++ {
			values[i] = convertAttributeValue(arrayVal.At(i), logger)
		}
		return values
	case pcommon.ValueTypeEmpty:
		return nil
	default:
		logger.Debug("Unhandled value type", zap.String("type", value.Type().String()))
		return value
	}
}

// convertLogRecordToJSON Takes `plog.LogRecord` and `pcommon.Resource` input, outputs byte array that represents the log record as json string
func convertLogRecordToJSON(log plog.LogRecord, resource pcommon.Resource, logger hclog.Logger) map[string]interface{} {
	jsonLog := map[string]interface{}{}
	if spanID := log.SpanID().HexString(); spanID != "" {
		jsonLog["spanID"] = spanID
	}
	if traceID := log.TraceID().HexString(); traceID != "" {
		jsonLog["traceID"] = traceID
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
		jsonLog[k] = convertAttributeValue(v, logger)
		return true
	})
	// add log record attributes to each json log
	log.Attributes().Range(func(k string, v pcommon.Value) bool {
		jsonLog[k] = convertAttributeValue(v, logger)
		return true
	})

	switch log.Body().Type() {
	case pcommon.ValueTypeString:
		jsonLog["message"] = log.Body().StringVal()
	case pcommon.ValueTypeMap:
		bodyFieldsMap := convertAttributeValue(log.Body(), logger).(map[string]interface{})
		for key, value := range bodyFieldsMap {
			jsonLog[key] = value
		}
	}
	return jsonLog
}
