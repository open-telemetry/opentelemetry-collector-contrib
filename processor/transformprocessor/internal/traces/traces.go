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
	getter func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) interface{}
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
	var getter func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) interface{}
	var setter func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource, val interface{})
	switch path[0].Name {
	case "resource":
		mapKey := path[1].MapKey
		if mapKey == nil {
			getter = func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) interface{} {
				return resource.Attributes()
			}
			setter = func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource, val interface{}) {
				if attrs, ok := val.(pdata.AttributeMap); ok {
					resource.Attributes().Clear()
					attrs.CopyTo(resource.Attributes())
				}
			}
		} else {
			getter = func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) interface{} {
				return getAttr(resource.Attributes(), *mapKey)
			}
			setter = func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource, val interface{}) {
				setAttr(resource.Attributes(), *mapKey, val)
			}
		}
	case "instrumentation_library":
		if len(path) == 1 {
			getter = func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) interface{} {
				return il
			}
			setter = func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource, val interface{}) {
				if newIl, ok := val.(pdata.InstrumentationLibrary); ok {
					newIl.CopyTo(il)
				}
			}
		} else {
			switch path[1].Name {
			case "name":
				getter = func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) interface{} {
					return il.Name()
				}
				setter = func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource, val interface{}) {
					if str, ok := val.(string); ok {
						il.SetName(str)
					}
				}
			case "version":
				getter = func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) interface{} {
					return il.Version()
				}
				setter = func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource, val interface{}) {
					if str, ok := val.(string); ok {
						il.SetVersion(str)
					}
				}
			}
		}
	case "trace_id":
		getter = func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) interface{} {
			return span.TraceID()
		}
		setter = func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource, val interface{}) {
			if str, ok := val.(string); ok {
				id, _ := hex.DecodeString(str)
				var idArr [16]byte
				copy(idArr[:16], id)
				span.SetTraceID(pdata.NewTraceID(idArr))
			}
		}
	case "span_id":
		getter = func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) interface{} {
			return span.SpanID()
		}
		setter = func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource, val interface{}) {
			if str, ok := val.(string); ok {
				id, _ := hex.DecodeString(str)
				var idArr [8]byte
				copy(idArr[:8], id)
				span.SetSpanID(pdata.NewSpanID(idArr))
			}
		}
	case "trace_state":
		getter = func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) interface{} {
			return span.TraceState()
		}
		setter = func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource, val interface{}) {
			if str, ok := val.(string); ok {
				span.SetTraceState(pdata.TraceState(str))
			}
		}
	case "parent_span_id":
		getter = func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) interface{} {
			return span.ParentSpanID()
		}
		setter = func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource, val interface{}) {
			if str, ok := val.(string); ok {
				id, _ := hex.DecodeString(str)
				var idArr [8]byte
				copy(idArr[:8], id)
				span.SetParentSpanID(pdata.NewSpanID(idArr))
			}
		}
	case "name":
		getter = func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) interface{} {
			return span.Name()
		}
		setter = func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource, val interface{}) {
			if str, ok := val.(string); ok {
				span.SetName(str)
			}
		}
	case "kind":
		getter = func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) interface{} {
			return span.Kind()
		}
		setter = func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource, val interface{}) {
			if i, ok := val.(int64); ok {
				span.SetKind(pdata.SpanKind(i))
			}
		}
	case "start_time_unix_nano":
		getter = func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) interface{} {
			return span.StartTimestamp().AsTime().UnixNano()
		}
		setter = func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource, val interface{}) {
			if i, ok := val.(int64); ok {
				span.SetStartTimestamp(pdata.NewTimestampFromTime(time.Unix(0, i)))
			}
		}
	case "end_time_unix_nano":
		getter = func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) interface{} {
			return span.EndTimestamp().AsTime().UnixNano()
		}
		setter = func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource, val interface{}) {
			if i, ok := val.(int64); ok {
				span.SetEndTimestamp(pdata.NewTimestampFromTime(time.Unix(0, i)))
			}
		}
	case "attributes":
		mapKey := path[0].MapKey
		if mapKey == nil {
			getter = func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) interface{} {
				return span.Attributes()
			}
			setter = func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource, val interface{}) {
				if attrs, ok := val.(pdata.AttributeMap); ok {
					span.Attributes().Clear()
					attrs.CopyTo(span.Attributes())
				}
			}
		} else {
			getter = func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) interface{} {
				return getAttr(span.Attributes(), *mapKey)
			}
			setter = func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource, val interface{}) {
				setAttr(span.Attributes(), *mapKey, val)
			}
		}
	case "dropped_attributes_count":
		getter = func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) interface{} {
			return span.DroppedAttributesCount()
		}
		setter = func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource, val interface{}) {
			if i, ok := val.(int64); ok {
				span.SetDroppedAttributesCount(uint32(i))
			}
		}
	case "events":
		getter = func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) interface{} {
			return span.Events()
		}
		setter = func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource, val interface{}) {
			if slc, ok := val.(pdata.SpanEventSlice); ok {
				span.Events().RemoveIf(func(event pdata.SpanEvent) bool {
					return true
				})
				slc.CopyTo(span.Events())
			}
		}
	case "dropped_events_count":
		getter = func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) interface{} {
			return span.DroppedEventsCount()
		}
		setter = func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource, val interface{}) {
			if i, ok := val.(int64); ok {
				span.SetDroppedEventsCount(uint32(i))
			}
		}
	case "links":
		getter = func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) interface{} {
			return span.Links()
		}
		setter = func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource, val interface{}) {
			if slc, ok := val.(pdata.SpanLinkSlice); ok {
				span.Links().RemoveIf(func(event pdata.SpanLink) bool {
					return true
				})
				slc.CopyTo(span.Links())
			}
		}
	case "dropped_links_count":
		getter = func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) interface{} {
			return span.DroppedLinksCount()
		}
		setter = func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource, val interface{}) {
			if i, ok := val.(int64); ok {
				span.SetDroppedLinksCount(uint32(i))
			}
		}
	case "status":
		if len(path) == 1 {
			getter = func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) interface{} {
				return span.Status()
			}
			setter = func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource, val interface{}) {
				if status, ok := val.(pdata.SpanStatus); ok {
					status.CopyTo(span.Status())
				}
			}
		} else {
			switch path[1].Name {
			case "code":
				getter = func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) interface{} {
					return span.Status().Code()
				}
				setter = func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource, val interface{}) {
					if i, ok := val.(int64); ok {
						span.Status().SetCode(pdata.StatusCode(i))
					}
				}
			case "message":
				getter = func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) interface{} {
					return span.Status().Message()
				}
				setter = func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource, val interface{}) {
					if str, ok := val.(string); ok {
						span.Status().SetMessage(str)
					}
				}
			}
		}
	default:
		return nil, fmt.Errorf("invalid path expression, unrecognized field %v", path[0].Name)
	}

	return pathGetSetter{
		getter: getter,
		setter: setter,
	}, nil
}

