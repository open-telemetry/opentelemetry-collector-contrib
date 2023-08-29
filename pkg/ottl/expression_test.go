// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottl

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottltest"
)

func hello() (ExprFunc[any], error) {
	return func(ctx context.Context, tCtx any) (any, error) {
		return "world", nil
	}, nil
}

func pmap() (ExprFunc[any], error) {
	return func(ctx context.Context, tCtx any) (interface{}, error) {
		m := pcommon.NewMap()
		m.PutEmptyMap("foo").PutStr("bar", "pass")
		return m, nil
	}, nil
}

func basicMap() (ExprFunc[any], error) {
	return func(ctx context.Context, tCtx any) (interface{}, error) {
		return map[string]interface{}{
			"foo": map[string]interface{}{
				"bar": "pass",
			},
		}, nil
	}, nil
}

func pslice() (ExprFunc[any], error) {
	return func(ctx context.Context, tCtx any) (interface{}, error) {
		s := pcommon.NewSlice()
		s.AppendEmpty().SetEmptySlice().AppendEmpty().SetStr("pass")
		return s, nil
	}, nil
}

func basicSlice() (ExprFunc[any], error) {
	return func(ctx context.Context, tCtx any) (interface{}, error) {
		return []interface{}{
			[]interface{}{
				"pass",
			},
		}, nil
	}, nil
}

