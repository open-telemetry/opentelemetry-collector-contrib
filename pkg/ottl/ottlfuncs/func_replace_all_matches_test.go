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

func Test_replaceAllMatches(t *testing.T) {
	input := pcommon.NewMap()
	input.PutStr("test", "hello world")
	input.PutStr("test2", "hello")
	input.PutStr("test3", "goodbye")

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
		pattern     string
		replacement ottl.StringGetter[pcommon.Map]
		function    ottl.Optional[ottl.FunctionGetter[pcommon.Map]]
		want        func(pcommon.Map)
	}{
		{
			name:    "replace only matches (with hash function)",
			target:  target,
			pattern: "hello*",
			replacement: ottl.StandardStringGetter[pcommon.Map]{
				Getter: func(context.Context, pcommon.Map) (any, error) {
					return "hello {universe}", nil
				},
			},
			function: optionalArg,
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutStr("test", "hash(hello {universe})")
				expectedMap.PutStr("test2", "hash(hello {universe})")
				expectedMap.PutStr("test3", "goodbye")
			},
		},
		{
			name:    "replace only matches",
			target:  target,
			pattern: "hello*",
			replacement: ottl.StandardStringGetter[pcommon.Map]{
				Getter: func(context.Context, pcommon.Map) (any, error) {
					return "hello {universe}", nil
				},
			},
			function: ottl.Optional[ottl.FunctionGetter[pcommon.Map]]{},
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutStr("test", "hello {universe}")
				expectedMap.PutStr("test2", "hello {universe}")
				expectedMap.PutStr("test3", "goodbye")
			},
		},
		{
			name:    "no matches",
			target:  target,
			pattern: "nothing*",
			replacement: ottl.StandardStringGetter[pcommon.Map]{
				Getter: func(context.Context, pcommon.Map) (any, error) {
					return "nothing {matches}", nil
				},
			},
			function: ottl.Optional[ottl.FunctionGetter[pcommon.Map]]{},
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutStr("test", "hello world")
				expectedMap.PutStr("test2", "hello")
				expectedMap.PutStr("test3", "goodbye")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scenarioMap := pcommon.NewMap()
			input.CopyTo(scenarioMap)

			exprFunc, err := replaceAllMatches(tt.target, tt.pattern, tt.replacement, tt.function)
			assert.NoError(t, err)

			result, err := exprFunc(nil, scenarioMap)
			assert.NoError(t, err)
			assert.Nil(t, result)

			expected := pcommon.NewMap()
			tt.want(expected)

			assert.Equal(t, expected, scenarioMap)
		})
	}
}

func Test_replaceAllMatches_bad_input(t *testing.T) {
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

	exprFunc, err := replaceAllMatches[any](target, "*", replacement, function)
	assert.NoError(t, err)
	_, err = exprFunc(nil, input)
	assert.Error(t, err)
}

func Test_replaceAllMatches_bad_function_input(t *testing.T) {
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

	exprFunc, err := replaceAllMatches[any](target, "regexp", replacement, function)
	assert.NoError(t, err)

	result, err := exprFunc(nil, input)
	require.Error(t, err)
	assert.ErrorContains(t, err, "expected pcommon.Map")
	assert.Nil(t, result)
}

func Test_replaceAllMatches_bad_function_result(t *testing.T) {
	input := pcommon.NewValueInt(1)
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
	ottlValue := ottl.StandardFunctionGetter[any]{
		FCtx: ottl.FunctionContext{
			Set: componenttest.NewNopTelemetrySettings(),
		},
		Fact: StandardConverters[any]()["IsString"],
	}
	function := ottl.NewTestingOptional[ottl.FunctionGetter[any]](ottlValue)

	exprFunc, err := replaceAllMatches[any](target, "regexp", replacement, function)
	assert.NoError(t, err)

	result, err := exprFunc(nil, input)
	require.Error(t, err)
	assert.Nil(t, result)
}

func Test_replaceAllMatches_get_nil(t *testing.T) {
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

	exprFunc, err := replaceAllMatches[any](target, "*", replacement, function)
	assert.NoError(t, err)
	_, err = exprFunc(nil, nil)
	assert.Error(t, err)
}
