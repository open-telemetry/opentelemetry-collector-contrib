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

func Test_concat(t *testing.T) {
	tests := []struct {
		name      string
		vals      []ottl.StandardStringLikeGetter[interface{}]
		delimiter string
		expected  string
	}{
		{
			name: "concat strings",
			vals: []ottl.StandardStringLikeGetter[interface{}]{
				{
					Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
						return "hello", nil
					},
				},
				{
					Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
						return "world", nil
					},
				},
			},
			delimiter: " ",
			expected:  "hello world",
		},
		{
			name: "nil",
			vals: []ottl.StandardStringLikeGetter[interface{}]{
				{
					Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
						return "hello", nil
					},
				},
				{
					Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
						return nil, nil
					},
				},
				{
					Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
						return "world", nil
					},
				},
			},
			delimiter: "",
			expected:  "hello<nil>world",
		},
		{
			name: "integers",
			vals: []ottl.StandardStringLikeGetter[interface{}]{
				{
					Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
						return "hello", nil
					},
				},
				{
					Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
						return int64(1), nil
					},
				},
			},
			delimiter: "",
			expected:  "hello1",
		},
		{
			name: "floats",
			vals: []ottl.StandardStringLikeGetter[interface{}]{
				{
					Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
						return "hello", nil
					},
				},
				{
					Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
						return 3.14159, nil
					},
				},
			},
			delimiter: "",
			expected:  "hello3.14159",
		},
		{
			name: "booleans",
			vals: []ottl.StandardStringLikeGetter[interface{}]{
				{
					Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
						return "hello", nil
					},
				},
				{
					Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
						return true, nil
					},
				},
			},
			delimiter: " ",
			expected:  "hello true",
		},
		{
			name: "byte slices",
			vals: []ottl.StandardStringLikeGetter[interface{}]{
				{
					Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
						return []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x0e, 0xd2, 0xe6, 0x3c, 0xbe, 0x71, 0xf5, 0xa8}, nil
					},
				},
			},
			delimiter: "",
			expected:  "00000000000000000ed2e63cbe71f5a8",
		},
		{
			name: "pcommon.Slice",
			vals: []ottl.StandardStringLikeGetter[interface{}]{
				{
					Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
						s := pcommon.NewSlice()
						_ = s.FromRaw([]any{1, 2})
						return s, nil
					},
				},
				{
					Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
						s := pcommon.NewSlice()
						_ = s.FromRaw([]any{3, 4})
						return s, nil
					},
				},
			},
			delimiter: ",",
			expected:  "[1,2],[3,4]",
		},
		{
			name: "maps",
			vals: []ottl.StandardStringLikeGetter[interface{}]{
				{
					Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
						m := pcommon.NewMap()
						m.PutStr("a", "b")
						return m, nil
					},
				},
				{
					Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
						m := pcommon.NewMap()
						m.PutStr("c", "d")
						return m, nil
					},
				},
			},
			delimiter: ",",
			expected:  `{"a":"b"},{"c":"d"}`,
		},
		{
			name: "empty string values",
			vals: []ottl.StandardStringLikeGetter[interface{}]{
				{
					Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
						return "", nil
					},
				},
				{
					Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
						return "", nil
					},
				},
				{
					Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
						return "", nil
					},
				},
			},
			delimiter: "__",
			expected:  "____",
		},
		{
			name: "single argument",
			vals: []ottl.StandardStringLikeGetter[interface{}]{
				{
					Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
						return "hello", nil
					},
				},
			},
			delimiter: "-",
			expected:  "hello",
		},
		{
			name:      "no arguments",
			vals:      []ottl.StandardStringLikeGetter[interface{}]{},
			delimiter: "-",
			expected:  "",
		},
		{
			name:      "no arguments with an empty delimiter",
			vals:      []ottl.StandardStringLikeGetter[interface{}]{},
			delimiter: "",
			expected:  "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			getters := make([]ottl.StringLikeGetter[interface{}], len(tt.vals))

			for i, val := range tt.vals {
				getters[i] = val
			}

			exprFunc := concat(getters, tt.delimiter)
			result, err := exprFunc(nil, nil)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_concat_error(t *testing.T) {
	target := &ottl.StandardStringLikeGetter[interface{}]{
		Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
			return make(chan int), nil
		},
	}
	exprFunc := concat[interface{}]([]ottl.StringLikeGetter[interface{}]{target}, "test")
	_, err := exprFunc(context.Background(), nil)
	assert.Error(t, err)
}
