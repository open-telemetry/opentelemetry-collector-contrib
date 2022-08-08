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

package tqltraces // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/telemetryquerylanguage/contexts/tqltraces"

import (
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/otel/trace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/telemetryquerylanguage/tql"
)

type SpanTransformContext struct {
	Span                 ptrace.Span
	InstrumentationScope pcommon.InstrumentationScope
	Resource             pcommon.Resource
}

func (ctx SpanTransformContext) GetItem() interface{} {
	return ctx.Span
}

func (ctx SpanTransformContext) GetInstrumentationScope() pcommon.InstrumentationScope {
	return ctx.InstrumentationScope
}

func (ctx SpanTransformContext) GetResource() pcommon.Resource {
	return ctx.Resource
}

// pathGetSetter is a getSetter which has been resolved using a path expression provided by a user.
type pathGetSetter struct {
	getter tql.ExprFunc
	setter func(ctx tql.TransformContext, val interface{})
}

func (path pathGetSetter) Get(ctx tql.TransformContext) interface{} {
	return path.getter(ctx)
}

func (path pathGetSetter) Set(ctx tql.TransformContext, val interface{}) {
	path.setter(ctx, val)
}

var symbolTable = map[tql.EnumSymbol]tql.Enum{
	"SPAN_KIND_UNSPECIFIED": tql.Enum(ptrace.SpanKindUnspecified),
	"SPAN_KIND_INTERNAL":    tql.Enum(ptrace.SpanKindInternal),
	"SPAN_KIND_SERVER":      tql.Enum(ptrace.SpanKindServer),
	"SPAN_KIND_CLIENT":      tql.Enum(ptrace.SpanKindClient),
	"SPAN_KIND_PRODUCER":    tql.Enum(ptrace.SpanKindProducer),
	"SPAN_KIND_CONSUMER":    tql.Enum(ptrace.SpanKindConsumer),
	"STATUS_CODE_UNSET":     tql.Enum(ptrace.StatusCodeUnset),
	"STATUS_CODE_OK":        tql.Enum(ptrace.StatusCodeOk),
	"STATUS_CODE_ERROR":     tql.Enum(ptrace.StatusCodeError),
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
		if path[1].Name == "attributes" {
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
		if path[1].Name == "string" {
			return accessStringTraceID(), nil
		}
	case "span_id":
		if len(path) == 1 {
			return accessSpanID(), nil
		}
		if path[1].Name == "string" {
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

func accessResource() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx tql.TransformContext) interface{} {
			return ctx.GetResource()
		},
		setter: func(ctx tql.TransformContext, val interface{}) {
			if newRes, ok := val.(pcommon.Resource); ok {
				ctx.GetResource().Attributes().Clear()
				newRes.CopyTo(ctx.GetResource())
			}
		},
	}
}

func accessResourceAttributes() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx tql.TransformContext) interface{} {
			return ctx.GetResource().Attributes()
		},
		setter: func(ctx tql.TransformContext, val interface{}) {
			if attrs, ok := val.(pcommon.Map); ok {
				ctx.GetResource().Attributes().Clear()
				attrs.CopyTo(ctx.GetResource().Attributes())
			}
		},
	}
}

func accessResourceAttributesKey(mapKey *string) pathGetSetter {
	return pathGetSetter{
		getter: func(ctx tql.TransformContext) interface{} {
			return getAttr(ctx.GetResource().Attributes(), *mapKey)
		},
		setter: func(ctx tql.TransformContext, val interface{}) {
			setAttr(ctx.GetResource().Attributes(), *mapKey, val)
		},
	}
}

func accessInstrumentationScope() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx tql.TransformContext) interface{} {
			return ctx.GetInstrumentationScope()
		},
		setter: func(ctx tql.TransformContext, val interface{}) {
			if newIl, ok := val.(pcommon.InstrumentationScope); ok {
				newIl.CopyTo(ctx.GetInstrumentationScope())
			}
		},
	}
}

func accessInstrumentationScopeName() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx tql.TransformContext) interface{} {
			return ctx.GetInstrumentationScope().Name()
		},
		setter: func(ctx tql.TransformContext, val interface{}) {
			if str, ok := val.(string); ok {
				ctx.GetInstrumentationScope().SetName(str)
			}
		},
	}
}

func accessInstrumentationScopeVersion() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx tql.TransformContext) interface{} {
			return ctx.GetInstrumentationScope().Version()
		},
		setter: func(ctx tql.TransformContext, val interface{}) {
			if str, ok := val.(string); ok {
				ctx.GetInstrumentationScope().SetVersion(str)
			}
		},
	}
}

