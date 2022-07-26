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
package traces // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/traces"

import (
	"fmt"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/otel/trace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/telemetryquerylanguage/tql"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"
)

type spanTransformContext struct {
	span     ptrace.Span
	il       pcommon.InstrumentationScope
	resource pcommon.Resource
}

func (ctx spanTransformContext) GetItem() interface{} {
	return ctx.span
}

func (ctx spanTransformContext) GetInstrumentationScope() pcommon.InstrumentationScope {
	return ctx.il
}

func (ctx spanTransformContext) GetResource() pcommon.Resource {
	return ctx.resource
}

var symbolTable = map[tql.EnumSymbol]tql.Enum{
	"SPAN_KIND_UNSPECIFIED": 0,
	"SPAN_KIND_INTERNAL":    1,
	"SPAN_KIND_SERVER":      2,
	"SPAN_KIND_CLIENT":      3,
	"SPAN_KIND_PRODUCER":    4,
	"SPAN_KIND_CONSUMER":    5,
	"STATUS_CODE_UNSET":     0,
	"STATUS_CODE_OK":        1,
	"STATUS_CODE_ERROR":     2,
}

func ParseEnum(val *tql.EnumSymbol) (*tql.Enum, error) {
	if val != nil {
		if enum, ok := symbolTable[*val]; ok {
			return &enum, nil
		}
		return nil, fmt.Errorf("enum symbol, %s, not found", *val)
	}
	return nil, fmt.Errorf("enum symbol not provided")
}

func ParsePath(val *tql.Path) (tql.GetSetter, error) {
	if val != nil && len(val.Fields) > 0 {
		return newPathGetSetter(val.Fields)
	}
	return nil, fmt.Errorf("bad path %v", val)
}