func Test_newGetter(t *testing.T) {
	tests := []struct {
		name string
		val  value
		ctx  interface{}
		want interface{}
	}{
		{
			name: "string literal",
			val: value{
				String: ottltest.Strp("str"),
			},
			want: "str",
		},
		{
			name: "float literal",
			val: value{
				Literal: &mathExprLiteral{
					Float: ottltest.Floatp(1.2),
				},
			},
			want: 1.2,
		},
		{
			name: "int literal",
			val: value{
				Literal: &mathExprLiteral{
					Int: ottltest.Intp(12),
				},
			},
			want: int64(12),
		},
		{
			name: "bytes literal",
			val: value{
				Bytes: (*byteSlice)(&[]byte{1, 2, 3, 4, 5, 6, 7, 8}),
			},
			want: []byte{1, 2, 3, 4, 5, 6, 7, 8},
		},
		{
			name: "nil literal",
			val: value{
				IsNil: (*isNil)(ottltest.Boolp(true)),
			},
			want: nil,
		},
		{
			name: "bool literal",
			val: value{
				Bool: (*boolean)(ottltest.Boolp(true)),
			},
			want: true,
		},
		{
			name: "path expression",
			val: value{
				Literal: &mathExprLiteral{
					Path: &Path{
						Fields: []Field{
							{
								Name: "name",
							},
						},
					},
				},
			},
			want: "bear",
		},
		{
			name: "complex path expression",
			val: value{
				Literal: &mathExprLiteral{
					Path: &Path{
						Fields: []Field{
							{
								Name: "attributes",
								Keys: []Key{
									{
										String: ottltest.Strp("foo"),
									},
									{
										String: ottltest.Strp("bar"),
									},
								},
							},
						},
					},
				},
			},
			want: "pass",
		},
		{
			name: "function call",
			val: value{
				Literal: &mathExprLiteral{
					Converter: &converter{
						Function: "Hello",
					},
				},
			},
			want: "world",
		},
		{
			name: "function call nested pcommon map",
			val: value{
				Literal: &mathExprLiteral{
					Converter: &converter{
						Function: "PMap",
						Keys: []Key{
							{
								String: ottltest.Strp("foo"),
							},
							{
								String: ottltest.Strp("bar"),
							},
						},
					},
				},
			},
			want: "pass",
		},
		{
			name: "function call nested map",
			val: value{
				Literal: &mathExprLiteral{
					Converter: &converter{
						Function: "Map",
						Keys: []Key{
							{
								String: ottltest.Strp("foo"),
							},
							{
								String: ottltest.Strp("bar"),
							},
						},
					},
				},
			},
			want: "pass",
		},
		{
			name: "function call pcommon slice",
			val: value{
				Literal: &mathExprLiteral{
					Converter: &converter{
						Function: "PSlice",
						Keys: []Key{
							{
								Int: ottltest.Intp(0),
							},
							{
								Int: ottltest.Intp(0),
							},
						},
					},
				},
			},
			want: "pass",
		},
		{
			name: "function call nested slice",
			val: value{
				Literal: &mathExprLiteral{
					Converter: &converter{
						Function: "Slice",
						Keys: []Key{
							{
								Int: ottltest.Intp(0),
							},
							{
								Int: ottltest.Intp(0),
							},
						},
					},
				},
			},
			want: "pass",
		},
		{
			name: "enum",
			val: value{
				Enum: (*EnumSymbol)(ottltest.Strp("TEST_ENUM_ONE")),
			},
			want: int64(1),
		},
		{
			name: "empty list",
			val: value{
				List: &list{
					Values: []value{},
				},
			},
			want: []any{},
		},
		{
			name: "string list",
			val: value{
				List: &list{
					Values: []value{
						{
							String: ottltest.Strp("test0"),
						},
						{
							String: ottltest.Strp("test1"),
						},
					},
				},
			},
			want: []any{"test0", "test1"},
		},
		{
			name: "int list",
			val: value{
				List: &list{
					Values: []value{
						{
							Literal: &mathExprLiteral{
								Int: ottltest.Intp(1),
							},
						},
						{
							Literal: &mathExprLiteral{
								Int: ottltest.Intp(2),
							},
						},
					},
				},
			},
			want: []any{int64(1), int64(2)},
		},
		{
			name: "float list",
			val: value{
				List: &list{
					Values: []value{
						{
							Literal: &mathExprLiteral{
								Float: ottltest.Floatp(1.2),
							},
						},
						{
							Literal: &mathExprLiteral{
								Float: ottltest.Floatp(2.4),
							},
						},
					},
				},
			},
			want: []any{1.2, 2.4},
		},
		{
			name: "bool list",
			val: value{
				List: &list{
					Values: []value{
						{
							Bool: (*boolean)(ottltest.Boolp(true)),
						},
						{
							Bool: (*boolean)(ottltest.Boolp(false)),
						},
					},
				},
			},
			want: []any{true, false},
		},
		{
			name: "byte slice list",
			val: value{
				List: &list{
					Values: []value{
						{
							Bytes: (*byteSlice)(&[]byte{1, 2, 3, 4, 5, 6, 7, 8}),
						},
						{
							Bytes: (*byteSlice)(&[]byte{9, 8, 7, 6, 5, 4, 3, 2}),
						},
					},
				},
			},
			want: []any{[]byte{1, 2, 3, 4, 5, 6, 7, 8}, []byte{9, 8, 7, 6, 5, 4, 3, 2}},
		},
		{
			name: "path expression",
			val: value{
				List: &list{
					Values: []value{
						{
							Literal: &mathExprLiteral{
								Path: &Path{
									Fields: []Field{
										{
											Name: "name",
										},
									},
								},
							},
						},
					},
				},
			},
			ctx:  "bear",
			want: []any{"bear"},
		},
		{
			name: "function call",
			val: value{
				List: &list{
					Values: []value{
						{
							Literal: &mathExprLiteral{
								Converter: &converter{
									Function: "Hello",
								},
							},
						},
					},
				},
			},
			want: []any{"world"},
		},
		{
			name: "nil slice",
			val: value{
				List: &list{
					Values: []value{
						{
							IsNil: (*isNil)(ottltest.Boolp(true)),
						},
						{
							IsNil: (*isNil)(ottltest.Boolp(true)),
						},
					},
				},
			},
			want: []any{nil, nil},
		},
		{
			name: "heterogeneous slice",
			val: value{
				List: &list{
					Values: []value{
						{
							String: ottltest.Strp("test0"),
						},
						{
							Literal: &mathExprLiteral{
								Int: ottltest.Intp(1),
							},
						},
					},
				},
			},
			want: []any{"test0", int64(1)},
		},
	}

	functions := CreateFactoryMap(
		createFactory("Hello", &struct{}{}, hello),
		createFactory("PMap", &struct{}{}, pmap),
		createFactory("Map", &struct{}{}, basicMap),
		createFactory("PSlice", &struct{}{}, pslice),
		createFactory("Slice", &struct{}{}, basicSlice),
	)

	p, _ := NewParser[any](
		functions,
		testParsePath,
		componenttest.NewNopTelemetrySettings(),
		WithEnumParser[any](testParseEnum),
	)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader, err := p.newGetter(tt.val)
			assert.NoError(t, err)

			tCtx := tt.want

			if tt.ctx != nil {
				tCtx = tt.ctx
			}

			val, err := reader.Get(context.Background(), tCtx)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, val)
		})
	}

	t.Run("empty value", func(t *testing.T) {
		_, err := p.newGetter(value{})
		assert.Error(t, err)
	})
}
func Test_exprGetter_Get_Invalid(t *testing.T) {
	tests := []struct {
		name string
		val  value
		err  error
	}{
		{
			name: "key not in pcommon map",
			val: value{
				Literal: &mathExprLiteral{
					Converter: &converter{
						Function: "PMap",
						Keys: []Key{
							{
								String: ottltest.Strp("unknown key"),
							},
						},
					},
				},
			},
			err: fmt.Errorf("key not found in map"),
		},
		{
			name: "key not in map",
			val: value{
				Literal: &mathExprLiteral{
					Converter: &converter{
						Function: "Map",
						Keys: []Key{
							{
								String: ottltest.Strp("unknown key"),
							},
						},
					},
				},
			},
			err: fmt.Errorf("key not found in map"),
		},
		{
			name: "index too large for pcommon slice",
			val: value{
				Literal: &mathExprLiteral{
					Converter: &converter{
						Function: "PSlice",
						Keys: []Key{
							{
								Int: ottltest.Intp(100),
							},
						},
					},
				},
			},
			err: fmt.Errorf("index 100 out of bounds"),
		},
		{
			name: "negative for pcommon slice",
			val: value{
				Literal: &mathExprLiteral{
					Converter: &converter{
						Function: "PSlice",
						Keys: []Key{
							{
								Int: ottltest.Intp(-1),
							},
						},
					},
				},
			},
			err: fmt.Errorf("index -1 out of bounds"),
		},
		{
			name: "index too large for Go slice",
			val: value{
				Literal: &mathExprLiteral{
					Converter: &converter{
						Function: "Slice",
						Keys: []Key{
							{
								Int: ottltest.Intp(100),
							},
						},
					},
				},
			},
			err: fmt.Errorf("index 100 out of bounds"),
		},
		{
			name: "negative for Go slice",
			val: value{
				Literal: &mathExprLiteral{
					Converter: &converter{
						Function: "Slice",
						Keys: []Key{
							{
								Int: ottltest.Intp(-1),
							},
						},
					},
				},
			},
			err: fmt.Errorf("index -1 out of bounds"),
		},
		{
			name: "invalid int indexing type",
			val: value{
				Literal: &mathExprLiteral{
					Converter: &converter{
						Function: "Hello",
						Keys: []Key{
							{
								Int: ottltest.Intp(-1),
							},
						},
					},
				},
			},
			err: fmt.Errorf("type, string, does not support int indexing"),
		},
		{
			name: "invalid string indexing type",
			val: value{
				Literal: &mathExprLiteral{
					Converter: &converter{
						Function: "Hello",
						Keys: []Key{
							{
								String: ottltest.Strp("test"),
							},
						},
					},
				},
			},
			err: fmt.Errorf("type, string, does not support string indexing"),
		},
	}

	functions := CreateFactoryMap(
		createFactory("Hello", &struct{}{}, hello),
		createFactory("PMap", &struct{}{}, pmap),
		createFactory("Map", &struct{}{}, basicMap),
		createFactory("PSlice", &struct{}{}, pslice),
		createFactory("Slice", &struct{}{}, basicSlice),
	)

	p, _ := NewParser[any](
		functions,
		testParsePath,
		componenttest.NewNopTelemetrySettings(),
		WithEnumParser[any](testParseEnum),
	)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader, err := p.newGetter(tt.val)
			assert.NoError(t, err)
			_, err = reader.Get(context.Background(), nil)
			assert.Equal(t, tt.err, err)
		})
	}
}

