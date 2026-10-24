// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlcommon // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/internal/ottlcommon"

import (
	"slices"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

func GetValue(val pcommon.Value) any {
	//exhaustive:enforce
	switch val.Type() {
	case pcommon.ValueTypeStr:
		return val.Str()
	case pcommon.ValueTypeBool:
		return val.Bool()
	case pcommon.ValueTypeInt:
		return val.Int()
	case pcommon.ValueTypeDouble:
		return val.Double()
	case pcommon.ValueTypeMap:
		return val.Map()
	case pcommon.ValueTypeSlice:
		return val.Slice()
	case pcommon.ValueTypeBytes:
		return val.Bytes().AsRaw()
	case pcommon.ValueTypeEmpty:
		return nil
	}
	return nil
}

// NormalizeValue normalizes known val types for OTTL evaluation and comparison.
// It returns the original value or a normalized version of it, which might be
// a different type. See OTTL comparison rules for more details:
// https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/pkg/ottl/LANGUAGE.md#comparison-rules
func NormalizeValue(val any) any {
	switch typedVal := val.(type) {
	// match already-normalized types early
	case string, int64, float64, bool, []byte, []any, pcommon.Slice, map[string]any, pcommon.Map:
		return typedVal
	case pcommon.Value:
		// Should be removed after https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49170
		return GetValue(typedVal)
	case int:
		return int64(typedVal)
	case uint:
		return int64(typedVal)
	case float32:
		return float64(typedVal)
	case int8:
		return int64(typedVal)
	case int16:
		return int64(typedVal)
	case int32:
		return int64(typedVal)
	case uint8:
		return int64(typedVal)
	case uint16:
		return int64(typedVal)
	case uint32:
		return int64(typedVal)
	case uint64:
		return int64(typedVal)
	case []string:
		return convertAnySlice(typedVal)
	case []bool:
		return convertAnySlice(typedVal)
	case []float64:
		return convertAnySlice(typedVal)
	case []int64:
		return convertAnySlice(typedVal)
	default:
		return typedVal
	}
}

func convertAnySlice[Slice ~[]E, E any](s Slice) []any {
	res := make([]any, len(s))
	for i, v := range s {
		res[i] = v
	}
	return res
}

// CopyValueTo copies the value to the dest pcommon.Value overriding the destination.
// Unlike pcommon.Value.FromRaw, it also accepts pdata types (pcommon.Map, pcommon.Slice and
// pcommon.Value), including as elements of []any and map[string]any at any nesting level. pdata
// values are copied, so dest never aliases the source.
// When pdata types are present, the first conversion error is returned, rather than aggregating
// all errors like pcommon.Value.FromRaw does.
func CopyValueTo(value any, dest pcommon.Value) error {
	switch typedVal := value.(type) {
	case pcommon.Map:
		typedVal.CopyTo(dest.SetEmptyMap())
	case pcommon.Slice:
		typedVal.CopyTo(dest.SetEmptySlice())
	case pcommon.Value:
		typedVal.CopyTo(dest)
	case []any:
		if cannotUseFromRaw(typedVal) {
			return CopyAnySliceTo(typedVal, dest.SetEmptySlice())
		}
		return dest.FromRaw(typedVal)
	case map[string]any:
		if cannotUseFromRaw(typedVal) {
			return copyAnyMapTo(typedVal, dest.SetEmptyMap())
		}
		return dest.FromRaw(typedVal)
	case []string:
		return copyValuesTo(typedVal, dest.SetEmptySlice())
	case []bool:
		return copyValuesTo(typedVal, dest.SetEmptySlice())
	case []float64:
		return copyValuesTo(typedVal, dest.SetEmptySlice())
	case []int64:
		return copyValuesTo(typedVal, dest.SetEmptySlice())
	default:
		return dest.FromRaw(value)
	}
	return nil
}

// cannotUseFromRaw reports whether value is, or contains at any nesting level of []any and
// map[string]any, a type that pcommon.Value.FromRaw does not accept: a pcommon.Map,
// pcommon.Slice, pcommon.Value, []string, []bool, []float64 or []int64.
func cannotUseFromRaw(value any) bool {
	switch typedVal := value.(type) {
	case pcommon.Map, pcommon.Slice, pcommon.Value, []string, []bool, []float64, []int64:
		return true
	case []any:
		return slices.ContainsFunc(typedVal, cannotUseFromRaw)
	case map[string]any:
		for _, el := range typedVal {
			if cannotUseFromRaw(el) {
				return true
			}
		}
	}
	return false
}

func copyValuesTo[Slice ~[]E, E any](s Slice, dest pcommon.Slice) error {
	dest.EnsureCapacity(len(s))
	for _, v := range s {
		err := dest.AppendEmpty().FromRaw(v)
		if err != nil {
			return err
		}
	}
	return nil
}

// CopyAnySliceTo appends the elements of src to dest, which is expected to be empty.
// Elements are converted with CopyValueTo, so pdata types are accepted at any nesting level.
func CopyAnySliceTo(src []any, dest pcommon.Slice) error {
	dest.EnsureCapacity(len(src))
	for _, el := range src {
		if err := CopyValueTo(el, dest.AppendEmpty()); err != nil {
			return err
		}
	}
	return nil
}

func copyAnyMapTo(src map[string]any, dest pcommon.Map) error {
	dest.EnsureCapacity(len(src))
	for k, el := range src {
		if err := CopyValueTo(el, dest.PutEmpty(k)); err != nil {
			return err
		}
	}
	return nil
}
