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

func Test_deleteMatchingKeys(t *testing.T) {
	input := pcommon.NewMap()
	input.PutStr("test", "hello world")
	input.PutInt("test2", 3)
	input.PutBool("test3", true)

	tests := []struct {
		name    string
		pattern ottl.StringGetter[pcommon.Map]
		want    func(pcommon.Map)
	}{
		{
			name:    "delete everything",
			pattern: ottl.StandardStringGetter[pcommon.Map]{Getter: func(_ context.Context, _ pcommon.Map) (any, error) { return "test.*", nil }},
			want: func(expectedMap pcommon.Map) {
				expectedMap.EnsureCapacity(3)
			},
		},
		{
			name:    "delete attributes that end in a number",
			pattern: ottl.StandardStringGetter[pcommon.Map]{Getter: func(_ context.Context, _ pcommon.Map) (any, error) { return "\\d$", nil }},
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutStr("test", "hello world")
			},
		},
		{
			name:    "delete nothing",
			pattern: ottl.StandardStringGetter[pcommon.Map]{Getter: func(_ context.Context, _ pcommon.Map) (any, error) { return "not a matching pattern", nil }},
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
				Setter: func(_ context.Context, tCtx pcommon.Map, m any) error {
					setterWasCalled = true
					if v, ok := m.(pcommon.Map); ok {
						v.CopyTo(tCtx)
						return nil
					}
					return errors.New("expected pcommon.Map")
				},
			}

			exprFunc, err := deleteMatchingKeys(target, tt.pattern)
			assert.NoError(t, err)

			_, err = exprFunc(nil, scenarioMap)
			assert.NoError(t, err)
			assert.True(t, setterWasCalled)

			expected := pcommon.NewMap()
			tt.want(expected)

			assert.Equal(t, expected, scenarioMap)
		})
	}
}

func Test_deleteMatchingKeys_bad_input(t *testing.T) {
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

	exprFunc, err := deleteMatchingKeys(target, pattern)
	assert.NoError(t, err)
	_, err = exprFunc(nil, input)
	assert.Error(t, err)
}

func Test_deleteMatchingKeys_get_nil(t *testing.T) {
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

	exprFunc, err := deleteMatchingKeys(target, pattern)
	assert.NoError(t, err)
	_, err = exprFunc(nil, nil)
	assert.Error(t, err)
}

func Test_deleteMatchingKeys_invalid_pattern(t *testing.T) {
	target := &ottl.StandardPMapGetSetter[any]{
		Getter: func(context.Context, any) (pcommon.Map, error) {
			t.Errorf("nothing should be received in this scenario")
			return pcommon.Map{}, nil
		},
	}

	invalidRegexPattern := ottl.StandardStringGetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
			return "*", nil
		},
	}
	exprFunc, err := deleteMatchingKeys(target, invalidRegexPattern)
	assert.NoError(t, err)
	_, err = exprFunc(nil, nil)
	assert.ErrorContains(t, err, "error parsing regexp:")
}