func Test_StandardStringGetter(t *testing.T) {
	tests := []struct {
		name             string
		getter           StandardStringGetter[interface{}]
		want             interface{}
		valid            bool
		expectedErrorMsg string
	}{
		{
			name: "string type",
			getter: StandardStringGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "str", nil
				},
			},
			want:  "str",
			valid: true,
		},
		{
			name: "ValueTypeString type",
			getter: StandardStringGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return pcommon.NewValueStr("str"), nil
				},
			},
			want:  "str",
			valid: true,
		},
		{
			name: "Incorrect type",
			getter: StandardStringGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return true, nil
				},
			},
			valid:            false,
			expectedErrorMsg: "expected string but got bool",
		},
		{
			name: "nil",
			getter: StandardStringGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return nil, nil
				},
			},
			valid:            false,
			expectedErrorMsg: "expected string but got nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, err := tt.getter.Get(context.Background(), nil)
			if tt.valid {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, val)
			} else {
				assert.IsType(t, TypeError(""), err)
				assert.EqualError(t, err, tt.expectedErrorMsg)
			}
		})
	}
}

func Test_FunctionGetter(t *testing.T) {
	functions := CreateFactoryMap(
		createFactory[any](
			"SHA256",
			&stringGetterArguments{},
			functionWithStringGetter,
		),
		createFactory[any](
			"test_arg_mismatch",
			&multipleArgsArguments{},
			functionWithStringGetter,
		),
		createFactory[any](
			"no_struct_tag",
			&noStructTagFunctionArguments{},
			functionWithStringGetter,
		),
		NewFactory(
			"cannot_create_function",
			&stringGetterArguments{},
			func(FunctionContext, Arguments) (ExprFunc[any], error) {
				return functionWithErr()
			},
		),
	)
	type EditorArguments struct {
		Replacement StringGetter[any]
		Function    FunctionGetter[any]
	}
	type FuncArgs struct {
		Input StringGetter[any] `ottlarg:"0"`
	}
	tests := []struct {
		name             string
		getter           StringGetter[any]
		function         FunctionGetter[any]
		want             interface{}
		valid            bool
		expectedErrorMsg string
	}{
		{
			name: "function getter",
			getter: StandardStringGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "str", nil
				},
			},
			function: StandardFunctionGetter[any]{fCtx: FunctionContext{Set: componenttest.NewNopTelemetrySettings()}, fact: functions["SHA256"]},
			want:     "anything",
			valid:    true,
		},
		{
			name: "function getter nil",
			getter: StandardStringGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return nil, nil
				},
			},
			function:         StandardFunctionGetter[any]{fCtx: FunctionContext{Set: componenttest.NewNopTelemetrySettings()}, fact: functions["SHA250"]},
			want:             "anything",
			valid:            false,
			expectedErrorMsg: "undefined function",
		},
		{
			name: "function arg mismatch",
			getter: StandardStringGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return nil, nil
				},
			},
			function:         StandardFunctionGetter[any]{fCtx: FunctionContext{Set: componenttest.NewNopTelemetrySettings()}, fact: functions["test_arg_mismatch"]},
			want:             "anything",
			valid:            false,
			expectedErrorMsg: "incorrect number of arguments. Expected: 4 Received: 1",
		},
		{
			name: "Invalid Arguments struct tag",
			getter: StandardStringGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return nil, nil
				},
			},
			function:         StandardFunctionGetter[any]{fCtx: FunctionContext{Set: componenttest.NewNopTelemetrySettings()}, fact: functions["no_struct_tag"]},
			want:             "anything",
			valid:            false,
			expectedErrorMsg: "no `ottlarg` struct tag on Arguments field \"StringArg\"",
		},
		{
			name: "Cannot create function",
			getter: StandardStringGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return nil, nil
				},
			},
			function:         StandardFunctionGetter[any]{fCtx: FunctionContext{Set: componenttest.NewNopTelemetrySettings()}, fact: functions["cannot_create_function"]},
			want:             "anything",
			valid:            false,
			expectedErrorMsg: "couldn't create function: error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			editorArgs := EditorArguments{
				Replacement: tt.getter,
				Function:    tt.function,
			}
			fn, err := editorArgs.Function.Get(&FuncArgs{Input: editorArgs.Replacement})
			if tt.valid {
				var result interface{}
				result, err = fn.Eval(context.Background(), nil)
				assert.NoError(t, err)
				assert.Equal(t, tt.want, result.(string))
			} else {
				assert.EqualError(t, err, tt.expectedErrorMsg)
			}
		})
	}
}

