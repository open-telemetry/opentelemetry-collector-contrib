// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_replaceMatch(t *testing.T) {
	input := pcommon.NewValueStr("hello world")

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
		replacement string
		want        func(pcommon.Value)
	}{
		{
			name:        "replace match",
			target:      target,
			pattern:     "hello*",
			replacement: "hello {universe}",
			want: func(expectedValue pcommon.Value) {
				expectedValue.SetStr("hello {universe}")
			},
		},
		{
			name:        "no match",
			target:      target,
			pattern:     "goodbye*",
			replacement: "goodbye {universe}",
			want: func(expectedValue pcommon.Value) {
				expectedValue.SetStr("hello world")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scenarioValue := pcommon.NewValueStr(input.Str())

			exprFunc, err := replaceMatch(tt.target, tt.pattern, tt.replacement)
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

	exprFunc, err := replaceMatch[interface{}](target, "*", "{replacement}")
	assert.NoError(t, err)

	result, err := exprFunc(nil, input)
	assert.NoError(t, err)
	assert.Nil(t, result)

	assert.Equal(t, pcommon.NewValueInt(1), input)
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

	exprFunc, err := replaceMatch[interface{}](target, "*", "{anything}")
	assert.NoError(t, err)

	result, err := exprFunc(nil, nil)
	assert.NoError(t, err)
	assert.Nil(t, result)
}
