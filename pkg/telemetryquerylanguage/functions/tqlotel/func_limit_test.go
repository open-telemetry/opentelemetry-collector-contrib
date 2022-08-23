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

func Test_limit(t *testing.T) {
	input := pcommon.NewMap()
	input.InsertString("test", "hello world")
	input.InsertInt("test2", 3)
	input.InsertBool("test3", true)

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
		limit  int64
		want   func(pcommon.Map)
	}{
		{
			name:   "limit to 1",
			target: target,
			limit:  int64(1),
			want: func(expectedMap pcommon.Map) {
				expectedMap.Clear()
				expectedMap.InsertString("test", "hello world")
			},
		},
		{
			name:   "limit to zero",
			target: target,
			limit:  int64(0),
			want: func(expectedMap pcommon.Map) {
				expectedMap.Clear()
			},
		},
		{
			name:   "limit nothing",
			target: target,
			limit:  int64(100),
			want: func(expectedMap pcommon.Map) {
				expectedMap.Clear()
				expectedMap.InsertString("test", "hello world")
				expectedMap.InsertInt("test2", 3)
				expectedMap.InsertBool("test3", true)
			},
		},
		{
			name:   "limit exact",
			target: target,
			limit:  int64(3),
			want: func(expectedMap pcommon.Map) {
				expectedMap.Clear()
				expectedMap.InsertString("test", "hello world")
				expectedMap.InsertInt("test2", 3)
				expectedMap.InsertBool("test3", true)
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

			exprFunc, _ := Limit(tt.target, tt.limit)
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
		target tql.GetSetter
		limit  int64
	}{
		{
			name:   "limit less than zero",
			target: &tql.StandardGetSetter{},
			limit:  int64(-1),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Limit(tt.target, tt.limit)
			assert.Error(t, err, "invalid limit for limit function, -1 cannot be negative")
		})
	}
}

func Test_limit_bad_input(t *testing.T) {
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

	exprFunc, _ := Limit(target, 1)
	exprFunc(ctx)

	assert.Equal(t, pcommon.NewValueString("not a map"), input)
}

func Test_limit_get_nil(t *testing.T) {
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

	exprFunc, _ := Limit(target, 1)
	exprFunc(ctx)
}
