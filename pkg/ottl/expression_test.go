// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottl

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottltest"
)

func hello() (ExprFunc[any], error) {
	return func(_ context.Context, _ any) (any, error) {
		return "world", nil
	}, nil
}

func pmap() (ExprFunc[any], error) {
	return func(_ context.Context, _ any) (any, error) {
		m := pcommon.NewMap()
		m.PutEmptyMap("foo").PutStr("bar", "pass")
		return m, nil
	}, nil
}

func basicMap() (ExprFunc[any], error) {
	return func(_ context.Context, _ any) (any, error) {
		return map[string]any{
			"foo": map[string]any{
				"bar": "pass",
			},
		}, nil
	}, nil
}

func pslice() (ExprFunc[any], error) {
	return func(_ context.Context, _ any) (any, error) {
		s := pcommon.NewSlice()
		s.AppendEmpty().SetEmptySlice().AppendEmpty().SetStr("pass")
		return s, nil
	}, nil
}

func basicSlice() (ExprFunc[any], error) {
	return func(_ context.Context, _ any) (any, error) {
		return []any{
			[]any{
				"pass",
			},
		}, nil
	}, nil
}

func basicSliceString() (ExprFunc[any], error) {
	return func(_ context.Context, _ any) (any, error) {
		return []any{
			[]string{
				"pass",
			},
		}, nil
	}, nil
}

func basicSliceBool() (ExprFunc[any], error) {
	return func(_ context.Context, _ any) (any, error) {
		return []any{
			[]bool{
				true,
			},
		}, nil
	}, nil
}

func basicSliceInteger() (ExprFunc[any], error) {
	return func(_ context.Context, _ any) (any, error) {
		return []any{
			[]int64{
				1,
			},
		}, nil
	}, nil
}

func basicSliceFloat() (ExprFunc[any], error) {
	return func(_ context.Context, _ any) (any, error) {
		return []any{
			[]float64{
				1,
			},
		}, nil
	}, nil
}

func basicSliceByte() (ExprFunc[any], error) {
	return func(_ context.Context, _ any) (any, error) {
		return []any{
			[]byte{
				byte('p'),
			},
		}, nil
	}, nil
}

