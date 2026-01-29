// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlruntime // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlruntime"

import (
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

type PValueValue struct {
	value[pcommon.Value]
}

func NewPValueValue(val pcommon.Value) PValueValue {
	return PValueValue{value: newValue(val, false)}
}

func NewNilPValueValue() PValueValue {
	return PValueValue{value: newValue(pcommon.Value{}, true)}
}

func ToPValueValue(v Value) (PValueValue, error) {
	if val, ok := v.(PValueValue); ok {
		return val, nil
	}
	if v.IsNil() {
		return PValueValue{}, fmt.Errorf("unsupported type conversion from nil %T to \"PValueValue\"", v)
	}
	switch val := v.(type) {
	case StringValue:
		return pValueTransformFromString(val)
	case Int64Value:
		return pValueTransformFromInt64(val)
	case Float64Value:
		return pValueTransformFromFloat64(val)
	case BoolValue:
		return pValueTransformFromBool(val)
	case PMapValue:
		return pValueTransformFromPMap(val)
	case PSliceValue:
		return pValueTransformFromPSlice(val)
	default:
		return PValueValue{}, fmt.Errorf("unsupported type conversion from %T to \"pcommon.Value\"", v)
	}
}

// pValueTransformFromPMap is a transformFunc that is capable of transforming PMapValue into PValueValue
func pValueTransformFromPMap(v PMapValue) (PValueValue, error) {
	ret := pcommon.NewValueMap()
	v.Val().CopyTo(ret.Map())
	return NewPValueValue(ret), nil
}

// pValueTransformFromPSlice is a transformFunc that is capable of transforming PSliceValue into PValueValue
func pValueTransformFromPSlice(v PSliceValue) (PValueValue, error) {
	ret := pcommon.NewValueSlice()
	v.Val().CopyTo(ret.Slice())
	return NewPValueValue(ret), nil
}

// pValueTransformFromString is a transformFunc that is capable of transforming any value into PValueValue
func pValueTransformFromString(v StringValue) (PValueValue, error) {
	return NewPValueValue(pcommon.NewValueStr(v.Val())), nil
}

// pValueTransformFromInt64 is a transformFunc that is capable of transforming any Int64Value into PValueValue
func pValueTransformFromInt64(v Int64Value) (PValueValue, error) {
	return NewPValueValue(pcommon.NewValueInt(v.Val())), nil
}

// pValueTransformFromFloat64 is a transformFunc that is capable of transforming any Float64Value into PValueValue
func pValueTransformFromFloat64(v Float64Value) (PValueValue, error) {
	return NewPValueValue(pcommon.NewValueDouble(v.Val())), nil
}

// pValueTransformFromDefault is a transformFunc that is capable of transforming any BoolValue into PValueValue
func pValueTransformFromBool(v BoolValue) (PValueValue, error) {
	return NewPValueValue(pcommon.NewValueBool(v.Val())), nil
}
