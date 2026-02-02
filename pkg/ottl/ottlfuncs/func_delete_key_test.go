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

func Test_deleteKey(t *testing.T) {
	input := pcommon.NewMap()
	input.PutStr("test", "hello world")
	input.PutInt("test2", 3)
	input.PutBool("test3", true)

	tests := []struct {
		name string
		key  ottl.StringGetter[pcommon.Map]
		want func(pcommon.Map)
	}{
		{
			name: "delete test",
			key: ottl.StandardStringGetter[pcommon.Map]{
				Getter: func(_ context.Context, _ pcommon.Map) (any, error) {
					return "test", nil
				},
			},
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutBool("test3", true)
				expectedMap.PutInt("test2", 3)
			},
		},
		{
			name: "delete test2",
			key: ottl.StandardStringGetter[pcommon.Map]{
				Getter: func(_ context.Context, _ pcommon.Map) (any, error) {
					return "test2", nil
				},
			},
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutStr("test", "hello world")
				expectedMap.PutBool("test3", true)
			},
		},
		{
			name: "delete nothing",
			key: ottl.StandardStringGetter[pcommon.Map]{
				Getter: func(_ context.Context, _ pcommon.Map) (any, error) {
					return "not a valid key", nil
				},
			},
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutStr("test", "hello world")
				expectedMap.PutInt("test2", 3)
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
				Setter: func(_ context.Context, tCtx pcommon.Map, val any) error {
					setterWasCalled = true
					if v, ok := val.(pcommon.Map); ok {
						v.CopyTo(tCtx)
						return nil
					}
					return errors.New("expected pcommon.Map")
				},
			}

			exprFunc := deleteKey(target, tt.key)

			_, err := exprFunc(nil, scenarioMap)
			require.NoError(t, err)
			assert.True(t, setterWasCalled)

			expected := pcommon.NewMap()
			tt.want(expected)

			assert.Equal(t, expected, scenarioMap)
		})
	}
}

func Test_deleteKey_bad_input(t *testing.T) {
	input := pcommon.NewValueStr("not a map")
	target := &ottl.StandardPMapGetSetter[any]{
		Getter: func(_ context.Context, tCtx any) (pcommon.Map, error) {
			if v, ok := tCtx.(pcommon.Map); ok {
				return v, nil
			}
			return pcommon.Map{}, errors.New("expected pcommon.Map")
		},
	}

	key := ottl.StandardStringGetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
			return "anything", nil
		},
	}

	exprFunc := deleteKey(target, key)
	_, err := exprFunc(nil, input)
	assert.Error(t, err)
}

func Test_deleteKey_get_nil(t *testing.T) {
	target := &ottl.StandardPMapGetSetter[any]{
		Getter: func(_ context.Context, tCtx any) (pcommon.Map, error) {
			if v, ok := tCtx.(pcommon.Map); ok {
				return v, nil
			}
			return pcommon.Map{}, errors.New("expected pcommon.Map")
		},
	}

	key := ottl.StandardStringGetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
			return "anything", nil
		},
	}

	exprFunc := deleteKey(target, key)
	_, err := exprFunc(nil, nil)
	assert.Error(t, err)
}
