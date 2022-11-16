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

	jsoniter "github.com/json-iterator/go"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func ParseJSONIntoMap[K any](target ottl.GetSetter[K], value ottl.Getter[K]) (ottl.ExprFunc[K], error) {
	return func(ctx context.Context, tCtx K) (interface{}, error) {
		targetVal, err := target.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}
		if attrs, ok := targetVal.(pcommon.Map); ok {
			val, err := value.Get(ctx, tCtx)
			if err != nil {
				return nil, err
			}
			if valStr, ok := val.(string); ok {
				var parsedValue map[string]interface{}
				err = jsoniter.UnmarshalFromString(valStr, &parsedValue)
				if err != nil {
					return nil, err
				}
				for k, v := range parsedValue {
					attrVal := pcommon.NewValueEmpty()
					err = setValue(attrVal, v)
					if err != nil {
						return nil, err
					}
					attrVal.CopyTo(attrs.PutEmpty(k))
				}
			}
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
		mapStr, err := jsoniter.MarshalToString(val)
		if err != nil {
			return err
		}
		value.SetStr(mapStr)
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