// nolint:errorlint
func Test_StandardStringGetter_WrappedError(t *testing.T) {
	getter := StandardStringGetter[interface{}]{
		Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
			return nil, TypeError("")
		},
	}
	_, err := getter.Get(context.Background(), nil)
	assert.Error(t, err)
	_, ok := err.(TypeError)
	assert.False(t, ok)
}

func Test_StandardStringLikeGetter(t *testing.T) {
	tests := []struct {
		name             string
		getter           StringLikeGetter[interface{}]
		want             interface{}
		valid            bool
		expectedErrorMsg string
	}{
		{
			name: "string type",
			getter: StandardStringLikeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "str", nil
				},
			},
			want:  "str",
			valid: true,
		},
		{
			name: "bool type",
			getter: StandardStringLikeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return true, nil
				},
			},
			want:  "true",
			valid: true,
		},
		{
			name: "int64 type",
			getter: StandardStringLikeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return int64(1), nil
				},
			},
			want:  "1",
			valid: true,
		},
		{
			name: "float64 type",
			getter: StandardStringLikeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return 1.1, nil
				},
			},
			want:  "1.1",
			valid: true,
		},
		{
			name: "byte[] type",
			getter: StandardStringLikeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return []byte{0}, nil
				},
			},
			want:  "00",
			valid: true,
		},
		{
			name: "pcommon.map type",
			getter: StandardStringLikeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					m := pcommon.NewMap()
					m.PutStr("test", "passed")
					return m, nil
				},
			},
			want:  `{"test":"passed"}`,
			valid: true,
		},
		{
			name: "pcommon.slice type",
			getter: StandardStringLikeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					s := pcommon.NewSlice()
					v := s.AppendEmpty()
					v.SetStr("test")
					return s, nil
				},
			},
			want:  `["test"]`,
			valid: true,
		},
		{
			name: "pcommon.value type",
			getter: StandardStringLikeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					v := pcommon.NewValueInt(int64(100))
					return v, nil
				},
			},
			want:  "100",
			valid: true,
		},
		{
			name: "nil",
			getter: StandardStringLikeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return nil, nil
				},
			},
			want:  nil,
			valid: true,
		},
		{
			name: "invalid type",
			getter: StandardStringLikeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return make(chan int), nil
				},
			},
			valid:            false,
			expectedErrorMsg: "unsupported type: chan int",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, err := tt.getter.Get(context.Background(), nil)
			if tt.valid {
				assert.NoError(t, err)
				if tt.want == nil {
					assert.Nil(t, val)
				} else {
					assert.Equal(t, tt.want, *val)
				}
			} else {
				assert.IsType(t, TypeError(""), err)
				assert.EqualError(t, err, tt.expectedErrorMsg)
			}
		})
	}
}

