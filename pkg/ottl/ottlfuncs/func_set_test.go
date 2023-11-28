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

func Test_set(t *testing.T) {
	input := pcommon.NewValueStr("original name")

	target := &ottl.StandardGetSetter[pcommon.Value]{
		Setter: func(ctx context.Context, tCtx pcommon.Value, val any) error {
			tCtx.SetStr(val.(string))
			return nil
		},
	}

	tests := []struct {
		name   string
		setter ottl.Setter[pcommon.Value]
		getter ottl.Getter[pcommon.Value]
		want   func(pcommon.Value)
	}{
		{
			name:   "set name",
			setter: target,
			getter: ottl.StandardGetSetter[pcommon.Value]{
				Getter: func(ctx context.Context, tCtx pcommon.Value) (any, error) {
					return "new name", nil
				},
			},
			want: func(expectedValue pcommon.Value) {
				expectedValue.SetStr("new name")
			},
		},
		{
			name:   "set nil value",
			setter: target,
			getter: ottl.StandardGetSetter[pcommon.Value]{
				Getter: func(ctx context.Context, tCtx pcommon.Value) (any, error) {
					return nil, nil
				},
			},
			want: func(expectedValue pcommon.Value) {
				expectedValue.SetStr("original name")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scenarioValue := pcommon.NewValueStr(input.Str())

			exprFunc := set(tt.setter, tt.getter)

			result, err := exprFunc(nil, scenarioValue)
			assert.NoError(t, err)
			assert.Nil(t, result)

			expected := pcommon.NewValueStr("")
			tt.want(expected)

			assert.Equal(t, expected, scenarioValue)
		})
	}
}

func Test_set_get_nil(t *testing.T) {
	setter := &ottl.StandardGetSetter[any]{
		Setter: func(ctx context.Context, tCtx any, val any) error {
			t.Errorf("nothing should be set in this scenario")
			return nil
		},
	}

	getter := &ottl.StandardGetSetter[any]{
		Getter: func(ctx context.Context, tCtx any) (any, error) {
			return tCtx, nil
		},
	}

	exprFunc := set[any](setter, getter)

	result, err := exprFunc(nil, nil)
	assert.NoError(t, err)
	assert.Nil(t, result)
}
