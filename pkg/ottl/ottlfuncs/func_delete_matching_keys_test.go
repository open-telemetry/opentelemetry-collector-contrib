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

func Test_deleteMatchingKeys(t *testing.T) {
	input := pcommon.NewMap()
	input.PutStr("test", "hello world")
	input.PutInt("test2", 3)
	input.PutBool("test3", true)

	target := &ottl.StandardTypeGetter[pcommon.Map, pcommon.Map]{
		Getter: func(ctx context.Context, tCtx pcommon.Map) (interface{}, error) {
			return tCtx, nil
		},
	}

	tests := []struct {
		name    string
		target  ottl.PMapGetter[pcommon.Map]
		pattern string
		want    func(pcommon.Map)
	}{
		{
			name:    "delete everything",
			target:  target,
			pattern: "test.*",
			want: func(expectedMap pcommon.Map) {
				expectedMap.EnsureCapacity(3)
			},
		},
		{
			name:    "delete attributes that end in a number",
			target:  target,
			pattern: "\\d$",
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutStr("test", "hello world")
			},
		},
		{
			name:    "delete nothing",
			target:  target,
			pattern: "not a matching pattern",
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

			exprFunc, err := deleteMatchingKeys(tt.target, tt.pattern)
			assert.NoError(t, err)

			_, err = exprFunc(nil, scenarioMap)
			assert.Nil(t, err)

			expected := pcommon.NewMap()
			tt.want(expected)

			assert.Equal(t, expected, scenarioMap)
		})
	}
}

func Test_deleteMatchingKeys_bad_input(t *testing.T) {
	input := pcommon.NewValueInt(1)
	target := &ottl.StandardTypeGetter[interface{}, pcommon.Map]{
		Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
			return tCtx, nil
		},
	}

	exprFunc, err := deleteMatchingKeys[interface{}](target, "anything")
	assert.NoError(t, err)

	_, err = exprFunc(nil, input)
	assert.Error(t, err)
}

func Test_deleteMatchingKeys_get_nil(t *testing.T) {
	target := &ottl.StandardTypeGetter[interface{}, pcommon.Map]{
		Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
			return tCtx, nil
		},
	}

	exprFunc, err := deleteMatchingKeys[interface{}](target, "anything")
	assert.NoError(t, err)
	_, err = exprFunc(nil, nil)
	assert.Error(t, err)
}

func Test_deleteMatchingKeys_invalid_pattern(t *testing.T) {
	target := &ottl.StandardTypeGetter[interface{}, pcommon.Map]{
		Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
			t.Errorf("nothing should be received in this scenario")
			return nil, nil
		},
	}

	invalidRegexPattern := "*"
	_, err := deleteMatchingKeys[interface{}](target, invalidRegexPattern)
	require.Error(t, err)
	assert.ErrorContains(t, err, "error parsing regexp:")
}
