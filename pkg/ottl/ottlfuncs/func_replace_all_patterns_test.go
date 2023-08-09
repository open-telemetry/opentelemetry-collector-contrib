// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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

	target := &ottl.StandardPMapGetter[pcommon.Map]{
		Getter: func(ctx context.Context, tCtx pcommon.Map) (interface{}, error) {
			return tCtx, nil
		},
	}

	tests := []struct {
		name        string
		target      ottl.PMapGetter[pcommon.Map]
		mode        string
		pattern     string
		replacement ottl.StringGetter[pcommon.Map]
		want        func(pcommon.Map)
	}{
		{
			name:    "replace only matches",
			target:  target,
			mode:    modeValue,
			pattern: "hello",
			replacement: ottl.StandardStringGetter[pcommon.Map]{
				Getter: func(context.Context, pcommon.Map) (interface{}, error) {
					return "hello {universe}", nil
				},
			},
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
			name:    "no matches",
			target:  target,
			mode:    modeValue,
			pattern: "nothing",
			replacement: ottl.StandardStringGetter[pcommon.Map]{
				Getter: func(context.Context, pcommon.Map) (interface{}, error) {
					return "nothing {matches}", nil
				},
			},
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
			name:    "multiple regex match",
			target:  target,
			mode:    modeValue,
			pattern: `world[^\s]*(\s?)`,
			replacement: ottl.StandardStringGetter[pcommon.Map]{
				Getter: func(context.Context, pcommon.Map) (interface{}, error) {
					return "**** ", nil
				},
			},
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
			name:    "replace only matches",
			target:  target,
			mode:    modeKey,
			pattern: "test2",
			replacement: ottl.StandardStringGetter[pcommon.Map]{
				Getter: func(context.Context, pcommon.Map) (interface{}, error) {
					return "foo", nil
				},
			},
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
			name:    "no matches",
			target:  target,
			mode:    modeKey,
			pattern: "nothing",
			replacement: ottl.StandardStringGetter[pcommon.Map]{
				Getter: func(context.Context, pcommon.Map) (interface{}, error) {
					return "nothing {matches}", nil
				},
			},
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
			name:    "multiple regex match",
			target:  target,
			mode:    modeKey,
			pattern: `test`,
			replacement: ottl.StandardStringGetter[pcommon.Map]{
				Getter: func(context.Context, pcommon.Map) (interface{}, error) {
					return "test.", nil
				},
			},
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
			name:    "expand capturing groups in values",
			target:  target,
			mode:    modeValue,
			pattern: `world(\d)`,
			replacement: ottl.StandardStringGetter[pcommon.Map]{
				Getter: func(context.Context, pcommon.Map) (interface{}, error) {
					return "world-$1", nil
				},
			},
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
			name:    "expand capturing groups in keys",
			target:  target,
			mode:    modeKey,
			pattern: `test(\d)`,
			replacement: ottl.StandardStringGetter[pcommon.Map]{
				Getter: func(context.Context, pcommon.Map) (interface{}, error) {
					return "test-$1", nil
				},
			},
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutStr("test", "hello world")
				expectedMap.PutStr("test-2", "hello")
				expectedMap.PutStr("test-3", "goodbye world1 and world2")
				expectedMap.PutInt("test-4", 1234)
				expectedMap.PutDouble("test-5", 1234)
				expectedMap.PutBool("test-6", true)
			},
		},
		{
			name:    "replacement with literal $",
			target:  target,
			mode:    modeValue,
			pattern: `world(\d)`,
			replacement: ottl.StandardStringGetter[pcommon.Map]{
				Getter: func(context.Context, pcommon.Map) (interface{}, error) {
					return "$$world-$1", nil
				},
			},
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

	target := &ottl.StandardPMapGetter[interface{}]{
		Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
			return tCtx, nil
		},
	}
	replacement := &ottl.StandardStringGetter[interface{}]{
		Getter: func(context.Context, interface{}) (interface{}, error) {
			return "{replacement}", nil
		},
	}

	exprFunc, err := replaceAllPatterns[interface{}](target, modeValue, "regexpattern", replacement)
	assert.Nil(t, err)

	_, err = exprFunc(nil, input)
	assert.Error(t, err)
}

func Test_replaceAllPatterns_get_nil(t *testing.T) {
	target := &ottl.StandardPMapGetter[interface{}]{
		Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
			return tCtx, nil
		},
	}
	replacement := &ottl.StandardStringGetter[interface{}]{
		Getter: func(context.Context, interface{}) (interface{}, error) {
			return "{anything}", nil
		},
	}

	exprFunc, err := replaceAllPatterns[interface{}](target, modeValue, "regexp", replacement)
	assert.NoError(t, err)

	_, err = exprFunc(nil, nil)
	assert.Error(t, err)
}

func Test_replaceAllPatterns_invalid_pattern(t *testing.T) {
	target := &ottl.StandardPMapGetter[interface{}]{
		Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
			t.Errorf("nothing should be received in this scenario")
			return nil, nil
		},
	}
	replacement := &ottl.StandardStringGetter[interface{}]{
		Getter: func(context.Context, interface{}) (interface{}, error) {
			return "{anything}", nil
		},
	}

	invalidRegexPattern := "*"
	exprFunc, err := replaceAllPatterns[interface{}](target, modeValue, invalidRegexPattern, replacement)
	require.Error(t, err)
	assert.ErrorContains(t, err, "error parsing regexp:")
	assert.Nil(t, exprFunc)
}

func Test_replaceAllPatterns_invalid_model(t *testing.T) {
	target := &ottl.StandardPMapGetter[interface{}]{
		Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
			t.Errorf("nothing should be received in this scenario")
			return nil, nil
		},
	}
	replacement := &ottl.StandardStringGetter[interface{}]{
		Getter: func(context.Context, interface{}) (interface{}, error) {
			return "{anything}", nil
		},
	}

	invalidMode := "invalid"
	exprFunc, err := replaceAllPatterns[interface{}](target, invalidMode, "regex", replacement)
	assert.Nil(t, exprFunc)
	assert.Contains(t, err.Error(), "invalid mode")
}
