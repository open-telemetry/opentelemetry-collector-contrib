// Copyright The OpenTelemetry Authors
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

package ottlcommon // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/internal/ottlcommon"

import (
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/otel/trace"
	"time"
)

type SpanContext interface {
	GetSpan() ptrace.Span
}

func SpanPathGetSetter[K SpanContext](path []ottl.Field) (ottl.GetSetter[K], error) {
	if len(path) == 0 {
		return accessSpan[K](), nil
	}

	switch path[0].Name {
	case "trace_id":
		if len(path) == 1 {
			return accessTraceID[K](), nil
		}
		if path[1].Name == "string" {
			return accessStringTraceID[K](), nil
		}
	case "span_id":
		if len(path) == 1 {
			return accessSpanID[K](), nil
		}
		if path[1].Name == "string" {
			return accessStringSpanID[K](), nil
		}
	case "trace_state":
		mapKey := path[0].MapKey
		if mapKey == nil {
			return accessTraceState[K](), nil
		}
		return accessTraceStateKey[K](mapKey), nil
	case "parent_span_id":
		return accessParentSpanID[K](), nil
	case "name":
		return accessName[K](), nil
	case "kind":
		return accessKind[K](), nil
	case "start_time_unix_nano":
		return accessStartTimeUnixNano[K](), nil
	case "end_time_unix_nano":
		return accessEndTimeUnixNano[K](), nil
	case "attributes":
		mapKey := path[0].MapKey
		if mapKey == nil {
			return accessAttributes[K](), nil
		}
		return accessAttributesKey[K](mapKey), nil
	case "dropped_attributes_count":
		return accessSpanDroppedAttributesCount[K](), nil
	case "events":
		return accessEvents[K](), nil
	case "dropped_events_count":
		return accessDroppedEventsCount[K](), nil
	case "links":
		return accessLinks[K](), nil
	case "dropped_links_count":
		return accessDroppedLinksCount[K](), nil
	case "status":
		if len(path) == 1 {
			return accessStatus[K](), nil
		}
		switch path[1].Name {
		case "code":
			return accessStatusCode[K](), nil
		case "message":
			return accessStatusMessage[K](), nil
		}
	}

	return nil, fmt.Errorf("invalid span path expression %v", path)
}

func accessSpan[K SpanContext]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(ctx K) interface{} {
			return ctx.GetSpan()
		},
		Setter: func(ctx K, val interface{}) {
			if newSpan, ok := val.(ptrace.Span); ok {
				newSpan.CopyTo(ctx.GetSpan())
			}
		},
	}
}

func accessTraceID[K SpanContext]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(ctx K) interface{} {
			return ctx.GetSpan().TraceID()
		},
		Setter: func(ctx K, val interface{}) {
			if newTraceID, ok := val.(pcommon.TraceID); ok {
				ctx.GetSpan().SetTraceID(newTraceID)
			}
		},
	}
}

func accessStringTraceID[K SpanContext]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(ctx K) interface{} {
			return ctx.GetSpan().TraceID().HexString()
		},
		Setter: func(ctx K, val interface{}) {
			if str, ok := val.(string); ok {
				if traceID, err := parseTraceID(str); err == nil {
					ctx.GetSpan().SetTraceID(traceID)
				}
			}
		},
	}
}

func accessSpanID[K SpanContext]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(ctx K) interface{} {
			return ctx.GetSpan().SpanID()
		},
		Setter: func(ctx K, val interface{}) {
			if newSpanID, ok := val.(pcommon.SpanID); ok {
				ctx.GetSpan().SetSpanID(newSpanID)
			}
		},
	}
}

func accessStringSpanID[K SpanContext]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(ctx K) interface{} {
			return ctx.GetSpan().SpanID().HexString()
		},
		Setter: func(ctx K, val interface{}) {
			if str, ok := val.(string); ok {
				if spanID, err := parseSpanID(str); err == nil {
					ctx.GetSpan().SetSpanID(spanID)
				}
			}
		},
	}
}

func accessTraceState[K SpanContext]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(ctx K) interface{} {
			return ctx.GetSpan().TraceState().AsRaw()
		},
		Setter: func(ctx K, val interface{}) {
			if str, ok := val.(string); ok {
				ctx.GetSpan().TraceState().FromRaw(str)
			}
		},
	}
}

func accessTraceStateKey[K SpanContext](mapKey *string) ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(ctx K) interface{} {
			if ts, err := trace.ParseTraceState(ctx.GetSpan().TraceState().AsRaw()); err == nil {
				return ts.Get(*mapKey)
			}
			return nil
		},
		Setter: func(ctx K, val interface{}) {
			if str, ok := val.(string); ok {
				if ts, err := trace.ParseTraceState(ctx.GetSpan().TraceState().AsRaw()); err == nil {
					if updated, err := ts.Insert(*mapKey, str); err == nil {
						ctx.GetSpan().TraceState().FromRaw(updated.String())
					}
				}
			}
		},
	}
}

func accessParentSpanID[K SpanContext]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(ctx K) interface{} {
			return ctx.GetSpan().ParentSpanID()
		},
		Setter: func(ctx K, val interface{}) {
			if newParentSpanID, ok := val.(pcommon.SpanID); ok {
				ctx.GetSpan().SetParentSpanID(newParentSpanID)
			}
		},
	}
}

