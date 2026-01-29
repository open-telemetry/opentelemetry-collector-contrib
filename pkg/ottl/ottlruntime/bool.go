// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlruntime // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlruntime"

import (
	"fmt"
	"strconv"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

type BoolValue struct {
	value[bool]
}

func NewBoolValue(val bool) BoolValue {
	return BoolValue{value: newValue(val, false)}
}

func NewNilBoolValue() BoolValue {
	return BoolValue{value: newValue(false, true)}
}

func ToBoolValue(v Value) (BoolValue, error) {
	if val, ok := v.(BoolValue); ok {
		return val, nil
	}
	if v.IsNil() {
		return BoolValue{}, fmt.Errorf("unsupported type conversion from nil %T to \"BoolValue\"", v)
	}
	switch val := v.(type) {
	case StringValue:
		return boolTransformFromString(val)
	case Int64Value:
		return boolTransformFromInt64(val)
	case Float64Value:
		return boolTransformFromFloat64(val)
	case PValueValue:
		return boolTransformFromPValue(val)
	default:
		return BoolValue{}, fmt.Errorf("unsupported type conversion from %T to \"bool\"", v)
	}
}

// boolTransformFromPValue is a transformFunc that is capable of transforming PValueValue into BoolValue
func boolTransformFromPValue(v PValueValue) (BoolValue, error) {
	val := v.Val()
	switch val.Type() {
	case pcommon.ValueTypeInt:
		return NewBoolValue(val.Int() != 0), nil
	case pcommon.ValueTypeDouble:
		return NewBoolValue(val.Double() != 0), nil
	case pcommon.ValueTypeStr:
		ret, err := strconv.ParseBool(val.Str())
		if err != nil {
			return BoolValue{}, fmt.Errorf("unable to convert string to bool: %w", err)
		}
		return NewBoolValue(ret), nil
	case pcommon.ValueTypeBool:
		return NewBoolValue(val.Bool()), nil
	default:
		return BoolValue{}, fmt.Errorf("unsupported type conversion from pcommon.Value[%q] to \"bool\"", val.Type())
	}
}

// boolTransformFromString is a transformFunc that is capable of transforming StringValue into BoolValue
func boolTransformFromString(v StringValue) (BoolValue, error) {
	val, err := strconv.ParseBool(v.Val())
	if err != nil {
		return BoolValue{}, fmt.Errorf("unable to convert string to bool: %w", err)
	}
	return NewBoolValue(val), nil
}

// boolTransformFromInt64 is a transformFunc that is capable of transforming Int64Value into BoolValue
func boolTransformFromInt64(v Int64Value) (BoolValue, error) {
	return NewBoolValue(v.Val() != 0), nil
}

// boolTransformFromFloat64 is a transformFunc that is capable of transforming Float64Value into BoolValue
func boolTransformFromFloat64(v Float64Value) (BoolValue, error) {
	return NewBoolValue(v.Val() != 0), nil
}
