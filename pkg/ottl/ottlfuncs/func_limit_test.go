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
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottltest"
)

func Test_limit(t *testing.T) {
	input := pcommon.NewMap()
	input.PutString("test", "hello world")
	input.PutInt("test2", 3)
	input.PutBool("test3", true)

	target := &ottl.StandardGetSetter{
		Getter: func(ctx ottl.TransformContext) interface{} {
			return ctx.GetItem()
		},
		Setter: func(ctx ottl.TransformContext, val interface{}) {
			ctx.GetItem().(pcommon.Map).Clear()
			val.(pcommon.Map).CopyTo(ctx.GetItem().(pcommon.Map))
		},
	}

	tests := []struct {
		name   string
		target ottl.GetSetter
		limit  int64
		keep   []string
		want   func(pcommon.Map)
	}{
		{
			name:   "limit to 1",
			target: target,
			limit:  int64(1),
			want: func(expectedMap pcommon.Map) {
				expectedMap.Clear()
				expectedMap.PutString("test", "hello world")
			},
		},
		{
			name:   "limit to zero",
			target: target,
			limit:  int64(0),
			want: func(expectedMap pcommon.Map) {
				expectedMap.EnsureCapacity(input.Len())
			},
		},
		{
			name:   "limit nothing",
			target: target,
			limit:  int64(100),
			want: func(expectedMap pcommon.Map) {
				expectedMap.Clear()
				expectedMap.PutString("test", "hello world")
				expectedMap.PutInt("test2", 3)
				expectedMap.PutBool("test3", true)
			},
		},
		{
			name:   "limit exact",
			target: target,
			limit:  int64(3),
			want: func(expectedMap pcommon.Map) {
				expectedMap.Clear()
				expectedMap.PutString("test", "hello world")
				expectedMap.PutInt("test2", 3)
				expectedMap.PutBool("test3", true)
			},
		},
		{
			name:   "keep one key",
			target: target,
			limit:  int64(2),
			keep:   []string{"test3"},
			want: func(expectedMap pcommon.Map) {
				expectedMap.Clear()
				expectedMap.PutString("test", "hello world")
				expectedMap.PutBool("test3", true)
			},
		},
		{
			name:   "keep same # of keys as limit",
			target: target,
			limit:  int64(2),
			keep:   []string{"test", "test3"},
			want: func(expectedMap pcommon.Map) {
				expectedMap.Clear()
				expectedMap.PutString("test", "hello world")
				expectedMap.PutBool("test3", true)
			},
		},
		{
			name:   "keep not existing key",
			target: target,
			limit:  int64(1),
			keep:   []string{"te"},
			want: func(expectedMap pcommon.Map) {
				expectedMap.Clear()
				expectedMap.PutString("test", "hello world")
			},
		},
		{
			name:   "keep not-/existing keys",
			target: target,
			limit:  int64(2),
			keep:   []string{"te", "test3"},
			want: func(expectedMap pcommon.Map) {
				expectedMap.Clear()
				expectedMap.PutString("test", "hello world")
				expectedMap.PutBool("test3", true)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scenarioMap := pcommon.NewMap()
			input.CopyTo(scenarioMap)

			ctx := ottltest.TestTransformContext{
				Item: scenarioMap,
			}

			exprFunc, _ := Limit(tt.target, tt.limit, tt.keep)
			exprFunc(ctx)
			actual := ctx.GetItem()

			expected := pcommon.NewMap()
			tt.want(expected)

			assert.Equal(t, expected, actual)
		})
	}
}

func Test_limit_validation(t *testing.T) {
	tests := []struct {
		name   string
		target ottl.GetSetter
		keep   []string
		limit  int64
	}{
		{
			name:   "limit less than zero",
			target: &ottl.StandardGetSetter{},
			limit:  int64(-1),
		},
		{
			name:   "limit less than # of keep attrs",
			target: &ottl.StandardGetSetter{},
			keep:   []string{"test", "test"},
			limit:  int64(1),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Limit(tt.target, tt.limit, tt.keep)
			assert.NotNil(t, err)
		})
	}
}

func Test_limit_bad_input(t *testing.T) {
	input := pcommon.NewValueString("not a map")
	ctx := ottltest.TestTransformContext{
		Item: input,
	}

	target := &ottl.StandardGetSetter{
		Getter: func(ctx ottl.TransformContext) interface{} {
			return ctx.GetItem()
		},
		Setter: func(ctx ottl.TransformContext, val interface{}) {
			t.Errorf("nothing should be set in this scenario")
		},
	}

	exprFunc, _ := Limit(target, 1, []string{})
	exprFunc(ctx)

	assert.Equal(t, pcommon.NewValueString("not a map"), input)
}

func Test_limit_get_nil(t *testing.T) {
	ctx := ottltest.TestTransformContext{
		Item: nil,
	}

	target := &ottl.StandardGetSetter{
		Getter: func(ctx ottl.TransformContext) interface{} {
			return ctx.GetItem()
		},
		Setter: func(ctx ottl.TransformContext, val interface{}) {
			t.Errorf("nothing should be set in this scenario")
		},
	}

	exprFunc, _ := Limit(target, 1, []string{})
	exprFunc(ctx)
}
