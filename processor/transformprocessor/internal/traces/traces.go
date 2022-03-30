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

package traces // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/traces"

import (
	"encoding/hex"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/model/pdata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"
)

type spanTransformContext struct {
	span     pdata.Span
	il       pdata.InstrumentationScope
	resource pdata.Resource
}

func (ctx spanTransformContext) GetItem() interface{} {
	return ctx.span
}

func (ctx spanTransformContext) GetInstrumentationScope() pdata.InstrumentationScope {
	return ctx.il
}

func (ctx spanTransformContext) GetResource() pdata.Resource {
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
	return newPathGetSetter(val.Fields)
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
	case "instrumentation_library":
		if len(path) == 1 {
			return accessInstrumentationScope(), nil
		}
		switch path[1].Name {
		case "name":
			return accessInstrumentationScopeName(), nil
		case "version":
			return accessInstrumentationScopeVersion(), nil
		}
	case "trace_id":
		return accessTraceID(), nil
	case "span_id":
		return accessSpanID(), nil
	case "trace_state":
		return accessTraceState(), nil
	case "parent_span_id":
		return accessParentSpanID(), nil
	case "name":
		return accessName(), nil
	case "kind":
		return accessKind(), nil
	case "start_time_unix_nano":
		return accessStartTimeUnixNano(), nil
	case "end_time_unix_nano":
		return accessEndTimeUnixNano(), nil
	case "attributes":
		mapKey := path[0].MapKey
		if mapKey == nil {
			return accessAttributes(), nil
		}
		return accessAttributesKey(mapKey), nil
	case "dropped_attributes_count":
		return accessDroppedAttributesCount(), nil
	case "events":
		return accessEvents(), nil
	case "dropped_events_count":
		return accessDroppedEventsCount(), nil
	case "links":
		return accessLinks(), nil
	case "dropped_links_count":
		return accessDroppedLinksCount(), nil
	case "status":
		if len(path) == 1 {
			return accessStatus(), nil
		}
		switch path[1].Name {
		case "code":
			return accessStatusCode(), nil
		case "message":
			return accessStatusMessage(), nil
		}
	default:
		return nil, fmt.Errorf("invalid path expression, unrecognized field %v", path[0].Name)
	}

	return nil, fmt.Errorf("invalid path expression %v", path)
}

func accessResource() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx common.TransformContext) interface{} {
			return ctx.GetResource()
		},
		setter: func(ctx common.TransformContext, val interface{}) {
			if newRes, ok := val.(pdata.Resource); ok {
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
			if attrs, ok := val.(pdata.Map); ok {
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
			if newIl, ok := val.(pdata.InstrumentationScope); ok {
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

func accessTraceID() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx common.TransformContext) interface{} {
			return ctx.GetItem().(pdata.Span).TraceID()
		},
		setter: func(ctx common.TransformContext, val interface{}) {
			if str, ok := val.(string); ok {
				id, _ := hex.DecodeString(str)
				var idArr [16]byte
				copy(idArr[:16], id)
				ctx.GetItem().(pdata.Span).SetTraceID(pdata.NewTraceID(idArr))
			}
		},
	}
}

func accessSpanID() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx common.TransformContext) interface{} {
			return ctx.GetItem().(pdata.Span).SpanID()
		},
		setter: func(ctx common.TransformContext, val interface{}) {
			if str, ok := val.(string); ok {
				id, _ := hex.DecodeString(str)
				var idArr [8]byte
				copy(idArr[:8], id)
				ctx.GetItem().(pdata.Span).SetSpanID(pdata.NewSpanID(idArr))
			}
		},
	}
}

func accessTraceState() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx common.TransformContext) interface{} {
			return ctx.GetItem().(pdata.Span).TraceState()
		},
		setter: func(ctx common.TransformContext, val interface{}) {
			if str, ok := val.(string); ok {
				ctx.GetItem().(pdata.Span).SetTraceState(pdata.TraceState(str))
			}
		},
	}
}