// nolint:errorlint
func Test_StandardStringLikeGetter_WrappedError(t *testing.T) {
	getter := StandardStringLikeGetter[interface{}]{
		Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
			return nil, TypeError("")
		},
	}
	_, err := getter.Get(context.Background(), nil)
	assert.Error(t, err)
	_, ok := err.(TypeError)
	assert.False(t, ok)
}

func Test_StandardFloatGetter(t *testing.T) {
	tests := []struct {
		name             string
		getter           StandardFloatGetter[interface{}]
		want             interface{}
		valid            bool
		expectedErrorMsg string
	}{
		{
			name: "float64 type",
			getter: StandardFloatGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return 1.1, nil
				},
			},
			want:  1.1,
			valid: true,
		},
		{
			name: "ValueTypeFloat type",
			getter: StandardFloatGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return pcommon.NewValueDouble(1.1), nil
				},
			},
			want:  1.1,
			valid: true,
		},
		{
			name: "Incorrect type",
			getter: StandardFloatGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return true, nil
				},
			},
			valid:            false,
			expectedErrorMsg: "expected float64 but got bool",
		},
		{
			name: "nil",
			getter: StandardFloatGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return nil, nil
				},
			},
			valid:            false,
			expectedErrorMsg: "expected float64 but got nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, err := tt.getter.Get(context.Background(), nil)
			if tt.valid {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, val)
			} else {
				assert.IsType(t, TypeError(""), err)
				assert.EqualError(t, err, tt.expectedErrorMsg)
			}
		})
	}
}

// nolint:errorlint
func Test_StandardFloatGetter_WrappedError(t *testing.T) {
	getter := StandardFloatGetter[interface{}]{
		Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
			return nil, TypeError("")
		},
	}
	_, err := getter.Get(context.Background(), nil)
	assert.Error(t, err)
	_, ok := err.(TypeError)
	assert.False(t, ok)
}

