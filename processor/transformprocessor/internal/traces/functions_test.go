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

package traces

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/model/pdata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"
)

func Test_newFunctionCall(t *testing.T) {
	input := pdata.NewSpan()
	input.SetName("bear")
	attrs := pdata.NewAttributeMap()
	attrs.InsertString("test", "1")
	attrs.InsertInt("test2", 3)
	attrs.InsertBool("test3", true)
	attrs.CopyTo(input.Attributes())

	tests := []struct {
		name string
		inv  common.Invocation
		want func(pdata.Span)
	}{
		{
			name: "set name",
			inv: common.Invocation{
				Function: "set",
				Arguments: []common.Value{
					{
						Path: &common.Path{
							Fields: []common.Field{
								{
									Name: "name",
								},
							},
						},
					},
					{
						String: strp("cat"),
					},
				},
			},
			want: func(span pdata.Span) {
				input.CopyTo(span)
				span.SetName("cat")
			},
		},
		{
			name: "set status.code",
			inv: common.Invocation{
				Function: "set",
				Arguments: []common.Value{
					{
						Path: &common.Path{
							Fields: []common.Field{
								{
									Name: "status",
								},
								{
									Name: "code",
								},
							},
						},
					},
					{
						Int: intp(1),
					},
				},
			},
			want: func(span pdata.Span) {
				input.CopyTo(span)
				span.Status().SetCode(pdata.StatusCodeOk)
			},
		},
		{
			name: "keep_keys one",
			inv: common.Invocation{
				Function: "keep_keys",
				Arguments: []common.Value{
					{
						Path: &common.Path{
							Fields: []common.Field{
								{
									Name: "attributes",
								},
							},
						},
					},
					{
						String: strp("test"),
					},
				},
			},
			want: func(span pdata.Span) {
				input.CopyTo(span)
				span.Attributes().Clear()
				attrs := pdata.NewAttributeMap()
				attrs.InsertString("test", "1")
				attrs.CopyTo(span.Attributes())
			},
		},
		{
			name: "keep_keys two",
			inv: common.Invocation{
				Function: "keep_keys",
				Arguments: []common.Value{
					{
						Path: &common.Path{
							Fields: []common.Field{
								{
									Name: "attributes",
								},
							},
						},
					},
					{
						String: strp("test"),
					},
					{
						String: strp("test2"),
					},
				},
			},
			want: func(span pdata.Span) {
				input.CopyTo(span)
				span.Attributes().Clear()
				attrs := pdata.NewAttributeMap()
				attrs.InsertString("test", "1")
				attrs.InsertInt("test2", 3)
				attrs.CopyTo(span.Attributes())
			},
		},
		{
			name: "keep_keys none",
			inv: common.Invocation{
				Function: "keep_keys",
				Arguments: []common.Value{
					{
						Path: &common.Path{
							Fields: []common.Field{
								{
									Name: "attributes",
								},
							},
						},
					},
				},
			},
			want: func(span pdata.Span) {
				input.CopyTo(span)
				span.Attributes().Clear()
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			span := pdata.NewSpan()
			input.CopyTo(span)

			evaluate, err := newFunctionCall(tt.inv, DefaultFunctions())
			assert.NoError(t, err)
			evaluate(span, pdata.NewInstrumentationLibrary(), pdata.NewResource())

			expected := pdata.NewSpan()
			tt.want(expected)
			assert.Equal(t, expected, span)
		})
	}
}

func Test_newFunctionCall_invalid(t *testing.T) {
	tests := []struct {
		name string
		inv  common.Invocation
	}{
		{
			name: "unknown function",
			inv: common.Invocation{
				Function:  "unknownfunc",
				Arguments: []common.Value{},
			},
		},
		{
			name: "not trace accessor",
			inv: common.Invocation{
				Function: "set",
				Arguments: []common.Value{
					{
						String: strp("not path"),
					},
					{
						String: strp("cat"),
					},
				},
			},
		},
		{
			name: "not trace reader (invalid function)",
			inv: common.Invocation{
				Function: "set",
				Arguments: []common.Value{
					{
						Path: &common.Path{
							Fields: []common.Field{
								{
									Name: "name",
								},
							},
						},
					},
					{
						Invocation: &common.Invocation{
							Function: "unknownfunc",
						},
					},
				},
			},
		},
		{
			name: "not enough args",
			inv: common.Invocation{
				Function: "set",
				Arguments: []common.Value{
					{
						Path: &common.Path{
							Fields: []common.Field{
								{
									Name: "name",
								},
							},
						},
					},
					{
						Invocation: &common.Invocation{
							Function: "unknownfunc",
						},
					},
				},
			},
		},
		{
			name: "not matching slice type",
			inv: common.Invocation{
				Function: "keep_keys",
				Arguments: []common.Value{
					{
						Path: &common.Path{
							Fields: []common.Field{
								{
									Name: "attributes",
								},
							},
						},
					},
					{
						Int: intp(10),
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := newFunctionCall(tt.inv, DefaultFunctions())
			assert.Error(t, err)
		})
	}
}
