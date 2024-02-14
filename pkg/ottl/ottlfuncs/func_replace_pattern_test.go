// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type optionalFnTestArgs[K any] struct {
	Target ottl.StringGetter[K]
}

func optionalFnTestFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("Test", &optionalFnTestArgs[K]{}, createTestFunction[K])
}

func createTestFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*optionalFnTestArgs[K])

	if !ok {
		return nil, fmt.Errorf("TestFactory args must be of type *optionalFnTestArgs[K]")
	}

	return hashString(args.Target), nil
}

func hashString[K any](target ottl.StringGetter[K]) ottl.ExprFunc[K] {

	return func(ctx context.Context, tCtx K) (any, error) {
		val, err := target.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}
		return fmt.Sprintf("hash(%s)", val), nil
	}
}

func Test_replacePattern(t *testing.T) {
	input := pcommon.NewValueStr("application passwd=sensitivedtata otherarg=notsensitive key1 key2")
	ottlValue := ottl.StandardFunctionGetter[pcommon.Value]{
		FCtx: ottl.FunctionContext{
			Set: componenttest.NewNopTelemetrySettings(),
		},
		Fact: optionalFnTestFactory[pcommon.Value](),
	}
	optionalArg := ottl.NewTestingOptional[ottl.FunctionGetter[pcommon.Value]](ottlValue)
	target := &ottl.StandardGetSetter[pcommon.Value]{
		Getter: func(ctx context.Context, tCtx pcommon.Value) (any, error) {
			return tCtx.Str(), nil
		},
		Setter: func(ctx context.Context, tCtx pcommon.Value, val any) error {
			tCtx.SetStr(val.(string))
			return nil
		},
	}

	tests := []struct {
		name        string
		target      ottl.GetSetter[pcommon.Value]
		pattern     string
		replacement ottl.StringGetter[pcommon.Value]
		function    ottl.Optional[ottl.FunctionGetter[pcommon.Value]]
		want        func(pcommon.Value)
	}{
		{
			name:    "replace regex match (with hash function)",
			target:  target,
			pattern: `passwd\=([^\s]*)\s?`,
			replacement: ottl.StandardStringGetter[pcommon.Value]{
				Getter: func(context.Context, pcommon.Value) (any, error) {
					return "$1", nil
				},
			},
			function: optionalArg,
			want: func(expectedValue pcommon.Value) {
				expectedValue.SetStr("application hash(sensitivedtata)otherarg=notsensitive key1 key2")
			},
		},
		{
			name:    "replace regex match (static text)",
			target:  target,
			pattern: `passwd\=([^\s]*)`,
			replacement: ottl.StandardStringGetter[pcommon.Value]{
				Getter: func(context.Context, pcommon.Value) (any, error) {
					return "passwd", nil
				},
			},
			function: optionalArg,
			want: func(expectedValue pcommon.Value) {
				expectedValue.SetStr("application hash(passwd) otherarg=notsensitive key1 key2")
			},
		},
		{
			name:    "replace regex match (no capture group with $1 and hash function)",
			target:  target,
			pattern: `passwd\=[^\s]*\s?`,
			replacement: ottl.StandardStringGetter[pcommon.Value]{
				Getter: func(context.Context, pcommon.Value) (any, error) {
					return "$1", nil
				},
			},
			function: optionalArg,
			want: func(expectedValue pcommon.Value) {
				expectedValue.SetStr("application hash()otherarg=notsensitive key1 key2")
			},
		},
		{
			name:    "replace regex match (no capture group or hash function with $1)",
			target:  target,
			pattern: `passwd\=[^\s]*\s?`,
			replacement: ottl.StandardStringGetter[pcommon.Value]{
				Getter: func(context.Context, pcommon.Value) (any, error) {
					return "$1", nil
				},
			},
			function: ottl.Optional[ottl.FunctionGetter[pcommon.Value]]{},
			want: func(expectedValue pcommon.Value) {
				expectedValue.SetStr("application otherarg=notsensitive key1 key2")
			},
		},
		{
			name:    "replace regex match",
			target:  target,
			pattern: `passwd\=[^\s]*(\s?)`,
			replacement: ottl.StandardStringGetter[pcommon.Value]{
				Getter: func(context.Context, pcommon.Value) (any, error) {
					return "passwd=*** ", nil
				},
			},
			function: ottl.Optional[ottl.FunctionGetter[pcommon.Value]]{},
			want: func(expectedValue pcommon.Value) {
				expectedValue.SetStr("application passwd=*** otherarg=notsensitive key1 key2")
			},
		},
		{
			name:    "no regex match",
			target:  target,
			pattern: `nomatch\=[^\s]*(\s?)`,
			replacement: ottl.StandardStringGetter[pcommon.Value]{
				Getter: func(context.Context, pcommon.Value) (any, error) {
					return "shouldnotbeinoutput", nil
				},
			},
			function: ottl.Optional[ottl.FunctionGetter[pcommon.Value]]{},
			want: func(expectedValue pcommon.Value) {
				expectedValue.SetStr("application passwd=sensitivedtata otherarg=notsensitive key1 key2")
			},
		},
		{
			name:    "multiple regex match",
			target:  target,
			pattern: `key[^\s]*(\s?)`,
			replacement: ottl.StandardStringGetter[pcommon.Value]{
				Getter: func(context.Context, pcommon.Value) (any, error) {
					return "**** ", nil
				},
			},
			function: ottl.Optional[ottl.FunctionGetter[pcommon.Value]]{},
			want: func(expectedValue pcommon.Value) {
				expectedValue.SetStr("application passwd=sensitivedtata otherarg=notsensitive **** **** ")
			},
		},
		{
			name:    "expand capturing groups",
			target:  target,
			pattern: `(\w+)=(\w+)`,
			replacement: ottl.StandardStringGetter[pcommon.Value]{
				Getter: func(context.Context, pcommon.Value) (any, error) {
					return "$1:$2", nil
				},
			},
			function: ottl.Optional[ottl.FunctionGetter[pcommon.Value]]{},
			want: func(expectedValue pcommon.Value) {
				expectedValue.SetStr("application passwd:sensitivedtata otherarg:notsensitive key1 key2")
			},
		},
		{
			name:    "replacement with literal $",
			target:  target,
			pattern: `passwd\=[^\s]*(\s?)`,
			replacement: ottl.StandardStringGetter[pcommon.Value]{
				Getter: func(context.Context, pcommon.Value) (any, error) {
					return "passwd=$$$$$$ ", nil
				},
			},
			function: ottl.Optional[ottl.FunctionGetter[pcommon.Value]]{},
			want: func(expectedValue pcommon.Value) {
				expectedValue.SetStr("application passwd=$$$ otherarg=notsensitive key1 key2")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scenarioValue := pcommon.NewValueStr(input.Str())
			exprFunc, err := replacePattern(tt.target, tt.pattern, tt.replacement, tt.function)
			assert.NoError(t, err)

			result, err := exprFunc(nil, scenarioValue)
			assert.NoError(t, err)
			assert.Nil(t, result)

			expected := pcommon.NewValueStr("")
			tt.want(expected)

			assert.Equal(t, expected, scenarioValue)
		})
	}
}

