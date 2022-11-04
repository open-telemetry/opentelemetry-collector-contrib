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

func Test_keepKeys(t *testing.T) {
	input := pcommon.NewMap()
	input.PutStr("test", "hello world")
	input.PutInt("test2", 3)
	input.PutBool("test3", true)

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
		keys   []string
		want   func(pcommon.Map)
	}{
		{
			name:   "keep one",
			target: target,
			keys:   []string{"test"},
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutStr("test", "hello world")
			},
		},
		{
			name:   "keep two",
			target: target,
			keys:   []string{"test", "test2"},
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutStr("test", "hello world")
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
			assert.NoError(t, err)

			_, err = exprFunc(nil, scenarioMap)
			assert.Nil(t, err)

			expected := pcommon.NewMap()
			tt.want(expected)

			assert.Equal(t, expected, scenarioMap)
		})
	}
}

func Test_keepKeys_bad_input(t *testing.T) {
	input := pcommon.NewValueStr("not a map")
	target := &ottl.StandardGetSetter[interface{}]{
		Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
			return tCtx, nil
		},
		Setter: func(ctx context.Context, tCtx interface{}, val interface{}) error {
			t.Errorf("nothing should be set in this scenario")
			return nil
		},
	}

	keys := []string{"anything"}

	exprFunc, err := KeepKeys[interface{}](target, keys)
	assert.NoError(t, err)

	_, err = exprFunc(nil, input)
	assert.Nil(t, err)

	assert.Equal(t, pcommon.NewValueStr("not a map"), input)
}

func Test_keepKeys_get_nil(t *testing.T) {
	target := &ottl.StandardGetSetter[interface{}]{
		Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
			return tCtx, nil
		},
		Setter: func(ctx context.Context, tCtx interface{}, val interface{}) error {
			t.Errorf("nothing should be set in this scenario")
			return nil
		},
	}

	keys := []string{"anything"}

	exprFunc, err := KeepKeys[interface{}](target, keys)
	assert.NoError(t, err)
	result, err := exprFunc(nil, nil)
	assert.NoError(t, err)
	assert.Nil(t, result)
}
