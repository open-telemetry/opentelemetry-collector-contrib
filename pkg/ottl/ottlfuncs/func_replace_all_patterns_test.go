// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
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

	ottlValue := ottl.StandardFunctionGetter[pcommon.Map]{
		FCtx: ottl.FunctionContext{
			Set: componenttest.NewNopTelemetrySettings(),
		},
		Fact: optionalFnTestFactory[pcommon.Map](),
	}
	optionalArg := ottl.NewTestingOptional[ottl.FunctionGetter[pcommon.Map]](ottlValue)

	target := &ottl.StandardPMapGetter[pcommon.Map]{
		Getter: func(ctx context.Context, tCtx pcommon.Map) (any, error) {
			return tCtx, nil
		},
	}

	tests := []struct {
		name        string
		target      ottl.PMapGetter[pcommon.Map]
		mode        string
		pattern     string
		replacement ottl.StringGetter[pcommon.Map]
		function    ottl.Optional[ottl.FunctionGetter[pcommon.Map]]
		want        func(pcommon.Map)
	}{
		{
			name:    "replace only matches (with hash function)",
			target:  target,
			mode:    modeValue,
			pattern: "hello",
			replacement: ottl.StandardStringGetter[pcommon.Map]{
				Getter: func(context.Context, pcommon.Map) (any, error) {
					return "hello {universe}", nil
				},
			},
			function: optionalArg,
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutStr("test", "hash(hello {universe}) world")
				expectedMap.PutStr("test2", "hash(hello {universe})")
				expectedMap.PutStr("test3", "goodbye world1 and world2")
				expectedMap.PutInt("test4", 1234)
				expectedMap.PutDouble("test5", 1234)
				expectedMap.PutBool("test6", true)
			},
		},
		{
			name:    "replace only matches (with capture group and hash function)",
			target:  target,
			mode:    modeValue,
			pattern: "(hello)",
			replacement: ottl.StandardStringGetter[pcommon.Map]{
				Getter: func(context.Context, pcommon.Map) (any, error) {
					return "$1", nil
				},
			},
			function: optionalArg,
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutStr("test", "hash(hello) world")
				expectedMap.PutStr("test2", "hash(hello)")
				expectedMap.PutStr("test3", "goodbye world1 and world2")
				expectedMap.PutInt("test4", 1234)
				expectedMap.PutDouble("test5", 1234)
				expectedMap.PutBool("test6", true)
			},
		},
		{
			name:    "replace only matches (no capture group and with hash function)",
			target:  target,
			mode:    modeValue,
			pattern: "hello",
			replacement: ottl.StandardStringGetter[pcommon.Map]{
				Getter: func(context.Context, pcommon.Map) (any, error) {
					return "$1", nil
				},
			},
			function: optionalArg,
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutStr("test", "hash() world")
				expectedMap.PutStr("test2", "hash()")
				expectedMap.PutStr("test3", "goodbye world1 and world2")
				expectedMap.PutInt("test4", 1234)
				expectedMap.PutDouble("test5", 1234)
				expectedMap.PutBool("test6", true)
			},
		},
		{
			name:    "replace only matches (no capture group or hash function)",
			target:  target,
			mode:    modeValue,
			pattern: "hello",
			replacement: ottl.StandardStringGetter[pcommon.Map]{
				Getter: func(context.Context, pcommon.Map) (any, error) {
					return "$1", nil
				},
			},
			function: ottl.Optional[ottl.FunctionGetter[pcommon.Map]]{},
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutStr("test", " world")
				expectedMap.PutStr("test2", "")
				expectedMap.PutStr("test3", "goodbye world1 and world2")
				expectedMap.PutInt("test4", 1234)
				expectedMap.PutDouble("test5", 1234)
				expectedMap.PutBool("test6", true)
			},
		},
		{
			name:    "replace only matches",
			target:  target,
			mode:    modeValue,
			pattern: "hello",
			replacement: ottl.StandardStringGetter[pcommon.Map]{
				Getter: func(context.Context, pcommon.Map) (any, error) {
					return "hello {universe}", nil
				},
			},
			function: ottl.Optional[ottl.FunctionGetter[pcommon.Map]]{},
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
				Getter: func(context.Context, pcommon.Map) (any, error) {
					return "nothing {matches}", nil
				},
			},
			function: ottl.Optional[ottl.FunctionGetter[pcommon.Map]]{},
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
				Getter: func(context.Context, pcommon.Map) (any, error) {
					return "**** ", nil
				},
			},
			function: ottl.Optional[ottl.FunctionGetter[pcommon.Map]]{},
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
			name:    "regex match (with multiple capture groups)",
			target:  target,
			mode:    modeValue,
			pattern: `(world1) and (world2)`,
			replacement: ottl.StandardStringGetter[pcommon.Map]{
				Getter: func(context.Context, pcommon.Map) (any, error) {
					return "blue-$1 and blue-$2", nil
				},
			},
			function: ottl.Optional[ottl.FunctionGetter[pcommon.Map]]{},
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutStr("test", "hello world")
				expectedMap.PutStr("test2", "hello")
				expectedMap.PutStr("test3", "goodbye blue-world1 and blue-world2")
				expectedMap.PutInt("test4", 1234)
				expectedMap.PutDouble("test5", 1234)
				expectedMap.PutBool("test6", true)
			},
		},
		{
			name:    "regex match (with multiple matches from one capture group)",
			target:  target,
			mode:    modeValue,
			pattern: `(world\d)`,
			replacement: ottl.StandardStringGetter[pcommon.Map]{
				Getter: func(context.Context, pcommon.Map) (any, error) {
					return "blue-$1", nil
				},
			},
			function: ottl.Optional[ottl.FunctionGetter[pcommon.Map]]{},
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutStr("test", "hello world")
				expectedMap.PutStr("test2", "hello")
				expectedMap.PutStr("test3", "goodbye blue-world1 and blue-world2")
				expectedMap.PutInt("test4", 1234)
				expectedMap.PutDouble("test5", 1234)
				expectedMap.PutBool("test6", true)
			},
		},
		{
			name:    "regex match (with multiple capture groups and hash function)",
			target:  target,
			mode:    modeValue,
			pattern: `(world1) and (world2)`,
			replacement: ottl.StandardStringGetter[pcommon.Map]{
				Getter: func(context.Context, pcommon.Map) (any, error) {
					return "$1", nil
				},
			},
			function: optionalArg,
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutStr("test", "hello world")
				expectedMap.PutStr("test2", "hello")
				expectedMap.PutStr("test3", "goodbye hash(world1)")
				expectedMap.PutInt("test4", 1234)
				expectedMap.PutDouble("test5", 1234)
				expectedMap.PutBool("test6", true)
			},
		},
		{
			name:    "regex match (with multiple capture groups and hash function)",
			target:  target,
			mode:    modeValue,
			pattern: `(world1) and (world2)`,
			replacement: ottl.StandardStringGetter[pcommon.Map]{
				Getter: func(context.Context, pcommon.Map) (any, error) {
					return "$2", nil
				},
			},
			function: optionalArg,
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutStr("test", "hello world")
				expectedMap.PutStr("test2", "hello")
				expectedMap.PutStr("test3", "goodbye hash(world2)")
				expectedMap.PutInt("test4", 1234)
				expectedMap.PutDouble("test5", 1234)
				expectedMap.PutBool("test6", true)
			},
		},
		{
			name:    "regex match (with multiple matches from one capture group and hash function)",
			target:  target,
			mode:    modeValue,
			pattern: `(world\d)`,
			replacement: ottl.StandardStringGetter[pcommon.Map]{
				Getter: func(context.Context, pcommon.Map) (any, error) {
					return "$1", nil
				},
			},
			function: optionalArg,
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutStr("test", "hello world")
				expectedMap.PutStr("test2", "hello")
				expectedMap.PutStr("test3", "goodbye hash(world1) and hash(world2)")
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
				Getter: func(context.Context, pcommon.Map) (any, error) {
					return "foo", nil
				},
			},
			function: ottl.Optional[ottl.FunctionGetter[pcommon.Map]]{},
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
				Getter: func(context.Context, pcommon.Map) (any, error) {
					return "nothing {matches}", nil
				},
			},
			function: ottl.Optional[ottl.FunctionGetter[pcommon.Map]]{},
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
				Getter: func(context.Context, pcommon.Map) (any, error) {
					return "test.", nil
				},
			},
			function: ottl.Optional[ottl.FunctionGetter[pcommon.Map]]{},
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
				Getter: func(context.Context, pcommon.Map) (any, error) {
					return "world-$1", nil
				},
			},
			function: ottl.Optional[ottl.FunctionGetter[pcommon.Map]]{},
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
				Getter: func(context.Context, pcommon.Map) (any, error) {
					return "test-$1", nil
				},
			},
			function: ottl.Optional[ottl.FunctionGetter[pcommon.Map]]{},
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
				Getter: func(context.Context, pcommon.Map) (any, error) {
					return "$$world-$1", nil
				},
			},
			function: ottl.Optional[ottl.FunctionGetter[pcommon.Map]]{},
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

			exprFunc, err := replaceAllPatterns[pcommon.Map](tt.target, tt.mode, tt.pattern, tt.replacement, tt.function)
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
	target := &ottl.StandardPMapGetter[any]{
		Getter: func(ctx context.Context, tCtx any) (any, error) {
			return tCtx, nil
		},
	}
	replacement := &ottl.StandardStringGetter[any]{
		Getter: func(context.Context, any) (any, error) {
			return "{replacement}", nil
		},
	}
	function := ottl.Optional[ottl.FunctionGetter[any]]{}

	exprFunc, err := replaceAllPatterns[any](target, modeValue, "regexpattern", replacement, function)
	assert.Nil(t, err)

	_, err = exprFunc(nil, input)
	assert.Error(t, err)
}

