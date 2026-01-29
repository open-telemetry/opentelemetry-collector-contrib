// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlruntime // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlruntime"

import (
	"fmt"
	"strconv"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

type Float64Value struct {
	value[float64]
}

func NewFloat64Value(val float64) Float64Value {
	return Float64Value{value: newValue(val, false)}
}

func NewNilFloat64Value() Float64Value {
	return Float64Value{value: newValue(float64(0), true)}
}

func ToFloat64Value(v Value) (Float64Value, error) {
	if val, ok := v.(Float64Value); ok {
		return val, nil
	}
	if v.IsNil() {
		return Float64Value{}, fmt.Errorf("unsupported type conversion from nil %T to \"Float64Value\"", v)
	}
	switch val := v.(type) {
	case StringValue:
		return float64TransformFromString(val)
	case Int64Value:
		return float64TransformFromInt64(val)
	case BoolValue:
		return float64TransformFromBool(val)
	case PValueValue:
		return float64TransformFromPValue(val)
	default:
		return Float64Value{}, fmt.Errorf("unsupported type conversion from %T to \"float64\"", v)
	}
}

// float64TransformFromPValue is a transformFunc that is capable of transforming PValueValue into Float64Value
func float64TransformFromPValue(v PValueValue) (Float64Value, error) {
	val := v.Val()
	switch val.Type() {
	case pcommon.ValueTypeInt:
		return NewFloat64Value(float64(val.Int())), nil
	case pcommon.ValueTypeDouble:
		return NewFloat64Value(val.Double()), nil
	case pcommon.ValueTypeStr:
		ret, err := strconv.ParseFloat(val.Str(), 64)
		if err != nil {
			return Float64Value{}, fmt.Errorf("unable to convert string to float64: %w", err)
		}
		return NewFloat64Value(ret), nil
	case pcommon.ValueTypeBool:
		if val.Bool() {
			return NewFloat64Value(float64(1)), nil
		}
		return NewFloat64Value(float64(0)), nil
	default:
		return Float64Value{}, fmt.Errorf("unsupported type conversion from pcommon.Map[%q] to \"float64\"", val.Type())
	}
}

// float64TransformFromString is a transformFunc that is capable of transforming StringValue into Float64Value
func float64TransformFromString(v StringValue) (Float64Value, error) {
	val, err := strconv.ParseFloat(v.Val(), 64)
	if err != nil {
		return Float64Value{}, fmt.Errorf("unable to convert string to float64: %w", err)
	}
	return NewFloat64Value(val), nil
}

// float64TransformFromInt64 is a transformFunc that is capable of transforming Int64Value into Float64Value
func float64TransformFromInt64(v Int64Value) (Float64Value, error) {
	return NewFloat64Value(float64(v.Val())), nil
}

// float64TransformFromBool is a transformFunc that is capable of transforming BoolValue into Float64Value
func float64TransformFromBool(v BoolValue) (Float64Value, error) {
	if v.Val() {
		return NewFloat64Value(float64(1)), nil
	}
	return NewFloat64Value(float64(0)), nil
}
