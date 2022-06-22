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

// nolint:gocritic
package logs // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/logs"

import (
	"fmt"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"
)

type logTransformContext struct {
	log      plog.LogRecord
	il       pcommon.InstrumentationScope
	resource pcommon.Resource
}

func (ctx logTransformContext) GetItem() interface{} {
	return ctx.log
}

func (ctx logTransformContext) GetInstrumentationScope() pcommon.InstrumentationScope {
	return ctx.il
}

func (ctx logTransformContext) GetResource() pcommon.Resource {
	return ctx.resource
}

// pathGetSetter is a getSetter which has been resolved using a path expression provided by a user.
type pathGetSetter struct {
	getter common.ExprFunc
	setter func(ctx common.TransformContext, val interface{})
}

func (path pathGetSetter) Get(ctx common.TransformContext) interface{} {
	return path.getter(ctx)
}

func (path pathGetSetter) Set(ctx common.TransformContext, val interface{}) {
	path.setter(ctx, val)
}

func ParsePath(val *common.Path) (common.GetSetter, error) {
	if val != nil && len(val.Fields) > 0 {
		return newPathGetSetter(val.Fields)
	}
	return nil, fmt.Errorf("bad path %v", val)
}

func newPathGetSetter(path []common.Field) (common.GetSetter, error) {
	switch path[0].Name {
	case "resource":
		if len(path) == 1 {
			return accessResource(), nil
		}
		switch path[1].Name {
		case "attributes":
			mapKey := path[1].MapKey
			if mapKey == nil {
				return accessResourceAttributes(), nil
			}
			return accessResourceAttributesKey(mapKey), nil
		}
	case "instrumentation_scope":
		if len(path) == 1 {
			return accessInstrumentationScope(), nil
		}
		switch path[1].Name {
		case "name":
			return accessInstrumentationScopeName(), nil
		case "version":
			return accessInstrumentationScopeVersion(), nil
		}
	case "time_unix_nano":
		return accessTimeUnixNano(), nil
	case "observed_time_unix_nano":
		return accessObservedTimeUnixNano(), nil
	case "severity_number":
		return accessSeverityNumber(), nil
	case "severity_text":
		return accessSeverityText(), nil
	case "body":
		return accessBody(), nil
	case "attributes":
		mapKey := path[0].MapKey
		if mapKey == nil {
			return accessAttributes(), nil
		}
		return accessAttributesKey(mapKey), nil
	case "dropped_attributes_count":
		return accessDroppedAttributesCount(), nil
	case "flags":
		return accessFlags(), nil
	case "trace_id":
		return accessTraceID(), nil
	case "span_id":
		return accessSpanID(), nil
	}

	return nil, fmt.Errorf("invalid path expression %v", path)
}

func accessResource() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx common.TransformContext) interface{} {
			return ctx.GetResource()
		},
		setter: func(ctx common.TransformContext, val interface{}) {
			if newRes, ok := val.(pcommon.Resource); ok {
				ctx.GetResource().Attributes().Clear()
				newRes.CopyTo(ctx.GetResource())
			}
		},
	}
}

func accessResourceAttributes() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx common.TransformContext) interface{} {
			return ctx.GetResource().Attributes()
		},
		setter: func(ctx common.TransformContext, val interface{}) {
			if attrs, ok := val.(pcommon.Map); ok {
				ctx.GetResource().Attributes().Clear()
				attrs.CopyTo(ctx.GetResource().Attributes())
			}
		},
	}
}

func accessResourceAttributesKey(mapKey *string) pathGetSetter {
	return pathGetSetter{
		getter: func(ctx common.TransformContext) interface{} {
			return getAttr(ctx.GetResource().Attributes(), *mapKey)
		},
		setter: func(ctx common.TransformContext, val interface{}) {
			setAttr(ctx.GetResource().Attributes(), *mapKey, val)
		},
	}
}

func accessInstrumentationScope() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx common.TransformContext) interface{} {
			return ctx.GetInstrumentationScope()
		},
		setter: func(ctx common.TransformContext, val interface{}) {
			if newIl, ok := val.(pcommon.InstrumentationScope); ok {
				newIl.CopyTo(ctx.GetInstrumentationScope())
			}
		},
	}
}

func accessInstrumentationScopeName() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx common.TransformContext) interface{} {
			return ctx.GetInstrumentationScope().Name()
		},
		setter: func(ctx common.TransformContext, val interface{}) {
			if str, ok := val.(string); ok {
				ctx.GetInstrumentationScope().SetName(str)
			}
		},
	}
}

func accessInstrumentationScopeVersion() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx common.TransformContext) interface{} {
			return ctx.GetInstrumentationScope().Version()
		},
		setter: func(ctx common.TransformContext, val interface{}) {
			if str, ok := val.(string); ok {
				ctx.GetInstrumentationScope().SetVersion(str)
			}
		},
	}
}

func accessTimeUnixNano() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx common.TransformContext) interface{} {
			return ctx.GetItem().(plog.LogRecord).Timestamp().AsTime().UnixNano()
		},
		setter: func(ctx common.TransformContext, val interface{}) {
			if i, ok := val.(int64); ok {
				ctx.GetItem().(plog.LogRecord).SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(0, i)))
			}
		},
	}
}

func accessObservedTimeUnixNano() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx common.TransformContext) interface{} {
			return ctx.GetItem().(plog.LogRecord).ObservedTimestamp().AsTime().UnixNano()
		},
		setter: func(ctx common.TransformContext, val interface{}) {
			if i, ok := val.(int64); ok {
				ctx.GetItem().(plog.LogRecord).SetObservedTimestamp(pcommon.NewTimestampFromTime(time.Unix(0, i)))
			}
		},
	}
}

