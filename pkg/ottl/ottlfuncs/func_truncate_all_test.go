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

func Test_truncateAll(t *testing.T) {
	input := pcommon.NewMap()
	// 19 bytes. "hello world, " is 13 bytes, "世界" is 6 bytes.
	input.PutStr("test", "hello world, 世界")
	input.PutInt("test2", 3)
	input.PutBool("test3", true)

	target := &ottl.StandardPMapGetter[pcommon.Map]{
		Getter: func(_ context.Context, tCtx pcommon.Map) (any, error) {
			return tCtx, nil
		},
	}

	tests := []struct {
		name   string
		target ottl.PMapGetter[pcommon.Map]
		limit  int64
		want   func(pcommon.Map)
	}{
		{
			name:   "truncate map",
			target: target,
			limit:  1,
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutStr("test", "h")
				expectedMap.PutInt("test2", 3)
				expectedMap.PutBool("test3", true)
			},
		},
		{
			name:   "truncate map to zero",
			target: target,
			limit:  0,
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutStr("test", "")
				expectedMap.PutInt("test2", 3)
				expectedMap.PutBool("test3", true)
			},
		},
		{
			name:   "truncate nothing",
			target: target,
			limit:  100,
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutStr("test", "hello world, 世界")
				expectedMap.PutInt("test2", 3)
				expectedMap.PutBool("test3", true)
			},
		},
		{
			name:   "truncate broken first utf8 character encoding - 1",
			target: target,
			limit:  14,
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutStr("test", "hello world, ")
				expectedMap.PutInt("test2", 3)
				expectedMap.PutBool("test3", true)
			},
		},
		{
			name:   "truncate broken first utf8 character encoding - 2",
			target: target,
			limit:  15,
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutStr("test", "hello world, ")
				expectedMap.PutInt("test2", 3)
				expectedMap.PutBool("test3", true)
			},
		},
		{
			name:   "truncate first utf8 character exactly",
			target: target,
			limit:  16,
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutStr("test", "hello world, 世")
				expectedMap.PutInt("test2", 3)
				expectedMap.PutBool("test3", true)
			},
		},
		{
			name:   "truncate broken second utf8 character encoding - 1",
			target: target,
			limit:  17,
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutStr("test", "hello world, 世")
				expectedMap.PutInt("test2", 3)
				expectedMap.PutBool("test3", true)
			},
		},
		{
			name:   "truncate broken second utf8 character encoding - 2",
			target: target,
			limit:  18,
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutStr("test", "hello world, 世")
				expectedMap.PutInt("test2", 3)
				expectedMap.PutBool("test3", true)
			},
		},
		{
			name:   "truncate exact",
			target: target,
			limit:  19,
			want: func(expectedMap pcommon.Map) {
				expectedMap.PutStr("test", "hello world, 世界")
				expectedMap.PutInt("test2", 3)
				expectedMap.PutBool("test3", true)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scenarioMap := pcommon.NewMap()
			input.CopyTo(scenarioMap)

			exprFunc, err := TruncateAll(tt.target, tt.limit)
			assert.NoError(t, err)

			result, err := exprFunc(nil, scenarioMap)
			assert.NoError(t, err)
			assert.Nil(t, result)

			expected := pcommon.NewMap()
			tt.want(expected)

			assert.Equal(t, expected, scenarioMap)
		})
	}
}

func Test_truncateAll_validation(t *testing.T) {
	_, err := TruncateAll[any](&ottl.StandardPMapGetter[any]{}, -1)
	require.Error(t, err)
	assert.ErrorContains(t, err, "invalid limit for truncate_all function, -1 cannot be negative")
}

func Test_truncateAll_bad_input(t *testing.T) {
	input := pcommon.NewValueStr("not a map")
	target := &ottl.StandardPMapGetter[any]{
		Getter: func(_ context.Context, tCtx any) (any, error) {
			return tCtx, nil
		},
	}

	exprFunc, err := TruncateAll[any](target, 1)
	assert.NoError(t, err)

	_, err = exprFunc(nil, input)
	assert.Error(t, err)
}

func Test_truncateAll_get_nil(t *testing.T) {
	target := &ottl.StandardPMapGetter[any]{
		Getter: func(_ context.Context, tCtx any) (any, error) {
			return tCtx, nil
		},
	}

	exprFunc, err := TruncateAll[any](target, 1)
	assert.NoError(t, err)

	_, err = exprFunc(nil, nil)
	assert.Error(t, err)
}

func Test_truncateAll_utf8_zero_limit(t *testing.T) {
	input := pcommon.NewMap()
	input.PutStr("test", "世界")

	target := &ottl.StandardPMapGetter[pcommon.Map]{
		Getter: func(_ context.Context, tCtx pcommon.Map) (any, error) {
			return tCtx, nil
		},
	}

	exprFunc, err := TruncateAll(target, 1)
	assert.NoError(t, err)

	result, err := exprFunc(nil, input)
	assert.NoError(t, err)
	assert.Nil(t, result)

	expected := pcommon.NewMap()
	expected.PutStr("test", "")
	assert.Equal(t, expected, input)
}
