// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlruntime // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlruntime"

import (
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

type PMapValue struct {
	value[pcommon.Map]
}

func NewPMapValue(val pcommon.Map) PMapValue {
	return PMapValue{value: newValue(val, false)}
}

func NewNilPMapValue() PMapValue {
	return PMapValue{value: newValue(pcommon.Map{}, true)}
}

func ToPMapValue(v Value) (PMapValue, error) {
	if val, ok := v.(PMapValue); ok {
		return val, nil
	}
	if v.IsNil() {
		return PMapValue{}, fmt.Errorf("unsupported type conversion from nil %T to \"PMapValue\"", v)
	}
	switch val := v.(type) {
	case PValueValue:
		return pMapTransformFromPValue(val)
	default:
		return PMapValue{}, fmt.Errorf("unsupported type conversion from %T to \"pcommon.Map\"", v)
	}
}

// pMapTransformFromPMap is a transformFunc that is capable of transforming PValueValue into PMapValue
func pMapTransformFromPValue(v PValueValue) (PMapValue, error) {
	val := v.Val()
	if val.Type() != pcommon.ValueTypeMap {
		return PMapValue{}, fmt.Errorf("invalid type conversion from pcommon.Value[%T] to \"pcommon.Map\"", val)
	}
	return NewPMapValue(val.Map()), nil
}