func accessTraceID() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx tql.TransformContext) interface{} {
			return ctx.GetItem().(ptrace.Span).TraceID()
		},
		setter: func(ctx tql.TransformContext, val interface{}) {
			if newTraceID, ok := val.(pcommon.TraceID); ok {
				ctx.GetItem().(ptrace.Span).SetTraceID(newTraceID)
			}
		},
	}
}

func accessStringTraceID() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx tql.TransformContext) interface{} {
			return ctx.GetItem().(ptrace.Span).TraceID().HexString()
		},
		setter: func(ctx tql.TransformContext, val interface{}) {
			if str, ok := val.(string); ok {
				if traceID, err := parseTraceID(str); err == nil {
					ctx.GetItem().(ptrace.Span).SetTraceID(traceID)
				}
			}
		},
	}
}

func accessSpanID() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx tql.TransformContext) interface{} {
			return ctx.GetItem().(ptrace.Span).SpanID()
		},
		setter: func(ctx tql.TransformContext, val interface{}) {
			if newSpanID, ok := val.(pcommon.SpanID); ok {
				ctx.GetItem().(ptrace.Span).SetSpanID(newSpanID)
			}
		},
	}
}

func accessStringSpanID() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx tql.TransformContext) interface{} {
			return ctx.GetItem().(ptrace.Span).SpanID().HexString()
		},
		setter: func(ctx tql.TransformContext, val interface{}) {
			if str, ok := val.(string); ok {
				if spanID, err := parseSpanID(str); err == nil {
					ctx.GetItem().(ptrace.Span).SetSpanID(spanID)
				}
			}
		},
	}
}

func accessTraceState() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx tql.TransformContext) interface{} {
			return (string)(ctx.GetItem().(ptrace.Span).TraceState())
		},
		setter: func(ctx tql.TransformContext, val interface{}) {
			if str, ok := val.(string); ok {
				ctx.GetItem().(ptrace.Span).SetTraceState(ptrace.TraceState(str))
			}
		},
	}
}

func accessTraceStateKey(mapKey *string) pathGetSetter {
	return pathGetSetter{
		getter: func(ctx tql.TransformContext) interface{} {
			if ts, err := trace.ParseTraceState(string(ctx.GetItem().(ptrace.Span).TraceState())); err == nil {
				return ts.Get(*mapKey)
			}
			return nil
		},
		setter: func(ctx tql.TransformContext, val interface{}) {
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

func accessParentSpanID() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx tql.TransformContext) interface{} {
			return ctx.GetItem().(ptrace.Span).ParentSpanID()
		},
		setter: func(ctx tql.TransformContext, val interface{}) {
			if newParentSpanID, ok := val.(pcommon.SpanID); ok {
				ctx.GetItem().(ptrace.Span).SetParentSpanID(newParentSpanID)
			}
		},
	}
}

func accessName() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx tql.TransformContext) interface{} {
			return ctx.GetItem().(ptrace.Span).Name()
		},
		setter: func(ctx tql.TransformContext, val interface{}) {
			if str, ok := val.(string); ok {
				ctx.GetItem().(ptrace.Span).SetName(str)
			}
		},
	}
}

func accessKind() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx tql.TransformContext) interface{} {
			return int64(ctx.GetItem().(ptrace.Span).Kind())
		},
		setter: func(ctx tql.TransformContext, val interface{}) {
			if i, ok := val.(int64); ok {
				ctx.GetItem().(ptrace.Span).SetKind(ptrace.SpanKind(i))
			}
		},
	}
}

func accessStartTimeUnixNano() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx tql.TransformContext) interface{} {
			return ctx.GetItem().(ptrace.Span).StartTimestamp().AsTime().UnixNano()
		},
		setter: func(ctx tql.TransformContext, val interface{}) {
			if i, ok := val.(int64); ok {
				ctx.GetItem().(ptrace.Span).SetStartTimestamp(pcommon.NewTimestampFromTime(time.Unix(0, i)))
			}
		},
	}
}

func accessEndTimeUnixNano() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx tql.TransformContext) interface{} {
			return ctx.GetItem().(ptrace.Span).EndTimestamp().AsTime().UnixNano()
		},
		setter: func(ctx tql.TransformContext, val interface{}) {
			if i, ok := val.(int64); ok {
				ctx.GetItem().(ptrace.Span).SetEndTimestamp(pcommon.NewTimestampFromTime(time.Unix(0, i)))
			}
		},
	}
}

func accessAttributes() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx tql.TransformContext) interface{} {
			return ctx.GetItem().(ptrace.Span).Attributes()
		},
		setter: func(ctx tql.TransformContext, val interface{}) {
			if attrs, ok := val.(pcommon.Map); ok {
				ctx.GetItem().(ptrace.Span).Attributes().Clear()
				attrs.CopyTo(ctx.GetItem().(ptrace.Span).Attributes())
			}
		},
	}
}

