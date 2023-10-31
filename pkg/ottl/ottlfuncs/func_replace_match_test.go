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

func Test_replaceMatch(t *testing.T) {
	input := pcommon.NewValueStr("hello world")
	ottlValue := ottl.StandardFunctionGetter[pcommon.Value]{
		FCtx: ottl.FunctionContext{
			Set: componenttest.NewNopTelemetrySettings(),
		},
		Fact: StandardConverters[pcommon.Value]()["SHA256"],
	}
	optionalArg := ottl.NewTestingOptional[ottl.FunctionGetter[pcommon.Value]](ottlValue)
	target := &ottl.StandardGetSetter[pcommon.Value]{
		Getter: func(ctx context.Context, tCtx pcommon.Value) (interface{}, error) {
			return tCtx.Str(), nil
		},
		Setter: func(ctx context.Context, tCtx pcommon.Value, val interface{}) error {
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
			name:    "replace match (with hash function)",
			target:  target,
			pattern: "hello*",
			replacement: ottl.StandardStringGetter[pcommon.Value]{
				Getter: func(context.Context, pcommon.Value) (interface{}, error) {
					return "hello {universe}", nil
				},
			},
			function: optionalArg,
			want: func(expectedValue pcommon.Value) {
				expectedValue.SetStr("4804d6b7f03268e33f78c484977f3d81771220df07cc6aac4ad4868102141fad")
			},
		},
		{
			name:    "replace match",
			target:  target,
			pattern: "hello*",
			replacement: ottl.StandardStringGetter[pcommon.Value]{
				Getter: func(context.Context, pcommon.Value) (interface{}, error) {
					return "hello {universe}", nil
				},
			},
			function: ottl.Optional[ottl.FunctionGetter[pcommon.Value]]{},
			want: func(expectedValue pcommon.Value) {
				expectedValue.SetStr("hello {universe}")
			},
		},
		{
			name:    "no match",
			target:  target,
			pattern: "goodbye*",
			replacement: ottl.StandardStringGetter[pcommon.Value]{
				Getter: func(context.Context, pcommon.Value) (interface{}, error) {
					return "goodbye {universe}", nil
				},
			},
			function: ottl.Optional[ottl.FunctionGetter[pcommon.Value]]{},
			want: func(expectedValue pcommon.Value) {
				expectedValue.SetStr("hello world")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scenarioValue := pcommon.NewValueStr(input.Str())

			exprFunc, err := replaceMatch(tt.target, tt.pattern, tt.replacement, tt.function)
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

func Test_replaceMatch_bad_input(t *testing.T) {
	input := pcommon.NewValueInt(1)
	target := &ottl.StandardGetSetter[interface{}]{
		Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
			return tCtx, nil
		},
		Setter: func(ctx context.Context, tCtx interface{}, val interface{}) error {
			t.Errorf("nothing should be set in this scenario")
			return nil
		},
	}
	replacement := &ottl.StandardStringGetter[interface{}]{
		Getter: func(context.Context, interface{}) (interface{}, error) {
			return "{replacement}", nil
		},
	}
	function := ottl.Optional[ottl.FunctionGetter[interface{}]]{}

	exprFunc, err := replaceMatch[interface{}](target, "*", replacement, function)
	assert.NoError(t, err)

	result, err := exprFunc(nil, input)
	assert.NoError(t, err)
	assert.Nil(t, result)

	assert.Equal(t, pcommon.NewValueInt(1), input)
}

func Test_replaceMatch_bad_function_input(t *testing.T) {
	input := pcommon.NewValueInt(1)
	target := &ottl.StandardGetSetter[interface{}]{
		Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
			return tCtx, nil
		},
		Setter: func(ctx context.Context, tCtx interface{}, val interface{}) error {
			t.Errorf("nothing should be set in this scenario")
			return nil
		},
	}
	replacement := &ottl.StandardStringGetter[interface{}]{
		Getter: func(context.Context, interface{}) (interface{}, error) {
			return nil, nil
		},
	}
	function := ottl.Optional[ottl.FunctionGetter[interface{}]]{}

	exprFunc, err := replaceMatch[interface{}](target, "regexp", replacement, function)
	assert.NoError(t, err)

	result, err := exprFunc(nil, input)
	require.Error(t, err)
	assert.ErrorContains(t, err, "expected string but got nil")
	assert.Nil(t, result)
}

func Test_replaceMatch_bad_function_result(t *testing.T) {
	input := pcommon.NewValueInt(1)
	target := &ottl.StandardGetSetter[interface{}]{
		Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
			return tCtx, nil
		},
		Setter: func(ctx context.Context, tCtx interface{}, val interface{}) error {
			t.Errorf("nothing should be set in this scenario")
			return nil
		},
	}
	replacement := &ottl.StandardStringGetter[interface{}]{
		Getter: func(context.Context, interface{}) (interface{}, error) {
			return nil, nil
		},
	}
	ottlValue := ottl.StandardFunctionGetter[interface{}]{
		FCtx: ottl.FunctionContext{
			Set: componenttest.NewNopTelemetrySettings(),
		},
		Fact: StandardConverters[interface{}]()["IsString"],
	}
	function := ottl.NewTestingOptional[ottl.FunctionGetter[interface{}]](ottlValue)

	exprFunc, err := replaceMatch[interface{}](target, "regexp", replacement, function)
	assert.NoError(t, err)

	result, err := exprFunc(nil, input)
	require.Error(t, err)
	assert.ErrorContains(t, err, "replacement value is not a string")
	assert.Nil(t, result)
}

func Test_replaceMatch_get_nil(t *testing.T) {
	target := &ottl.StandardGetSetter[interface{}]{
		Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
			return tCtx, nil
		},
		Setter: func(ctx context.Context, tCtx interface{}, val interface{}) error {
			t.Errorf("nothing should be set in this scenario")
			return nil
		},
	}
	replacement := &ottl.StandardStringGetter[interface{}]{
		Getter: func(context.Context, interface{}) (interface{}, error) {
			return "{anything}", nil
		},
	}
	function := ottl.Optional[ottl.FunctionGetter[interface{}]]{}

	exprFunc, err := replaceMatch[interface{}](target, "*", replacement, function)
	assert.NoError(t, err)

	result, err := exprFunc(nil, nil)
	assert.NoError(t, err)
	assert.Nil(t, result)
}