func Test_StandardFloatLikeGetter(t *testing.T) {
	tests := []struct {
		name             string
		getter           FloatLikeGetter[interface{}]
		want             interface{}
		valid            bool
		expectedErrorMsg string
	}{
		{
			name: "string type",
			getter: StandardFloatLikeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "1.0", nil
				},
			},
			want:  1.0,
			valid: true,
		},
		{
			name: "int64 type",
			getter: StandardFloatLikeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return int64(1), nil
				},
			},
			want:  float64(1),
			valid: true,
		},
		{
			name: "float64 type",
			getter: StandardFloatLikeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return 1.1, nil
				},
			},
			want:  1.1,
			valid: true,
		},
		{
			name: "float64 bool true",
			getter: StandardFloatLikeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return true, nil
				},
			},
			want:  float64(1),
			valid: true,
		},
		{
			name: "float64 bool false",
			getter: StandardFloatLikeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return false, nil
				},
			},
			want:  float64(0),
			valid: true,
		},
		{
			name: "pcommon.value type int",
			getter: StandardFloatLikeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					v := pcommon.NewValueInt(int64(100))
					return v, nil
				},
			},
			want:  float64(100),
			valid: true,
		},
		{
			name: "pcommon.value type float",
			getter: StandardFloatLikeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					v := pcommon.NewValueDouble(float64(1.1))
					return v, nil
				},
			},
			want:  1.1,
			valid: true,
		},
		{
			name: "pcommon.value type string",
			getter: StandardFloatLikeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					v := pcommon.NewValueStr("1.1")
					return v, nil
				},
			},
			want:  1.1,
			valid: true,
		},
		{
			name: "pcommon.value type bool true",
			getter: StandardFloatLikeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					v := pcommon.NewValueBool(true)
					return v, nil
				},
			},
			want:  float64(1),
			valid: true,
		},
		{
			name: "pcommon.value type bool false",
			getter: StandardFloatLikeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					v := pcommon.NewValueBool(false)
					return v, nil
				},
			},
			want:  float64(0),
			valid: true,
		},
		{
			name: "nil",
			getter: StandardFloatLikeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return nil, nil
				},
			},
			want:  nil,
			valid: true,
		},
		{
			name: "invalid type",
			getter: StandardFloatLikeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return []byte{}, nil
				},
			},
			valid:            false,
			expectedErrorMsg: "unsupported type: []uint8",
		},
		{
			name: "invalid pcommon.Value type",
			getter: StandardFloatLikeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					v := pcommon.NewValueMap()
					return v, nil
				},
			},
			valid:            false,
			expectedErrorMsg: "unsupported value type: Map",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, err := tt.getter.Get(context.Background(), nil)
			if tt.valid {
				assert.NoError(t, err)
				if tt.want == nil {
					assert.Nil(t, val)
				} else {
					assert.Equal(t, tt.want, *val)
				}
			} else {
				assert.IsType(t, TypeError(""), err)
				assert.EqualError(t, err, tt.expectedErrorMsg)
			}
		})
	}
}

// nolint:errorlint
func Test_StandardFloatLikeGetter_WrappedError(t *testing.T) {
	getter := StandardFloatLikeGetter[interface{}]{
		Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
			return nil, TypeError("")
		},
	}
	_, err := getter.Get(context.Background(), nil)
	assert.Error(t, err)
	_, ok := err.(TypeError)
	assert.False(t, ok)
}

func Test_StandardIntGetter(t *testing.T) {
	tests := []struct {
		name             string
		getter           StandardIntGetter[interface{}]
		want             interface{}
		valid            bool
		expectedErrorMsg string
	}{
		{
			name: "int64 type",
			getter: StandardIntGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return int64(1), nil
				},
			},
			want:  int64(1),
			valid: true,
		},
		{
			name: "ValueTypeInt type",
			getter: StandardIntGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return pcommon.NewValueInt(1), nil
				},
			},
			want:  int64(1),
			valid: true,
		},
		{
			name: "Incorrect type",
			getter: StandardIntGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return true, nil
				},
			},
			valid:            false,
			expectedErrorMsg: "expected int64 but got bool",
		},
		{
			name: "nil",
			getter: StandardIntGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return nil, nil
				},
			},
			valid:            false,
			expectedErrorMsg: "expected int64 but got nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, err := tt.getter.Get(context.Background(), nil)
			if tt.valid {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, val)
			} else {
				assert.IsType(t, TypeError(""), err)
				assert.EqualError(t, err, tt.expectedErrorMsg)
			}
		})
	}
}

// nolint:errorlint
func Test_StandardIntGetter_WrappedError(t *testing.T) {
	getter := StandardIntGetter[interface{}]{
		Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
			return nil, TypeError("")
		},
	}
	_, err := getter.Get(context.Background(), nil)
	assert.Error(t, err)
	_, ok := err.(TypeError)
	assert.False(t, ok)
}