func accessName[K SpanContext]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(ctx K) interface{} {
			return ctx.GetSpan().Name()
		},
		Setter: func(ctx K, val interface{}) {
			if str, ok := val.(string); ok {
				ctx.GetSpan().SetName(str)
			}
		},
	}
}

func accessKind[K SpanContext]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(ctx K) interface{} {
			return int64(ctx.GetSpan().Kind())
		},
		Setter: func(ctx K, val interface{}) {
			if i, ok := val.(int64); ok {
				ctx.GetSpan().SetKind(ptrace.SpanKind(i))
			}
		},
	}
}

func accessStartTimeUnixNano[K SpanContext]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(ctx K) interface{} {
			return ctx.GetSpan().StartTimestamp().AsTime().UnixNano()
		},
		Setter: func(ctx K, val interface{}) {
			if i, ok := val.(int64); ok {
				ctx.GetSpan().SetStartTimestamp(pcommon.NewTimestampFromTime(time.Unix(0, i)))
			}
		},
	}
}

func accessEndTimeUnixNano[K SpanContext]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(ctx K) interface{} {
			return ctx.GetSpan().EndTimestamp().AsTime().UnixNano()
		},
		Setter: func(ctx K, val interface{}) {
			if i, ok := val.(int64); ok {
				ctx.GetSpan().SetEndTimestamp(pcommon.NewTimestampFromTime(time.Unix(0, i)))
			}
		},
	}
}

func accessAttributes[K SpanContext]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(ctx K) interface{} {
			return ctx.GetSpan().Attributes()
		},
		Setter: func(ctx K, val interface{}) {
			if attrs, ok := val.(pcommon.Map); ok {
				attrs.CopyTo(ctx.GetSpan().Attributes())
			}
		},
	}
}

func accessAttributesKey[K SpanContext](mapKey *string) ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(ctx K) interface{} {
			return GetMapValue(ctx.GetSpan().Attributes(), *mapKey)
		},
		Setter: func(ctx K, val interface{}) {
			SetMapValue(ctx.GetSpan().Attributes(), *mapKey, val)
		},
	}
}

func accessSpanDroppedAttributesCount[K SpanContext]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(ctx K) interface{} {
			return int64(ctx.GetSpan().DroppedAttributesCount())
		},
		Setter: func(ctx K, val interface{}) {
			if i, ok := val.(int64); ok {
				ctx.GetSpan().SetDroppedAttributesCount(uint32(i))
			}
		},
	}
}

func accessEvents[K SpanContext]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(ctx K) interface{} {
			return ctx.GetSpan().Events()
		},
		Setter: func(ctx K, val interface{}) {
			if slc, ok := val.(ptrace.SpanEventSlice); ok {
				ctx.GetSpan().Events().RemoveIf(func(event ptrace.SpanEvent) bool {
					return true
				})
				slc.CopyTo(ctx.GetSpan().Events())
			}
		},
	}
}

func accessDroppedEventsCount[K SpanContext]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(ctx K) interface{} {
			return int64(ctx.GetSpan().DroppedEventsCount())
		},
		Setter: func(ctx K, val interface{}) {
			if i, ok := val.(int64); ok {
				ctx.GetSpan().SetDroppedEventsCount(uint32(i))
			}
		},
	}
}

func accessLinks[K SpanContext]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(ctx K) interface{} {
			return ctx.GetSpan().Links()
		},
		Setter: func(ctx K, val interface{}) {
			if slc, ok := val.(ptrace.SpanLinkSlice); ok {
				ctx.GetSpan().Links().RemoveIf(func(event ptrace.SpanLink) bool {
					return true
				})
				slc.CopyTo(ctx.GetSpan().Links())
			}
		},
	}
}

func accessDroppedLinksCount[K SpanContext]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(ctx K) interface{} {
			return int64(ctx.GetSpan().DroppedLinksCount())
		},
		Setter: func(ctx K, val interface{}) {
			if i, ok := val.(int64); ok {
				ctx.GetSpan().SetDroppedLinksCount(uint32(i))
			}
		},
	}
}

func accessStatus[K SpanContext]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(ctx K) interface{} {
			return ctx.GetSpan().Status()
		},
		Setter: func(ctx K, val interface{}) {
			if status, ok := val.(ptrace.Status); ok {
				status.CopyTo(ctx.GetSpan().Status())
			}
		},
	}
}

func accessStatusCode[K SpanContext]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(ctx K) interface{} {
			return int64(ctx.GetSpan().Status().Code())
		},
		Setter: func(ctx K, val interface{}) {
			if i, ok := val.(int64); ok {
				ctx.GetSpan().Status().SetCode(ptrace.StatusCode(i))
			}
		},
	}
}

func accessStatusMessage[K SpanContext]() ottl.StandardGetSetter[K] {
	return ottl.StandardGetSetter[K]{
		Getter: func(ctx K) interface{} {
			return ctx.GetSpan().Status().Message()
		},
		Setter: func(ctx K, val interface{}) {
			if str, ok := val.(string); ok {
				ctx.GetSpan().Status().SetMessage(str)
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
