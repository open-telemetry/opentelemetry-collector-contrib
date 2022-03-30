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

// pathGetSetter is a getSetter which has been resolved using a path expression provided by a user.
type pathGetSetter struct {
	getter exprFunc
	setter func(ctx spanTransformContext, val interface{})
}

func (path pathGetSetter) get(ctx spanTransformContext) interface{} {
	return path.getter(ctx)
}

func (path pathGetSetter) set(ctx spanTransformContext, val interface{}) {
	path.setter(ctx, val)
}

func newGetSetter(val common.Value) (getSetter, error) {
	if val.Path == nil {
		return nil, fmt.Errorf("must be a trace path expression")
	}

	return newPathGetSetter(val.Path.Fields)
}

func newPathGetSetter(path []common.Field) (getSetter, error) {
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
			return accessInstrumentationLibrary(), nil
		}
		switch path[1].Name {
		case "name":
			return accessInstrumentationLibraryName(), nil
		case "version":
			return accessInstrumentationLibraryVersion(), nil
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
		getter: func(ctx spanTransformContext) interface{} {
			return ctx.resource
		},
		setter: func(ctx spanTransformContext, val interface{}) {
			if newRes, ok := val.(pdata.Resource); ok {
				ctx.resource.Attributes().Clear()
				newRes.CopyTo(ctx.resource)
			}
		},
	}
}

func accessResourceAttributes() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx spanTransformContext) interface{} {
			return ctx.resource.Attributes()
		},
		setter: func(ctx spanTransformContext, val interface{}) {
			if attrs, ok := val.(pdata.Map); ok {
				ctx.resource.Attributes().Clear()
				attrs.CopyTo(ctx.resource.Attributes())
			}
		},
	}
}

func accessResourceAttributesKey(mapKey *string) pathGetSetter {
	return pathGetSetter{
		getter: func(ctx spanTransformContext) interface{} {
			return getAttr(ctx.resource.Attributes(), *mapKey)
		},
		setter: func(ctx spanTransformContext, val interface{}) {
			setAttr(ctx.resource.Attributes(), *mapKey, val)
		},
	}
}

func accessInstrumentationLibrary() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx spanTransformContext) interface{} {
			return ctx.il
		},
		setter: func(ctx spanTransformContext, val interface{}) {
			if newIl, ok := val.(pdata.InstrumentationScope); ok {
				newIl.CopyTo(ctx.il)
			}
		},
	}
}

func accessInstrumentationLibraryName() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx spanTransformContext) interface{} {
			return ctx.il.Name()
		},
		setter: func(ctx spanTransformContext, val interface{}) {
			if str, ok := val.(string); ok {
				ctx.il.SetName(str)
			}
		},
	}
}

func accessInstrumentationLibraryVersion() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx spanTransformContext) interface{} {
			return ctx.il.Version()
		},
		setter: func(ctx spanTransformContext, val interface{}) {
			if str, ok := val.(string); ok {
				ctx.il.SetVersion(str)
			}
		},
	}
}

func accessTraceID() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx spanTransformContext) interface{} {
			return ctx.span.TraceID()
		},
		setter: func(ctx spanTransformContext, val interface{}) {
			if str, ok := val.(string); ok {
				id, _ := hex.DecodeString(str)
				var idArr [16]byte
				copy(idArr[:16], id)
				ctx.span.SetTraceID(pdata.NewTraceID(idArr))
			}
		},
	}
}

func accessSpanID() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx spanTransformContext) interface{} {
			return ctx.span.SpanID()
		},
		setter: func(ctx spanTransformContext, val interface{}) {
			if str, ok := val.(string); ok {
				id, _ := hex.DecodeString(str)
				var idArr [8]byte
				copy(idArr[:8], id)
				ctx.span.SetSpanID(pdata.NewSpanID(idArr))
			}
		},
	}
}

func accessTraceState() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx spanTransformContext) interface{} {
			return ctx.span.TraceState()
		},
		setter: func(ctx spanTransformContext, val interface{}) {
			if str, ok := val.(string); ok {
				ctx.span.SetTraceState(pdata.TraceState(str))
			}
		},
	}
}

func accessParentSpanID() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx spanTransformContext) interface{} {
			return ctx.span.ParentSpanID()
		},
		setter: func(ctx spanTransformContext, val interface{}) {
			if str, ok := val.(string); ok {
				id, _ := hex.DecodeString(str)
				var idArr [8]byte
				copy(idArr[:8], id)
				ctx.span.SetParentSpanID(pdata.NewSpanID(idArr))
			}
		},
	}
}