func Test_newGetter(t *testing.T) {
	tests := []struct {
		name string
		val  value
		ctx  any
		want any
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
					Path: &path{
						Fields: []field{
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
					Path: &path{
						Fields: []field{
							{
								Name: "attributes",
								Keys: []key{
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
						Keys: []key{
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
						Keys: []key{
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
						Keys: []key{
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
						Keys: []key{
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
			name: "function call nested SliceString",
			val: value{
				Literal: &mathExprLiteral{
					Converter: &converter{
						Function: "SliceString",
						Keys: []key{
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
			name: "function call nested SliceBool",
			val: value{
				Literal: &mathExprLiteral{
					Converter: &converter{
						Function: "SliceBool",
						Keys: []key{
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
			want: true,
		},
		{
			name: "function call nested SliceInteger",
			val: value{
				Literal: &mathExprLiteral{
					Converter: &converter{
						Function: "SliceInteger",
						Keys: []key{
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
			want: int64(1),
		},
		{
			name: "function call nested SliceFloat",
			val: value{
				Literal: &mathExprLiteral{
					Converter: &converter{
						Function: "SliceFloat",
						Keys: []key{
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
			want: 1.0,
		},
		{
			name: "function call nested SliceByte",
			val: value{
				Literal: &mathExprLiteral{
					Converter: &converter{
						Function: "SliceByte",
						Keys: []key{
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
			want: byte('p'),
		},
		{
			name: "enum",
			val: value{
				Enum: (*enumSymbol)(ottltest.Strp("TEST_ENUM_ONE")),
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
								Path: &path{
									Fields: []field{
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
		{
			name: "map",
			val: value{
				Map: &mapValue{
					Values: []mapItem{
						{
							Key:   ottltest.Strp("stringAttr"),
							Value: &value{String: ottltest.Strp("value")},
						},
						{
							Key: ottltest.Strp("intAttr"),
							Value: &value{
								Literal: &mathExprLiteral{
									Int: ottltest.Intp(3),
								},
							},
						},
						{
							Key: ottltest.Strp("floatAttr"),
							Value: &value{
								Literal: &mathExprLiteral{
									Float: ottltest.Floatp(2.5),
								},
							},
						},
						{
							Key:   ottltest.Strp("boolAttr"),
							Value: &value{Bool: (*boolean)(ottltest.Boolp(true))},
						},
						{
							Key:   ottltest.Strp("byteAttr"),
							Value: &value{Bytes: (*byteSlice)(&[]byte{1, 2, 3, 4, 5, 6, 7, 8})},
						},
						{
							Key:   ottltest.Strp("enumAttr"),
							Value: &value{Enum: (*enumSymbol)(ottltest.Strp("TEST_ENUM_ONE"))},
						},
						{
							Key: ottltest.Strp("pathAttr"),
							Value: &value{
								Literal: &mathExprLiteral{
									Path: &path{
										Fields: []field{
											{
												Name: "name",
											},
										},
									},
								},
							},
						},
						{
							Key: ottltest.Strp("mapAttr"),
							Value: &value{
								Map: &mapValue{
									Values: []mapItem{
										{
											Key: ottltest.Strp("foo"),
											Value: &value{
												Map: &mapValue{
													Values: []mapItem{
														{
															Key:   ottltest.Strp("test"),
															Value: &value{String: ottltest.Strp("value")},
														},
													},
												},
											},
										},
										{
											Key: ottltest.Strp("listAttr"),
											Value: &value{
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
														{
															Map: &mapValue{
																Values: []mapItem{
																	{
																		Key:   ottltest.Strp("stringAttr"),
																		Value: &value{String: ottltest.Strp("value")},
																	},
																},
															},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			ctx: "bear",
			want: map[string]any{
				"enumAttr": int64(1),
				"pathAttr": "bear",
				"mapAttr": map[string]any{
					"foo": map[string]any{
						"test": "value",
					},
					"listAttr": []any{"test0", int64(1), map[string]any{"stringAttr": "value"}},
				},
				"stringAttr": "value",
				"intAttr":    int64(3),
				"floatAttr":  2.5,
				"boolAttr":   true,
				"byteAttr":   []byte{1, 2, 3, 4, 5, 6, 7, 8},
			},
		},
	}

	functions := CreateFactoryMap(
		createFactory("Hello", &struct{}{}, hello),
		createFactory("PMap", &struct{}{}, pmap),
		createFactory("Map", &struct{}{}, basicMap),
		createFactory("PSlice", &struct{}{}, pslice),
		createFactory("Slice", &struct{}{}, basicSlice),
		createFactory("SliceString", &struct{}{}, basicSliceString),
		createFactory("SliceBool", &struct{}{}, basicSliceBool),
		createFactory("SliceInteger", &struct{}{}, basicSliceInteger),
		createFactory("SliceFloat", &struct{}{}, basicSliceFloat),
		createFactory("SliceByte", &struct{}{}, basicSliceByte),
	)

	p, _ := NewParser[any](
		functions,
		testParsePath[any],
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

			switch v := val.(type) {
			case pcommon.Map:
				// need to compare the raw map here as require.EqualValues can not seem to handle
				// the comparison of pcommon.Map
				assert.EqualValues(t, tt.want, v.AsRaw())
			default:
				assert.Equal(t, tt.want, v)
			}
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
						Keys: []key{
							{
								String: ottltest.Strp("unknown key"),
							},
						},
					},
				},
			},
			err: errors.New("key not found in map"),
		},
		{
			name: "key not in map",
			val: value{
				Literal: &mathExprLiteral{
					Converter: &converter{
						Function: "Map",
						Keys: []key{
							{
								String: ottltest.Strp("unknown key"),
							},
						},
					},
				},
			},
			err: errors.New("key not found in map"),
		},
		{
			name: "index too large for pcommon slice",
			val: value{
				Literal: &mathExprLiteral{
					Converter: &converter{
						Function: "PSlice",
						Keys: []key{
							{
								Int: ottltest.Intp(100),
							},
						},
					},
				},
			},
			err: errors.New("index 100 out of bounds"),
		},
		{
			name: "negative for pcommon slice",
			val: value{
				Literal: &mathExprLiteral{
					Converter: &converter{
						Function: "PSlice",
						Keys: []key{
							{
								Int: ottltest.Intp(-1),
							},
						},
					},
				},
			},
			err: errors.New("index -1 out of bounds"),
		},
		{
			name: "index too large for Go slice",
			val: value{
				Literal: &mathExprLiteral{
					Converter: &converter{
						Function: "Slice",
						Keys: []key{
							{
								Int: ottltest.Intp(100),
							},
						},
					},
				},
			},
			err: errors.New("index 100 out of bounds"),
		},
		{
			name: "negative for Go slice",
			val: value{
				Literal: &mathExprLiteral{
					Converter: &converter{
						Function: "Slice",
						Keys: []key{
							{
								Int: ottltest.Intp(-1),
							},
						},
					},
				},
			},
			err: errors.New("index -1 out of bounds"),
		},
		{
			name: "invalid int indexing type",
			val: value{
				Literal: &mathExprLiteral{
					Converter: &converter{
						Function: "Hello",
						Keys: []key{
							{
								Int: ottltest.Intp(-1),
							},
						},
					},
				},
			},
			err: errors.New("type, string, does not support int indexing"),
		},
		{
			name: "invalid string indexing type",
			val: value{
				Literal: &mathExprLiteral{
					Converter: &converter{
						Function: "Hello",
						Keys: []key{
							{
								String: ottltest.Strp("test"),
							},
						},
					},
				},
			},
			err: errors.New("type, string, does not support string indexing"),
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
		testParsePath[any],
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
		getter           StandardStringGetter[any]
		want             any
		valid            bool
		expectedErrorMsg string
	}{
		{
			name: "string type",
			getter: StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "str", nil
				},
			},
			want:  "str",
			valid: true,
		},
		{
			name: "ValueTypeString type",
			getter: StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return pcommon.NewValueStr("str"), nil
				},
			},
			want:  "str",
			valid: true,
		},
		{
			name: "Incorrect type",
			getter: StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return true, nil
				},
			},
			valid:            false,
			expectedErrorMsg: "expected string but got bool",
		},
		{
			name: "nil",
			getter: StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
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
		Input StringGetter[any]
	}
	tests := []struct {
		name             string
		getter           StringGetter[any]
		function         FunctionGetter[any]
		want             any
		valid            bool
		expectedErrorMsg string
	}{
		{
			name: "function getter",
			getter: StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "str", nil
				},
			},
			function: StandardFunctionGetter[any]{FCtx: FunctionContext{Set: componenttest.NewNopTelemetrySettings()}, Fact: functions["SHA256"]},
			want:     "anything",
			valid:    true,
		},
		{
			name: "function getter nil",
			getter: StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return nil, nil
				},
			},
			function:         StandardFunctionGetter[any]{FCtx: FunctionContext{Set: componenttest.NewNopTelemetrySettings()}, Fact: functions["SHA250"]},
			want:             "anything",
			valid:            false,
			expectedErrorMsg: "undefined function",
		},
		{
			name: "function arg mismatch",
			getter: StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return nil, nil
				},
			},
			function:         StandardFunctionGetter[any]{FCtx: FunctionContext{Set: componenttest.NewNopTelemetrySettings()}, Fact: functions["test_arg_mismatch"]},
			want:             "anything",
			valid:            false,
			expectedErrorMsg: "incorrect number of arguments. Expected: 4 Received: 1",
		},
		{
			name: "Cannot create function",
			getter: StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return nil, nil
				},
			},
			function:         StandardFunctionGetter[any]{FCtx: FunctionContext{Set: componenttest.NewNopTelemetrySettings()}, Fact: functions["cannot_create_function"]},
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
				var result any
				result, err = fn.Eval(context.Background(), nil)
				assert.NoError(t, err)
				assert.Equal(t, tt.want, result.(string))
			} else {
				assert.EqualError(t, err, tt.expectedErrorMsg)
			}
		})
	}
}

//nolint:errorlint
func Test_StandardStringGetter_WrappedError(t *testing.T) {
	getter := StandardStringGetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
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
		getter           StringLikeGetter[any]
		want             any
		valid            bool
		expectedErrorMsg string
	}{
		{
			name: "string type",
			getter: StandardStringLikeGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "str", nil
				},
			},
			want:  "str",
			valid: true,
		},
		{
			name: "bool type",
			getter: StandardStringLikeGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return true, nil
				},
			},
			want:  "true",
			valid: true,
		},
		{
			name: "int64 type",
			getter: StandardStringLikeGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return int64(1), nil
				},
			},
			want:  "1",
			valid: true,
		},
		{
			name: "float64 type",
			getter: StandardStringLikeGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return 1.1, nil
				},
			},
			want:  "1.1",
			valid: true,
		},
		{
			name: "byte[] type",
			getter: StandardStringLikeGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return []byte{0}, nil
				},
			},
			want:  "00",
			valid: true,
		},
		{
			name: "pcommon.map type",
			getter: StandardStringLikeGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
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
			getter: StandardStringLikeGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
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
			getter: StandardStringLikeGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					v := pcommon.NewValueInt(int64(100))
					return v, nil
				},
			},
			want:  "100",
			valid: true,
		},
		{
			name: "nil",
			getter: StandardStringLikeGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return nil, nil
				},
			},
			want:  nil,
			valid: true,
		},
		{
			name: "invalid type",
			getter: StandardStringLikeGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
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

//nolint:errorlint
func Test_StandardStringLikeGetter_WrappedError(t *testing.T) {
	getter := StandardStringLikeGetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
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
		getter           StandardFloatGetter[any]
		want             any
		valid            bool
		expectedErrorMsg string
	}{
		{
			name: "float64 type",
			getter: StandardFloatGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return 1.1, nil
				},
			},
			want:  1.1,
			valid: true,
		},
		{
			name: "ValueTypeFloat type",
			getter: StandardFloatGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return pcommon.NewValueDouble(1.1), nil
				},
			},
			want:  1.1,
			valid: true,
		},
		{
			name: "Incorrect type",
			getter: StandardFloatGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return true, nil
				},
			},
			valid:            false,
			expectedErrorMsg: "expected float64 but got bool",
		},
		{
			name: "nil",
			getter: StandardFloatGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
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

//nolint:errorlint
func Test_StandardFloatGetter_WrappedError(t *testing.T) {
	getter := StandardFloatGetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
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
		getter           FloatLikeGetter[any]
		want             any
		valid            bool
		expectedErrorMsg string
	}{
		{
			name: "string type",
			getter: StandardFloatLikeGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "1.0", nil
				},
			},
			want:  1.0,
			valid: true,
		},
		{
			name: "int64 type",
			getter: StandardFloatLikeGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return int64(1), nil
				},
			},
			want:  float64(1),
			valid: true,
		},
		{
			name: "float64 type",
			getter: StandardFloatLikeGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return 1.1, nil
				},
			},
			want:  1.1,
			valid: true,
		},
		{
			name: "float64 bool true",
			getter: StandardFloatLikeGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return true, nil
				},
			},
			want:  float64(1),
			valid: true,
		},
		{
			name: "float64 bool false",
			getter: StandardFloatLikeGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return false, nil
				},
			},
			want:  float64(0),
			valid: true,
		},
		{
			name: "pcommon.value type int",
			getter: StandardFloatLikeGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					v := pcommon.NewValueInt(int64(100))
					return v, nil
				},
			},
			want:  float64(100),
			valid: true,
		},
		{
			name: "pcommon.value type float",
			getter: StandardFloatLikeGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					v := pcommon.NewValueDouble(float64(1.1))
					return v, nil
				},
			},
			want:  1.1,
			valid: true,
		},
		{
			name: "pcommon.value type string",
			getter: StandardFloatLikeGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					v := pcommon.NewValueStr("1.1")
					return v, nil
				},
			},
			want:  1.1,
			valid: true,
		},
		{
			name: "pcommon.value type bool true",
			getter: StandardFloatLikeGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					v := pcommon.NewValueBool(true)
					return v, nil
				},
			},
			want:  float64(1),
			valid: true,
		},
		{
			name: "pcommon.value type bool false",
			getter: StandardFloatLikeGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					v := pcommon.NewValueBool(false)
					return v, nil
				},
			},
			want:  float64(0),
			valid: true,
		},
		{
			name: "nil",
			getter: StandardFloatLikeGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return nil, nil
				},
			},
			want:  nil,
			valid: true,
		},
		{
			name: "invalid type",
			getter: StandardFloatLikeGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return []byte{}, nil
				},
			},
			valid:            false,
			expectedErrorMsg: "unsupported type: []uint8",
		},
		{
			name: "invalid pcommon.Value type",
			getter: StandardFloatLikeGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
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

//nolint:errorlint
func Test_StandardFloatLikeGetter_WrappedError(t *testing.T) {
	getter := StandardFloatLikeGetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
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
		getter           StandardIntGetter[any]
		want             any
		valid            bool
		expectedErrorMsg string
	}{
		{
			name: "int64 type",
			getter: StandardIntGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return int64(1), nil
				},
			},
			want:  int64(1),
			valid: true,
		},
		{
			name: "ValueTypeInt type",
			getter: StandardIntGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return pcommon.NewValueInt(1), nil
				},
			},
			want:  int64(1),
			valid: true,
		},
		{
			name: "Incorrect type",
			getter: StandardIntGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return true, nil
				},
			},
			valid:            false,
			expectedErrorMsg: "expected int64 but got bool",
		},
		{
			name: "nil",
			getter: StandardIntGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
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

//nolint:errorlint
func Test_StandardIntGetter_WrappedError(t *testing.T) {
	getter := StandardIntGetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
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
		getter           IntLikeGetter[any]
		want             any
		valid            bool
		expectedErrorMsg string
	}{
		{
			name: "string type",
			getter: StandardIntLikeGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "1", nil
				},
			},
			want:  int64(1),
			valid: true,
		},
		{
			name: "int64 type",
			getter: StandardIntLikeGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return int64(1), nil
				},
			},
			want:  int64(1),
			valid: true,
		},
		{
			name: "float64 type",
			getter: StandardIntLikeGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return 1.1, nil
				},
			},
			want:  int64(1),
			valid: true,
		},
		{
			name: "primitive bool true",
			getter: StandardIntLikeGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return true, nil
				},
			},
			want:  int64(1),
			valid: true,
		},
		{
			name: "primitive bool false",
			getter: StandardIntLikeGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return false, nil
				},
			},
			want:  int64(0),
			valid: true,
		},
		{
			name: "pcommon.value type int",
			getter: StandardIntLikeGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					v := pcommon.NewValueInt(int64(100))
					return v, nil
				},
			},
			want:  int64(100),
			valid: true,
		},
		{
			name: "pcommon.value type float",
			getter: StandardIntLikeGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					v := pcommon.NewValueDouble(float64(1.9))
					return v, nil
				},
			},
			want:  int64(1),
			valid: true,
		},
		{
			name: "pcommon.value type string",
			getter: StandardIntLikeGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					v := pcommon.NewValueStr("1")
					return v, nil
				},
			},
			want:  int64(1),
			valid: true,
		},
		{
			name: "pcommon.value type bool true",
			getter: StandardIntLikeGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					v := pcommon.NewValueBool(true)
					return v, nil
				},
			},
			want:  int64(1),
			valid: true,
		},
		{
			name: "pcommon.value type bool false",
			getter: StandardIntLikeGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					v := pcommon.NewValueBool(false)
					return v, nil
				},
			},
			want:  int64(0),
			valid: true,
		},
		{
			name: "nil",
			getter: StandardIntLikeGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return nil, nil
				},
			},
			want:  nil,
			valid: true,
		},
		{
			name: "invalid type",
			getter: StandardIntLikeGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return []byte{}, nil
				},
			},
			valid:            false,
			expectedErrorMsg: "unsupported type: []uint8",
		},
		{
			name: "invalid pcommon.Value type",
			getter: StandardIntLikeGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
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

//nolint:errorlint
func Test_StandardIntLikeGetter_WrappedError(t *testing.T) {
	getter := StandardIntLikeGetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
			return nil, TypeError("")
		},
	}
	_, err := getter.Get(context.Background(), nil)
	assert.Error(t, err)
	_, ok := err.(TypeError)
	assert.False(t, ok)
}

