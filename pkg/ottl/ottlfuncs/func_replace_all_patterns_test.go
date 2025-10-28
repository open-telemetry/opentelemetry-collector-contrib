// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"errors"
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
	input.PutStr("test7", "")

	ottlValue := ottl.StandardFunctionGetter[pcommon.Map]{
		FCtx: ottl.FunctionContext{
			Set: componenttest.NewNopTelemetrySettings(),
		},
		Fact: optionalFnTestFactory[pcommon.Map](),
	}
	prefix := ottl.StandardStringGetter[pcommon.Map]{
		Getter: func(context.Context, pcommon.Map) (any, error) {
			return "prefix=%s", nil
		},
	}
	invalidPrefix := ottl.StandardStringGetter[pcommon.Map]{
		Getter: func(context.Context, pcommon.Map) (any, error) {
			return "prefix=", nil
		},
	}
	optionalArg := ottl.NewTestingOptional[ottl.FunctionGetter[pcommon.Map]](ottlValue)

	tests := []struct {
		name              string
		mode              string
		pattern           string
		replacement       ottl.StringGetter[pcommon.Map]
		replacementFormat ottl.Optional[ottl.StringGetter[pcommon.Map]]
		function          ottl.Optional[ottl.FunctionGetter[pcommon.Map]]
		want              func(pcommon.Map)
	}{
		{
			name:    "replace only matches (with hash function)",
			mode:    modeValue,
			pattern: "hello",
			replacement: ottl.StandardStringGetter[pcommon.Map]{
				Getter: func(context.Context, pcommon.Map) (any, error) {
					return "hello {universe}", nil
				},
			},
			replacementFormat: ottl.Optional[ottl.StringGetter[pcommon.Map]]{},
			function:          optionalArg,
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutStr("test", "hash(hello {universe}) world")
				expectedMap.PutStr("test2", "hash(hello {universe})")
				expectedMap.PutStr("test3", "goodbye world1 and world2")
				expectedMap.PutInt("test4", 1234)
				expectedMap.PutDouble("test5", 1234)
				expectedMap.PutBool("test6", true)
				expectedMap.PutStr("test7", "")
			},
		},
		{
			name:    "replace only matches (with capture group and hash function)",
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
				expectedMap.PutStr("test7", "")
			},
		},
		{
			name:    "replace only matches (no capture group and with hash function)",
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
				expectedMap.PutStr("test7", "")
			},
		},
		{
			name:    "replace only matches (no capture group or hash function)",
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
				expectedMap.PutStr("test7", "")
			},
		},
		{
			name:    "replace only matches (with replacement format)",
			mode:    modeValue,
			pattern: "hello",
			replacement: ottl.StandardStringGetter[pcommon.Map]{
				Getter: func(context.Context, pcommon.Map) (any, error) {
					return "hello {universe}", nil
				},
			},
			replacementFormat: ottl.NewTestingOptional[ottl.StringGetter[pcommon.Map]](prefix),
			function:          optionalArg,
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutStr("test", "prefix=hash(hello {universe}) world")
				expectedMap.PutStr("test2", "prefix=hash(hello {universe})")
				expectedMap.PutStr("test3", "goodbye world1 and world2")
				expectedMap.PutInt("test4", 1234)
				expectedMap.PutDouble("test5", 1234)
				expectedMap.PutBool("test6", true)
				expectedMap.PutStr("test7", "")
			},
		},
		{
			name:    "replace only matches (with invalid replacement format)",
			mode:    modeValue,
			pattern: "hello",
			replacement: ottl.StandardStringGetter[pcommon.Map]{
				Getter: func(context.Context, pcommon.Map) (any, error) {
					return "hello {universe}", nil
				},
			},
			replacementFormat: ottl.NewTestingOptional[ottl.StringGetter[pcommon.Map]](invalidPrefix),
			function:          optionalArg,
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutStr("test", "hello world")
				expectedMap.PutStr("test2", "hello")
				expectedMap.PutStr("test3", "goodbye world1 and world2")
				expectedMap.PutInt("test4", 1234)
				expectedMap.PutDouble("test5", 1234)
				expectedMap.PutBool("test6", true)
				expectedMap.PutStr("test7", "")
			},
		},
		{
			name:    "replace only matches",
			mode:    modeValue,
			pattern: "hello",
			replacement: ottl.StandardStringGetter[pcommon.Map]{
				Getter: func(context.Context, pcommon.Map) (any, error) {
					return "hello {universe}", nil
				},
			},
			replacementFormat: ottl.Optional[ottl.StringGetter[pcommon.Map]]{},
			function:          ottl.Optional[ottl.FunctionGetter[pcommon.Map]]{},
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutStr("test", "hello {universe} world")
				expectedMap.PutStr("test2", "hello {universe}")
				expectedMap.PutStr("test3", "goodbye world1 and world2")
				expectedMap.PutInt("test4", 1234)
				expectedMap.PutDouble("test5", 1234)
				expectedMap.PutBool("test6", true)
				expectedMap.PutStr("test7", "")
			},
		},
		{
			name:    "no matches",
			mode:    modeValue,
			pattern: "nothing",
			replacement: ottl.StandardStringGetter[pcommon.Map]{
				Getter: func(context.Context, pcommon.Map) (any, error) {
					return "nothing {matches}", nil
				},
			},
			replacementFormat: ottl.Optional[ottl.StringGetter[pcommon.Map]]{},
			function:          ottl.Optional[ottl.FunctionGetter[pcommon.Map]]{},
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutStr("test", "hello world")
				expectedMap.PutStr("test2", "hello")
				expectedMap.PutStr("test3", "goodbye world1 and world2")
				expectedMap.PutInt("test4", 1234)
				expectedMap.PutDouble("test5", 1234)
				expectedMap.PutBool("test6", true)
				expectedMap.PutStr("test7", "")
			},
		},
		{
			name:    "multiple regex match",
			mode:    modeValue,
			pattern: `world[^\s]*(\s?)`,
			replacement: ottl.StandardStringGetter[pcommon.Map]{
				Getter: func(context.Context, pcommon.Map) (any, error) {
					return "**** ", nil
				},
			},
			replacementFormat: ottl.Optional[ottl.StringGetter[pcommon.Map]]{},
			function:          ottl.Optional[ottl.FunctionGetter[pcommon.Map]]{},
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutStr("test", "hello **** ")
				expectedMap.PutStr("test2", "hello")
				expectedMap.PutStr("test3", "goodbye **** and **** ")
				expectedMap.PutInt("test4", 1234)
				expectedMap.PutDouble("test5", 1234)
				expectedMap.PutBool("test6", true)
				expectedMap.PutStr("test7", "")
			},
		},
		{
			name:    "regex match (with multiple capture groups)",
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
				expectedMap.PutStr("test7", "")
			},
		},
		{
			name:    "regex match (with multiple matches from one capture group)",
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
				expectedMap.PutStr("test7", "")
			},
		},
		{
			name:    "regex match (with multiple capture groups and hash function)",
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
				expectedMap.PutStr("test7", "")
			},
		},
		{
			name:    "regex match (with multiple capture groups and hash function)",
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
				expectedMap.PutStr("test7", "")
			},
		},
		{
			name:    "regex match (with multiple matches from one capture group and hash function)",
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
				expectedMap.PutStr("test7", "")
			},
		},
		{
			name:    "replace only matches",
			mode:    modeKey,
			pattern: "test2",
			replacement: ottl.StandardStringGetter[pcommon.Map]{
				Getter: func(context.Context, pcommon.Map) (any, error) {
					return "foo", nil
				},
			},
			replacementFormat: ottl.Optional[ottl.StringGetter[pcommon.Map]]{},
			function:          ottl.Optional[ottl.FunctionGetter[pcommon.Map]]{},
			want: func(expectedMap pcommon.Map) {
				expectedMap.Clear()
				expectedMap.PutStr("test", "hello world")
				expectedMap.PutStr("foo", "hello")
				expectedMap.PutStr("test3", "goodbye world1 and world2")
				expectedMap.PutInt("test4", 1234)
				expectedMap.PutDouble("test5", 1234)
				expectedMap.PutBool("test6", true)
				expectedMap.PutStr("test7", "")
			},
		},
		{
			name:    "no matches",
			mode:    modeKey,
			pattern: "nothing",
			replacement: ottl.StandardStringGetter[pcommon.Map]{
				Getter: func(context.Context, pcommon.Map) (any, error) {
					return "nothing {matches}", nil
				},
			},
			replacementFormat: ottl.Optional[ottl.StringGetter[pcommon.Map]]{},
			function:          ottl.Optional[ottl.FunctionGetter[pcommon.Map]]{},
			want: func(expectedMap pcommon.Map) {
				expectedMap.Clear()
				expectedMap.PutStr("test", "hello world")
				expectedMap.PutStr("test2", "hello")
				expectedMap.PutStr("test3", "goodbye world1 and world2")
				expectedMap.PutInt("test4", 1234)
				expectedMap.PutDouble("test5", 1234)
				expectedMap.PutBool("test6", true)
				expectedMap.PutStr("test7", "")
			},
		},
		{
			name:    "multiple regex match",
			mode:    modeKey,
			pattern: `test`,
			replacement: ottl.StandardStringGetter[pcommon.Map]{
				Getter: func(context.Context, pcommon.Map) (any, error) {
					return "test.", nil
				},
			},
			replacementFormat: ottl.Optional[ottl.StringGetter[pcommon.Map]]{},
			function:          ottl.Optional[ottl.FunctionGetter[pcommon.Map]]{},
			want: func(expectedMap pcommon.Map) {
				expectedMap.Clear()
				expectedMap.PutStr("test.", "hello world")
				expectedMap.PutStr("test.2", "hello")
				expectedMap.PutStr("test.3", "goodbye world1 and world2")
				expectedMap.PutInt("test.4", 1234)
				expectedMap.PutDouble("test.5", 1234)
				expectedMap.PutBool("test.6", true)
				expectedMap.PutStr("test.7", "")
			},
		},
		{
			name:    "expand capturing groups in values",
			mode:    modeValue,
			pattern: `world(\d)`,
			replacement: ottl.StandardStringGetter[pcommon.Map]{
				Getter: func(context.Context, pcommon.Map) (any, error) {
					return "world-$1", nil
				},
			},
			replacementFormat: ottl.Optional[ottl.StringGetter[pcommon.Map]]{},
			function:          ottl.Optional[ottl.FunctionGetter[pcommon.Map]]{},
			want: func(expectedMap pcommon.Map) {
				expectedMap.Clear()
				expectedMap.PutStr("test", "hello world")
				expectedMap.PutStr("test2", "hello")
				expectedMap.PutStr("test3", "goodbye world-1 and world-2")
				expectedMap.PutInt("test4", 1234)
				expectedMap.PutDouble("test5", 1234)
				expectedMap.PutBool("test6", true)
				expectedMap.PutStr("test7", "")
			},
		},
		{
			name:    "expand capturing groups in keys",
			mode:    modeKey,
			pattern: `test(\d)`,
			replacement: ottl.StandardStringGetter[pcommon.Map]{
				Getter: func(context.Context, pcommon.Map) (any, error) {
					return "test-$1", nil
				},
			},
			replacementFormat: ottl.Optional[ottl.StringGetter[pcommon.Map]]{},
			function:          ottl.Optional[ottl.FunctionGetter[pcommon.Map]]{},
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutStr("test", "hello world")
				expectedMap.PutStr("test-2", "hello")
				expectedMap.PutStr("test-3", "goodbye world1 and world2")
				expectedMap.PutInt("test-4", 1234)
				expectedMap.PutDouble("test-5", 1234)
				expectedMap.PutBool("test-6", true)
				expectedMap.PutStr("test-7", "")
			},
		},
		{
			name:    "replacement with literal $",
			mode:    modeValue,
			pattern: `world(\d)`,
			replacement: ottl.StandardStringGetter[pcommon.Map]{
				Getter: func(context.Context, pcommon.Map) (any, error) {
					return "$$world-$1", nil
				},
			},
			replacementFormat: ottl.Optional[ottl.StringGetter[pcommon.Map]]{},
			function:          ottl.Optional[ottl.FunctionGetter[pcommon.Map]]{},
			want: func(expectedMap pcommon.Map) {
				expectedMap.Clear()
				expectedMap.PutStr("test", "hello world")
				expectedMap.PutStr("test2", "hello")
				expectedMap.PutStr("test3", "goodbye $world-1 and $world-2")
				expectedMap.PutInt("test4", 1234)
				expectedMap.PutDouble("test5", 1234)
				expectedMap.PutBool("test6", true)
				expectedMap.PutStr("test7", "")
			},
		},
		{
			name:    "replacement for empty string",
			mode:    modeValue,
			pattern: `^$`,
			replacement: ottl.StandardStringGetter[pcommon.Map]{
				Getter: func(context.Context, pcommon.Map) (any, error) {
					return "empty_string_replacement", nil
				},
			},
			replacementFormat: ottl.Optional[ottl.StringGetter[pcommon.Map]]{},
			function:          ottl.Optional[ottl.FunctionGetter[pcommon.Map]]{},
			want: func(expectedMap pcommon.Map) {
				expectedMap.Clear()
				expectedMap.PutStr("test", "hello world")
				expectedMap.PutStr("test2", "hello")
				expectedMap.PutStr("test3", "goodbye world1 and world2")
				expectedMap.PutInt("test4", 1234)
				expectedMap.PutDouble("test5", 1234)
				expectedMap.PutBool("test6", true)
				expectedMap.PutStr("test7", "empty_string_replacement")
			},
		},
		{
			name:    "replacement matches with function",
			mode:    modeKey,
			pattern: `test(\d)`,
			replacement: ottl.StandardStringGetter[pcommon.Map]{
				Getter: func(context.Context, pcommon.Map) (any, error) {
					return "$1", nil
				},
			},
			replacementFormat: ottl.Optional[ottl.StringGetter[pcommon.Map]]{},
			function:          optionalArg,
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutStr("test", "hello world")
				expectedMap.PutStr("hash(2)", "hello")
				expectedMap.PutStr("hash(3)", "goodbye world1 and world2")
				expectedMap.PutInt("hash(4)", 1234)
				expectedMap.PutDouble("hash(5)", 1234)
				expectedMap.PutBool("hash(6)", true)
				expectedMap.PutStr("hash(7)", "")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scenarioMap := pcommon.NewMap()
			input.CopyTo(scenarioMap)

			setterWasCalled := false
			target := ottl.StandardPMapGetSetter[pcommon.Map]{
				Getter: func(_ context.Context, tCtx pcommon.Map) (pcommon.Map, error) {
					return tCtx, nil
				},
				Setter: func(_ context.Context, tCtx pcommon.Map, m any) error {
					setterWasCalled = true
					if v, ok := m.(pcommon.Map); ok {
						v.CopyTo(tCtx)
						return nil
					}
					return errors.New("expected pcommon.Map")
				},
			}

			pattern := &ottl.StandardStringGetter[pcommon.Map]{
				Getter: func(_ context.Context, _ pcommon.Map) (any, error) {
					return tt.pattern, nil
				},
			}

			exprFunc, err := replaceAllPatterns[pcommon.Map](target, tt.mode, pattern, tt.replacement, tt.function, tt.replacementFormat)
			assert.NoError(t, err)

			_, err = exprFunc(nil, scenarioMap)
			assert.NoError(t, err)
			assert.True(t, setterWasCalled)

			expected := pcommon.NewMap()
			tt.want(expected)

			assert.Equal(t, expected, scenarioMap)
		})
	}
}

