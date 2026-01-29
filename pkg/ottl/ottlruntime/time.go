// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlruntime // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlruntime"

import (
	"fmt"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

type TimeValue struct {
	value[time.Time]
}

func NewTimeValue(val time.Time) TimeValue {
	return TimeValue{value: newValue(val, false)}
}

func NewNilTimeValue() TimeValue {
	return TimeValue{value: newValue(time.Time{}, true)}
}

func ToTimeValue(v Value) (TimeValue, error) {
	if val, ok := v.(TimeValue); ok {
		return val, nil
	}
	if v.IsNil() {
		return TimeValue{}, fmt.Errorf("unsupported type conversion from nil %T to \"TimeValue\"", v)
	}
	switch val := v.(type) {
	case StringValue:
		return timeTransformFromString(val)
	case Int64Value:
		return timeTransformFromInt64(val)
	case PValueValue:
		return timeTransformFromPValue(val)
	default:
		return TimeValue{}, fmt.Errorf("unsupported type conversion from %T to \"time\"", v)
	}
}

func timeTransformFromPValue(v PValueValue) (TimeValue, error) {
	pv := v.Val()
	switch pv.Type() {
	case pcommon.ValueTypeStr:
		t, err := time.Parse(time.RFC3339, pv.Str())
		if err != nil {
			return TimeValue{}, fmt.Errorf("unable to parse string to time.Time: %w", err)
		}
		return NewTimeValue(t), nil
	case pcommon.ValueTypeInt:
		return NewTimeValue(time.Unix(0, pv.Int())), nil
	default:
		return TimeValue{}, fmt.Errorf("unsupported type conversion from pcommon.Value[%q] to \"time\"", pv.Type())
	}
}

func timeTransformFromString(v StringValue) (TimeValue, error) {
	t, err := time.Parse(time.RFC3339, v.Val())
	if err != nil {
		return TimeValue{}, fmt.Errorf("unable to parse string to time.Time: %w", err)
	}
	return NewTimeValue(t), nil
}

func timeTransformFromInt64(v Int64Value) (TimeValue, error) {
	return NewTimeValue(time.Unix(0, v.Val())), nil
}
