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

package ottllogs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllogs"

import (
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal/ottlcommon"
)

var _ ottlcommon.ResourceContext = TransformContext{}
var _ ottlcommon.InstrumentationScopeContext = TransformContext{}

type TransformContext struct {
	logRecord            plog.LogRecord
	instrumentationScope pcommon.InstrumentationScope
	resource             pcommon.Resource
}

func NewTransformContext(logRecord plog.LogRecord, instrumentationScope pcommon.InstrumentationScope, resource pcommon.Resource) TransformContext {
	return TransformContext{
		logRecord:            logRecord,
		instrumentationScope: instrumentationScope,
		resource:             resource,
	}
}

func (ctx TransformContext) GetLogRecord() plog.LogRecord {
	return ctx.logRecord
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

var symbolTable = map[ottl.EnumSymbol]ottl.Enum{
	"SEVERITY_NUMBER_UNSPECIFIED": ottl.Enum(plog.SeverityNumberUnspecified),
	"SEVERITY_NUMBER_TRACE":       ottl.Enum(plog.SeverityNumberTrace),
	"SEVERITY_NUMBER_TRACE2":      ottl.Enum(plog.SeverityNumberTrace2),
	"SEVERITY_NUMBER_TRACE3":      ottl.Enum(plog.SeverityNumberTrace3),
	"SEVERITY_NUMBER_TRACE4":      ottl.Enum(plog.SeverityNumberTrace4),
	"SEVERITY_NUMBER_DEBUG":       ottl.Enum(plog.SeverityNumberDebug),
	"SEVERITY_NUMBER_DEBUG2":      ottl.Enum(plog.SeverityNumberDebug2),
	"SEVERITY_NUMBER_DEBUG3":      ottl.Enum(plog.SeverityNumberDebug3),
	"SEVERITY_NUMBER_DEBUG4":      ottl.Enum(plog.SeverityNumberDebug4),
	"SEVERITY_NUMBER_INFO":        ottl.Enum(plog.SeverityNumberInfo),
	"SEVERITY_NUMBER_INFO2":       ottl.Enum(plog.SeverityNumberInfo2),
	"SEVERITY_NUMBER_INFO3":       ottl.Enum(plog.SeverityNumberInfo3),
	"SEVERITY_NUMBER_INFO4":       ottl.Enum(plog.SeverityNumberInfo4),
	"SEVERITY_NUMBER_WARN":        ottl.Enum(plog.SeverityNumberWarn),
	"SEVERITY_NUMBER_WARN2":       ottl.Enum(plog.SeverityNumberWarn2),
	"SEVERITY_NUMBER_WARN3":       ottl.Enum(plog.SeverityNumberWarn3),
	"SEVERITY_NUMBER_WARN4":       ottl.Enum(plog.SeverityNumberWarn4),
	"SEVERITY_NUMBER_ERROR":       ottl.Enum(plog.SeverityNumberError),
	"SEVERITY_NUMBER_ERROR2":      ottl.Enum(plog.SeverityNumberError2),
	"SEVERITY_NUMBER_ERROR3":      ottl.Enum(plog.SeverityNumberError3),
	"SEVERITY_NUMBER_ERROR4":      ottl.Enum(plog.SeverityNumberError4),
	"SEVERITY_NUMBER_FATAL":       ottl.Enum(plog.SeverityNumberFatal),
	"SEVERITY_NUMBER_FATAL2":      ottl.Enum(plog.SeverityNumberFatal2),
	"SEVERITY_NUMBER_FATAL3":      ottl.Enum(plog.SeverityNumberFatal3),
	"SEVERITY_NUMBER_FATAL4":      ottl.Enum(plog.SeverityNumberFatal4),
}

func parseEnum(val *ottl.EnumSymbol) (*ottl.Enum, error) {
	if val != nil {
		if enum, ok := symbolTable[*val]; ok {
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
	}

	return nil, fmt.Errorf("invalid path expression %v", path)
}

func accessTimeUnixNano() ottl.StandardGetSetter[TransformContext] {
	return ottl.StandardGetSetter[TransformContext]{
		Getter: func(ctx TransformContext) (interface{}, error) {
			return ctx.GetLogRecord().Timestamp().AsTime().UnixNano(), nil
		},
		Setter: func(ctx TransformContext, val interface{}) error {
			if i, ok := val.(int64); ok {
				ctx.GetLogRecord().SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(0, i)))
			}
			return nil
		},
	}
}

func accessObservedTimeUnixNano() ottl.StandardGetSetter[TransformContext] {
	return ottl.StandardGetSetter[TransformContext]{
		Getter: func(ctx TransformContext) (interface{}, error) {
			return ctx.GetLogRecord().ObservedTimestamp().AsTime().UnixNano(), nil
		},
		Setter: func(ctx TransformContext, val interface{}) error {
			if i, ok := val.(int64); ok {
				ctx.GetLogRecord().SetObservedTimestamp(pcommon.NewTimestampFromTime(time.Unix(0, i)))
			}
			return nil
		},
	}
}

func accessSeverityNumber() ottl.StandardGetSetter[TransformContext] {
	return ottl.StandardGetSetter[TransformContext]{
		Getter: func(ctx TransformContext) (interface{}, error) {
			return int64(ctx.GetLogRecord().SeverityNumber()), nil
		},
		Setter: func(ctx TransformContext, val interface{}) error {
			if i, ok := val.(int64); ok {
				ctx.GetLogRecord().SetSeverityNumber(plog.SeverityNumber(i))
			}
			return nil
		},
	}
}