func Test_replaceAllPatterns_bad_input(t *testing.T) {
	input := pcommon.NewValueStr("not a map")
	target := &ottl.StandardPMapGetSetter[any]{
		Getter: func(_ context.Context, tCtx any) (pcommon.Map, error) {
			if v, ok := tCtx.(pcommon.Map); ok {
				return v, nil
			}
			return pcommon.Map{}, errors.New("expected pcommon.Map")
		},
	}
	replacement := &ottl.StandardStringGetter[any]{
		Getter: func(context.Context, any) (any, error) {
			return "{replacement}", nil
		},
	}
	function := ottl.Optional[ottl.FunctionGetter[any]]{}
	replacementFormat := ottl.Optional[ottl.StringGetter[any]]{}

	pattern := &ottl.StandardStringGetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
			return "regexpattern", nil
		},
	}

	exprFunc, err := replaceAllPatterns[any](target, modeValue, pattern, replacement, function, replacementFormat)
	assert.NoError(t, err)

	_, err = exprFunc(nil, input)
	assert.Error(t, err)
}

func Test_replaceAllPatterns_bad_function_input(t *testing.T) {
	input := pcommon.NewValueInt(1)
	target := &ottl.StandardPMapGetSetter[any]{
		Getter: func(_ context.Context, tCtx any) (pcommon.Map, error) {
			if v, ok := tCtx.(pcommon.Map); ok {
				return v, nil
			}
			return pcommon.Map{}, errors.New("expected pcommon.Map")
		},
	}
	replacement := &ottl.StandardStringGetter[any]{
		Getter: func(context.Context, any) (any, error) {
			return nil, nil
		},
	}
	function := ottl.Optional[ottl.FunctionGetter[any]]{}
	replacementFormat := ottl.Optional[ottl.StringGetter[any]]{}

	pattern := &ottl.StandardStringGetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
			return "regexp", nil
		},
	}

	exprFunc, err := replaceAllPatterns[any](target, modeValue, pattern, replacement, function, replacementFormat)
	assert.NoError(t, err)

	result, err := exprFunc(nil, input)
	require.Error(t, err)
	assert.ErrorContains(t, err, "expected pcommon.Map")
	assert.Nil(t, result)
}

