// Copyright The OpenTelemetry Authors
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

package googlecloud

import (
	"fmt"
	"math"
	"reflect"

	structpb "github.com/golang/protobuf/ptypes/struct"
	logpb "google.golang.org/genproto/googleapis/logging/v2"
)

func setPayload(entry *logpb.LogEntry, record interface{}) (err error) {
	// Protect against the panic condition inside `jsonValueToStructValue`
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf(r.(string))
		}
	}()
	switch p := record.(type) {
	case string:
		entry.Payload = &logpb.LogEntry_TextPayload{TextPayload: p}
	case []byte:
		entry.Payload = &logpb.LogEntry_TextPayload{TextPayload: string(p)}
	case map[string]interface{}:
		s := jsonMapToProtoStruct(p)
		entry.Payload = &logpb.LogEntry_JsonPayload{JsonPayload: s}
	case map[string]string:
		fields := map[string]*structpb.Value{}
		for k, v := range p {
			fields[k] = jsonValueToStructValue(v)
		}
		entry.Payload = &logpb.LogEntry_JsonPayload{JsonPayload: &structpb.Struct{Fields: fields}}
	default:
		return fmt.Errorf("cannot convert record of type %T to a protobuf representation", record)
	}

	return nil
}

func jsonMapToProtoStruct(m map[string]interface{}) *structpb.Struct {
	fields := map[string]*structpb.Value{}
	for k, v := range m {
		fields[k] = jsonValueToStructValue(v)
	}
	return &structpb.Struct{Fields: fields}
}

func jsonValueToStructValue(v interface{}) *structpb.Value {
	switch x := v.(type) {
	case bool:
		return &structpb.Value{Kind: &structpb.Value_BoolValue{BoolValue: x}}
	case float32, float64, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return toFloatStructValue(v)
	case string:
		return &structpb.Value{Kind: &structpb.Value_StringValue{StringValue: x}}
	case nil:
		return &structpb.Value{Kind: &structpb.Value_NullValue{}}
	case map[string]interface{}:
		return &structpb.Value{Kind: &structpb.Value_StructValue{StructValue: jsonMapToProtoStruct(x)}}
	case map[string]map[string]string:
		fields := map[string]*structpb.Value{}
		for k, v := range x {
			fields[k] = jsonValueToStructValue(v)
		}
		return &structpb.Value{Kind: &structpb.Value_StructValue{StructValue: &structpb.Struct{Fields: fields}}}
	case map[string]string:
		fields := map[string]*structpb.Value{}
		for k, v := range x {
			fields[k] = &structpb.Value{Kind: &structpb.Value_StringValue{StringValue: v}}
		}
		return &structpb.Value{Kind: &structpb.Value_StructValue{StructValue: &structpb.Struct{Fields: fields}}}
	case []interface{}:
		var vals []*structpb.Value
		for _, e := range x {
			vals = append(vals, jsonValueToStructValue(e))
		}
		return &structpb.Value{Kind: &structpb.Value_ListValue{ListValue: &structpb.ListValue{Values: vals}}}
	case []string:
		var vals []*structpb.Value
		for _, e := range x {
			vals = append(vals, jsonValueToStructValue(e))
		}
		return &structpb.Value{Kind: &structpb.Value_ListValue{ListValue: &structpb.ListValue{Values: vals}}}
	default:
		// Fallback to reflection for other types
		return reflectToValue(reflect.ValueOf(v))
	}
}

