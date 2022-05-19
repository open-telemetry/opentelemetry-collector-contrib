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

package common // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"

import (
	"fmt"
	"reflect"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

var registry = map[string]interface{}{
	"keep_keys":    keepKeys,
	"set":          set,
	"truncate_all": truncateAll,
	"limit":        limit,
}

type PathExpressionParser func(*Path) (GetSetter, error)

func DefaultFunctions() map[string]interface{} {
	return registry
}

func set(target Setter, value Getter) (ExprFunc, error) {
	return func(ctx TransformContext) interface{} {
		val := value.Get(ctx)
		if val != nil {
			target.Set(ctx, val)
		}
		return nil
	}, nil
}

func keepKeys(target GetSetter, keys []string) (ExprFunc, error) {
	keySet := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		keySet[key] = struct{}{}
	}

	return func(ctx TransformContext) interface{} {
		val := target.Get(ctx)
		if val == nil {
			return nil
		}

		if attrs, ok := val.(pcommon.Map); ok {
			// TODO(anuraaga): Avoid copying when filtering keys https://github.com/open-telemetry/opentelemetry-collector/issues/4756
			filtered := pcommon.NewMap()
			filtered.EnsureCapacity(attrs.Len())
			attrs.Range(func(key string, val pcommon.Value) bool {
				if _, ok := keySet[key]; ok {
					filtered.Insert(key, val)
				}
				return true
			})
			target.Set(ctx, filtered)
		}
		return nil
	}, nil
}

func truncateAll(target GetSetter, limit int64) (ExprFunc, error) {
	if limit < 0 {
		return nil, fmt.Errorf("invalid limit for truncate_all function, %d cannot be negative", limit)
	}
	return func(ctx TransformContext) interface{} {
		if limit < 0 {
			return nil
		}

		val := target.Get(ctx)
		if val == nil {
			return nil
		}

		if attrs, ok := val.(pcommon.Map); ok {
			updated := pcommon.NewMap()
			updated.EnsureCapacity(attrs.Len())
			attrs.Range(func(key string, val pcommon.Value) bool {
				stringVal := val.StringVal()
				if int64(len(stringVal)) > limit {
					updated.InsertString(key, stringVal[:limit])
				} else {
					updated.Insert(key, val)
				}
				return true
			})
			target.Set(ctx, updated)
			// TODO: Write log when truncation is performed
			// https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/9730
		}
		return nil
	}, nil
}

func limit(target GetSetter, limit int64) (ExprFunc, error) {
	if limit < 0 {
		return nil, fmt.Errorf("invalid limit for limit function, %d cannot be negative", limit)
	}
	return func(ctx TransformContext) interface{} {
		val := target.Get(ctx)
		if val == nil {
			return nil
		}

		if attrs, ok := val.(pcommon.Map); ok {
			if int64(attrs.Len()) <= limit {
				return nil
			}

			updated := pcommon.NewMap()
			updated.EnsureCapacity(attrs.Len())
			count := int64(0)
			attrs.Range(func(key string, val pcommon.Value) bool {
				if count < limit {
					updated.Insert(key, val)
					count++
					return true
				}
				return false
			})
			target.Set(ctx, updated)
			// TODO: Write log when limiting is performed
			// https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/9730
		}
		return nil
	}, nil
}

// NewFunctionCall Visible for testing
func NewFunctionCall(inv Invocation, functions map[string]interface{}, pathParser PathExpressionParser) (ExprFunc, error) {
	if f, ok := functions[inv.Function]; ok {
		args, err := buildArgs(inv, functions, pathParser, f)
		if err != nil {
			return nil, err
		}

		val := reflect.ValueOf(f)
		ret := val.Call(*args)

		if ret[1].IsNil() {
			err = nil
		} else {
			err = ret[1].Interface().(error)
		}

		return ret[0].Interface().(ExprFunc), err
	}
	return nil, fmt.Errorf("undefined function %v", inv.Function)
}

func buildArgs(inv Invocation, functions map[string]interface{}, pathParser PathExpressionParser, f interface{}) (*[]reflect.Value, error) {
	fType := reflect.TypeOf(f)
	args := make([]reflect.Value, 0)
	for i := 0; i < fType.NumIn(); i++ {
		argType := fType.In(i)

		if argType.Kind() == reflect.Slice {
			err := buildSliceArg(inv, argType.Elem().Kind(), i, &args)
			if err != nil {
				return nil, err
			}
			continue
		}

		if i >= len(inv.Arguments) {
			return nil, fmt.Errorf("not enough arguments for function %v", inv.Function)
		}

		argDef := inv.Arguments[i]
		err := buildArg(argDef, argType, i, functions, pathParser, &args)
		if err != nil {
			return nil, err
		}
	}
	return &args, nil
}

func buildSliceArg(inv Invocation, argType reflect.Kind, startingIndex int, args *[]reflect.Value) error {
	switch argType {
	case reflect.String:
		arg := make([]string, 0)
		for j := startingIndex; j < len(inv.Arguments); j++ {
			if inv.Arguments[j].String == nil {
				return fmt.Errorf("invalid argument for slice parameter at position %v, must be a string", j)
			}
			arg = append(arg, *inv.Arguments[j].String)
		}
		*args = append(*args, reflect.ValueOf(arg))
	case reflect.Float64:
		arg := make([]float64, 0)
		for j := startingIndex; j < len(inv.Arguments); j++ {
			if inv.Arguments[j].Float == nil {
				return fmt.Errorf("invalid argument for slice parameter at position %v, must be a float", j)
			}
			arg = append(arg, *inv.Arguments[j].Float)
		}
		*args = append(*args, reflect.ValueOf(arg))
	case reflect.Int64:
		arg := make([]int64, 0)
		for j := startingIndex; j < len(inv.Arguments); j++ {
			if inv.Arguments[j].Int == nil {
				return fmt.Errorf("invalid argument for slice parameter at position %v, must be an int", j)
			}
			arg = append(arg, *inv.Arguments[j].Int)
		}
		*args = append(*args, reflect.ValueOf(arg))
	default:
		return fmt.Errorf("unsupported slice type for function %v", inv.Function)
	}
	return nil
}

func buildArg(argDef Value, argType reflect.Type, index int, functions map[string]interface{},
	pathParser PathExpressionParser, args *[]reflect.Value) error {
	switch argType.Name() {
	case "Setter":
		fallthrough
	case "GetSetter":
		arg, err := pathParser(argDef.Path)
		if err != nil {
			return fmt.Errorf("invalid argument at position %v %w", index, err)
		}
		*args = append(*args, reflect.ValueOf(arg))
	case "Getter":
		arg, err := NewGetter(argDef, functions, pathParser)
		if err != nil {
			return fmt.Errorf("invalid argument at position %v %w", index, err)
		}
		*args = append(*args, reflect.ValueOf(arg))
	case "string":
		if argDef.String == nil {
			return fmt.Errorf("invalid argument at position %v, must be an string", index)
		}
		*args = append(*args, reflect.ValueOf(*argDef.String))
	case "float64":
		if argDef.Float == nil {
			return fmt.Errorf("invalid argument at position %v, must be an float", index)
		}
		*args = append(*args, reflect.ValueOf(*argDef.Float))
	case "int64":
		if argDef.Int == nil {
			return fmt.Errorf("invalid argument at position %v, must be an int", index)
		}
		*args = append(*args, reflect.ValueOf(*argDef.Int))
	}
	return nil
}
