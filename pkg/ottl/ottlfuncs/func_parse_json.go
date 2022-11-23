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

package ottlfuncs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"

import (
	"context"
	"fmt"

	jsoniter "github.com/json-iterator/go"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func ParseJSON[K any](target ottl.Getter[K]) (ottl.ExprFunc[K], error) {
	return func(ctx context.Context, tCtx K) (interface{}, error) {
		targetVal, err := target.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}
		if jsonStr, ok := targetVal.(string); ok {
			var parsedValue map[string]interface{}
			err := jsoniter.UnmarshalFromString(jsonStr, &parsedValue)
			if err != nil {
				return pcommon.Map{}, err
			}
			result := pcommon.NewMap()
			for k, v := range parsedValue {
				attrVal := pcommon.NewValueEmpty()
				err = setValue(attrVal, v)
				if err != nil {
					return pcommon.Map{}, err
				}
				attrVal.CopyTo(result.PutEmpty(k))
			}
			return result, nil
		}
		return nil, nil
	}, nil
}

func setValue(value pcommon.Value, val interface{}) error {
	switch v := val.(type) {
	case string:
		value.SetStr(v)
	case bool:
		value.SetBool(v)
	case float64:
		value.SetDouble(v)
	case nil:
	case []interface{}:
		emptySlice := value.SetEmptySlice()
		err := setSlice(emptySlice, v)
		if err != nil {
			return err
		}
	case map[string]interface{}:
		err := value.SetEmptyMap().FromRaw(v)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown type, %T", v)
	}
	return nil
}

func setSlice(slice pcommon.Slice, value []interface{}) error {
	for _, item := range value {
		emptyValue := slice.AppendEmpty()
		err := setValue(emptyValue, item)
		if err != nil {
			return err
		}
	}
	return nil
}
