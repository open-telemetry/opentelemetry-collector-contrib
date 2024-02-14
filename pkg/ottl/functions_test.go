// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottl

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
			"testing_unknown_function",
			&functionGetterArguments{},
			functionWithFunctionGetter,
		),
		createFactory[any](
			"testing_functiongetter",
			&functionGetterArguments{},
			functionWithFunctionGetter,
		),
		createFactory[any](
			"testing_optional_args",
			&optionalArgsArguments{},
			functionWithOptionalArgs,
		),
	)

	p, _ := NewParser(
		functions,
		testParsePath[any],
		componenttest.NewNopTelemetrySettings(),
		WithEnumParser[any](testParseEnum),
	)

	tests := []struct {
		name string
		inv  editor
	}{
		{
			name: "unknown function",
			inv: editor{
				Function:  "unknownfunc",
				Arguments: []argument{},
			},
		},
		{
			name: "Invalid Function Name",
			inv: editor{
				Function: "testing_functiongetter",
				Arguments: []argument{
					{
						Value: value{
							String: (ottltest.Strp("SHA256")),
						},
					},
				},
			},
		},
		{
			name: "not accessor",
			inv: editor{
				Function: "testing_getsetter",
				Arguments: []argument{
					{
						Value: value{
							String: ottltest.Strp("not path"),
						},
					},
				},
			},
		},
		{
			name: "not reader (invalid function)",
			inv: editor{
				Function: "testing_getter",
				Arguments: []argument{
					{
						Value: value{
							Literal: &mathExprLiteral{
								Converter: &converter{
									Function: "Unknownfunc",
								},
							},
						},
					},
				},
			},
		},
		{
			name: "not enough args",
			inv: editor{
				Function: "testing_multiple_args",
				Arguments: []argument{
					{
						Value: value{
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
						Value: value{
							String: ottltest.Strp("test"),
						},
					},
				},
			},
		},
		{
			name: "too many args",
			inv: editor{
				Function: "testing_multiple_args",
				Arguments: []argument{
					{
						Value: value{
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
						Value: value{
							String: ottltest.Strp("test"),
						},
					},
					{
						Value: value{
							String: ottltest.Strp("test"),
						},
					},
				},
			},
		},
		{
			name: "not matching arg type",
			inv: editor{
				Function: "testing_string",
				Arguments: []argument{
					{
						Value: value{
							Literal: &mathExprLiteral{
								Int: ottltest.Intp(10),
							},
						},
					},
				},
			},
		},
		{
			name: "not matching arg type when byte slice",
			inv: editor{
				Function: "testing_byte_slice",
				Arguments: []argument{
					{
						Value: value{
							String: ottltest.Strp("test"),
						},
					},
					{
						Value: value{
							String: ottltest.Strp("test"),
						},
					},
					{
						Value: value{
							String: ottltest.Strp("test"),
						},
					},
				},
			},
		},
		{
			name: "mismatching slice element type",
			inv: editor{
				Function: "testing_string_slice",
				Arguments: []argument{
					{
						Value: value{
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
		},
		{
			name: "mismatching slice argument type",
			inv: editor{
				Function: "testing_string_slice",
				Arguments: []argument{
					{
						Value: value{
							String: ottltest.Strp("test"),
						},
					},
				},
			},
		},
		{
			name: "named parameters used before unnamed parameters",
			inv: editor{
				Function: "testing_optional_args",
				Arguments: []argument{
					{
						Name: "get_setter_arg",
						Value: value{
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
						Value: value{
							String: ottltest.Strp("test"),
						},
					},
					{
						Name: "optional_arg",
						Value: value{
							String: ottltest.Strp("test_optional"),
						},
					},
					{
						Name: "optional_float_arg",
						Value: value{
							Literal: &mathExprLiteral{
								Float: ottltest.Floatp(1.1),
							},
						},
					},
				},
			},
		},
		{
			name: "nonexistent named parameter used",
			inv: editor{
				Function: "testing_optional_args",
				Arguments: []argument{
					{
						Value: value{
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
						Value: value{
							String: ottltest.Strp("test"),
						},
					},
					{
						Name: "no_such_name",
						Value: value{
							String: ottltest.Strp("test_optional"),
						},
					},
					{
						Name: "optional_float_arg",
						Value: value{
							Literal: &mathExprLiteral{
								Float: ottltest.Floatp(1.1),
							},
						},
					},
				},
			},
		},
		{
			name: "function call returns error",
			inv: editor{
				Function: "testing_error",
			},
		},
		{
			name: "Enum not found",
			inv: editor{
				Function: "testing_enum",
				Arguments: []argument{
					{
						Value: value{
							Enum: (*enumSymbol)(ottltest.Strp("SYMBOL_NOT_FOUND")),
						},
					},
				},
			},
		},
		{
			name: "Unknown Function",
			inv: editor{
				Function: "testing_functiongetter",
				Arguments: []argument{
					{
						Value: value{
							FunctionName: (ottltest.Strp("SHA256")),
						},
					},
				},
			},
		},
		{
			name: "factory definition uses a non-pointer Arguments value",
			inv: editor{
				Function: "non_pointer",
			},
		},
		{
			name: "path parts not all used",
			inv: editor{
				Function: "testing_getsetter",
				Arguments: []argument{
					{
						Value: value{
							Literal: &mathExprLiteral{
								Path: &path{
									Fields: []field{
										{
											Name: "name",
										},
										{
											Name: "not-used",
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
			name: "Keys not allowed",
			inv: editor{
				Function: "testing_getsetter",
				Arguments: []argument{
					{
						Value: value{
							Literal: &mathExprLiteral{
								Path: &path{
									Fields: []field{
										{
											Name: "name",
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
		testParsePath[any],
		componenttest.NewNopTelemetrySettings(),
		WithEnumParser[any](testParseEnum),
	)

	tests := []struct {
		name string
		inv  editor
		want any
	}{
		{
			name: "no arguments",
			inv: editor{
				Function:  "testing_noop",
				Arguments: []argument{},
			},
			want: nil,
		},
		{
			name: "empty slice arg",
			inv: editor{
				Function: "testing_string_slice",
				Arguments: []argument{
					{
						Value: value{
							List: &list{
								Values: []value{},
							},
						},
					},
				},
			},
			want: 0,
		},
		{
			name: "string slice arg",
			inv: editor{
				Function: "testing_string_slice",
				Arguments: []argument{
					{
						Value: value{
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
			},
			want: 3,
		},
		{
			name: "float slice arg",
			inv: editor{
				Function: "testing_float_slice",
				Arguments: []argument{
					{
						Value: value{
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
			},
			want: 3,
		},
		{
			name: "int slice arg",
			inv: editor{
				Function: "testing_int_slice",
				Arguments: []argument{
					{
						Value: value{
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
			},
			want: 3,
		},
		{
			name: "getter slice arg",
			inv: editor{
				Function: "testing_getter_slice",
				Arguments: []argument{
					{
						Value: value{
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
										Enum: (*enumSymbol)(ottltest.Strp("TEST_ENUM")),
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
												Arguments: []argument{
													{
														Value: value{
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
			inv: editor{
				Function: "testing_stringgetter_slice",
				Arguments: []argument{
					{
						Value: value{
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
			},
			want: 2,
		},
		{
			name: "durationgetter slice arg",
			inv: editor{
				Function: "testing_durationgetter_slice",
				Arguments: []argument{
					{
						Value: value{
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
		},
		{
			name: "timegetter slice arg",
			inv: editor{
				Function: "testing_timegetter_slice",
				Arguments: []argument{
					{
						Value: value{
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
		},
		{
			name: "floatgetter slice arg",
			inv: editor{
				Function: "testing_floatgetter_slice",
				Arguments: []argument{
					{
						Value: value{
							List: &list{
								Values: []value{
									{
										String: ottltest.Strp("1.1"),
									},
									{
										Literal: &mathExprLiteral{
											Float: ottltest.Floatp(1),
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
			name: "intgetter slice arg",
			inv: editor{
				Function: "testing_intgetter_slice",
				Arguments: []argument{
					{
						Value: value{
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
					},
				},
			},
			want: 2,
		},
		{
			name: "pmapgetter slice arg",
			inv: editor{
				Function: "testing_pmapgetter_slice",
				Arguments: []argument{
					{
						Value: value{
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
					},
				},
			},
			want: 2,
		},
		{
			name: "stringlikegetter slice arg",
			inv: editor{
				Function: "testing_stringlikegetter_slice",
				Arguments: []argument{
					{
						Value: value{
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
			},
			want: 2,
		},
		{
			name: "floatlikegetter slice arg",
			inv: editor{
				Function: "testing_floatlikegetter_slice",
				Arguments: []argument{
					{
						Value: value{
							List: &list{
								Values: []value{
									{
										String: ottltest.Strp("1.1"),
									},
									{
										Literal: &mathExprLiteral{
											Float: ottltest.Floatp(1.1),
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
			name: "intlikegetter slice arg",
			inv: editor{
				Function: "testing_intlikegetter_slice",
				Arguments: []argument{
					{
						Value: value{
							List: &list{
								Values: []value{
									{
										String: ottltest.Strp("1"),
									},
									{
										Literal: &mathExprLiteral{
											Float: ottltest.Floatp(1.1),
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
			name: "setter arg",
			inv: editor{
				Function: "testing_setter",
				Arguments: []argument{
					{
						Value: value{
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
			want: nil,
		},
		{
			name: "getsetter arg",
			inv: editor{
				Function: "testing_getsetter",
				Arguments: []argument{
					{
						Value: value{
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
			want: nil,
		},
		{
			name: "getter arg",
			inv: editor{
				Function: "testing_getter",
				Arguments: []argument{
					{
						Value: value{
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
			want: nil,
		},
		{
			name: "getter arg with nil literal",
			inv: editor{
				Function: "testing_getter",
				Arguments: []argument{
					{
						Value: value{
							IsNil: (*isNil)(ottltest.Boolp(true)),
						},
					},
				},
			},
			want: nil,
		},
		{
			name: "getter arg with list",
			inv: editor{
				Function: "testing_getter",
				Arguments: []argument{
					{
						Value: value{
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
											Path: &path{
												Fields: []field{
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
												Arguments: []argument{
													{
														Value: value{
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
			inv: editor{
				Function: "testing_stringgetter",
				Arguments: []argument{
					{
						Value: value{
							String: ottltest.Strp("test"),
						},
					},
				},
			},
			want: nil,
		},
		{
			name: "durationgetter arg",
			inv: editor{
				Function: "testing_durationgetter",
				Arguments: []argument{
					{
						Value: value{
							String: ottltest.Strp("test"),
						},
					},
				},
			},
		},
		{
			name: "timegetter arg",
			inv: editor{
				Function: "testing_timegetter",
				Arguments: []argument{
					{
						Value: value{
							String: ottltest.Strp("test"),
						},
					},
				},
			},
		},
		{
			name: "functiongetter arg (Uppercase)",
			inv: editor{
				Function: "testing_functiongetter",
				Arguments: []argument{
					{
						Value: value{
							FunctionName: (ottltest.Strp("SHA256")),
						},
					},
				},
			},
			want: "hashstring",
		},
		{
			name: "functiongetter arg",
			inv: editor{
				Function: "testing_functiongetter",
				Arguments: []argument{
					{
						Value: value{
							FunctionName: (ottltest.Strp("Sha256")),
						},
					},
				},
			},
			want: "hashstring",
		},
		{
			name: "stringlikegetter arg",
			inv: editor{
				Function: "testing_stringlikegetter",
				Arguments: []argument{
					{
						Value: value{
							Bool: (*boolean)(ottltest.Boolp(false)),
						},
					},
				},
			},
			want: nil,
		},
		{
			name: "floatgetter arg",
			inv: editor{
				Function: "testing_floatgetter",
				Arguments: []argument{
					{
						Value: value{
							String: ottltest.Strp("1.1"),
						},
					},
				},
			},
			want: nil,
		},
		{
			name: "floatlikegetter arg",
			inv: editor{
				Function: "testing_floatlikegetter",
				Arguments: []argument{
					{
						Value: value{
							Bool: (*boolean)(ottltest.Boolp(false)),
						},
					},
				},
			},
			want: nil,
		},
		{
			name: "intgetter arg",
			inv: editor{
				Function: "testing_intgetter",
				Arguments: []argument{
					{
						Value: value{
							Literal: &mathExprLiteral{
								Int: ottltest.Intp(1),
							},
						},
					},
				},
			},
			want: nil,
		},
		{
			name: "intlikegetter arg",
			inv: editor{
				Function: "testing_intgetter",
				Arguments: []argument{
					{
						Value: value{
							Literal: &mathExprLiteral{
								Float: ottltest.Floatp(1.1),
							},
						},
					},
				},
			},
			want: nil,
		},
		{
			name: "pmapgetter arg",
			inv: editor{
				Function: "testing_pmapgetter",
				Arguments: []argument{
					{
						Value: value{
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
			want: nil,
		},
		{
			name: "string arg",
			inv: editor{
				Function: "testing_string",
				Arguments: []argument{
					{
						Value: value{
							String: ottltest.Strp("test"),
						},
					},
				},
			},
			want: nil,
		},
		{
			name: "float arg",
			inv: editor{
				Function: "testing_float",
				Arguments: []argument{
					{
						Value: value{
							Literal: &mathExprLiteral{
								Float: ottltest.Floatp(1.1),
							},
						},
					},
				},
			},
			want: nil,
		},
		{
			name: "int arg",
			inv: editor{
				Function: "testing_int",
				Arguments: []argument{
					{
						Value: value{
							Literal: &mathExprLiteral{
								Int: ottltest.Intp(1),
							},
						},
					},
				},
			},
			want: nil,
		},
		{
			name: "bool arg",
			inv: editor{
				Function: "testing_bool",
				Arguments: []argument{
					{
						Value: value{
							Bool: (*boolean)(ottltest.Boolp(true)),
						},
					},
				},
			},
			want: nil,
		},
		{
			name: "byteSlice arg",
			inv: editor{
				Function: "testing_byte_slice",
				Arguments: []argument{
					{
						Value: value{
							Bytes: (*byteSlice)(&[]byte{1, 2, 3, 4, 5, 6, 7, 8}),
						},
					},
				},
			},
			want: nil,
		},
		{
			name: "multiple args",
			inv: editor{
				Function: "testing_multiple_args",
				Arguments: []argument{
					{
						Value: value{
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
						Value: value{
							String: ottltest.Strp("test"),
						},
					},
					{
						Value: value{
							Literal: &mathExprLiteral{
								Float: ottltest.Floatp(1.1),
							},
						},
					},
					{
						Value: value{
							Literal: &mathExprLiteral{
								Int: ottltest.Intp(1),
							},
						},
					},
				},
			},
			want: nil,
		},
		{
			name: "optional args",
			inv: editor{
				Function: "testing_optional_args",
				Arguments: []argument{
					{
						Value: value{
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
						Value: value{
							String: ottltest.Strp("test"),
						},
					},
					{
						Value: value{
							String: ottltest.Strp("test_optional"),
						},
					},
					{
						Value: value{
							Literal: &mathExprLiteral{
								Float: ottltest.Floatp(1.1),
							},
						},
					},
				},
			},
			want: nil,
		},
		{
			name: "optional named args",
			inv: editor{
				Function: "testing_optional_args",
				Arguments: []argument{
					{
						Value: value{
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
						Value: value{
							String: ottltest.Strp("test"),
						},
					},
					{
						Name: "optional_arg",
						Value: value{
							String: ottltest.Strp("test_optional"),
						},
					},
					{
						Name: "optional_float_arg",
						Value: value{
							Literal: &mathExprLiteral{
								Float: ottltest.Floatp(1.1),
							},
						},
					},
				},
			},
			want: nil,
		},
		{
			name: "Enum arg",
			inv: editor{
				Function: "testing_enum",
				Arguments: []argument{
					{
						Value: value{
							Enum: (*enumSymbol)(ottltest.Strp("TEST_ENUM")),
						},
					},
				},
			},
			want: nil,
		},
		{
			name: "Complex Indexing",
			inv: editor{
				Function: "testing_getsetter",
				Arguments: []argument{
					{
						Value: value{
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
					},
				},
			},
			want: nil,
		},
		{
			name: "path that allows keys but none have been specified",
			inv: editor{
				Function: "testing_getsetter",
				Arguments: []argument{
					{
						Value: value{
							Literal: &mathExprLiteral{
								Path: &path{
									Fields: []field{
										{
											Name: "attributes",
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

func Test_ArgumentsNotMutated(t *testing.T) {
	args := optionalArgsArguments{}
	fact := createFactory[any](
		"testing_optional_args",
		&args,
		functionWithOptionalArgs,
	)
	p, _ := NewParser(
		CreateFactoryMap[any](fact),
		testParsePath[any],
		componenttest.NewNopTelemetrySettings(),
		WithEnumParser[any](testParseEnum),
	)

	invWithOptArg := editor{
		Function: "testing_optional_args",
		Arguments: []argument{
			{
				Value: value{
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
				Value: value{
					String: ottltest.Strp("test"),
				},
			},
			{
				Value: value{
					String: ottltest.Strp("test_optional"),
				},
			},
			{
				Value: value{
					Literal: &mathExprLiteral{
						Float: ottltest.Floatp(1.1),
					},
				},
			},
		},
	}

	invWithoutOptArg := editor{
		Function: "testing_optional_args",
		Arguments: []argument{
			{
				Value: value{
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
				Value: value{
					String: ottltest.Strp("test"),
				},
			},
		},
	}

	fn, err := p.newFunctionCall(invWithOptArg)
	require.NoError(t, err)
	res, _ := fn.Eval(context.Background(), nil)
	require.Equal(t, 4, res)

	fn, err = p.newFunctionCall(invWithoutOptArg)
	require.NoError(t, err)
	res, _ = fn.Eval(context.Background(), nil)
	require.Equal(t, 2, res)

	assert.Zero(t, args.OptionalArg)
	assert.Zero(t, args.OptionalFloatArg)
}

func functionWithNoArguments() (ExprFunc[any], error) {
	return func(context.Context, any) (any, error) {
		return nil, nil
	}, nil
}

func functionWithErr() (ExprFunc[any], error) {
	return func(context.Context, any) (any, error) {
		return nil, nil
	}, fmt.Errorf("error")
}

type stringSliceArguments struct {
	Strings []string
}

func functionWithStringSlice(strs []string) (ExprFunc[any], error) {
	return func(context.Context, any) (any, error) {
		return len(strs), nil
	}, nil
}

type floatSliceArguments struct {
	Floats []float64
}

func functionWithFloatSlice(floats []float64) (ExprFunc[any], error) {
	return func(context.Context, any) (any, error) {
		return len(floats), nil
	}, nil
}

type intSliceArguments struct {
	Ints []int64
}

func functionWithIntSlice(ints []int64) (ExprFunc[any], error) {
	return func(context.Context, any) (any, error) {
		return len(ints), nil
	}, nil
}

type byteSliceArguments struct {
	Bytes []byte
}

func functionWithByteSlice(bytes []byte) (ExprFunc[any], error) {
	return func(context.Context, any) (any, error) {
		return len(bytes), nil
	}, nil
}

type getterSliceArguments struct {
	Getters []Getter[any]
}

func functionWithGetterSlice(getters []Getter[any]) (ExprFunc[any], error) {
	return func(context.Context, any) (any, error) {
		return len(getters), nil
	}, nil
}

type stringGetterSliceArguments struct {
	StringGetters []StringGetter[any]
}

func functionWithStringGetterSlice(getters []StringGetter[any]) (ExprFunc[any], error) {
	return func(context.Context, any) (any, error) {
		return len(getters), nil
	}, nil
}

type durationGetterSliceArguments struct {
	DurationGetters []DurationGetter[any]
}

func functionWithDurationGetterSlice(_ []DurationGetter[any]) (ExprFunc[any], error) {
	return func(context.Context, any) (any, error) {
		return nil, nil
	}, nil
}

type timeGetterSliceArguments struct {
	TimeGetters []TimeGetter[any]
}

func functionWithTimeGetterSlice(_ []TimeGetter[any]) (ExprFunc[any], error) {
	return func(context.Context, any) (any, error) {
		return nil, nil
	}, nil
}

type floatGetterSliceArguments struct {
	FloatGetters []FloatGetter[any]
}

func functionWithFloatGetterSlice(getters []FloatGetter[any]) (ExprFunc[any], error) {
	return func(context.Context, any) (any, error) {
		return len(getters), nil
	}, nil
}

type intGetterSliceArguments struct {
	IntGetters []IntGetter[any]
}

func functionWithIntGetterSlice(getters []IntGetter[any]) (ExprFunc[any], error) {
	return func(context.Context, any) (any, error) {
		return len(getters), nil
	}, nil
}

type pMapGetterSliceArguments struct {
	PMapGetters []PMapGetter[any]
}

func functionWithPMapGetterSlice(getters []PMapGetter[any]) (ExprFunc[any], error) {
	return func(context.Context, any) (any, error) {
		return len(getters), nil
	}, nil
}

type stringLikeGetterSliceArguments struct {
	StringLikeGetters []StringLikeGetter[any]
}

func functionWithStringLikeGetterSlice(getters []StringLikeGetter[any]) (ExprFunc[any], error) {
	return func(context.Context, any) (any, error) {
		return len(getters), nil
	}, nil
}

type floatLikeGetterSliceArguments struct {
	FloatLikeGetters []FloatLikeGetter[any]
}

func functionWithFloatLikeGetterSlice(getters []FloatLikeGetter[any]) (ExprFunc[any], error) {
	return func(context.Context, any) (any, error) {
		return len(getters), nil
	}, nil
}

type intLikeGetterSliceArguments struct {
	IntLikeGetters []IntLikeGetter[any]
}

func functionWithIntLikeGetterSlice(getters []IntLikeGetter[any]) (ExprFunc[any], error) {
	return func(context.Context, any) (any, error) {
		return len(getters), nil
	}, nil
}

type setterArguments struct {
	SetterArg Setter[any]
}

func functionWithSetter(Setter[any]) (ExprFunc[any], error) {
	return func(context.Context, any) (any, error) {
		return "anything", nil
	}, nil
}

type getSetterArguments struct {
	GetSetterArg GetSetter[any]
}

func functionWithGetSetter(GetSetter[any]) (ExprFunc[any], error) {
	return func(context.Context, any) (any, error) {
		return "anything", nil
	}, nil
}

type getterArguments struct {
	GetterArg Getter[any]
}

func functionWithGetter(Getter[any]) (ExprFunc[any], error) {
	return func(context.Context, any) (any, error) {
		return "anything", nil
	}, nil
}

type stringGetterArguments struct {
	StringGetterArg StringGetter[any]
}

func functionWithStringGetter(StringGetter[any]) (ExprFunc[any], error) {
	return func(context.Context, any) (any, error) {
		return "anything", nil
	}, nil
}

type durationGetterArguments struct {
	DurationGetterArg DurationGetter[any]
}

func functionWithDurationGetter(DurationGetter[any]) (ExprFunc[any], error) {
	return func(context.Context, any) (any, error) {
		return "anything", nil
	}, nil
}

type timeGetterArguments struct {
	TimeGetterArg TimeGetter[any]
}

func functionWithTimeGetter(TimeGetter[any]) (ExprFunc[any], error) {
	return func(context.Context, any) (any, error) {
		return "anything", nil
	}, nil
}

type functionGetterArguments struct {
	FunctionGetterArg FunctionGetter[any]
}

func functionWithFunctionGetter(FunctionGetter[any]) (ExprFunc[any], error) {
	return func(context.Context, any) (any, error) {
		return "hashstring", nil
	}, nil
}

type stringLikeGetterArguments struct {
	StringLikeGetterArg StringLikeGetter[any]
}

func functionWithStringLikeGetter(StringLikeGetter[any]) (ExprFunc[any], error) {
	return func(context.Context, any) (any, error) {
		return "anything", nil
	}, nil
}

type floatGetterArguments struct {
	FloatGetterArg FloatGetter[any]
}

func functionWithFloatGetter(FloatGetter[any]) (ExprFunc[any], error) {
	return func(context.Context, any) (any, error) {
		return "anything", nil
	}, nil
}

type floatLikeGetterArguments struct {
	FloatLikeGetterArg FloatLikeGetter[any]
}

func functionWithFloatLikeGetter(FloatLikeGetter[any]) (ExprFunc[any], error) {
	return func(context.Context, any) (any, error) {
		return "anything", nil
	}, nil
}

type intGetterArguments struct {
	IntGetterArg IntGetter[any]
}

func functionWithIntGetter(IntGetter[any]) (ExprFunc[any], error) {
	return func(context.Context, any) (any, error) {
		return "anything", nil
	}, nil
}

type intLikeGetterArguments struct {
	IntLikeGetterArg IntLikeGetter[any]
}

func functionWithIntLikeGetter(IntLikeGetter[any]) (ExprFunc[any], error) {
	return func(context.Context, any) (any, error) {
		return "anything", nil
	}, nil
}

type pMapGetterArguments struct {
	PMapArg PMapGetter[any]
}

func functionWithPMapGetter(PMapGetter[any]) (ExprFunc[any], error) {
	return func(context.Context, any) (any, error) {
		return "anything", nil
	}, nil
}

type stringArguments struct {
	StringArg string
}

func functionWithString(string) (ExprFunc[any], error) {
	return func(context.Context, any) (any, error) {
		return "anything", nil
	}, nil
}

type floatArguments struct {
	FloatArg float64
}

func functionWithFloat(float64) (ExprFunc[any], error) {
	return func(context.Context, any) (any, error) {
		return "anything", nil
	}, nil
}

type intArguments struct {
	IntArg int64
}

func functionWithInt(int64) (ExprFunc[any], error) {
	return func(context.Context, any) (any, error) {
		return "anything", nil
	}, nil
}

type boolArguments struct {
	BoolArg bool
}

func functionWithBool(bool) (ExprFunc[any], error) {
	return func(context.Context, any) (any, error) {
		return "anything", nil
	}, nil
}

type multipleArgsArguments struct {
	GetSetterArg GetSetter[any]
	StringArg    string
	FloatArg     float64
	IntArg       int64
}

func functionWithMultipleArgs(GetSetter[any], string, float64, int64) (ExprFunc[any], error) {
	return func(context.Context, any) (any, error) {
		return "anything", nil
	}, nil
}

type optionalArgsArguments struct {
	GetSetterArg     GetSetter[any]              `ottlarg:"0"`
	StringArg        string                      `ottlarg:"1"`
	OptionalArg      Optional[StringGetter[any]] `ottlarg:"2"`
	OptionalFloatArg Optional[float64]           `ottlarg:"3"`
}

func functionWithOptionalArgs(_ GetSetter[any], _ string, stringOpt Optional[StringGetter[any]], floatOpt Optional[float64]) (ExprFunc[any], error) {
	return func(context.Context, any) (any, error) {
		argCount := 2

		if !stringOpt.IsEmpty() {
			argCount++
		}
		if !floatOpt.IsEmpty() {
			argCount++
		}

		return argCount, nil
	}, nil
}

type errorFunctionArguments struct{}

func functionThatHasAnError() (ExprFunc[any], error) {
	err := errors.New("testing")
	return func(context.Context, any) (any, error) {
		return "anything", nil
	}, err
}

type enumArguments struct {
	EnumArg Enum
}

func functionWithEnum(Enum) (ExprFunc[any], error) {
	return func(context.Context, any) (any, error) {
		return "anything", nil
	}, nil
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
			"testing_durationgetter_slice",
			&durationGetterSliceArguments{},
			functionWithDurationGetterSlice,
		),
		createFactory[any](
			"testing_timegetter_slice",
			&timeGetterSliceArguments{},
			functionWithTimeGetterSlice,
		),
		createFactory[any](
			"testing_stringlikegetter_slice",
			&stringLikeGetterSliceArguments{},
			functionWithStringLikeGetterSlice,
		),
		createFactory[any](
			"testing_floatgetter_slice",
			&floatGetterSliceArguments{},
			functionWithFloatGetterSlice,
		),
		createFactory[any](
			"testing_floatlikegetter_slice",
			&floatLikeGetterSliceArguments{},
			functionWithFloatLikeGetterSlice,
		),
		createFactory[any](
			"testing_intgetter_slice",
			&intGetterSliceArguments{},
			functionWithIntGetterSlice,
		),
		createFactory[any](
			"testing_intlikegetter_slice",
			&intLikeGetterSliceArguments{},
			functionWithIntLikeGetterSlice,
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
			"testing_durationgetter",
			&durationGetterArguments{},
			functionWithDurationGetter,
		),
		createFactory[any](
			"testing_timegetter",
			&timeGetterArguments{},
			functionWithTimeGetter,
		),
		createFactory[any](
			"testing_stringgetter",
			&stringGetterArguments{},
			functionWithStringGetter,
		),
		createFactory[any](
			"testing_functiongetter",
			&functionGetterArguments{},
			functionWithFunctionGetter,
		),
		createFactory[any](
			"SHA256",
			&stringGetterArguments{},
			functionWithStringGetter,
		),
		createFactory[any](
			"Sha256",
			&stringGetterArguments{},
			functionWithStringGetter,
		),
		createFactory[any](
			"testing_stringlikegetter",
			&stringLikeGetterArguments{},
			functionWithStringLikeGetter,
		),
		createFactory[any](
			"testing_floatgetter",
			&floatGetterArguments{},
			functionWithFloatGetter,
		),
		createFactory[any](
			"testing_floatlikegetter",
			&floatLikeGetterArguments{},
			functionWithFloatLikeGetter,
		),
		createFactory[any](
			"testing_intgetter",
			&intGetterArguments{},
			functionWithIntGetter,
		),
		createFactory[any](
			"testing_intlikegetter",
			&intLikeGetterArguments{},
			functionWithIntLikeGetter,
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
			"testing_optional_args",
			&optionalArgsArguments{},
			functionWithOptionalArgs,
		),
		createFactory[any](
			"testing_enum",
			&enumArguments{},
			functionWithEnum,
		),
	)
}

func Test_basePath_Name(t *testing.T) {
	bp := basePath[any]{
		name: "test",
	}
	n := bp.Name()
	assert.Equal(t, "test", n)
}

func Test_basePath_Next(t *testing.T) {
	bp := basePath[any]{
		nextPath: &basePath[any]{},
	}
	next := bp.Next()
	assert.NotNil(t, next)
	assert.Nil(t, next.Next())
}

func Test_basePath_Keys(t *testing.T) {
	k := &baseKey[any]{}
	bp := basePath[any]{
		keys: []Key[any]{
			k,
		},
	}
	ks := bp.Keys()
	assert.Equal(t, 1, len(ks))
	assert.Equal(t, k, ks[0])
}

func Test_basePath_isComplete(t *testing.T) {
	tests := []struct {
		name          string
		p             basePath[any]
		expectedError bool
	}{
		{
			name: "fetched no next",
			p: basePath[any]{
				fetched: true,
			},
		},
		{
			name: "fetched with next",
			p: basePath[any]{
				fetched: true,
				nextPath: &basePath[any]{
					fetched: true,
				},
			},
		},
		{
			name: "not fetched no next",
			p: basePath[any]{
				fetched: false,
			},
			expectedError: true,
		},
		{
			name: "not fetched with next",
			p: basePath[any]{
				fetched: true,
				nextPath: &basePath[any]{
					fetched: false,
				},
			},
			expectedError: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.p.isComplete()
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func Test_basePath_NextWithIsComplete(t *testing.T) {
	tests := []struct {
		name          string
		pathFunc      func() *basePath[any]
		expectedError bool
	}{
		{
			name: "fetched",
			pathFunc: func() *basePath[any] {
				bp := basePath[any]{
					fetched: true,
					nextPath: &basePath[any]{
						fetched: false,
					},
				}
				bp.Next()
				return &bp
			},
		},
		{
			name: "not fetched enough",
			pathFunc: func() *basePath[any] {
				bp := basePath[any]{
					fetched: true,
					nextPath: &basePath[any]{
						fetched: false,
						nextPath: &basePath[any]{
							fetched: false,
						},
					},
				}
				bp.Next()
				return &bp
			},
			expectedError: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.pathFunc().isComplete()
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func Test_newPath(t *testing.T) {
	fields := []field{
		{
			Name: "body",
		},
		{
			Name: "string",
		},
	}
	np, err := newPath[any](fields)
	assert.NoError(t, err)
	p := Path[any](np)
	assert.Equal(t, "body", p.Name())
	assert.Nil(t, p.Keys())
	p = p.Next()
	assert.Equal(t, "string", p.Name())
	assert.Nil(t, p.Keys())
	assert.Nil(t, p.Next())
}

func Test_baseKey_String(t *testing.T) {
	bp := baseKey[any]{
		s: ottltest.Strp("test"),
	}
	s, err := bp.String(context.Background(), nil)
	assert.NoError(t, err)
	assert.NotNil(t, s)
	assert.Equal(t, "test", *s)
}

func Test_baseKey_Int(t *testing.T) {
	bp := baseKey[any]{
		i: ottltest.Intp(1),
	}
	i, err := bp.Int(context.Background(), nil)
	assert.NoError(t, err)
	assert.NotNil(t, i)
	assert.Equal(t, int64(1), *i)
}

func Test_newKey(t *testing.T) {
	keys := []key{
		{
			String: ottltest.Strp("foo"),
		},
		{
			String: ottltest.Strp("bar"),
		},
	}
	ks := newKeys[any](keys)

	assert.Equal(t, 2, len(ks))

	s, err := ks[0].String(context.Background(), nil)
	assert.NoError(t, err)
	assert.NotNil(t, s)
	assert.Equal(t, "foo", *s)
	s, err = ks[1].String(context.Background(), nil)
	assert.NoError(t, err)
	assert.NotNil(t, s)
	assert.Equal(t, "bar", *s)
}