func Test_StandardIntLikeGetter(t *testing.T) {
	tests := []struct {
		name             string
		getter           IntLikeGetter[interface{}]
		want             interface{}
		valid            bool
		expectedErrorMsg string
	}{
		{
			name: "string type",
			getter: StandardIntLikeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "1", nil
				},
			},
			want:  int64(1),
			valid: true,
		},
		{
			name: "int64 type",
			getter: StandardIntLikeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return int64(1), nil
				},
			},
			want:  int64(1),
			valid: true,
		},
		{
			name: "float64 type",
			getter: StandardIntLikeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return 1.1, nil
				},
			},
			want:  int64(1),
			valid: true,
		},
		{
			name: "primitive bool true",
			getter: StandardIntLikeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return true, nil
				},
			},
			want:  int64(1),
			valid: true,
		},
		{
			name: "primitive bool false",
			getter: StandardIntLikeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return false, nil
				},
			},
			want:  int64(0),
			valid: true,
		},
		{
			name: "pcommon.value type int",
			getter: StandardIntLikeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					v := pcommon.NewValueInt(int64(100))
					return v, nil
				},
			},
			want:  int64(100),
			valid: true,
		},
		{
			name: "pcommon.value type float",
			getter: StandardIntLikeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					v := pcommon.NewValueDouble(float64(1.9))
					return v, nil
				},
			},
			want:  int64(1),
			valid: true,
		},
		{
			name: "pcommon.value type string",
			getter: StandardIntLikeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					v := pcommon.NewValueStr("1")
					return v, nil
				},
			},
			want:  int64(1),
			valid: true,
		},
		{
			name: "pcommon.value type bool true",
			getter: StandardIntLikeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					v := pcommon.NewValueBool(true)
					return v, nil
				},
			},
			want:  int64(1),
			valid: true,
		},
		{
			name: "pcommon.value type bool false",
			getter: StandardIntLikeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					v := pcommon.NewValueBool(false)
					return v, nil
				},
			},
			want:  int64(0),
			valid: true,
		},
		{
			name: "nil",
			getter: StandardIntLikeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return nil, nil
				},
			},
			want:  nil,
			valid: true,
		},
		{
			name: "invalid type",
			getter: StandardIntLikeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return []byte{}, nil
				},
			},
			valid:            false,
			expectedErrorMsg: "unsupported type: []uint8",
		},
		{
			name: "invalid pcommon.Value type",
			getter: StandardIntLikeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					v := pcommon.NewValueMap()
					return v, nil
				},
			},
			valid:            false,
			expectedErrorMsg: "unsupported value type: Map",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, err := tt.getter.Get(context.Background(), nil)
			if tt.valid {
				assert.NoError(t, err)
				if tt.want == nil {
					assert.Nil(t, val)
				} else {
					assert.Equal(t, tt.want, *val)
				}
			} else {
				assert.IsType(t, TypeError(""), err)
				assert.EqualError(t, err, tt.expectedErrorMsg)
			}
		})
	}
}

// nolint:errorlint
func Test_StandardIntLikeGetter_WrappedError(t *testing.T) {
	getter := StandardIntLikeGetter[interface{}]{
		Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
			return nil, TypeError("")
		},
	}
	_, err := getter.Get(context.Background(), nil)
	assert.Error(t, err)
	_, ok := err.(TypeError)
	assert.False(t, ok)
}

func Test_StandardPMapGetter(t *testing.T) {
	tests := []struct {
		name             string
		getter           StandardPMapGetter[interface{}]
		want             interface{}
		valid            bool
		expectedErrorMsg string
	}{
		{
			name: "pcommon.map type",
			getter: StandardPMapGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return pcommon.NewMap(), nil
				},
			},
			want:  pcommon.NewMap(),
			valid: true,
		},
		{
			name: "map[string]any type",
			getter: StandardPMapGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return make(map[string]any), nil
				},
			},
			want:  pcommon.NewMap(),
			valid: true,
		},
		{
			name: "ValueTypeMap type",
			getter: StandardPMapGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return pcommon.NewValueMap(), nil
				},
			},
			want:  pcommon.NewMap(),
			valid: true,
		},
		{
			name: "Incorrect type",
			getter: StandardPMapGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return true, nil
				},
			},
			valid:            false,
			expectedErrorMsg: "expected pcommon.Map but got bool",
		},
		{
			name: "nil",
			getter: StandardPMapGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return nil, nil
				},
			},
			valid:            false,
			expectedErrorMsg: "expected pcommon.Map but got nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, err := tt.getter.Get(context.Background(), nil)
			if tt.valid {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, val)
			} else {
				assert.IsType(t, TypeError(""), err)
				assert.EqualError(t, err, tt.expectedErrorMsg)
			}
		})
	}
}

