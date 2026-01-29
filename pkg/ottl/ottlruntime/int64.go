// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlruntime // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlruntime"

import (
	"fmt"
	"strconv"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

type Int64Value struct {
	value[int64]
}

func NewInt64Value(val int64) Int64Value {
	return Int64Value{value: newValue(val, false)}
}

func NewNilInt64Value() Int64Value {
	return Int64Value{value: newValue(int64(0), true)}
}

func ToInt64Value(v Value) (Int64Value, error) {
	if val, ok := v.(Int64Value); ok {
		return val, nil
	}
	if v.IsNil() {
		return Int64Value{}, fmt.Errorf("unsupported type conversion from nil %T to \"Int64Value\"", v)
	}
	switch val := v.(type) {
	case StringValue:
		return int64TransformFromString(val)
	case Float64Value:
		return int64TransformFromFloat64(val)
	case BoolValue:
		return int64TransformFromBool(val)
	case PValueValue:
		return int64TransformFromPValue(val)
	default:
		return Int64Value{}, fmt.Errorf("unsupported type conversion from %T to \"int64\"", v)
	}
}

// int64TransformFromPValue is a transformFunc that is capable of transforming pcommon.Value into Int64Value
func int64TransformFromPValue(v PValueValue) (Int64Value, error) {
	val := v.Val()
	switch val.Type() {
	case pcommon.ValueTypeInt:
		return NewInt64Value(val.Int()), nil
	case pcommon.ValueTypeDouble:
		return NewInt64Value(int64(val.Double())), nil
	case pcommon.ValueTypeStr:
		ret, err := strconv.ParseInt(val.Str(), 10, 64)
		if err != nil {
			return Int64Value{}, fmt.Errorf("unable to convert string to int64: %w", err)
		}
		return NewInt64Value(ret), nil
	case pcommon.ValueTypeBool:
		if val.Bool() {
			return NewInt64Value(int64(1)), nil
		}
		return NewInt64Value(int64(0)), nil
	default:
		return Int64Value{}, fmt.Errorf("unsupported type conversion from pcommon.Map[%q] to \"int64\"", val.Type())
	}
}

// int64TransformFromString is a transformFunc that is capable of transforming string into Int64Value
func int64TransformFromString(v StringValue) (Int64Value, error) {
	val, err := strconv.ParseInt(v.Val(), 10, 64)
	if err != nil {
		return Int64Value{}, fmt.Errorf("unable to convert string to int64: %w", err)
	}
	return NewInt64Value(val), nil
}

// int64TransformFromFloat64 is a transformFunc that is capable of transforming float64 into Int64Value
func int64TransformFromFloat64(v Float64Value) (Int64Value, error) {
	return NewInt64Value(int64(v.Val())), nil
}

// int64TransformFromBool is a transformFunc that is capable of transforming bool into Int64Value
func int64TransformFromBool(v BoolValue) (Int64Value, error) {
	if v.Val() {
		return NewInt64Value(int64(1)), nil
	}
	return NewInt64Value(int64(0)), nil
}