func Test_replacePattern_bad_input(t *testing.T) {
	input := pcommon.NewValueInt(1)
	target := &ottl.StandardGetSetter[any]{
		Getter: func(ctx context.Context, tCtx any) (any, error) {
			return tCtx, nil
		},
		Setter: func(ctx context.Context, tCtx any, val any) error {
			t.Errorf("nothing should be set in this scenario")
			return nil
		},
	}
	replacement := &ottl.StandardStringGetter[any]{
		Getter: func(context.Context, any) (any, error) {
			return "{replacement}", nil
		},
	}
	function := ottl.Optional[ottl.FunctionGetter[any]]{}

	exprFunc, err := replacePattern[any](target, "regexp", replacement, function)
	assert.NoError(t, err)

	result, err := exprFunc(nil, input)
	assert.NoError(t, err)
	assert.Nil(t, result)
	assert.Equal(t, pcommon.NewValueInt(1), input)
}

func Test_replacePattern_bad_function_input(t *testing.T) {
	input := pcommon.NewValueInt(1)
	target := &ottl.StandardGetSetter[any]{
		Getter: func(ctx context.Context, tCtx any) (any, error) {
			return tCtx, nil
		},
		Setter: func(ctx context.Context, tCtx any, val any) error {
			t.Errorf("nothing should be set in this scenario")
			return nil
		},
	}
	replacement := &ottl.StandardStringGetter[any]{
		Getter: func(context.Context, any) (any, error) {
			return nil, nil
		},
	}
	function := ottl.Optional[ottl.FunctionGetter[any]]{}

	exprFunc, err := replacePattern[any](target, "regexp", replacement, function)
	assert.NoError(t, err)

	result, err := exprFunc(nil, input)
	require.Error(t, err)
	assert.ErrorContains(t, err, "expected string but got nil")
	assert.Nil(t, result)
}

