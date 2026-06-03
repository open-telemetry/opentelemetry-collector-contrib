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
	}
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				exprFunc := substring(
					tt.target,
					tt.start,
					tt.length,
					ottl.Optional[bool]{},
				)
				result, err := exprFunc(nil, nil)
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			},
		)
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
		t.Run(
			tt.name, func(t *testing.T) {
				exprFunc := substring(
					tt.target,
					tt.start,
					tt.length,
					ottl.Optional[bool]{},
				)
				result, err := exprFunc(nil, nil)
				assert.Error(t, err)
				assert.Nil(t, result)
			},
		)
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
		t.Run(
			tt.name, func(t *testing.T) {
				exprFunc := substring(
					tt.target,
					tt.start,
					tt.length,
					ottl.Optional[bool]{},
				)
				result, err := exprFunc(nil, nil)
				assert.Error(t, err)
				assert.Nil(t, result)
			},
		)
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
		utf8Safe ottl.Optional[bool]
		expected any
	}{
		{
			name:     "end backs up to rune boundary",
			input:    "一二三",
			start:    0,
			length:   4,
			utf8Safe: ottl.NewTestingOptional(true),
			expected: "一",
		},
		{
			name:     "start advances to rune boundary",
			input:    "一二三",
			start:    1,
			length:   5,
			utf8Safe: ottl.NewTestingOptional(true),
			expected: "二",
		},
		{
			name:     "start advances past end yields empty",
			input:    "一二三",
			start:    1,
			length:   2,
			utf8Safe: ottl.NewTestingOptional(true),
			expected: "",
		},
		{
			name:     "ascii unaffected",
			input:    "abcdef",
			start:    1,
			length:   3,
			utf8Safe: ottl.NewTestingOptional(true),
			expected: "bcd",
		},
		{
			name:     "default (utf8_safe omitted) splits multibyte rune",
			input:    "一二三",
			start:    0,
			length:   4,
			utf8Safe: ottl.Optional[bool]{},
			expected: "一\xe4",
		},
		{
			name:     "utf8_safe=false splits multibyte rune",
			input:    "一二三",
			start:    0,
			length:   1,
			utf8Safe: ottl.NewTestingOptional(false),
			expected: "\xe4",
		},
		{
			name:     "all mid-character bytes",
			input:    "\x80\x80",
			start:    0,
			length:   2,
			utf8Safe: ottl.NewTestingOptional(true),
			expected: "",
		},
		{
			// start snaps forward to end-of-rune,
			// end snaps back to start-of-rune byteStart > byteEnd;
			// the clamp prevents a val[hi:lo] slice panic.
			name:     "both bounds inside one multibyte rune yields empty",
			input:    "\xf0\x9f\x98\x80",
			start:    1,
			length:   1,
			utf8Safe: ottl.NewTestingOptional(true),
			expected: "",
		},
	}
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				exprFunc := substring(
					target(tt.input),
					intGetter(tt.start),
					intGetter(tt.length),
					tt.utf8Safe,
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
		Getter: func(context.Context, any) (any, error) {
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
		{name: "start past byte length", start: 10, length: 1},
		{name: "start+length past byte length", start: 0, length: 10},
		{name: "length past remaining byte length", start: 6, length: 5},
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
