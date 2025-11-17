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
		pattern  ottl.StringGetter[any]
		expected bool
	}{
		{
			name: "replace match true",
			target: &ottl.StandardStringLikeGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return "hello world", nil
				},
			},
			pattern: &ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "hello.*", nil
				},
			},
			expected: true,
		},
		{
			name: "replace match false",
			target: &ottl.StandardStringLikeGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return "goodbye world", nil
				},
			},
			pattern: &ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "hello.*", nil
				},
			},
			expected: false,
		},
		{
			name: "replace match complex",
			target: &ottl.StandardStringLikeGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return "-12.001", nil
				},
			},
			pattern: &ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "[-+]?\\d*\\.\\d+([eE][-+]?\\d+)?", nil
				},
			},
			expected: true,
		},
		{
			name: "target bool",
			target: &ottl.StandardStringLikeGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return true, nil
				},
			},
			pattern: &ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "true", nil
				},
			},
			expected: true,
		},
		{
			name: "target int",
			target: &ottl.StandardStringLikeGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return int64(1), nil
				},
			},
			pattern: &ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return `\d`, nil
				},
			},
			expected: true,
		},
		{
			name: "target float",
			target: &ottl.StandardStringLikeGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return 1.1, nil
				},
			},
			pattern: &ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return `\d\.\d`, nil
				},
			},
			expected: true,
		},
		{
			name: "target pcommon.Value",
			target: &ottl.StandardStringLikeGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					v := pcommon.NewValueEmpty()
					v.SetStr("test")
					return v, nil
				},
			},
			pattern: &ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return `test`, nil
				},
			},
			expected: true,
		},
		{
			name: "nil target",
			target: &ottl.StandardStringLikeGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return nil, nil
				},
			},
			pattern: &ottl.StandardStringGetter[any]{
				Getter: func(_ context.Context, _ any) (any, error) {
					return "impossible to match", nil
				},
			},
			expected: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc, err := isMatch(tt.target, tt.pattern)
			require.NoError(t, err)
			result, err := exprFunc(t.Context(), nil)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_isMatch_validation(t *testing.T) {
	target := &ottl.StandardStringLikeGetter[any]{
		Getter: func(context.Context, any) (any, error) {
			return "anything", nil
		},
	}
	invalidRegexPattern := ottl.StandardStringGetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
			return "\\K", nil
		},
	}
	exprFunc, err := isMatch[any](target, invalidRegexPattern)
	require.NoError(t, err)
	_, err = exprFunc(t.Context(), nil)
	require.Error(t, err)
}

func Test_isMatch_error(t *testing.T) {
	target := &ottl.StandardStringLikeGetter[any]{
		Getter: func(context.Context, any) (any, error) {
			return make(chan int), nil
		},
	}
	regexPattern := ottl.StandardStringGetter[any]{
		Getter: func(_ context.Context, _ any) (any, error) {
			return "test", nil
		},
	}
	exprFunc, err := isMatch[any](target, regexPattern)
	require.NoError(t, err)
	_, err = exprFunc(t.Context(), nil)
	require.Error(t, err)
}