func Test_StandardByteSliceLikeGetter(t *testing.T) {
	tests := []struct {
		name             string
		getter           ByteSliceLikeGetter[any]
		want             any
		valid            bool
		expectedErrorMsg string
	}{
		{
			name: "string type",
			getter: StandardByteSliceLikeGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "1", nil
				},
			},
			want:  []byte{49},
			valid: true,
		},
		{
			name: "byte type",
			getter: StandardByteSliceLikeGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return []byte{49}, nil
				},
			},
			want:  []byte{49},
			valid: true,
		},
		{
			name: "int64 type",
			getter: StandardByteSliceLikeGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return int64(12), nil
				},
			},
			want:  []byte{0, 0, 0, 0, 0, 0, 0, 12},
			valid: true,
		},
		{
			name: "float64 type",
			getter: StandardByteSliceLikeGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return 1.1, nil
				},
			},
			want:  []byte{63, 241, 153, 153, 153, 153, 153, 154},
			valid: true,
		},
		{
			name: "primitive bool true",
			getter: StandardByteSliceLikeGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return true, nil
				},
			},
			want:  []byte{1},
			valid: true,
		},
		{
			name: "primitive bool false",
			getter: StandardByteSliceLikeGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return false, nil
				},
			},
			want:  []byte{0},
			valid: true,
		},
		{
			name: "pcommon.value type int",
			getter: StandardByteSliceLikeGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					v := pcommon.NewValueInt(int64(100))
					return v, nil
				},
			},
			want:  []byte{0, 0, 0, 0, 0, 0, 0, 100},
			valid: true,
		},
		{
			name: "pcommon.value type float",
			getter: StandardByteSliceLikeGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					v := pcommon.NewValueDouble(float64(1.9))
					return v, nil
				},
			},
			want:  []byte{63, 254, 102, 102, 102, 102, 102, 102},
			valid: true,
		},
		{
			name: "pcommon.value type string",
			getter: StandardByteSliceLikeGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					v := pcommon.NewValueStr("1")
					return v, nil
				},
			},
			want:  []byte{49},
			valid: true,
		},
		{
			name: "pcommon.value type bytes",
			getter: StandardByteSliceLikeGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					v := pcommon.NewValueBytes()
					v.SetEmptyBytes().Append(byte(12))
					return v, nil
				},
			},
			want:  []byte{12},
			valid: true,
		},
		{
			name: "pcommon.value type bool true",
			getter: StandardByteSliceLikeGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					v := pcommon.NewValueBool(true)
					return v, nil
				},
			},
			want:  []byte{1},
			valid: true,
		},
		{
			name: "pcommon.value type bool false",
			getter: StandardByteSliceLikeGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					v := pcommon.NewValueBool(false)
					return v, nil
				},
			},
			want:  []byte{0},
			valid: true,
		},
		{
			name: "nil",
			getter: StandardByteSliceLikeGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return nil, nil
				},
			},
			want:  nil,
			valid: true,
		},
		{
			name: "invalid type",
			getter: StandardByteSliceLikeGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return map[string]string{}, nil
				},
			},
			valid:            false,
			expectedErrorMsg: "unsupported type: map[string]string",
		},
		{
			name: "invalid pcommon.Value type",
			getter: StandardByteSliceLikeGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
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
					assert.Equal(t, tt.want, val)
				}
			} else {
				assert.IsType(t, TypeError(""), err)
				assert.EqualError(t, err, tt.expectedErrorMsg)
			}
		})
	}
}

