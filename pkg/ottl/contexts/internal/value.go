// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func SetValue(value pcommon.Value, val interface{}) error {
	var err error
	switch v := val.(type) {
	case string:
		value.SetStr(v)
	case bool:
		value.SetBool(v)
	case int64:
		value.SetInt(v)
	case float64:
		value.SetDouble(v)
	case []byte:
		value.SetEmptyBytes().FromRaw(v)
	case []string:
		value.SetEmptySlice().EnsureCapacity(len(v))
		for _, str := range v {
			value.Slice().AppendEmpty().SetStr(str)
		}
	case []bool:
		value.SetEmptySlice().EnsureCapacity(len(v))
		for _, b := range v {
			value.Slice().AppendEmpty().SetBool(b)
		}
	case []int64:
		value.SetEmptySlice().EnsureCapacity(len(v))
		for _, i := range v {
			value.Slice().AppendEmpty().SetInt(i)
		}
	case []float64:
		value.SetEmptySlice().EnsureCapacity(len(v))
		for _, f := range v {
			value.Slice().AppendEmpty().SetDouble(f)
		}
	case [][]byte:
		value.SetEmptySlice().EnsureCapacity(len(v))
		for _, b := range v {
			value.Slice().AppendEmpty().SetEmptyBytes().FromRaw(b)
		}
	case []any:
		value.SetEmptySlice().EnsureCapacity(len(v))
		for _, a := range v {
			pval := value.Slice().AppendEmpty()
			err = SetValue(pval, a)
		}
	case pcommon.Map:
		v.CopyTo(value.SetEmptyMap())
	case map[string]interface{}:
		value.SetEmptyMap()
		for mk, mv := range v {
			err = SetMapValue(value.Map(), []ottl.Key{{String: &mk}}, mv)
		}
	}
	return err
}
