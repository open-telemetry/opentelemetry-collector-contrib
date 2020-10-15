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

package splunkhecreceiver

import (
	"errors"
	"sort"

	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/translator/conventions"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/splunk"
)

const (
	cannotConvertValue = "cannot convert field value to attribute"
)

// SplunkHecToLogData transforms splunk events into logs
func SplunkHecToLogData(logger *zap.Logger, events []*splunk.Event, resourceCustomizer func(pdata.Resource)) (pdata.Logs, error) {
	ld := pdata.NewLogs()
	rls := ld.ResourceLogs()
	rls.Resize(len(events))

	for i, event := range events {
		rl := rls.At(i)
		rl.InitEmpty()
		logRecord := pdata.NewLogRecord()
		logRecord.InitEmpty()

		// The SourceType field is the most logical "name" of the event.
		logRecord.SetName(event.SourceType)
		logRecord.Body().InitEmpty()
		attrValue, err := convertInterfaceToAttributeValue(logger, event.Event)
		if err != nil {
			logger.Debug("Unsupported value conversion", zap.Any("value", event.Event))
			return ld, errors.New(cannotConvertValue)
		}
		attrValue.CopyTo(logRecord.Body())

		// Splunk timestamps are in seconds so convert to nanos by multiplying
		// by 1 billion.
		logRecord.SetTimestamp(pdata.TimestampUnixNano(event.Time * 1e9))

		rl.Resource().InitEmpty()
		attrs := rl.Resource().Attributes()
		attrs.InitEmptyWithCapacity(3)
		attrs.InsertString(conventions.AttributeHostHostname, event.Host)
		attrs.InsertString(conventions.AttributeServiceName, event.Source)
		attrs.InsertString(splunk.SourcetypeLabel, event.SourceType)
		resourceCustomizer(rl.Resource())
		//TODO consider setting the index field as well for pass through scenarios.
		keys := make([]string, 0, len(event.Fields))
		for k := range event.Fields {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, key := range keys {
			val := event.Fields[key]
			attrValue, err := convertInterfaceToAttributeValue(logger, val)
			if err != nil {
				return ld, err
			}
			logRecord.Attributes().Insert(key, attrValue)
		}
		ill := pdata.NewInstrumentationLibraryLogs()
		ill.InitEmpty()
		ill.Logs().Append(logRecord)
		rl.InstrumentationLibraryLogs().Append(ill)
	}

	return ld, nil
}

func convertInterfaceToAttributeValue(logger *zap.Logger, originalValue interface{}) (pdata.AttributeValue, error) {
	if originalValue == nil {
		return pdata.NewAttributeValueNull(), nil
	} else if value, ok := originalValue.(string); ok {
		return pdata.NewAttributeValueString(value), nil
	} else if value, ok := originalValue.(int64); ok {
		return pdata.NewAttributeValueInt(value), nil
	} else if value, ok := originalValue.(float64); ok {
		return pdata.NewAttributeValueDouble(value), nil
	} else if value, ok := originalValue.(bool); ok {
		return pdata.NewAttributeValueBool(value), nil
	} else if value, ok := originalValue.(map[string]interface{}); ok {
		mapContents, err := convertToAttributeMap(logger, value)
		if err != nil {
			return pdata.NewAttributeValueNull(), err
		}
		mapValue := pdata.NewAttributeValueMap()
		mapValue.SetMapVal(mapContents)
		return mapValue, nil
	} else if value, ok := originalValue.([]interface{}); ok {
		arrValue := pdata.NewAttributeValueArray()
		arrContents, err := convertToArrayVal(logger, value)
		if err != nil {
			return pdata.NewAttributeValueNull(), err
		}
		arrValue.SetArrayVal(arrContents)
		return arrValue, nil
	} else {
		logger.Debug("Unsupported value conversion", zap.Any("value", originalValue))
		return pdata.NewAttributeValueNull(), errors.New(cannotConvertValue)
	}
}

func convertToArrayVal(logger *zap.Logger, value []interface{}) (pdata.AnyValueArray, error) {
	arr := pdata.NewAnyValueArray()
	for _, elt := range value {
		translatedElt, err := convertInterfaceToAttributeValue(logger, elt)
		if err != nil {
			return arr, err
		}
		arr.Append(translatedElt)
	}
	return arr, nil
}

func convertToAttributeMap(logger *zap.Logger, value map[string]interface{}) (pdata.AttributeMap, error) {
	attrMap := pdata.NewAttributeMap()
	keys := make([]string, 0, len(value))
	for k := range value {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := value[k]
		translatedElt, err := convertInterfaceToAttributeValue(logger, v)
		if err != nil {
			return attrMap, err
		}
		attrMap.Insert(k, translatedElt)
	}
	return attrMap, nil
}
