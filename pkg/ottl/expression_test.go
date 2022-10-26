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
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottltest"
)

func hello[K any]() (ExprFunc[K], error) {
	return func(ctx K) (interface{}, error) {
		return "world", nil
	}, nil
}

func Test_newGetter(t *testing.T) {
	tests := []struct {
		name string
		val  value
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
				Float: ottltest.Floatp(1.2),
			},
			want: 1.2,
		},
		{
			name: "int literal",
			val: value{
				Int: ottltest.Intp(12),
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
			val: value{
				Invocation: &invocation{
					Function: "hello",
				},
			},
			want: "world",
		},
		{
			name: "enum",
			val: value{
				Enum: (*EnumSymbol)(ottltest.Strp("TEST_ENUM_ONE")),
			},
			want: int64(1),
		},
	}

	functions := map[string]interface{}{"hello": hello[interface{}]}

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
			val, _ := reader.Get(tt.want)
			assert.Equal(t, tt.want, val)
		})
	}

	t.Run("empty value", func(t *testing.T) {
		_, err := p.newGetter(value{})
		assert.Error(t, err)
	})
}
