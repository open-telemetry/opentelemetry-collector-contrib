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
		Fact: optionalFnTestFactory[pcommon.Value](),
	}
	passwdPrefix := ottl.StandardStringGetter[pcommon.Value]{
		Getter: func(context.Context, pcommon.Value) (any, error) {
			return "passwd=%s", nil
		},
	}
	optionalArg := ottl.NewTestingOptional[ottl.FunctionGetter[pcommon.Value]](ottlValue)
	target := &ottl.StandardGetSetter[pcommon.Value]{
		Getter: func(_ context.Context, tCtx pcommon.Value) (any, error) {
			return tCtx.Str(), nil
		},
		Setter: func(_ context.Context, tCtx pcommon.Value, val any) error {
			tCtx.SetStr(val.(string))
			return nil
		},
	}

	tests := []struct {
		name              string
		target            ottl.GetSetter[pcommon.Value]
		pattern           string
		replacement       ottl.StringGetter[pcommon.Value]
		function          ottl.Optional[ottl.FunctionGetter[pcommon.Value]]
		replacementFormat ottl.Optional[ottl.StringGetter[pcommon.Value]]
		want              func(pcommon.Value)
	}{
		{
			name:    "replace match (with hash function)",
			target:  target,
			pattern: "hello*",
			replacement: ottl.StandardStringGetter[pcommon.Value]{
				Getter: func(context.Context, pcommon.Value) (any, error) {
					return "hello {universe}", nil
				},
			},
			function:          optionalArg,
			replacementFormat: ottl.NewTestingOptional[ottl.StringGetter[pcommon.Value]](passwdPrefix),
			want: func(expectedValue pcommon.Value) {
				expectedValue.SetStr("passwd=hash(hello {universe})")
			},
		},
		{
			name:    "replace match",
			target:  target,
			pattern: "hello*",
			replacement: ottl.StandardStringGetter[pcommon.Value]{
				Getter: func(context.Context, pcommon.Value) (any, error) {
					return "hello {universe}", nil
				},
			},
			function:          ottl.Optional[ottl.FunctionGetter[pcommon.Value]]{},
			replacementFormat: ottl.Optional[ottl.StringGetter[pcommon.Value]]{},
			want: func(expectedValue pcommon.Value) {
				expectedValue.SetStr("hello {universe}")
			},
		},
		{
			name:    "no match",
			target:  target,
			pattern: "goodbye*",
			replacement: ottl.StandardStringGetter[pcommon.Value]{
				Getter: func(context.Context, pcommon.Value) (any, error) {
					return "goodbye {universe}", nil
				},
			},
			function:          ottl.Optional[ottl.FunctionGetter[pcommon.Value]]{},
			replacementFormat: ottl.Optional[ottl.StringGetter[pcommon.Value]]{},
			want: func(expectedValue pcommon.Value) {
				expectedValue.SetStr("hello world")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scenarioValue := pcommon.NewValueStr(input.Str())

			pattern := ottl.StandardStringGetter[pcommon.Value]{
				Getter: func(_ context.Context, _ pcommon.Value) (any, error) {
					return tt.pattern, nil
				},
			}

			exprFunc, err := replaceMatch(tt.target, pattern, tt.replacement, tt.function, tt.replacementFormat)
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
	target := &ottl.StandardGetSetter[any]{
		Getter: func(_ context.Context, tCtx any) (any, error) {
			return tCtx, nil
		},
		Setter: func(context.Context, any, any) error {
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
	replacementFormat := ottl.Optional[ottl.StringGetter[any]]{}

	pattern := ottl.StandardStringGetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
			return "*", nil
		},
	}

	exprFunc, err := replaceMatch[any](target, pattern, replacement, function, replacementFormat)
	assert.NoError(t, err)

	result, err := exprFunc(nil, input)
	assert.NoError(t, err)
	assert.Nil(t, result)

	assert.Equal(t, pcommon.NewValueInt(1), input)
}

func Test_replaceMatch_bad_function_input(t *testing.T) {
	input := pcommon.NewValueInt(1)
	target := &ottl.StandardGetSetter[any]{
		Getter: func(_ context.Context, tCtx any) (any, error) {
			return tCtx, nil
		},
		Setter: func(context.Context, any, any) error {
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
	replacementFormat := ottl.Optional[ottl.StringGetter[any]]{}

	pattern := ottl.StandardStringGetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
			return "regexp", nil
		},
	}

	exprFunc, err := replaceMatch[any](target, pattern, replacement, function, replacementFormat)
	assert.NoError(t, err)

	result, err := exprFunc(nil, input)
	require.Error(t, err)
	assert.ErrorContains(t, err, "expected string but got nil")
	assert.Nil(t, result)
}

func Test_replaceMatch_bad_function_result(t *testing.T) {
	input := pcommon.NewValueInt(1)
	target := &ottl.StandardGetSetter[any]{
		Getter: func(_ context.Context, tCtx any) (any, error) {
			return tCtx, nil
		},
		Setter: func(context.Context, any, any) error {
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
	replacementFormat := ottl.Optional[ottl.StringGetter[any]]{}

	pattern := ottl.StandardStringGetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
			return "regexp", nil
		},
	}

	exprFunc, err := replaceMatch[any](target, pattern, replacement, function, replacementFormat)
	assert.NoError(t, err)

	result, err := exprFunc(nil, input)
	require.Error(t, err)
	assert.ErrorContains(t, err, "replacement value is not a string")
	assert.Nil(t, result)
}

func Test_replaceMatch_get_nil(t *testing.T) {
	target := &ottl.StandardGetSetter[any]{
		Getter: func(_ context.Context, tCtx any) (any, error) {
			return tCtx, nil
		},
		Setter: func(context.Context, any, any) error {
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
	replacementFormat := ottl.Optional[ottl.StringGetter[any]]{}

	pattern := ottl.StandardStringGetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
			return "*", nil
		},
	}

	exprFunc, err := replaceMatch[any](target, pattern, replacement, function, replacementFormat)
	assert.NoError(t, err)

	result, err := exprFunc(nil, nil)
	assert.NoError(t, err)
	assert.Nil(t, result)
}
