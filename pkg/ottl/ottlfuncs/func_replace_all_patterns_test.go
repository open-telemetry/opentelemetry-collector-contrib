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

func Test_replaceAllPatterns(t *testing.T) {
	input := pcommon.NewMap()
	input.PutString("test", "hello world")
	input.PutString("test2", "hello")
	input.PutString("test3", "goodbye world1 and world2")

	target := &ottl.StandardGetSetter[pcommon.Map]{
		Getter: func(ctx pcommon.Map) interface{} {
			return ctx
		},
		Setter: func(ctx pcommon.Map, val interface{}) {
			ctx.Clear()
			val.(pcommon.Map).CopyTo(ctx)
		},
	}

	tests := []struct {
		name        string
		target      ottl.GetSetter[pcommon.Map]
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

			exprFunc, err := ReplaceAllPatterns[pcommon.Map](tt.target, tt.pattern, tt.replacement)
			require.NoError(t, err)
			exprFunc(scenarioMap)

			expected := pcommon.NewMap()
			tt.want(expected)

			assert.Equal(t, expected, scenarioMap)
		})
	}
}

func Test_replaceAllPatterns_bad_input(t *testing.T) {
	input := pcommon.NewValueString("not a map")

	target := &ottl.StandardGetSetter[interface{}]{
		Getter: func(ctx interface{}) interface{} {
			return ctx
		},
		Setter: func(ctx interface{}, val interface{}) {
			t.Errorf("nothing should be set in this scenario")
		},
	}

	exprFunc, err := ReplaceAllPatterns[interface{}](target, "regexpattern", "{replacement}")
	assert.Nil(t, err)

	exprFunc(input)

	assert.Equal(t, pcommon.NewValueString("not a map"), input)
}

func Test_replaceAllPatterns_get_nil(t *testing.T) {
	target := &ottl.StandardGetSetter[interface{}]{
		Getter: func(ctx interface{}) interface{} {
			return ctx
		},
		Setter: func(ctx interface{}, val interface{}) {
			t.Errorf("nothing should be set in this scenario")
		},
	}

	exprFunc, err := ReplaceAllPatterns[interface{}](target, "regexp", "{anything}")
	require.NoError(t, err)
	exprFunc(nil)
}

func Test_replaceAllPatterns_invalid_pattern(t *testing.T) {
	target := &ottl.StandardGetSetter[interface{}]{
		Getter: func(ctx interface{}) interface{} {
			t.Errorf("nothing should be received in this scenario")
			return nil
		},
		Setter: func(ctx interface{}, val interface{}) {
			t.Errorf("nothing should be set in this scenario")
		},
	}

	invalidRegexPattern := "*"
	exprFunc, err := ReplaceAllPatterns[interface{}](target, invalidRegexPattern, "{anything}")
	require.Error(t, err)
	assert.ErrorContains(t, err, "error parsing regexp:")
	assert.Nil(t, exprFunc)
}
