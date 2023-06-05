// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal"

import (
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/internal/ottlcommon"
)

func GetMapValue(m pcommon.Map, keys []ottl.Key) (interface{}, error) {
	if len(keys) == 0 {
		return nil, fmt.Errorf("cannot get map value without key")
	}
	if keys[0].String == nil {
		return nil, fmt.Errorf("non-string indexing is not supported")
	}

	val, ok := m.Get(*keys[0].String)
	if !ok {
		return nil, nil
	}
	for i := 1; i < len(keys); i++ {
		switch val.Type() {
		case pcommon.ValueTypeMap:
			if keys[i].String == nil {
				return nil, fmt.Errorf("map must be indexed by a string")
			}
			val, ok = val.Map().Get(*keys[i].String)
			if !ok {
				return nil, nil
			}
		case pcommon.ValueTypeSlice:
			if keys[i].Int == nil {
				return nil, fmt.Errorf("slice must be indexed by an int")
			}
			if int(*keys[i].Int) >= val.Slice().Len() || int(*keys[i].Int) < 0 {
				return nil, fmt.Errorf("index %v out of bounds", *keys[i].Int)
			}
			val = val.Slice().At(int(*keys[i].Int))
		default:
			return nil, fmt.Errorf("type %v does not support string indexing", val.Type())
		}
	}
	return ottlcommon.GetValue(val), nil
}

func SetMapValue(m pcommon.Map, keys []ottl.Key, val interface{}) error {
	if len(keys) == 0 {
		return fmt.Errorf("cannot set map value without key")
	}
	if keys[0].String == nil {
		return fmt.Errorf("non-string indexing is not supported")
	}

	var newValue pcommon.Value
	switch val.(type) {
	case []string, []bool, []int64, []float64, [][]byte, []any:
		newValue = pcommon.NewValueSlice()
	default:
		newValue = pcommon.NewValueEmpty()
	}
	err := SetValue(newValue, val)
	if err != nil {
		return err
	}

	currentValue, ok := m.Get(*keys[0].String)
	if !ok {
		currentValue = m.PutEmpty(*keys[0].String)
	}

	for i := 1; i < len(keys); i++ {
		switch currentValue.Type() {
		case pcommon.ValueTypeMap:
			if keys[i].String == nil {
				return fmt.Errorf("map must be indexed by a string")
			}
			potentialValue, ok := currentValue.Map().Get(*keys[i].String)
			if !ok {
				currentValue = currentValue.Map().PutEmpty(*keys[i].String)
			} else {
				currentValue = potentialValue
			}
		case pcommon.ValueTypeSlice:
			if keys[i].Int == nil {
				return fmt.Errorf("slice must be indexed by an int")
			}
			if int(*keys[i].Int) >= currentValue.Slice().Len() || int(*keys[i].Int) < 0 {
				return fmt.Errorf("index %v out of bounds", *keys[i].Int)
			}
			currentValue = currentValue.Slice().At(int(*keys[i].Int))
		case pcommon.ValueTypeEmpty:
			if keys[i].String != nil {
				currentValue = currentValue.SetEmptyMap().PutEmpty(*keys[i].String)
			} else if keys[i].Int != nil {
				currentValue.SetEmptySlice()
				for k := 0; k < int(*keys[i].Int); k++ {
					currentValue.Slice().AppendEmpty()
				}
				currentValue = currentValue.Slice().AppendEmpty()
			}
		default:
			return fmt.Errorf("type %v does not support string indexing", currentValue.Type())
		}
	}
	newValue.CopyTo(currentValue)
	return nil
}
