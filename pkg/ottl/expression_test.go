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

package ottl

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottltest"
)

func hello() (ExprFunc, error) {
	return func(ctx TransformContext) interface{} {
		return "world"
	}, nil
}

func Test_newGetter(t *testing.T) {
	tests := []struct {
		name string
		val  Value
		want interface{}
	}{
		{
			name: "string literal",
			val: Value{
				String: ottltest.Strp("str"),
			},
			want: "str",
		},
		{
			name: "float literal",
			val: Value{
				Float: ottltest.Floatp(1.2),
			},
			want: 1.2,
		},
		{
			name: "int literal",
			val: Value{
				Int: ottltest.Intp(12),
			},
			want: int64(12),
		},
		{
			name: "bytes literal",
			val: Value{
				Bytes: (*Bytes)(&[]byte{1, 2, 3, 4, 5, 6, 7, 8}),
			},
			want: []byte{1, 2, 3, 4, 5, 6, 7, 8},
		},
		{
			name: "nil literal",
			val: Value{
				IsNil: (*IsNil)(ottltest.Boolp(true)),
			},
			want: nil,
		},
		{
			name: "bool literal",
			val: Value{
				Bool: (*Boolean)(ottltest.Boolp(true)),
			},
			want: true,
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
		{
			name: "enum",
			val: Value{
				Enum: (*EnumSymbol)(ottltest.Strp("TEST_ENUM_ONE")),
			},
			want: int64(1),
		},
	}

	functions := map[string]interface{}{"hello": hello}

	p := NewParser(
		functions,
		testParsePath,
		testParseEnum,
		component.TelemetrySettings{},
	)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader, err := p.newGetter(tt.val)
			assert.NoError(t, err)
			val := reader.Get(ottltest.TestTransformContext{
				Item: tt.want,
			})
			assert.Equal(t, tt.want, val)
		})
	}

	t.Run("empty value", func(t *testing.T) {
		_, err := p.newGetter(Value{})
		assert.Error(t, err)
	})
}

// pathGetSetter is a getSetter which has been resolved using a path expression provided by a user.
type testGetSetter struct {
	getter ExprFunc
	setter func(ctx TransformContext, val interface{})
}

func (path testGetSetter) Get(ctx TransformContext) interface{} {
	return path.getter(ctx)
}

func (path testGetSetter) Set(ctx TransformContext, val interface{}) {
	path.setter(ctx, val)
}
