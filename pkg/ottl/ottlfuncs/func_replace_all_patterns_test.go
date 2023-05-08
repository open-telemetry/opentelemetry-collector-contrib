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
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_replaceAllPatterns(t *testing.T) {
	input := pcommon.NewMap()
	input.PutStr("test", "hello world")
	input.PutStr("test2", "hello")
	input.PutStr("test3", "goodbye world1 and world2")
	input.PutInt("test4", 1234)
	input.PutDouble("test5", 1234)
	input.PutBool("test6", true)

	target := &ottl.StandardTypeGetter[pcommon.Map, pcommon.Map]{
		Getter: func(ctx context.Context, tCtx pcommon.Map) (interface{}, error) {
			return tCtx, nil
		},
	}

	tests := []struct {
		name        string
		target      ottl.PMapGetter[pcommon.Map]
		mode        string
		pattern     string
		replacement string
		want        func(pcommon.Map)
	}{
		{
			name:        "replace only matches",
			target:      target,
			mode:        modeValue,
			pattern:     "hello",
			replacement: "hello {universe}",
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutStr("test", "hello {universe} world")
				expectedMap.PutStr("test2", "hello {universe}")
				expectedMap.PutStr("test3", "goodbye world1 and world2")
				expectedMap.PutInt("test4", 1234)
				expectedMap.PutDouble("test5", 1234)
				expectedMap.PutBool("test6", true)
			},
		},
		{
			name:        "no matches",
			target:      target,
			mode:        modeValue,
			pattern:     "nothing",
			replacement: "nothing {matches}",
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutStr("test", "hello world")
				expectedMap.PutStr("test2", "hello")
				expectedMap.PutStr("test3", "goodbye world1 and world2")
				expectedMap.PutInt("test4", 1234)
				expectedMap.PutDouble("test5", 1234)
				expectedMap.PutBool("test6", true)
			},
		},
		{
			name:        "multiple regex match",
			target:      target,
			mode:        modeValue,
			pattern:     `world[^\s]*(\s?)`,
			replacement: "**** ",
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutStr("test", "hello **** ")
				expectedMap.PutStr("test2", "hello")
				expectedMap.PutStr("test3", "goodbye **** and **** ")
				expectedMap.PutInt("test4", 1234)
				expectedMap.PutDouble("test5", 1234)
				expectedMap.PutBool("test6", true)
			},
		},
		{
			name:        "replace only matches",
			target:      target,
			mode:        modeKey,
			pattern:     "test2",
			replacement: "foo",
			want: func(expectedMap pcommon.Map) {
				expectedMap.Clear()
				expectedMap.PutStr("test", "hello world")
				expectedMap.PutStr("foo", "hello")
				expectedMap.PutStr("test3", "goodbye world1 and world2")
				expectedMap.PutInt("test4", 1234)
				expectedMap.PutDouble("test5", 1234)
				expectedMap.PutBool("test6", true)
			},
		},
		{
			name:        "no matches",
			target:      target,
			mode:        modeKey,
			pattern:     "nothing",
			replacement: "nothing {matches}",
			want: func(expectedMap pcommon.Map) {
				expectedMap.Clear()
				expectedMap.PutStr("test", "hello world")
				expectedMap.PutStr("test2", "hello")
				expectedMap.PutStr("test3", "goodbye world1 and world2")
				expectedMap.PutInt("test4", 1234)
				expectedMap.PutDouble("test5", 1234)
				expectedMap.PutBool("test6", true)
			},
		},
		{
			name:        "multiple regex match",
			target:      target,
			mode:        modeKey,
			pattern:     `test`,
			replacement: "test.",
			want: func(expectedMap pcommon.Map) {
				expectedMap.Clear()
				expectedMap.PutStr("test.", "hello world")
				expectedMap.PutStr("test.2", "hello")
				expectedMap.PutStr("test.3", "goodbye world1 and world2")
				expectedMap.PutInt("test.4", 1234)
				expectedMap.PutDouble("test.5", 1234)
				expectedMap.PutBool("test.6", true)
			},
		},
		{
			name:        "expand capturing groups",
			target:      target,
			mode:        modeValue,
			pattern:     `world(\d)`,
			replacement: "world-$1",
			want: func(expectedMap pcommon.Map) {
				expectedMap.Clear()
				expectedMap.PutStr("test", "hello world")
				expectedMap.PutStr("test2", "hello")
				expectedMap.PutStr("test3", "goodbye world-1 and world-2")
				expectedMap.PutInt("test4", 1234)
				expectedMap.PutDouble("test5", 1234)
				expectedMap.PutBool("test6", true)
			},
		},
		{
			name:        "replacement with literal $",
			target:      target,
			mode:        modeValue,
			pattern:     `world(\d)`,
			replacement: "$$world-$1",
			want: func(expectedMap pcommon.Map) {
				expectedMap.Clear()
				expectedMap.PutStr("test", "hello world")
				expectedMap.PutStr("test2", "hello")
				expectedMap.PutStr("test3", "goodbye $world-1 and $world-2")
				expectedMap.PutInt("test4", 1234)
				expectedMap.PutDouble("test5", 1234)
				expectedMap.PutBool("test6", true)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scenarioMap := pcommon.NewMap()
			input.CopyTo(scenarioMap)

			exprFunc, err := replaceAllPatterns[pcommon.Map](tt.target, tt.mode, tt.pattern, tt.replacement)
			assert.NoError(t, err)

			_, err = exprFunc(nil, scenarioMap)
			assert.Nil(t, err)

			expected := pcommon.NewMap()
			tt.want(expected)

			assert.Equal(t, expected, scenarioMap)
		})
	}
}

func Test_replaceAllPatterns_bad_input(t *testing.T) {
	input := pcommon.NewValueStr("not a map")

	target := &ottl.StandardTypeGetter[interface{}, pcommon.Map]{
		Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
			return tCtx, nil
		},
	}

	exprFunc, err := replaceAllPatterns[interface{}](target, modeValue, "regexpattern", "{replacement}")
	assert.Nil(t, err)

	_, err = exprFunc(nil, input)
	assert.Error(t, err)
}

func Test_replaceAllPatterns_get_nil(t *testing.T) {
	target := &ottl.StandardTypeGetter[interface{}, pcommon.Map]{
		Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
			return tCtx, nil
		},
	}

	exprFunc, err := replaceAllPatterns[interface{}](target, modeValue, "regexp", "{anything}")
	assert.NoError(t, err)

	_, err = exprFunc(nil, nil)
	assert.Error(t, err)
}

func Test_replaceAllPatterns_invalid_pattern(t *testing.T) {
	target := &ottl.StandardTypeGetter[interface{}, pcommon.Map]{
		Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
			t.Errorf("nothing should be received in this scenario")
			return nil, nil
		},
	}

	invalidRegexPattern := "*"
	exprFunc, err := replaceAllPatterns[interface{}](target, modeValue, invalidRegexPattern, "{anything}")
	require.Error(t, err)
	assert.ErrorContains(t, err, "error parsing regexp:")
	assert.Nil(t, exprFunc)
}

func Test_replaceAllPatterns_invalid_model(t *testing.T) {
	target := &ottl.StandardTypeGetter[interface{}, pcommon.Map]{
		Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
			t.Errorf("nothing should be received in this scenario")
			return nil, nil
		},
	}

	invalidMode := "invalid"
	exprFunc, err := replaceAllPatterns[interface{}](target, invalidMode, "regex", "{anything}")
	assert.Nil(t, exprFunc)
	assert.Contains(t, err.Error(), "invalid mode")
}
