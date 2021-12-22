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

//go:build !windows
// +build !windows

package podmanreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/podmanreceiver"

import (
	"errors"
	"fmt"
	"sort"

	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"
)

func translateEventsToLogs(logger *zap.Logger, event event) (pdata.Logs, error) {
	ld := pdata.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	ill := rl.InstrumentationLibraryLogs().AppendEmpty()

	logRecord := ill.Logs().AppendEmpty()
	logRecord.SetName(event.Action)
	logRecord.SetTimestamp(pdata.Timestamp(event.TimeNano))
	logRecord.Attributes().InsertString("contianer.id", event.ID)

	bodyString := fmt.Sprintf("podman %s ( %s ) %s", event.Type, event.Actor.Attributes["name"], event.Action)
	body := pdata.NewAttributeValueString(bodyString)
	body.CopyTo(logRecord.Body())

	keys := make([]string, 0, len(event.Actor.Attributes))
	for k := range event.Actor.Attributes {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, key := range keys {
		val := event.Actor.Attributes[key]
		attrValue, err := convertInterfaceToAttributeValue(logger, val)
		if err != nil {
			return pdata.NewLogs(), err
		}
		if key == "image" {
			logRecord.Attributes().Insert("container.image.name", attrValue)
		}
		if key == "name" {
			logRecord.Attributes().Insert("container.name", attrValue)
		}
	}
	return ld, nil
}

func convertInterfaceToAttributeValue(logger *zap.Logger, originalValue interface{}) (pdata.AttributeValue, error) {
	if originalValue == nil {
		return pdata.NewAttributeValueEmpty(), nil
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
			return pdata.NewAttributeValueEmpty(), err
		}
		return mapValue, nil
	} else if value, ok := originalValue.([]interface{}); ok {
		arrValue, err := convertToSliceVal(logger, value)
		if err != nil {
			return pdata.NewAttributeValueEmpty(), err
		}
		return arrValue, nil
	} else {
		logger.Debug("Unsupported value conversion", zap.Any("value", originalValue))
		return pdata.NewAttributeValueEmpty(), errors.New("cannot convert field value to attribute")
	}
}

func convertToSliceVal(logger *zap.Logger, value []interface{}) (pdata.AttributeValue, error) {
	attrVal := pdata.NewAttributeValueArray()
	arr := attrVal.SliceVal()
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
