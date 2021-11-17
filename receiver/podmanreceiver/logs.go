package podmanreceiver

import (
	"errors"
	"sort"

	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"
)

func traslateEventsToLogs(logger *zap.Logger, event Event) (pdata.Logs, error) {
	ld := pdata.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	ill := rl.InstrumentationLibraryLogs().AppendEmpty()

	logRecord := ill.Logs().AppendEmpty()
	logRecord.SetName(event.Action)
	logRecord.SetTimestamp(pdata.Timestamp(event.TimeNano))
	logRecord.Attributes().InsertString("contianer.id", event.ID)

	body, err := convertInterfaceToAttributeValue(logger, "podman "+event.Type+"("+event.Actor.Attributes["name"]+") "+event.Action)
	if err != nil {
		return pdata.NewLogs(), err
	}
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
