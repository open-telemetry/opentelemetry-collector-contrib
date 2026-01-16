// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_split(t *testing.T) {
	tests := []struct {
		name      string
		target    ottl.StringGetter[any]
		delimiter ottl.StringGetter[any]
		expected  any
	}{
		{
			name: "split string",
			target: &ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return "A|B|C", nil
				},
			},
			delimiter: &ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return "|", nil
				},
			},
			expected: []string{"A", "B", "C"},
		},
		{
			name: "split empty string",
			target: &ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return "", nil
				},
			},
			delimiter: &ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return "|", nil
				},
			},
			expected: []string{""},
		},
		{
			name: "split empty delimiter",
			target: &ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return "A|B|C", nil
				},
			},
			delimiter: &ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return "", nil
				},
			},
			expected: []string{"A", "|", "B", "|", "C"},
		},
		{
			name: "split empty string and empty delimiter",
			target: &ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return "", nil
				},
			},
			delimiter: &ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return "", nil
				},
			},
			expected: []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc := split(tt.target, tt.delimiter)
			result, err := exprFunc(nil, nil)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_Split_Error(t *testing.T) {
	target := &ottl.StandardStringGetter[any]{
		Getter: func(context.Context, any) (any, error) {
			return 1, nil
		},
	}
	delimiter := &ottl.StandardStringGetter[any]{
		Getter: func(context.Context, any) (any, error) {
			return ",", nil
		},
	}
	exprFunc := split[any](target, delimiter)
	_, err := exprFunc(t.Context(), nil)
	assert.Error(t, err)
}

func Test_Split_Error_delimiter(t *testing.T) {
	target := &ottl.StandardStringGetter[any]{
		Getter: func(context.Context, any) (any, error) {
			return "str", nil
		},
	}
	delimiter := &ottl.StandardStringGetter[any]{
		Getter: func(context.Context, any) (any, error) {
			return 7, nil
		},
	}
	exprFunc := split[any](target, delimiter)
	_, err := exprFunc(t.Context(), nil)
	assert.Error(t, err)
}
