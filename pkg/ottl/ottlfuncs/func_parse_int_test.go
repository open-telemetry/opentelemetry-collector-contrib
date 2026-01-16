// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"fmt"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_ParseInt(t *testing.T) {
	tests := []struct {
		name     string
		target   ottl.StringGetter[any]
		base     ottl.IntGetter[any]
		expected any
		err      bool
	}{
		{
			name: "string in base 10",
			target: &ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return "123456789", nil
				},
			},
			base: &ottl.StandardIntGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return int64(10), nil
				},
			},
			expected: int64(123456789),
		},
		{
			name: "string in base 2",
			target: &ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return "01010101", nil
				},
			},
			base: &ottl.StandardIntGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return int64(2), nil
				},
			},
			expected: int64(85),
		},
		{
			name: "an out of range string",
			target: &ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return fmt.Sprintf("%d", uint64(math.MaxUint64)), nil
				},
			},
			base: &ottl.StandardIntGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return int64(10), nil
				},
			},
			expected: nil,
			err:      true,
		},
		{
			name: "string in base 16",
			target: &ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return "AF", nil
				},
			},
			base: &ottl.StandardIntGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return int64(16), nil
				},
			},
			expected: int64(175),
		},
		{
			name: "string in base 16 with 0x prefix",
			target: &ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return "0XAF", nil
				},
			},
			base: &ottl.StandardIntGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return int64(0), nil
				},
			},
			expected: int64(175),
		},
		{
			name: "not a number string",
			target: &ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return "test", nil
				},
			},
			base: &ottl.StandardIntGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return int64(10), nil
				},
			},
			expected: nil,
			err:      true,
		},
		{
			name: "empty string",
			target: &ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return "", nil
				},
			},
			base: &ottl.StandardIntGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return int64(10), nil
				},
			},
			expected: nil,
			err:      true,
		},
		{
			name: "negative base value",
			target: &ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return "12345", nil
				},
			},
			base: &ottl.StandardIntGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return int64(-10), nil
				},
			},
			expected: nil,
			err:      true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc := parseIntFunc[any](tt.target, tt.base)
			result, err := exprFunc(nil, nil)
			if tt.err {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.expected, result)
		})
	}
}
