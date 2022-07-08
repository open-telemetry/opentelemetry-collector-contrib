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
)

var registry = map[string]interface{}{
	"TraceID":              traceID,
	"SpanID":               spanID,
	"IsMatch":              isMatch,
	"keep_keys":            keepKeys,
	"set":                  set,
	"truncate_all":         truncateAll,
	"limit":                limit,
	"replace_match":        replaceMatch,
	"replace_all_matches":  replaceAllMatches,
	"replace_pattern":      replacePattern,
	"replace_all_patterns": replaceAllPatterns,
}

type PathExpressionParser func(*Path) (GetSetter, error)

func DefaultFunctions() map[string]interface{} {
	return registry
}

// NewFunctionCall Visible for testing
func NewFunctionCall(inv Invocation, functions map[string]interface{}, pathParser PathExpressionParser) (ExprFunc, error) {
	if f, ok := functions[inv.Function]; ok {
		args, err := buildArgs(inv, reflect.TypeOf(f), functions, pathParser)
		if err != nil {
			return nil, err
		}

		returnVals := reflect.ValueOf(f).Call(args)

		if returnVals[1].IsNil() {
			err = nil
		} else {
			err = returnVals[1].Interface().(error)
		}

		return returnVals[0].Interface().(ExprFunc), err
	}
	return nil, fmt.Errorf("undefined function %v", inv.Function)
}

func buildArgs(inv Invocation, fType reflect.Type, functions map[string]interface{}, pathParser PathExpressionParser) ([]reflect.Value, error) {
	args := make([]reflect.Value, 0)
	for i := 0; i < fType.NumIn(); i++ {
		argType := fType.In(i)

		if argType.Kind() == reflect.Slice {
			err := buildSliceArg(inv, argType, i, &args)
			if err != nil {
				return nil, err
			}
		} else {
			if i >= len(inv.Arguments) {
				return nil, fmt.Errorf("not enough arguments for function %v", inv.Function)
			}

			argDef := inv.Arguments[i]
			err := buildArg(argDef, argType, i, &args, functions, pathParser)
			if err != nil {
				return nil, err
			}
		}
	}
	return args, nil
}

func buildSliceArg(inv Invocation, argType reflect.Type, startingIndex int, args *[]reflect.Value) error {
	switch argType.Elem().Kind() {
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
	case reflect.Uint8:
		if inv.Arguments[startingIndex].Bytes == nil {
			return fmt.Errorf("invalid argument for slice parameter at position %v, must be a byte slice literal", startingIndex)
		}
		*args = append(*args, reflect.ValueOf(([]byte)(*inv.Arguments[startingIndex].Bytes)))
	default:
		return fmt.Errorf("unsupported slice type for function %v", inv.Function)
	}
	return nil
}

func buildArg(argDef Value, argType reflect.Type, index int, args *[]reflect.Value,
	functions map[string]interface{}, pathParser PathExpressionParser) error {
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
	case "bool":
		if argDef.Bool == nil {
			return fmt.Errorf("invalid argument at position %v, must be a bool", index)
		}
		*args = append(*args, reflect.ValueOf(bool(*argDef.Bool)))
	}
	return nil
}
