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
	setter func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource, val interface{})
}

func (path pathGetSetter) get(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) interface{} {
	return path.getter(span, il, resource)
}

func (path pathGetSetter) set(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource, val interface{}) {
	path.setter(span, il, resource, val)
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
		getter: func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) interface{} {
			return resource
		},
		setter: func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource, val interface{}) {
			if newRes, ok := val.(pdata.Resource); ok {
				resource.Attributes().Clear()
				newRes.CopyTo(resource)
			}
		},
	}
}

func accessResourceAttributes() pathGetSetter {
	return pathGetSetter{
		getter: func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) interface{} {
			return resource.Attributes()
		},
		setter: func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource, val interface{}) {
			if attrs, ok := val.(pdata.AttributeMap); ok {
				resource.Attributes().Clear()
				attrs.CopyTo(resource.Attributes())
			}
		},
	}
}

func accessResourceAttributesKey(mapKey *string) pathGetSetter {
	return pathGetSetter{
		getter: func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) interface{} {
			return getAttr(resource.Attributes(), *mapKey)
		},
		setter: func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource, val interface{}) {
			setAttr(resource.Attributes(), *mapKey, val)
		},
	}
}

func accessInstrumentationLibrary() pathGetSetter {
	return pathGetSetter{
		getter: func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) interface{} {
			return il
		},
		setter: func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource, val interface{}) {
			if newIl, ok := val.(pdata.InstrumentationLibrary); ok {
				newIl.CopyTo(il)
			}
		},
	}
}

func accessInstrumentationLibraryName() pathGetSetter {
	return pathGetSetter{
		getter: func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) interface{} {
			return il.Name()
		},
		setter: func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource, val interface{}) {
			if str, ok := val.(string); ok {
				il.SetName(str)
			}
		},
	}
}

func accessInstrumentationLibraryVersion() pathGetSetter {
	return pathGetSetter{
		getter: func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) interface{} {
			return il.Version()
		},
		setter: func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource, val interface{}) {
			if str, ok := val.(string); ok {
				il.SetVersion(str)
			}
		},
	}
}

func accessTraceID() pathGetSetter {
	return pathGetSetter{
		getter: func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) interface{} {
			return span.TraceID()
		},
		setter: func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource, val interface{}) {
			if str, ok := val.(string); ok {
				id, _ := hex.DecodeString(str)
				var idArr [16]byte
				copy(idArr[:16], id)
				span.SetTraceID(pdata.NewTraceID(idArr))
			}
		},
	}
}

func accessSpanID() pathGetSetter {
	return pathGetSetter{
		getter: func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) interface{} {
			return span.SpanID()
		},
		setter: func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource, val interface{}) {
			if str, ok := val.(string); ok {
				id, _ := hex.DecodeString(str)
				var idArr [8]byte
				copy(idArr[:8], id)
				span.SetSpanID(pdata.NewSpanID(idArr))
			}
		},
	}
}

func accessTraceState() pathGetSetter {
	return pathGetSetter{
		getter: func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) interface{} {
			return span.TraceState()
		},
		setter: func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource, val interface{}) {
			if str, ok := val.(string); ok {
				span.SetTraceState(pdata.TraceState(str))
			}
		},
	}
}

func accessParentSpanID() pathGetSetter {
	return pathGetSetter{
		getter: func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) interface{} {
			return span.ParentSpanID()
		},
		setter: func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource, val interface{}) {
			if str, ok := val.(string); ok {
				id, _ := hex.DecodeString(str)
				var idArr [8]byte
				copy(idArr[:8], id)
				span.SetParentSpanID(pdata.NewSpanID(idArr))
			}
		},
	}
}

func accessName() pathGetSetter {
	return pathGetSetter{
		getter: func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) interface{} {
			return span.Name()
		},
		setter: func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource, val interface{}) {
			if str, ok := val.(string); ok {
				span.SetName(str)
			}
		},
	}
}

func accessKind() pathGetSetter {
	return pathGetSetter{
		getter: func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) interface{} {
			return span.Kind()
		},
		setter: func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource, val interface{}) {
			if i, ok := val.(int64); ok {
				span.SetKind(pdata.SpanKind(i))
			}
		},
	}
}

func accessStartTimeUnixNano() pathGetSetter {
	return pathGetSetter{
		getter: func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) interface{} {
			return span.StartTimestamp().AsTime().UnixNano()
		},
		setter: func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource, val interface{}) {
			if i, ok := val.(int64); ok {
				span.SetStartTimestamp(pdata.NewTimestampFromTime(time.Unix(0, i)))
			}
		},
	}
}

func accessEndTimeUnixNano() pathGetSetter {
	return pathGetSetter{
		getter: func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) interface{} {
			return span.EndTimestamp().AsTime().UnixNano()
		},
		setter: func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource, val interface{}) {
			if i, ok := val.(int64); ok {
				span.SetEndTimestamp(pdata.NewTimestampFromTime(time.Unix(0, i)))
			}
		},
	}
}

