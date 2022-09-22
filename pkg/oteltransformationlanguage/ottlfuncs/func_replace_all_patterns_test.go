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

package ottlfuncs

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/oteltransformationlanguage/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/oteltransformationlanguage/ottl/ottltest"
)

func Test_replaceAllPatterns(t *testing.T) {
	input := pcommon.NewMap()
	input.PutString("test", "hello world")
	input.PutString("test2", "hello")
	input.PutString("test3", "goodbye world1 and world2")

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
		name        string
		target      ottl.GetSetter
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
				expectedMap.PutString("test", "hello {universe} world")
				expectedMap.PutString("test2", "hello {universe}")
				expectedMap.PutString("test3", "goodbye world1 and world2")
			},
		},
		{
			name:        "no matches",
			target:      target,
			pattern:     "nothing",
			replacement: "nothing {matches}",
			want: func(expectedMap pcommon.Map) {
				expectedMap.Clear()
				expectedMap.PutString("test", "hello world")
				expectedMap.PutString("test2", "hello")
				expectedMap.PutString("test3", "goodbye world1 and world2")
			},
		},
		{
			name:        "multiple regex match",
			target:      target,
			pattern:     `world[^\s]*(\s?)`,
			replacement: "**** ",
			want: func(expectedMap pcommon.Map) {
				expectedMap.Clear()
				expectedMap.PutString("test", "hello **** ")
				expectedMap.PutString("test2", "hello")
				expectedMap.PutString("test3", "goodbye **** and **** ")
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

			exprFunc, _ := ReplaceAllPatterns(tt.target, tt.pattern, tt.replacement)
			exprFunc(ctx)

			expected := pcommon.NewMap()
			tt.want(expected)

			assert.Equal(t, expected, scenarioMap)
		})
	}
}

func Test_replaceAllPatterns_bad_input(t *testing.T) {
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

	exprFunc, err := ReplaceAllPatterns(target, "regexpattern", "{replacement}")
	assert.Nil(t, err)

	exprFunc(ctx)

	assert.Equal(t, pcommon.NewValueString("not a map"), input)
}

func Test_replaceAllPatterns_get_nil(t *testing.T) {
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

	exprFunc, err := ReplaceAllPatterns(target, "regexp", "{anything}")
	assert.Nil(t, err)
	exprFunc(ctx)
}

func Test_replaceAllPatterns_invalid_pattern(t *testing.T) {
	target := &ottl.StandardGetSetter{
		Getter: func(ctx ottl.TransformContext) interface{} {
			t.Errorf("nothing should be received in this scenario")
			return nil
		},
		Setter: func(ctx ottl.TransformContext, val interface{}) {
			t.Errorf("nothing should be set in this scenario")
		},
	}

	invalidRegexPattern := "*"
	exprFunc, err := ReplaceAllPatterns(target, invalidRegexPattern, "{anything}")
	assert.Nil(t, exprFunc)
	assert.Contains(t, err.Error(), "error parsing regexp:")
}