func Test_replaceAllPatterns_bad_function_result(t *testing.T) {
	input := pcommon.NewValueInt(1)
	target := &ottl.StandardPMapGetSetter[any]{
		Getter: func(_ context.Context, tCtx any) (pcommon.Map, error) {
			if v, ok := tCtx.(pcommon.Map); ok {
				return v, nil
			}
			return pcommon.Map{}, errors.New("expected pcommon.Map")
		},
		Setter: func(_ context.Context, tCtx, m any) error {
			if v, ok := tCtx.(pcommon.Map); ok {
				if v2, ok2 := m.(pcommon.Map); ok2 {
					v.CopyTo(v2)
					return nil
				}
			}
			return errors.New("expected pcommon.Map")
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
	replacementFormat := ottl.Optional[ottl.StringGetter[any]]{}

	pattern := &ottl.StandardStringGetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
			return "regexp", nil
		},
	}

	exprFunc, err := replaceAllPatterns[any](target, modeValue, pattern, replacement, function, replacementFormat)
	assert.NoError(t, err)

	result, err := exprFunc(nil, input)
	require.Error(t, err)
	assert.Nil(t, result)
}

func Test_replaceAllPatterns_get_nil(t *testing.T) {
	target := &ottl.StandardPMapGetSetter[any]{
		Getter: func(_ context.Context, tCtx any) (pcommon.Map, error) {
			assert.Nil(t, tCtx)
			return pcommon.NewMap(), nil
		},
		Setter: func(_ context.Context, tCtx, m any) error {
			if v, ok := tCtx.(pcommon.Map); ok {
				if v2, ok2 := m.(pcommon.Map); ok2 {
					v.CopyTo(v2)
					return nil
				}
			}
			return errors.New("expected pcommon.Map")
		},
	}
	replacement := &ottl.StandardStringGetter[any]{
		Getter: func(context.Context, any) (any, error) {
			return "{anything}", nil
		},
	}
	function := ottl.Optional[ottl.FunctionGetter[any]]{}
	replacementFormat := ottl.Optional[ottl.StringGetter[any]]{}

	pattern := &ottl.StandardStringGetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
			return "regexp", nil
		},
	}

	exprFunc, err := replaceAllPatterns[any](target, modeValue, pattern, replacement, function, replacementFormat)
	assert.NoError(t, err)

	_, err = exprFunc(nil, nil)
	assert.Error(t, err)
}

