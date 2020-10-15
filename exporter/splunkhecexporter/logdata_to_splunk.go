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

	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/translator/conventions"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/splunk"
)

func logDataToSplunk(logger *zap.Logger, ld pdata.Logs, config *Config) ([]*splunk.Event, int) {
	numDroppedLogs := 0
	splunkEvents := make([]*splunk.Event, 0)
	rls := ld.ResourceLogs()
	for i := 0; i < rls.Len(); i++ {
		rl := rls.At(i)
		if rl.IsNil() {
			continue
		}

		ills := rl.InstrumentationLibraryLogs()
		for j := 0; j < ills.Len(); j++ {
			ils := ills.At(j)
			if ils.IsNil() {
				continue
			}

			logs := ils.Logs()
			for j := 0; j < logs.Len(); j++ {
				lr := logs.At(j)
				if lr.IsNil() {
					continue
				}
				ev := mapLogRecordToSplunkEvent(lr, config, logger)
				if ev == nil {
					numDroppedLogs++
				} else {
					splunkEvents = append(splunkEvents, ev)
				}
			}
		}
	}

	return splunkEvents, numDroppedLogs
}

func mapLogRecordToSplunkEvent(lr pdata.LogRecord, config *Config, logger *zap.Logger) *splunk.Event {
	if lr.Body().IsNil() {
		return nil
	}
	var host string
	var source string
	var sourcetype string
	fields := map[string]interface{}{}
	lr.Attributes().ForEach(func(k string, v pdata.AttributeValue) {
		if v.Type() != pdata.AttributeValueSTRING {
			logger.Debug("Failed to convert log record attribute value to Splunk property value, value is not a string", zap.String("key", k))
			return
		}
		if k == conventions.AttributeHostHostname {
			host = v.StringVal()
		} else if k == conventions.AttributeServiceName {
			source = v.StringVal()
		} else if k == splunk.SourcetypeLabel {
			sourcetype = v.StringVal()
		} else {
			fields[k] = v.StringVal()
		}
	})

	if host == "" {
		host = unknownHostName
	}

	if source == "" {
		source = config.Source
	}

	if sourcetype == "" {
		sourcetype = config.SourceType
	}

	eventValue := convertAttributeValue(lr.Body(), logger)
	return &splunk.Event{
		Time:       nanoTimestampToEpochMilliseconds(lr.Timestamp()),
		Host:       host,
		Source:     source,
		SourceType: sourcetype,
		Index:      config.Index,
		Event:      eventValue,
		Fields:     fields,
	}
}

func convertAttributeValue(value pdata.AttributeValue, logger *zap.Logger) interface{} {
	switch value.Type() {
	case pdata.AttributeValueINT:
		return value.IntVal()
	case pdata.AttributeValueBOOL:
		return value.BoolVal()
	case pdata.AttributeValueDOUBLE:
		return value.DoubleVal()
	case pdata.AttributeValueSTRING:
		return value.StringVal()
	case pdata.AttributeValueMAP:
		values := map[string]interface{}{}
		value.MapVal().ForEach(func(k string, v pdata.AttributeValue) {
			values[k] = convertAttributeValue(v, logger)
		})
		return values
	case pdata.AttributeValueARRAY:
		arrayVal := value.ArrayVal()
		values := make([]interface{}, arrayVal.Len())
		for i := 0; i < arrayVal.Len(); i++ {
			values[i] = convertAttributeValue(arrayVal.At(i), logger)
		}
		return values
	case pdata.AttributeValueNULL:
		return nil
	default:
		logger.Debug("Unhandled value type", zap.String("type", value.Type().String()))
		return value
	}
}

// nanoTimestampToEpochMilliseconds transforms nanoseconds into <sec>.<ms>. For example, 1433188255.500 indicates 1433188255 seconds and 500 milliseconds after epoch.
func nanoTimestampToEpochMilliseconds(ts pdata.TimestampUnixNano) float64 {
	return time.Duration(ts).Round(time.Millisecond).Seconds()
}