func accessParentSpanID() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx common.TransformContext) interface{} {
			return ctx.GetItem().(pdata.Span).ParentSpanID()
		},
		setter: func(ctx common.TransformContext, val interface{}) {
			if str, ok := val.(string); ok {
				id, _ := hex.DecodeString(str)
				var idArr [8]byte
				copy(idArr[:8], id)
				ctx.GetItem().(pdata.Span).SetParentSpanID(pdata.NewSpanID(idArr))
			}
		},
	}
}

func accessName() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx common.TransformContext) interface{} {
			return ctx.GetItem().(pdata.Span).Name()
		},
		setter: func(ctx common.TransformContext, val interface{}) {
			if str, ok := val.(string); ok {
				ctx.GetItem().(pdata.Span).SetName(str)
			}
		},
	}
}

func accessKind() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx common.TransformContext) interface{} {
			return ctx.GetItem().(pdata.Span).Kind()
		},
		setter: func(ctx common.TransformContext, val interface{}) {
			if i, ok := val.(int64); ok {
				ctx.GetItem().(pdata.Span).SetKind(pdata.SpanKind(i))
			}
		},
	}
}

func accessStartTimeUnixNano() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx common.TransformContext) interface{} {
			return ctx.GetItem().(pdata.Span).StartTimestamp().AsTime().UnixNano()
		},
		setter: func(ctx common.TransformContext, val interface{}) {
			if i, ok := val.(int64); ok {
				ctx.GetItem().(pdata.Span).SetStartTimestamp(pdata.NewTimestampFromTime(time.Unix(0, i)))
			}
		},
	}
}

func accessEndTimeUnixNano() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx common.TransformContext) interface{} {
			return ctx.GetItem().(pdata.Span).EndTimestamp().AsTime().UnixNano()
		},
		setter: func(ctx common.TransformContext, val interface{}) {
			if i, ok := val.(int64); ok {
				ctx.GetItem().(pdata.Span).SetEndTimestamp(pdata.NewTimestampFromTime(time.Unix(0, i)))
			}
		},
	}
}

func accessAttributes() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx common.TransformContext) interface{} {
			return ctx.GetItem().(pdata.Span).Attributes()
		},
		setter: func(ctx common.TransformContext, val interface{}) {
			if attrs, ok := val.(pdata.Map); ok {
				ctx.GetItem().(pdata.Span).Attributes().Clear()
				attrs.CopyTo(ctx.GetItem().(pdata.Span).Attributes())
			}
		},
	}
}

func accessAttributesKey(mapKey *string) pathGetSetter {
	return pathGetSetter{
		getter: func(ctx common.TransformContext) interface{} {
			return getAttr(ctx.GetItem().(pdata.Span).Attributes(), *mapKey)
		},
		setter: func(ctx common.TransformContext, val interface{}) {
			setAttr(ctx.GetItem().(pdata.Span).Attributes(), *mapKey, val)
		},
	}
}

func accessDroppedAttributesCount() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx common.TransformContext) interface{} {
			return ctx.GetItem().(pdata.Span).DroppedAttributesCount()
		},
		setter: func(ctx common.TransformContext, val interface{}) {
			if i, ok := val.(int64); ok {
				ctx.GetItem().(pdata.Span).SetDroppedAttributesCount(uint32(i))
			}
		},
	}
}

func accessEvents() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx common.TransformContext) interface{} {
			return ctx.GetItem().(pdata.Span).Events()
		},
		setter: func(ctx common.TransformContext, val interface{}) {
			if slc, ok := val.(pdata.SpanEventSlice); ok {
				ctx.GetItem().(pdata.Span).Events().RemoveIf(func(event pdata.SpanEvent) bool {
					return true
				})
				slc.CopyTo(ctx.GetItem().(pdata.Span).Events())
			}
		},
	}
}

func accessDroppedEventsCount() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx common.TransformContext) interface{} {
			return ctx.GetItem().(pdata.Span).DroppedEventsCount()
		},
		setter: func(ctx common.TransformContext, val interface{}) {
			if i, ok := val.(int64); ok {
				ctx.GetItem().(pdata.Span).SetDroppedEventsCount(uint32(i))
			}
		},
	}
}

