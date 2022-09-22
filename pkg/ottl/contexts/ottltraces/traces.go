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

package ottltraces // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottltraces"

import (
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/otel/trace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ottlcommon"
)

type transformContext struct {
	span                 ptrace.Span
	instrumentationScope pcommon.InstrumentationScope
	resource             pcommon.Resource
}

func NewTransformContext(span ptrace.Span, instrumentationScope pcommon.InstrumentationScope, resource pcommon.Resource) ottl.TransformContext {
	return transformContext{
		span:                 span,
		instrumentationScope: instrumentationScope,
		resource:             resource,
	}
}

func (ctx transformContext) GetItem() interface{} {
	return ctx.span
}

func (ctx transformContext) GetInstrumentationScope() pcommon.InstrumentationScope {
	return ctx.instrumentationScope
}

func (ctx transformContext) GetResource() pcommon.Resource {
	return ctx.resource
}

var symbolTable = map[ottl.EnumSymbol]ottl.Enum{
	"SPAN_KIND_UNSPECIFIED": ottl.Enum(ptrace.SpanKindUnspecified),
	"SPAN_KIND_INTERNAL":    ottl.Enum(ptrace.SpanKindInternal),
	"SPAN_KIND_SERVER":      ottl.Enum(ptrace.SpanKindServer),
	"SPAN_KIND_CLIENT":      ottl.Enum(ptrace.SpanKindClient),
	"SPAN_KIND_PRODUCER":    ottl.Enum(ptrace.SpanKindProducer),
	"SPAN_KIND_CONSUMER":    ottl.Enum(ptrace.SpanKindConsumer),
	"STATUS_CODE_UNSET":     ottl.Enum(ptrace.StatusCodeUnset),
	"STATUS_CODE_OK":        ottl.Enum(ptrace.StatusCodeOk),
	"STATUS_CODE_ERROR":     ottl.Enum(ptrace.StatusCodeError),
}

func ParseEnum(val *ottl.EnumSymbol) (*ottl.Enum, error) {
	if val != nil {
		if enum, ok := symbolTable[*val]; ok {
			return &enum, nil
		}
		return nil, fmt.Errorf("enum symbol, %s, not found", *val)
	}
	return nil, fmt.Errorf("enum symbol not provided")
}

func ParsePath(val *ottl.Path) (ottl.GetSetter, error) {
	if val != nil && len(val.Fields) > 0 {
		return newPathGetSetter(val.Fields)
	}
	return nil, fmt.Errorf("bad path %v", val)
}

