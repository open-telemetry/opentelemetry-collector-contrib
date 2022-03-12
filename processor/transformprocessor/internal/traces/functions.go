// Copyright  The OpenTelemetry Authors
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

package traces // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/traces"

import (
	"fmt"
	"reflect"

	"go.opentelemetry.io/collector/model/pdata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"
)

var registry = map[string]interface{}{
	"keep_keys": keepKeys,
	"set":       set,
}

func DefaultFunctions() map[string]interface{} {
	return registry
}

func set(target setter, value getter) exprFunc {
	return func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) interface{} {
		val := value.get(span, il, resource)
		if val != nil {
			target.set(span, il, resource, val)
		}
		return nil
	}
}

func keepKeys(target getSetter, keys []string) exprFunc {
	keySet := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		keySet[key] = struct{}{}
	}

	return func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) interface{} {
		val := target.get(span, il, resource)
		if val == nil {
			return nil
		}

		if attrs, ok := val.(pdata.AttributeMap); ok {
			// TODO(anuraaga): Avoid copying when filtering keys https://github.com/open-telemetry/opentelemetry-collector/issues/4756
			filtered := pdata.NewAttributeMap()
			filtered.EnsureCapacity(attrs.Len())
			attrs.Range(func(key string, val pdata.AttributeValue) bool {
				if _, ok := keySet[key]; ok {
					filtered.Insert(key, val)
				}
				return true
			})
			target.set(span, il, resource, filtered)
		}
		return nil
	}
}

// TODO(anuraaga): See if reflection can be avoided without complicating definition of transform functions.
func newFunctionCall(inv common.Invocation, functions map[string]interface{}) (exprFunc, error) {
	if f, ok := functions[inv.Function]; ok {
		fType := reflect.TypeOf(f)
		args := make([]reflect.Value, 0)
		for i := 0; i < fType.NumIn(); i++ {
			argType := fType.In(i)

			if argType.Kind() == reflect.Slice {
				switch argType.Elem().Kind() {
				case reflect.String:
					arg := make([]string, 0)
					for j := i; j < len(inv.Arguments); j++ {
						if inv.Arguments[j].String == nil {
							return nil, fmt.Errorf("invalid argument for slice parameter at position %v, must be string", j)
						}
						arg = append(arg, *inv.Arguments[j].String)
					}
					args = append(args, reflect.ValueOf(arg))
				default:
					return nil, fmt.Errorf("unsupported slice type for function %v", inv.Function)
				}
				continue
			}

			if i >= len(inv.Arguments) {
				return nil, fmt.Errorf("not enough arguments for function %v", inv.Function)
			}
			argDef := inv.Arguments[i]
			switch argType.Name() {
			case "setter":
				fallthrough
			case "getSetter":
				arg, err := newGetSetter(argDef)
				if err != nil {
					return nil, fmt.Errorf("invalid argument at position %v %w", i, err)
				}
				args = append(args, reflect.ValueOf(arg))
				continue
			case "getter":
				arg, err := newGetter(argDef, functions)
				if err != nil {
					return nil, fmt.Errorf("invalid argument at position %v %w", i, err)
				}
				args = append(args, reflect.ValueOf(arg))
				continue
			}
		}
		val := reflect.ValueOf(f)
		ret := val.Call(args)
		return ret[0].Interface().(exprFunc), nil
	}
	return nil, fmt.Errorf("undefined function %v", inv.Function)
}