func accessAttributesKey(mapKey *string) pathGetSetter {
	return pathGetSetter{
		getter: func(ctx tql.TransformContext) interface{} {
			return getAttr(ctx.GetItem().(ptrace.Span).Attributes(), *mapKey)
		},
		setter: func(ctx tql.TransformContext, val interface{}) {
			setAttr(ctx.GetItem().(ptrace.Span).Attributes(), *mapKey, val)
		},
	}
}

func accessDroppedAttributesCount() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx tql.TransformContext) interface{} {
			return int64(ctx.GetItem().(ptrace.Span).DroppedAttributesCount())
		},
		setter: func(ctx tql.TransformContext, val interface{}) {
			if i, ok := val.(int64); ok {
				ctx.GetItem().(ptrace.Span).SetDroppedAttributesCount(uint32(i))
			}
		},
	}
}

func accessEvents() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx tql.TransformContext) interface{} {
			return ctx.GetItem().(ptrace.Span).Events()
		},
		setter: func(ctx tql.TransformContext, val interface{}) {
			if slc, ok := val.(ptrace.SpanEventSlice); ok {
				ctx.GetItem().(ptrace.Span).Events().RemoveIf(func(event ptrace.SpanEvent) bool {
					return true
				})
				slc.CopyTo(ctx.GetItem().(ptrace.Span).Events())
			}
		},
	}
}

func accessDroppedEventsCount() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx tql.TransformContext) interface{} {
			return int64(ctx.GetItem().(ptrace.Span).DroppedEventsCount())
		},
		setter: func(ctx tql.TransformContext, val interface{}) {
			if i, ok := val.(int64); ok {
				ctx.GetItem().(ptrace.Span).SetDroppedEventsCount(uint32(i))
			}
		},
	}
}

func accessLinks() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx tql.TransformContext) interface{} {
			return ctx.GetItem().(ptrace.Span).Links()
		},
		setter: func(ctx tql.TransformContext, val interface{}) {
			if slc, ok := val.(ptrace.SpanLinkSlice); ok {
				ctx.GetItem().(ptrace.Span).Links().RemoveIf(func(event ptrace.SpanLink) bool {
					return true
				})
				slc.CopyTo(ctx.GetItem().(ptrace.Span).Links())
			}
		},
	}
}

func accessDroppedLinksCount() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx tql.TransformContext) interface{} {
			return int64(ctx.GetItem().(ptrace.Span).DroppedLinksCount())
		},
		setter: func(ctx tql.TransformContext, val interface{}) {
			if i, ok := val.(int64); ok {
				ctx.GetItem().(ptrace.Span).SetDroppedLinksCount(uint32(i))
			}
		},
	}
}

func accessStatus() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx tql.TransformContext) interface{} {
			return ctx.GetItem().(ptrace.Span).Status()
		},
		setter: func(ctx tql.TransformContext, val interface{}) {
			if status, ok := val.(ptrace.SpanStatus); ok {
				status.CopyTo(ctx.GetItem().(ptrace.Span).Status())
			}
		},
	}
}

func accessStatusCode() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx tql.TransformContext) interface{} {
			return int64(ctx.GetItem().(ptrace.Span).Status().Code())
		},
		setter: func(ctx tql.TransformContext, val interface{}) {
			if i, ok := val.(int64); ok {
				ctx.GetItem().(ptrace.Span).Status().SetCode(ptrace.StatusCode(i))
			}
		},
	}
}

func accessStatusMessage() pathGetSetter {
	return pathGetSetter{
		getter: func(ctx tql.TransformContext) interface{} {
			return ctx.GetItem().(ptrace.Span).Status().Message()
		},
		setter: func(ctx tql.TransformContext, val interface{}) {
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
		return val.BytesVal().AsRaw()
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

func parseSpanID(spanIDStr string) (pcommon.SpanID, error) {
	id, err := hex.DecodeString(spanIDStr)
	if err != nil {
		return pcommon.SpanID{}, err
	}
	if len(id) != 8 {
		return pcommon.SpanID{}, errors.New("span ids must be 8 bytes")
	}
	var idArr [8]byte
	copy(idArr[:8], id)
	return pcommon.NewSpanID(idArr), nil
}

func parseTraceID(traceIDStr string) (pcommon.TraceID, error) {
	id, err := hex.DecodeString(traceIDStr)
	if err != nil {
		return pcommon.TraceID{}, err
	}
	if len(id) != 16 {
		return pcommon.TraceID{}, errors.New("traces ids must be 16 bytes")
	}
	var idArr [16]byte
	copy(idArr[:16], id)
	return pcommon.NewTraceID(idArr), nil
}