// nolint:errorlint
func Test_StandardPMapGetter_WrappedError(t *testing.T) {
	getter := StandardPMapGetter[interface{}]{
		Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
			return nil, TypeError("")
		},
	}
	_, err := getter.Get(context.Background(), nil)
	assert.Error(t, err)
	_, ok := err.(TypeError)
	assert.False(t, ok)
}

func Test_StandardDurationGetter(t *testing.T) {
	oneHourOneMinuteOneSecond, err := time.ParseDuration("1h1m1s")
	require.NoError(t, err)

	oneHundredNsecs, err := time.ParseDuration("100ns")
	require.NoError(t, err)

	tenMilliseconds, err := time.ParseDuration("10ms66us7000ns")
	require.NoError(t, err)

	tests := []struct {
		name             string
		getter           StandardDurationGetter[interface{}]
		want             interface{}
		valid            bool
		expectedErrorMsg string
	}{
		{
			name: "complex duration",
			getter: StandardDurationGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return time.ParseDuration("1h1m1s")
				},
			},
			want:  oneHourOneMinuteOneSecond,
			valid: true,
		},
		{
			name: "simple duration",
			getter: StandardDurationGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return time.ParseDuration("100ns")
				},
			},
			want:  oneHundredNsecs,
			valid: true,
		},
		{
			name: "complex duation values less than 1 seconc",
			getter: StandardDurationGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return time.ParseDuration("10ms66us7000ns")
				},
			},
			want:  tenMilliseconds,
			valid: true,
		},
		{
			name: "invalid duration units",
			getter: StandardDurationGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return time.ParseDuration("70ps")
				},
			},
			valid:            false,
			expectedErrorMsg: "unknown unit",
		},
		{
			name: "wrong type - int",
			getter: StandardDurationGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return 1, nil
				},
			},
			valid:            false,
			expectedErrorMsg: "expected duration but got int",
		},
		{
			name: "nil",
			getter: StandardDurationGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return nil, nil
				},
			},
			valid:            false,
			expectedErrorMsg: "expected duration but got nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, err := tt.getter.Get(context.Background(), nil)
			if tt.valid {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, val)
			} else {
				assert.ErrorContains(t, err, tt.expectedErrorMsg)
			}
		})
	}
}

// nolint:errorlint
func Test_StandardDurationGetter_WrappedError(t *testing.T) {
	getter := StandardDurationGetter[interface{}]{
		Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
			return nil, TypeError("")
		},
	}
	_, err := getter.Get(context.Background(), nil)
	assert.Error(t, err)
	_, ok := err.(TypeError)
	assert.False(t, ok)
}

func Test_StandardTimeGetter(t *testing.T) {
	tests := []struct {
		name   string
		getter StandardTimeGetter[interface{}]
		want             string
		valid            bool
		expectedErrorMsg string
	}{
		{
			name: "2023 time",
			getter: StandardTimeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return time.Date(2023, 8, 17, 1, 1, 1, 1, time.UTC), nil
				},
			},
			want:  "2023-08-17T01:01:01.000000001Z",
			valid: true,
		},
		{
			name: "before 2000 time",
			getter: StandardTimeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return time.Date(1999, 12, 1, 10, 59, 58, 57, time.UTC), nil
				},
			},
			want:  "1999-12-01T10:59:58.000000057Z",
			valid: true,
		},
		{
			name: "wrong type - duration",
			getter: StandardTimeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return time.ParseDuration("70ns")
				},
			},
			valid:            false,
			expectedErrorMsg: "expected time but got time.Duration",
		},
		{
			name: "wrong type - bool",
			getter: StandardTimeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return true, nil
				},
			},
			valid:            false,
			expectedErrorMsg: "expected time but got bool",
		},
		{
			name: "nil",
			getter: StandardTimeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return nil, nil
				},
			},
			valid:            false,
			expectedErrorMsg: "expected time but got nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, err := tt.getter.Get(context.Background(), nil)
			if tt.valid {
				assert.NoError(t, err)
				want, err := time.Parse("2006-01-02T15:04:05.000000000Z", tt.want)
				assert.NoError(t, err)
				assert.Equal(t, want, val)
			} else {
				assert.ErrorContains(t, err, tt.expectedErrorMsg)
			}
		})
	}
}

// nolint:errorlint
func Test_StandardTimeGetter_WrappedError(t *testing.T) {
	getter := StandardTimeGetter[interface{}]{
		Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
			return nil, TypeError("")
		},
	}
	_, err := getter.Get(context.Background(), nil)
	assert.Error(t, err)
	_, ok := err.(TypeError)
	assert.False(t, ok)
}
