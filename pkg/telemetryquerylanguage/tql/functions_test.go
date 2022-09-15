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

package tql

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/telemetryquerylanguage/tql/tqltest"
)

func Test_NewFunctionCall_invalid(t *testing.T) {
	functions := make(map[string]interface{})
	functions["testing_error"] = functionThatHasAnError
	functions["testing_getsetter"] = functionWithGetSetter
	functions["testing_getter"] = functionWithGetter
	functions["testing_multiple_args"] = functionWithMultipleArgs
	functions["testing_string"] = functionWithString
	functions["testing_byte_slice"] = functionWithByteSlice
	functions["testing_enum"] = functionWithEnum
	functions["testing_logger_first"] = functionWithLoggerFirst

	p := NewParser(
		functions,
		testParsePath,
		testParseEnum,
		NoOpLogger{},
	)

	tests := []struct {
		name string
		inv  Invocation
	}{
		{
			name: "unknown function",
			inv: Invocation{
				Function:  "unknownfunc",
				Arguments: []Value{},
			},
		},
		{
			name: "not accessor",
			inv: Invocation{
				Function: "testing_getsetter",
				Arguments: []Value{
					{
						String: tqltest.Strp("not path"),
					},
				},
			},
		},
		{
			name: "not reader (invalid function)",
			inv: Invocation{
				Function: "testing_getter",
				Arguments: []Value{
					{
						Invocation: &Invocation{
							Function: "unknownfunc",
						},
					},
				},
			},
		},
		{
			name: "not enough args",
			inv: Invocation{
				Function: "testing_multiple_args",
				Arguments: []Value{
					{
						Path: &Path{
							Fields: []Field{
								{
									Name: "name",
								},
							},
						},
					},
					{
						String: tqltest.Strp("test"),
					},
				},
			},
		},
		{
			name: "too many args",
			inv: Invocation{
				Function: "testing_multiple_args",
				Arguments: []Value{
					{
						Path: &Path{
							Fields: []Field{
								{
									Name: "name",
								},
							},
						},
					},
					{
						String: tqltest.Strp("test"),
					},
					{
						String: tqltest.Strp("test"),
					},
				},
			},
		},
		{
			name: "not enough args with logger",
			inv: Invocation{
				Function: "testing_logger_first",
				Arguments: []Value{
					{
						String: tqltest.Strp("test"),
					},
					{
						String: tqltest.Strp("test"),
					},
				},
			},
		},
		{
			name: "too many args with logger",
			inv: Invocation{
				Function: "testing_logger_first",
				Arguments: []Value{
					{
						String: tqltest.Strp("test"),
					},
					{
						String: tqltest.Strp("test"),
					},
					{
						Int: tqltest.Intp(10),
					},
					{
						Int: tqltest.Intp(10),
					},
				},
			},
		},
		{
			name: "not matching arg type",
			inv: Invocation{
				Function: "testing_string",
				Arguments: []Value{
					{
						Int: tqltest.Intp(10),
					},
				},
			},
		},
		{
			name: "not matching arg type when byte slice",
			inv: Invocation{
				Function: "testing_byte_slice",
				Arguments: []Value{
					{
						String: tqltest.Strp("test"),
					},
					{
						String: tqltest.Strp("test"),
					},
					{
						String: tqltest.Strp("test"),
					},
				},
			},
		},
		{
			name: "function call returns error",
			inv: Invocation{
				Function: "testing_error",
			},
		},
		{
			name: "Enum not found",
			inv: Invocation{
				Function: "testing_enum",
				Arguments: []Value{
					{
						Enum: (*EnumSymbol)(tqltest.Strp("SYMBOL_NOT_FOUND")),
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := p.NewFunctionCall(tt.inv)
			assert.Error(t, err)
		})
	}
}

func Test_NewFunctionCall(t *testing.T) {
	p := NewParser(
		DefaultFunctionsForTests(),
		testParsePath,
		testParseEnum,
		NoOpLogger{},
	)

	tests := []struct {
		name string
		inv  Invocation
	}{
		{
			name: "string slice arg",
			inv: Invocation{
				Function: "testing_string_slice",
				Arguments: []Value{
					{
						String: tqltest.Strp("test"),
					},
					{
						String: tqltest.Strp("test"),
					},
					{
						String: tqltest.Strp("test"),
					},
				},
			},
		},
		{
			name: "float slice arg",
			inv: Invocation{
				Function: "testing_float_slice",
				Arguments: []Value{
					{
						Float: tqltest.Floatp(1.1),
					},
					{
						Float: tqltest.Floatp(1.2),
					},
					{
						Float: tqltest.Floatp(1.3),
					},
				},
			},
		},
		{
			name: "int slice arg",
			inv: Invocation{
				Function: "testing_int_slice",
				Arguments: []Value{
					{
						Int: tqltest.Intp(1),
					},
					{
						Int: tqltest.Intp(1),
					},
					{
						Int: tqltest.Intp(1),
					},
				},
			},
		},
		{
			name: "getter slice arg",
			inv: Invocation{
				Function: "testing_getter_slice",
				Arguments: []Value{
					{
						Path: &Path{
							Fields: []Field{
								{
									Name: "name",
								},
							},
						},
					},
					{
						String: tqltest.Strp("test"),
					},
					{
						Int: tqltest.Intp(1),
					},
					{
						Float: tqltest.Floatp(1.1),
					},
					{
						Bool: (*Boolean)(tqltest.Boolp(true)),
					},
					{
						Enum: (*EnumSymbol)(tqltest.Strp("TEST_ENUM")),
					},
					{
						Invocation: &Invocation{
							Function: "testing_getter",
							Arguments: []Value{
								{
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
		{
			name: "setter arg",
			inv: Invocation{
				Function: "testing_setter",
				Arguments: []Value{
					{
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
		{
			name: "getsetter arg",
			inv: Invocation{
				Function: "testing_getsetter",
				Arguments: []Value{
					{
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
		{
			name: "getter arg",
			inv: Invocation{
				Function: "testing_getter",
				Arguments: []Value{
					{
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
		{
			name: "getter arg with nil literal",
			inv: Invocation{
				Function: "testing_getter",
				Arguments: []Value{
					{
						IsNil: (*IsNil)(tqltest.Boolp(true)),
					},
				},
			},
		},
		{
			name: "string arg",
			inv: Invocation{
				Function: "testing_string",
				Arguments: []Value{
					{
						String: tqltest.Strp("test"),
					},
				},
			},
		},
		{
			name: "float arg",
			inv: Invocation{
				Function: "testing_float",
				Arguments: []Value{
					{
						Float: tqltest.Floatp(1.1),
					},
				},
			},
		},
		{
			name: "int arg",
			inv: Invocation{
				Function: "testing_int",
				Arguments: []Value{
					{
						Int: tqltest.Intp(1),
					},
				},
			},
		},
		{
			name: "bool arg",
			inv: Invocation{
				Function: "testing_bool",
				Arguments: []Value{
					{
						Bool: (*Boolean)(tqltest.Boolp(true)),
					},
				},
			},
		},
		{
			name: "bytes arg",
			inv: Invocation{
				Function: "testing_byte_slice",
				Arguments: []Value{
					{
						Bytes: (*Bytes)(&[]byte{1, 2, 3, 4, 5, 6, 7, 8}),
					},
				},
			},
		},
		{
			name: "multiple args",
			inv: Invocation{
				Function: "testing_multiple_args",
				Arguments: []Value{
					{
						Path: &Path{
							Fields: []Field{
								{
									Name: "name",
								},
							},
						},
					},
					{
						String: tqltest.Strp("test"),
					},
					{
						Float: tqltest.Floatp(1.1),
					},
					{
						Int: tqltest.Intp(1),
					},
					{
						String: tqltest.Strp("test"),
					},
					{
						String: tqltest.Strp("test"),
					},
				},
			},
		},
		{
			name: "Enum arg",
			inv: Invocation{
				Function: "testing_enum",
				Arguments: []Value{
					{
						Enum: (*EnumSymbol)(tqltest.Strp("TEST_ENUM")),
					},
				},
			},
		},
		{
			name: "logger first",
			inv: Invocation{
				Function: "testing_logger_first",
				Arguments: []Value{
					{
						String: tqltest.Strp("test0"),
					},
					{
						String: tqltest.Strp("test1"),
					},
					{
						Int: tqltest.Intp(1),
					},
				},
			},
		},
		{
			name: "logger middle",
			inv: Invocation{
				Function: "testing_logger_middle",
				Arguments: []Value{
					{
						String: tqltest.Strp("test0"),
					},
					{
						String: tqltest.Strp("test1"),
					},
					{
						Int: tqltest.Intp(1),
					},
				},
			},
		},
		{
			name: "logger last",
			inv: Invocation{
				Function: "testing_logger_last",
				Arguments: []Value{
					{
						String: tqltest.Strp("test0"),
					},
					{
						String: tqltest.Strp("test1"),
					},
					{
						Int: tqltest.Intp(1),
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := p.NewFunctionCall(tt.inv)
			assert.NoError(t, err)
		})
	}
}

func functionWithStringSlice(_ []string) (ExprFunc, error) {
	return func(ctx TransformContext) interface{} {
		return "anything"
	}, nil
}

func functionWithFloatSlice(_ []float64) (ExprFunc, error) {
	return func(ctx TransformContext) interface{} {
		return "anything"
	}, nil
}

func functionWithIntSlice(_ []int64) (ExprFunc, error) {
	return func(ctx TransformContext) interface{} {
		return "anything"
	}, nil
}

func functionWithByteSlice(_ []byte) (ExprFunc, error) {
	return func(ctx TransformContext) interface{} {
		return "anything"
	}, nil
}

func functionWithGetterSlice(_ []Getter) (ExprFunc, error) {
	return func(ctx TransformContext) interface{} {
		return "anything"
	}, nil
}

func functionWithSetter(_ Setter) (ExprFunc, error) {
	return func(ctx TransformContext) interface{} {
		return "anything"
	}, nil
}

func functionWithGetSetter(_ GetSetter) (ExprFunc, error) {
	return func(ctx TransformContext) interface{} {
		return "anything"
	}, nil
}

func functionWithGetter(_ Getter) (ExprFunc, error) {
	return func(ctx TransformContext) interface{} {
		return "anything"
	}, nil
}

func functionWithString(_ string) (ExprFunc, error) {
	return func(ctx TransformContext) interface{} {
		return "anything"
	}, nil
}

func functionWithFloat(_ float64) (ExprFunc, error) {
	return func(ctx TransformContext) interface{} {
		return "anything"
	}, nil
}

func functionWithInt(_ int64) (ExprFunc, error) {
	return func(ctx TransformContext) interface{} {
		return "anything"
	}, nil
}

func functionWithBool(_ bool) (ExprFunc, error) {
	return func(ctx TransformContext) interface{} {
		return "anything"
	}, nil
}

func functionWithMultipleArgs(_ GetSetter, _ string, _ float64, _ int64, _ []string) (ExprFunc, error) {
	return func(ctx TransformContext) interface{} {
		return "anything"
	}, nil
}

func functionThatHasAnError() (ExprFunc, error) {
	err := errors.New("testing")
	return func(ctx TransformContext) interface{} {
		return "anything"
	}, err
}

func functionWithEnum(_ Enum) (ExprFunc, error) {
	return func(ctx TransformContext) interface{} {
		return "anything"
	}, nil
}

func functionWithLoggerFirst(_ Logger, _ string, _ string, _ int64) (ExprFunc, error) {
	return func(ctx TransformContext) interface{} {
		return "anything"
	}, nil
}

func functionWithLoggerMiddle(_ string, _ string, _ Logger, _ int64) (ExprFunc, error) {
	return func(ctx TransformContext) interface{} {
		return "anything"
	}, nil
}

func functionWithLoggerLast(_ string, _ string, _ int64, _ Logger) (ExprFunc, error) {
	return func(ctx TransformContext) interface{} {
		return "anything"
	}, nil
}

func DefaultFunctionsForTests() map[string]interface{} {
	functions := make(map[string]interface{})
	functions["testing_string_slice"] = functionWithStringSlice
	functions["testing_float_slice"] = functionWithFloatSlice
	functions["testing_int_slice"] = functionWithIntSlice
	functions["testing_byte_slice"] = functionWithByteSlice
	functions["testing_getter_slice"] = functionWithGetterSlice
	functions["testing_setter"] = functionWithSetter
	functions["testing_getsetter"] = functionWithGetSetter
	functions["testing_getter"] = functionWithGetter
	functions["testing_string"] = functionWithString
	functions["testing_float"] = functionWithFloat
	functions["testing_int"] = functionWithInt
	functions["testing_bool"] = functionWithBool
	functions["testing_multiple_args"] = functionWithMultipleArgs
	functions["testing_enum"] = functionWithEnum
	functions["testing_logger_first"] = functionWithLoggerFirst
	functions["testing_logger_middle"] = functionWithLoggerMiddle
	functions["testing_logger_last"] = functionWithLoggerLast
	return functions
}
