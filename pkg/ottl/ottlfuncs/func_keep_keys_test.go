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
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_keepKeys(t *testing.T) {
	input := pcommon.NewMap()
	input.PutString("test", "hello world")
	input.PutInt("test2", 3)
	input.PutBool("test3", true)

	target := &ottl.StandardGetSetter[pcommon.Map]{
		Getter: func(ctx pcommon.Map) interface{} {
			return ctx
		},
		Setter: func(ctx pcommon.Map, val interface{}) {
			val.(pcommon.Map).CopyTo(ctx)
		},
	}

	tests := []struct {
		name   string
		target ottl.GetSetter[pcommon.Map]
		keys   []string
		want   func(pcommon.Map)
	}{
		{
			name:   "keep one",
			target: target,
			keys:   []string{"test"},
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutString("test", "hello world")
			},
		},
		{
			name:   "keep two",
			target: target,
			keys:   []string{"test", "test2"},
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutString("test", "hello world")
				expectedMap.PutInt("test2", 3)
			},
		},
		{
			name:   "keep none",
			target: target,
			keys:   []string{},
			want:   func(expectedMap pcommon.Map) {},
		},
		{
			name:   "no match",
			target: target,
			keys:   []string{"no match"},
			want:   func(expectedMap pcommon.Map) {},
		},
		{
			name:   "input is not a pcommon.Map",
			target: target,
			keys:   []string{"no match"},
			want:   func(expectedMap pcommon.Map) {},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scenarioMap := pcommon.NewMap()
			input.CopyTo(scenarioMap)

			exprFunc, err := KeepKeys(tt.target, tt.keys)
			require.NoError(t, err)
			exprFunc(scenarioMap)

			expected := pcommon.NewMap()
			tt.want(expected)

			assert.Equal(t, expected, scenarioMap)
		})
	}
}

func Test_keepKeys_bad_input(t *testing.T) {
	input := pcommon.NewValueString("not a map")
	target := &ottl.StandardGetSetter[interface{}]{
		Getter: func(ctx interface{}) interface{} {
			return ctx
		},
		Setter: func(ctx interface{}, val interface{}) {
			t.Errorf("nothing should be set in this scenario")
		},
	}

	keys := []string{"anything"}

	exprFunc, err := KeepKeys[interface{}](target, keys)
	require.NoError(t, err)
	exprFunc(input)

	assert.Equal(t, pcommon.NewValueString("not a map"), input)
}

func Test_keepKeys_get_nil(t *testing.T) {
	target := &ottl.StandardGetSetter[interface{}]{
		Getter: func(ctx interface{}) interface{} {
			return ctx
		},
		Setter: func(ctx interface{}, val interface{}) {
			t.Errorf("nothing should be set in this scenario")
		},
	}

	keys := []string{"anything"}

	exprFunc, err := KeepKeys[interface{}](target, keys)
	require.NoError(t, err)
	assert.Nil(t, exprFunc(nil))
}