func accessAttributes() pathGetSetter {
	return pathGetSetter{
		getter: func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) interface{} {
			return span.Attributes()
		},
		setter: func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource, val interface{}) {
			if attrs, ok := val.(pdata.AttributeMap); ok {
				span.Attributes().Clear()
				attrs.CopyTo(span.Attributes())
			}
		},
	}
}

func accessAttributesKey(mapKey *string) pathGetSetter {
	return pathGetSetter{
		getter: func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) interface{} {
			return getAttr(span.Attributes(), *mapKey)
		},
		setter: func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource, val interface{}) {
			setAttr(span.Attributes(), *mapKey, val)
		},
	}
}

func accessDroppedAttributesCount() pathGetSetter {
	return pathGetSetter{
		getter: func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) interface{} {
			return span.DroppedAttributesCount()
		},
		setter: func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource, val interface{}) {
			if i, ok := val.(int64); ok {
				span.SetDroppedAttributesCount(uint32(i))
			}
		},
	}
}

func accessEvents() pathGetSetter {
	return pathGetSetter{
		getter: func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) interface{} {
			return span.Events()
		},
		setter: func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource, val interface{}) {
			if slc, ok := val.(pdata.SpanEventSlice); ok {
				span.Events().RemoveIf(func(event pdata.SpanEvent) bool {
					return true
				})
				slc.CopyTo(span.Events())
			}
		},
	}
}

func accessDroppedEventsCount() pathGetSetter {
	return pathGetSetter{
		getter: func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) interface{} {
			return span.DroppedEventsCount()
		},
		setter: func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource, val interface{}) {
			if i, ok := val.(int64); ok {
				span.SetDroppedEventsCount(uint32(i))
			}
		},
	}
}

func accessLinks() pathGetSetter {
	return pathGetSetter{
		getter: func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) interface{} {
			return span.Links()
		},
		setter: func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource, val interface{}) {
			if slc, ok := val.(pdata.SpanLinkSlice); ok {
				span.Links().RemoveIf(func(event pdata.SpanLink) bool {
					return true
				})
				slc.CopyTo(span.Links())
			}
		},
	}
}

func accessDroppedLinksCount() pathGetSetter {
	return pathGetSetter{
		getter: func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) interface{} {
			return span.DroppedLinksCount()
		},
		setter: func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource, val interface{}) {
			if i, ok := val.(int64); ok {
				span.SetDroppedLinksCount(uint32(i))
			}
		},
	}
}

func accessStatus() pathGetSetter {
	return pathGetSetter{
		getter: func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) interface{} {
			return span.Status()
		},
		setter: func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource, val interface{}) {
			if status, ok := val.(pdata.SpanStatus); ok {
				status.CopyTo(span.Status())
			}
		},
	}
}

func accessStatusCode() pathGetSetter {
	return pathGetSetter{
		getter: func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) interface{} {
			return span.Status().Code()
		},
		setter: func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource, val interface{}) {
			if i, ok := val.(int64); ok {
				span.Status().SetCode(pdata.StatusCode(i))
			}
		},
	}
}

func accessStatusMessage() pathGetSetter {
	return pathGetSetter{
		getter: func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) interface{} {
			return span.Status().Message()
		},
		setter: func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource, val interface{}) {
			if str, ok := val.(string); ok {
				span.Status().SetMessage(str)
			}
		},
	}
}

func getAttr(attrs pdata.AttributeMap, mapKey string) interface{} {
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
	case pdata.ValueTypeArray:
		return val.SliceVal()
	case pdata.ValueTypeBytes:
		return val.BytesVal()
	}
	return nil
}

func setAttr(attrs pdata.AttributeMap, mapKey string, val interface{}) {
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
		arr := pdata.NewValueArray()
		for _, str := range v {
			arr.SliceVal().AppendEmpty().SetStringVal(str)
		}
		attrs.Upsert(mapKey, arr)
	case []bool:
		arr := pdata.NewValueArray()
		for _, b := range v {
			arr.SliceVal().AppendEmpty().SetBoolVal(b)
		}
		attrs.Upsert(mapKey, arr)
	case []int64:
		arr := pdata.NewValueArray()
		for _, i := range v {
			arr.SliceVal().AppendEmpty().SetIntVal(i)
		}
		attrs.Upsert(mapKey, arr)
	case []float64:
		arr := pdata.NewValueArray()
		for _, f := range v {
			arr.SliceVal().AppendEmpty().SetDoubleVal(f)
		}
		attrs.Upsert(mapKey, arr)
	case [][]byte:
		arr := pdata.NewValueArray()
		for _, b := range v {
			arr.SliceVal().AppendEmpty().SetBytesVal(b)
		}
		attrs.Upsert(mapKey, arr)
	default:
		// TODO(anuraaga): Support set of map type.
	}
}
