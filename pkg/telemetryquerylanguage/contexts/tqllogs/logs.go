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

package tqllogs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/telemetryquerylanguage/contexts/tqllogs"

import (
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/telemetryquerylanguage/contexts/internal/tqlcommon"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/telemetryquerylanguage/tql"
)

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

func (ctx TransformContext) GetItem() interface{} {
	return ctx.logRecord
}

func (ctx TransformContext) GetInstrumentationScope() pcommon.InstrumentationScope {
	return ctx.instrumentationScope
}

func (ctx TransformContext) GetResource() pcommon.Resource {
	return ctx.resource
}

var symbolTable = map[tql.EnumSymbol]tql.Enum{
	"SEVERITY_NUMBER_UNSPECIFIED": tql.Enum(plog.SeverityNumberUndefined),
	"SEVERITY_NUMBER_TRACE":       tql.Enum(plog.SeverityNumberTrace),
	"SEVERITY_NUMBER_TRACE2":      tql.Enum(plog.SeverityNumberTrace2),
	"SEVERITY_NUMBER_TRACE3":      tql.Enum(plog.SeverityNumberTrace3),
	"SEVERITY_NUMBER_TRACE4":      tql.Enum(plog.SeverityNumberTrace4),
	"SEVERITY_NUMBER_DEBUG":       tql.Enum(plog.SeverityNumberDebug),
	"SEVERITY_NUMBER_DEBUG2":      tql.Enum(plog.SeverityNumberDebug2),
	"SEVERITY_NUMBER_DEBUG3":      tql.Enum(plog.SeverityNumberDebug3),
	"SEVERITY_NUMBER_DEBUG4":      tql.Enum(plog.SeverityNumberDebug4),
	"SEVERITY_NUMBER_INFO":        tql.Enum(plog.SeverityNumberInfo),
	"SEVERITY_NUMBER_INFO2":       tql.Enum(plog.SeverityNumberInfo2),
	"SEVERITY_NUMBER_INFO3":       tql.Enum(plog.SeverityNumberInfo3),
	"SEVERITY_NUMBER_INFO4":       tql.Enum(plog.SeverityNumberInfo4),
	"SEVERITY_NUMBER_WARN":        tql.Enum(plog.SeverityNumberWarn),
	"SEVERITY_NUMBER_WARN2":       tql.Enum(plog.SeverityNumberWarn2),
	"SEVERITY_NUMBER_WARN3":       tql.Enum(plog.SeverityNumberWarn3),
	"SEVERITY_NUMBER_WARN4":       tql.Enum(plog.SeverityNumberWarn4),
	"SEVERITY_NUMBER_ERROR":       tql.Enum(plog.SeverityNumberError),
	"SEVERITY_NUMBER_ERROR2":      tql.Enum(plog.SeverityNumberError2),
	"SEVERITY_NUMBER_ERROR3":      tql.Enum(plog.SeverityNumberError3),
	"SEVERITY_NUMBER_ERROR4":      tql.Enum(plog.SeverityNumberError4),
	"SEVERITY_NUMBER_FATAL":       tql.Enum(plog.SeverityNumberFatal),
	"SEVERITY_NUMBER_FATAL2":      tql.Enum(plog.SeverityNumberFatal2),
	"SEVERITY_NUMBER_FATAL3":      tql.Enum(plog.SeverityNumberFatal3),
	"SEVERITY_NUMBER_FATAL4":      tql.Enum(plog.SeverityNumberFatal4),
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
		return tqlcommon.ResourcePathGetSetter(path[1:])
	case "instrumentation_scope":
		return tqlcommon.ScopePathGetSetter(path[1:])
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

func accessTimeUnixNano() tql.StandardGetSetter {
	return tql.StandardGetSetter{
		Getter: func(ctx tql.TransformContext) interface{} {
			return ctx.GetItem().(plog.LogRecord).Timestamp().AsTime().UnixNano()
		},
		Setter: func(ctx tql.TransformContext, val interface{}) {
			if i, ok := val.(int64); ok {
				ctx.GetItem().(plog.LogRecord).SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(0, i)))
			}
		},
	}
}

func accessObservedTimeUnixNano() tql.StandardGetSetter {
	return tql.StandardGetSetter{
		Getter: func(ctx tql.TransformContext) interface{} {
			return ctx.GetItem().(plog.LogRecord).ObservedTimestamp().AsTime().UnixNano()
		},
		Setter: func(ctx tql.TransformContext, val interface{}) {
			if i, ok := val.(int64); ok {
				ctx.GetItem().(plog.LogRecord).SetObservedTimestamp(pcommon.NewTimestampFromTime(time.Unix(0, i)))
			}
		},
	}
}

func accessSeverityNumber() tql.StandardGetSetter {
	return tql.StandardGetSetter{
		Getter: func(ctx tql.TransformContext) interface{} {
			return int64(ctx.GetItem().(plog.LogRecord).SeverityNumber())
		},
		Setter: func(ctx tql.TransformContext, val interface{}) {
			if i, ok := val.(int64); ok {
				ctx.GetItem().(plog.LogRecord).SetSeverityNumber(plog.SeverityNumber(i))
			}
		},
	}
}

