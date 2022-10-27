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

package ottl // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
)

type PathExpressionParser[K any] func(*Path) (GetSetter[K], error)

type EnumParser func(*EnumSymbol) (*Enum, error)

type Enum int64

func (p *Parser[K]) newFunctionCall(inv invocation) (ExprFunc[K], error) {
	f, ok := p.functions[inv.Function]
	if !ok {
		return nil, fmt.Errorf("undefined function %v", inv.Function)
	}
	args, err := p.buildArgs(inv, reflect.TypeOf(f))
	if err != nil {
		return nil, err
	}

	returnVals := reflect.ValueOf(f).Call(args)

	if returnVals[1].IsNil() {
		err = nil
	} else {
		err = returnVals[1].Interface().(error)
	}

	return returnVals[0].Interface().(ExprFunc[K]), err
}

func (p *Parser[K]) buildArgs(inv invocation, fType reflect.Type) ([]reflect.Value, error) {
	var args []reflect.Value
	// Some function arguments may be intended to take values from the calling processor
	// instead of being passed by the caller of the OTTL function, so we have to keep
	// track of the index of the argument passed within the DSL.
	// e.g. TelemetrySettings, which is provided by the processor to the OTTL Parser struct.
	DSLArgumentIndex := 0
	for i := 0; i < fType.NumIn(); i++ {
		argType := fType.In(i)

		if argType.Kind() == reflect.Slice {
			arg, err := p.buildSliceArg(inv, argType, i)
			if err != nil {
				return nil, err
			}
			args = append(args, arg)
		} else {
			arg, isInternalArg := p.buildInternalArg(argType)

			if isInternalArg {
				args = append(args, arg)
				continue
			}

			if DSLArgumentIndex >= len(inv.Arguments) {
				return nil, fmt.Errorf("not enough arguments for function %v", inv.Function)
			}

			argDef := inv.Arguments[DSLArgumentIndex]
			val, err := p.buildArg(argDef, argType, DSLArgumentIndex)

			if err != nil {
				return nil, err
			}

			args = append(args, reflect.ValueOf(val))
		}

		DSLArgumentIndex++
	}

	if len(inv.Arguments) > DSLArgumentIndex {
		return nil, fmt.Errorf("too many arguments for function %v", inv.Function)
	}

	return args, nil
}

func (p *Parser[K]) buildSliceArg(inv invocation, argType reflect.Type, index int) (reflect.Value, error) {
	name := argType.Elem().Name()
	switch {
	case name == reflect.Uint8.String():
		if inv.Arguments[index].Bytes == nil {
			return reflect.ValueOf(nil), fmt.Errorf("invalid argument for slice parameter at position %v, must be a byte slice literal", index)
		}
		return reflect.ValueOf(([]byte)(*inv.Arguments[index].Bytes)), nil
	case name == reflect.String.String():
		arg, err := buildSlice[string](inv, argType, index, p.buildArg, name)
		if err != nil {
			return reflect.ValueOf(nil), err
		}
		return arg, nil
	case name == reflect.Float64.String():
		arg, err := buildSlice[float64](inv, argType, index, p.buildArg, name)
		if err != nil {
			return reflect.ValueOf(nil), err
		}
		return arg, nil
	case name == reflect.Int64.String():
		arg, err := buildSlice[int64](inv, argType, index, p.buildArg, name)
		if err != nil {
			return reflect.ValueOf(nil), err
		}
		return arg, nil
	case strings.HasPrefix(name, "Getter"):
		arg, err := buildSlice[Getter[K]](inv, argType, index, p.buildArg, name)
		if err != nil {
			return reflect.ValueOf(nil), err
		}
		return arg, nil
	default:
		return reflect.ValueOf(nil), fmt.Errorf("unsupported slice type '%s' for function '%v'", argType.Elem().Name(), inv.Function)
	}
}

// Handle interfaces that can be passed as arguments to OTTL function invocations.
func (p *Parser[K]) buildArg(argDef value, argType reflect.Type, index int) (any, error) {
	name := argType.Name()
	switch {
	case strings.HasPrefix(name, "Setter"):
		fallthrough
	case strings.HasPrefix(name, "GetSetter"):
		arg, err := p.pathParser(argDef.Path)
		if err != nil {
			return nil, fmt.Errorf("invalid argument at position %v %w", index, err)
		}
		return arg, nil
	case strings.HasPrefix(name, "Getter"):
		arg, err := p.newGetter(argDef)
		if err != nil {
			return nil, fmt.Errorf("invalid argument at position %v %w", index, err)
		}
		return arg, nil
	case name == "Enum":
		arg, err := p.enumParser(argDef.Enum)
		if err != nil {
			return nil, fmt.Errorf("invalid argument at position %v must be an Enum", index)
		}
		return *arg, nil
	case name == reflect.String.String():
		if argDef.String == nil {
			return nil, fmt.Errorf("invalid argument at position %v, must be an string", index)
		}
		return *argDef.String, nil
	case name == reflect.Float64.String():
		if argDef.Float == nil {
			return nil, fmt.Errorf("invalid argument at position %v, must be an float", index)
		}
		return *argDef.Float, nil
	case name == reflect.Int64.String():
		if argDef.Int == nil {
			return nil, fmt.Errorf("invalid argument at position %v, must be an int", index)
		}
		return *argDef.Int, nil
	case name == reflect.Bool.String():
		if argDef.Bool == nil {
			return nil, fmt.Errorf("invalid argument at position %v, must be a bool", index)
		}
		return bool(*argDef.Bool), nil
	default:
		return nil, errors.New("unsupported argument type")
	}
}

// Handle interfaces that can be declared as parameters to a OTTL function, but will
// never be called in an invocation. Returns whether the arg is an internal arg.
func (p *Parser[K]) buildInternalArg(argType reflect.Type) (reflect.Value, bool) {
	if argType.Name() == "TelemetrySettings" {
		return reflect.ValueOf(p.telemetrySettings), true
	}
	return reflect.ValueOf(nil), false
}

type buildArgFunc func(value, reflect.Type, int) (any, error)

func buildSlice[T any](inv invocation, argType reflect.Type, index int, buildArg buildArgFunc, name string) (reflect.Value, error) {
	if inv.Arguments[index].List == nil {
		return reflect.ValueOf(nil), fmt.Errorf("invalid argument for parameter at position %v, must be a list of type %v", index, name)
	}

	vals := []T{}
	values := inv.Arguments[index].List.Values
	for j := 0; j < len(values); j++ {
		untypedVal, err := buildArg(values[j], argType.Elem(), j)
		if err != nil {
			return reflect.ValueOf(nil), err
		}

		val, ok := untypedVal.(T)

		if !ok {
			return reflect.ValueOf(nil), fmt.Errorf("invalid element type at list index %v for argument at position %v, must be of type %v", j, index, name)
		}

		vals = append(vals, val)
	}

	return reflect.ValueOf(vals), nil
}