func Test_replaceAllPatterns_bad_function_input(t *testing.T) {
	input := pcommon.NewValueInt(1)
	target := &ottl.StandardPMapGetter[any]{
		Getter: func(ctx context.Context, tCtx any) (any, error) {
			return tCtx, nil
		},
	}
	replacement := &ottl.StandardStringGetter[any]{
		Getter: func(context.Context, any) (any, error) {
			return nil, nil
		},
	}
	function := ottl.Optional[ottl.FunctionGetter[any]]{}

	exprFunc, err := replaceAllPatterns[any](target, modeValue, "regexp", replacement, function)
	assert.NoError(t, err)

	result, err := exprFunc(nil, input)
	require.Error(t, err)
	assert.ErrorContains(t, err, "expected pcommon.Map")
	assert.Nil(t, result)
}

func Test_replaceAllPatterns_bad_function_result(t *testing.T) {
	input := pcommon.NewValueInt(1)
	target := &ottl.StandardPMapGetter[any]{
		Getter: func(ctx context.Context, tCtx any) (any, error) {
			return tCtx, nil
		},
	}
	replacement := &ottl.StandardStringGetter[any]{
		Getter: func(context.Context, any) (any, error) {
			return "{anything}", nil
		},
	}
	ottlValue := ottl.StandardFunctionGetter[any]{
		FCtx: ottl.FunctionContext{
			Set: componenttest.NewNopTelemetrySettings(),
		},
		Fact: StandardConverters[any]()["IsString"],
	}
	function := ottl.NewTestingOptional[ottl.FunctionGetter[any]](ottlValue)

	exprFunc, err := replaceAllPatterns[any](target, modeValue, "regexp", replacement, function)
	assert.NoError(t, err)

	result, err := exprFunc(nil, input)
	require.Error(t, err)
	assert.Nil(t, result)
}

