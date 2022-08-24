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

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/telemetryquerylanguage/tql"
)

type LogTransformContext struct {
	Log                  plog.LogRecord
	InstrumentationScope pcommon.InstrumentationScope
	Resource             pcommon.Resource
}

func (ctx LogTransformContext) GetItem() interface{} {
	return ctx.Log
}

func (ctx LogTransformContext) GetInstrumentationScope() pcommon.InstrumentationScope {
	return ctx.InstrumentationScope
}

func (ctx LogTransformContext) GetResource() pcommon.Resource {
	return ctx.Resource
}

var symbolTable = map[tql.EnumSymbol]tql.Enum{
	"SEVERITY_NUMBER_UNSPECIFIED": tql.Enum(plog.SeverityNumberUNDEFINED),
	"SEVERITY_NUMBER_TRACE":       tql.Enum(plog.SeverityNumberTRACE),
	"SEVERITY_NUMBER_TRACE2":      tql.Enum(plog.SeverityNumberTRACE2),
	"SEVERITY_NUMBER_TRACE3":      tql.Enum(plog.SeverityNumberTRACE3),
	"SEVERITY_NUMBER_TRACE4":      tql.Enum(plog.SeverityNumberTRACE4),
	"SEVERITY_NUMBER_DEBUG":       tql.Enum(plog.SeverityNumberDEBUG),
	"SEVERITY_NUMBER_DEBUG2":      tql.Enum(plog.SeverityNumberDEBUG2),
	"SEVERITY_NUMBER_DEBUG3":      tql.Enum(plog.SeverityNumberDEBUG3),
	"SEVERITY_NUMBER_DEBUG4":      tql.Enum(plog.SeverityNumberDEBUG4),
	"SEVERITY_NUMBER_INFO":        tql.Enum(plog.SeverityNumberINFO),
	"SEVERITY_NUMBER_INFO2":       tql.Enum(plog.SeverityNumberINFO2),
	"SEVERITY_NUMBER_INFO3":       tql.Enum(plog.SeverityNumberINFO3),
	"SEVERITY_NUMBER_INFO4":       tql.Enum(plog.SeverityNumberINFO4),
	"SEVERITY_NUMBER_WARN":        tql.Enum(plog.SeverityNumberWARN),
	"SEVERITY_NUMBER_WARN2":       tql.Enum(plog.SeverityNumberWARN2),
	"SEVERITY_NUMBER_WARN3":       tql.Enum(plog.SeverityNumberWARN3),
	"SEVERITY_NUMBER_WARN4":       tql.Enum(plog.SeverityNumberWARN4),
	"SEVERITY_NUMBER_ERROR":       tql.Enum(plog.SeverityNumberERROR),
	"SEVERITY_NUMBER_ERROR2":      tql.Enum(plog.SeverityNumberERROR2),
	"SEVERITY_NUMBER_ERROR3":      tql.Enum(plog.SeverityNumberERROR3),
	"SEVERITY_NUMBER_ERROR4":      tql.Enum(plog.SeverityNumberERROR4),
	"SEVERITY_NUMBER_FATAL":       tql.Enum(plog.SeverityNumberFATAL),
	"SEVERITY_NUMBER_FATAL2":      tql.Enum(plog.SeverityNumberFATAL2),
	"SEVERITY_NUMBER_FATAL3":      tql.Enum(plog.SeverityNumberFATAL3),
	"SEVERITY_NUMBER_FATAL4":      tql.Enum(plog.SeverityNumberFATAL4),
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
	case "instrumentation_scope":
		if len(path) == 1 {
			return accessInstrumentationScope(), nil
		}
		switch path[1].Name {
		case "name":
			return accessInstrumentationScopeName(), nil
		case "version":
			return accessInstrumentationScopeVersion(), nil
		}
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
			return getValue(ctx.GetItem().(plog.LogRecord).Body())
		},
		Setter: func(ctx tql.TransformContext, val interface{}) {
			setValue(ctx.GetItem().(plog.LogRecord).Body(), val)
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
				ctx.GetItem().(plog.LogRecord).Attributes().Clear()
				attrs.CopyTo(ctx.GetItem().(plog.LogRecord).Attributes())
			}
		},
	}
}

func accessAttributesKey(mapKey *string) tql.StandardGetSetter {
	return tql.StandardGetSetter{
		Getter: func(ctx tql.TransformContext) interface{} {
			return getAttr(ctx.GetItem().(plog.LogRecord).Attributes(), *mapKey)
		},
		Setter: func(ctx tql.TransformContext, val interface{}) {
			setAttr(ctx.GetItem().(plog.LogRecord).Attributes(), *mapKey, val)
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
			return int64(ctx.GetItem().(plog.LogRecord).Flags())
		},
		Setter: func(ctx tql.TransformContext, val interface{}) {
			if i, ok := val.(int64); ok {
				ctx.GetItem().(plog.LogRecord).SetFlags(uint32(i))
			}
		},
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

func getAttr(attrs pcommon.Map, mapKey string) interface{} {
	val, ok := attrs.Get(mapKey)
	if !ok {
		return nil
	}
	return getValue(val)
}

func getValue(val pcommon.Value) interface{} {
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

func setValue(value pcommon.Value, val interface{}) {
	switch v := val.(type) {
	case string:
		value.SetStringVal(v)
	case bool:
		value.SetBoolVal(v)
	case int64:
		value.SetIntVal(v)
	case float64:
		value.SetDoubleVal(v)
	case []byte:
		value.SetBytesVal(pcommon.NewImmutableByteSlice(v))
	case []string:
		value.SliceVal().RemoveIf(func(_ pcommon.Value) bool {
			return true
		})
		for _, str := range v {
			value.SliceVal().AppendEmpty().SetStringVal(str)
		}
	case []bool:
		value.SliceVal().RemoveIf(func(_ pcommon.Value) bool {
			return true
		})
		for _, b := range v {
			value.SliceVal().AppendEmpty().SetBoolVal(b)
		}
	case []int64:
		value.SliceVal().RemoveIf(func(_ pcommon.Value) bool {
			return true
		})
		for _, i := range v {
			value.SliceVal().AppendEmpty().SetIntVal(i)
		}
	case []float64:
		value.SliceVal().RemoveIf(func(_ pcommon.Value) bool {
			return true
		})
		for _, f := range v {
			value.SliceVal().AppendEmpty().SetDoubleVal(f)
		}
	case [][]byte:
		value.SliceVal().RemoveIf(func(_ pcommon.Value) bool {
			return true
		})
		for _, b := range v {
			value.SliceVal().AppendEmpty().SetBytesVal(pcommon.NewImmutableByteSlice(b))
		}
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