func newPathGetSetter(path []tql.Field) (tql.GetSetter, error) {
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
		if len(path) == 1 {
			return accessTraceID(), nil
		}
		switch path[1].Name {
		case "string":
			return accessStringTraceID(), nil
		}
	case "span_id":
		if len(path) == 1 {
			return accessSpanID(), nil
		}
		switch path[1].Name {
		case "string":
			return accessStringSpanID(), nil
		}
	case "trace_state":
		mapKey := path[0].MapKey
		if mapKey == nil {
			return accessTraceState(), nil
		}
		return accessTraceStateKey(mapKey), nil
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

func accessResource() tql.StandardGetSetter {
	return tql.StandardGetSetter{
		Getter: func(ctx tql.TransformContext) interface{} {
			return ctx.GetResource()
		},
		Setter: func(ctx tql.TransformContext, val interface{}) {
			if newRes, ok := val.(pcommon.Resource); ok {
				ctx.GetResource().Attributes().Clear()
				newRes.CopyTo(ctx.GetResource())
			}
		},
	}
}

func accessResourceAttributes() tql.StandardGetSetter {
	return tql.StandardGetSetter{
		Getter: func(ctx tql.TransformContext) interface{} {
			return ctx.GetResource().Attributes()
		},
		Setter: func(ctx tql.TransformContext, val interface{}) {
			if attrs, ok := val.(pcommon.Map); ok {
				ctx.GetResource().Attributes().Clear()
				attrs.CopyTo(ctx.GetResource().Attributes())
			}
		},
	}
}

func accessResourceAttributesKey(mapKey *string) tql.StandardGetSetter {
	return tql.StandardGetSetter{
		Getter: func(ctx tql.TransformContext) interface{} {
			return getAttr(ctx.GetResource().Attributes(), *mapKey)
		},
		Setter: func(ctx tql.TransformContext, val interface{}) {
			setAttr(ctx.GetResource().Attributes(), *mapKey, val)
		},
	}
}

func accessInstrumentationScope() tql.StandardGetSetter {
	return tql.StandardGetSetter{
		Getter: func(ctx tql.TransformContext) interface{} {
			return ctx.GetInstrumentationScope()
		},
		Setter: func(ctx tql.TransformContext, val interface{}) {
			if newIl, ok := val.(pcommon.InstrumentationScope); ok {
				newIl.CopyTo(ctx.GetInstrumentationScope())
			}
		},
	}
}

func accessInstrumentationScopeName() tql.StandardGetSetter {
	return tql.StandardGetSetter{
		Getter: func(ctx tql.TransformContext) interface{} {
			return ctx.GetInstrumentationScope().Name()
		},
		Setter: func(ctx tql.TransformContext, val interface{}) {
			if str, ok := val.(string); ok {
				ctx.GetInstrumentationScope().SetName(str)
			}
		},
	}
}

func accessInstrumentationScopeVersion() tql.StandardGetSetter {
	return tql.StandardGetSetter{
		Getter: func(ctx tql.TransformContext) interface{} {
			return ctx.GetInstrumentationScope().Version()
		},
		Setter: func(ctx tql.TransformContext, val interface{}) {
			if str, ok := val.(string); ok {
				ctx.GetInstrumentationScope().SetVersion(str)
			}
		},
	}
}

func accessTraceID() tql.StandardGetSetter {
	return tql.StandardGetSetter{
		Getter: func(ctx tql.TransformContext) interface{} {
			return ctx.GetItem().(ptrace.Span).TraceID()
		},
		Setter: func(ctx tql.TransformContext, val interface{}) {
			if newTraceID, ok := val.(pcommon.TraceID); ok {
				ctx.GetItem().(ptrace.Span).SetTraceID(newTraceID)
			}
		},
	}
}

func accessStringTraceID() tql.StandardGetSetter {
	return tql.StandardGetSetter{
		Getter: func(ctx tql.TransformContext) interface{} {
			return ctx.GetItem().(ptrace.Span).TraceID().HexString()
		},
		Setter: func(ctx tql.TransformContext, val interface{}) {
			if str, ok := val.(string); ok {
				if traceID, err := common.ParseTraceID(str); err == nil {
					ctx.GetItem().(ptrace.Span).SetTraceID(traceID)
				}
			}
		},
	}
}

func accessSpanID() tql.StandardGetSetter {
	return tql.StandardGetSetter{
		Getter: func(ctx tql.TransformContext) interface{} {
			return ctx.GetItem().(ptrace.Span).SpanID()
		},
		Setter: func(ctx tql.TransformContext, val interface{}) {
			if newSpanID, ok := val.(pcommon.SpanID); ok {
				ctx.GetItem().(ptrace.Span).SetSpanID(newSpanID)
			}
		},
	}
}

func accessStringSpanID() tql.StandardGetSetter {
	return tql.StandardGetSetter{
		Getter: func(ctx tql.TransformContext) interface{} {
			return ctx.GetItem().(ptrace.Span).SpanID().HexString()
		},
		Setter: func(ctx tql.TransformContext, val interface{}) {
			if str, ok := val.(string); ok {
				if spanID, err := common.ParseSpanID(str); err == nil {
					ctx.GetItem().(ptrace.Span).SetSpanID(spanID)
				}
			}
		},
	}
}

func accessTraceState() tql.StandardGetSetter {
	return tql.StandardGetSetter{
		Getter: func(ctx tql.TransformContext) interface{} {
			return (string)(ctx.GetItem().(ptrace.Span).TraceState())
		},
		Setter: func(ctx tql.TransformContext, val interface{}) {
			if str, ok := val.(string); ok {
				ctx.GetItem().(ptrace.Span).SetTraceState(ptrace.TraceState(str))
			}
		},
	}
}

func accessTraceStateKey(mapKey *string) tql.StandardGetSetter {
	return tql.StandardGetSetter{
		Getter: func(ctx tql.TransformContext) interface{} {
			if ts, err := trace.ParseTraceState(string(ctx.GetItem().(ptrace.Span).TraceState())); err == nil {
				return ts.Get(*mapKey)
			}
			return nil
		},
		Setter: func(ctx tql.TransformContext, val interface{}) {
			if str, ok := val.(string); ok {
				if ts, err := trace.ParseTraceState(string(ctx.GetItem().(ptrace.Span).TraceState())); err == nil {
					if updated, err := ts.Insert(*mapKey, str); err == nil {
						ctx.GetItem().(ptrace.Span).SetTraceState(ptrace.TraceState(updated.String()))
					}
				}
			}
		},
	}
}

func accessParentSpanID() tql.StandardGetSetter {
	return tql.StandardGetSetter{
		Getter: func(ctx tql.TransformContext) interface{} {
			return ctx.GetItem().(ptrace.Span).ParentSpanID()
		},
		Setter: func(ctx tql.TransformContext, val interface{}) {
			if newParentSpanID, ok := val.(pcommon.SpanID); ok {
				ctx.GetItem().(ptrace.Span).SetParentSpanID(newParentSpanID)
			}
		},
	}
}

func accessName() tql.StandardGetSetter {
	return tql.StandardGetSetter{
		Getter: func(ctx tql.TransformContext) interface{} {
			return ctx.GetItem().(ptrace.Span).Name()
		},
		Setter: func(ctx tql.TransformContext, val interface{}) {
			if str, ok := val.(string); ok {
				ctx.GetItem().(ptrace.Span).SetName(str)
			}
		},
	}
}

func accessKind() tql.StandardGetSetter {
	return tql.StandardGetSetter{
		Getter: func(ctx tql.TransformContext) interface{} {
			return int64(ctx.GetItem().(ptrace.Span).Kind())
		},
		Setter: func(ctx tql.TransformContext, val interface{}) {
			if i, ok := val.(int64); ok {
				ctx.GetItem().(ptrace.Span).SetKind(ptrace.SpanKind(i))
			}
		},
	}
}

func accessStartTimeUnixNano() tql.StandardGetSetter {
	return tql.StandardGetSetter{
		Getter: func(ctx tql.TransformContext) interface{} {
			return ctx.GetItem().(ptrace.Span).StartTimestamp().AsTime().UnixNano()
		},
		Setter: func(ctx tql.TransformContext, val interface{}) {
			if i, ok := val.(int64); ok {
				ctx.GetItem().(ptrace.Span).SetStartTimestamp(pcommon.NewTimestampFromTime(time.Unix(0, i)))
			}
		},
	}
}

func accessEndTimeUnixNano() tql.StandardGetSetter {
	return tql.StandardGetSetter{
		Getter: func(ctx tql.TransformContext) interface{} {
			return ctx.GetItem().(ptrace.Span).EndTimestamp().AsTime().UnixNano()
		},
		Setter: func(ctx tql.TransformContext, val interface{}) {
			if i, ok := val.(int64); ok {
				ctx.GetItem().(ptrace.Span).SetEndTimestamp(pcommon.NewTimestampFromTime(time.Unix(0, i)))
			}
		},
	}
}

func accessAttributes() tql.StandardGetSetter {
	return tql.StandardGetSetter{
		Getter: func(ctx tql.TransformContext) interface{} {
			return ctx.GetItem().(ptrace.Span).Attributes()
		},
		Setter: func(ctx tql.TransformContext, val interface{}) {
			if attrs, ok := val.(pcommon.Map); ok {
				ctx.GetItem().(ptrace.Span).Attributes().Clear()
				attrs.CopyTo(ctx.GetItem().(ptrace.Span).Attributes())
			}
		},
	}
}

func accessAttributesKey(mapKey *string) tql.StandardGetSetter {
	return tql.StandardGetSetter{
		Getter: func(ctx tql.TransformContext) interface{} {
			return getAttr(ctx.GetItem().(ptrace.Span).Attributes(), *mapKey)
		},
		Setter: func(ctx tql.TransformContext, val interface{}) {
			setAttr(ctx.GetItem().(ptrace.Span).Attributes(), *mapKey, val)
		},
	}
}

func accessDroppedAttributesCount() tql.StandardGetSetter {
	return tql.StandardGetSetter{
		Getter: func(ctx tql.TransformContext) interface{} {
			return int64(ctx.GetItem().(ptrace.Span).DroppedAttributesCount())
		},
		Setter: func(ctx tql.TransformContext, val interface{}) {
			if i, ok := val.(int64); ok {
				ctx.GetItem().(ptrace.Span).SetDroppedAttributesCount(uint32(i))
			}
		},
	}
}

func accessEvents() tql.StandardGetSetter {
	return tql.StandardGetSetter{
		Getter: func(ctx tql.TransformContext) interface{} {
			return ctx.GetItem().(ptrace.Span).Events()
		},
		Setter: func(ctx tql.TransformContext, val interface{}) {
			if slc, ok := val.(ptrace.SpanEventSlice); ok {
				ctx.GetItem().(ptrace.Span).Events().RemoveIf(func(event ptrace.SpanEvent) bool {
					return true
				})
				slc.CopyTo(ctx.GetItem().(ptrace.Span).Events())
			}
		},
	}
}

func accessDroppedEventsCount() tql.StandardGetSetter {
	return tql.StandardGetSetter{
		Getter: func(ctx tql.TransformContext) interface{} {
			return int64(ctx.GetItem().(ptrace.Span).DroppedEventsCount())
		},
		Setter: func(ctx tql.TransformContext, val interface{}) {
			if i, ok := val.(int64); ok {
				ctx.GetItem().(ptrace.Span).SetDroppedEventsCount(uint32(i))
			}
		},
	}
}

func accessLinks() tql.StandardGetSetter {
	return tql.StandardGetSetter{
		Getter: func(ctx tql.TransformContext) interface{} {
			return ctx.GetItem().(ptrace.Span).Links()
		},
		Setter: func(ctx tql.TransformContext, val interface{}) {
			if slc, ok := val.(ptrace.SpanLinkSlice); ok {
				ctx.GetItem().(ptrace.Span).Links().RemoveIf(func(event ptrace.SpanLink) bool {
					return true
				})
				slc.CopyTo(ctx.GetItem().(ptrace.Span).Links())
			}
		},
	}
}

func accessDroppedLinksCount() tql.StandardGetSetter {
	return tql.StandardGetSetter{
		Getter: func(ctx tql.TransformContext) interface{} {
			return int64(ctx.GetItem().(ptrace.Span).DroppedLinksCount())
		},
		Setter: func(ctx tql.TransformContext, val interface{}) {
			if i, ok := val.(int64); ok {
				ctx.GetItem().(ptrace.Span).SetDroppedLinksCount(uint32(i))
			}
		},
	}
}

func accessStatus() tql.StandardGetSetter {
	return tql.StandardGetSetter{
		Getter: func(ctx tql.TransformContext) interface{} {
			return ctx.GetItem().(ptrace.Span).Status()
		},
		Setter: func(ctx tql.TransformContext, val interface{}) {
			if status, ok := val.(ptrace.SpanStatus); ok {
				status.CopyTo(ctx.GetItem().(ptrace.Span).Status())
			}
		},
	}
}

func accessStatusCode() tql.StandardGetSetter {
	return tql.StandardGetSetter{
		Getter: func(ctx tql.TransformContext) interface{} {
			return int64(ctx.GetItem().(ptrace.Span).Status().Code())
		},
		Setter: func(ctx tql.TransformContext, val interface{}) {
			if i, ok := val.(int64); ok {
				ctx.GetItem().(ptrace.Span).Status().SetCode(ptrace.StatusCode(i))
			}
		},
	}
}

func accessStatusMessage() tql.StandardGetSetter {
	return tql.StandardGetSetter{
		Getter: func(ctx tql.TransformContext) interface{} {
			return ctx.GetItem().(ptrace.Span).Status().Message()
		},
		Setter: func(ctx tql.TransformContext, val interface{}) {
			if str, ok := val.(string); ok {
				ctx.GetItem().(ptrace.Span).Status().SetMessage(str)
			}
		},
	}
}

func getAttr(attrs pcommon.Map, mapKey string) interface{} {
	val, ok := attrs.Get(mapKey)
	if !ok {
		return nil
	}
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
