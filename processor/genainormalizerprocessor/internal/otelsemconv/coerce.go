// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otelsemconv // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/genainormalizerprocessor/internal/otelsemconv"

import (
	"strconv"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

// kind is the type a gen_ai.* attribute carries, per the semantic-conventions
// registry.
type kind uint8

const (
	// kindAny is the registry's "any". Values are passed through unchecked.
	kindAny kind = iota
	kindString
	kindInt
	kindDouble
	kindBoolean
	kindStringSlice
)

// Coerce writes src into dst conforming to the Go type the OTel GenAI
// semconv defines for targetKey. Returns true if the value was written
// (passthrough or coerced). Returns false when the key has a known type but
// src cannot be safely coerced; callers must drop the attribute.
//
// Keys typed "any", and keys the registry does not define, are copied verbatim.
func Coerce(targetKey string, src, dst pcommon.Value) bool {
	expected, known := targetTypes[targetKey]
	if !known || expected == kindAny {
		src.CopyTo(dst)
		return true
	}
	switch expected {
	case kindInt:
		return coerceInt(src, dst)
	case kindDouble:
		return coerceFloat64(src, dst)
	case kindString:
		return coerceString(src, dst)
	case kindStringSlice:
		return coerceStringSlice(src, dst)
	case kindBoolean:
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
