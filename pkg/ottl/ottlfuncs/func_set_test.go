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
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_set(t *testing.T) {
	input := pcommon.NewValueStr("original name")

	target := &ottl.StandardGetSetter[pcommon.Value]{
		Setter: func(ctx pcommon.Value, val interface{}) error {
			ctx.SetStr(val.(string))
			return nil
		},
	}

	tests := []struct {
		name   string
		setter ottl.Setter[pcommon.Value]
		getter ottl.Getter[pcommon.Value]
		want   func(pcommon.Value)
	}{
		{
			name:   "set name",
			setter: target,
			getter: ottl.StandardGetSetter[pcommon.Value]{
				Getter: func(ctx pcommon.Value) (interface{}, error) {
					return "new name", nil
				},
			},
			want: func(expectedValue pcommon.Value) {
				expectedValue.SetStr("new name")
			},
		},
		{
			name:   "set nil value",
			setter: target,
			getter: ottl.StandardGetSetter[pcommon.Value]{
				Getter: func(ctx pcommon.Value) (interface{}, error) {
					return nil, nil
				},
			},
			want: func(expectedValue pcommon.Value) {
				expectedValue.SetStr("original name")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scenarioValue := pcommon.NewValueStr(input.Str())

			exprFunc, err := Set(tt.setter, tt.getter)
			assert.NoError(t, err)

			result, err := exprFunc(scenarioValue)
			assert.NoError(t, err)
			assert.Nil(t, result)

			expected := pcommon.NewValueStr("")
			tt.want(expected)

			assert.Equal(t, expected, scenarioValue)
		})
	}
}

func Test_set_get_nil(t *testing.T) {
	setter := &ottl.StandardGetSetter[interface{}]{
		Setter: func(ctx interface{}, val interface{}) error {
			t.Errorf("nothing should be set in this scenario")
			return nil
		},
	}

	getter := &ottl.StandardGetSetter[interface{}]{
		Getter: func(ctx interface{}) (interface{}, error) {
			return ctx, nil
		},
	}

	exprFunc, err := Set[interface{}](setter, getter)
	assert.NoError(t, err)

	result, err := exprFunc(nil)
	assert.NoError(t, err)
	assert.Nil(t, result)
}