func Test_replaceAllPatterns_get_nil(t *testing.T) {
	target := &ottl.StandardPMapGetter[any]{
		Getter: func(ctx context.Context, tCtx any) (any, error) {
			return tCtx, nil
		},
	}
	replacement := &ottl.StandardStringGetter[any]{
		Getter: func(context.Context, any) (any, error) {
			return "{anything}", nil
		},
	}
	function := ottl.Optional[ottl.FunctionGetter[any]]{}

	exprFunc, err := replaceAllPatterns[any](target, modeValue, "regexp", replacement, function)
	assert.NoError(t, err)

	_, err = exprFunc(nil, nil)
	assert.Error(t, err)
}

func Test_replaceAllPatterns_invalid_pattern(t *testing.T) {
	target := &ottl.StandardPMapGetter[any]{
		Getter: func(ctx context.Context, tCtx any) (any, error) {
			t.Errorf("nothing should be received in this scenario")
			return nil, nil
		},
	}
	replacement := &ottl.StandardStringGetter[any]{
		Getter: func(context.Context, any) (any, error) {
			return "{anything}", nil
		},
	}
	function := ottl.Optional[ottl.FunctionGetter[any]]{}

	invalidRegexPattern := "*"
	exprFunc, err := replaceAllPatterns[any](target, modeValue, invalidRegexPattern, replacement, function)
	require.Error(t, err)
	assert.ErrorContains(t, err, "error parsing regexp:")
	assert.Nil(t, exprFunc)
}

func Test_replaceAllPatterns_invalid_model(t *testing.T) {
	target := &ottl.StandardPMapGetter[any]{
		Getter: func(ctx context.Context, tCtx any) (any, error) {
			t.Errorf("nothing should be received in this scenario")
			return nil, nil
		},
	}
	replacement := &ottl.StandardStringGetter[any]{
		Getter: func(context.Context, any) (any, error) {
			return "{anything}", nil
		},
	}
	function := ottl.Optional[ottl.FunctionGetter[any]]{}

	invalidMode := "invalid"
	exprFunc, err := replaceAllPatterns[any](target, invalidMode, "regex", replacement, function)
	assert.Nil(t, exprFunc)
	assert.Contains(t, err.Error(), "invalid mode")
}
