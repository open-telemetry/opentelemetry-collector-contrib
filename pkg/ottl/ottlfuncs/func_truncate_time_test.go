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

func Test_TruncateTime(t *testing.T) {
	tests := []struct {
		name     string
		time     ottl.TimeGetter[any]
		duration ottl.DurationGetter[any]
		expected time.Time
	}{
		{
			name: "truncate to 1s",
			time: &ottl.StandardTimeGetter[any]{
				Getter: func(ctx context.Context, tCtx any) (any, error) {
					return time.Date(2022, 1, 1, 1, 1, 1, 999999999, time.Local), nil
				},
			},
			duration: &ottl.StandardDurationGetter[any]{
				Getter: func(ctx context.Context, tCtx any) (any, error) {
					d, _ := time.ParseDuration("1s")
					return d, nil
				},
			},
			expected: time.Date(2022, 1, 1, 1, 1, 1, 0, time.Local),
		},
		{
			name: "truncate to 1ms",
			time: &ottl.StandardTimeGetter[any]{
				Getter: func(ctx context.Context, tCtx any) (any, error) {
					return time.Date(2022, 1, 1, 1, 1, 1, 999999999, time.Local), nil
				},
			},
			duration: &ottl.StandardDurationGetter[any]{
				Getter: func(ctx context.Context, tCtx any) (any, error) {
					d, _ := time.ParseDuration("1ms")
					return d, nil
				},
			},
			expected: time.Date(2022, 1, 1, 1, 1, 1, 999000000, time.Local),
		},
		{
			name: "truncate old time",
			time: &ottl.StandardTimeGetter[any]{
				Getter: func(ctx context.Context, tCtx any) (any, error) {
					return time.Date(1980, 9, 9, 9, 59, 59, 999999999, time.Local), nil
				},
			},
			duration: &ottl.StandardDurationGetter[any]{
				Getter: func(ctx context.Context, tCtx any) (any, error) {
					d, _ := time.ParseDuration("1h")
					return d, nil
				},
			},
			expected: time.Date(1980, 9, 9, 9, 0, 0, 0, time.Local),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc, err := TruncateTime(tt.time, tt.duration)
			assert.NoError(t, err)
			result, err := exprFunc(context.Background(), nil)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected.UnixNano(), result.(time.Time).UnixNano())
		})
	}
}

func Test_TruncateTimeError(t *testing.T) {
	tests := []struct {
		name          string
		time          ottl.TimeGetter[any]
		duration      ottl.DurationGetter[any]
		expectedError string
	}{
		{
			name: "not a time",
			time: &ottl.StandardTimeGetter[any]{
				Getter: func(ctx context.Context, tCtx any) (any, error) {
					return "11/11/11", nil
				},
			},
			duration: &ottl.StandardDurationGetter[any]{
				Getter: func(ctx context.Context, tCtx any) (any, error) {
					d, _ := time.ParseDuration("1ms")
					return d, nil
				},
			},
			expectedError: "expected time but got string",
		},
		{
			name: "not a duration",
			time: &ottl.StandardTimeGetter[any]{
				Getter: func(ctx context.Context, tCtx any) (any, error) {
					return time.Now(), nil
				},
			},
			duration: &ottl.StandardDurationGetter[any]{
				Getter: func(ctx context.Context, tCtx any) (any, error) {
					return "string", nil
				},
			},
			expectedError: "expected duration but got string",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc, err := TruncateTime[any](tt.time, tt.duration)
			require.NoError(t, err)
			_, err = exprFunc(context.Background(), nil)
			assert.ErrorContains(t, err, tt.expectedError)
		})
	}
}
