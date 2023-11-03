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

func Test_isMatch(t *testing.T) {
	tests := []struct {
		name     string
		target   ottl.StringLikeGetter[interface{}]
		pattern  ottl.StringLikeGetter[interface{}]
		expected bool
	}{
		{
			name: "replace match true",
			target: &ottl.StandardStringLikeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "hello world", nil
				},
			},
			pattern: &ottl.StandardStringLikeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "hello.*", nil
				},
			},
			expected: true,
		},
		{
			name: "replace match false",
			target: &ottl.StandardStringLikeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "goodbye world", nil
				},
			},
			pattern: &ottl.StandardStringLikeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "hello.*", nil
				},
			},
			expected: false,
		},
		{
			name: "replace match complex",
			target: &ottl.StandardStringLikeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "-12.001", nil
				},
			},
			pattern: &ottl.StandardStringLikeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "[-+]?\\d*\\.\\d+([eE][-+]?\\d+)?", nil
				},
			},
			expected: true,
		},
		{
			name: "target bool",
			target: &ottl.StandardStringLikeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return true, nil
				},
			},
			pattern: &ottl.StandardStringLikeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "true", nil
				},
			},
			expected: true,
		},
		{
			name: "target int",
			target: &ottl.StandardStringLikeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return int64(1), nil
				},
			},
			pattern: &ottl.StandardStringLikeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "\\d", nil
				},
			},
			expected: true,
		},
		{
			name: "target float",
			target: &ottl.StandardStringLikeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return 1.1, nil
				},
			},
			pattern: &ottl.StandardStringLikeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "\\d.\\d", nil
				},
			},
			expected: true,
		},
		{
			name: "target pcommon.Value",
			target: &ottl.StandardStringLikeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					v := pcommon.NewValueEmpty()
					v.SetStr("test")
					return v, nil
				},
			},
			pattern: &ottl.StandardStringLikeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "test", nil
				},
			},
			expected: true,
		},
		{
			name: "nil target",
			target: &ottl.StandardStringLikeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return nil, nil
				},
			},
			pattern: &ottl.StandardStringLikeGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "impossible to match", nil
				},
			},
			expected: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc, err := isMatch(tt.target, tt.pattern)
			assert.NoError(t, err)
			result, err := exprFunc(context.Background(), nil)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_isMatch_validation(t *testing.T) {
	target := &ottl.StandardStringLikeGetter[interface{}]{
		Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
			return "anything", nil
		},
	}
	pattern := &ottl.StandardStringLikeGetter[interface{}]{
		Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
			return "\\K", nil
		},
	}
	_, err := isMatch[interface{}](target, pattern)
	require.Error(t, err)
}

func Test_isMatch_error(t *testing.T) {
	target := &ottl.StandardStringLikeGetter[interface{}]{
		Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
			return make(chan int), nil
		},
	}
	pattern := &ottl.StandardStringLikeGetter[interface{}]{
		Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
			return "test", nil
		},
	}
	exprFunc, err := isMatch[interface{}](target, pattern)
	assert.NoError(t, err)
	_, err = exprFunc(context.Background(), nil)
	require.Error(t, err)
}
