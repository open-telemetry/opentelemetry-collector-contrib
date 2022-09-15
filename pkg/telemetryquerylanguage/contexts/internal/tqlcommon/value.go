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

package tqlcommon // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/telemetryquerylanguage/contexts/internal/tqlcommon"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func GetValue(val pcommon.Value) interface{} {
	switch val.Type() {
	case pcommon.ValueTypeString:
		return val.StringVal()
	case pcommon.ValueTypeBool:
		return val.BoolVal()
	case pcommon.ValueTypeInt:
		return val.IntVal()
	case pcommon.ValueTypeDouble:
		return val.DoubleVal()
	case pcommon.ValueTypeMap:
		return val.MapVal()
	case pcommon.ValueTypeSlice:
		return val.SliceVal()
	case pcommon.ValueTypeBytes:
		return val.BytesVal().AsRaw()
	}
	return nil
}

func SetValue(value pcommon.Value, val interface{}) {
	switch v := val.(type) {
	case string:
		value.SetStringVal(v)
	case bool:
		value.SetBoolVal(v)
	case int64:
		value.SetIntVal(v)
	case float64:
		value.SetDoubleVal(v)
	case []byte:
		value.SetEmptyBytesVal().FromRaw(v)
	case []string:
		value.SliceVal().RemoveIf(func(_ pcommon.Value) bool {
			return true
		})
		for _, str := range v {
			value.SliceVal().AppendEmpty().SetStringVal(str)
		}
	case []bool:
		value.SliceVal().RemoveIf(func(_ pcommon.Value) bool {
			return true
		})
		for _, b := range v {
			value.SliceVal().AppendEmpty().SetBoolVal(b)
		}
	case []int64:
		value.SliceVal().RemoveIf(func(_ pcommon.Value) bool {
			return true
		})
		for _, i := range v {
			value.SliceVal().AppendEmpty().SetIntVal(i)
		}
	case []float64:
		value.SliceVal().RemoveIf(func(_ pcommon.Value) bool {
			return true
		})
		for _, f := range v {
			value.SliceVal().AppendEmpty().SetDoubleVal(f)
		}
	case [][]byte:
		value.SliceVal().RemoveIf(func(_ pcommon.Value) bool {
			return true
		})
		for _, b := range v {
			value.SliceVal().AppendEmpty().SetEmptyBytesVal().FromRaw(b)
		}
	default:
		// TODO(anuraaga): Support set of map type.
	}
}
