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

package googlecloudexporter

import (
	"encoding/base64"
	"fmt"
	"unicode/utf8"

	"google.golang.org/protobuf/types/known/structpb"
)

// convertToProto converts a value to a protobuf equivalent
func convertToProto(v interface{}) (*structpb.Value, error) {
	switch v := v.(type) {
	case nil:
		return structpb.NewNullValue(), nil
	case bool:
		return structpb.NewBoolValue(v), nil
	case int:
		return structpb.NewNumberValue(float64(v)), nil
	case int8:
		return structpb.NewNumberValue(float64(v)), nil
	case int16:
		return structpb.NewNumberValue(float64(v)), nil
	case int32:
		return structpb.NewNumberValue(float64(v)), nil
	case int64:
		return structpb.NewNumberValue(float64(v)), nil
	case uint:
		return structpb.NewNumberValue(float64(v)), nil
	case uint8:
		return structpb.NewNumberValue(float64(v)), nil
	case uint16:
		return structpb.NewNumberValue(float64(v)), nil
	case uint32:
		return structpb.NewNumberValue(float64(v)), nil
	case uint64:
		return structpb.NewNumberValue(float64(v)), nil
	case float32:
		return structpb.NewNumberValue(float64(v)), nil
	case float64:
		return structpb.NewNumberValue(v), nil
	case string:
		return convertStringToProto(v)
	case []byte:
		return convertBytesToProto(v), nil
	case map[string]interface{}:
		return convertMapToProto(v)
	case map[string]string:
		return convertStringMapToProto(v)
	case map[string]map[string]string:
		return convertEmbeddedMapToProto(v)
	case []interface{}:
		return convertListToProto(v)
	case []string:
		return convertStringListToProto(v)
	default:
		return nil, fmt.Errorf("invalid type for proto conversion: %T", v)
	}
}

// convertStringToProto converts a string to a proto string
func convertStringToProto(value string) (*structpb.Value, error) {
	if !utf8.ValidString(value) {
		return nil, fmt.Errorf("invalid UTF-8 in string: %q", value)
	}
	return structpb.NewStringValue(value), nil
}

// convertBytesToProto converts a byte array to a protobuf equivalent
func convertBytesToProto(bytes []byte) *structpb.Value {
	s := base64.StdEncoding.EncodeToString(bytes)
	return structpb.NewStringValue(s)
}

// convertMapToProto converts a map to a protobuf equivalent
func convertMapToProto(mapValue map[string]interface{}) (*structpb.Value, error) {
	fields := make(map[string]*structpb.Value, len(mapValue))

	for key, value := range mapValue {
		if !utf8.ValidString(key) {
			return nil, fmt.Errorf("invalid UTF-8 in key: %q", key)
		}

		protoValue, err := convertToProto(value)
		if err != nil {
			return nil, fmt.Errorf("failed to convert value for key %s to proto: %w", key, err)
		}

		fields[key] = protoValue
	}

	protoStruct := &structpb.Struct{Fields: fields}
	return structpb.NewStructValue(protoStruct), nil
}

// convertStringMapToProto converts a string map to a protobuf equivalent
func convertStringMapToProto(mapValue map[string]string) (*structpb.Value, error) {
	fields := make(map[string]*structpb.Value, len(mapValue))

	for key, value := range mapValue {
		if !utf8.ValidString(key) {
			return nil, fmt.Errorf("invalid UTF-8 in key: %q", key)
		}

		protoValue, err := convertStringToProto(value)
		if err != nil {
			return nil, fmt.Errorf("failed to convert value for key %s to proto: %w", key, err)
		}

		fields[key] = protoValue
	}

	protoStruct := &structpb.Struct{Fields: fields}
	return structpb.NewStructValue(protoStruct), nil
}

// convertEmbeddedMapToProto converts an embedded string map to a protobuf equivalent
func convertEmbeddedMapToProto(mapValue map[string]map[string]string) (*structpb.Value, error) {
	fields := make(map[string]*structpb.Value, len(mapValue))

	for key, value := range mapValue {
		if !utf8.ValidString(key) {
			return nil, fmt.Errorf("invalid UTF-8 in key: %q", key)
		}

		protoValue, err := convertToProto(value)
		if err != nil {
			return nil, fmt.Errorf("failed to convert embedded value for key %s to proto: %w", key, err)
		}

		fields[key] = protoValue
	}

	protoStruct := &structpb.Struct{Fields: fields}
	return structpb.NewStructValue(protoStruct), nil
}

// convertListToProto converts a generic list to a protobuf equivalent
func convertListToProto(list []interface{}) (*structpb.Value, error) {
	values := make([]*structpb.Value, len(list))

	for i, value := range list {
		protoValue, err := convertToProto(value)
		if err != nil {
			return nil, fmt.Errorf("failed to convert index %d of list to proto: %w", i, err)
		}

		values[i] = protoValue
	}

	protoList := &structpb.ListValue{Values: values}
	return structpb.NewListValue(protoList), nil
}

// convertStringListToProto converts a string list to a protobuf equivalent
func convertStringListToProto(list []string) (*structpb.Value, error) {
	values := make([]*structpb.Value, len(list))

	for i, value := range list {
		protoValue, err := convertStringToProto(value)
		if err != nil {
			return nil, fmt.Errorf("failed to convert index %d to proto: %w", i, err)
		}

		values[i] = protoValue
	}

	protoList := &structpb.ListValue{Values: values}
	return structpb.NewListValue(protoList), nil
}
