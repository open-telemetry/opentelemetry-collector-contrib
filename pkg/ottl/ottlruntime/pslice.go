// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlruntime // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlruntime"

import (
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

type PSliceValue struct {
	value[pcommon.Slice]
}

func NewPSliceValue(val pcommon.Slice) PSliceValue {
	return PSliceValue{value: newValue(val, false)}
}

func NewNilPSliceValue() PSliceValue {
	return PSliceValue{value: newValue(pcommon.Slice{}, true)}
}

func ToPSliceValue(v Value) (PSliceValue, error) {
	if val, ok := v.(PSliceValue); ok {
		return val, nil
	}
	if v.IsNil() {
		return PSliceValue{}, fmt.Errorf("unsupported type conversion from nil %T to \"PSliceValue\"", v)
	}
	switch val := v.(type) {
	case PValueValue:
		return pSliceTransformFromPValue(val)
	default:
		return PSliceValue{}, fmt.Errorf("unsupported type conversion from %T to \"pcommon.Slice\"", v)
	}
}

// pSliceTransformFromPSlice is a transformFunc that is capable of transforming PValueValue into PSliceValue
func pSliceTransformFromPValue(v PValueValue) (PSliceValue, error) {
	val := v.Val()
	if val.Type() != pcommon.ValueTypeSlice {
		return PSliceValue{}, fmt.Errorf("invalid type conversion from pcommon.Value[%T] to \"pcommon.Slice\"", val)
	}
	return NewPSliceValue(val.Slice()), nil
}