//nolint:errorlint
func Test_StandardByteSliceLikeGetter_WrappedError(t *testing.T) {
	getter := StandardByteSliceLikeGetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
			return nil, TypeError("")
		},
	}
	_, err := getter.Get(context.Background(), nil)
	assert.Error(t, err)
	_, ok := err.(TypeError)
	assert.False(t, ok)
}

func Test_StandardBoolGetter(t *testing.T) {
	tests := []struct {
		name             string
		getter           StandardBoolGetter[any]
		want             bool
		valid            bool
		expectedErrorMsg string
	}{
		{
			name: "primitive bool type",
			getter: StandardBoolGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return true, nil
				},
			},
			want:  true,
			valid: true,
		},
		{
			name: "ValueTypeBool type",
			getter: StandardBoolGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return pcommon.NewValueBool(true), nil
				},
			},
			want:  true,
			valid: true,
		},
		{
			name: "Incorrect type",
			getter: StandardBoolGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return 1, nil
				},
			},
			valid:            false,
			expectedErrorMsg: "expected bool but got int",
		},
		{
			name: "nil",
			getter: StandardBoolGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return nil, nil
				},
			},
			valid:            false,
			expectedErrorMsg: "expected bool but got nil",
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

//nolint:errorlint
func Test_StandardBoolGetter_WrappedError(t *testing.T) {
	getter := StandardBoolGetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
			return nil, TypeError("")
		},
	}
	_, err := getter.Get(context.Background(), nil)
	assert.Error(t, err)
	_, ok := err.(TypeError)
	assert.False(t, ok)
}

