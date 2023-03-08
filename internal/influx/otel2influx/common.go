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

package otel2influx // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/influx/otel2influx"

import (
	"encoding/json"
	"fmt"
	"strconv"

	"go.opentelemetry.io/collector/pdata/pcommon"
	semconv "go.opentelemetry.io/collector/semconv/v1.16.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/influx/common"
)

func ResourceToTags(logger common.Logger, resource pcommon.Resource, tags map[string]string) (tagsAgain map[string]string) {
	resource.Attributes().Range(func(k string, v pcommon.Value) bool {
		if k == "" {
			logger.Debug("resource attribute key is empty")
		} else if v, err := AttributeValueToInfluxTagValue(v); err != nil {
			logger.Debug("invalid resource attribute value", "key", k, err)
		} else {
			tags[k] = v
		}
		return true
	})
	return tags
}

func InstrumentationScopeToTags(instrumentationLibrary pcommon.InstrumentationScope, tags map[string]string) (tagsAgain map[string]string) {
	if instrumentationLibrary.Name() != "" {
		tags[semconv.OtelLibraryName] = instrumentationLibrary.Name()
	}
	if instrumentationLibrary.Version() != "" {
		tags[semconv.OtelLibraryVersion] = instrumentationLibrary.Version()
	}
	return tags
}

func AttributeValueToInfluxTagValue(value pcommon.Value) (string, error) {
	switch value.Type() {
	case pcommon.ValueTypeStr:
		return value.Str(), nil
	case pcommon.ValueTypeInt:
		return strconv.FormatInt(value.Int(), 10), nil
	case pcommon.ValueTypeDouble:
		return strconv.FormatFloat(value.Double(), 'f', -1, 64), nil
	case pcommon.ValueTypeBool:
		return strconv.FormatBool(value.Bool()), nil
	case pcommon.ValueTypeMap:
		jsonBytes, err := json.Marshal(otlpKeyValueListToMap(value.Map()))
		if err != nil {
			return "", err
		}
		return string(jsonBytes), nil
	case pcommon.ValueTypeSlice:
		jsonBytes, err := json.Marshal(otlpArrayToSlice(value.Slice()))
		if err != nil {
			return "", err
		}
		return string(jsonBytes), nil
	case pcommon.ValueTypeEmpty:
		return "", nil
	default:
		return "", fmt.Errorf("unknown value type %d", value.Type())
	}
}

func AttributeValueToInfluxFieldValue(value pcommon.Value) (interface{}, error) {
	switch value.Type() {
	case pcommon.ValueTypeStr:
		return value.Str(), nil
	case pcommon.ValueTypeInt:
		return value.Int(), nil
	case pcommon.ValueTypeDouble:
		return value.Double(), nil
	case pcommon.ValueTypeBool:
		return value.Bool(), nil
	case pcommon.ValueTypeMap:
		jsonBytes, err := json.Marshal(otlpKeyValueListToMap(value.Map()))
		if err != nil {
			return nil, err
		}
		return string(jsonBytes), nil
	case pcommon.ValueTypeSlice:
		jsonBytes, err := json.Marshal(otlpArrayToSlice(value.Slice()))
		if err != nil {
			return nil, err
		}
		return string(jsonBytes), nil
	case pcommon.ValueTypeEmpty:
		return nil, nil
	default:
		return nil, fmt.Errorf("unknown value type %v", value)
	}
}

func otlpKeyValueListToMap(kvList pcommon.Map) map[string]interface{} {
	m := make(map[string]interface{}, kvList.Len())
	kvList.Range(func(k string, v pcommon.Value) bool {
		switch v.Type() {
		case pcommon.ValueTypeStr:
			m[k] = v.Str()
		case pcommon.ValueTypeInt:
			m[k] = v.Int()
		case pcommon.ValueTypeDouble:
			m[k] = v.Double()
		case pcommon.ValueTypeBool:
			m[k] = v.Bool()
		case pcommon.ValueTypeMap:
			m[k] = otlpKeyValueListToMap(v.Map())
		case pcommon.ValueTypeSlice:
			m[k] = otlpArrayToSlice(v.Slice())
		case pcommon.ValueTypeEmpty:
			m[k] = nil
		default:
			m[k] = fmt.Sprintf("<invalid map value> %v", v)
		}
		return true
	})
	return m
}

func otlpArrayToSlice(arr pcommon.Slice) []interface{} {
	s := make([]interface{}, 0, arr.Len())
	for i := 0; i < arr.Len(); i++ {
		v := arr.At(i)
		switch v.Type() {
		case pcommon.ValueTypeStr:
			s = append(s, v.Str())
		case pcommon.ValueTypeInt:
			s = append(s, v.Int())
		case pcommon.ValueTypeDouble:
			s = append(s, v.Double())
		case pcommon.ValueTypeBool:
			s = append(s, v.Bool())
		case pcommon.ValueTypeEmpty:
			s = append(s, nil)
		default:
			s = append(s, fmt.Sprintf("<invalid array value> %v", v))
		}
	}
	return s
}

func convertResourceTags(resource pcommon.Resource) map[string]string {
	tags := make(map[string]string, resource.Attributes().Len())
	resource.Attributes().Range(func(k string, v pcommon.Value) bool {
		tags[k] = v.AsString()
		return true
	})
	// TODO dropped attributes counts
	return tags
}

func convertResourceFields(resource pcommon.Resource) map[string]interface{} {
	fields := make(map[string]interface{}, resource.Attributes().Len())
	resource.Attributes().Range(func(k string, v pcommon.Value) bool {
		fields[k] = v.AsRaw()
		return true
	})
	// TODO dropped attributes counts
	return fields
}

func convertScopeFields(is pcommon.InstrumentationScope) map[string]interface{} {
	fields := make(map[string]interface{}, is.Attributes().Len()+2)
	is.Attributes().Range(func(k string, v pcommon.Value) bool {
		fields[k] = v.AsRaw()
		return true
	})
	if name := is.Name(); name != "" {
		fields[semconv.AttributeTelemetrySDKName] = name
	}
	if version := is.Version(); version != "" {
		fields[semconv.AttributeTelemetrySDKVersion] = version
	}
	// TODO dropped attributes counts
	return fields
}

type basicDataPoint interface {
	Timestamp() pcommon.Timestamp
	StartTimestamp() pcommon.Timestamp
	Attributes() pcommon.Map
}