func toFloatStructValue(v interface{}) *structpb.Value {
	switch x := v.(type) {
	case float32:
		return &structpb.Value{Kind: &structpb.Value_NumberValue{NumberValue: float64(x)}}
	case float64:
		return &structpb.Value{Kind: &structpb.Value_NumberValue{NumberValue: x}}
	case int:
		return &structpb.Value{Kind: &structpb.Value_NumberValue{NumberValue: float64(x)}}
	case int8:
		return &structpb.Value{Kind: &structpb.Value_NumberValue{NumberValue: float64(x)}}
	case int16:
		return &structpb.Value{Kind: &structpb.Value_NumberValue{NumberValue: float64(x)}}
	case int32:
		return &structpb.Value{Kind: &structpb.Value_NumberValue{NumberValue: float64(x)}}
	case int64:
		return &structpb.Value{Kind: &structpb.Value_NumberValue{NumberValue: float64(x)}}
	case uint:
		return &structpb.Value{Kind: &structpb.Value_NumberValue{NumberValue: float64(x)}}
	case uint8:
		return &structpb.Value{Kind: &structpb.Value_NumberValue{NumberValue: float64(x)}}
	case uint16:
		return &structpb.Value{Kind: &structpb.Value_NumberValue{NumberValue: float64(x)}}
	case uint32:
		return &structpb.Value{Kind: &structpb.Value_NumberValue{NumberValue: float64(x)}}
	case uint64:
		return &structpb.Value{Kind: &structpb.Value_NumberValue{NumberValue: float64(x)}}
	default:
		return &structpb.Value{Kind: &structpb.Value_NumberValue{NumberValue: math.NaN()}}
	}
}

func reflectToValue(v reflect.Value) *structpb.Value {
	switch v.Kind() {
	case reflect.Bool:
		return &structpb.Value{Kind: &structpb.Value_BoolValue{BoolValue: v.Bool()}}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return &structpb.Value{Kind: &structpb.Value_NumberValue{NumberValue: float64(v.Int())}}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return &structpb.Value{Kind: &structpb.Value_NumberValue{NumberValue: float64(v.Uint())}}
	case reflect.Float32, reflect.Float64:
		return &structpb.Value{Kind: &structpb.Value_NumberValue{NumberValue: v.Float()}}
	case reflect.Ptr:
		return reflectToPtrValue(v)
	case reflect.Array, reflect.Slice:
		return reflectToArrayValue(v)
	case reflect.Struct:
		return reflectToStructValue(v)
	case reflect.Map:
		return reflectToMapValue(v)
	default:
		// Last resort
		return &structpb.Value{Kind: &structpb.Value_StringValue{StringValue: fmt.Sprint(v)}}
	}
}

func reflectToPtrValue(v reflect.Value) *structpb.Value {
	if v.IsNil() {
		return nil
	}
	return reflectToValue(reflect.Indirect(v))
}

func reflectToArrayValue(v reflect.Value) *structpb.Value {
	size := v.Len()
	if size == 0 {
		return nil
	}
	values := make([]*structpb.Value, size)
	for i := 0; i < size; i++ {
		values[i] = reflectToValue(v.Index(i))
	}
	return &structpb.Value{Kind: &structpb.Value_ListValue{ListValue: &structpb.ListValue{Values: values}}}
}

func reflectToStructValue(v reflect.Value) *structpb.Value {
	t := v.Type()
	size := v.NumField()
	if size == 0 {
		return nil
	}
	fields := make(map[string]*structpb.Value, size)
	for i := 0; i < size; i++ {
		name := t.Field(i).Name
		// Better way?
		if len(name) > 0 && 'A' <= name[0] && name[0] <= 'Z' {
			fields[name] = reflectToValue(v.Field(i))
		}
	}
	if len(fields) == 0 {
		return nil
	}
	return &structpb.Value{Kind: &structpb.Value_StructValue{StructValue: &structpb.Struct{Fields: fields}}}
}

func reflectToMapValue(v reflect.Value) *structpb.Value {
	keys := v.MapKeys()
	if len(keys) == 0 {
		return nil
	}
	fields := make(map[string]*structpb.Value, len(keys))
	for _, k := range keys {
		if k.Kind() == reflect.String {
			fields[k.String()] = reflectToValue(v.MapIndex(k))
		}
	}
	if len(fields) == 0 {
		return nil
	}
	return &structpb.Value{Kind: &structpb.Value_StructValue{StructValue: &structpb.Struct{Fields: fields}}}
}