func Test_replacePattern_bad_function_result(t *testing.T) {
	input := pcommon.NewValueInt(1)
	target := &ottl.StandardGetSetter[any]{
		Getter: func(ctx context.Context, tCtx any) (any, error) {
			return tCtx, nil
		},
		Setter: func(ctx context.Context, tCtx any, val any) error {
			t.Errorf("nothing should be set in this scenario")
			return nil
		},
	}
	replacement := &ottl.StandardStringGetter[any]{
		Getter: func(context.Context, any) (any, error) {
			return nil, nil
		},
	}
	ottlValue := ottl.StandardFunctionGetter[any]{
		FCtx: ottl.FunctionContext{
			Set: componenttest.NewNopTelemetrySettings(),
		},
		Fact: StandardConverters[any]()["IsString"],
	}
	function := ottl.NewTestingOptional[ottl.FunctionGetter[any]](ottlValue)

	exprFunc, err := replacePattern[any](target, "regexp", replacement, function)
	assert.NoError(t, err)

	result, err := exprFunc(nil, input)
	require.Error(t, err)
	assert.ErrorContains(t, err, "expected string but got nil")
	assert.Nil(t, result)
}

func Test_replacePattern_get_nil(t *testing.T) {
	target := &ottl.StandardGetSetter[any]{
		Getter: func(ctx context.Context, tCtx any) (any, error) {
			return tCtx, nil
		},
		Setter: func(ctx context.Context, tCtx any, val any) error {
			t.Errorf("nothing should be set in this scenario")
			return nil
		},
	}
	replacement := &ottl.StandardStringGetter[any]{
		Getter: func(context.Context, any) (any, error) {
			return "{anything}", nil
		},
	}
	function := ottl.Optional[ottl.FunctionGetter[any]]{}

	exprFunc, err := replacePattern[any](target, `nomatch\=[^\s]*(\s?)`, replacement, function)
	assert.NoError(t, err)

	result, err := exprFunc(nil, nil)
	assert.NoError(t, err)
	assert.Nil(t, result)
}

func Test_replacePatterns_invalid_pattern(t *testing.T) {
	target := &ottl.StandardGetSetter[any]{
		Getter: func(ctx context.Context, tCtx any) (any, error) {
			t.Errorf("nothing should be received in this scenario")
			return nil, nil
		},
		Setter: func(ctx context.Context, tCtx any, val any) error {
			t.Errorf("nothing should be set in this scenario")
			return nil
		},
	}
	replacement := &ottl.StandardStringGetter[any]{
		Getter: func(context.Context, any) (any, error) {
			return "{anything}", nil
		},
	}
	function := ottl.Optional[ottl.FunctionGetter[any]]{}

	invalidRegexPattern := "*"
	_, err := replacePattern[any](target, invalidRegexPattern, replacement, function)
	require.Error(t, err)
	assert.ErrorContains(t, err, "error parsing regexp:")
}
