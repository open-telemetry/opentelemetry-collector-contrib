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
		target   ottl.StringLikeGetter[any]
		pattern  string
		expected bool
	}{
		{
			name: "replace match true",
			target: &ottl.StandardStringLikeGetter[any]{
				Getter: func(ctx context.Context, tCtx any) (any, error) {
					return "hello world", nil
				},
			},
			pattern:  "hello.*",
			expected: true,
		},
		{
			name: "replace match false",
			target: &ottl.StandardStringLikeGetter[any]{
				Getter: func(ctx context.Context, tCtx any) (any, error) {
					return "goodbye world", nil
				},
			},
			pattern:  "hello.*",
			expected: false,
		},
		{
			name: "replace match complex",
			target: &ottl.StandardStringLikeGetter[any]{
				Getter: func(ctx context.Context, tCtx any) (any, error) {
					return "-12.001", nil
				},
			},
			pattern:  "[-+]?\\d*\\.\\d+([eE][-+]?\\d+)?",
			expected: true,
		},
		{
			name: "target bool",
			target: &ottl.StandardStringLikeGetter[any]{
				Getter: func(ctx context.Context, tCtx any) (any, error) {
					return true, nil
				},
			},
			pattern:  "true",
			expected: true,
		},
		{
			name: "target int",
			target: &ottl.StandardStringLikeGetter[any]{
				Getter: func(ctx context.Context, tCtx any) (any, error) {
					return int64(1), nil
				},
			},
			pattern:  `\d`,
			expected: true,
		},
		{
			name: "target float",
			target: &ottl.StandardStringLikeGetter[any]{
				Getter: func(ctx context.Context, tCtx any) (any, error) {
					return 1.1, nil
				},
			},
			pattern:  `\d\.\d`,
			expected: true,
		},
		{
			name: "target pcommon.Value",
			target: &ottl.StandardStringLikeGetter[any]{
				Getter: func(ctx context.Context, tCtx any) (any, error) {
					v := pcommon.NewValueEmpty()
					v.SetStr("test")
					return v, nil
				},
			},
			pattern:  `test`,
			expected: true,
		},
		{
			name: "nil target",
			target: &ottl.StandardStringLikeGetter[any]{
				Getter: func(ctx context.Context, tCtx any) (any, error) {
					return nil, nil
				},
			},
			pattern:  "impossible to match",
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
	target := &ottl.StandardStringLikeGetter[any]{
		Getter: func(ctx context.Context, tCtx any) (any, error) {
			return "anything", nil
		},
	}
	_, err := isMatch[any](target, "\\K")
	require.Error(t, err)
}

func Test_isMatch_error(t *testing.T) {
	target := &ottl.StandardStringLikeGetter[any]{
		Getter: func(ctx context.Context, tCtx any) (any, error) {
			return make(chan int), nil
		},
	}
	exprFunc, err := isMatch[any](target, "test")
	assert.NoError(t, err)
	_, err = exprFunc(context.Background(), nil)
	require.Error(t, err)
}
