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

var registry = make(map[string]interface{})

func newFunctionCall(inv common.Invocation) (func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) interface{}, error) {
	if f, ok := registry[inv.Function]; ok {
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
			case "getSetter":
				arg, err := newGetSetter(argDef)
				if err != nil {
					return nil, fmt.Errorf("invalid argument at position %v %w", i, err)
				}
				args = append(args, reflect.ValueOf(arg))
				continue
			case "getter":
				arg, err := newGetter(argDef)
				if err != nil {
					return nil, fmt.Errorf("invalid argument at position %v %w", i, err)
				}
				args = append(args, reflect.ValueOf(arg))
				continue
			}
		}
		val := reflect.ValueOf(f)
		ret := val.Call(args)
		return ret[0].Interface().(func(span pdata.Span, il pdata.InstrumentationLibrary, resource pdata.Resource) interface{}), nil
	}
	return nil, fmt.Errorf("undefined function %v", inv.Function)
}

func registerFunction(name string, fun interface{}) {
	registry[name] = fun
}

func unregisterFunction(name string) {
	delete(registry, name)
}
