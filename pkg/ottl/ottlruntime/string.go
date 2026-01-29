// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlruntime // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlruntime"

import (
	"encoding/json"
	"fmt"
	"strconv"
)

type StringValue struct {
	value[string]
}

func NewStringValue(val string) StringValue {
	return StringValue{value: newValue(val, false)}
}

func NewNilStringValue() StringValue {
	return StringValue{value: newValue("", true)}
}

func ToStringValue(v Value) (StringValue, error) {
	if val, ok := v.(StringValue); ok {
		return val, nil
	}
	if v.IsNil() {
		return StringValue{}, fmt.Errorf("unsupported type conversion from nil %T to \"StringValue\"", v)
	}
	switch val := v.(type) {
	case Int64Value:
		return stringTransformFromInt64(val)
	case Float64Value:
		return stringTransformFromFloat64(val)
	case BoolValue:
		return stringTransformFromBool(val)
	case PMapValue:
		return stringTransformFromPMap(val)
	case PSliceValue:
		return stringTransformFromPSlice(val)
	case PValueValue:
		return stringTransformFromPValue(val)
	default:
		return StringValue{}, fmt.Errorf("unsupported type conversion from %T to \"string\"", v)
	}
}

// stringTransformFromPValue is a transformFunc that is capable of transforming pcommon.Value into string
func stringTransformFromPValue(v PValueValue) (StringValue, error) {
	return NewStringValue(v.Val().AsString()), nil
}

// stringTransformFromPMap is a transformFunc that is capable of transforming pcommon.Map into string
func stringTransformFromPMap(v PMapValue) (StringValue, error) {
	val, err := json.Marshal(v.Val().AsRaw())
	if err != nil {
		return StringValue{}, fmt.Errorf("unable to marshal pcommon.Map to string: %w", err)
	}
	return NewStringValue(string(val)), nil
}

// stringTransformFromPSlice is a transformFunc that is capable of transforming pcommon.Slice into string
func stringTransformFromPSlice(v PSliceValue) (StringValue, error) {
	val, err := json.Marshal(v.Val().AsRaw())
	if err != nil {
		return StringValue{}, fmt.Errorf("unable to convert pcommon.Slice to string: %w", err)
	}
	return NewStringValue(string(val)), nil
}

// stringTransformFromInt64 is a transformFunc that is capable of transforming any Int64Value into string
func stringTransformFromInt64(v Int64Value) (StringValue, error) {
	return NewStringValue(strconv.FormatInt(v.Val(), 10)), nil
}

// stringTransformFromFloat64 is a transformFunc that is capable of transforming any value into string
func stringTransformFromFloat64(v Float64Value) (StringValue, error) {
	return NewStringValue(strconv.FormatFloat(v.Val(), 'g', -1, 64)), nil
}

// stringTransformFromBool is a transformFunc that is capable of transforming any value into string
func stringTransformFromBool(v BoolValue) (StringValue, error) {
	return NewStringValue(strconv.FormatBool(v.Val())), nil
}
