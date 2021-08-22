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

	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
)

const (
	cannotConvertValue = "cannot convert field value to attribute"
)

// splunkHecToLogData transforms splunk events into logs
func splunkHecToLogData(logger *zap.Logger, events []*splunk.Event, resourceCustomizer func(pdata.Resource), config splunk.HECConfiguration) (pdata.Logs, error) {
	ld := pdata.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	ill := rl.InstrumentationLibraryLogs().AppendEmpty()
	for _, event := range events {
		attrValue, err := convertInterfaceToAttributeValue(logger, event.Event)
		if err != nil {
			logger.Debug("Unsupported value conversion", zap.Any("value", event.Event))
			return ld, errors.New(cannotConvertValue)
		}
		logRecord := ill.Logs().AppendEmpty()
		// The SourceType field is the most logical "name" of the event.
		logRecord.SetName(event.SourceType)
		attrValue.CopyTo(logRecord.Body())

		// Splunk timestamps are in seconds so convert to nanos by multiplying
		// by 1 billion.
		if event.Time != nil {
			logRecord.SetTimestamp(pdata.Timestamp(*event.Time * 1e9))
		}

		if event.Host != "" {
			logRecord.Attributes().InsertString(config.GetHostKey(), event.Host)
		}
		if event.Source != "" {
			logRecord.Attributes().InsertString(config.GetSourceKey(), event.Source)
		}
		if event.SourceType != "" {
			logRecord.Attributes().InsertString(config.GetSourceTypeKey(), event.SourceType)
		}
		if event.Index != "" {
			logRecord.Attributes().InsertString(config.GetIndexKey(), event.Index)
		}
		resourceCustomizer(rl.Resource())
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
		mapValue, err := convertToAttributeMap(logger, value)
		if err != nil {
			return pdata.NewAttributeValueNull(), err
		}
		return mapValue, nil
	} else if value, ok := originalValue.([]interface{}); ok {
		arrValue, err := convertToArrayVal(logger, value)
		if err != nil {
			return pdata.NewAttributeValueNull(), err
		}
		return arrValue, nil
	} else {
		logger.Debug("Unsupported value conversion", zap.Any("value", originalValue))
		return pdata.NewAttributeValueNull(), errors.New(cannotConvertValue)
	}
}

func convertToArrayVal(logger *zap.Logger, value []interface{}) (pdata.AttributeValue, error) {
	attrVal := pdata.NewAttributeValueArray()
	arr := attrVal.ArrayVal()
	for _, elt := range value {
		translatedElt, err := convertInterfaceToAttributeValue(logger, elt)
		if err != nil {
			return attrVal, err
		}
		tgt := arr.AppendEmpty()
		translatedElt.CopyTo(tgt)
	}
	return attrVal, nil
}

func convertToAttributeMap(logger *zap.Logger, value map[string]interface{}) (pdata.AttributeValue, error) {
	attrVal := pdata.NewAttributeValueMap()
	attrMap := attrVal.MapVal()
	keys := make([]string, 0, len(value))
	for k := range value {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := value[k]
		translatedElt, err := convertInterfaceToAttributeValue(logger, v)
		if err != nil {
			return attrVal, err
		}
		attrMap.Insert(k, translatedElt)
	}
	return attrVal, nil
}
