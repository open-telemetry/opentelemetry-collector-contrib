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

func Test_deleteKey(t *testing.T) {
	input := pcommon.NewMap()
	input.PutStr("test", "hello world")
	input.PutInt("test2", 3)
	input.PutBool("test3", true)

	target := &ottl.StandardPMapGetter[pcommon.Map]{
		Getter: func(ctx context.Context, tCtx pcommon.Map) (interface{}, error) {
			return tCtx, nil
		},
	}

	tests := []struct {
		name   string
		target ottl.PMapGetter[pcommon.Map]
		key    string
		want   func(pcommon.Map)
	}{
		{
			name:   "delete test",
			target: target,
			key:    "test",
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutBool("test3", true)
				expectedMap.PutInt("test2", 3)
			},
		},
		{
			name:   "delete test2",
			target: target,
			key:    "test2",
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutStr("test", "hello world")
				expectedMap.PutBool("test3", true)
			},
		},
		{
			name:   "delete nothing",
			target: target,
			key:    "not a valid key",
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

			exprFunc := deleteKey(tt.target, tt.key)

			_, err := exprFunc(nil, scenarioMap)
			assert.Nil(t, err)

			expected := pcommon.NewMap()
			tt.want(expected)

			assert.Equal(t, expected, scenarioMap)
		})
	}
}

func Test_deleteKey_bad_input(t *testing.T) {
	input := pcommon.NewValueStr("not a map")
	target := &ottl.StandardPMapGetter[interface{}]{
		Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
			return tCtx, nil
		},
	}

	key := "anything"

	exprFunc := deleteKey[interface{}](target, key)
	_, err := exprFunc(nil, input)
	assert.Error(t, err)
}

func Test_deleteKey_get_nil(t *testing.T) {
	target := &ottl.StandardPMapGetter[interface{}]{
		Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
			return tCtx, nil
		},
	}

	key := "anything"

	exprFunc := deleteKey[interface{}](target, key)
	_, err := exprFunc(nil, nil)
	assert.Error(t, err)
}
