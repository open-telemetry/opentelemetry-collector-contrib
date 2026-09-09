// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otelsemconv // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/genainormalizerprocessor/internal/otelsemconv"

import (
	"reflect"
	"strconv"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

// Coerce writes src into dst conforming to the Go type the OTel GenAI
// semconv defines for targetKey. Returns true if the value was written
// (passthrough or coerced). Returns false when the key has a known type but
// src cannot be safely coerced; callers must drop the attribute.
//
// Keys without a known typed constructor (spec type "any") fall through to
// a verbatim src.CopyTo(dst).
func Coerce(targetKey string, src, dst pcommon.Value) bool {
	expected, known := targetTypes[targetKey]
	if !known {
		src.CopyTo(dst)
		return true
	}
	switch expected.Kind() {
	case reflect.Int, reflect.Int64:
		return coerceInt(src, dst)
	case reflect.Float64:
		return coerceFloat64(src, dst)
	case reflect.String:
		return coerceString(src, dst)
	case reflect.Slice:
		if expected.Elem().Kind() == reflect.String {
			return coerceStringSlice(src, dst)
		}
	case reflect.Bool:
		return coerceBool(src, dst)
	}
	return false
}

func coerceInt(src, dst pcommon.Value) bool {
	switch src.Type() {
	case pcommon.ValueTypeInt:
		dst.SetInt(src.Int())
		return true
	case pcommon.ValueTypeDouble:
		// Lossless only for integral-valued doubles.
		f := src.Double()
		i := int64(f)
		if float64(i) == f {
			dst.SetInt(i)
			return true
		}
	case pcommon.ValueTypeStr:
		if i, err := strconv.ParseInt(src.Str(), 10, 64); err == nil {
			dst.SetInt(i)
			return true
		}
	}
	return false
}

func coerceFloat64(src, dst pcommon.Value) bool {
	switch src.Type() {
	case pcommon.ValueTypeDouble:
		dst.SetDouble(src.Double())
		return true
	case pcommon.ValueTypeInt:
		dst.SetDouble(float64(src.Int()))
		return true
	case pcommon.ValueTypeStr:
		if f, err := strconv.ParseFloat(src.Str(), 64); err == nil {
			dst.SetDouble(f)
			return true
		}
	}
	return false
}

func coerceString(src, dst pcommon.Value) bool {
	switch src.Type() {
	case pcommon.ValueTypeStr:
		dst.SetStr(src.Str())
		return true
	case pcommon.ValueTypeBool, pcommon.ValueTypeInt, pcommon.ValueTypeDouble:
		dst.SetStr(src.AsString())
		return true
	}
	// Map / Slice / Bytes: do not stringify. Caller drops the rename.
	return false
}

func coerceStringSlice(src, dst pcommon.Value) bool {
	switch src.Type() {
	case pcommon.ValueTypeStr:
		dst.SetEmptySlice().AppendEmpty().SetStr(src.Str())
		return true
	case pcommon.ValueTypeSlice:
		// Pass through only when every element is already a string.
		ss := src.Slice()
		for i := 0; i < ss.Len(); i++ {
			if ss.At(i).Type() != pcommon.ValueTypeStr {
				return false
			}
		}
		ss.CopyTo(dst.SetEmptySlice())
		return true
	}
	return false
}

func coerceBool(src, dst pcommon.Value) bool {
	switch src.Type() {
	case pcommon.ValueTypeBool:
		dst.SetBool(src.Bool())
		return true
	case pcommon.ValueTypeStr:
		if b, err := strconv.ParseBool(src.Str()); err == nil {
			dst.SetBool(b)
			return true
		}
	}
	return false
}
