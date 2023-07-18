// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_Duration(t *testing.T) {
	tests := []struct {
		name     string
		duration ottl.StringGetter[interface{}]
		expected time.Duration
	}{
		{
			name: "100 milliseconds",
			duration: &ottl.StandardStringGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "100ms", nil
				},
			},
			expected: time.Duration(100000000),
		}, {
			name: "234 microseconds",
			duration: &ottl.StandardStringGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "234us", nil
				},
			},
			expected: time.Duration(234000),
		}, {
			name: "777 nanoseconds",
			duration: &ottl.StandardStringGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "777ns", nil
				},
			},
			expected: time.Duration(777),
		},
		{
			name: "one second",
			duration: &ottl.StandardStringGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "1s", nil
				},
			},
			expected: time.Duration(1000000000),
		},
		{
			name: "two hundred second",
			duration: &ottl.StandardStringGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "200s", nil
				},
			},
			expected: time.Duration(200000000000),
		},
		{
			name: "three minutes",
			duration: &ottl.StandardStringGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "3m", nil
				},
			},
			expected: time.Duration(180000000000),
		},
		{
			name: "45 minutes",
			duration: &ottl.StandardStringGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "45m", nil
				},
			},
			expected: time.Duration(2700000000000),
		},
		{
			name: "7 mins, 12 secs",
			duration: &ottl.StandardStringGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "7m12s", nil
				},
			},
			expected: time.Duration(432000000000),
		},
		{
			name: "4 hours",
			duration: &ottl.StandardStringGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "4h", nil
				},
			},
			expected: time.Duration(14400000000000),
		},
		{
			name: "5 hours, 23 mins, 59 secs",
			duration: &ottl.StandardStringGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "5h23m59s", nil
				},
			},
			expected: time.Duration(19439000000000),
		},
		{
			name: "5 hours, 59 secs",
			duration: &ottl.StandardStringGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "5h59s", nil
				},
			},
			expected: time.Duration(18059000000000),
		},
		{
			name: "5 hours, 23 mins",
			duration: &ottl.StandardStringGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "5h23m", nil
				},
			},
			expected: time.Duration(19380000000000),
		},
		{
			name: "2 mins, 1 sec, 64 microsecs",
			duration: &ottl.StandardStringGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "2m1s64us", nil
				},
			},
			expected: time.Duration(121000064000),
		},
		{
			name: "59 hours, 1 min, 78 millisecs",
			duration: &ottl.StandardStringGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "59h1m78ms", nil
				},
			},
			expected: time.Duration(212460078000000),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc, err := Duration(tt.duration)
			assert.NoError(t, err)
			result, err := exprFunc(nil, nil)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_DurationError(t *testing.T) {
	tests := []struct {
		name          string
		duration      ottl.StringGetter[interface{}]
		expectedError string
	}{
		{
			name: "empty duration",
			duration: &ottl.StandardStringGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "", nil
				},
			},
			expectedError: "invalid duration",
		},
		{
			name: "empty duration",
			duration: &ottl.StandardStringGetter[interface{}]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "one second", nil
				},
			},
			expectedError: "invalid duration",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc, err := Duration[any](tt.duration)
			require.NoError(t, err)
			_, err = exprFunc(context.Background(), nil)
			assert.ErrorContains(t, err, tt.expectedError)
		})
	}
}
