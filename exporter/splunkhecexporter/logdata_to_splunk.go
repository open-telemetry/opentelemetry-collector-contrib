// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package splunkhecexporter

import (
	"time"

	"go.opentelemetry.io/collector/model/pdata"
	conventions "go.opentelemetry.io/collector/model/semconv/v1.5.0"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
)

const (
	// Keys are taken from https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/logs/overview.md#trace-context-in-legacy-formats.
	// spanIDFieldKey is the key used in log event for the span id (if any).
	spanIDFieldKey = "span_id"
	// traceIDFieldKey is the key used in the log event for the trace id (if any).
	traceIDFieldKey = "trace_id"
)

// Composite index of a log record in pdata.Logs.
type logIndex struct {
	// Index in orig list (i.e. root parent index).
	resource int
	// Index in InstrumentationLibraryLogs list (i.e. immediate parent index).
	library int
	// Index in Logs list (i.e. the log record index).
	record int
}

func mapLogRecordToSplunkEvent(res pdata.Resource, lr pdata.LogRecord, config *Config, logger *zap.Logger) *splunk.Event {
	host := unknownHostName
	source := config.Source
	sourcetype := config.SourceType
	index := config.Index
	fields := map[string]interface{}{}
	sourceKey := config.HecToOtelAttrs.Source
	sourceTypeKey := config.HecToOtelAttrs.SourceType
	indexKey := config.HecToOtelAttrs.Index
	hostKey := config.HecToOtelAttrs.Host
	nameKey := config.HecFields.Name
	severityTextKey := config.HecFields.SeverityText
	severityNumberKey := config.HecFields.SeverityNumber
	if lr.Name() != "" {
		fields[nameKey] = lr.Name()
	}
	if spanID := lr.SpanID().HexString(); spanID != "" {
		fields[spanIDFieldKey] = spanID
	}
	if traceID := lr.TraceID().HexString(); traceID != "" {
		fields[traceIDFieldKey] = traceID
	}
	if lr.SeverityText() != "" {
		fields[severityTextKey] = lr.SeverityText()
	}
	if lr.SeverityNumber() != pdata.SeverityNumberUNDEFINED {
		fields[severityNumberKey] = lr.SeverityNumber()
	}

	res.Attributes().Range(func(k string, v pdata.AttributeValue) bool {
		switch k {
		case hostKey:
			host = v.StringVal()
			fields[conventions.AttributeHostName] = v.StringVal()
		case sourceKey:
			source = v.StringVal()
		case sourceTypeKey:
			sourcetype = v.StringVal()
		case indexKey:
			index = v.StringVal()
		default:
			fields[k] = convertAttributeValue(v, logger)
		}
		return true
	})
	lr.Attributes().Range(func(k string, v pdata.AttributeValue) bool {
		switch k {
		case hostKey:
			host = v.StringVal()
			fields[conventions.AttributeHostName] = v.StringVal()
		case sourceKey:
			source = v.StringVal()
		case sourceTypeKey:
			sourcetype = v.StringVal()
		case indexKey:
			index = v.StringVal()
		default:
			fields[k] = convertAttributeValue(v, logger)
		}
		return true
	})

	eventValue := convertAttributeValue(lr.Body(), logger)
	return &splunk.Event{
		Time:       nanoTimestampToEpochMilliseconds(lr.Timestamp()),
		Host:       host,
		Source:     source,
		SourceType: sourcetype,
		Index:      index,
		Event:      eventValue,
		Fields:     fields,
	}
}

func convertAttributeValue(value pdata.AttributeValue, logger *zap.Logger) interface{} {
	switch value.Type() {
	case pdata.AttributeValueTypeInt:
		return value.IntVal()
	case pdata.AttributeValueTypeBool:
		return value.BoolVal()
	case pdata.AttributeValueTypeDouble:
		return value.DoubleVal()
	case pdata.AttributeValueTypeString:
		return value.StringVal()
	case pdata.AttributeValueTypeMap:
		values := map[string]interface{}{}
		value.MapVal().Range(func(k string, v pdata.AttributeValue) bool {
			values[k] = convertAttributeValue(v, logger)
			return true
		})
		return values
	case pdata.AttributeValueTypeArray:
		arrayVal := value.ArrayVal()
		values := make([]interface{}, arrayVal.Len())
		for i := 0; i < arrayVal.Len(); i++ {
			values[i] = convertAttributeValue(arrayVal.At(i), logger)
		}
		return values
	case pdata.AttributeValueTypeEmpty:
		return nil
	default:
		logger.Debug("Unhandled value type", zap.String("type", value.Type().String()))
		return value
	}
}

// nanoTimestampToEpochMilliseconds transforms nanoseconds into <sec>.<ms>. For example, 1433188255.500 indicates 1433188255 seconds and 500 milliseconds after epoch.
func nanoTimestampToEpochMilliseconds(ts pdata.Timestamp) *float64 {
	duration := time.Duration(ts)
	if duration == 0 {
		// some telemetry sources send data with timestamps set to 0 by design, as their original target destinations
		// (i.e. before Open Telemetry) are setup with the know-how on how to consume them. In this case,
		// we want to omit the time field when sending data to the Splunk HEC so that the HEC adds a timestamp
		// at indexing time, which will be much more useful than a 0-epoch-time value.
		return nil
	}

	val := duration.Round(time.Millisecond).Seconds()
	return &val
}
