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

	"go.opentelemetry.io/collector/pdata/pcommon"
)

type TransformFunction func(arguments []Value, pathParser PathExpressionParser, functions map[string]TransformFunction) (ExprFunc, error)

var registry = map[string]TransformFunction{
	"set":          set,
	"keep_keys":    keepKeys,
	"truncate_all": truncateAll,
	"limit":        limit,
}

type PathExpressionParser func(*Path) (GetSetter, error)

func DefaultFunctions() map[string]TransformFunction {
	return registry
}

func set(arguments []Value, pathParser PathExpressionParser, functions map[string]TransformFunction) (ExprFunc, error) {
	if len(arguments) != 2 {
		return nil, fmt.Errorf("incorrect number of arguments for function 'set': received %v, expected 2", len(arguments))
	}

	target, err := toSetter(arguments[0], pathParser)
	if err != nil {
		return nil, err
	}

	value, err := toGetter(arguments[1], pathParser, functions)
	if err != nil {
		return nil, err
	}

	return func(ctx TransformContext) interface{} {
		val := value.Get(ctx)
		if val != nil {
			target.Set(ctx, val)
		}
		return nil
	}, nil
}

func keepKeys(arguments []Value, pathParser PathExpressionParser, functions map[string]TransformFunction) (ExprFunc, error) {
	if len(arguments) < 1 {
		return nil, fmt.Errorf("incorrect number of arguments for function 'keep_keys': received %v, expected at least 1", len(arguments))
	}

	target, err := toGetSetter(arguments[0], pathParser)
	if err != nil {
		return nil, err
	}

	keys, err := toStringArray(arguments[1:])
	if err != nil {
		return nil, err
	}

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

func truncateAll(arguments []Value, pathParser PathExpressionParser, functions map[string]TransformFunction) (ExprFunc, error) {
	if len(arguments) != 2 {
		return nil, fmt.Errorf("incorrect number of arguments for function 'truncate_all': received %v, expected 2", len(arguments))
	}

	target, err := toGetSetter(arguments[0], pathParser)
	if err != nil {
		return nil, err
	}

	limitVal, err := toInt(arguments[1])
	if err != nil {
		return nil, err
	}

	if limitVal < 0 {
		return nil, fmt.Errorf("invalid limit for truncate_all function, %d cannot be negative", limitVal)
	}

	return func(ctx TransformContext) interface{} {
		if limitVal < 0 {
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
				if int64(len(stringVal)) > limitVal {
					updated.InsertString(key, stringVal[:limitVal])
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

func limit(arguments []Value, pathParser PathExpressionParser, functions map[string]TransformFunction) (ExprFunc, error) {
	if len(arguments) != 2 {
		return nil, fmt.Errorf("incorrect number of arguments for function 'limit': received %v, expected 2", len(arguments))
	}

	target, err := toGetSetter(arguments[0], pathParser)
	if err != nil {
		return nil, err
	}

	limitVal, err := toInt(arguments[1])
	if err != nil {
		return nil, err
	}

	if limitVal < 0 {
		return nil, fmt.Errorf("invalid limit for limit function, %d cannot be negative", limitVal)
	}
	return func(ctx TransformContext) interface{} {
		val := target.Get(ctx)
		if val == nil {
			return nil
		}

		if attrs, ok := val.(pcommon.Map); ok {
			if int64(attrs.Len()) <= limitVal {
				return nil
			}

			updated := pcommon.NewMap()
			updated.EnsureCapacity(attrs.Len())
			count := int64(0)
			attrs.Range(func(key string, val pcommon.Value) bool {
				if count < limitVal {
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

func toInt(arg Value) (int64, error) {
	if arg.Int == nil {
		return 0, fmt.Errorf("expected argument to be an int")
	}
	return *arg.Int, nil
}

func toStringArray(args []Value) ([]string, error) {
	results := make([]string, len(args))
	for _, arg := range args {
		if arg.String == nil {
			return nil, fmt.Errorf("invalid argument for slice parameter, must be string")
		}
		results = append(results, *arg.String)
	}
	return results, nil
}

func toSetter(arg Value, pathParser PathExpressionParser) (Setter, error) {
	return toGetSetter(arg, pathParser)
}

func toGetter(arg Value, pathParser PathExpressionParser, functions map[string]TransformFunction) (Getter, error) {
	getter, err := NewGetter(arg, functions, pathParser)
	if err != nil {
		return nil, fmt.Errorf("invalid argument, %w", err)
	}
	return getter, nil
}

func toGetSetter(arg Value, pathParser PathExpressionParser) (GetSetter, error) {
	if arg.Path == nil {
		return nil, fmt.Errorf("expected argument to be a path")
	}
	getSetter, err := pathParser(arg.Path)
	if err != nil {
		return nil, fmt.Errorf("could not parse path, %v", err)
	}
	return getSetter, nil
}

// NewFunctionCall Visible for testing
func NewFunctionCall(inv Invocation, functions map[string]TransformFunction, pathParser PathExpressionParser) (ExprFunc, error) {
	if f, ok := functions[inv.Function]; ok {
		return f(inv.Arguments, pathParser, functions)
	}
	return nil, fmt.Errorf("undefined function %v", inv.Function)
}
