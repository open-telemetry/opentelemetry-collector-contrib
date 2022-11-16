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

package ottlfuncs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_ParseJSONIntoMap(t *testing.T) {
	input := pcommon.NewMap()
	input.PutStr("existing", "attr")

	target := &ottl.StandardGetSetter[pcommon.Map]{
		Getter: func(ctx context.Context, tCtx pcommon.Map) (interface{}, error) {
			return tCtx, nil
		},
		Setter: func(ctx context.Context, tCtx pcommon.Map, val interface{}) error {
			val.(pcommon.Map).CopyTo(tCtx)
			return nil
		},
	}

	tests := []struct {
		name   string
		target ottl.GetSetter[pcommon.Map]
		value  ottl.Getter[pcommon.Map]
		want   func(pcommon.Map)
	}{
		{
			name:   "handle string",
			target: target,
			value: ottl.StandardGetSetter[pcommon.Map]{
				Getter: func(ctx context.Context, tCtx pcommon.Map) (interface{}, error) {
					return `{"test":"string value"}`, nil
				},
			},
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutStr("test", "string value")
			},
		},
		{
			name:   "handle bool",
			target: target,
			value: ottl.StandardGetSetter[pcommon.Map]{
				Getter: func(ctx context.Context, tCtx pcommon.Map) (interface{}, error) {
					return `{"test":true}`, nil
				},
			},
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutBool("test", true)
			},
		},
		{
			name:   "handle int",
			target: target,
			value: ottl.StandardGetSetter[pcommon.Map]{
				Getter: func(ctx context.Context, tCtx pcommon.Map) (interface{}, error) {
					return `{"test":1}`, nil
				},
			},
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutDouble("test", 1)
			},
		},
		{
			name:   "handle float",
			target: target,
			value: ottl.StandardGetSetter[pcommon.Map]{
				Getter: func(ctx context.Context, tCtx pcommon.Map) (interface{}, error) {
					return `{"test":1.1}`, nil
				},
			},
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutDouble("test", 1.1)
			},
		},
		{
			name:   "handle nil",
			target: target,
			value: ottl.StandardGetSetter[pcommon.Map]{
				Getter: func(ctx context.Context, tCtx pcommon.Map) (interface{}, error) {
					return `{"test":null}`, nil
				},
			},
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutEmpty("test")
			},
		},
		{
			name:   "handle array",
			target: target,
			value: ottl.StandardGetSetter[pcommon.Map]{
				Getter: func(ctx context.Context, tCtx pcommon.Map) (interface{}, error) {
					return `{"test":["string","value"]}`, nil
				},
			},
			want: func(expectedMap pcommon.Map) {
				emptySlice := expectedMap.PutEmptySlice("test")
				emptySlice.AppendEmpty().SetStr("string")
				emptySlice.AppendEmpty().SetStr("value")
			},
		},
		{
			name:   "handle nested object",
			target: target,
			value: ottl.StandardGetSetter[pcommon.Map]{
				Getter: func(ctx context.Context, tCtx pcommon.Map) (interface{}, error) {
					return `{"test":{"nested":"true"}}`, nil
				},
			},
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutStr("test", `{"nested":"true"}`)
			},
		},
		{
			name:   "updates existing",
			target: target,
			value: ottl.StandardGetSetter[pcommon.Map]{
				Getter: func(ctx context.Context, tCtx pcommon.Map) (interface{}, error) {
					return `{"existing":"pass"}`, nil
				},
			},
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutStr("existing", "pass")
			},
		},
		{
			name:   "complex",
			target: target,
			value: ottl.StandardGetSetter[pcommon.Map]{
				Getter: func(ctx context.Context, tCtx pcommon.Map) (interface{}, error) {
					return `{"test1":{"nested":"true"},"test2":"string","test3":1,"test4":1.1,"test5":["string", 1, [2, 3],{"nested":true}],"test6":null}`, nil
				},
			},
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutStr("test1", `{"nested":"true"}`)
				expectedMap.PutStr("test2", "string")
				expectedMap.PutDouble("test3", 1)
				expectedMap.PutDouble("test4", 1.1)
				slice := expectedMap.PutEmptySlice("test5")
				slice.AppendEmpty().SetStr("string")
				slice.AppendEmpty().SetDouble(1)
				nestedSlice := slice.AppendEmpty().SetEmptySlice()
				nestedSlice.AppendEmpty().SetDouble(2)
				nestedSlice.AppendEmpty().SetDouble(3)
				slice.AppendEmpty().SetStr(`{"nested":true}`)
				expectedMap.PutEmpty("test6")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scenarioMap := pcommon.NewMap()
			input.CopyTo(scenarioMap)

			exprFunc, err := ParseJSONIntoMap(tt.target, tt.value)
			assert.NoError(t, err)

			result, err := exprFunc(context.Background(), scenarioMap)
			assert.NoError(t, err)
			assert.Nil(t, result)

			expected := pcommon.NewMap()
			input.CopyTo(expected)
			tt.want(expected)

			assert.Equal(t, expected.Len(), scenarioMap.Len())
			expected.Range(func(k string, v pcommon.Value) bool {
				ev, _ := expected.Get(k)
				av, _ := scenarioMap.Get(k)
				assert.Equal(t, ev, av)
				return true
			})
		})
	}
}