func accessSeverityNumber() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx common.TransformContext) interface{} {
			return ctx.GetItem().(plog.LogRecord).SeverityNumber()
		},
		setter: func(ctx common.TransformContext, val interface{}) {
			if i, ok := val.(int64); ok {
				ctx.GetItem().(plog.LogRecord).SetSeverityNumber(plog.SeverityNumber(i))
			}
		},
	}
}

func accessSeverityText() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx common.TransformContext) interface{} {
			return ctx.GetItem().(plog.LogRecord).SeverityText()
		},
		setter: func(ctx common.TransformContext, val interface{}) {
			if s, ok := val.(string); ok {
				ctx.GetItem().(plog.LogRecord).SetSeverityText(s)
			}
		},
	}
}

func accessBody() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx common.TransformContext) interface{} {
			return getValue(ctx.GetItem().(plog.LogRecord).Body())
		},
		setter: func(ctx common.TransformContext, val interface{}) {
			setValue(ctx.GetItem().(plog.LogRecord).Body(), val)
		},
	}
}

func accessAttributes() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx common.TransformContext) interface{} {
			return ctx.GetItem().(plog.LogRecord).Attributes()
		},
		setter: func(ctx common.TransformContext, val interface{}) {
			if attrs, ok := val.(pcommon.Map); ok {
				ctx.GetItem().(plog.LogRecord).Attributes().Clear()
				attrs.CopyTo(ctx.GetItem().(plog.LogRecord).Attributes())
			}
		},
	}
}

func accessAttributesKey(mapKey *string) pathGetSetter {
	return pathGetSetter{
		getter: func(ctx common.TransformContext) interface{} {
			return getAttr(ctx.GetItem().(plog.LogRecord).Attributes(), *mapKey)
		},
		setter: func(ctx common.TransformContext, val interface{}) {
			setAttr(ctx.GetItem().(plog.LogRecord).Attributes(), *mapKey, val)
		},
	}
}

func accessDroppedAttributesCount() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx common.TransformContext) interface{} {
			return int64(ctx.GetItem().(plog.LogRecord).DroppedAttributesCount())
		},
		setter: func(ctx common.TransformContext, val interface{}) {
			if i, ok := val.(int64); ok {
				ctx.GetItem().(plog.LogRecord).SetDroppedAttributesCount(uint32(i))
			}
		},
	}
}

func accessFlags() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx common.TransformContext) interface{} {
			return int64(ctx.GetItem().(plog.LogRecord).Flags())
		},
		setter: func(ctx common.TransformContext, val interface{}) {
			if i, ok := val.(int64); ok {
				ctx.GetItem().(plog.LogRecord).SetFlags(uint32(i))
			}
		},
	}
}

func accessTraceID() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx common.TransformContext) interface{} {
			return ctx.GetItem().(plog.LogRecord).TraceID()
		},
		setter: func(ctx common.TransformContext, val interface{}) {
			if newTraceID, ok := val.(pcommon.TraceID); ok {
				ctx.GetItem().(plog.LogRecord).SetTraceID(newTraceID)
			}
		},
	}
}

func accessSpanID() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx common.TransformContext) interface{} {
			return ctx.GetItem().(plog.LogRecord).SpanID()
		},
		setter: func(ctx common.TransformContext, val interface{}) {
			if newSpanID, ok := val.(pcommon.SpanID); ok {
				ctx.GetItem().(plog.LogRecord).SetSpanID(newSpanID)
			}
		},
	}
}

func getAttr(attrs pcommon.Map, mapKey string) interface{} {
	val, ok := attrs.Get(mapKey)
	if !ok {
		return nil
	}
	return getValue(val)
}

func getValue(val pcommon.Value) interface{} {
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
		return val.MBytesVal()
	}
	return nil
}

func setAttr(attrs pcommon.Map, mapKey string, val interface{}) {
	switch v := val.(type) {
	case string:
		attrs.UpsertString(mapKey, v)
	case bool:
		attrs.UpsertBool(mapKey, v)
	case int64:
		attrs.UpsertInt(mapKey, v)
	case float64:
		attrs.UpsertDouble(mapKey, v)
	case []byte:
		attrs.UpsertBytes(mapKey, pcommon.NewImmutableByteSlice(v))
	case []string:
		arr := pcommon.NewValueSlice()
		for _, str := range v {
			arr.SliceVal().AppendEmpty().SetStringVal(str)
		}
		attrs.Upsert(mapKey, arr)
	case []bool:
		arr := pcommon.NewValueSlice()
		for _, b := range v {
			arr.SliceVal().AppendEmpty().SetBoolVal(b)
		}
		attrs.Upsert(mapKey, arr)
	case []int64:
		arr := pcommon.NewValueSlice()
		for _, i := range v {
			arr.SliceVal().AppendEmpty().SetIntVal(i)
		}
		attrs.Upsert(mapKey, arr)
	case []float64:
		arr := pcommon.NewValueSlice()
		for _, f := range v {
			arr.SliceVal().AppendEmpty().SetDoubleVal(f)
		}
		attrs.Upsert(mapKey, arr)
	case [][]byte:
		arr := pcommon.NewValueSlice()
		for _, b := range v {
			arr.SliceVal().AppendEmpty().SetBytesVal(pcommon.NewImmutableByteSlice(b))
		}
		attrs.Upsert(mapKey, arr)
	default:
		// TODO(anuraaga): Support set of map type.
	}
}

func setValue(value pcommon.Value, val interface{}) {
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
		value.SetBytesVal(pcommon.NewImmutableByteSlice(v))
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
			value.SliceVal().AppendEmpty().SetBytesVal(pcommon.NewImmutableByteSlice(b))
		}
	default:
		// TODO(anuraaga): Support set of map type.
	}
}
