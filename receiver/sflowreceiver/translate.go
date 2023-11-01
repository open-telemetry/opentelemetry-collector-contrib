// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sflowreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sflowreceiver"

import (
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
)

type Itranslator interface{}

type Translator struct {
	Logger *zap.Logger
}

func (t *Translator) SflowToOtelLogs(sflowData *SFlowData, config *Config) plog.Logs {
	logs := plog.NewLogs()
	rls := logs.ResourceLogs().AppendEmpty()
	logSlice := rls.ScopeLogs().AppendEmpty().LogRecords()
	logSlice.EnsureCapacity(1)

	m := make(map[string]interface{}, 0)
	b, err := json.Marshal(sflowData)
	if err != nil {
		t.Logger.Error("Translate error", zap.Error(err))
		return logs
	}

	err = json.Unmarshal(b, &m)
	if err != nil {
		t.Logger.Error("Translate error", zap.Error(err))
		return logs
	}

	flattened := flattenJSON2(m)

	log := logSlice.AppendEmpty()
	t.parseRecordToLogRecord(flattened, log, config)

	return logs
}

func (t *Translator) parseRecordToLogRecord(flow map[string]interface{}, log plog.LogRecord, config *Config) {
	parseToAttributesValue(flow, log.Attributes())

	// Add Labels to log record
	if config.Labels != nil {
		for k, v := range config.Labels {
			log.Attributes().PutStr(k, v)
		}
	}

	log.SetObservedTimestamp(pcommon.NewTimestampFromTime(time.Now()))

	if _, exist := flow["timestamp"]; exist {
		ts, err := timeFromTimestamp(flow["timestamp"])
		if err != nil {
			t.Logger.Error("unknown timestamp", zap.Error(err))
		}
		log.SetTimestamp(pcommon.NewTimestampFromTime(ts))
	}
}

func parseToAttributesValue(m map[string]interface{}, dest pcommon.Map) {
	for key := range m {
		if val, ok := m[key]; ok {
			parseToAttributesToValue(key, val, dest)
		}
	}
}

func parseToAttributesToValue(key string, val interface{}, dest pcommon.Map) {
	switch r := val.(type) {
	case bool:
		dest.PutBool(key, r)
	case string:
		dest.PutStr(key, r)
	case []byte:
		dest.PutStr(key, string(r))
	case int:
		dest.PutInt(key, int64(r))
	case int8:
		dest.PutInt(key, int64(r))
	case int16:
		dest.PutInt(key, int64(r))
	case int32:
		dest.PutInt(key, int64(r))
	case int64:
		dest.PutInt(key, r)
	case uint:
		dest.PutInt(key, int64(r))
	case uint8:
		dest.PutInt(key, int64(r))
	case uint16:
		dest.PutInt(key, int64(r))
	case uint32:
		dest.PutInt(key, int64(r))
	case uint64:
		dest.PutInt(key, int64(r))
	case float32:
		dest.PutDouble(key, float64(r))
	case float64:
		dest.PutDouble(key, r)
	case []interface{}:
		es := dest.PutEmptySlice(key)
		es.EnsureCapacity(len(r))
		for _, v := range r {
			parseToAttributesToValue(key, v, es.AppendEmpty().Map())
		}
	case map[string]interface{}:
		em := dest.PutEmptyMap(key)
		em.EnsureCapacity(len(r))
		for _, v := range r {
			parseToAttributesToValue(key, v, em.PutEmptyMap(key))
		}
	case nil:
	default:
		dest.PutStr(key, fmt.Sprintf("%v", r))
	}
}

func parseToBodyValue(val interface{}, dest pcommon.Value) {
	switch r := val.(type) {
	case bool:
		dest.SetBool(r)
	case string:
		dest.SetStr(r)
	case []byte:
		dest.SetStr(string(r))
	case int64:
		dest.SetInt(r)
	case uint64:
		dest.SetInt(int64(r))
	case float32:
		dest.SetDouble(float64(r))
	case float64:
		dest.SetDouble(r)
	case map[string]interface{}:
		parseInterfaceToMap(r, dest)
	case []interface{}:
		parseInterfaceToArray(r, dest)
	case nil:
	default:
		dest.SetStr(fmt.Sprintf("%v", val))
	}
}

