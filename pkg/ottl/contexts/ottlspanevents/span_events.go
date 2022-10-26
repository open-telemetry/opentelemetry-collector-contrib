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

package ottlspanevents // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspanevents"

import (
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ottlcommon"
)

var _ ottlcommon.ResourceContext = TransformContext{}
var _ ottlcommon.InstrumentationScopeContext = TransformContext{}
var _ ottlcommon.SpanContext = TransformContext{}

type TransformContext struct {
	spanEvent            ptrace.SpanEvent
	span                 ptrace.Span
	instrumentationScope pcommon.InstrumentationScope
	resource             pcommon.Resource
}

func NewTransformContext(spanEvent ptrace.SpanEvent, span ptrace.Span, instrumentationScope pcommon.InstrumentationScope, resource pcommon.Resource) TransformContext {
	return TransformContext{
		spanEvent:            spanEvent,
		span:                 span,
		instrumentationScope: instrumentationScope,
		resource:             resource,
	}
}

func (ctx TransformContext) GetSpanEvent() ptrace.SpanEvent {
	return ctx.spanEvent
}

func (ctx TransformContext) GetSpan() ptrace.Span {
	return ctx.span
}

func (ctx TransformContext) GetInstrumentationScope() pcommon.InstrumentationScope {
	return ctx.instrumentationScope
}

func (ctx TransformContext) GetResource() pcommon.Resource {
	return ctx.resource
}

func NewParser(functions map[string]interface{}, telemetrySettings component.TelemetrySettings) ottl.Parser[TransformContext] {
	return ottl.NewParser[TransformContext](functions, parsePath, parseEnum, telemetrySettings)
}

func parseEnum(val *ottl.EnumSymbol) (*ottl.Enum, error) {
	if val != nil {
		if enum, ok := ottlcommon.SpanSymbolTable[*val]; ok {
			return &enum, nil
		}
		return nil, fmt.Errorf("enum symbol, %s, not found", *val)
	}
	return nil, fmt.Errorf("enum symbol not provided")
}

func parsePath(val *ottl.Path) (ottl.GetSetter[TransformContext], error) {
	if val != nil && len(val.Fields) > 0 {
		return newPathGetSetter(val.Fields)
	}
	return nil, fmt.Errorf("bad path %v", val)
}

func newPathGetSetter(path []ottl.Field) (ottl.GetSetter[TransformContext], error) {
	switch path[0].Name {
	case "resource":
		return ottlcommon.ResourcePathGetSetter[TransformContext](path[1:])
	case "instrumentation_scope":
		return ottlcommon.ScopePathGetSetter[TransformContext](path[1:])
	case "span":
		return ottlcommon.SpanPathGetSetter[TransformContext](path[1:])
	case "time_unix_nano":
		return accessSpanEventTimeUnixNano(), nil
	case "name":
		return accessSpanEventName(), nil
	case "attributes":
		mapKey := path[0].MapKey
		if mapKey == nil {
			return accessSpanEventAttributes(), nil
		}
		return accessSpanEventAttributesKey(mapKey), nil
	case "dropped_attributes_count":
		return accessSpanEventDroppedAttributeCount(), nil
	}

	return nil, fmt.Errorf("invalid scope path expression %v", path)
}

func accessSpanEventTimeUnixNano() ottl.StandardGetSetter[TransformContext] {
	return ottl.StandardGetSetter[TransformContext]{
		Getter: func(ctx TransformContext) interface{} {
			return ctx.GetSpanEvent().Timestamp().AsTime().UnixNano()
		},
		Setter: func(ctx TransformContext, val interface{}) {
			if newTimestamp, ok := val.(int64); ok {
				ctx.GetSpanEvent().SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(0, newTimestamp)))
			}
		},
	}
}

func accessSpanEventName() ottl.StandardGetSetter[TransformContext] {
	return ottl.StandardGetSetter[TransformContext]{
		Getter: func(ctx TransformContext) interface{} {
			return ctx.GetSpanEvent().Name()
		},
		Setter: func(ctx TransformContext, val interface{}) {
			if newName, ok := val.(string); ok {
				ctx.GetSpanEvent().SetName(newName)
			}
		},
	}
}

func accessSpanEventAttributes() ottl.StandardGetSetter[TransformContext] {
	return ottl.StandardGetSetter[TransformContext]{
		Getter: func(ctx TransformContext) interface{} {
			return ctx.GetSpanEvent().Attributes()
		},
		Setter: func(ctx TransformContext, val interface{}) {
			if attrs, ok := val.(pcommon.Map); ok {
				attrs.CopyTo(ctx.GetSpanEvent().Attributes())
			}
		},
	}
}

func accessSpanEventAttributesKey(mapKey *string) ottl.StandardGetSetter[TransformContext] {
	return ottl.StandardGetSetter[TransformContext]{
		Getter: func(ctx TransformContext) interface{} {
			return ottlcommon.GetMapValue(ctx.GetSpanEvent().Attributes(), *mapKey)
		},
		Setter: func(ctx TransformContext, val interface{}) {
			ottlcommon.SetMapValue(ctx.GetSpanEvent().Attributes(), *mapKey, val)
		},
	}
}

func accessSpanEventDroppedAttributeCount() ottl.StandardGetSetter[TransformContext] {
	return ottl.StandardGetSetter[TransformContext]{
		Getter: func(ctx TransformContext) interface{} {
			return int64(ctx.GetSpanEvent().DroppedAttributesCount())
		},
		Setter: func(ctx TransformContext, val interface{}) {
			if newCount, ok := val.(int64); ok {
				ctx.GetSpanEvent().SetDroppedAttributesCount(uint32(newCount))
			}
		},
	}
}
