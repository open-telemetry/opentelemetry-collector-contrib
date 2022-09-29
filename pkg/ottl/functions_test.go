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
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottltest"
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
	functions["testing_telemetry_settings_first"] = functionWithTelemetrySettingsFirst

	p := NewParser(
		functions,
		testParsePath,
		testParseEnum,
		component.TelemetrySettings{},
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
						Invocation: &invocation{
							Function: "unknownfunc",
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
						Path: &Path{
							Fields: []Field{
								{
									Name: "name",
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
						Path: &Path{
							Fields: []Field{
								{
									Name: "name",
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
						String: ottltest.Strp("test"),
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
						String: ottltest.Strp("test"),
					},
					{
						Int: ottltest.Intp(10),
					},
					{
						Int: ottltest.Intp(10),
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
						Int: ottltest.Intp(10),
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := p.newFunctionCall(tt.inv)
			assert.Error(t, err)
		})
	}
}

func Test_NewFunctionCall(t *testing.T) {
	p := NewParser(
		defaultFunctionsForTests(),
		testParsePath,
		testParseEnum,
		component.TelemetrySettings{},
	)

	tests := []struct {
		name string
		inv  invocation
	}{
		{
			name: "string slice arg",
			inv: invocation{
				Function: "testing_string_slice",
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
			name: "float slice arg",
			inv: invocation{
				Function: "testing_float_slice",
				Arguments: []value{
					{
						Float: ottltest.Floatp(1.1),
					},
					{
						Float: ottltest.Floatp(1.2),
					},
					{
						Float: ottltest.Floatp(1.3),
					},
				},
			},
		},
		{
			name: "int slice arg",
			inv: invocation{
				Function: "testing_int_slice",
				Arguments: []value{
					{
						Int: ottltest.Intp(1),
					},
					{
						Int: ottltest.Intp(1),
					},
					{
						Int: ottltest.Intp(1),
					},
				},
			},
		},
		{
			name: "getter slice arg",
			inv: invocation{
				Function: "testing_getter_slice",
				Arguments: []value{
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
						String: ottltest.Strp("test"),
					},
					{
						Int: ottltest.Intp(1),
					},
					{
						Float: ottltest.Floatp(1.1),
					},
					{
						Bool: (*boolean)(ottltest.Boolp(true)),
					},
					{
						Enum: (*EnumSymbol)(ottltest.Strp("TEST_ENUM")),
					},
					{
						Invocation: &invocation{
							Function: "testing_getter",
							Arguments: []value{
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
			inv: invocation{
				Function: "testing_setter",
				Arguments: []value{
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
			inv: invocation{
				Function: "testing_getsetter",
				Arguments: []value{
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
			inv: invocation{
				Function: "testing_getter",
				Arguments: []value{
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
			inv: invocation{
				Function: "testing_getter",
				Arguments: []value{
					{
						IsNil: (*isNil)(ottltest.Boolp(true)),
					},
				},
			},
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
		},
		{
			name: "float arg",
			inv: invocation{
				Function: "testing_float",
				Arguments: []value{
					{
						Float: ottltest.Floatp(1.1),
					},
				},
			},
		},
		{
			name: "int arg",
			inv: invocation{
				Function: "testing_int",
				Arguments: []value{
					{
						Int: ottltest.Intp(1),
					},
				},
			},
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
		},
		{
			name: "multiple args",
			inv: invocation{
				Function: "testing_multiple_args",
				Arguments: []value{
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
						String: ottltest.Strp("test"),
					},
					{
						Float: ottltest.Floatp(1.1),
					},
					{
						Int: ottltest.Intp(1),
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
			name: "Enum arg",
			inv: invocation{
				Function: "testing_enum",
				Arguments: []value{
					{
						Enum: (*EnumSymbol)(ottltest.Strp("TEST_ENUM")),
					},
				},
			},
		},
		{
			name: "telemetrySettings first",
			inv: invocation{
				Function: "testing_telemetry_settings_first",
				Arguments: []value{
					{
						String: ottltest.Strp("test0"),
					},
					{
						String: ottltest.Strp("test1"),
					},
					{
						Int: ottltest.Intp(1),
					},
				},
			},
		},
		{
			name: "telemetrySettings middle",
			inv: invocation{
				Function: "testing_telemetry_settings_middle",
				Arguments: []value{
					{
						String: ottltest.Strp("test0"),
					},
					{
						String: ottltest.Strp("test1"),
					},
					{
						Int: ottltest.Intp(1),
					},
				},
			},
		},
		{
			name: "telemetrySettings last",
			inv: invocation{
				Function: "testing_telemetry_settings_last",
				Arguments: []value{
					{
						String: ottltest.Strp("test0"),
					},
					{
						String: ottltest.Strp("test1"),
					},
					{
						Int: ottltest.Intp(1),
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := p.newFunctionCall(tt.inv)
			assert.NoError(t, err)
		})
	}
}

func functionWithStringSlice(_ []string) (ExprFunc[interface{}], error) {
	return func(interface{}) interface{} {
		return "anything"
	}, nil
}

func functionWithFloatSlice(_ []float64) (ExprFunc[interface{}], error) {
	return func(interface{}) interface{} {
		return "anything"
	}, nil
}

func functionWithIntSlice(_ []int64) (ExprFunc[interface{}], error) {
	return func(interface{}) interface{} {
		return "anything"
	}, nil
}

func functionWithByteSlice([]byte) (ExprFunc[interface{}], error) {
	return func(interface{}) interface{} {
		return "anything"
	}, nil
}

func functionWithGetterSlice([]Getter[interface{}]) (ExprFunc[interface{}], error) {
	return func(interface{}) interface{} {
		return "anything"
	}, nil
}

func functionWithSetter(Setter[interface{}]) (ExprFunc[interface{}], error) {
	return func(interface{}) interface{} {
		return "anything"
	}, nil
}

func functionWithGetSetter(GetSetter[interface{}]) (ExprFunc[interface{}], error) {
	return func(interface{}) interface{} {
		return "anything"
	}, nil
}

func functionWithGetter(Getter[interface{}]) (ExprFunc[interface{}], error) {
	return func(interface{}) interface{} {
		return "anything"
	}, nil
}

func functionWithString(string) (ExprFunc[interface{}], error) {
	return func(interface{}) interface{} {
		return "anything"
	}, nil
}

func functionWithFloat(float64) (ExprFunc[interface{}], error) {
	return func(interface{}) interface{} {
		return "anything"
	}, nil
}

func functionWithInt(int64) (ExprFunc[interface{}], error) {
	return func(interface{}) interface{} {
		return "anything"
	}, nil
}

func functionWithBool(bool) (ExprFunc[interface{}], error) {
	return func(interface{}) interface{} {
		return "anything"
	}, nil
}

func functionWithMultipleArgs(GetSetter[interface{}], string, float64, int64, []string) (ExprFunc[interface{}], error) {
	return func(interface{}) interface{} {
		return "anything"
	}, nil
}

func functionThatHasAnError() (ExprFunc[interface{}], error) {
	err := errors.New("testing")
	return func(interface{}) interface{} {
		return "anything"
	}, err
}

func functionWithEnum(_ Enum) (ExprFunc[interface{}], error) {
	return func(interface{}) interface{} {
		return "anything"
	}, nil
}

func functionWithTelemetrySettingsFirst(_ component.TelemetrySettings, _ string, _ string, _ int64) (ExprFunc[interface{}], error) {
	return func(interface{}) interface{} {
		return "anything"
	}, nil
}

func functionWithTelemetrySettingsMiddle(_ string, _ string, _ component.TelemetrySettings, _ int64) (ExprFunc[interface{}], error) {
	return func(interface{}) interface{} {
		return "anything"
	}, nil
}

func functionWithTelemetrySettingsLast(_ string, _ string, _ int64, _ component.TelemetrySettings) (ExprFunc[interface{}], error) {
	return func(interface{}) interface{} {
		return "anything"
	}, nil
}

func defaultFunctionsForTests() map[string]interface{} {
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
	functions["testing_telemetry_settings_first"] = functionWithTelemetrySettingsFirst
	functions["testing_telemetry_settings_middle"] = functionWithTelemetrySettingsMiddle
	functions["testing_telemetry_settings_last"] = functionWithTelemetrySettingsLast
	return functions
}
