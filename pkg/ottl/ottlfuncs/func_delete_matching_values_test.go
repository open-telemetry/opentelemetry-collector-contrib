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

func Test_deleteMatchingValues(t *testing.T) {
	tests := []struct {
		name    string
		input   func() pcommon.Map
		pattern ottl.StringGetter[pcommon.Map]
		want    func(pcommon.Map)
	}{
		{
			name: "delete everything",
			input: func() pcommon.Map {
				m := pcommon.NewMap()
				m.PutStr("key1", "hello world")
				m.PutStr("key2", "foo bar")
				return m
			},
			pattern: ottl.StandardStringGetter[pcommon.Map]{Getter: func(_ context.Context, _ pcommon.Map) (any, error) { return ".*", nil }},
			want: func(expectedMap pcommon.Map) {
				expectedMap.EnsureCapacity(2)
			},
		},
		{
			name: "delete values that are empty strings",
			input: func() pcommon.Map {
				m := pcommon.NewMap()
				m.PutStr("key1", "hello world")
				m.PutStr("key2", "")
				return m
			},
			pattern: ottl.StandardStringGetter[pcommon.Map]{Getter: func(_ context.Context, _ pcommon.Map) (any, error) { return "^$", nil }},
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutStr("key1", "hello world")
			},
		},
		{
			name: "delete nothing",
			input: func() pcommon.Map {
				m := pcommon.NewMap()
				m.PutStr("key1", "hello world")
				m.PutStr("key2", "foo bar")
				return m
			},
			pattern: ottl.StandardStringGetter[pcommon.Map]{Getter: func(_ context.Context, _ pcommon.Map) (any, error) { return "not a matching pattern", nil }},
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutStr("key1", "hello world")
				expectedMap.PutStr("key2", "foo bar")
			},
		},
		{
			name: "non-string types are not deleted",
			input: func() pcommon.Map {
				m := pcommon.NewMap()
				m.PutStr("str", "hello")
				m.PutInt("int", 3)
				m.PutBool("bool", true)
				m.PutEmptyMap("map").PutStr("nested", "value")
				m.PutEmptySlice("slice").AppendEmpty().SetStr("item")
				return m
			},
			pattern: ottl.StandardStringGetter[pcommon.Map]{Getter: func(_ context.Context, _ pcommon.Map) (any, error) { return ".*", nil }},
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutInt("int", 3)
				expectedMap.PutBool("bool", true)
				expectedMap.PutEmptyMap("map").PutStr("nested", "value")
				expectedMap.PutEmptySlice("slice").AppendEmpty().SetStr("item")
			},
		},
		{
			name: "mixed types only delete matching strings",
			input: func() pcommon.Map {
				m := pcommon.NewMap()
				m.PutStr("str1", "sensitive-data")
				m.PutStr("str2", "safe-value")
				m.PutInt("int", 42)
				m.PutBool("bool", false)
				return m
			},
			pattern: ottl.StandardStringGetter[pcommon.Map]{Getter: func(_ context.Context, _ pcommon.Map) (any, error) { return "sensitive.*", nil }},
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutStr("str2", "safe-value")
				expectedMap.PutInt("int", 42)
				expectedMap.PutBool("bool", false)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scenarioMap := tt.input()

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

			exprFunc, err := deleteMatchingValues(target, tt.pattern)
			require.NoError(t, err)

			_, err = exprFunc(nil, scenarioMap)
			require.NoError(t, err)
			assert.True(t, setterWasCalled)

			expected := pcommon.NewMap()
			tt.want(expected)

			assert.Equal(t, expected, scenarioMap)
		})
	}
}

func Test_deleteMatchingValues_bad_input(t *testing.T) {
	input := pcommon.NewValueInt(1)
	target := &ottl.StandardPMapGetSetter[any]{
		Getter: func(_ context.Context, tCtx any) (pcommon.Map, error) {
			if v, ok := tCtx.(pcommon.Map); ok {
				return v, nil
			}
			return pcommon.Map{}, errors.New("expected pcommon.Map")
		},
	}

	pattern := ottl.StandardStringGetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
			return "anything", nil
		},
	}

	exprFunc, err := deleteMatchingValues(target, pattern)
	require.NoError(t, err)
	_, err = exprFunc(nil, input)
	assert.Error(t, err)
}

func Test_deleteMatchingValues_get_nil(t *testing.T) {
	target := &ottl.StandardPMapGetSetter[any]{
		Getter: func(_ context.Context, tCtx any) (pcommon.Map, error) {
			if v, ok := tCtx.(pcommon.Map); ok {
				return v, nil
			}
			return pcommon.Map{}, errors.New("expected pcommon.Map")
		},
	}

	pattern := ottl.StandardStringGetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
			return "anything", nil
		},
	}

	exprFunc, err := deleteMatchingValues(target, pattern)
	require.NoError(t, err)
	_, err = exprFunc(nil, nil)
	assert.Error(t, err)
}
