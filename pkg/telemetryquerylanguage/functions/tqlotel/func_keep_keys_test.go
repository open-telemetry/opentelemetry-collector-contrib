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

package tqlotel

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/telemetryquerylanguage/tql"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/telemetryquerylanguage/tql/tqltest"
)

func Test_keepKeys(t *testing.T) {
	input := pcommon.NewMap()
	input.UpsertString("test", "hello world")
	input.UpsertInt("test2", 3)
	input.UpsertBool("test3", true)

	target := &tql.StandardGetSetter{
		Getter: func(ctx tql.TransformContext) interface{} {
			return ctx.GetItem()
		},
		Setter: func(ctx tql.TransformContext, val interface{}) {
			ctx.GetItem().(pcommon.Map).Clear()
			val.(pcommon.Map).CopyTo(ctx.GetItem().(pcommon.Map))
		},
	}

	tests := []struct {
		name   string
		target tql.GetSetter
		keys   []string
		want   func(pcommon.Map)
	}{
		{
			name:   "keep one",
			target: target,
			keys:   []string{"test"},
			want: func(expectedMap pcommon.Map) {
				expectedMap.Clear()
				expectedMap.UpsertString("test", "hello world")
			},
		},
		{
			name:   "keep two",
			target: target,
			keys:   []string{"test", "test2"},
			want: func(expectedMap pcommon.Map) {
				expectedMap.Clear()
				expectedMap.UpsertString("test", "hello world")
				expectedMap.UpsertInt("test2", 3)
			},
		},
		{
			name:   "keep none",
			target: target,
			keys:   []string{},
			want: func(expectedMap pcommon.Map) {
				expectedMap.Clear()
			},
		},
		{
			name:   "no match",
			target: target,
			keys:   []string{"no match"},
			want: func(expectedMap pcommon.Map) {
				expectedMap.Clear()
			},
		},
		{
			name:   "input is not a pcommon.Map",
			target: target,
			keys:   []string{"no match"},
			want: func(expectedMap pcommon.Map) {
				expectedMap.Clear()
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scenarioMap := pcommon.NewMap()
			input.CopyTo(scenarioMap)

			ctx := tqltest.TestTransformContext{
				Item: scenarioMap,
			}

			exprFunc, _ := KeepKeys(tt.target, tt.keys)
			exprFunc(ctx)

			expected := pcommon.NewMap()
			tt.want(expected)

			assert.Equal(t, expected, scenarioMap)
		})
	}
}

func Test_keepKeys_bad_input(t *testing.T) {
	input := pcommon.NewValueString("not a map")
	ctx := tqltest.TestTransformContext{
		Item: input,
	}

	target := &tql.StandardGetSetter{
		Getter: func(ctx tql.TransformContext) interface{} {
			return ctx.GetItem()
		},
		Setter: func(ctx tql.TransformContext, val interface{}) {
			t.Errorf("nothing should be set in this scenario")
		},
	}

	keys := []string{"anything"}

	exprFunc, _ := KeepKeys(target, keys)
	exprFunc(ctx)

	assert.Equal(t, pcommon.NewValueString("not a map"), input)
}

func Test_keepKeys_get_nil(t *testing.T) {
	ctx := tqltest.TestTransformContext{
		Item: nil,
	}

	target := &tql.StandardGetSetter{
		Getter: func(ctx tql.TransformContext) interface{} {
			return ctx.GetItem()
		},
		Setter: func(ctx tql.TransformContext, val interface{}) {
			t.Errorf("nothing should be set in this scenario")
		},
	}

	keys := []string{"anything"}

	exprFunc, _ := KeepKeys(target, keys)
	exprFunc(ctx)
}
