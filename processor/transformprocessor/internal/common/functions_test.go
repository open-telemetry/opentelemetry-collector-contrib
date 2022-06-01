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

package common

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common/testhelper"
)

// Test for valid functions are in internal/traces/functions_test.go as there are many different data model cases.

func Test_newFunctionCall_invalid(t *testing.T) {
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
				Function: "set",
				Arguments: []Value{
					{
						String: testhelper.Strp("not path"),
					},
					{
						String: testhelper.Strp("cat"),
					},
				},
			},
		},
		{
			name: "not reader (invalid function)",
			inv: Invocation{
				Function: "set",
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
				Function: "set",
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
						Invocation: &Invocation{
							Function: "unknownfunc",
						},
					},
				},
			},
		},
		{
			name: "keep_keys not matching slice type",
			inv: Invocation{
				Function: "keep_keys",
				Arguments: []Value{
					{
						Path: &Path{
							Fields: []Field{
								{
									Name: "attributes",
								},
							},
						},
					},
					{
						Int: testhelper.Intp(10),
					},
				},
			},
		},
		{
			name: "truncate_all not int",
			inv: Invocation{
				Function: "truncate_all",
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
						String: testhelper.Strp("not an int"),
					},
				},
			},
		},
		{
			name: "truncate_all negative limit",
			inv: Invocation{
				Function: "truncate_all",
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
						Int: testhelper.Intp(-1),
					},
				},
			},
		},
		{
			name: "limit not int",
			inv: Invocation{
				Function: "limit",
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
						String: testhelper.Strp("not an int"),
					},
				},
			},
		},
		{
			name: "limit negative limit",
			inv: Invocation{
				Function: "limit",
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
						Int: testhelper.Intp(-1),
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
			name: "replace_match invalid pattern",
			inv: Invocation{
				Function: "replace_match",
				Arguments: []Value{
					{
						Path: &Path{
							Fields: []Field{
								{
									Name:   "attributes",
									MapKey: testhelper.Strp("test"),
								},
							},
						},
					},
					{
						String: testhelper.Strp("\\*"),
					},
					{
						String: testhelper.Strp("test"),
					},
				},
			},
		},
		{
			name: "replace_all_matches invalid pattern",
			inv: Invocation{
				Function: "replace_all_matches",
				Arguments: []Value{
					{
						Path: &Path{
							Fields: []Field{
								{
									Name: "attributes",
								},
							},
						},
					},
					{
						String: testhelper.Strp("\\*"),
					},
					{
						String: testhelper.Strp("test"),
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			functions := DefaultFunctions()
			functions["testing_error"] = functionThatHasAnError
			functions["testing_span_id"] = functionThatTakesASpanID
			functions["testing_trace_id"] = functionThatTakesATraceID

			_, err := NewFunctionCall(tt.inv, functions, testParsePath)
			assert.Error(t, err)
		})
	}
}

func functionThatHasAnError() (ExprFunc, error) {
	err := errors.New("testing")
	return func(ctx TransformContext) interface{} {
		return "anything"
	}, err
}

func Test_NewFunctionCall(t *testing.T) {
	spanID := pcommon.NewSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})
	traceID := pcommon.NewTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})

	functions := make(map[string]interface{})
	functions["testing_span_id"] = functionThatTakesASpanID
	functions["testing_trace_id"] = functionThatTakesATraceID

	tests := []struct {
		name string
		inv  Invocation
	}{
		{
			name: "span id argument",
			inv: Invocation{
				Function: "testing_span_id",
				Arguments: []Value{
					{
						SpanIDWrapper: &SpanIDWrapper{
							SpanID: &spanID,
						},
					},
				},
			},
		},
		{
			name: "trace id argument",
			inv: Invocation{
				Function: "testing_trace_id",
				Arguments: []Value{
					{
						TraceIDWrapper: &TraceIDWrapper{
							TraceID: &traceID,
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewFunctionCall(tt.inv, functions, testParsePath)
			assert.NoError(t, err)
		})
	}
}

func functionThatTakesASpanID(_ pcommon.SpanID) (ExprFunc, error) {
	return func(ctx TransformContext) interface{} {
		return "anything"
	}, nil
}

func functionThatTakesATraceID(_ pcommon.TraceID) (ExprFunc, error) {
	return func(ctx TransformContext) interface{} {
		return "anything"
	}, nil
}
