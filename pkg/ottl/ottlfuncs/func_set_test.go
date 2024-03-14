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
	target := &ottl.StandardGetSetter[pcommon.Value]{
		Setter: func(ctx context.Context, tCtx pcommon.Value, val any) error {
			tCtx.SetStr(val.(string))
			return nil
		},
		Getter: func(ctx context.Context, tCtx pcommon.Value) (any, error) {
			if tCtx.Str() == "" {
				return nil, nil
			}
			return tCtx.Str(), nil
		},
	}

	tests := []struct {
		name          string
		initialValue  string
		setter        ottl.GetSetter[pcommon.Value]
		getter        ottl.Getter[pcommon.Value]
		strategy      ottl.Optional[string]
		want          func(pcommon.Value)
		expectedError error
	}{
		{
			name:         "set name",
			setter:       target,
			initialValue: "original name",
			strategy:     ottl.Optional[string]{},
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
			name:         "set nil value",
			setter:       target,
			initialValue: "original name",
			strategy:     ottl.Optional[string]{},
			getter: ottl.StandardGetSetter[pcommon.Value]{
				Getter: func(ctx context.Context, tCtx pcommon.Value) (any, error) {
					return nil, nil
				},
			},
			want: func(expectedValue pcommon.Value) {
				expectedValue.SetStr("original name")
			},
		},
		{
			name:         "set value existing - IGNORE",
			initialValue: "some value",
			setter:       target,
			strategy:     ottl.NewTestingOptional[string](setConflictIgnore),
			getter: ottl.StandardGetSetter[pcommon.Value]{
				Getter: func(ctx context.Context, tCtx pcommon.Value) (any, error) {
					return "new name", nil
				},
			},
			want: func(expectedValue pcommon.Value) {
				expectedValue.SetStr("some value")
			},
		},
		{
			name:         "set value existing - FAIL",
			initialValue: "some value",
			setter:       target,
			strategy:     ottl.NewTestingOptional[string](setConflictFail),
			getter: ottl.StandardGetSetter[pcommon.Value]{
				Getter: func(ctx context.Context, tCtx pcommon.Value) (any, error) {
					return "new name", nil
				},
			},
			expectedError: ErrValueAlreadyPresent,
		},
		{
			name:         "set value existing - REPLACE",
			initialValue: "some value",
			setter:       target,
			strategy:     ottl.NewTestingOptional[string](setConflictReplace),
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
			name:         "set value existing - UPDATE",
			initialValue: "some value",
			setter:       target,
			strategy:     ottl.NewTestingOptional[string](setConflictUpdate),
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
			name:     "set value missing - UPDATE",
			setter:   target,
			strategy: ottl.NewTestingOptional[string](setConflictUpdate),
			getter: ottl.StandardGetSetter[pcommon.Value]{
				Getter: func(ctx context.Context, tCtx pcommon.Value) (any, error) {
					return "new name", nil
				},
			},
			want: func(expectedValue pcommon.Value) {
				expectedValue.SetStr("")
			},
		},
		{
			name:         "set value existing - default",
			initialValue: "some value",
			setter:       target,
			strategy:     ottl.Optional[string]{},
			getter: ottl.StandardGetSetter[pcommon.Value]{
				Getter: func(ctx context.Context, tCtx pcommon.Value) (any, error) {
					return "new name", nil
				},
			},
			want: func(expectedValue pcommon.Value) {
				expectedValue.SetStr("new name")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scenarioValue := pcommon.NewValueStr(tt.initialValue)
			exprFunc, err := set(tt.setter, tt.getter, tt.strategy)
			assert.NoError(t, err)

			result, err := exprFunc(nil, scenarioValue)
			if tt.expectedError != nil {
				assert.Equal(t, tt.expectedError, err)
				return
			}

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
		Getter: func(ctx context.Context, tCtx any) (any, error) {
			return tCtx, nil
		},
	}

	getter := &ottl.StandardGetSetter[any]{
		Getter: func(ctx context.Context, tCtx any) (any, error) {
			return tCtx, nil
		},
	}

	exprFunc, err := set[any](setter, getter, ottl.Optional[string]{})
	assert.NoError(t, err)

	result, err := exprFunc(nil, nil)
	assert.NoError(t, err)
	assert.Nil(t, result)
}