func accessSeverityText() ottl.StandardGetSetter[TransformContext] {
	return ottl.StandardGetSetter[TransformContext]{
		Getter: func(ctx TransformContext) (interface{}, error) {
			return ctx.GetLogRecord().SeverityText(), nil
		},
		Setter: func(ctx TransformContext, val interface{}) error {
			if s, ok := val.(string); ok {
				ctx.GetLogRecord().SetSeverityText(s)
			}
			return nil
		},
	}
}

func accessBody() ottl.StandardGetSetter[TransformContext] {
	return ottl.StandardGetSetter[TransformContext]{
		Getter: func(ctx TransformContext) (interface{}, error) {
			return ottlcommon.GetValue(ctx.GetLogRecord().Body()), nil
		},
		Setter: func(ctx TransformContext, val interface{}) error {
			ottlcommon.SetValue(ctx.GetLogRecord().Body(), val)
			return nil
		},
	}
}

func accessAttributes() ottl.StandardGetSetter[TransformContext] {
	return ottl.StandardGetSetter[TransformContext]{
		Getter: func(ctx TransformContext) (interface{}, error) {
			return ctx.GetLogRecord().Attributes(), nil
		},
		Setter: func(ctx TransformContext, val interface{}) error {
			if attrs, ok := val.(pcommon.Map); ok {
				attrs.CopyTo(ctx.GetLogRecord().Attributes())
			}
			return nil
		},
	}
}

func accessAttributesKey(mapKey *string) ottl.StandardGetSetter[TransformContext] {
	return ottl.StandardGetSetter[TransformContext]{
		Getter: func(ctx TransformContext) (interface{}, error) {
			return ottlcommon.GetMapValue(ctx.GetLogRecord().Attributes(), *mapKey), nil
		},
		Setter: func(ctx TransformContext, val interface{}) error {
			ottlcommon.SetMapValue(ctx.GetLogRecord().Attributes(), *mapKey, val)
			return nil
		},
	}
}

func accessDroppedAttributesCount() ottl.StandardGetSetter[TransformContext] {
	return ottl.StandardGetSetter[TransformContext]{
		Getter: func(ctx TransformContext) (interface{}, error) {
			return int64(ctx.GetLogRecord().DroppedAttributesCount()), nil
		},
		Setter: func(ctx TransformContext, val interface{}) error {
			if i, ok := val.(int64); ok {
				ctx.GetLogRecord().SetDroppedAttributesCount(uint32(i))
			}
			return nil
		},
	}
}

func accessFlags() ottl.StandardGetSetter[TransformContext] {
	return ottl.StandardGetSetter[TransformContext]{
		Getter: func(ctx TransformContext) (interface{}, error) {
			return int64(ctx.GetLogRecord().Flags()), nil
		},
		Setter: func(ctx TransformContext, val interface{}) error {
			if i, ok := val.(int64); ok {
				ctx.GetLogRecord().SetFlags(plog.LogRecordFlags(i))
			}
			return nil
		},
	}
}

func accessTraceID() ottl.StandardGetSetter[TransformContext] {
	return ottl.StandardGetSetter[TransformContext]{
		Getter: func(ctx TransformContext) (interface{}, error) {
			return ctx.GetLogRecord().TraceID(), nil
		},
		Setter: func(ctx TransformContext, val interface{}) error {
			if newTraceID, ok := val.(pcommon.TraceID); ok {
				ctx.GetLogRecord().SetTraceID(newTraceID)
			}
			return nil
		},
	}
}

func accessStringTraceID() ottl.StandardGetSetter[TransformContext] {
	return ottl.StandardGetSetter[TransformContext]{
		Getter: func(ctx TransformContext) (interface{}, error) {
			return ctx.GetLogRecord().TraceID().HexString(), nil
		},
		Setter: func(ctx TransformContext, val interface{}) error {
			if str, ok := val.(string); ok {
				if traceID, err := parseTraceID(str); err == nil {
					ctx.GetLogRecord().SetTraceID(traceID)
				}
			}
			return nil
		},
	}
}

func accessSpanID() ottl.StandardGetSetter[TransformContext] {
	return ottl.StandardGetSetter[TransformContext]{
		Getter: func(ctx TransformContext) (interface{}, error) {
			return ctx.GetLogRecord().SpanID(), nil
		},
		Setter: func(ctx TransformContext, val interface{}) error {
			if newSpanID, ok := val.(pcommon.SpanID); ok {
				ctx.GetLogRecord().SetSpanID(newSpanID)
			}
			return nil
		},
	}
}

func accessStringSpanID() ottl.StandardGetSetter[TransformContext] {
	return ottl.StandardGetSetter[TransformContext]{
		Getter: func(ctx TransformContext) (interface{}, error) {
			return ctx.GetLogRecord().SpanID().HexString(), nil
		},
		Setter: func(ctx TransformContext, val interface{}) error {
			if str, ok := val.(string); ok {
				if spanID, err := parseSpanID(str); err == nil {
					ctx.GetLogRecord().SetSpanID(spanID)
				}
			}
			return nil
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
