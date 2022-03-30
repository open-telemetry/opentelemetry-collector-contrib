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
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/model/pdata"
)

func hello() ExprFunc {
	return func(ctx TransformContext) interface{} {
		return "world"
	}
}

func Test_newGetter(t *testing.T) {
	span := pdata.NewSpan()
	span.SetName("bear")
	tests := []struct {
		name string
		val  Value
		want interface{}
	}{
		{
			name: "string literal",
			val: Value{
				String: strp("str"),
			},
			want: "str",
		},
		{
			name: "float literal",
			val: Value{
				Float: floatp(1.2),
			},
			want: 1.2,
		},
		{
			name: "int literal",
			val: Value{
				Int: intp(12),
			},
			want: int64(12),
		},
		{
			name: "path expression",
			val: Value{
				Path: &Path{
					Fields: []Field{
						{
							Name: "name",
						},
					},
				},
			},
			want: "bear",
		},
		{
			name: "function call",
			val: Value{
				Invocation: &Invocation{
					Function: "hello",
				},
			},
			want: "world",
		},
	}

	functions := map[string]interface{}{"hello": hello}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader, err := NewGetter(tt.val, functions, testParsePath)
			assert.NoError(t, err)
			val := reader.Get(testTransformContext{
				span:     span,
				il:       pdata.NewInstrumentationLibrary(),
				resource: pdata.NewResource(),
			})
			assert.Equal(t, tt.want, val)
		})
	}

	t.Run("empty value", func(t *testing.T) {
		_, err := NewGetter(Value{}, functions, testParsePath)
		assert.Error(t, err)
	})
}