func newPathGetSetter(path []ottl.Field) (ottl.GetSetter, error) {
	switch path[0].Name {
	case "resource":
		return ottlcommon.ResourcePathGetSetter(path[1:])
	case "instrumentation_library":
		return ottlcommon.ScopePathGetSetter(path[1:])
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

func accessTraceID() ottl.StandardGetSetter {
	return ottl.StandardGetSetter{
		Getter: func(ctx ottl.TransformContext) interface{} {
			return ctx.GetItem().(ptrace.Span).TraceID()
		},
		Setter: func(ctx ottl.TransformContext, val interface{}) {
			if newTraceID, ok := val.(pcommon.TraceID); ok {
				ctx.GetItem().(ptrace.Span).SetTraceID(newTraceID)
			}
		},
	}
}

func accessStringTraceID() ottl.StandardGetSetter {
	return ottl.StandardGetSetter{
		Getter: func(ctx ottl.TransformContext) interface{} {
			return ctx.GetItem().(ptrace.Span).TraceID().HexString()
		},
		Setter: func(ctx ottl.TransformContext, val interface{}) {
			if str, ok := val.(string); ok {
				if traceID, err := parseTraceID(str); err == nil {
					ctx.GetItem().(ptrace.Span).SetTraceID(traceID)
				}
			}
		},
	}
}

func accessSpanID() ottl.StandardGetSetter {
	return ottl.StandardGetSetter{
		Getter: func(ctx ottl.TransformContext) interface{} {
			return ctx.GetItem().(ptrace.Span).SpanID()
		},
		Setter: func(ctx ottl.TransformContext, val interface{}) {
			if newSpanID, ok := val.(pcommon.SpanID); ok {
				ctx.GetItem().(ptrace.Span).SetSpanID(newSpanID)
			}
		},
	}
}

func accessStringSpanID() ottl.StandardGetSetter {
	return ottl.StandardGetSetter{
		Getter: func(ctx ottl.TransformContext) interface{} {
			return ctx.GetItem().(ptrace.Span).SpanID().HexString()
		},
		Setter: func(ctx ottl.TransformContext, val interface{}) {
			if str, ok := val.(string); ok {
				if spanID, err := parseSpanID(str); err == nil {
					ctx.GetItem().(ptrace.Span).SetSpanID(spanID)
				}
			}
		},
	}
}

func accessTraceState() ottl.StandardGetSetter {
	return ottl.StandardGetSetter{
		Getter: func(ctx ottl.TransformContext) interface{} {
			return ctx.GetItem().(ptrace.Span).TraceStateStruct().AsRaw()
		},
		Setter: func(ctx ottl.TransformContext, val interface{}) {
			if str, ok := val.(string); ok {
				ctx.GetItem().(ptrace.Span).TraceStateStruct().FromRaw(str)
			}
		},
	}
}

func accessTraceStateKey(mapKey *string) ottl.StandardGetSetter {
	return ottl.StandardGetSetter{
		Getter: func(ctx ottl.TransformContext) interface{} {
			if ts, err := trace.ParseTraceState(ctx.GetItem().(ptrace.Span).TraceStateStruct().AsRaw()); err == nil {
				return ts.Get(*mapKey)
			}
			return nil
		},
		Setter: func(ctx ottl.TransformContext, val interface{}) {
			if str, ok := val.(string); ok {
				if ts, err := trace.ParseTraceState(ctx.GetItem().(ptrace.Span).TraceStateStruct().AsRaw()); err == nil {
					if updated, err := ts.Insert(*mapKey, str); err == nil {
						ctx.GetItem().(ptrace.Span).TraceStateStruct().FromRaw(updated.String())
					}
				}
			}
		},
	}
}

func accessParentSpanID() ottl.StandardGetSetter {
	return ottl.StandardGetSetter{
		Getter: func(ctx ottl.TransformContext) interface{} {
			return ctx.GetItem().(ptrace.Span).ParentSpanID()
		},
		Setter: func(ctx ottl.TransformContext, val interface{}) {
			if newParentSpanID, ok := val.(pcommon.SpanID); ok {
				ctx.GetItem().(ptrace.Span).SetParentSpanID(newParentSpanID)
			}
		},
	}
}

func accessName() ottl.StandardGetSetter {
	return ottl.StandardGetSetter{
		Getter: func(ctx ottl.TransformContext) interface{} {
			return ctx.GetItem().(ptrace.Span).Name()
		},
		Setter: func(ctx ottl.TransformContext, val interface{}) {
			if str, ok := val.(string); ok {
				ctx.GetItem().(ptrace.Span).SetName(str)
			}
		},
	}
}

func accessKind() ottl.StandardGetSetter {
	return ottl.StandardGetSetter{
		Getter: func(ctx ottl.TransformContext) interface{} {
			return int64(ctx.GetItem().(ptrace.Span).Kind())
		},
		Setter: func(ctx ottl.TransformContext, val interface{}) {
			if i, ok := val.(int64); ok {
				ctx.GetItem().(ptrace.Span).SetKind(ptrace.SpanKind(i))
			}
		},
	}
}

func accessStartTimeUnixNano() ottl.StandardGetSetter {
	return ottl.StandardGetSetter{
		Getter: func(ctx ottl.TransformContext) interface{} {
			return ctx.GetItem().(ptrace.Span).StartTimestamp().AsTime().UnixNano()
		},
		Setter: func(ctx ottl.TransformContext, val interface{}) {
			if i, ok := val.(int64); ok {
				ctx.GetItem().(ptrace.Span).SetStartTimestamp(pcommon.NewTimestampFromTime(time.Unix(0, i)))
			}
		},
	}
}

func accessEndTimeUnixNano() ottl.StandardGetSetter {
	return ottl.StandardGetSetter{
		Getter: func(ctx ottl.TransformContext) interface{} {
			return ctx.GetItem().(ptrace.Span).EndTimestamp().AsTime().UnixNano()
		},
		Setter: func(ctx ottl.TransformContext, val interface{}) {
			if i, ok := val.(int64); ok {
				ctx.GetItem().(ptrace.Span).SetEndTimestamp(pcommon.NewTimestampFromTime(time.Unix(0, i)))
			}
		},
	}
}

func accessAttributes() ottl.StandardGetSetter {
	return ottl.StandardGetSetter{
		Getter: func(ctx ottl.TransformContext) interface{} {
			return ctx.GetItem().(ptrace.Span).Attributes()
		},
		Setter: func(ctx ottl.TransformContext, val interface{}) {
			if attrs, ok := val.(pcommon.Map); ok {
				attrs.CopyTo(ctx.GetItem().(ptrace.Span).Attributes())
			}
		},
	}
}

func accessAttributesKey(mapKey *string) ottl.StandardGetSetter {
	return ottl.StandardGetSetter{
		Getter: func(ctx ottl.TransformContext) interface{} {
			return ottlcommon.GetMapValue(ctx.GetItem().(ptrace.Span).Attributes(), *mapKey)
		},
		Setter: func(ctx ottl.TransformContext, val interface{}) {
			ottlcommon.SetMapValue(ctx.GetItem().(ptrace.Span).Attributes(), *mapKey, val)
		},
	}
}

func accessDroppedAttributesCount() ottl.StandardGetSetter {
	return ottl.StandardGetSetter{
		Getter: func(ctx ottl.TransformContext) interface{} {
			return int64(ctx.GetItem().(ptrace.Span).DroppedAttributesCount())
		},
		Setter: func(ctx ottl.TransformContext, val interface{}) {
			if i, ok := val.(int64); ok {
				ctx.GetItem().(ptrace.Span).SetDroppedAttributesCount(uint32(i))
			}
		},
	}
}

func accessEvents() ottl.StandardGetSetter {
	return ottl.StandardGetSetter{
		Getter: func(ctx ottl.TransformContext) interface{} {
			return ctx.GetItem().(ptrace.Span).Events()
		},
		Setter: func(ctx ottl.TransformContext, val interface{}) {
			if slc, ok := val.(ptrace.SpanEventSlice); ok {
				ctx.GetItem().(ptrace.Span).Events().RemoveIf(func(event ptrace.SpanEvent) bool {
					return true
				})
				slc.CopyTo(ctx.GetItem().(ptrace.Span).Events())
			}
		},
	}
}

func accessDroppedEventsCount() ottl.StandardGetSetter {
	return ottl.StandardGetSetter{
		Getter: func(ctx ottl.TransformContext) interface{} {
			return int64(ctx.GetItem().(ptrace.Span).DroppedEventsCount())
		},
		Setter: func(ctx ottl.TransformContext, val interface{}) {
			if i, ok := val.(int64); ok {
				ctx.GetItem().(ptrace.Span).SetDroppedEventsCount(uint32(i))
			}
		},
	}
}

func accessLinks() ottl.StandardGetSetter {
	return ottl.StandardGetSetter{
		Getter: func(ctx ottl.TransformContext) interface{} {
			return ctx.GetItem().(ptrace.Span).Links()
		},
		Setter: func(ctx ottl.TransformContext, val interface{}) {
			if slc, ok := val.(ptrace.SpanLinkSlice); ok {
				ctx.GetItem().(ptrace.Span).Links().RemoveIf(func(event ptrace.SpanLink) bool {
					return true
				})
				slc.CopyTo(ctx.GetItem().(ptrace.Span).Links())
			}
		},
	}
}

func accessDroppedLinksCount() ottl.StandardGetSetter {
	return ottl.StandardGetSetter{
		Getter: func(ctx ottl.TransformContext) interface{} {
			return int64(ctx.GetItem().(ptrace.Span).DroppedLinksCount())
		},
		Setter: func(ctx ottl.TransformContext, val interface{}) {
			if i, ok := val.(int64); ok {
				ctx.GetItem().(ptrace.Span).SetDroppedLinksCount(uint32(i))
			}
		},
	}
}

func accessStatus() ottl.StandardGetSetter {
	return ottl.StandardGetSetter{
		Getter: func(ctx ottl.TransformContext) interface{} {
			return ctx.GetItem().(ptrace.Span).Status()
		},
		Setter: func(ctx ottl.TransformContext, val interface{}) {
			if status, ok := val.(ptrace.SpanStatus); ok {
				status.CopyTo(ctx.GetItem().(ptrace.Span).Status())
			}
		},
	}
}

func accessStatusCode() ottl.StandardGetSetter {
	return ottl.StandardGetSetter{
		Getter: func(ctx ottl.TransformContext) interface{} {
			return int64(ctx.GetItem().(ptrace.Span).Status().Code())
		},
		Setter: func(ctx ottl.TransformContext, val interface{}) {
			if i, ok := val.(int64); ok {
				ctx.GetItem().(ptrace.Span).Status().SetCode(ptrace.StatusCode(i))
			}
		},
	}
}

func accessStatusMessage() ottl.StandardGetSetter {
	return ottl.StandardGetSetter{
		Getter: func(ctx ottl.TransformContext) interface{} {
			return ctx.GetItem().(ptrace.Span).Status().Message()
		},
		Setter: func(ctx ottl.TransformContext, val interface{}) {
			if str, ok := val.(string); ok {
				ctx.GetItem().(ptrace.Span).Status().SetMessage(str)
			}
		},
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
	return pcommon.SpanID(idArr), nil
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
	return pcommon.TraceID(idArr), nil
}
