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

func Test_deleteMatchingKeys(t *testing.T) {
	input := pcommon.NewMap()
	input.InsertString("test", "hello world")
	input.InsertInt("test2", 3)
	input.InsertBool("test3", true)

	target := &tql.StandardGetSetter{
		Getter: func(ctx tql.TransformContext) interface{} {
			return ctx.GetItem()
		},
	}

	tests := []struct {
		name    string
		target  tql.Getter
		pattern string
		want    func(pcommon.Map)
	}{
		{
			name:    "delete everything",
			target:  target,
			pattern: "test.*",
			want: func(expectedMap pcommon.Map) {
				expectedMap.Clear()
				expectedMap.EnsureCapacity(3)
			},
		},
		{
			name:    "delete attributes that end in a number",
			target:  target,
			pattern: "\\d$",
			want: func(expectedMap pcommon.Map) {
				expectedMap.Clear()
				expectedMap.InsertString("test", "hello world")
			},
		},
		{
			name:    "delete nothing",
			target:  target,
			pattern: "not a matching pattern",
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

			exprFunc, _ := DeleteMatchingKeys(tt.target, tt.pattern)
			exprFunc(ctx)

			expected := pcommon.NewMap()
			tt.want(expected)

			assert.Equal(t, expected, scenarioMap)
		})
	}
}

func Test_deleteMatchingKeys_bad_input(t *testing.T) {
	input := pcommon.NewValueInt(1)
	ctx := tqltest.TestTransformContext{
		Item: input,
	}

	target := &tql.StandardGetSetter{
		Getter: func(ctx tql.TransformContext) interface{} {
			return ctx.GetItem()
		},
	}

	exprFunc, err := DeleteMatchingKeys(target, "anything")
	assert.Nil(t, err)
	exprFunc(ctx)

	assert.Equal(t, pcommon.NewValueInt(1), input)
}

func Test_deleteMatchingKeys_get_nil(t *testing.T) {
	ctx := tqltest.TestTransformContext{
		Item: nil,
	}

	target := &tql.StandardGetSetter{
		Getter: func(ctx tql.TransformContext) interface{} {
			return ctx.GetItem()
		},
	}

	exprFunc, _ := DeleteMatchingKeys(target, "anything")
	exprFunc(ctx)
}

func Test_deleteMatchingKeys_invalid_pattern(t *testing.T) {
	target := &tql.StandardGetSetter{
		Getter: func(ctx tql.TransformContext) interface{} {
			t.Errorf("nothing should be received in this scenario")
			return nil
		},
	}

	invalidRegexPattern := "*"
	exprFunc, err := DeleteMatchingKeys(target, invalidRegexPattern)
	assert.Nil(t, exprFunc)
	assert.Contains(t, err.Error(), "error parsing regexp:")
}
