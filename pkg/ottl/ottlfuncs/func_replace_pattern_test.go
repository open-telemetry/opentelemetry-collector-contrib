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

func Test_replacePattern(t *testing.T) {
	input := pcommon.NewValueStr("application passwd=sensitivedtata otherarg=notsensitive key1 key2")

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
		want        func(pcommon.Value)
	}{
		{
			name:    "replace regex match",
			target:  target,
			pattern: `passwd\=[^\s]*(\s?)`,
			replacement: ottl.StandardStringGetter[pcommon.Value]{
				Getter: func(context.Context, pcommon.Value) (interface{}, error) {
					return "passwd=*** ", nil
				},
			},
			want: func(expectedValue pcommon.Value) {
				expectedValue.SetStr("application passwd=*** otherarg=notsensitive key1 key2")
			},
		},
		{
			name:    "no regex match",
			target:  target,
			pattern: `nomatch\=[^\s]*(\s?)`,
			replacement: ottl.StandardStringGetter[pcommon.Value]{
				Getter: func(context.Context, pcommon.Value) (interface{}, error) {
					return "shouldnotbeinoutput", nil
				},
			},
			want: func(expectedValue pcommon.Value) {
				expectedValue.SetStr("application passwd=sensitivedtata otherarg=notsensitive key1 key2")
			},
		},
		{
			name:    "multiple regex match",
			target:  target,
			pattern: `key[^\s]*(\s?)`,
			replacement: ottl.StandardStringGetter[pcommon.Value]{
				Getter: func(context.Context, pcommon.Value) (interface{}, error) {
					return "**** ", nil
				},
			},
			want: func(expectedValue pcommon.Value) {
				expectedValue.SetStr("application passwd=sensitivedtata otherarg=notsensitive **** **** ")
			},
		},
		{
			name:    "expand capturing groups",
			target:  target,
			pattern: `(\w+)=(\w+)`,
			replacement: ottl.StandardStringGetter[pcommon.Value]{
				Getter: func(context.Context, pcommon.Value) (interface{}, error) {
					return "$1:$2", nil
				},
			},
			want: func(expectedValue pcommon.Value) {
				expectedValue.SetStr("application passwd:sensitivedtata otherarg:notsensitive key1 key2")
			},
		},
		{
			name:    "replacement with literal $",
			target:  target,
			pattern: `passwd\=[^\s]*(\s?)`,
			replacement: ottl.StandardStringGetter[pcommon.Value]{
				Getter: func(context.Context, pcommon.Value) (interface{}, error) {
					return "passwd=$$$$$$ ", nil
				},
			},
			want: func(expectedValue pcommon.Value) {
				expectedValue.SetStr("application passwd=$$$ otherarg=notsensitive key1 key2")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scenarioValue := pcommon.NewValueStr(input.Str())
			exprFunc, err := replacePattern(tt.target, tt.pattern, tt.replacement)
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

	exprFunc, err := replacePattern[interface{}](target, "regexp", replacement)
	assert.NoError(t, err)

	result, err := exprFunc(nil, input)
	assert.NoError(t, err)
	assert.Nil(t, result)
	assert.Equal(t, pcommon.NewValueInt(1), input)
}

func Test_replacePattern_get_nil(t *testing.T) {
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

	exprFunc, err := replacePattern[interface{}](target, `nomatch\=[^\s]*(\s?)`, replacement)
	assert.NoError(t, err)

	result, err := exprFunc(nil, nil)
	assert.NoError(t, err)
	assert.Nil(t, result)
}

func Test_replacePatterns_invalid_pattern(t *testing.T) {
	target := &ottl.StandardGetSetter[interface{}]{
		Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
			t.Errorf("nothing should be received in this scenario")
			return nil, nil
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

	invalidRegexPattern := "*"
	_, err := replacePattern[interface{}](target, invalidRegexPattern, replacement)
	require.Error(t, err)
	assert.ErrorContains(t, err, "error parsing regexp:")
}