func accessLinks() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx common.TransformContext) interface{} {
			return ctx.GetItem().(pdata.Span).Links()
		},
		setter: func(ctx common.TransformContext, val interface{}) {
			if slc, ok := val.(pdata.SpanLinkSlice); ok {
				ctx.GetItem().(pdata.Span).Links().RemoveIf(func(event pdata.SpanLink) bool {
					return true
				})
				slc.CopyTo(ctx.GetItem().(pdata.Span).Links())
			}
		},
	}
}

func accessDroppedLinksCount() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx common.TransformContext) interface{} {
			return ctx.GetItem().(pdata.Span).DroppedLinksCount()
		},
		setter: func(ctx common.TransformContext, val interface{}) {
			if i, ok := val.(int64); ok {
				ctx.GetItem().(pdata.Span).SetDroppedLinksCount(uint32(i))
			}
		},
	}
}

func accessStatus() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx common.TransformContext) interface{} {
			return ctx.GetItem().(pdata.Span).Status()
		},
		setter: func(ctx common.TransformContext, val interface{}) {
			if status, ok := val.(pdata.SpanStatus); ok {
				status.CopyTo(ctx.GetItem().(pdata.Span).Status())
			}
		},
	}
}

func accessStatusCode() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx common.TransformContext) interface{} {
			return ctx.GetItem().(pdata.Span).Status().Code()
		},
		setter: func(ctx common.TransformContext, val interface{}) {
			if i, ok := val.(int64); ok {
				ctx.GetItem().(pdata.Span).Status().SetCode(pdata.StatusCode(i))
			}
		},
	}
}

func accessStatusMessage() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx common.TransformContext) interface{} {
			return ctx.GetItem().(pdata.Span).Status().Message()
		},
		setter: func(ctx common.TransformContext, val interface{}) {
			if str, ok := val.(string); ok {
				ctx.GetItem().(pdata.Span).Status().SetMessage(str)
			}
		},
	}
}

func getAttr(attrs pdata.Map, mapKey string) interface{} {
	val, ok := attrs.Get(mapKey)
	if !ok {
		return nil
	}
	switch val.Type() {
	case pdata.ValueTypeString:
		return val.StringVal()
	case pdata.ValueTypeBool:
		return val.BoolVal()
	case pdata.ValueTypeInt:
		return val.IntVal()
	case pdata.ValueTypeDouble:
		return val.DoubleVal()
	case pdata.ValueTypeMap:
		return val.MapVal()
	case pdata.ValueTypeSlice:
		return val.SliceVal()
	case pdata.ValueTypeBytes:
		return val.BytesVal()
	}
	return nil
}

func setAttr(attrs pdata.Map, mapKey string, val interface{}) {
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
		attrs.UpsertBytes(mapKey, v)
	case []string:
		arr := pdata.NewValueSlice()
		for _, str := range v {
			arr.SliceVal().AppendEmpty().SetStringVal(str)
		}
		attrs.Upsert(mapKey, arr)
	case []bool:
		arr := pdata.NewValueSlice()
		for _, b := range v {
			arr.SliceVal().AppendEmpty().SetBoolVal(b)
		}
		attrs.Upsert(mapKey, arr)
	case []int64:
		arr := pdata.NewValueSlice()
		for _, i := range v {
			arr.SliceVal().AppendEmpty().SetIntVal(i)
		}
		attrs.Upsert(mapKey, arr)
	case []float64:
		arr := pdata.NewValueSlice()
		for _, f := range v {
			arr.SliceVal().AppendEmpty().SetDoubleVal(f)
		}
		attrs.Upsert(mapKey, arr)
	case [][]byte:
		arr := pdata.NewValueSlice()
		for _, b := range v {
			arr.SliceVal().AppendEmpty().SetBytesVal(b)
		}
		attrs.Upsert(mapKey, arr)
	default:
		// TODO(anuraaga): Support set of map type.
	}
}