func Test_replaceAllPatterns_invalid_pattern(t *testing.T) {
	target := &ottl.StandardPMapGetSetter[any]{
		Getter: func(context.Context, any) (pcommon.Map, error) {
			return pcommon.Map{}, nil
		},
	}
	replacement := &ottl.StandardStringGetter[any]{
		Getter: func(context.Context, any) (any, error) {
			return "{anything}", nil
		},
	}
	function := ottl.Optional[ottl.FunctionGetter[any]]{}
	replacementFormat := ottl.Optional[ottl.StringGetter[any]]{}

	pattern := &ottl.StandardStringGetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
			return "*", nil
		},
	}
	exprFunc, err := replaceAllPatterns[any](target, modeValue, pattern, replacement, function, replacementFormat)
	require.NoError(t, err)
	_, err = exprFunc(nil, nil)
	assert.ErrorContains(t, err, "error parsing regexp:")
}

func Test_replaceAllPatterns_invalid_model(t *testing.T) {
	target := &ottl.StandardPMapGetSetter[any]{
		Getter: func(context.Context, any) (pcommon.Map, error) {
			return pcommon.Map{}, errors.New("nothing should be received in this scenario")
		},
	}
	replacement := &ottl.StandardStringGetter[any]{
		Getter: func(context.Context, any) (any, error) {
			return "{anything}", nil
		},
	}
	function := ottl.Optional[ottl.FunctionGetter[any]]{}
	replacementFormat := ottl.Optional[ottl.StringGetter[any]]{}

	invalidMode := "invalid"
	pattern := &ottl.StandardStringGetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
			return "regex", nil
		},
	}
	exprFunc, err := replaceAllPatterns[any](target, invalidMode, pattern, replacement, function, replacementFormat)
	assert.Nil(t, exprFunc)
	assert.ErrorContains(t, err, "invalid mode")
}
