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
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common/testhelper"
)

func Test_replaceAllPatterns(t *testing.T) {
	input := pcommon.NewMap()
	input.InsertString("test", "hello world")
	input.InsertString("test2", "hello")
	input.InsertString("test3", "goodbye world1 and world2")

	target := &testGetSetter{
		getter: func(ctx TransformContext) interface{} {
			return ctx.GetItem()
		},
		setter: func(ctx TransformContext, val interface{}) {
			ctx.GetItem().(pcommon.Map).Clear()
			val.(pcommon.Map).CopyTo(ctx.GetItem().(pcommon.Map))
		},
	}

	tests := []struct {
		name        string
		target      GetSetter
		pattern     string
		replacement string
		want        func(pcommon.Map)
	}{
		{
			name:        "replace only matches",
			target:      target,
			pattern:     "hello",
			replacement: "hello {universe}",
			want: func(expectedMap pcommon.Map) {
				expectedMap.Clear()
				expectedMap.InsertString("test", "hello {universe} world")
				expectedMap.InsertString("test2", "hello {universe}")
				expectedMap.InsertString("test3", "goodbye world1 and world2")
			},
		},
		{
			name:        "no matches",
			target:      target,
			pattern:     "nothing",
			replacement: "nothing {matches}",
			want: func(expectedMap pcommon.Map) {
				expectedMap.Clear()
				expectedMap.InsertString("test", "hello world")
				expectedMap.InsertString("test2", "hello")
				expectedMap.InsertString("test3", "goodbye world1 and world2")
			},
		},
		{
			name:        "multiple regex match",
			target:      target,
			pattern:     `world[^\s]*(\s?)`,
			replacement: "**** ",
			want: func(expectedMap pcommon.Map) {
				expectedMap.Clear()
				expectedMap.InsertString("test", "hello **** ")
				expectedMap.InsertString("test2", "hello")
				expectedMap.InsertString("test3", "goodbye **** and **** ")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scenarioMap := pcommon.NewMap()
			input.CopyTo(scenarioMap)

			ctx := testhelper.TestTransformContext{
				Item: scenarioMap,
			}

			exprFunc, _ := replaceAllPatterns(tt.target, tt.pattern, tt.replacement)
			exprFunc(ctx)

			expected := pcommon.NewMap()
			tt.want(expected)

			assert.Equal(t, expected, scenarioMap)
		})
	}
}

func Test_replaceAllPatterns_bad_input(t *testing.T) {
	input := pcommon.NewValueString("not a map")
	ctx := testhelper.TestTransformContext{
		Item: input,
	}

	target := &testGetSetter{
		getter: func(ctx TransformContext) interface{} {
			return ctx.GetItem()
		},
		setter: func(ctx TransformContext, val interface{}) {
			t.Errorf("nothing should be set in this scenario")
		},
	}

	exprFunc, err := replaceAllPatterns(target, "regexpattern", "{replacement}")
	assert.Nil(t, err)

	exprFunc(ctx)

	assert.Equal(t, pcommon.NewValueString("not a map"), input)
}

func Test_replaceAllPatterns_get_nil(t *testing.T) {
	ctx := testhelper.TestTransformContext{
		Item: nil,
	}

	target := &testGetSetter{
		getter: func(ctx TransformContext) interface{} {
			return ctx.GetItem()
		},
		setter: func(ctx TransformContext, val interface{}) {
			t.Errorf("nothing should be set in this scenario")
		},
	}

	exprFunc, err := replaceAllPatterns(target, "regexp", "{anything}")
	assert.Nil(t, err)
	exprFunc(ctx)
}

func Test_replaceAllPatterns_invalid_pattern(t *testing.T) {
	target := &testGetSetter{
		getter: func(ctx TransformContext) interface{} {
			t.Errorf("nothing should be received in this scenario")
			return nil
		},
		setter: func(ctx TransformContext, val interface{}) {
			t.Errorf("nothing should be set in this scenario")
		},
	}

	invalidRegexPattern := "*"
	exprFunc, err := replaceAllPatterns(target, invalidRegexPattern, "{anything}")
	assert.Nil(t, exprFunc)
	assert.Contains(t, err.Error(), "error parsing regexp:")
}
