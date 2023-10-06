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
		Fact: StandardConverters[pcommon.Map]()["SHA256"],
	}
	optionalArg := ottl.NewTestingOptional[ottl.FunctionGetter[pcommon.Map]](ottlValue)

	target := &ottl.StandardPMapGetter[pcommon.Map]{
		Getter: func(ctx context.Context, tCtx pcommon.Map) (interface{}, error) {
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
				Getter: func(context.Context, pcommon.Map) (interface{}, error) {
					return "hello {universe}", nil
				},
			},
			function: optionalArg,
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutStr("test", "4804d6b7f03268e33f78c484977f3d81771220df07cc6aac4ad4868102141fad")
				expectedMap.PutStr("test2", "4804d6b7f03268e33f78c484977f3d81771220df07cc6aac4ad4868102141fad")
				expectedMap.PutStr("test3", "goodbye")
			},
		},
		{
			name:    "replace only matches",
			target:  target,
			pattern: "hello*",
			replacement: ottl.StandardStringGetter[pcommon.Map]{
				Getter: func(context.Context, pcommon.Map) (interface{}, error) {
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
				Getter: func(context.Context, pcommon.Map) (interface{}, error) {
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
	function := ottl.Optional[ottl.FunctionGetter[interface{}]]{}

	exprFunc, err := replaceAllMatches[interface{}](target, "*", replacement, function)
	assert.NoError(t, err)
	_, err = exprFunc(nil, input)
	assert.Error(t, err)
}

func Test_replaceAllMatches_bad_function_input(t *testing.T) {
	input := pcommon.NewValueInt(1)
	target := &ottl.StandardPMapGetter[interface{}]{
		Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
			return tCtx, nil
		},
	}
	replacement := &ottl.StandardStringGetter[interface{}]{
		Getter: func(context.Context, interface{}) (interface{}, error) {
			return nil, nil
		},
	}
	function := ottl.Optional[ottl.FunctionGetter[interface{}]]{}

	exprFunc, err := replaceAllMatches[interface{}](target, "regexp", replacement, function)
	assert.NoError(t, err)

	result, err := exprFunc(nil, input)
	require.Error(t, err)
	assert.ErrorContains(t, err, "expected pcommon.Map")
	assert.Nil(t, result)
}

func Test_replaceAllMatches_bad_function_result(t *testing.T) {
	input := pcommon.NewValueInt(1)
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
	ottlValue := ottl.StandardFunctionGetter[interface{}]{
		FCtx: ottl.FunctionContext{
			Set: componenttest.NewNopTelemetrySettings(),
		},
		Fact: StandardConverters[interface{}]()["IsString"],
	}
	function := ottl.NewTestingOptional[ottl.FunctionGetter[interface{}]](ottlValue)

	exprFunc, err := replaceAllMatches[interface{}](target, "regexp", replacement, function)
	assert.NoError(t, err)

	result, err := exprFunc(nil, input)
	require.Error(t, err)
	assert.Nil(t, result)
}

func Test_replaceAllMatches_get_nil(t *testing.T) {
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
	function := ottl.Optional[ottl.FunctionGetter[interface{}]]{}

	exprFunc, err := replaceAllMatches[interface{}](target, "*", replacement, function)
	assert.NoError(t, err)
	_, err = exprFunc(nil, nil)
	assert.Error(t, err)
}
