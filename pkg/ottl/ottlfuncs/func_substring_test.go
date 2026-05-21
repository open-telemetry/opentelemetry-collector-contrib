// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
				Getter: func(context.Context, any) (any, error) {
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
				Getter: func(context.Context, any) (any, error) {
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
		{
			name: "default byte mode splits multibyte rune",
			target: &ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return "一二三", nil
				},
			},
			start: &ottl.StandardIntGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return int64(0), nil
				},
			},
			length: &ottl.StandardIntGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return int64(1), nil
				},
			},
			expected: "\xe4",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc := substring(tt.target, tt.start, tt.length, ottl.Optional[bool]{})
			result, err := exprFunc(nil, nil)
			require.NoError(t, err)
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
				Getter: func(context.Context, any) (any, error) {
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
				Getter: func(context.Context, any) (any, error) {
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
			exprFunc := substring(tt.target, tt.start, tt.length, ottl.Optional[bool]{})
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
				Getter: func(context.Context, any) (any, error) {
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
				Getter: func(context.Context, any) (any, error) {
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
			name: "substring with start+length overflowing int64",
			target: &ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return "123456789", nil
				},
			},
			start: &ottl.StandardIntGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return int64(math.MaxInt64 - 2), nil
				},
			},
			length: &ottl.StandardIntGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return int64(10), nil
				},
			},
		},
		{
			name: "substring with length overflowing int64 while start is in range",
			target: &ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return "123456789", nil
				},
			},
			start: &ottl.StandardIntGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return int64(5), nil
				},
			},
			length: &ottl.StandardIntGetter[any]{
				Getter: func(context.Context, any) (any, error) {
					return int64(math.MaxInt64), nil
				},
			},
		},
		{
			name: "substring non-string",
			target: &ottl.StandardStringGetter[any]{
				Getter: func(context.Context, any) (any, error) {
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
				Getter: func(context.Context, any) (any, error) {
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
			exprFunc := substring(tt.target, tt.start, tt.length, ottl.Optional[bool]{})
			result, err := exprFunc(nil, nil)
			assert.Error(t, err)
			assert.Nil(t, result)
		})
	}
}

func Test_substring_utf8Safe(t *testing.T) {
	target := func(s string) ottl.StringGetter[any] {
		return &ottl.StandardStringGetter[any]{
			Getter: func(context.Context, any) (any, error) { return s, nil },
		}
	}
	intGetter := func(v int64) ottl.IntGetter[any] {
		return &ottl.StandardIntGetter[any]{
			Getter: func(context.Context, any) (any, error) { return v, nil },
		}
	}

	tests := []struct {
		name     string
		input    string
		start    int64
		length   int64
		expected any
	}{
		{
			name:     "CJK single rune at start",
			input:    "一二三",
			start:    0,
			length:   1,
			expected: "一",
		},
		{
			name:     "CJK full string by rune count",
			input:    "一二三",
			start:    0,
			length:   3,
			expected: "一二三",
		},
		{name: "CJK mid", input: "一二三", start: 1, length: 1, expected: "二"},
		{
			name:     "emoji multiple runes",
			input:    "🎉🎊🎈",
			start:    1,
			length:   2,
			expected: "🎊🎈",
		},
		{
			name:     "ascii unchanged",
			input:    "12345",
			start:    1,
			length:   3,
			expected: "234",
		},
		{
			name:     "mixed ascii and CJK",
			input:    "ab一c",
			start:    1,
			length:   2,
			expected: "b一",
		},
	}
	utf8SafeOpt := ottl.NewTestingOptional(true)
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				exprFunc := substring(
					target(tt.input),
					intGetter(tt.start),
					intGetter(tt.length),
					utf8SafeOpt,
				)
				result, err := exprFunc(nil, nil)
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			},
		)
	}
}

func Test_substring_utf8Safe_error(t *testing.T) {
	target := &ottl.StandardStringGetter[any]{
		Getter: func(context.Context, any) (
			any,
			error,
		) {
			return "一二三", nil
		},
	}
	intGetter := func(v int64) ottl.IntGetter[any] {
		return &ottl.StandardIntGetter[any]{
			Getter: func(context.Context, any) (any, error) { return v, nil },
		}
	}
	utf8SafeOpt := ottl.NewTestingOptional(true)

	tests := []struct {
		name   string
		start  int64
		length int64
	}{
		{name: "start past rune length", start: 4, length: 1},
		{name: "length past rune length", start: 0, length: 4},
		{name: "start+length past rune length", start: 2, length: 2},
	}
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				exprFunc := substring(
					target,
					intGetter(tt.start),
					intGetter(tt.length),
					utf8SafeOpt,
				)
				result, err := exprFunc(nil, nil)
				assert.Error(t, err)
				assert.Nil(t, result)
			},
		)
	}
}
