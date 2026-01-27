// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_limit(t *testing.T) {
	input := pcommon.NewMap()
	input.PutStr("test", "hello world")
	input.PutInt("test2", 3)
	input.PutBool("test3", true)

	tests := []struct {
		name  string
		limit int64
		keep  []string
		want  func(pcommon.Map)
	}{
		{
			name:  "limit to 1",
			limit: int64(1),
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutStr("test", "hello world")
			},
		},
		{
			name:  "limit to zero",
			limit: int64(0),
			want: func(expectedMap pcommon.Map) {
				expectedMap.EnsureCapacity(input.Len())
			},
		},
		{
			name:  "limit nothing",
			limit: int64(100),
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutStr("test", "hello world")
				expectedMap.PutInt("test2", 3)
				expectedMap.PutBool("test3", true)
			},
		},
		{
			name:  "limit exact",
			limit: int64(3),
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutStr("test", "hello world")
				expectedMap.PutInt("test2", 3)
				expectedMap.PutBool("test3", true)
			},
		},
		{
			name:  "keep one key",
			limit: int64(2),
			keep:  []string{"test3"},
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutStr("test", "hello world")
				expectedMap.PutBool("test3", true)
			},
		},
		{
			name:  "keep same # of keys as limit",
			limit: int64(2),
			keep:  []string{"test", "test3"},
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutStr("test", "hello world")
				expectedMap.PutBool("test3", true)
			},
		},
		{
			name:  "keep not existing key",
			limit: int64(1),
			keep:  []string{"te"},
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutStr("test", "hello world")
			},
		},
		{
			name:  "keep not-/existing keys",
			limit: int64(2),
			keep:  []string{"te", "test3"},
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutStr("test", "hello world")
				expectedMap.PutBool("test3", true)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scenarioMap := pcommon.NewMap()
			input.CopyTo(scenarioMap)

			setterWasCalled := false
			target := &ottl.StandardPMapGetSetter[pcommon.Map]{
				Getter: func(_ context.Context, tCtx pcommon.Map) (pcommon.Map, error) {
					return tCtx, nil
				},
				Setter: func(_ context.Context, tCtx pcommon.Map, m any) error {
					setterWasCalled = true
					if v, ok := m.(pcommon.Map); ok {
						v.CopyTo(tCtx)
						return nil
					}
					return errors.New("expected pcommon.Map")
				},
			}

			exprFunc, err := limit(target, tt.limit, tt.keep)
			require.NoError(t, err)

			result, err := exprFunc(nil, scenarioMap)
			require.NoError(t, err)
			assert.Nil(t, result)
			// There is a shortcut in limit() that does not call the setter.
			if int(tt.limit) < scenarioMap.Len() {
				assert.True(t, setterWasCalled)
			}

			expected := pcommon.NewMap()
			tt.want(expected)

			assert.Equal(t, expected, scenarioMap)
		})
	}
}

func Test_limit_validation(t *testing.T) {
	tests := []struct {
		name   string
		target ottl.PMapGetSetter[any]
		keep   []string
		limit  int64
	}{
		{
			name:   "limit less than zero",
			target: &ottl.StandardPMapGetSetter[any]{},
			limit:  int64(-1),
		},
		{
			name:   "limit less than # of keep attrs",
			target: &ottl.StandardPMapGetSetter[any]{},
			keep:   []string{"test", "test"},
			limit:  int64(1),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := limit(tt.target, tt.limit, tt.keep)
			assert.Error(t, err)
		})
	}
}

func Test_limit_bad_input(t *testing.T) {
	input := pcommon.NewValueStr("not a map")
	target := &ottl.StandardPMapGetSetter[any]{
		Getter: func(_ context.Context, tCtx any) (pcommon.Map, error) {
			if v, ok := tCtx.(pcommon.Map); ok {
				return v, nil
			}
			return pcommon.Map{}, errors.New("expected pcommon.Map")
		},
	}

	exprFunc, err := limit[any](target, 1, []string{})
	require.NoError(t, err)
	_, err = exprFunc(nil, input)
	assert.Error(t, err)
}

func Test_limit_get_nil(t *testing.T) {
	target := &ottl.StandardPMapGetSetter[any]{
		Getter: func(_ context.Context, tCtx any) (pcommon.Map, error) {
			if v, ok := tCtx.(pcommon.Map); ok {
				return v, nil
			}
			return pcommon.Map{}, errors.New("expected pcommon.Map")
		},
	}

	exprFunc, err := limit[any](target, 1, []string{})
	require.NoError(t, err)
	_, err = exprFunc(nil, nil)
	assert.Error(t, err)
}
