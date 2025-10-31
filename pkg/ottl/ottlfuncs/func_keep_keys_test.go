// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_keepKeys(t *testing.T) {
	input := pcommon.NewMap()
	input.PutStr("test", "hello world")
	input.PutInt("test2", 3)
	input.PutBool("test3", true)

	tests := []struct {
		name string
		keys []string
		want func(pcommon.Map)
	}{
		{
			name: "keep one",
			keys: []string{"test"},
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutStr("test", "hello world")
			},
		},
		{
			name: "keep two",
			keys: []string{"test", "test2"},
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutStr("test", "hello world")
				expectedMap.PutInt("test2", 3)
			},
		},
		{
			name: "keep none",
			keys: []string{},
			want: func(_ pcommon.Map) {},
		},
		{
			name: "no match",
			keys: []string{"no match"},
			want: func(_ pcommon.Map) {},
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

			keys := make([]ottl.StringGetter[pcommon.Map], len(tt.keys))
			for i, key := range tt.keys {
				k := key
				keys[i] = ottl.StandardStringGetter[pcommon.Map]{
					Getter: func(_ context.Context, _ pcommon.Map) (any, error) {
						return k, nil
					},
				}
			}

			exprFunc := keepKeys(target, keys)

			_, err := exprFunc(nil, scenarioMap)
			assert.NoError(t, err)
			assert.True(t, setterWasCalled)

			expected := pcommon.NewMap()
			tt.want(expected)

			assert.Equal(t, expected, scenarioMap)
		})
	}
}

func Test_keepKeys_bad_input(t *testing.T) {
	input := pcommon.NewValueStr("not a map")
	target := &ottl.StandardPMapGetSetter[any]{
		Getter: func(_ context.Context, tCtx any) (pcommon.Map, error) {
			if v, ok := tCtx.(pcommon.Map); ok {
				return v, nil
			}
			return pcommon.Map{}, errors.New("expected pcommon.Map")
		},
	}

	keys := []ottl.StringGetter[any]{
		ottl.StandardStringGetter[any]{
			Getter: func(_ context.Context, _ any) (any, error) {
				return "anything", nil
			},
		},
	}

	exprFunc := keepKeys[any](target, keys)

	_, err := exprFunc(nil, input)
	assert.Error(t, err)
}

func Test_keepKeys_get_nil(t *testing.T) {
	target := &ottl.StandardPMapGetSetter[any]{
		Getter: func(_ context.Context, tCtx any) (pcommon.Map, error) {
			if v, ok := tCtx.(pcommon.Map); ok {
				return v, nil
			}
			return pcommon.Map{}, errors.New("expected pcommon.Map")
		},
	}

	keys := []ottl.StringGetter[any]{
		ottl.StandardStringGetter[any]{
			Getter: func(_ context.Context, _ any) (any, error) {
				return "anything", nil
			},
		},
	}

	exprFunc := keepKeys[any](target, keys)
	_, err := exprFunc(nil, nil)
	assert.Error(t, err)
}