func getAttr(attrs pdata.AttributeMap, mapKey string) interface{} {
	val, ok := attrs.Get(mapKey)
	if !ok {
		return nil
	}
	switch val.Type() {
	case pdata.AttributeValueTypeString:
		return val.StringVal()
	case pdata.AttributeValueTypeBool:
		return val.BoolVal()
	case pdata.AttributeValueTypeInt:
		return val.IntVal()
	case pdata.AttributeValueTypeDouble:
		return val.DoubleVal()
	case pdata.AttributeValueTypeMap:
		return val.MapVal()
	case pdata.AttributeValueTypeArray:
		return val.SliceVal()
	case pdata.AttributeValueTypeBytes:
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
		arr := pdata.NewAttributeValueArray()
		for _, str := range v {
			arr.SliceVal().AppendEmpty().SetStringVal(str)
		}
		attrs.Upsert(mapKey, arr)
	case []bool:
		arr := pdata.NewAttributeValueArray()
		for _, b := range v {
			arr.SliceVal().AppendEmpty().SetBoolVal(b)
		}
		attrs.Upsert(mapKey, arr)
	case []int64:
		arr := pdata.NewAttributeValueArray()
		for _, i := range v {
			arr.SliceVal().AppendEmpty().SetIntVal(i)
		}
		attrs.Upsert(mapKey, arr)
	case []float64:
		arr := pdata.NewAttributeValueArray()
		for _, f := range v {
			arr.SliceVal().AppendEmpty().SetDoubleVal(f)
		}
		attrs.Upsert(mapKey, arr)
	case [][]byte:
		arr := pdata.NewAttributeValueArray()
		for _, b := range v {
			arr.SliceVal().AppendEmpty().SetBytesVal(b)
		}
		attrs.Upsert(mapKey, arr)
	default:
		// TODO(anuraaga): Support set of map type.
	}
}