func Test_StandardBoolLikeGetter(t *testing.T) {
	tests := []struct {
		name             string
		getter           BoolLikeGetter[any]
		want             any
		valid            bool
		expectedErrorMsg string
	}{
		{
			name: "string type true",
			getter: StandardBoolLikeGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "true", nil
				},
			},
			want:  true,
			valid: true,
		},
		{
			name: "string type false",
			getter: StandardBoolLikeGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "false", nil
				},
			},
			want:  false,
			valid: true,
		},
		{
			name: "int type",
			getter: StandardBoolLikeGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return 0, nil
				},
			},
			want:  false,
			valid: true,
		},
		{
			name: "float64 type",
			getter: StandardBoolLikeGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return float64(0.0), nil
				},
			},
			want:  false,
			valid: true,
		},
		{
			name: "pcommon.value type int",
			getter: StandardBoolLikeGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					v := pcommon.NewValueInt(int64(0))
					return v, nil
				},
			},
			want:  false,
			valid: true,
		},
		{
			name: "pcommon.value type string",
			getter: StandardBoolLikeGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					v := pcommon.NewValueStr("false")
					return v, nil
				},
			},
			want:  false,
			valid: true,
		},
		{
			name: "pcommon.value type bool",
			getter: StandardBoolLikeGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					v := pcommon.NewValueBool(true)
					return v, nil
				},
			},
			want:  true,
			valid: true,
		},
		{
			name: "pcommon.value type double",
			getter: StandardBoolLikeGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					v := pcommon.NewValueDouble(float64(0.0))
					return v, nil
				},
			},
			want:  false,
			valid: true,
		},
		{
			name: "nil",
			getter: StandardBoolLikeGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return nil, nil
				},
			},
			want:  nil,
			valid: true,
		},
		{
			name: "invalid type",
			getter: StandardBoolLikeGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return []byte{}, nil
				},
			},
			valid:            false,
			expectedErrorMsg: "unsupported type: []uint8",
		},
		{
			name: "invalid pcommon.value type",
			getter: StandardBoolLikeGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
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

//nolint:errorlint
func Test_StandardBoolLikeGetter_WrappedError(t *testing.T) {
	getter := StandardBoolLikeGetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
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
		getter           StandardPMapGetter[any]
		want             any
		valid            bool
		expectedErrorMsg string
	}{
		{
			name: "pcommon.map type",
			getter: StandardPMapGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return pcommon.NewMap(), nil
				},
			},
			want:  pcommon.NewMap(),
			valid: true,
		},
		{
			name: "map[string]any type",
			getter: StandardPMapGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return make(map[string]any), nil
				},
			},
			want:  pcommon.NewMap(),
			valid: true,
		},
		{
			name: "ValueTypeMap type",
			getter: StandardPMapGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return pcommon.NewValueMap(), nil
				},
			},
			want:  pcommon.NewMap(),
			valid: true,
		},
		{
			name: "Incorrect type",
			getter: StandardPMapGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return true, nil
				},
			},
			valid:            false,
			expectedErrorMsg: "expected pcommon.Map but got bool",
		},
		{
			name: "nil",
			getter: StandardPMapGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
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

//nolint:errorlint
func Test_StandardPMapGetter_WrappedError(t *testing.T) {
	getter := StandardPMapGetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
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
		getter           StandardDurationGetter[any]
		want             any
		valid            bool
		expectedErrorMsg string
	}{
		{
			name: "complex duration",
			getter: StandardDurationGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return time.ParseDuration("1h1m1s")
				},
			},
			want:  oneHourOneMinuteOneSecond,
			valid: true,
		},
		{
			name: "simple duration",
			getter: StandardDurationGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return time.ParseDuration("100ns")
				},
			},
			want:  oneHundredNsecs,
			valid: true,
		},
		{
			name: "complex duation values less than 1 seconc",
			getter: StandardDurationGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return time.ParseDuration("10ms66us7000ns")
				},
			},
			want:  tenMilliseconds,
			valid: true,
		},
		{
			name: "invalid duration units",
			getter: StandardDurationGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return time.ParseDuration("70ps")
				},
			},
			valid:            false,
			expectedErrorMsg: "unknown unit",
		},
		{
			name: "wrong type - int",
			getter: StandardDurationGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return 1, nil
				},
			},
			valid:            false,
			expectedErrorMsg: "expected duration but got int",
		},
		{
			name: "nil",
			getter: StandardDurationGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
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

//nolint:errorlint
func Test_StandardDurationGetter_WrappedError(t *testing.T) {
	getter := StandardDurationGetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
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
		name             string
		getter           StandardTimeGetter[any]
		want             string
		valid            bool
		expectedErrorMsg string
	}{
		{
			name: "2023 time",
			getter: StandardTimeGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return time.Date(2023, 8, 17, 1, 1, 1, 1, time.UTC), nil
				},
			},
			want:  "2023-08-17T01:01:01.000000001Z",
			valid: true,
		},
		{
			name: "before 2000 time",
			getter: StandardTimeGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return time.Date(1999, 12, 1, 10, 59, 58, 57, time.UTC), nil
				},
			},
			want:  "1999-12-01T10:59:58.000000057Z",
			valid: true,
		},
		{
			name: "wrong type - duration",
			getter: StandardTimeGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return time.ParseDuration("70ns")
				},
			},
			valid:            false,
			expectedErrorMsg: "expected time but got time.Duration",
		},
		{
			name: "wrong type - bool",
			getter: StandardTimeGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return true, nil
				},
			},
			valid:            false,
			expectedErrorMsg: "expected time but got bool",
		},
		{
			name: "nil",
			getter: StandardTimeGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
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
				var want time.Time
				want, err = time.Parse("2006-01-02T15:04:05.000000000Z", tt.want)
				assert.NoError(t, err)
				assert.Equal(t, want, val)
			} else {
				assert.ErrorContains(t, err, tt.expectedErrorMsg)
			}
		})
	}
}

//nolint:errorlint
func Test_StandardTimeGetter_WrappedError(t *testing.T) {
	getter := StandardTimeGetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
			return nil, TypeError("")
		},
	}
	_, err := getter.Get(context.Background(), nil)
	assert.Error(t, err)
	_, ok := err.(TypeError)
	assert.False(t, ok)
}
