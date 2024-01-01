// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_substring(t *testing.T) {
	tests := []struct {
		name     string
		target   ottl.StringGetter[any]
		start    ottl.IntGetter[any]
		length   ottl.IntGetter[any]
		expected any
	}{
		{
			name: "substring",
			target: &ottl.StandardStringGetter[any]{
				Getter: func(ctx context.Context, tCtx any) (any, error) {
					return "123456789", nil
				},
			},
			start: &ottl.StandardIntGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return int64(3), nil
				},
			},
			length: &ottl.StandardIntGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return int64(3), nil
				},
			},
			expected: "456",
		},
		{
			name: "substring with result of total string",
			target: &ottl.StandardStringGetter[any]{
				Getter: func(ctx context.Context, tCtx any) (any, error) {
					return "123456789", nil
				},
			},
			start: &ottl.StandardIntGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return int64(0), nil
				},
			},
			length: &ottl.StandardIntGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return int64(9), nil
				},
			},
			expected: "123456789",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc := substring(tt.target, tt.start, tt.length)
			result, err := exprFunc(nil, nil)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_substring_validation(t *testing.T) {
	tests := []struct {
		name   string
		target ottl.StringGetter[any]
		start  ottl.IntGetter[any]
		length ottl.IntGetter[any]
	}{
		{
			name: "substring with result of empty string",
			target: &ottl.StandardStringGetter[any]{
				Getter: func(ctx context.Context, tCtx any) (any, error) {
					return "123456789", nil
				},
			},
			start: &ottl.StandardIntGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return int64(0), nil
				},
			},
			length: &ottl.StandardIntGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return int64(0), nil
				},
			},
		},
		{
			name: "substring with invalid start index",
			target: &ottl.StandardStringGetter[any]{
				Getter: func(ctx context.Context, tCtx any) (any, error) {
					return "123456789", nil
				},
			},
			start: &ottl.StandardIntGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return int64(-1), nil
				},
			},
			length: &ottl.StandardIntGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return int64(6), nil
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc := substring(tt.target, tt.start, tt.length)
			result, err := exprFunc(nil, nil)
			assert.Error(t, err)
			assert.Nil(t, result)
		})
	}
}

func Test_substring_error(t *testing.T) {
	tests := []struct {
		name   string
		target ottl.StringGetter[any]
		start  ottl.IntGetter[any]
		length ottl.IntGetter[any]
	}{
		{
			name: "substring empty string",
			target: &ottl.StandardStringGetter[any]{
				Getter: func(ctx context.Context, tCtx any) (any, error) {
					return "", nil
				},
			},
			start: &ottl.StandardIntGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return int64(3), nil
				},
			},
			length: &ottl.StandardIntGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return int64(6), nil
				},
			},
		},
		{
			name: "substring with invalid length index",
			target: &ottl.StandardStringGetter[any]{
				Getter: func(ctx context.Context, tCtx any) (any, error) {
					return "123456789", nil
				},
			},
			start: &ottl.StandardIntGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return int64(3), nil
				},
			},
			length: &ottl.StandardIntGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return int64(20), nil
				},
			},
		},
		{
			name: "substring non-string",
			target: &ottl.StandardStringGetter[any]{
				Getter: func(ctx context.Context, tCtx any) (any, error) {
					return 123456789, nil
				},
			},
			start: &ottl.StandardIntGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return int64(3), nil
				},
			},
			length: &ottl.StandardIntGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return int64(6), nil
				},
			},
		},
		{
			name: "substring nil string",
			target: &ottl.StandardStringGetter[any]{
				Getter: func(ctx context.Context, tCtx any) (any, error) {
					return nil, nil
				},
			},
			start: &ottl.StandardIntGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return int64(3), nil
				},
			},
			length: &ottl.StandardIntGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return int64(6), nil
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc := substring(tt.target, tt.start, tt.length)
			result, err := exprFunc(nil, nil)
			assert.Error(t, err)
			assert.Equal(t, nil, result)
		})
	}
}