func accessSeverityText() tql.StandardGetSetter {
	return tql.StandardGetSetter{
		Getter: func(ctx tql.TransformContext) interface{} {
			return ctx.GetItem().(plog.LogRecord).SeverityText()
		},
		Setter: func(ctx tql.TransformContext, val interface{}) {
			if s, ok := val.(string); ok {
				ctx.GetItem().(plog.LogRecord).SetSeverityText(s)
			}
		},
	}
}

func accessBody() tql.StandardGetSetter {
	return tql.StandardGetSetter{
		Getter: func(ctx tql.TransformContext) interface{} {
			return tqlcommon.GetValue(ctx.GetItem().(plog.LogRecord).Body())
		},
		Setter: func(ctx tql.TransformContext, val interface{}) {
			tqlcommon.SetValue(ctx.GetItem().(plog.LogRecord).Body(), val)
		},
	}
}

func accessAttributes() tql.StandardGetSetter {
	return tql.StandardGetSetter{
		Getter: func(ctx tql.TransformContext) interface{} {
			return ctx.GetItem().(plog.LogRecord).Attributes()
		},
		Setter: func(ctx tql.TransformContext, val interface{}) {
			if attrs, ok := val.(pcommon.Map); ok {
				attrs.CopyTo(ctx.GetItem().(plog.LogRecord).Attributes())
			}
		},
	}
}

func accessAttributesKey(mapKey *string) tql.StandardGetSetter {
	return tql.StandardGetSetter{
		Getter: func(ctx tql.TransformContext) interface{} {
			return tqlcommon.GetMapValue(ctx.GetItem().(plog.LogRecord).Attributes(), *mapKey)
		},
		Setter: func(ctx tql.TransformContext, val interface{}) {
			tqlcommon.SetMapValue(ctx.GetItem().(plog.LogRecord).Attributes(), *mapKey, val)
		},
	}
}

func accessDroppedAttributesCount() tql.StandardGetSetter {
	return tql.StandardGetSetter{
		Getter: func(ctx tql.TransformContext) interface{} {
			return int64(ctx.GetItem().(plog.LogRecord).DroppedAttributesCount())
		},
		Setter: func(ctx tql.TransformContext, val interface{}) {
			if i, ok := val.(int64); ok {
				ctx.GetItem().(plog.LogRecord).SetDroppedAttributesCount(uint32(i))
			}
		},
	}
}

func accessFlags() tql.StandardGetSetter {
	return tql.StandardGetSetter{
		Getter: func(ctx tql.TransformContext) interface{} {
			return int64(ctx.GetItem().(plog.LogRecord).FlagsStruct().AsRaw())
		},
		Setter: func(ctx tql.TransformContext, val interface{}) {
			if i, ok := val.(int64); ok {
				setFlagsValue(ctx.GetItem().(plog.LogRecord).FlagsStruct(), i)
			}
		},
	}
}

func setFlagsValue(flags plog.LogRecordFlags, value int64) {
	if value&1 != 0 {
		flags.SetIsSampled(true)
	} else {
		flags.SetIsSampled(false)
	}
}

func accessTraceID() tql.StandardGetSetter {
	return tql.StandardGetSetter{
		Getter: func(ctx tql.TransformContext) interface{} {
			return ctx.GetItem().(plog.LogRecord).TraceID()
		},
		Setter: func(ctx tql.TransformContext, val interface{}) {
			if newTraceID, ok := val.(pcommon.TraceID); ok {
				ctx.GetItem().(plog.LogRecord).SetTraceID(newTraceID)
			}
		},
	}
}

func accessStringTraceID() tql.StandardGetSetter {
	return tql.StandardGetSetter{
		Getter: func(ctx tql.TransformContext) interface{} {
			return ctx.GetItem().(plog.LogRecord).TraceID().HexString()
		},
		Setter: func(ctx tql.TransformContext, val interface{}) {
			if str, ok := val.(string); ok {
				if traceID, err := parseTraceID(str); err == nil {
					ctx.GetItem().(plog.LogRecord).SetTraceID(traceID)
				}
			}
		},
	}
}

func accessSpanID() tql.StandardGetSetter {
	return tql.StandardGetSetter{
		Getter: func(ctx tql.TransformContext) interface{} {
			return ctx.GetItem().(plog.LogRecord).SpanID()
		},
		Setter: func(ctx tql.TransformContext, val interface{}) {
			if newSpanID, ok := val.(pcommon.SpanID); ok {
				ctx.GetItem().(plog.LogRecord).SetSpanID(newSpanID)
			}
		},
	}
}

func accessStringSpanID() tql.StandardGetSetter {
	return tql.StandardGetSetter{
		Getter: func(ctx tql.TransformContext) interface{} {
			return ctx.GetItem().(plog.LogRecord).SpanID().HexString()
		},
		Setter: func(ctx tql.TransformContext, val interface{}) {
			if str, ok := val.(string); ok {
				if spanID, err := parseSpanID(str); err == nil {
					ctx.GetItem().(plog.LogRecord).SetSpanID(spanID)
				}
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
