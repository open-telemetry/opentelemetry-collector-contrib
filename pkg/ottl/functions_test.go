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

package ottl

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottltest"
)

func Test_NewFunctionCall_invalid(t *testing.T) {
	functions := CreateFactoryMap(
		createFactory(
			"testing_error",
			&errorFunctionArguments{},
			functionThatHasAnError,
		),
		createFactory[any](
			"testing_getsetter",
			&getSetterArguments{},
			functionWithGetSetter,
		),
		createFactory[any](
			"testing_getter",
			&getterArguments{},
			functionWithGetter,
		),
		createFactory[any](
			"testing_multiple_args",
			&multipleArgsArguments{},
			functionWithMultipleArgs,
		),
		createFactory[any](
			"testing_string",
			&stringArguments{},
			functionWithString,
		),
		createFactory(
			"testing_string_slice",
			&stringSliceArguments{},
			functionWithStringSlice,
		),
		createFactory(
			"testing_byte_slice",
			&byteSliceArguments{},
			functionWithByteSlice,
		),
		createFactory[any](
			"testing_enum",
			&enumArguments{},
			functionWithEnum,
		),
		createFactory(
			"non_pointer",
			errorFunctionArguments{},
			functionThatHasAnError,
		),
		createFactory(
			"no_struct_tag",
			&noStructTagFunctionArguments{},
			functionThatHasAnError,
		),
		createFactory(
			"wrong_struct_tag",
			&wrongTagFunctionArguments{},
			functionThatHasAnError,
		),
		createFactory(
			"bad_struct_tag",
			&badStructTagFunctionArguments{},
			functionThatHasAnError,
		),
		createFactory(
			"negative_struct_tag",
			&negativeStructTagFunctionArguments{},
			functionThatHasAnError,
		),
		createFactory(
			"out_of_bounds_struct_tag",
			&outOfBoundsStructTagFunctionArguments{},
			functionThatHasAnError,
		),
	)

	p, _ := NewParser(
		functions,
		testParsePath,
		componenttest.NewNopTelemetrySettings(),
		WithEnumParser[any](testParseEnum),
	)

	tests := []struct {
		name string
		inv  invocation
	}{
		{
			name: "unknown function",
			inv: invocation{
				Function:  "unknownfunc",
				Arguments: []value{},
			},
		},
		{
			name: "not accessor",
			inv: invocation{
				Function: "testing_getsetter",
				Arguments: []value{
					{
						String: ottltest.Strp("not path"),
					},
				},
			},
		},
		{
			name: "not reader (invalid function)",
			inv: invocation{
				Function: "testing_getter",
				Arguments: []value{
					{
						Literal: &mathExprLiteral{
							Converter: &converter{
								Function: "Unknownfunc",
							},
						},
					},
				},
			},
		},
		{
			name: "not enough args",
			inv: invocation{
				Function: "testing_multiple_args",
				Arguments: []value{
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
					{
						String: ottltest.Strp("test"),
					},
				},
			},
		},
		{
			name: "too many args",
			inv: invocation{
				Function: "testing_multiple_args",
				Arguments: []value{
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
					{
						String: ottltest.Strp("test"),
					},
					{
						String: ottltest.Strp("test"),
					},
				},
			},
		},
		{
			name: "not enough args with telemetrySettings",
			inv: invocation{
				Function: "testing_telemetry_settings_first",
				Arguments: []value{
					{
						String: ottltest.Strp("test"),
					},
					{
						List: &list{
							Values: []value{
								{
									String: ottltest.Strp("test"),
								},
							},
						},
					},
				},
			},
		},
		{
			name: "too many args with telemetrySettings",
			inv: invocation{
				Function: "testing_telemetry_settings_first",
				Arguments: []value{
					{
						String: ottltest.Strp("test"),
					},
					{
						List: &list{
							Values: []value{
								{
									String: ottltest.Strp("test"),
								},
							},
						},
					},
					{
						Literal: &mathExprLiteral{
							Int: ottltest.Intp(10),
						},
					},
					{
						Literal: &mathExprLiteral{
							Int: ottltest.Intp(10),
						},
					},
				},
			},
		},
		{
			name: "not matching arg type",
			inv: invocation{
				Function: "testing_string",
				Arguments: []value{
					{
						Literal: &mathExprLiteral{
							Int: ottltest.Intp(10),
						},
					},
				},
			},
		},
		{
			name: "not matching arg type when byte slice",
			inv: invocation{
				Function: "testing_byte_slice",
				Arguments: []value{
					{
						String: ottltest.Strp("test"),
					},
					{
						String: ottltest.Strp("test"),
					},
					{
						String: ottltest.Strp("test"),
					},
				},
			},
		},
		{
			name: "mismatching slice element type",
			inv: invocation{
				Function: "testing_string_slice",
				Arguments: []value{
					{
						List: &list{
							Values: []value{
								{
									String: ottltest.Strp("test"),
								},
								{
									Literal: &mathExprLiteral{
										Int: ottltest.Intp(10),
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "mismatching slice argument type",
			inv: invocation{
				Function: "testing_string_slice",
				Arguments: []value{
					{
						String: ottltest.Strp("test"),
					},
				},
			},
		},
		{
			name: "function call returns error",
			inv: invocation{
				Function: "testing_error",
			},
		},
		{
			name: "Enum not found",
			inv: invocation{
				Function: "testing_enum",
				Arguments: []value{
					{
						Enum: (*EnumSymbol)(ottltest.Strp("SYMBOL_NOT_FOUND")),
					},
				},
			},
		},
		{
			name: "factory definition uses a non-pointer Arguments value",
			inv: invocation{
				Function: "non_pointer",
			},
		},
		{
			name: "no struct tags",
			inv: invocation{
				Function: "no_struct_tag",
				Arguments: []value{
					{
						String: ottltest.Strp("str"),
					},
				},
			},
		},
		{
			name: "using the wrong struct tag",
			inv: invocation{
				Function: "wrong_struct_tag",
				Arguments: []value{
					{
						String: ottltest.Strp("str"),
					},
				},
			},
		},
		{
			name: "non-integer struct tags",
			inv: invocation{
				Function: "bad_struct_tag",
				Arguments: []value{
					{
						String: ottltest.Strp("str"),
					},
				},
			},
		},
		{
			name: "struct tag index too low",
			inv: invocation{
				Function: "negative_struct_tag",
				Arguments: []value{
					{
						String: ottltest.Strp("str"),
					},
				},
			},
		},
		{
			name: "struct tag index too high",
			inv: invocation{
				Function: "out_of_bounds_struct_tag",
				Arguments: []value{
					{
						String: ottltest.Strp("str"),
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := p.newFunctionCall(tt.inv)
			t.Log(err)
			assert.Error(t, err)
		})
	}
}

func Test_NewFunctionCall(t *testing.T) {
	p, _ := NewParser(
		defaultFunctionsForTests(),
		testParsePath,
		componenttest.NewNopTelemetrySettings(),
		WithEnumParser[any](testParseEnum),
	)

	tests := []struct {
		name string
		inv  invocation
		want any
	}{
		{
			name: "no arguments",
			inv: invocation{
				Function: "testing_noop",
				Arguments: []value{
					{
						List: &list{
							Values: []value{},
						},
					},
				},
			},
			want: nil,
		},
		{
			name: "empty slice arg",
			inv: invocation{
				Function: "testing_string_slice",
				Arguments: []value{
					{
						List: &list{
							Values: []value{},
						},
					},
				},
			},
			want: 0,
		},
		{
			name: "string slice arg",
			inv: invocation{
				Function: "testing_string_slice",
				Arguments: []value{
					{
						List: &list{
							Values: []value{
								{
									String: ottltest.Strp("test"),
								},
								{
									String: ottltest.Strp("test"),
								},
								{
									String: ottltest.Strp("test"),
								},
							},
						},
					},
				},
			},
			want: 3,
		},
		{
			name: "float slice arg",
			inv: invocation{
				Function: "testing_float_slice",
				Arguments: []value{
					{
						List: &list{
							Values: []value{
								{
									Literal: &mathExprLiteral{
										Float: ottltest.Floatp(1.1),
									},
								},
								{
									Literal: &mathExprLiteral{
										Float: ottltest.Floatp(1.2),
									},
								},
								{
									Literal: &mathExprLiteral{
										Float: ottltest.Floatp(1.3),
									},
								},
							},
						},
					},
				},
			},
			want: 3,
		},
		{
			name: "int slice arg",
			inv: invocation{
				Function: "testing_int_slice",
				Arguments: []value{
					{
						List: &list{
							Values: []value{
								{
									Literal: &mathExprLiteral{
										Int: ottltest.Intp(1),
									},
								},
								{
									Literal: &mathExprLiteral{
										Int: ottltest.Intp(1),
									},
								},
								{
									Literal: &mathExprLiteral{
										Int: ottltest.Intp(1),
									},
								},
							},
						},
					},
				},
			},
			want: 3,
		},
		{
			name: "getter slice arg",
			inv: invocation{
				Function: "testing_getter_slice",
				Arguments: []value{
					{
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
								{
									String: ottltest.Strp("test"),
								},
								{
									Literal: &mathExprLiteral{
										Int: ottltest.Intp(1),
									},
								},
								{
									Literal: &mathExprLiteral{
										Float: ottltest.Floatp(1.1),
									},
								},
								{
									Bool: (*boolean)(ottltest.Boolp(true)),
								},
								{
									Enum: (*EnumSymbol)(ottltest.Strp("TEST_ENUM")),
								},
								{
									List: &list{
										Values: []value{
											{
												String: ottltest.Strp("test"),
											},
											{
												String: ottltest.Strp("test"),
											},
										},
									},
								},
								{
									List: &list{
										Values: []value{
											{
												String: ottltest.Strp("test"),
											},
											{
												List: &list{
													Values: []value{
														{
															String: ottltest.Strp("test"),
														},
														{
															List: &list{
																Values: []value{
																	{
																		String: ottltest.Strp("test"),
																	},
																	{
																		String: ottltest.Strp("test"),
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
								{
									Literal: &mathExprLiteral{
										Converter: &converter{
											Function: "testing_getter",
											Arguments: []value{
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
								},
							},
						},
					},
				},
			},
			want: 9,
		},
		{
			name: "stringgetter slice arg",
			inv: invocation{
				Function: "testing_stringgetter_slice",
				Arguments: []value{
					{
						List: &list{
							Values: []value{
								{
									String: ottltest.Strp("test"),
								},
								{
									String: ottltest.Strp("also test"),
								},
							},
						},
					},
				},
			},
			want: 2,
		},
		{
			name: "pmapgetter slice arg",
			inv: invocation{
				Function: "testing_pmapgetter_slice",
				Arguments: []value{
					{
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
				},
			},
			want: 2,
		},
		{
			name: "stringlikegetter slice arg",
			inv: invocation{
				Function: "testing_stringlikegetter_slice",
				Arguments: []value{
					{
						List: &list{
							Values: []value{
								{
									String: ottltest.Strp("test"),
								},
								{
									Literal: &mathExprLiteral{
										Int: ottltest.Intp(1),
									},
								},
							},
						},
					},
				},
			},
			want: 2,
		},
		{
			name: "setter arg",
			inv: invocation{
				Function: "testing_setter",
				Arguments: []value{
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
			want: nil,
		},
		{
			name: "getsetter arg",
			inv: invocation{
				Function: "testing_getsetter",
				Arguments: []value{
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
			want: nil,
		},
		{
			name: "getter arg",
			inv: invocation{
				Function: "testing_getter",
				Arguments: []value{
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
			want: nil,
		},
		{
			name: "getter arg with nil literal",
			inv: invocation{
				Function: "testing_getter",
				Arguments: []value{
					{
						IsNil: (*isNil)(ottltest.Boolp(true)),
					},
				},
			},
			want: nil,
		},
		{
			name: "getter arg with list",
			inv: invocation{
				Function: "testing_getter",
				Arguments: []value{
					{
						List: &list{
							Values: []value{
								{
									String: ottltest.Strp("test"),
								},
								{
									Literal: &mathExprLiteral{
										Int: ottltest.Intp(1),
									},
								},
								{
									Literal: &mathExprLiteral{
										Float: ottltest.Floatp(1.1),
									},
								},
								{
									Bool: (*boolean)(ottltest.Boolp(true)),
								},
								{
									Bytes: (*byteSlice)(&[]byte{1, 2, 3, 4, 5, 6, 7, 8}),
								},
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
								{
									Literal: &mathExprLiteral{
										Converter: &converter{
											Function: "testing_getter",
											Arguments: []value{
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
								},
							},
						},
					},
				},
			},
			want: nil,
		},
		{
			name: "stringgetter arg",
			inv: invocation{
				Function: "testing_stringgetter",
				Arguments: []value{
					{
						String: ottltest.Strp("test"),
					},
				},
			},
			want: nil,
		},
		{
			name: "stringlikegetter arg",
			inv: invocation{
				Function: "testing_stringlikegetter",
				Arguments: []value{
					{
						Bool: (*boolean)(ottltest.Boolp(false)),
					},
				},
			},
			want: nil,
		},
		{
			name: "intgetter arg",
			inv: invocation{
				Function: "testing_intgetter",
				Arguments: []value{
					{
						Literal: &mathExprLiteral{
							Int: ottltest.Intp(1),
						},
					},
				},
			},
			want: nil,
		},
		{
			name: "pmapgetter arg",
			inv: invocation{
				Function: "testing_pmapgetter",
				Arguments: []value{
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
			want: nil,
		},
		{
			name: "string arg",
			inv: invocation{
				Function: "testing_string",
				Arguments: []value{
					{
						String: ottltest.Strp("test"),
					},
				},
			},
			want: nil,
		},
		{
			name: "float arg",
			inv: invocation{
				Function: "testing_float",
				Arguments: []value{
					{
						Literal: &mathExprLiteral{
							Float: ottltest.Floatp(1.1),
						},
					},
				},
			},
			want: nil,
		},
		{
			name: "int arg",
			inv: invocation{
				Function: "testing_int",
				Arguments: []value{
					{
						Literal: &mathExprLiteral{
							Int: ottltest.Intp(1),
						},
					},
				},
			},
			want: nil,
		},
		{
			name: "bool arg",
			inv: invocation{
				Function: "testing_bool",
				Arguments: []value{
					{
						Bool: (*boolean)(ottltest.Boolp(true)),
					},
				},
			},
			want: nil,
		},
		{
			name: "byteSlice arg",
			inv: invocation{
				Function: "testing_byte_slice",
				Arguments: []value{
					{
						Bytes: (*byteSlice)(&[]byte{1, 2, 3, 4, 5, 6, 7, 8}),
					},
				},
			},
			want: nil,
		},
		{
			name: "multiple args",
			inv: invocation{
				Function: "testing_multiple_args",
				Arguments: []value{
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
					{
						String: ottltest.Strp("test"),
					},
					{
						Literal: &mathExprLiteral{
							Float: ottltest.Floatp(1.1),
						},
					},
					{
						Literal: &mathExprLiteral{
							Int: ottltest.Intp(1),
						},
					},
				},
			},
			want: nil,
		},
		{
			name: "Enum arg",
			inv: invocation{
				Function: "testing_enum",
				Arguments: []value{
					{
						Enum: (*EnumSymbol)(ottltest.Strp("TEST_ENUM")),
					},
				},
			},
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fn, err := p.newFunctionCall(tt.inv)
			assert.NoError(t, err)

			if tt.want != nil {
				result, _ := fn.Eval(context.Background(), nil)
				assert.Equal(t, tt.want, result)
			}
		})
	}
}

func functionWithNoArguments() (ExprFunc[any], error) {
	return func(context.Context, any) (any, error) {
		return nil, nil
	}, nil
}

type stringSliceArguments struct {
	Strings []string `ottlarg:"0"`
}

func functionWithStringSlice(strs []string) (ExprFunc[any], error) {
	return func(context.Context, any) (any, error) {
		return len(strs), nil
	}, nil
}

type floatSliceArguments struct {
	Floats []float64 `ottlarg:"0"`
}

func functionWithFloatSlice(floats []float64) (ExprFunc[interface{}], error) {
	return func(context.Context, interface{}) (interface{}, error) {
		return len(floats), nil
	}, nil
}

type intSliceArguments struct {
	Ints []int64 `ottlarg:"0"`
}

func functionWithIntSlice(ints []int64) (ExprFunc[interface{}], error) {
	return func(context.Context, interface{}) (interface{}, error) {
		return len(ints), nil
	}, nil
}

type byteSliceArguments struct {
	Bytes []byte `ottlarg:"0"`
}

func functionWithByteSlice(bytes []byte) (ExprFunc[interface{}], error) {
	return func(context.Context, interface{}) (interface{}, error) {
		return len(bytes), nil
	}, nil
}

type getterSliceArguments struct {
	Getters []Getter[any] `ottlarg:"0"`
}

func functionWithGetterSlice(getters []Getter[interface{}]) (ExprFunc[interface{}], error) {
	return func(context.Context, interface{}) (interface{}, error) {
		return len(getters), nil
	}, nil
}

type stringGetterSliceArguments struct {
	StringGetters []StringGetter[any] `ottlarg:"0"`
}

func functionWithStringGetterSlice(getters []StringGetter[interface{}]) (ExprFunc[interface{}], error) {
	return func(context.Context, interface{}) (interface{}, error) {
		return len(getters), nil
	}, nil
}

type pMapGetterSliceArguments struct {
	PMapGetters []PMapGetter[any] `ottlarg:"0"`
}

func functionWithPMapGetterSlice(getters []PMapGetter[interface{}]) (ExprFunc[interface{}], error) {
	return func(context.Context, interface{}) (interface{}, error) {
		return len(getters), nil
	}, nil
}

type stringLikeGetterSliceArguments struct {
	StringLikeGetters []StringLikeGetter[any] `ottlarg:"0"`
}

func functionWithStringLikeGetterSlice(getters []StringLikeGetter[interface{}]) (ExprFunc[interface{}], error) {
	return func(context.Context, interface{}) (interface{}, error) {
		return len(getters), nil
	}, nil
}

type setterArguments struct {
	SetterArg Setter[any] `ottlarg:"0"`
}

func functionWithSetter(Setter[interface{}]) (ExprFunc[interface{}], error) {
	return func(context.Context, interface{}) (interface{}, error) {
		return "anything", nil
	}, nil
}

type getSetterArguments struct {
	GetSetterArg GetSetter[any] `ottlarg:"0"`
}

func functionWithGetSetter(GetSetter[interface{}]) (ExprFunc[interface{}], error) {
	return func(context.Context, interface{}) (interface{}, error) {
		return "anything", nil
	}, nil
}

type getterArguments struct {
	GetterArg Getter[any] `ottlarg:"0"`
}

func functionWithGetter(Getter[interface{}]) (ExprFunc[interface{}], error) {
	return func(context.Context, interface{}) (interface{}, error) {
		return "anything", nil
	}, nil
}

type stringGetterArguments struct {
	StringGetterArg StringGetter[any] `ottlarg:"0"`
}

func functionWithStringGetter(StringGetter[interface{}]) (ExprFunc[interface{}], error) {
	return func(context.Context, interface{}) (interface{}, error) {
		return "anything", nil
	}, nil
}

type stringLikeGetterArguments struct {
	StringLikeGetterArg StringLikeGetter[any] `ottlarg:"0"`
}

func functionWithStringLikeGetter(StringLikeGetter[interface{}]) (ExprFunc[interface{}], error) {
	return func(context.Context, interface{}) (interface{}, error) {
		return "anything", nil
	}, nil
}

type intGetterArguments struct {
	IntGetterArg IntGetter[any] `ottlarg:"0"`
}

func functionWithIntGetter(IntGetter[interface{}]) (ExprFunc[interface{}], error) {
	return func(context.Context, interface{}) (interface{}, error) {
		return "anything", nil
	}, nil
}

type pMapGetterArguments struct {
	PMapArg PMapGetter[any] `ottlarg:"0"`
}

func functionWithPMapGetter(PMapGetter[interface{}]) (ExprFunc[interface{}], error) {
	return func(context.Context, interface{}) (interface{}, error) {
		return "anything", nil
	}, nil
}

type stringArguments struct {
	StringArg string `ottlarg:"0"`
}

func functionWithString(string) (ExprFunc[interface{}], error) {
	return func(context.Context, interface{}) (interface{}, error) {
		return "anything", nil
	}, nil
}

type floatArguments struct {
	FloatArg float64 `ottlarg:"0"`
}

func functionWithFloat(float64) (ExprFunc[interface{}], error) {
	return func(context.Context, interface{}) (interface{}, error) {
		return "anything", nil
	}, nil
}

type intArguments struct {
	IntArg int64 `ottlarg:"0"`
}

func functionWithInt(int64) (ExprFunc[interface{}], error) {
	return func(context.Context, interface{}) (interface{}, error) {
		return "anything", nil
	}, nil
}

type boolArguments struct {
	BoolArg bool `ottlarg:"0"`
}

func functionWithBool(bool) (ExprFunc[interface{}], error) {
	return func(context.Context, interface{}) (interface{}, error) {
		return "anything", nil
	}, nil
}

type multipleArgsArguments struct {
	GetSetterArg GetSetter[any] `ottlarg:"0"`
	StringArg    string         `ottlarg:"1"`
	FloatArg     float64        `ottlarg:"2"`
	IntArg       int64          `ottlarg:"3"`
}

func functionWithMultipleArgs(GetSetter[interface{}], string, float64, int64) (ExprFunc[interface{}], error) {
	return func(context.Context, interface{}) (interface{}, error) {
		return "anything", nil
	}, nil
}

type errorFunctionArguments struct{}

func functionThatHasAnError() (ExprFunc[interface{}], error) {
	err := errors.New("testing")
	return func(context.Context, interface{}) (interface{}, error) {
		return "anything", nil
	}, err
}

type enumArguments struct {
	EnumArg Enum `ottlarg:"0"`
}

func functionWithEnum(Enum) (ExprFunc[interface{}], error) {
	return func(context.Context, interface{}) (interface{}, error) {
		return "anything", nil
	}, nil
}

type noStructTagFunctionArguments struct {
	StringArg string
}

type badStructTagFunctionArguments struct {
	StringArg string `ottlarg:"a"`
}

type negativeStructTagFunctionArguments struct {
	StringArg string `ottlarg:"-1"`
}

type outOfBoundsStructTagFunctionArguments struct {
	StringArg string `ottlarg:"1"`
}

type wrongTagFunctionArguments struct {
	StringArg string `argument:"1"`
}

func createFactory[A any](name string, args A, fn any) Factory[any] {
	createFunction := func(fCtx FunctionContext, oArgs Arguments) (ExprFunc[any], error) {
		fArgs, ok := oArgs.(A)

		if !ok {
			return nil, fmt.Errorf("createFactory args must be of type %T", fArgs)
		}

		funcVal := reflect.ValueOf(fn)

		if funcVal.Kind() != reflect.Func {
			return nil, fmt.Errorf("a non-function value was passed to createFactory")
		}

		argsVal := reflect.ValueOf(fArgs).Elem()
		fnArgs := make([]reflect.Value, argsVal.NumField())

		for i := 0; i < argsVal.NumField(); i++ {
			fnArgs[i] = argsVal.Field(i)
		}

		out := funcVal.Call(fnArgs)

		if !out[1].IsNil() {
			return out[0].Interface().(ExprFunc[any]), out[1].Interface().(error)
		}

		return out[0].Interface().(ExprFunc[any]), nil
	}

	return NewFactory(name, args, createFunction)
}

func defaultFunctionsForTests() map[string]Factory[any] {
	return CreateFactoryMap(
		NewFactory(
			"testing_noop",
			nil,
			func(FunctionContext, Arguments) (ExprFunc[any], error) {
				return functionWithNoArguments()
			},
		),
		createFactory(
			"testing_string_slice",
			&stringSliceArguments{},
			functionWithStringSlice,
		),
		createFactory(
			"testing_float_slice",
			&floatSliceArguments{},
			functionWithFloatSlice,
		),
		createFactory(
			"testing_int_slice",
			&intSliceArguments{},
			functionWithIntSlice,
		),
		createFactory(
			"testing_byte_slice",
			&byteSliceArguments{},
			functionWithByteSlice,
		),
		createFactory[any](
			"testing_getter_slice",
			&getterSliceArguments{},
			functionWithGetterSlice,
		),
		createFactory[any](
			"testing_stringgetter_slice",
			&stringGetterSliceArguments{},
			functionWithStringGetterSlice,
		),
		createFactory[any](
			"testing_stringlikegetter_slice",
			&stringLikeGetterSliceArguments{},
			functionWithStringLikeGetterSlice,
		),
		createFactory[any](
			"testing_pmapgetter_slice",
			&pMapGetterSliceArguments{},
			functionWithPMapGetterSlice,
		),
		createFactory[any](
			"testing_setter",
			&setterArguments{},
			functionWithSetter,
		),
		createFactory[any](
			"testing_getsetter",
			&getSetterArguments{},
			functionWithGetSetter,
		),
		createFactory[any](
			"testing_getter",
			&getterArguments{},
			functionWithGetter,
		),
		createFactory[any](
			"testing_stringgetter",
			&stringGetterArguments{},
			functionWithStringGetter,
		),
		createFactory[any](
			"testing_stringlikegetter",
			&stringLikeGetterArguments{},
			functionWithStringLikeGetter,
		),
		createFactory[any](
			"testing_intgetter",
			&intGetterArguments{},
			functionWithIntGetter,
		),
		createFactory[any](
			"testing_pmapgetter",
			&pMapGetterArguments{},
			functionWithPMapGetter,
		),
		createFactory[any](
			"testing_string",
			&stringArguments{},
			functionWithString,
		),
		createFactory[any](
			"testing_float",
			&floatArguments{},
			functionWithFloat,
		),
		createFactory[any](
			"testing_int",
			&intArguments{},
			functionWithInt,
		),
		createFactory[any](
			"testing_bool",
			&boolArguments{},
			functionWithBool,
		),
		createFactory[any](
			"testing_multiple_args",
			&multipleArgsArguments{},
			functionWithMultipleArgs,
		),
		createFactory[any](
			"testing_enum",
			&enumArguments{},
			functionWithEnum,
		),
	)
}