func accessName() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx spanTransformContext) interface{} {
			return ctx.span.Name()
		},
		setter: func(ctx spanTransformContext, val interface{}) {
			if str, ok := val.(string); ok {
				ctx.span.SetName(str)
			}
		},
	}
}

func accessKind() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx spanTransformContext) interface{} {
			return ctx.span.Kind()
		},
		setter: func(ctx spanTransformContext, val interface{}) {
			if i, ok := val.(int64); ok {
				ctx.span.SetKind(pdata.SpanKind(i))
			}
		},
	}
}

func accessStartTimeUnixNano() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx spanTransformContext) interface{} {
			return ctx.span.StartTimestamp().AsTime().UnixNano()
		},
		setter: func(ctx spanTransformContext, val interface{}) {
			if i, ok := val.(int64); ok {
				ctx.span.SetStartTimestamp(pdata.NewTimestampFromTime(time.Unix(0, i)))
			}
		},
	}
}

func accessEndTimeUnixNano() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx spanTransformContext) interface{} {
			return ctx.span.EndTimestamp().AsTime().UnixNano()
		},
		setter: func(ctx spanTransformContext, val interface{}) {
			if i, ok := val.(int64); ok {
				ctx.span.SetEndTimestamp(pdata.NewTimestampFromTime(time.Unix(0, i)))
			}
		},
	}
}

func accessAttributes() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx spanTransformContext) interface{} {
			return ctx.span.Attributes()
		},
		setter: func(ctx spanTransformContext, val interface{}) {
			if attrs, ok := val.(pdata.Map); ok {
				ctx.span.Attributes().Clear()
				attrs.CopyTo(ctx.span.Attributes())
			}
		},
	}
}

func accessAttributesKey(mapKey *string) pathGetSetter {
	return pathGetSetter{
		getter: func(ctx spanTransformContext) interface{} {
			return getAttr(ctx.span.Attributes(), *mapKey)
		},
		setter: func(ctx spanTransformContext, val interface{}) {
			setAttr(ctx.span.Attributes(), *mapKey, val)
		},
	}
}

func accessDroppedAttributesCount() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx spanTransformContext) interface{} {
			return ctx.span.DroppedAttributesCount()
		},
		setter: func(ctx spanTransformContext, val interface{}) {
			if i, ok := val.(int64); ok {
				ctx.span.SetDroppedAttributesCount(uint32(i))
			}
		},
	}
}

func accessEvents() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx spanTransformContext) interface{} {
			return ctx.span.Events()
		},
		setter: func(ctx spanTransformContext, val interface{}) {
			if slc, ok := val.(pdata.SpanEventSlice); ok {
				ctx.span.Events().RemoveIf(func(event pdata.SpanEvent) bool {
					return true
				})
				slc.CopyTo(ctx.span.Events())
			}
		},
	}
}

func accessDroppedEventsCount() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx spanTransformContext) interface{} {
			return ctx.span.DroppedEventsCount()
		},
		setter: func(ctx spanTransformContext, val interface{}) {
			if i, ok := val.(int64); ok {
				ctx.span.SetDroppedEventsCount(uint32(i))
			}
		},
	}
}

func accessLinks() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx spanTransformContext) interface{} {
			return ctx.span.Links()
		},
		setter: func(ctx spanTransformContext, val interface{}) {
			if slc, ok := val.(pdata.SpanLinkSlice); ok {
				ctx.span.Links().RemoveIf(func(event pdata.SpanLink) bool {
					return true
				})
				slc.CopyTo(ctx.span.Links())
			}
		},
	}
}

func accessDroppedLinksCount() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx spanTransformContext) interface{} {
			return ctx.span.DroppedLinksCount()
		},
		setter: func(ctx spanTransformContext, val interface{}) {
			if i, ok := val.(int64); ok {
				ctx.span.SetDroppedLinksCount(uint32(i))
			}
		},
	}
}

func accessStatus() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx spanTransformContext) interface{} {
			return ctx.span.Status()
		},
		setter: func(ctx spanTransformContext, val interface{}) {
			if status, ok := val.(pdata.SpanStatus); ok {
				status.CopyTo(ctx.span.Status())
			}
		},
	}
}

func accessStatusCode() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx spanTransformContext) interface{} {
			return ctx.span.Status().Code()
		},
		setter: func(ctx spanTransformContext, val interface{}) {
			if i, ok := val.(int64); ok {
				ctx.span.Status().SetCode(pdata.StatusCode(i))
			}
		},
	}
}

func accessStatusMessage() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx spanTransformContext) interface{} {
			return ctx.span.Status().Message()
		},
		setter: func(ctx spanTransformContext, val interface{}) {
			if str, ok := val.(string); ok {
				ctx.span.Status().SetMessage(str)
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
