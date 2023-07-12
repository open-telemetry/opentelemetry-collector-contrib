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

func Test_truncate(t *testing.T) {
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
		name   string
		target ottl.GetSetter[pcommon.Value]
		limit  int64
		want   func(value pcommon.Value)
	}{
		{
			name:   "below limit",
			target: target,
			limit:  20,
			want: func(expectedValue pcommon.Value) {
				expectedValue.SetStr("hello world")
			},
		},
		{
			name:   "above limit",
			target: target,
			limit:  10,
			want: func(expectedValue pcommon.Value) {
				expectedValue.SetStr("hello worl")
			},
		},
		{
			name:   "at limit",
			target: target,
			limit:  11,
			want: func(expectedValue pcommon.Value) {
				expectedValue.SetStr("hello world")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scenarioValue := pcommon.NewValueStr(input.Str())

			exprFunc, err := truncate(tt.target, tt.limit)
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

func Test_truncate_bad_input(t *testing.T) {
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

	exprFunc, err := truncate[interface{}](target, 10)
	assert.NoError(t, err)

	result, err := exprFunc(nil, input)
	assert.NoError(t, err)
	assert.Nil(t, result)

	assert.Equal(t, pcommon.NewValueInt(1), input)
}

func Test_truncate_get_nil(t *testing.T) {
	target := &ottl.StandardGetSetter[interface{}]{
		Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
			return tCtx, nil
		},
		Setter: func(ctx context.Context, tCtx interface{}, val interface{}) error {
			t.Errorf("nothing should be set in this scenario")
			return nil
		},
	}

	exprFunc, err := truncate[interface{}](target, 10)
	assert.NoError(t, err)

	result, err := exprFunc(nil, nil)
	assert.NoError(t, err)
	assert.Nil(t, result)
}

func Test_truncate_validation(t *testing.T) {
	target := &ottl.StandardGetSetter[interface{}]{
		Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
			return tCtx, nil
		},
		Setter: func(ctx context.Context, tCtx interface{}, val interface{}) error {
			t.Errorf("nothing should be set in this scenario")
			return nil
		},
	}

	_, err := truncate[interface{}](target, -1)
	require.Error(t, err)
	assert.ErrorContains(t, err, "invalid limit for truncate function, -1 cannot be negative")
}