func timeFromTimestamp(ts interface{}) (time.Time, error) {
	switch v := ts.(type) {
	case uint64:
		return time.Unix(int64(v), 0), nil
	case int64:
		return time.Unix(v, 0), nil
	case int:
		return time.Unix(int64(v), 0), nil
	case float64:
		return time.UnixMilli(int64(v)), nil
	default:
		return time.Time{}, fmt.Errorf("unknown type of value: %v", ts)
	}
}

func parseInterfaceToMap(m map[string]interface{}, dest pcommon.Value) {
	am := dest.SetEmptyMap()
	am.EnsureCapacity(len(m))
	for k, value := range m {
		parseToBodyValue(value, am.PutEmpty(k))
	}
}

func parseInterfaceToArray(a []interface{}, dest pcommon.Value) {
	av := dest.SetEmptySlice()
	av.EnsureCapacity(len(a))
	for _, value := range a {
		parseToBodyValue(value, av.AppendEmpty())
	}
}

func flattenStruct(input interface{}, separator string) map[string]interface{} {
	output := make(map[string]interface{})
	flattenStructHelper(reflect.ValueOf(input), "", separator, output)
	return output
}

func flattenStructHelper(value reflect.Value, prefix, separator string, output map[string]interface{}) {
	switch value.Kind() {
	case reflect.Struct:
		for i := 0; i < value.NumField(); i++ {
			field := value.Field(i)
			fieldName := value.Type().Field(i).Name
			newKey := fieldName

			if prefix != "" {
				newKey = prefix + separator + newKey
			}

			flattenStructHelper(field, newKey, separator, output)
		}
	case reflect.Map:
		for _, key := range value.MapKeys() {
			field := value.MapIndex(key)
			keyName := key.Interface().(string)
			newKey := keyName

			if prefix != "" {
				newKey = prefix + separator + newKey
			}

			flattenStructHelper(field, newKey, separator, output)
		}
	default:
		output[prefix] = value.Interface()
	}
}

func flattenJSON(input map[string]interface{}, separator string) map[string]interface{} {
	output := make(map[string]interface{})
	flattenHelper(input, "", separator, output)
	return output
}

func flattenHelper(input interface{}, prefix, separator string, output map[string]interface{}) {
	switch val := input.(type) {
	case map[string]interface{}:
		for key, value := range val {
			newKey := key
			if prefix != "" {
				newKey = prefix + separator + newKey
			}
			flattenHelper(value, newKey, separator, output)
		}
	default:
		output[prefix] = val
	}
}

func flattenJSON2(jsonObj map[string]interface{}) map[string]interface{} {
	m := make(map[string]interface{}, 0)
	for k, v := range jsonObj {
		flatten(k, v, m)
	}
	return m
}

func flatten(key string, jsonObj interface{}, m map[string]interface{}) {
	switch jsonObj.(type) {
	case map[string]interface{}:
		for k, v := range jsonObj.(map[string]interface{}) {
			newkey := fmt.Sprintf("%s.%s", key, k)
			flatten(newkey, v, m)
		}
	case []interface{}:
		for i, v := range jsonObj.([]interface{}) {
			newkey := fmt.Sprintf("%s[%d]", key, i)
			flatten(newkey, v, m)
		}
	case int8:
		m[key] = jsonObj.(int)
	case int16:
		m[key] = jsonObj.(int)
	case int32:
		m[key] = jsonObj.(int)
	case int64:
		m[key] = jsonObj.(int)
	case float32:
		m[key] = jsonObj.(float32)
	case float64:
		m[key] = jsonObj.(float64)
	case []byte:
		m[key] = jsonObj.([]byte)
	case string:
		m[key] = jsonObj.(string)
	case bool:
		m[key] = jsonObj.(bool)
	case nil:
		m[key] = nil
	default:
		m[key] = jsonObj.(string)
	}
}
