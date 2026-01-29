// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlruntime // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlruntime"

import (
	"fmt"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

type DurationValue struct {
	value[time.Duration]
}

func NewDurationValue(val time.Duration) DurationValue {
	return DurationValue{value: newValue(val, false)}
}

func NewNilDurationValue() DurationValue {
	return DurationValue{value: newValue(time.Duration(0), true)}
}

func ToDurationValue(v Value) (DurationValue, error) {
	if val, ok := v.(DurationValue); ok {
		return val, nil
	}
	if v.IsNil() {
		return DurationValue{}, fmt.Errorf("unsupported type conversion from nil %T to \"DurationValue\"", v)
	}
	switch val := v.(type) {
	case StringValue:
		return durationTransformFromString(val)
	case Int64Value:
		return durationTransformFromInt64(val)
	case PValueValue:
		return durationTransformFromPValue(val)
	default:
		return DurationValue{}, fmt.Errorf("unsupported type conversion from %T to \"duration\"", v)
	}
}

func durationTransformFromPValue(v PValueValue) (DurationValue, error) {
	pv := v.Val()
	switch pv.Type() {
	case pcommon.ValueTypeInt:
		return NewDurationValue(time.Duration(pv.Int())), nil
	case pcommon.ValueTypeStr:
		d, err := time.ParseDuration(pv.Str())
		if err != nil {
			return DurationValue{}, err
		}
		return NewDurationValue(d), nil
	default:
		return DurationValue{}, fmt.Errorf("unsupported type conversion from pcommon.Value[%q] to \"duration\"", pv.Type())
	}
}

func durationTransformFromString(v StringValue) (DurationValue, error) {
	d, err := time.ParseDuration(v.Val())
	if err != nil {
		return DurationValue{}, err
	}
	return NewDurationValue(d), nil
}

func durationTransformFromInt64(v Int64Value) (DurationValue, error) {
	return NewDurationValue(time.Duration(v.Val())), nil
}
